package server

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/memory"
)

// MemoryHandler handles memory-related endpoints.
type MemoryHandler struct {
	service *memory.MemoryService
}

// NewMemoryHandler creates a new MemoryHandler.
func NewMemoryHandler(service *memory.MemoryService) *MemoryHandler {
	return &MemoryHandler{service: service}
}

// RegisterRoutes registers memory routes.
func (h *MemoryHandler) RegisterRoutes(g *echo.Group) {
	g.POST("/memory/store", h.Store)
	g.POST("/memory/search", h.Search)
	g.GET("/memory/:id", h.Get)
	g.DELETE("/memory/:id", h.Delete)
	g.POST("/memory/prune", h.Prune)
	g.DELETE("/memory", h.Clear)
	g.GET("/memory/stats", h.Stats)
}

// StoreRequest represents a memory store request.
type StoreRequest struct {
	Content string   `json:"content" validate:"required"`
	Tags    []string `json:"tags,omitempty"`
}

// StoreResponse represents a memory store response.
type StoreResponse struct {
	ID        string `json:"id"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

// Store stores a new memory.
func (h *MemoryHandler) Store(c echo.Context) error {
	if h.service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "memory service not configured")
	}

	var req StoreRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.Content == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "content is required")
	}

	chunk, err := h.service.Remember(c.Request().Context(), req.Content, req.Tags)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusCreated, StoreResponse{
		ID:        chunk.ID,
		Content:   chunk.Content,
		CreatedAt: chunk.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// SearchRequest represents a memory search request.
type SearchRequest struct {
	Query      string `json:"query" validate:"required"`
	Limit      int    `json:"limit,omitempty"`
	SearchType string `json:"search_type,omitempty"` // "hybrid", "vector", "keyword"
}

// SearchResponse represents a memory search response.
type SearchResponse struct {
	Results []SearchResultItem `json:"results"`
	Total   int                `json:"total"`
}

// SearchResultItem represents a single search result.
type SearchResultItem struct {
	ID            string            `json:"id"`
	Content       string            `json:"content"`
	Score         float32           `json:"score"`
	VectorScore   float32           `json:"vector_score,omitempty"`
	KeywordScore  float32           `json:"keyword_score,omitempty"`
	MatchTypes    []string          `json:"match_types"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	CreatedAt     string            `json:"created_at"`
}

// Search searches memories.
func (h *MemoryHandler) Search(c echo.Context) error {
	if h.service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "memory service not configured")
	}

	var req SearchRequest
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

	results, err := h.service.Recall(c.Request().Context(), req.Query, limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	items := make([]SearchResultItem, len(results))
	for i, r := range results {
		items[i] = SearchResultItem{
			ID:           r.Chunk.ID,
			Content:      r.Chunk.Content,
			Score:        r.CombinedScore,
			VectorScore:  r.VectorScore,
			KeywordScore: r.KeywordScore,
			MatchTypes:   r.MatchTypes,
			Metadata:     r.Chunk.Metadata,
			CreatedAt:    r.Chunk.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	return c.JSON(http.StatusOK, SearchResponse{
		Results: items,
		Total:   len(items),
	})
}

// Get retrieves a memory by ID.
func (h *MemoryHandler) Get(c echo.Context) error {
	if h.service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "memory service not configured")
	}

	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is required")
	}

	chunk, err := h.service.Get(c.Request().Context(), id)
	if err != nil {
		if err == memory.ErrNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "memory not found")
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
	if h.service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "memory service not configured")
	}

	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is required")
	}

	if err := h.service.Forget(c.Request().Context(), id); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

// PruneResponse represents a prune response.
type PruneResponse struct {
	Deleted int `json:"deleted"`
}

// Prune removes old memories.
func (h *MemoryHandler) Prune(c echo.Context) error {
	if h.service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "memory service not configured")
	}

	deleted, err := h.service.Prune(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, PruneResponse{Deleted: deleted})
}

// Clear removes all memories.
func (h *MemoryHandler) Clear(c echo.Context) error {
	if h.service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "memory service not configured")
	}

	if err := h.service.ForgetAll(c.Request().Context()); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

// Stats returns memory statistics.
func (h *MemoryHandler) Stats(c echo.Context) error {
	if h.service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "memory service not configured")
	}

	stats, err := h.service.Stats(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, stats)
}
