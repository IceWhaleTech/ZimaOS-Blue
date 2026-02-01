package companion

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/sync/singleflight"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/cache"
)

// Handler handles REST API requests for the companion service.
type Handler struct {
	manager *Manager
	storage Storage

	// singleflight for deduplicating concurrent requests
	sfGroup singleflight.Group

	// cache for frequently accessed data (using ecache2 generic cache)
	statsCache   *cache.GenericCache[string]
	sessionCache *cache.GenericCache[string]
}

// NewHandler creates a new REST API handler.
func NewHandler(manager *Manager, storage Storage) *Handler {
	return &Handler{
		manager: manager,
		storage: storage,
		statsCache: cache.NewGenericCacheWithStats(cache.Config{
			MaxSize:    50,
			DefaultTTL: 5 * time.Second,
		}, "companion_stats"),
		sessionCache: cache.NewGenericCacheWithStats(cache.Config{
			MaxSize:    200,
			DefaultTTL: 10 * time.Second,
		}, "companion_sessions"),
	}
}

// getContextString safely gets a string value from echo context with a default fallback.
func getContextString(c echo.Context, key, defaultValue string) string {
	if val := c.Get(key); val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

// RegisterRoutes registers the companion API routes.
func (h *Handler) RegisterRoutes(e *echo.Echo) {
	// Register under /api/v1/companion
	g := e.Group("/api/v1/companion")
	h.registerCompanionRoutes(g)

	// Also register under /api/companion for frontend compatibility
	g2 := e.Group("/api/companion")
	h.registerCompanionRoutes(g2)
}

// registerCompanionRoutes registers companion routes on a group.
func (h *Handler) registerCompanionRoutes(g *echo.Group) {
	// Sessions
	g.GET("/sessions", h.ListSessions)
	g.GET("/sessions/:id", h.GetSession)
	g.GET("/sessions/:id/events", h.GetSessionEvents)
	g.GET("/sessions/:id/flow", h.GetSessionFlow)
	g.DELETE("/sessions/:id", h.DeleteSession)

	// Alerts
	g.GET("/alerts", h.ListAlerts)
	g.PUT("/alerts/:id/ack", h.AcknowledgeAlert)
	g.PUT("/alerts/bulk-ack", h.BulkAcknowledgeAlerts)

	// Stats
	g.GET("/stats", h.GetStats)

	// Export
	g.GET("/export", h.Export)

	// Settings
	g.GET("/settings", h.GetSettings)
	g.PUT("/settings", h.UpdateSettings)
	g.POST("/cleanup", h.TriggerCleanup)
}

// ListSessions handles GET /api/v1/companion/sessions
func (h *Handler) ListSessions(c echo.Context) error {
	opts := h.parseListOptions(c)

	sessions, total, err := h.manager.ListSessions(c.Request().Context(), opts)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"sessions": sessions,
		"total":    total,
		"offset":   opts.Offset,
		"limit":    opts.Limit,
	})
}

// GetSession handles GET /api/v1/companion/sessions/:id
func (h *Handler) GetSession(c echo.Context) error {
	id := c.Param("id")

	session, err := h.manager.GetSession(c.Request().Context(), id)
	if err != nil {
		if err == ErrSessionNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "session not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, session)
}

// GetSessionEvents handles GET /api/v1/companion/sessions/:id/events
func (h *Handler) GetSessionEvents(c echo.Context) error {
	id := c.Param("id")
	opts := h.parseListOptions(c)

	events, total, err := h.manager.GetSessionEvents(c.Request().Context(), id, opts)
	if err != nil {
		if err == ErrSessionNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "session not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"events": events,
		"total":  total,
		"offset": opts.Offset,
		"limit":  opts.Limit,
	})
}

// GetSessionFlow handles GET /api/v1/companion/sessions/:id/flow
func (h *Handler) GetSessionFlow(c echo.Context) error {
	id := c.Param("id")

	flow, err := h.manager.GetSessionFlow(c.Request().Context(), id)
	if err != nil {
		if err == ErrSessionNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "session not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, flow)
}

// DeleteSession handles DELETE /api/v1/companion/sessions/:id
func (h *Handler) DeleteSession(c echo.Context) error {
	id := c.Param("id")

	err := h.storage.DeleteSession(c.Request().Context(), id)
	if err != nil {
		if err == ErrSessionNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "session not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "session deleted"})
}

// ListAlerts handles GET /api/v1/companion/alerts
func (h *Handler) ListAlerts(c echo.Context) error {
	opts := h.parseListOptions(c)

	// Parse additional alert filters
	if severity := c.QueryParam("severity"); severity != "" {
		if opts.Filters == nil {
			opts.Filters = make(map[string]string)
		}
		opts.Filters["severity"] = severity
	}
	if acked := c.QueryParam("acknowledged"); acked != "" {
		if opts.Filters == nil {
			opts.Filters = make(map[string]string)
		}
		opts.Filters["acknowledged"] = acked
	}

	alerts, total, err := h.storage.ListAlerts(c.Request().Context(), opts)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"alerts": alerts,
		"total":  total,
		"offset": opts.Offset,
		"limit":  opts.Limit,
	})
}

// AcknowledgeAlert handles PUT /api/v1/companion/alerts/:id/ack
func (h *Handler) AcknowledgeAlert(c echo.Context) error {
	id := c.Param("id")

	// Get user ID from context (set by auth middleware)
	userID := getContextString(c, "user_id", "anonymous")

	alert, err := h.storage.GetAlert(c.Request().Context(), id)
	if err != nil {
		if err == ErrAlertNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "alert not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	now := time.Now()
	alert.Acknowledged = true
	alert.AckedAt = &now
	alert.AckedBy = userID

	if err := h.storage.UpdateAlert(c.Request().Context(), alert); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, alert)
}

// BulkAcknowledgeAlerts handles PUT /api/v1/companion/alerts/bulk-ack
func (h *Handler) BulkAcknowledgeAlerts(c echo.Context) error {
	var req struct {
		AlertIDs []string `json:"alert_ids"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	if len(req.AlertIDs) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "alert_ids is required"})
	}

	// Get user ID from context (set by auth middleware)
	userID := getContextString(c, "user_id", "anonymous")
	now := time.Now()
	acknowledged := 0

	for _, id := range req.AlertIDs {
		alert, err := h.storage.GetAlert(c.Request().Context(), id)
		if err != nil {
			continue // Skip alerts that don't exist
		}

		if alert.Acknowledged {
			continue // Skip already acknowledged alerts
		}

		alert.Acknowledged = true
		alert.AckedAt = &now
		alert.AckedBy = userID

		if err := h.storage.UpdateAlert(c.Request().Context(), alert); err == nil {
			acknowledged++
		}
	}

	return c.JSON(http.StatusOK, map[string]int{"acknowledged": acknowledged})
}

// GetStats handles GET /api/v1/companion/stats
func (h *Handler) GetStats(c echo.Context) error {
	ctx := c.Request().Context()

	// Try cache first
	cacheKey := "companion_stats"
	if cached, ok := h.statsCache.Get(cacheKey); ok {
		return c.JSON(http.StatusOK, cached)
	}

	// Use singleflight to deduplicate concurrent requests
	result, err, _ := h.sfGroup.Do("get_stats", func() (interface{}, error) {
		stats, err := h.manager.GetStats(ctx)
		if err != nil {
			return nil, err
		}

		// Cache the result
		h.statsCache.Put(cacheKey, stats)
		return stats, nil
	})

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, result)
}

// Export handles GET /api/v1/companion/export
func (h *Handler) Export(c echo.Context) error {
	format := c.QueryParam("format")
	if format == "" {
		format = "json"
	}

	opts := &ExportOptions{
		Format: format,
	}

	// Parse session IDs
	if sessionIDs := c.QueryParams()["session_id"]; len(sessionIDs) > 0 {
		opts.SessionIDs = sessionIDs
	}

	// Parse date range
	if from := c.QueryParam("from"); from != "" {
		t, err := time.Parse(time.RFC3339, from)
		if err == nil {
			opts.From = &t
		}
	}
	if to := c.QueryParam("to"); to != "" {
		t, err := time.Parse(time.RFC3339, to)
		if err == nil {
			opts.To = &t
		}
	}

	// Get sessions
	listOpts := &ListOptions{
		From: opts.From,
		To:   opts.To,
	}
	sessions, _, err := h.manager.ListSessions(c.Request().Context(), listOpts)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Filter by session IDs if specified
	if len(opts.SessionIDs) > 0 {
		sessionIDSet := make(map[string]bool)
		for _, id := range opts.SessionIDs {
			sessionIDSet[id] = true
		}
		var filtered []*Session
		for _, s := range sessions {
			if sessionIDSet[s.ID] {
				filtered = append(filtered, s)
			}
		}
		sessions = filtered
	}

	switch format {
	case "csv":
		return h.exportCSV(c, sessions)
	default:
		return h.exportJSON(c, sessions)
	}
}

func (h *Handler) exportJSON(c echo.Context, sessions []*Session) error {
	// Collect all events for each session
	type exportData struct {
		Sessions []*Session                 `json:"sessions"`
		Events   map[string][]*SessionEvent `json:"events"`
	}

	data := exportData{
		Sessions: sessions,
		Events:   make(map[string][]*SessionEvent),
	}

	for _, session := range sessions {
		events, _, err := h.manager.GetSessionEvents(c.Request().Context(), session.ID, nil)
		if err == nil {
			data.Events[session.ID] = events
		}
	}

	c.Response().Header().Set("Content-Disposition", "attachment; filename=companion-export.json")
	c.Response().Header().Set("Content-Type", "application/json")

	encoder := json.NewEncoder(c.Response().Writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func (h *Handler) exportCSV(c echo.Context, sessions []*Session) error {
	c.Response().Header().Set("Content-Disposition", "attachment; filename=companion-export.csv")
	c.Response().Header().Set("Content-Type", "text/csv")

	writer := csv.NewWriter(c.Response().Writer)
	defer writer.Flush()

	// Write header
	writer.Write([]string{
		"session_id",
		"platform",
		"user_id",
		"status",
		"started_at",
		"ended_at",
		"duration_ms",
		"event_count",
		"threat_level",
		"threat_score",
	})

	// Write sessions
	for _, session := range sessions {
		endedAt := ""
		if session.EndedAt != nil {
			endedAt = session.EndedAt.Format(time.RFC3339)
		}

		writer.Write([]string{
			session.ID,
			string(session.Platform),
			session.UserID,
			string(session.Status),
			session.StartedAt.Format(time.RFC3339),
			endedAt,
			strconv.FormatInt(session.Duration.Milliseconds(), 10),
			strconv.Itoa(session.EventCount),
			string(session.ThreatLevel),
			strconv.Itoa(session.ThreatScore),
		})
	}

	return nil
}

func (h *Handler) parseListOptions(c echo.Context) *ListOptions {
	opts := &ListOptions{
		Offset: 0,
		Limit:  50,
	}

	if offset := c.QueryParam("offset"); offset != "" {
		if v, err := strconv.Atoi(offset); err == nil && v >= 0 {
			opts.Offset = v
		}
	}

	if limit := c.QueryParam("limit"); limit != "" {
		if v, err := strconv.Atoi(limit); err == nil && v > 0 && v <= 1000 {
			opts.Limit = v
		}
	}

	if sort := c.QueryParam("sort"); sort != "" {
		opts.Sort = sort
	}

	if order := c.QueryParam("order"); order != "" {
		opts.Order = order
	}

	if platform := c.QueryParam("platform"); platform != "" {
		opts.Platform = Platform(platform)
	}

	if userID := c.QueryParam("user_id"); userID != "" {
		opts.UserID = userID
	}

	if status := c.QueryParam("status"); status != "" {
		opts.Status = SessionStatus(status)
	}

	if from := c.QueryParam("from"); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			opts.From = &t
		}
	}

	if to := c.QueryParam("to"); to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			opts.To = &t
		}
	}

	return opts
}

// SettingsResponse represents the companion settings response.
type SettingsResponse struct {
	Retention   RetentionConfig `json:"retention"`
	StorageInfo StorageInfo     `json:"storage_info"`
}

// StorageInfo contains storage statistics.
type StorageInfo struct {
	SessionCount int `json:"session_count"`
	AlertCount   int `json:"alert_count"`
	EventCount   int `json:"event_count"`
}

// GetSettings handles GET /api/v1/companion/settings
func (h *Handler) GetSettings(c echo.Context) error {
	config := h.manager.GetRetentionConfig()

	// Get storage stats
	sessions, sessionTotal, _ := h.storage.ListSessions(c.Request().Context(), nil)
	_, alertTotal, _ := h.storage.ListAlerts(c.Request().Context(), nil)

	eventCount := 0
	for _, s := range sessions {
		eventCount += s.EventCount
	}

	return c.JSON(http.StatusOK, SettingsResponse{
		Retention: *config,
		StorageInfo: StorageInfo{
			SessionCount: sessionTotal,
			AlertCount:   alertTotal,
			EventCount:   eventCount,
		},
	})
}

// UpdateSettingsRequest represents the update settings request.
type UpdateSettingsRequest struct {
	Retention RetentionConfig `json:"retention"`
}

// UpdateSettings handles PUT /api/v1/companion/settings
func (h *Handler) UpdateSettings(c echo.Context) error {
	var req UpdateSettingsRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	// Validate retention values
	if req.Retention.EventsDays < 1 {
		req.Retention.EventsDays = 1
	}
	if req.Retention.SessionsDays < 1 {
		req.Retention.SessionsDays = 1
	}
	if req.Retention.AlertsDays < 1 {
		req.Retention.AlertsDays = 1
	}

	// Cap maximum values
	if req.Retention.EventsDays > 365 {
		req.Retention.EventsDays = 365
	}
	if req.Retention.SessionsDays > 365 {
		req.Retention.SessionsDays = 365
	}
	if req.Retention.AlertsDays > 365 {
		req.Retention.AlertsDays = 365
	}

	h.manager.UpdateRetentionConfig(&req.Retention)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":   "settings updated",
		"retention": req.Retention,
	})
}

// CleanupResponse represents the cleanup response.
type CleanupResponse struct {
	Message string `json:"message"`
	Success bool   `json:"success"`
}

// TriggerCleanup handles POST /api/v1/companion/cleanup
func (h *Handler) TriggerCleanup(c echo.Context) error {
	err := h.manager.CleanupExpired(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, CleanupResponse{
			Message: "cleanup failed: " + err.Error(),
			Success: false,
		})
	}

	return c.JSON(http.StatusOK, CleanupResponse{
		Message: "cleanup completed successfully",
		Success: true,
	})
}
