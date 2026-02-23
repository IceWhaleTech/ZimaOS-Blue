package heartbeat

import (
	"net/http"
	"os"
	"path/filepath"
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
	g.GET("/heartbeat/content", h.GetContent)
	g.PUT("/heartbeat/content", h.PutContent)
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

	// Early check: skip if HEARTBEAT.md is effectively empty
	if data, err := os.ReadFile(h.heartbeatFilePath()); err == nil && IsEffectivelyEmpty(string(data)) {
		return c.JSON(http.StatusOK, map[string]string{
			"status": "skipped",
			"reason": "empty-heartbeat-file",
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
		// Wake the runner so it can transition between enabled/disabled states
		h.runner.RequestNow("config-toggle")
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status": "updated",
	})
}

// heartbeatFilePath returns the resolved path to HEARTBEAT.md.
func (h *Handler) heartbeatFilePath() string {
	if h.runner == nil {
		return HeartbeatFilename
	}
	h.runner.mu.RLock()
	workDir := h.runner.cfg.WorkspaceDir
	h.runner.mu.RUnlock()
	if workDir == "" {
		workDir = "."
	}
	return filepath.Join(workDir, HeartbeatFilename)
}

// GetContent returns the content of HEARTBEAT.md.
func (h *Handler) GetContent(c echo.Context) error {
	path := h.heartbeatFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c.JSON(http.StatusOK, map[string]string{
				"content": "",
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}
	return c.JSON(http.StatusOK, map[string]string{
		"content": string(data),
	})
}

// PutContent writes content to HEARTBEAT.md.
func (h *Handler) PutContent(c echo.Context) error {
	var req struct {
		Content string `json:"content"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
	}

	path := h.heartbeatFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}
	if err := os.WriteFile(path, []byte(req.Content), 0644); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}
	return c.JSON(http.StatusOK, map[string]string{
		"status": "saved",
	})
}
