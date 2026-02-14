package heartbeat

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// Handler provides HTTP endpoints for heartbeat status.
type Handler struct {
	runner *Runner
}

// NewHandler creates a new heartbeat HTTP handler.
func NewHandler(runner *Runner) *Handler {
	return &Handler{runner: runner}
}

// RegisterRoutes registers heartbeat routes on the given group.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.GET("/heartbeat/status", h.Status)
	g.POST("/heartbeat/trigger", h.Trigger)
	g.PATCH("/heartbeat/config", h.UpdateConfig)
}

// Status returns the current heartbeat state.
func (h *Handler) Status(c echo.Context) error {
	if h.runner == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"enabled": false,
		})
	}

	h.runner.mu.RLock()
	cfg := h.runner.cfg
	h.runner.mu.RUnlock()

	resp := map[string]interface{}{
		"enabled":  cfg.Enabled,
		"interval": cfg.Interval.String(),
	}

	if cfg.Enabled {
		resp["next_due"] = h.runner.NextDue().Format(time.RFC3339)
		if evt := h.runner.LastEvent(); evt != nil {
			resp["last_event"] = evt
		}
	}

	return c.JSON(http.StatusOK, resp)
}

// Trigger manually triggers a heartbeat run.
func (h *Handler) Trigger(c echo.Context) error {
	if h.runner == nil || !h.runner.cfg.Enabled {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "heartbeat is not enabled",
		})
	}

	h.runner.RequestNow("manual")
	return c.JSON(http.StatusOK, map[string]string{
		"status": "triggered",
	})
}

// UpdateConfig toggles heartbeat enabled/disabled at runtime.
func (h *Handler) UpdateConfig(c echo.Context) error {
	if h.runner == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "heartbeat runner not initialized",
		})
	}

	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
	}

	if req.Enabled != nil {
		h.runner.mu.Lock()
		h.runner.cfg.Enabled = *req.Enabled
		h.runner.mu.Unlock()
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status": "updated",
	})
}
