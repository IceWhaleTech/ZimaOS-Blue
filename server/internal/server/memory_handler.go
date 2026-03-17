package server

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// MemoryHandler handles memory-related endpoints.
type MemoryHandler struct {
	unifiedService *memory.UnifiedMemoryService
	layeredService *memory.LayeredMemoryService

	// Lazy init support
	initOnce sync.Once
	initFunc func()
	initErr  error

	// Callback when layered service becomes ready (e.g. wire into ChatHandler)
	onLayeredReady func(*memory.LayeredMemoryService)
}

// NewMemoryHandler creates a new MemoryHandler.
func NewMemoryHandler() *MemoryHandler {
	return &MemoryHandler{}
}

// NewLazyMemoryHandler creates a handler that defers heavy init to first request.
func NewLazyMemoryHandler(initFunc func(h *MemoryHandler) error) *MemoryHandler {
	h := &MemoryHandler{}
	h.initFunc = func() {
		h.initErr = initFunc(h)
	}
	return h
}

// ensureInit triggers lazy initialization if configured.
func (h *MemoryHandler) ensureInit() {
	if h.initFunc != nil {
		h.initOnce.Do(h.initFunc)
	}
}

// Init triggers lazy initialization eagerly (e.g. at startup).
func (h *MemoryHandler) Init() {
	h.ensureInit()
}

// SetUnifiedService sets the unified memory service.
func (h *MemoryHandler) SetUnifiedService(svc *memory.UnifiedMemoryService) {
	h.unifiedService = svc
}

// SetOnLayeredReady registers a callback invoked when the layered service is set.
// Used by bootstrap to wire layered memory into ChatHandler without the caller
// having to know about the lazy init timing.
func (h *MemoryHandler) SetOnLayeredReady(fn func(*memory.LayeredMemoryService)) {
	h.onLayeredReady = fn
}

// SetLayeredService sets the layered memory service.
func (h *MemoryHandler) SetLayeredService(svc *memory.LayeredMemoryService) {
	h.layeredService = svc
	if h.onLayeredReady != nil {
		h.onLayeredReady(svc)
	}
}

// GetUnifiedService returns the unified memory service.
func (h *MemoryHandler) GetUnifiedService() *memory.UnifiedMemoryService {
	return h.unifiedService
}

// RegisterRoutes registers memory routes.
func (h *MemoryHandler) RegisterRoutes(g *echo.Group) {
	g.POST("/memory/store", h.Store)
	g.POST("/memory/search", h.Search)
	g.POST("/memory/prune", h.Prune)
	g.DELETE("/memory", h.Clear)
	g.GET("/memory/stats", h.Stats)
	g.GET("/memory/export", h.ExportMarkdown)
	g.POST("/memory/import", h.ImportMarkdown)
	// Param routes after specific routes
	g.GET("/memory/:id", h.Get)
	g.DELETE("/memory/:id", h.Delete)
	// Layered memory routes
	g.POST("/memory/daily", h.AppendToDaily)
	g.GET("/memory/daily", h.ListDailyLogs)
	g.GET("/memory/daily/:date", h.GetDailyLog)
	g.POST("/memory/longterm", h.PromoteToLongTerm)
	g.GET("/memory/longterm", h.GetLongTermMemory)
	g.POST("/memory/daily/prune", h.PruneDailyLogs)
}

// Store stores a new memory.
func (h *MemoryHandler) Store(c echo.Context) error {
	h.ensureInit()
	if h.unifiedService == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "memory service not configured")
	}
	var req struct {
		Content string   `json:"content"`
		Tags    []string `json:"tags,omitempty"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.Content == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "content is required")
	}
	chunk, err := h.unifiedService.Remember(c.Request().Context(), req.Content, req.Tags)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusCreated, map[string]interface{}{
		"id":         chunk.ID,
		"content":    chunk.Content,
		"created_at": chunk.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// Search searches memories.
func (h *MemoryHandler) Search(c echo.Context) error {
	h.ensureInit()
	if h.unifiedService == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "memory service not configured")
	}
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit,omitempty"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.Query == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "query is required")
	}
	limit := req.Limit
	if limit == 0 {
		limit = 10
	}
	results, err := h.unifiedService.Recall(c.Request().Context(), req.Query, limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	items := make([]map[string]interface{}, len(results))
	for i, r := range results {
		items[i] = map[string]interface{}{
			"id":            r.Chunk.ID,
			"content":       r.Chunk.Content,
			"score":         r.Score,
			"keyword_score": r.KeywordScore,
			"match_types":   r.MatchTypes,
			"metadata":      r.Chunk.Metadata,
			"created_at":    r.Chunk.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"results": items,
		"total":   len(items),
	})
}

// Get retrieves a memory by ID.
func (h *MemoryHandler) Get(c echo.Context) error {
	h.ensureInit()
	if h.unifiedService == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "memory service not configured")
	}
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is required")
	}
	decodedID, err := url.PathUnescape(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	id = decodedID
	chunk, err := h.unifiedService.Get(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, memory.ErrInvalidPath) {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":         chunk.ID,
		"content":    chunk.Content,
		"metadata":   chunk.Metadata,
		"created_at": chunk.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"updated_at": chunk.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// Delete deletes a memory.
func (h *MemoryHandler) Delete(c echo.Context) error {
	h.ensureInit()
	if h.unifiedService == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "memory service not configured")
	}
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is required")
	}
	decodedID, err := url.PathUnescape(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	id = decodedID
	if err := h.unifiedService.Forget(c.Request().Context(), id); err != nil {
		if errors.Is(err, memory.ErrInvalidPath) {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

// Prune removes old memories.
func (h *MemoryHandler) Prune(c echo.Context) error {
	h.ensureInit()
	if h.unifiedService == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "memory service not configured")
	}
	deleted, err := h.unifiedService.Prune(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]int{"deleted": deleted})
}

// Clear removes all memories.
func (h *MemoryHandler) Clear(c echo.Context) error {
	h.ensureInit()
	if h.unifiedService == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "memory service not configured")
	}
	if err := h.unifiedService.ForgetAll(c.Request().Context()); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

// Stats returns memory statistics.
func (h *MemoryHandler) Stats(c echo.Context) error {
	h.ensureInit()
	if h.unifiedService == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "memory service not configured")
	}
	stats, err := h.unifiedService.Stats(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	resp := map[string]interface{}{
		"total_chunks":             stats.TotalChunks,
		"total_size_bytes":         stats.TotalSizeBytes,
		"oldest_chunk":             stats.OldestChunk,
		"newest_chunk":             stats.NewestChunk,
		"backend":                  stats.Backend,
		"total_display_count":      stats.TotalChunks,
		"total_display_size_bytes": stats.TotalSizeBytes,
		"daily_logs_count":         0,
		"daily_entries_count":      0,
		"daily_total_size_bytes":   int64(0),
	}

	// Auto-extracted memories are written to daily logs (layered memory) and are
	// not part of unified chunk stats. Include them for UI display counters.
	if h.layeredService != nil {
		dates, listErr := h.layeredService.ListDailyLogs(c.Request().Context())
		if listErr == nil {
			dailyEntries := 0
			dailySize := int64(0)
			for _, d := range dates {
				content, readErr := h.layeredService.GetDailyLog(c.Request().Context(), d)
				if readErr != nil {
					continue
				}
				dailyEntries += countDailyLogEntries(content)
				dailySize += int64(len(content))
			}
			resp["daily_logs_count"] = len(dates)
			resp["daily_entries_count"] = dailyEntries
			resp["daily_total_size_bytes"] = dailySize
			resp["total_display_count"] = stats.TotalChunks + dailyEntries
			resp["total_display_size_bytes"] = stats.TotalSizeBytes + dailySize
		}
	}

	return c.JSON(http.StatusOK, resp)
}

func countDailyLogEntries(content string) int {
	if content == "" {
		return 0
	}
	count := 0
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "## ") && len(line) >= 11 {
			count++
		}
	}
	return count
}

// ExportMarkdown exports all memories as a Markdown file.
func (h *MemoryHandler) ExportMarkdown(c echo.Context) error {
	h.ensureInit()
	if h.unifiedService == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "memory service not configured")
	}
	stats, err := h.unifiedService.Stats(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	now := timeutil.NowTime().Format("2006-01-02 15:04:05")
	md := "# ZimaOS-Blue Memory Export\n\n"
	md += "> Exported at: " + now + "\n"
	md += "> Backend: " + stats.Backend + "\n\n"
	c.Response().Header().Set("Content-Type", "text/markdown; charset=utf-8")
	c.Response().Header().Set("Content-Disposition", "attachment; filename=memory-export.md")
	return c.String(http.StatusOK, md)
}

// ImportMarkdown imports memories from a Markdown payload.
func (h *MemoryHandler) ImportMarkdown(c echo.Context) error {
	h.ensureInit()
	if h.unifiedService == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "memory service not configured")
	}

	var req struct {
		Content string `json:"content"`
		Mode    string `json:"mode,omitempty"` // append | replace
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if strings.TrimSpace(req.Content) == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "content is required")
	}

	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = "append"
	}
	if mode != "append" && mode != "replace" {
		return echo.NewHTTPError(http.StatusBadRequest, "mode must be append or replace")
	}

	if mode == "replace" {
		if err := h.unifiedService.ForgetAll(c.Request().Context()); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	}

	entries := parseMarkdownImportEntries(req.Content)
	imported := 0
	skipped := 0
	errors := make([]string, 0)

	for i, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			skipped++
			continue
		}
		if _, err := h.unifiedService.Remember(c.Request().Context(), entry, nil); err != nil {
			errors = append(errors, fmt.Sprintf("entry %d: %v", i+1, err))
			continue
		}
		imported++
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"imported": imported,
		"skipped":  skipped,
		"errors":   errors,
	})
}

func parseMarkdownImportEntries(content string) []string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	parts := strings.Split(normalized, "\n---")
	entries := make([]string, 0, len(parts))

	for _, raw := range parts {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		lines := strings.Split(raw, "\n")
		kept := make([]string, 0, len(lines))
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			switch {
			case trimmed == "---":
				continue
			case strings.HasPrefix(trimmed, "# ZimaOS-Blue Memory Export"):
				continue
			case strings.HasPrefix(trimmed, "> Exported at:"):
				continue
			case strings.HasPrefix(trimmed, "> Backend:"):
				continue
			default:
				kept = append(kept, line)
			}
		}

		entry := strings.TrimSpace(strings.Join(kept, "\n"))
		if entry != "" {
			entries = append(entries, entry)
		}
	}

	return entries
}

// === Layered Memory Handlers ===

// AppendToDaily appends content to today's daily log.
func (h *MemoryHandler) AppendToDaily(c echo.Context) error {
	h.ensureInit()
	if h.layeredService == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "layered memory service not configured")
	}
	var req struct {
		Content string   `json:"content"`
		Tags    []string `json:"tags,omitempty"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.Content == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "content is required")
	}
	if err := h.layeredService.AppendToDaily(c.Request().Context(), req.Content, req.Tags); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusCreated, map[string]bool{"success": true})
}

// ListDailyLogs returns a list of available daily log dates.
func (h *MemoryHandler) ListDailyLogs(c echo.Context) error {
	h.ensureInit()
	if h.layeredService == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "layered memory service not configured")
	}
	dates, err := h.layeredService.ListDailyLogs(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"dates": dates,
		"count": len(dates),
	})
}

// GetDailyLog returns a specific day's log content.
func (h *MemoryHandler) GetDailyLog(c echo.Context) error {
	h.ensureInit()
	if h.layeredService == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "layered memory service not configured")
	}
	date := c.Param("date")
	if date == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "date is required")
	}
	content, err := h.layeredService.GetDailyLog(c.Request().Context(), date)
	if err != nil {
		if errors.Is(err, memory.ErrInvalidDailyLogDate) {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"date":    date,
		"content": content,
	})
}

// PromoteToLongTerm promotes content to the long-term memory layer.
func (h *MemoryHandler) PromoteToLongTerm(c echo.Context) error {
	h.ensureInit()
	if h.layeredService == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "layered memory service not configured")
	}
	var req struct {
		Content  string `json:"content"`
		Category string `json:"category,omitempty"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.Content == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "content is required")
	}
	category := req.Category
	if category == "" {
		category = "General"
	}
	if err := h.layeredService.PromoteToLongTerm(c.Request().Context(), req.Content, category); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusCreated, map[string]bool{"success": true})
}

// GetLongTermMemory returns the long-term memory content.
func (h *MemoryHandler) GetLongTermMemory(c echo.Context) error {
	h.ensureInit()
	if h.layeredService == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "layered memory service not configured")
	}
	content, err := h.layeredService.GetLongTermMemory(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"content": content,
	})
}

// PruneDailyLogs removes old daily logs based on retention policy.
func (h *MemoryHandler) PruneDailyLogs(c echo.Context) error {
	h.ensureInit()
	if h.layeredService == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "layered memory service not configured")
	}
	deleted, err := h.layeredService.PruneDailyLogs(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"deleted": deleted,
	})
}
