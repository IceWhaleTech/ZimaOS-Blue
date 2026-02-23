package server

import (
	"net/http"
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

// SetLayeredService sets the layered memory service.
func (h *MemoryHandler) SetLayeredService(svc *memory.LayeredMemoryService) {
	h.layeredService = svc
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
			"id":           r.Chunk.ID,
			"content":      r.Chunk.Content,
			"score":        r.Score,
			"keyword_score": r.KeywordScore,
			"match_types":  r.MatchTypes,
			"metadata":     r.Chunk.Metadata,
			"created_at":   r.Chunk.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
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
	chunk, err := h.unifiedService.Get(c.Request().Context(), id)
	if err != nil {
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
	if err := h.unifiedService.Forget(c.Request().Context(), id); err != nil {
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
	return c.JSON(http.StatusOK, stats)
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
