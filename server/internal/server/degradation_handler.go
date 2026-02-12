package server

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/resilience"
)

// DegradationHandler handles degradation-related API endpoints
type DegradationHandler struct {
	manager *resilience.DegradationManager
}

// NewDegradationHandler creates a new degradation handler
func NewDegradationHandler(manager *resilience.DegradationManager) *DegradationHandler {
	return &DegradationHandler{
		manager: manager,
	}
}

// RegisterRoutes registers degradation routes
func (h *DegradationHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/degradation/status", h.GetStatus)
	g.POST("/degradation/level", h.SetLevel)
	g.DELETE("/degradation/override", h.ClearOverride)
	g.POST("/degradation/reset", h.ResetMetrics)
}

// DegradationStatusResponse represents the degradation status response
type DegradationStatusResponse struct {
	Level           string  `json:"level"`
	ManualOverride  bool    `json:"manual_override"`
	ErrorRate       float64 `json:"error_rate"`
	TotalRequests   int64   `json:"total_requests"`
	FailedRequests  int64   `json:"failed_requests"`
	LastLevelChange string  `json:"last_level_change,omitempty"`
}

// GetStatus handles GET /api/v1/degradation/status
// @Summary Get degradation status
// @Description Returns the current degradation status and metrics
// @Tags degradation
// @Produce json
// @Success 200 {object} DegradationStatusResponse
// @Router /api/v1/degradation/status [get]
func (h *DegradationHandler) GetStatus(c echo.Context) error {
	if h.manager == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Degradation manager not configured",
		})
	}

	stats := h.manager.Stats()

	response := DegradationStatusResponse{
		Level:          stats["level"].(string),
		ManualOverride: stats["manual_override"].(bool),
		ErrorRate:      stats["error_rate"].(float64),
		TotalRequests:  stats["total_requests"].(int64),
		FailedRequests: stats["failed_requests"].(int64),
	}

	if lastChange, ok := stats["last_level_change"]; ok {
		response.LastLevelChange = lastChange.(string)
	}

	return c.JSON(http.StatusOK, response)
}

// SetLevelRequest represents a request to set degradation level
type SetLevelRequest struct {
	Level string `json:"level" validate:"required,oneof=normal partial minimal emergency"`
}

// SetLevelResponse represents the response after setting degradation level
type SetLevelResponse struct {
	Success      bool   `json:"success"`
	PreviousLevel string `json:"previous_level"`
	CurrentLevel  string `json:"current_level"`
	Message      string `json:"message"`
}

// SetLevel handles POST /api/v1/degradation/level
// @Summary Set degradation level
// @Description Manually sets the degradation level
// @Tags degradation
// @Accept json
// @Produce json
// @Param request body SetLevelRequest true "Level to set"
// @Success 200 {object} SetLevelResponse
// @Failure 400 {object} map[string]string
// @Router /api/v1/degradation/level [post]
func (h *DegradationHandler) SetLevel(c echo.Context) error {
	if h.manager == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Degradation manager not configured",
		})
	}

	var req SetLevelRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
	}

	// Parse level
	var level resilience.DegradationLevel
	switch strings.ToLower(req.Level) {
	case "normal":
		level = resilience.LevelNormal
	case "partial":
		level = resilience.LevelPartial
	case "minimal":
		level = resilience.LevelMinimal
	case "emergency":
		level = resilience.LevelEmergency
	default:
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid level. Must be one of: normal, partial, minimal, emergency",
		})
	}

	previousLevel := h.manager.Level().String()
	h.manager.SetLevel(level)

	return c.JSON(http.StatusOK, SetLevelResponse{
		Success:       true,
		PreviousLevel: previousLevel,
		CurrentLevel:  level.String(),
		Message:       "Degradation level updated successfully",
	})
}

// ClearOverrideResponse represents the response after clearing override
type ClearOverrideResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ClearOverride handles DELETE /api/v1/degradation/override
// @Summary Clear manual override
// @Description Clears manual override and returns to automatic degradation management
// @Tags degradation
// @Produce json
// @Success 200 {object} ClearOverrideResponse
// @Router /api/v1/degradation/override [delete]
func (h *DegradationHandler) ClearOverride(c echo.Context) error {
	if h.manager == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Degradation manager not configured",
		})
	}

	h.manager.ClearOverride()

	return c.JSON(http.StatusOK, ClearOverrideResponse{
		Success: true,
		Message: "Manual override cleared. Automatic degradation management resumed.",
	})
}

// ResetMetricsResponse represents the response after resetting metrics
type ResetMetricsResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ResetMetrics handles POST /api/v1/degradation/reset
// @Summary Reset degradation metrics
// @Description Resets the error rate metrics
// @Tags degradation
// @Produce json
// @Success 200 {object} ResetMetricsResponse
// @Router /api/v1/degradation/reset [post]
func (h *DegradationHandler) ResetMetrics(c echo.Context) error {
	if h.manager == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Degradation manager not configured",
		})
	}

	h.manager.ResetMetrics()

	return c.JSON(http.StatusOK, ResetMetricsResponse{
		Success: true,
		Message: "Degradation metrics reset successfully",
	})
}
