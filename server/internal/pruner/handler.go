package pruner

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// APIHandler provides HTTP handlers for pruner management.
type APIHandler struct {
	middleware *Middleware
	config     *Config
}

// NewAPIHandler creates a new pruner API handler.
func NewAPIHandler(mw *Middleware, cfg *Config) *APIHandler {
	return &APIHandler{
		middleware: mw,
		config:    cfg,
	}
}

// RegisterRoutes registers pruner API routes.
func (h *APIHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/stats", h.GetStats)
	g.GET("/config", h.GetConfig)
	g.PUT("/config", h.UpdateConfig)
	g.GET("/health", h.HealthCheck)
}

// GetStats returns pruning statistics.
// GET /api/v1/proxy/pruner/stats
func (h *APIHandler) GetStats(c echo.Context) error {
	if h.middleware == nil || h.middleware.stats == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"enabled": false,
		})
	}
	snap := h.middleware.stats.Snapshot()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"enabled": h.config.Enabled,
		"stats":   snap,
	})
}

// GetConfig returns the current pruner configuration.
// GET /api/v1/proxy/pruner/config
func (h *APIHandler) GetConfig(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"enabled":    h.config.Enabled,
		"backend":    h.config.Backend,
		"threshold":  h.config.Threshold,
		"min_lines":  h.config.MinLines,
		"timeout_ms": h.config.TimeoutMs,
	})
}

// HealthCheck checks if the pruner backend is available.
// GET /api/v1/proxy/pruner/health
func (h *APIHandler) HealthCheck(c echo.Context) error {
	if h.middleware == nil || h.middleware.backend == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status": "disabled",
		})
	}
	if err := h.middleware.backend.Health(c.Request().Context()); err != nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status": "unhealthy",
			"error":  err.Error(),
		})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "healthy",
	})
}

// UpdateConfig updates the pruner configuration (partial update).
// PUT /api/v1/proxy/pruner/config
func (h *APIHandler) UpdateConfig(c echo.Context) error {
	if h.config == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "pruner not initialized",
		})
	}

	var update struct {
		Enabled   *bool    `json:"enabled"`
		Threshold *float64 `json:"threshold"`
	}

	if err := c.Bind(&update); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	if update.Enabled != nil {
		h.config.Enabled = *update.Enabled
		if h.middleware != nil {
			h.middleware.SetEnabled(*update.Enabled)
		}
	}
	if update.Threshold != nil {
		h.config.Threshold = *update.Threshold
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"config": map[string]interface{}{
			"enabled":    h.config.Enabled,
			"backend":    h.config.Backend,
			"threshold":  h.config.Threshold,
			"min_lines":  h.config.MinLines,
			"timeout_ms": h.config.TimeoutMs,
		},
	})
}
