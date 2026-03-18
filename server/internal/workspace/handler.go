package workspace

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
	"github.com/labstack/echo/v4"
)

// Handler provides HTTP endpoints for workspace file management.
type Handler struct {
	mgr                  *Manager
	allowedRootsProvider func() []string
}

// NewHandler creates a new workspace HTTP handler.
func NewHandler(mgr *Manager) *Handler {
	return &Handler{mgr: mgr}
}

// SetAllowedRootsProvider sets an optional provider for extra roots that can
// be scanned by getTree (for example directory whitelist roots).
func (h *Handler) SetAllowedRootsProvider(fn func() []string) {
	h.allowedRootsProvider = fn
}

// Manager returns the underlying workspace manager.
func (h *Handler) Manager() *Manager { return h.mgr }

// RegisterRoutes registers workspace API routes on the given group.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.GET("/meta", h.getMeta)
	g.GET("/tree", h.getTree)
	g.GET("/files", h.listFiles)
	g.GET("/files/:name", h.getFile)
	g.PUT("/files/:name", h.putFile)
	g.GET("/git/capability", h.getGitCapability)
	g.GET("/stats", h.getStats)
	g.POST("/bootstrap/complete", h.completeBootstrap)
	g.GET("/bootstrap/status", h.bootstrapStatus)
}

// getMeta returns lightweight workspace metadata.
func (h *Handler) getMeta(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"dir": h.mgr.Dir(),
	})
}

type workspaceTreeEntry struct {
	Path      string `json:"path"`
	AbsPath   string `json:"abs_path"`
	Name      string `json:"name"`
	Type      string `json:"type"` // "file" | "dir"
	Depth     int    `json:"depth"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
}

// getTree returns the real workspace directory tree for browsing files.
func (h *Handler) getTree(c echo.Context) error {
	const (
		defaultMaxDepth = 8
		maxAllowedDepth = 16
		maxEntries      = 5000
	)

	maxDepth := defaultMaxDepth
	if raw := strings.TrimSpace(c.QueryParam("max_depth")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			if v < 1 {
				maxDepth = 1
			} else if v > maxAllowedDepth {
				maxDepth = maxAllowedDepth
			} else {
				maxDepth = v
			}
		}
	}

	includeHidden := false
	if raw := strings.TrimSpace(strings.ToLower(c.QueryParam("include_hidden"))); raw == "1" || raw == "true" || raw == "yes" {
		includeHidden = true
	}

	root := strings.TrimSpace(h.mgr.Dir())
	if requestedRoot := strings.TrimSpace(c.QueryParam("root")); requestedRoot != "" {
		resolvedRoot, err := h.resolveTreeRoot(requestedRoot)
		if err != nil {
			return c.JSON(http.StatusForbidden, map[string]string{
				"error": err.Error(),
			})
		}
		root = resolvedRoot
	}
	if root == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"root":    "",
			"entries": []workspaceTreeEntry{},
		})
	}
	info, statErr := os.Stat(root)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return c.JSON(http.StatusOK, map[string]interface{}{
				"root":    root,
				"entries": []workspaceTreeEntry{},
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": statErr.Error(),
		})
	}
	if !info.IsDir() {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "root is not a directory",
		})
	}

	entries := make([]workspaceTreeEntry, 0, 512)
	count := 0

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if path == root {
			return nil
		}
		if count >= maxEntries {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		rel = strings.TrimSpace(rel)
		if rel == "" || rel == "." {
			return nil
		}

		if !includeHidden && hasHiddenSegment(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		depth := strings.Count(rel, "/") + 1
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		entryType := "file"
		if d.IsDir() {
			entryType = "dir"
		}

		var sizeBytes int64
		if !d.IsDir() {
			if info, err := d.Info(); err == nil {
				sizeBytes = info.Size()
			}
		}

		entries = append(entries, workspaceTreeEntry{
			Path:      rel,
			AbsPath:   path,
			Name:      d.Name(),
			Type:      entryType,
			Depth:     depth,
			SizeBytes: sizeBytes,
		})
		count++
		return nil
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"root":    root,
		"entries": entries,
	})
}

func (h *Handler) resolveTreeRoot(raw string) (string, error) {
	workspaceRoot := strings.TrimSpace(h.mgr.Dir())
	if workspaceRoot == "" {
		return "", nil
	}
	workspaceAbs, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return "", err
	}
	workspaceAbs = filepath.Clean(workspaceAbs)

	candidate := strings.TrimSpace(raw)
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(workspaceAbs, candidate)
	}
	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	candidateAbs = filepath.Clean(candidateAbs)

	for _, allowedRoot := range h.collectAllowedRoots(workspaceAbs) {
		if isWithinRoot(allowedRoot, candidateAbs) {
			return candidateAbs, nil
		}
	}
	return "", fmt.Errorf("root path is outside allowed directories")
}

func (h *Handler) collectAllowedRoots(workspaceAbs string) []string {
	roots := []string{workspaceAbs}
	if h.allowedRootsProvider == nil {
		return roots
	}
	extra := h.allowedRootsProvider()
	if len(extra) == 0 {
		return roots
	}
	seen := map[string]struct{}{workspaceAbs: {}}
	for _, raw := range extra {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		abs, err := filepath.Abs(trimmed)
		if err != nil {
			continue
		}
		clean := filepath.Clean(abs)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		roots = append(roots, clean)
	}
	return roots
}

func isWithinRoot(root string, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func hasHiddenSegment(rel string) bool {
	parts := strings.Split(rel, "/")
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p == "" || p == "." || p == ".." {
			continue
		}
		if strings.HasPrefix(p, ".") {
			return true
		}
	}
	return false
}

// listFiles returns all workspace files with their content.
func (h *Handler) listFiles(c echo.Context) error {
	files := h.mgr.LoadBootstrapFiles()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"files": files,
	})
}

// getFile returns a single workspace file.
func (h *Handler) getFile(c echo.Context) error {
	name := c.Param("name")
	content, err := h.mgr.ReadFile(name)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": err.Error(),
		})
	}
	return c.JSON(http.StatusOK, BootstrapFile{
		Name:    name,
		Content: content,
	})
}

// putFile updates a workspace file.
func (h *Handler) putFile(c echo.Context) error {
	name := c.Param("name")

	// Support both JSON body and raw text
	contentType := c.Request().Header.Get("Content-Type")

	var content string
	if contentType == "" || strings.HasPrefix(contentType, "application/json") {
		var req struct {
			Content string `json:"content"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "invalid request body",
			})
		}
		content = req.Content
	} else {
		body, err := io.ReadAll(io.LimitReader(c.Request().Body, MaxFileSize)) // 1MB limit
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "failed to read body",
			})
		}
		content = string(body)
	}

	if err := h.mgr.WriteFile(name, content); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "ok",
		"name":   name,
		"bytes":  len(content),
	})
}

// completeBootstrap removes BOOTSTRAP.md, marking the first-run guide as done.
func (h *Handler) completeBootstrap(c echo.Context) error {
	if err := h.mgr.CompleteBootstrap(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":  "completed",
		"pending": false,
	})
}

// bootstrapStatus returns whether the first-run bootstrap is still pending.
func (h *Handler) bootstrapStatus(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"pending": h.mgr.IsBootstrapPending(),
	})
}

func (h *Handler) getGitCapability(c echo.Context) error {
	return c.JSON(http.StatusOK, h.mgr.DetectGitCapability(c.Request().Context()))
}

// FileTokenStat holds per-file token estimate.
type FileTokenStat struct {
	Name   string `json:"name"`
	Bytes  int    `json:"bytes"`
	Tokens int    `json:"tokens"`
}

// getStats returns token estimates for all workspace context files.
func (h *Handler) getStats(c echo.Context) error {
	ctx := h.mgr.LoadContextFiles()
	stats := make([]FileTokenStat, 0, len(ctx))
	totalTokens := 0
	totalBytes := 0
	for name, content := range ctx {
		text := pruner.MarkdownToTextMinimal(content)
		tokens := pruner.EstimateTokens(text)
		stats = append(stats, FileTokenStat{
			Name:   name,
			Bytes:  len(text),
			Tokens: tokens,
		})
		totalTokens += tokens
		totalBytes += len(text)
	}
	// Sort for deterministic JSON output
	sort.Slice(stats, func(i, j int) bool { return stats[i].Name < stats[j].Name })
	return c.JSON(http.StatusOK, map[string]interface{}{
		"files":        stats,
		"total_tokens": totalTokens,
		"total_bytes":  totalBytes,
	})
}
