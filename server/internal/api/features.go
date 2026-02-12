package api

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/features"
)

// FeaturesHandler handles feature gating API requests.
type FeaturesHandler struct {
	gate *features.FeatureGate
}

// NewFeaturesHandler creates a new features handler.
func NewFeaturesHandler(gate *features.FeatureGate) *FeaturesHandler {
	return &FeaturesHandler{
		gate: gate,
	}
}

// RegisterRoutes registers feature routes.
func (h *FeaturesHandler) RegisterRoutes(e *echo.Echo) {
	g := e.Group("/api/v1/features")
	g.GET("", h.GetAllFeatures)
	g.GET("/status", h.GetStatus)
	g.GET("/:name", h.GetFeature)
	g.GET("/cli-dependent", h.GetCLIDependentFeatures)
	g.GET("/by-category", h.GetFeaturesByCategory)
}

// GetAllFeatures returns all features and their status.
func (h *FeaturesHandler) GetAllFeatures(c echo.Context) error {
	allFeatures := h.gate.GetAllFeatures()

	// Convert to a more JSON-friendly format
	result := make(map[string]interface{})
	for f, info := range allFeatures {
		result[string(f)] = info
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"cli_installed": h.gate.IsCLIInstalled(),
		"features":      result,
	})
}

// GetStatus returns the overall feature status.
func (h *FeaturesHandler) GetStatus(c echo.Context) error {
	status := h.gate.GetStatus()

	// Convert features map to JSON-friendly format
	featuresMap := make(map[string]interface{})
	for f, info := range status.Features {
		featuresMap[string(f)] = info
	}

	disabledReasons := make(map[string]string)
	for f, reason := range status.DisabledReasons {
		disabledReasons[string(f)] = reason
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"cli_installed":    status.CLIInstalled,
		"total_features":   status.TotalFeatures,
		"enabled_count":    status.EnabledCount,
		"disabled_count":   status.DisabledCount,
		"features":         featuresMap,
		"disabled_reasons": disabledReasons,
	})
}

// GetFeature returns information about a specific feature.
func (h *FeaturesHandler) GetFeature(c echo.Context) error {
	name := c.Param("name")
	feature := features.Feature(name)

	info := h.gate.GetFeatureInfo(feature)
	if info == nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error": "Feature not found",
		})
	}

	return c.JSON(http.StatusOK, info)
}

// GetCLIDependentFeatures returns features that require CLI.
func (h *FeaturesHandler) GetCLIDependentFeatures(c echo.Context) error {
	cliFeatures := h.gate.GetCLIDependentFeatures()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"cli_installed": h.gate.IsCLIInstalled(),
		"features":      cliFeatures,
	})
}

// GetFeaturesByCategory returns features grouped by category.
func (h *FeaturesHandler) GetFeaturesByCategory(c echo.Context) error {
	byCategory := h.gate.GetFeaturesByCategory()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"cli_installed": h.gate.IsCLIInstalled(),
		"categories":    byCategory,
	})
}

// FeatureMiddleware creates middleware that checks if a feature is enabled.
func FeatureMiddleware(gate *features.FeatureGate, feature features.Feature) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if err := gate.RequireFeature(feature); err != nil {
				info := gate.GetFeatureInfo(feature)
				response := map[string]interface{}{
					"error":        "Feature not available",
					"feature":      string(feature),
					"requires_cli": false,
				}
				if info != nil {
					response["requires_cli"] = info.RequiresCLI
					response["feature_name"] = info.Name
					response["description"] = info.Description
				}
				if info != nil && info.RequiresCLI && !gate.IsCLIInstalled() {
					response["message"] = "This feature requires Claude Code CLI to be installed"
				}
				return c.JSON(http.StatusForbidden, response)
			}
			return next(c)
		}
	}
}
