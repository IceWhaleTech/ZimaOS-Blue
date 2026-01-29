// Package api provides HTTP API handlers for the ZimaOS-Echo server.
package api

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/setup"
)

// FirstRunHandler handles first-run wizard API requests (v0.10.3).
type FirstRunHandler struct {
	manager *setup.FirstRunManager
}

// NewFirstRunHandler creates a new first-run handler.
func NewFirstRunHandler(manager *setup.FirstRunManager) *FirstRunHandler {
	return &FirstRunHandler{
		manager: manager,
	}
}

// RegisterRoutes registers first-run routes.
func (h *FirstRunHandler) RegisterRoutes(e *echo.Echo) {
	g := e.Group("/api/v1/first-run")
	g.GET("/status", h.GetStatus)
	g.POST("/complete", h.MarkComplete)
	g.POST("/skip-cli", h.SkipCLI)
	g.POST("/reset", h.Reset)
}

// FirstRunStatusResponse represents the first-run status response.
type FirstRunStatusResponse struct {
	IsFirstRun bool                   `json:"is_first_run"`
	Status     *setup.FirstRunStatus `json:"status"`
}

// GetStatus returns the current first-run status.
func (h *FirstRunHandler) GetStatus(c echo.Context) error {
	return c.JSON(http.StatusOK, &FirstRunStatusResponse{
		IsFirstRun: h.manager.IsFirstRun(),
		Status:     h.manager.GetStatus(),
	})
}

// MarkCompleteRequest represents the mark complete request.
type MarkCompleteRequest struct {
	StatisticsOptIn bool `json:"statistics_opt_in"`
}

// MarkComplete marks the first-run as complete.
func (h *FirstRunHandler) MarkComplete(c echo.Context) error {
	var req MarkCompleteRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	// Set statistics opt-in
	if err := h.manager.SetStatisticsOptIn(req.StatisticsOptIn); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
	}

	// Mark complete
	if err := h.manager.MarkComplete(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "First-run completed",
	})
}

// SkipCLI marks CLI download as skipped.
func (h *FirstRunHandler) SkipCLI(c echo.Context) error {
	if err := h.manager.SetCLISkipped(true); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "CLI download skipped",
	})
}

// Reset resets the first-run status.
func (h *FirstRunHandler) Reset(c echo.Context) error {
	if err := h.manager.Reset(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "First-run reset",
	})
}
