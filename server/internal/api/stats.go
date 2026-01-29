package api

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/stats"
)

// StatsHandler handles statistics API requests.
type StatsHandler struct {
	collector      *stats.StatisticsCollector
	consentManager *stats.ConsentManager
}

// NewStatsHandler creates a new stats handler.
func NewStatsHandler(collector *stats.StatisticsCollector, consentManager *stats.ConsentManager) *StatsHandler {
	return &StatsHandler{
		collector:      collector,
		consentManager: consentManager,
	}
}

// RegisterRoutes registers statistics routes.
func (h *StatsHandler) RegisterRoutes(e *echo.Echo) {
	g := e.Group("/api/v1/stats")
	g.GET("", h.GetStats)
	g.GET("/recent", h.GetRecentEvents)
	g.GET("/export", h.ExportStats)
	g.DELETE("", h.ClearStats)

	// Consent routes
	g.GET("/consent", h.GetConsentStatus)
	g.POST("/consent", h.SetConsentStatus)
	g.GET("/consent/info", h.GetConsentInfo)
}

// GetStatsRequest represents the get stats request.
type GetStatsRequest struct {
	Period string `query:"period"` // hour, day, week, month, all
}

// GetStats returns usage statistics.
func (h *StatsHandler) GetStats(c echo.Context) error {
	// Check if statistics collection is enabled
	if !h.collector.IsEnabled() {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"enabled": false,
			"message": "Statistics collection is disabled. Enable it in settings to view usage data.",
		})
	}

	period := c.QueryParam("period")
	if period == "" {
		period = "day"
	}

	usageStats, err := h.collector.GetStats(period)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"enabled": true,
		"period":  period,
		"stats":   usageStats,
	})
}

// GetRecentEvents returns recent API call events.
func (h *StatsHandler) GetRecentEvents(c echo.Context) error {
	if !h.collector.IsEnabled() {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"enabled": false,
			"events":  []stats.APICallEvent{},
		})
	}

	limit := 50 // Default limit
	events := h.collector.GetRecentEvents(limit)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"enabled": true,
		"count":   len(events),
		"events":  events,
	})
}

// ExportStats exports statistics data.
func (h *StatsHandler) ExportStats(c echo.Context) error {
	if !h.collector.IsEnabled() {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Statistics collection is disabled",
		})
	}

	format := c.QueryParam("format")
	if format == "" {
		format = "json"
	}

	data, err := h.collector.Export(format)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Set appropriate content type
	contentType := "application/json"
	filename := "statistics.json"

	c.Response().Header().Set("Content-Disposition", "attachment; filename="+filename)
	return c.Blob(http.StatusOK, contentType, data)
}

// ClearStats clears all statistics.
func (h *StatsHandler) ClearStats(c echo.Context) error {
	if err := h.collector.Clear(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Statistics cleared",
	})
}

// GetConsentStatus returns the current consent status.
func (h *StatsHandler) GetConsentStatus(c echo.Context) error {
	status := h.consentManager.GetStatus()
	return c.JSON(http.StatusOK, status)
}

// SetConsentRequest represents the set consent request.
type SetConsentRequest struct {
	Consented bool `json:"consented"`
	ClearData bool `json:"clear_data,omitempty"` // Only used when revoking consent
}

// SetConsentStatus sets the consent status.
func (h *StatsHandler) SetConsentStatus(c echo.Context) error {
	var req SetConsentRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	if req.Consented {
		if err := h.consentManager.SetConsent(true); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
		}
	} else {
		if err := h.consentManager.RevokeConsent(req.ClearData); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"status":  h.consentManager.GetStatus(),
	})
}

// GetConsentInfo returns information about what data is collected.
func (h *StatsHandler) GetConsentInfo(c echo.Context) error {
	return c.JSON(http.StatusOK, stats.GetConsentInfo())
}
