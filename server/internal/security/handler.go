package security

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

// Handler handles security-related API endpoints.
type Handler struct {
	detector *ThreatDetector
}

// NewHandler creates a new security handler.
func NewHandler(detector *ThreatDetector) *Handler {
	return &Handler{detector: detector}
}

// GetThreatStats handles GET /api/v1/security/threats/stats
func (h *Handler) GetThreatStats(c echo.Context) error {
	stats := h.detector.GetStats()
	return c.JSON(http.StatusOK, stats)
}

// GetRecentThreats handles GET /api/v1/security/threats
func (h *Handler) GetRecentThreats(c echo.Context) error {
	limitStr := c.QueryParam("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	threats := h.detector.GetRecentThreats(limit)
	return c.JSON(http.StatusOK, threats)
}

// ScanInput handles POST /api/v1/security/scan
// This endpoint allows scanning arbitrary input for threats.
type ScanRequest struct {
	Input  string `json:"input" validate:"required"`
	Source string `json:"source"`
}

type ScanResponse struct {
	Safe    bool          `json:"safe"`
	Threats []ThreatEvent `json:"threats"`
}

func (h *Handler) ScanInput(c echo.Context) error {
	var req ScanRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.Input == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "input is required")
	}

	source := req.Source
	if source == "" {
		source = "api_scan"
	}

	// Get client IP
	ipAddress := c.RealIP()

	// Detect threats
	threats := h.detector.DetectThreats(req.Input, source, ipAddress, "")

	return c.JSON(http.StatusOK, ScanResponse{
		Safe:    len(threats) == 0,
		Threats: threats,
	})
}

// RegisterRoutes registers the security routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.GET("/threats/stats", h.GetThreatStats)
	g.GET("/threats", h.GetRecentThreats)
	g.POST("/scan", h.ScanInput)
}
