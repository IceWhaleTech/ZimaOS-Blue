package workspace

import (
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
	"github.com/labstack/echo/v4"
)

// Handler provides HTTP endpoints for workspace file management.
type Handler struct {
	mgr *Manager
}

// NewHandler creates a new workspace HTTP handler.
func NewHandler(mgr *Manager) *Handler {
	return &Handler{mgr: mgr}
}

// Manager returns the underlying workspace manager.
func (h *Handler) Manager() *Manager { return h.mgr }

// RegisterRoutes registers workspace API routes on the given group.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.GET("/files", h.listFiles)
	g.GET("/files/:name", h.getFile)
	g.PUT("/files/:name", h.putFile)
	g.GET("/stats", h.getStats)
	g.POST("/bootstrap/complete", h.completeBootstrap)
	g.GET("/bootstrap/status", h.bootstrapStatus)
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

	// Auto-complete bootstrap when USER.md is written with non-default content
	if name == FileUSER && h.mgr.IsBootstrapPending() {
		_ = h.mgr.CompleteBootstrap()
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "ok",
		"name":   name,
		"bytes":  len(content),
	})
}

// completeBootstrap removes BOOTSTRAP.md, marking the first-run guide as done.
func (h *Handler) completeBootstrap(c echo.Context) error {
	if !h.mgr.IsBootstrapPending() {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":  "already_completed",
			"pending": false,
		})
	}
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
		compacted := pruner.CompactMarkdown(content)
		tokens := pruner.EstimateTokens(compacted)
		stats = append(stats, FileTokenStat{
			Name:   name,
			Bytes:  len(compacted),
			Tokens: tokens,
		})
		totalTokens += tokens
		totalBytes += len(compacted)
	}
	// Sort for deterministic JSON output
	sort.Slice(stats, func(i, j int) bool { return stats[i].Name < stats[j].Name })
	return c.JSON(http.StatusOK, map[string]interface{}{
		"files":        stats,
		"total_tokens": totalTokens,
		"total_bytes":  totalBytes,
	})
}
