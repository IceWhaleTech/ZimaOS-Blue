package agentsessions

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(g *echo.Group) {
	if g == nil || h == nil || h.service == nil {
		return
	}
	h.RegisterProfileRoutes(g)
	h.RegisterSessionRoutes(g)
}

func (h *Handler) RegisterProfileRoutes(g *echo.Group) {
	if g == nil || h == nil || h.service == nil {
		return
	}
	g.GET("/agent-sessions/profiles", h.ListProfiles)
	g.POST("/agent-sessions/profiles", h.SaveProfile)
	g.POST("/agent-sessions/profiles/verify", h.VerifyProfile)
	g.POST("/agent-sessions/profiles/:id/health", h.HealthProfile)
	g.DELETE("/agent-sessions/profiles/:id", h.DeleteProfile)
}

func (h *Handler) RegisterSessionRoutes(g *echo.Group) {
	if g == nil || h == nil || h.service == nil {
		return
	}
	g.GET("/agent-sessions/sessions", h.ListSessions)
	g.POST("/agent-sessions/sessions", h.CreateSession)
	g.GET("/agent-sessions/sessions/:id", h.GetSession)
	g.GET("/agent-sessions/sessions/:id/history", h.GetHistory)
	g.POST("/agent-sessions/sessions/:id/messages", h.SendMessage)
	g.POST("/agent-sessions/sessions/:id/cancel", h.CancelSession)
	g.POST("/agent-sessions/sessions/:id/close", h.CloseSession)
	g.GET("/agent-sessions/sessions/:id/events/stream", h.StreamEvents)
}

func (h *Handler) ListProfiles(c echo.Context) error {
	profiles, err := h.service.ListProfiles()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"profiles": profiles})
}

func (h *Handler) SaveProfile(c echo.Context) error {
	var profile AgentProfile
	if err := c.Bind(&profile); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid profile body"})
	}
	if err := h.service.SaveProfile(&profile); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, profile)
}

func (h *Handler) VerifyProfile(c echo.Context) error {
	var req struct {
		ID      string       `json:"id"`
		Profile AgentProfile `json:"profile"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid verify request"})
	}
	var candidate *AgentProfile
	if strings.TrimSpace(req.Profile.Name) != "" || req.Profile.Protocol != "" {
		candidate = &req.Profile
	}
	result, err := h.service.VerifyProfile(c.Request().Context(), req.ID, candidate)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) HealthProfile(c echo.Context) error {
	result, err := h.service.HealthProfile(c.Request().Context(), c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) DeleteProfile(c echo.Context) error {
	err := h.service.DeleteProfile(c.Param("id"))
	if err != nil {
		status := http.StatusBadRequest
		switch {
		case errors.Is(err, ErrProfileNotFound):
			status = http.StatusNotFound
		case errors.Is(err, ErrBuiltinProfileDeleteDenied), errors.Is(err, ErrProfileInUse):
			status = http.StatusConflict
		}
		return c.JSON(status, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]bool{"deleted": true})
}

func (h *Handler) ListSessions(c echo.Context) error {
	limit, _ := strconv.Atoi(strings.TrimSpace(c.QueryParam("limit")))
	offset, _ := strconv.Atoi(strings.TrimSpace(c.QueryParam("offset")))
	var protocol ProtocolKind
	if runtime := strings.TrimSpace(c.QueryParam("runtime")); runtime != "" {
		parsed, err := ParseProtocolKind(runtime)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid runtime filter"})
		}
		protocol = parsed
	}
	userID := userIDFromContext(c)
	sessions, err := h.service.ListSessions(limit, offset, userID, protocol)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"sessions": sessions})
}

func (h *Handler) CreateSession(c echo.Context) error {
	var params CreateSessionParams
	if err := c.Bind(&params); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid session body"})
	}
	if params.UserID == "" {
		params.UserID = userIDFromContext(c)
	}
	detail, err := h.service.CreateSession(c.Request().Context(), params)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, detail)
}

func (h *Handler) GetSession(c echo.Context) error {
	detail, err := h.service.GetSessionDetail(c.Param("id"), 50)
	if err != nil {
		status := http.StatusInternalServerError
		if err == ErrSessionNotFound {
			status = http.StatusNotFound
		}
		return c.JSON(status, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, detail)
}

func (h *Handler) GetHistory(c echo.Context) error {
	limit, _ := strconv.Atoi(strings.TrimSpace(c.QueryParam("limit")))
	history, err := h.service.GetSessionHistory(c.Param("id"), limit)
	if err != nil {
		status := http.StatusInternalServerError
		if err == ErrSessionNotFound {
			status = http.StatusNotFound
		}
		return c.JSON(status, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"history": history})
}

func (h *Handler) SendMessage(c echo.Context) error {
	var req struct {
		Message string `json:"message"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid message body"})
	}
	run, err := h.service.SendMessage(c.Request().Context(), SendMessageParams{
		SessionID: c.Param("id"),
		Message:   req.Message,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, run)
}

func (h *Handler) CancelSession(c echo.Context) error {
	if err := h.service.CancelSession(c.Request().Context(), c.Param("id")); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]bool{"cancelled": true})
}

func (h *Handler) CloseSession(c echo.Context) error {
	if err := h.service.CloseSession(c.Request().Context(), c.Param("id")); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]bool{"closed": true})
}

func (h *Handler) StreamEvents(c echo.Context) error {
	sessionID := strings.TrimSpace(c.Param("id"))
	afterID, _ := strconv.ParseInt(strings.TrimSpace(c.QueryParam("after_id")), 10, 64)

	w := c.Response()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.Writer.(http.Flusher)
	if !ok {
		return echo.NewHTTPError(http.StatusInternalServerError, "streaming not supported")
	}

	existing, err := h.service.ListEvents(sessionID, 1000, afterID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	for _, event := range existing {
		if err := writeSSEEvent(w, "agent-session-event", event); err != nil {
			return nil
		}
		afterID = event.ID
	}
	flusher.Flush()

	updates, cancel := h.service.SubscribeSession(sessionID)
	defer cancel()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request().Context().Done():
			return nil
		case event, ok := <-updates:
			if !ok {
				return nil
			}
			if event.ID <= afterID {
				continue
			}
			if err := writeSSEEvent(w, "agent-session-event", event); err != nil {
				return nil
			}
			flusher.Flush()
			afterID = event.ID
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return nil
			}
			flusher.Flush()
		}
	}
}

func writeSSEEvent(w *echo.Response, eventName string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventName, data)
	return err
}

func userIDFromContext(c echo.Context) string {
	if claims := auth.GetUserFromContext(c); claims != nil {
		if userID := strings.TrimSpace(claims.UserID); userID != "" {
			return userID
		}
	}
	if raw := c.Get("user_id"); raw != nil {
		if userID, ok := raw.(string); ok {
			return strings.TrimSpace(userID)
		}
	}
	return ""
}
