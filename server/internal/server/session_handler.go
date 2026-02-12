package server

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/session"
)

// SessionHandler handles session-related endpoints.
type SessionHandler struct {
	manager *session.SessionManager
}

// NewSessionHandler creates a new SessionHandler.
func NewSessionHandler(manager *session.SessionManager) *SessionHandler {
	return &SessionHandler{manager: manager}
}

// RegisterRoutes registers session routes.
func (h *SessionHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/sessions", h.List)
	g.GET("/sessions/:id", h.Get)
	g.POST("/sessions/:id/compact", h.Compact)
	g.POST("/sessions/:id/reset", h.Reset)
	g.POST("/sessions/:id/archive", h.Archive)
	g.DELETE("/sessions/:id", h.Delete)
	g.GET("/sessions/stats", h.Stats)
}

// ListSessionsResponse represents a list sessions response.
type ListSessionsResponse struct {
	Sessions []session.SessionInfo `json:"sessions"`
	Total    int                   `json:"total"`
}

// List lists sessions.
func (h *SessionHandler) List(c echo.Context) error {
	if h.manager == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "session manager not configured")
	}

	// Parse query parameters
	agentID := c.QueryParam("agent_id")
	channelID := c.QueryParam("channel_id")
	peerID := c.QueryParam("peer_id")
	limitStr := c.QueryParam("limit")
	offsetStr := c.QueryParam("offset")

	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	filter := session.SessionFilter{
		AgentID:   agentID,
		ChannelID: channelID,
		PeerID:    peerID,
		Limit:     limit,
		Offset:    offset,
	}

	sessions, err := h.manager.List(filter)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	infos := make([]session.SessionInfo, len(sessions))
	for i, s := range sessions {
		infos[i] = s.Info()
	}

	return c.JSON(http.StatusOK, ListSessionsResponse{
		Sessions: infos,
		Total:    len(infos),
	})
}

// Get retrieves a session by ID.
func (h *SessionHandler) Get(c echo.Context) error {
	if h.manager == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "session manager not configured")
	}

	idStr := c.Param("id")
	if idStr == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is required")
	}

	id, err := session.ParseSessionID(idStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid session ID format")
	}

	sess, ok := h.manager.Get(id)
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "session not found")
	}

	return c.JSON(http.StatusOK, sess.Info())
}

// CompactResponse represents a compact response.
type CompactResponse struct {
	Summary           string `json:"summary,omitempty"`
	PreservedMessages int    `json:"preserved_messages"`
	RemovedMessages   int    `json:"removed_messages"`
	TokensBefore      int    `json:"tokens_before"`
	TokensAfter       int    `json:"tokens_after"`
	TokensSaved       int    `json:"tokens_saved"`
}

// Compact compacts a session.
func (h *SessionHandler) Compact(c echo.Context) error {
	if h.manager == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "session manager not configured")
	}

	idStr := c.Param("id")
	if idStr == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is required")
	}

	id, err := session.ParseSessionID(idStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid session ID format")
	}

	result, err := h.manager.Compact(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if result == nil {
		return echo.NewHTTPError(http.StatusNotFound, "session not found")
	}

	return c.JSON(http.StatusOK, CompactResponse{
		Summary:           result.Summary,
		PreservedMessages: result.PreservedMessages,
		RemovedMessages:   result.RemovedMessages,
		TokensBefore:      result.TokensBefore,
		TokensAfter:       result.TokensAfter,
		TokensSaved:       result.TokensSaved,
	})
}

// Reset resets a session.
func (h *SessionHandler) Reset(c echo.Context) error {
	if h.manager == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "session manager not configured")
	}

	idStr := c.Param("id")
	if idStr == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is required")
	}

	id, err := session.ParseSessionID(idStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid session ID format")
	}

	if err := h.manager.Reset(id); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

// Archive archives a session.
func (h *SessionHandler) Archive(c echo.Context) error {
	if h.manager == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "session manager not configured")
	}

	idStr := c.Param("id")
	if idStr == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is required")
	}

	id, err := session.ParseSessionID(idStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid session ID format")
	}

	if err := h.manager.Archive(id); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

// Delete deletes a session.
func (h *SessionHandler) Delete(c echo.Context) error {
	if h.manager == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "session manager not configured")
	}

	idStr := c.Param("id")
	if idStr == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is required")
	}

	id, err := session.ParseSessionID(idStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid session ID format")
	}

	if err := h.manager.Delete(id); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

// Stats returns session manager statistics.
func (h *SessionHandler) Stats(c echo.Context) error {
	if h.manager == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "session manager not configured")
	}

	stats := h.manager.Stats()
	return c.JSON(http.StatusOK, stats)
}
