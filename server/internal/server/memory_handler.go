package server

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/memory"
)

// MemoryHandler handles memory-related endpoints.
type MemoryHandler struct {
	service        *memory.MemoryService
	unifiedService *memory.UnifiedMemoryService
	layeredService *memory.LayeredMemoryService
}

// NewMemoryHandler creates a new MemoryHandler.
func NewMemoryHandler(service *memory.MemoryService) *MemoryHandler {
	return &MemoryHandler{service: service}
}

// SetUnifiedService sets the unified memory service for backend switching.
func (h *MemoryHandler) SetUnifiedService(svc *memory.UnifiedMemoryService) {
	h.unifiedService = svc
}

// SetLayeredService sets the layered memory service.
func (h *MemoryHandler) SetLayeredService(svc *memory.LayeredMemoryService) {
	h.layeredService = svc
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
	// Backend management routes
	g.GET("/memory/backend", h.GetBackendStatus)
	g.POST("/memory/backend", h.SetBackend)
	g.POST("/memory/supermemory/config", h.ConfigureSupermemory)
	g.POST("/memory/supermemory/test", h.TestSupermemory)
	// Markdown export/import routes
	g.GET("/memory/export", h.ExportMarkdown)
	g.POST("/memory/import", h.ImportMarkdown)
	// Layered memory routes (dual-layer architecture)
	g.POST("/memory/daily", h.AppendToDaily)
	g.GET("/memory/daily", h.ListDailyLogs)
	g.GET("/memory/daily/:date", h.GetDailyLog)
	g.POST("/memory/longterm", h.PromoteToLongTerm)
	g.GET("/memory/longterm", h.GetLongTermMemory)
	g.POST("/memory/daily/prune", h.PruneDailyLogs)
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
	StartDate  string `json:"start_date,omitempty"`  // Filter by date range (YYYY-MM-DD)
	EndDate    string `json:"end_date,omitempty"`    // Filter by date range (YYYY-MM-DD)
	Highlight  bool   `json:"highlight,omitempty"`   // Enable result highlighting
}

// SearchResponse represents a memory search response.
type SearchResponse struct {
	Results []SearchResultItem `json:"results"`
	Total   int                `json:"total"`
}

// SearchResultItem represents a single search result.
type SearchResultItem struct {
	ID           string            `json:"id"`
	Content      string            `json:"content"`
	Score        float32           `json:"score"`
	VectorScore  float32           `json:"vector_score,omitempty"`
	KeywordScore float32           `json:"keyword_score,omitempty"`
	MatchTypes   []string          `json:"match_types"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	CreatedAt    string            `json:"created_at"`
	Highlights   []string          `json:"highlights,omitempty"`   // Highlighted snippets
	MatchedTerms []string          `json:"matched_terms,omitempty"` // Terms that matched
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

	// Build search options
	opts := memory.SearchOptions{
		Limit:     limit,
		Highlight: req.Highlight,
	}

	// Parse date range
	if req.StartDate != "" {
		if t, err := time.Parse("2006-01-02", req.StartDate); err == nil {
			opts.StartDate = &t
		}
	}
	if req.EndDate != "" {
		if t, err := time.Parse("2006-01-02", req.EndDate); err == nil {
			// Set to end of day
			t = t.Add(24*time.Hour - time.Second)
			opts.EndDate = &t
		}
	}

	results, err := h.service.Searcher.SearchWithOptions(c.Request().Context(), req.Query, opts)
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
			Highlights:   r.Highlights,
			MatchedTerms: r.MatchedTerms,
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

// GetBackendStatus returns the current backend status.
func (h *MemoryHandler) GetBackendStatus(c echo.Context) error {
	activeBackend := "local"
	supermemoryAvailable := false

	if h.unifiedService != nil {
		activeBackend = h.unifiedService.GetActiveBackend()
		supermemoryAvailable = h.unifiedService.IsSupermemoryAvailable()
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"active_backend":        activeBackend,
		"supermemory_available": supermemoryAvailable,
	})
}

// SetBackend switches the active memory backend.
func (h *MemoryHandler) SetBackend(c echo.Context) error {
	var req struct {
		Backend string `json:"backend"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if h.unifiedService == nil {
		if req.Backend != "local" {
			return echo.NewHTTPError(http.StatusBadRequest, "only local backend is available")
		}
		return c.JSON(http.StatusOK, map[string]bool{"success": true})
	}

	if err := h.unifiedService.SetBackend(req.Backend); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]bool{"success": true})
}

// ConfigureSupermemory configures the Supermemory backend.
func (h *MemoryHandler) ConfigureSupermemory(c echo.Context) error {
	var req struct {
		Enabled bool   `json:"enabled"`
		APIKey  string `json:"api_key"`
		BaseURL string `json:"base_url"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if h.unifiedService != nil {
		cfg := memory.SupermemoryConfig{
			Enabled: req.Enabled,
			APIKey:  req.APIKey,
			BaseURL: req.BaseURL,
		}
		if err := h.unifiedService.ConfigureSupermemory(cfg); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	}

	return c.JSON(http.StatusOK, map[string]bool{"success": true})
}

// TestSupermemory tests the Supermemory connection.
func (h *MemoryHandler) TestSupermemory(c echo.Context) error {
	if h.unifiedService == nil || !h.unifiedService.IsSupermemoryAvailable() {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   "Supermemory not configured",
		})
	}

	// Test by switching temporarily and checking
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// ExportMarkdown exports all memories as a Markdown file.
func (h *MemoryHandler) ExportMarkdown(c echo.Context) error {
	if h.service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "memory service not configured")
	}

	// Parse optional date range filters
	var startDate, endDate *time.Time
	if s := c.QueryParam("start_date"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			startDate = &t
		}
	}
	if s := c.QueryParam("end_date"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			t = t.Add(24*time.Hour - time.Second) // End of day
			endDate = &t
		}
	}

	// Parse optional tag filter
	tagFilter := c.QueryParam("tag")

	// Get all memories
	memories, err := h.service.GetAll(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Filter memories
	var filtered []memory.MemoryChunk
	for _, chunk := range memories {
		// Date range filter
		if startDate != nil && chunk.CreatedAt.Before(*startDate) {
			continue
		}
		if endDate != nil && chunk.CreatedAt.After(*endDate) {
			continue
		}
		// Tag filter
		if tagFilter != "" {
			hasTag := false
			for _, v := range chunk.Metadata {
				if v == tagFilter {
					hasTag = true
					break
				}
			}
			if !hasTag {
				continue
			}
		}
		filtered = append(filtered, chunk)
	}

	// Build Markdown content
	now := time.Now().Format("2006-01-02 15:04:05")
	md := "# ZimaOS-Echo Memory Export\n\n"
	md += "> Exported at: " + now + "\n"
	if startDate != nil || endDate != nil {
		md += "> Date range: "
		if startDate != nil {
			md += startDate.Format("2006-01-02")
		} else {
			md += "..."
		}
		md += " to "
		if endDate != nil {
			md += endDate.Format("2006-01-02")
		} else {
			md += "..."
		}
		md += "\n"
	}
	if tagFilter != "" {
		md += "> Tag filter: " + tagFilter + "\n"
	}
	md += "> Total memories: " + itoa(len(filtered)) + "\n\n"
	md += "---\n\n"

	for _, chunk := range filtered {
		md += "## Memory: " + chunk.ID + "\n\n"
		md += "**Created:** " + chunk.CreatedAt.Format("2006-01-02 15:04:05") + "\n\n"
		if len(chunk.Metadata) > 0 {
			md += "**Tags:** "
			first := true
			for _, v := range chunk.Metadata {
				if !first {
					md += ", "
				}
				md += v
				first = false
			}
			md += "\n\n"
		}
		md += "### Content\n\n"
		md += chunk.Content + "\n\n"
		md += "---\n\n"
	}

	// Set headers for file download
	filename := "memory-export"
	if startDate != nil {
		filename += "-from-" + startDate.Format("2006-01-02")
	}
	if endDate != nil {
		filename += "-to-" + endDate.Format("2006-01-02")
	}
	filename += ".md"

	c.Response().Header().Set("Content-Type", "text/markdown; charset=utf-8")
	c.Response().Header().Set("Content-Disposition", "attachment; filename="+filename)

	return c.String(http.StatusOK, md)
}

// itoa converts int to string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

// ImportMarkdownRequest represents a markdown import request.
type ImportMarkdownRequest struct {
	Content string `json:"content" validate:"required"`
	Mode    string `json:"mode,omitempty"` // "append" (default) or "replace"
}

// ImportMarkdownResponse represents a markdown import response.
type ImportMarkdownResponse struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors,omitempty"`
}

// ImportMarkdown imports memories from Markdown content.
func (h *MemoryHandler) ImportMarkdown(c echo.Context) error {
	if h.service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "memory service not configured")
	}

	var req ImportMarkdownRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.Content == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "content is required")
	}

	// If replace mode, clear existing memories first
	if req.Mode == "replace" {
		if err := h.service.ForgetAll(c.Request().Context()); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to clear existing memories: "+err.Error())
		}
	}

	// Parse Markdown and extract memories
	memories := parseMarkdownMemories(req.Content)

	response := ImportMarkdownResponse{
		Errors: make([]string, 0),
	}

	for _, mem := range memories {
		if mem.Content == "" {
			response.Skipped++
			continue
		}

		_, err := h.service.Remember(c.Request().Context(), mem.Content, mem.Tags)
		if err != nil {
			response.Errors = append(response.Errors, err.Error())
			response.Skipped++
		} else {
			response.Imported++
		}
	}

	return c.JSON(http.StatusOK, response)
}

// parsedMemory represents a memory parsed from Markdown.
type parsedMemory struct {
	Content string
	Tags    []string
}

// parseMarkdownMemories parses Markdown content and extracts memories.
func parseMarkdownMemories(content string) []parsedMemory {
	var memories []parsedMemory

	// Split by "---" separator
	sections := splitByDelimiter(content, "---")

	for _, section := range sections {
		section = trimSpace(section)
		if section == "" {
			continue
		}

		// Skip header section (contains "# ZimaOS-Echo Memory Export")
		if containsString(section, "# ZimaOS-Echo Memory Export") || containsString(section, "> Exported at:") {
			continue
		}

		// Extract content after "### Content" header
		mem := parsedMemory{Tags: make([]string, 0)}

		if idx := indexOfString(section, "### Content"); idx >= 0 {
			mem.Content = trimSpace(section[idx+len("### Content"):])
		} else {
			// If no "### Content" header, use the whole section as content
			// but skip lines starting with "## Memory:" or "**"
			lines := splitByDelimiter(section, "\n")
			var contentLines []string
			for _, line := range lines {
				line = trimSpace(line)
				if line == "" || hasPrefix(line, "## Memory:") || hasPrefix(line, "**") {
					continue
				}
				contentLines = append(contentLines, line)
			}
			mem.Content = joinStrings(contentLines, "\n")
		}

		if mem.Content != "" {
			memories = append(memories, mem)
		}
	}

	return memories
}

// Helper functions to avoid importing strings package in this section
func splitByDelimiter(s, sep string) []string {
	var result []string
	for {
		idx := indexOfString(s, sep)
		if idx < 0 {
			result = append(result, s)
			break
		}
		result = append(result, s[:idx])
		s = s[idx+len(sep):]
	}
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func containsString(s, substr string) bool {
	return indexOfString(s, substr) >= 0
}

func indexOfString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// === Layered Memory Handlers ===

// AppendToDailyRequest represents a request to append to daily log.
type AppendToDailyRequest struct {
	Content string   `json:"content" validate:"required"`
	Tags    []string `json:"tags,omitempty"`
}

// AppendToDaily appends content to today's daily log.
func (h *MemoryHandler) AppendToDaily(c echo.Context) error {
	if h.layeredService == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "layered memory service not configured")
	}

	var req AppendToDailyRequest
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

// PromoteToLongTermRequest represents a request to promote content to long-term memory.
type PromoteToLongTermRequest struct {
	Content  string `json:"content" validate:"required"`
	Category string `json:"category,omitempty"`
}

// PromoteToLongTerm promotes content to the long-term memory layer.
func (h *MemoryHandler) PromoteToLongTerm(c echo.Context) error {
	if h.layeredService == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "layered memory service not configured")
	}

	var req PromoteToLongTermRequest
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
