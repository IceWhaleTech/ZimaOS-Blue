package proxy

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// RestrictionsHandler handles provider restriction management endpoints.
type RestrictionsHandler struct {
	providerMemory *ProviderMemory
}

// NewRestrictionsHandler creates a new restrictions handler.
func NewRestrictionsHandler(pm *ProviderMemory) *RestrictionsHandler {
	return &RestrictionsHandler{
		providerMemory: pm,
	}
}

// RegisterRoutes registers restriction management routes.
func (h *RestrictionsHandler) RegisterRoutes(g *echo.Group) {
	restrictions := g.Group("/restrictions")
	restrictions.GET("/providers/:provider", h.GetProviderRestrictions)
	restrictions.POST("/providers/:provider/clear-throttle", h.ClearProviderThrottle)
	restrictions.POST("/providers/:provider/clear-model/:model", h.ClearModelBlacklist)
}

// GetProviderRestrictions returns current restrictions for a provider.
// GET /api/v1/restrictions/providers/{provider}
func (h *RestrictionsHandler) GetProviderRestrictions(c echo.Context) error {
	provider := c.Param("provider")
	baseURL := c.QueryParam("base_url")

	if provider == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "provider is required",
		})
	}

	restrictions := h.providerMemory.GetRestrictions(provider, baseURL)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"provider":      provider,
		"restrictions":  restrictions,
	})
}

// ClearProviderThrottle clears throttle restriction for a provider.
// POST /api/v1/restrictions/providers/{provider}/clear-throttle
func (h *RestrictionsHandler) ClearProviderThrottle(c echo.Context) error {
	provider := c.Param("provider")
	baseURL := c.QueryParam("base_url")

	if provider == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "provider is required",
		})
	}

	h.providerMemory.ClearThrottle(provider, baseURL)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "throttle restriction cleared, provider can be retried",
	})
}

// ClearModelBlacklist clears blacklist for a specific model on a provider.
// POST /api/v1/restrictions/providers/{provider}/clear-model/{model}
func (h *RestrictionsHandler) ClearModelBlacklist(c echo.Context) error {
	provider := c.Param("provider")
	model := c.Param("model")
	baseURL := c.QueryParam("base_url")

	if provider == "" || model == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "provider and model are required",
		})
	}

	h.providerMemory.ClearModelBlacklist(provider, baseURL, model)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "model blacklist cleared, model can be retried",
	})
}
