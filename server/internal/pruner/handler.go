package pruner

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
)

// APIHandler provides HTTP handlers for pruner management.
type APIHandler struct {
	middleware    *Middleware
	config       *Config
	modelManager *PrunerModelManager
}

// NewAPIHandler creates a new pruner API handler.
func NewAPIHandler(mw *Middleware, cfg *Config, mm *PrunerModelManager) *APIHandler {
	return &APIHandler{
		middleware:    mw,
		config:       cfg,
		modelManager: mm,
	}
}

// RegisterRoutes registers pruner API routes.
func (h *APIHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/stats", h.GetStats)
	g.GET("/config", h.GetConfig)
	g.PUT("/config", h.UpdateConfig)
	g.GET("/health", h.HealthCheck)
	g.POST("/model/download", h.StartModelDownload)
	g.POST("/model/cancel", h.CancelModelDownload)
	g.GET("/model/status", h.GetModelStatus)
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

// StartModelDownload starts downloading the ONNX pruner model.
// POST /api/v1/proxy/pruner/model/download
func (h *APIHandler) StartModelDownload(c echo.Context) error {
	if h.modelManager == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "model manager not initialized",
		})
	}

	go func() {
		_ = h.modelManager.Download(context.Background())
	}()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "download started",
	})
}

// CancelModelDownload cancels the current model download.
// POST /api/v1/proxy/pruner/model/cancel
func (h *APIHandler) CancelModelDownload(c echo.Context) error {
	if h.modelManager == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "model manager not initialized",
		})
	}
	h.modelManager.CancelDownload()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// GetModelStatus returns the model download status.
// GET /api/v1/proxy/pruner/model/status
func (h *APIHandler) GetModelStatus(c echo.Context) error {
	if h.modelManager == nil {
		return c.JSON(http.StatusOK, PrunerModelStatus{Ready: false})
	}
	return c.JSON(http.StatusOK, h.modelManager.GetStatus())
}
