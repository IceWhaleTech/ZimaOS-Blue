package memory

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

// APIHandler provides HTTP endpoints for the Memory Service API.
type APIHandler struct {
	repo *MemoryRepository
	ns   *NamespaceStore
}

// NewAPIHandler creates a new API handler.
func NewAPIHandler(repo *MemoryRepository, ns *NamespaceStore) *APIHandler {
	return &APIHandler{repo: repo, ns: ns}
}

// RegisterRoutes registers all memory API routes on the given Echo group.
func (h *APIHandler) RegisterRoutes(g *echo.Group) {
	mem := g.Group("/memories")
	mem.POST("", h.CreateEntry)
	mem.GET("/:id", h.GetEntry)
	mem.PUT("/:id", h.UpdateEntry)
	mem.DELETE("/:id", h.DeleteEntry)
	mem.GET("", h.ListEntries)
	mem.POST("/search", h.SearchEntries)
	mem.POST("/batch", h.BatchCreate)
	mem.GET("/:id/history", h.GetHistory)
	mem.DELETE("/expired", h.PurgeExpired)
	mem.GET("/stats", h.GetStats)

	ns := g.Group("/namespaces")
	ns.POST("", h.CreateNamespace)
	ns.GET("", h.ListNamespaces)
	ns.DELETE("/:id", h.DeleteNamespace)
}

func getNamespace(c echo.Context) string {
	ns := c.Request().Header.Get("X-Namespace")
	if ns == "" {
		ns = c.QueryParam("namespace")
	}
	if ns == "" {
		ns = "default"
	}
	return ns
}

// CreateEntry handles POST /memories
func (h *APIHandler) CreateEntry(c echo.Context) error {
	var req struct {
		Content     string         `json:"content"`
		ContentType ContentType    `json:"content_type,omitempty"`
		Category    string         `json:"category,omitempty"`
		Tags        []string       `json:"tags,omitempty"`
		Metadata    map[string]any `json:"metadata,omitempty"`
		Importance  *float32       `json:"importance,omitempty"`
		Source      string         `json:"source,omitempty"`
		TTL         Duration       `json:"ttl,omitempty"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if req.Content == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "content is required"})
	}

	e := NewMemoryEntry(getNamespace(c), req.Content)
	if req.ContentType != "" {
		e.ContentType = req.ContentType
	}
	e.Category = req.Category
	e.Tags = req.Tags
	e.Metadata = req.Metadata
	e.Source = req.Source
	e.TTL = req.TTL
	if req.Importance != nil {
		e.Importance = *req.Importance
	}
	e.ComputeExpiresAt()

	if err := h.repo.Create(c.Request().Context(), e); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, e)
}

// GetEntry handles GET /memories/:id
func (h *APIHandler) GetEntry(c echo.Context) error {
	e, err := h.repo.Get(c.Request().Context(), c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, e)
}

// UpdateEntry handles PUT /memories/:id
func (h *APIHandler) UpdateEntry(c echo.Context) error {
	var req struct {
		Content string `json:"content"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if req.Content == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "content is required"})
	}

	updated, err := h.repo.Update(c.Request().Context(), c.Param("id"), req.Content)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, updated)
}

// DeleteEntry handles DELETE /memories/:id
func (h *APIHandler) DeleteEntry(c echo.Context) error {
	if err := h.repo.Delete(c.Request().Context(), c.Param("id")); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "deleted"})
}

// ListEntries handles GET /memories
func (h *APIHandler) ListEntries(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	entries, err := h.repo.List(c.Request().Context(), ListParams{
		Namespace: getNamespace(c),
		Status:    EntryStatus(c.QueryParam("status")),
		Category:  c.QueryParam("category"),
		Cursor:    c.QueryParam("cursor"),
		Limit:     limit,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	var nextCursor string
	if len(entries) > 0 {
		nextCursor = entries[len(entries)-1].ID
	}
	return c.JSON(http.StatusOK, map[string]any{
		"entries":     entries,
		"next_cursor": nextCursor,
		"count":       len(entries),
	})
}

// SearchEntries handles POST /memories/search
func (h *APIHandler) SearchEntries(c echo.Context) error {
	var q SearchQuery
	if err := c.Bind(&q); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if q.Namespace == "" {
		q.Namespace = getNamespace(c)
	}
	if q.TopK <= 0 {
		q.TopK = 10
	}
	// For now, search delegates to List with category filter.
	// Full vector search integration will use the existing HybridSearcher.
	entries, err := h.repo.List(c.Request().Context(), ListParams{
		Namespace: q.Namespace,
		Limit:     q.TopK,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	results := make([]SearchResult, len(entries))
	for i, e := range entries {
		results[i] = SearchResult{Entry: e, Score: 1.0, MatchTypes: []string{"keyword"}}
	}
	return c.JSON(http.StatusOK, map[string]any{
		"results": results,
		"count":   len(results),
	})
}

// BatchCreate handles POST /memories/batch
func (h *APIHandler) BatchCreate(c echo.Context) error {
	var req struct {
		Entries []struct {
			Content     string         `json:"content"`
			ContentType ContentType    `json:"content_type,omitempty"`
			Category    string         `json:"category,omitempty"`
			Tags        []string       `json:"tags,omitempty"`
			Metadata    map[string]any `json:"metadata,omitempty"`
			Importance  *float32       `json:"importance,omitempty"`
			Source      string         `json:"source,omitempty"`
			TTL         Duration       `json:"ttl,omitempty"`
		} `json:"entries"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	ns := getNamespace(c)
	created := make([]*MemoryEntry, 0, len(req.Entries))
	for _, r := range req.Entries {
		if r.Content == "" {
			continue
		}
		e := NewMemoryEntry(ns, r.Content)
		if r.ContentType != "" {
			e.ContentType = r.ContentType
		}
		e.Category = r.Category
		e.Tags = r.Tags
		e.Metadata = r.Metadata
		e.Source = r.Source
		e.TTL = r.TTL
		if r.Importance != nil {
			e.Importance = *r.Importance
		}
		e.ComputeExpiresAt()
		if err := h.repo.Create(c.Request().Context(), e); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		created = append(created, e)
	}
	return c.JSON(http.StatusCreated, map[string]any{
		"entries": created,
		"count":   len(created),
	})
}

// GetHistory handles GET /memories/:id/history
func (h *APIHandler) GetHistory(c echo.Context) error {
	chain, err := h.repo.History(c.Request().Context(), c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"versions": chain,
		"count":    len(chain),
	})
}

// PurgeExpired handles DELETE /memories/expired
func (h *APIHandler) PurgeExpired(c echo.Context) error {
	n, err := h.repo.PurgeExpired(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"purged": n,
	})
}

// GetStats handles GET /memories/stats
func (h *APIHandler) GetStats(c echo.Context) error {
	stats, err := h.repo.Stats(c.Request().Context(), getNamespace(c))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, stats)
}

// CreateNamespace handles POST /namespaces
func (h *APIHandler) CreateNamespace(c echo.Context) error {
	var req struct {
		ID     string          `json:"id"`
		Config NamespaceConfig `json:"config"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if req.ID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "id is required"})
	}
	ns := &Namespace{ID: req.ID, Config: req.Config}
	if ns.Config.MaxEntries == 0 {
		ns.Config = DefaultNamespaceConfig()
	}
	if err := h.ns.Create(c.Request().Context(), ns); err != nil {
		return c.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, ns)
}

// ListNamespaces handles GET /namespaces
func (h *APIHandler) ListNamespaces(c echo.Context) error {
	list, err := h.ns.List(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"namespaces": list,
		"count":      len(list),
	})
}

// DeleteNamespace handles DELETE /namespaces/:id
func (h *APIHandler) DeleteNamespace(c echo.Context) error {
	if err := h.ns.Delete(c.Request().Context(), c.Param("id")); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "deleted"})
}
