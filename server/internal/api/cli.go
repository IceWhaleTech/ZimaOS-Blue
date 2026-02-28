package api

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/features"
)

// CLIHandler handles CLI configuration API requests.
type CLIHandler struct {
	config         *config.ClaudeCodeCLIConfig
	featureGate    *features.FeatureGate
	onConfigChange func(*config.ClaudeCodeCLIConfig) error
}

// NewCLIHandler creates a new CLI handler.
func NewCLIHandler(cfg *config.ClaudeCodeCLIConfig, gate *features.FeatureGate) *CLIHandler {
	return &CLIHandler{
		config:      cfg,
		featureGate: gate,
	}
}

// SetOnConfigChange sets the callback for config changes.
func (h *CLIHandler) SetOnConfigChange(fn func(*config.ClaudeCodeCLIConfig) error) {
	h.onConfigChange = fn
}

// RegisterRoutes registers CLI configuration routes.
func (h *CLIHandler) RegisterRoutes(e *echo.Echo) {
	g := e.Group("/api/v1/cli")
	g.GET("/config", h.GetConfig)
	g.PUT("/config", h.UpdateConfig)
	g.POST("/enable", h.EnableCLI)
	g.POST("/disable", h.DisableCLI)
}

// CLIConfigResponse represents the CLI configuration response.
type CLIConfigResponse struct {
	Enabled  bool                     `json:"enabled"`
	Install  config.CLIInstallConfig  `json:"install"`
	Download config.CLIDownloadConfig `json:"download"`
	Features config.CLIFeaturesConfig `json:"features"`

	// Additional status info
	Installed        bool     `json:"installed"`
	Version          string   `json:"version,omitempty"`
	AffectedFeatures []string `json:"affected_features,omitempty"`
}

// GetConfig returns the current CLI configuration.
func (h *CLIHandler) GetConfig(c echo.Context) error {
	// Get affected features when CLI is disabled
	var affectedFeatures []string
	if !h.config.Enabled {
		cliFeatures := h.featureGate.GetCLIDependentFeatures()
		for _, f := range cliFeatures {
			affectedFeatures = append(affectedFeatures, f.Name)
		}
	}

	return c.JSON(http.StatusOK, &CLIConfigResponse{
		Enabled:          h.config.Enabled,
		Install:          h.config.Install,
		Download:         h.config.Download,
		Features:         h.config.Features,
		Installed:        h.featureGate.IsCLIInstalled(),
		AffectedFeatures: affectedFeatures,
	})
}

// UpdateConfigRequest represents the update config request.
type UpdateConfigRequest struct {
	Enabled  *bool                     `json:"enabled,omitempty"`
	Install  *config.CLIInstallConfig  `json:"install,omitempty"`
	Download *config.CLIDownloadConfig `json:"download,omitempty"`
	Features *config.CLIFeaturesConfig `json:"features,omitempty"`
}

// UpdateConfig updates the CLI configuration.
func (h *CLIHandler) UpdateConfig(c echo.Context) error {
	var req UpdateConfigRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	// Update config fields
	if req.Enabled != nil {
		h.config.Enabled = *req.Enabled
	}
	if req.Install != nil {
		h.config.Install = *req.Install
	}
	if req.Download != nil {
		h.config.Download = *req.Download
	}
	if req.Features != nil {
		h.config.Features = *req.Features
	}

	// Notify config change
	if h.onConfigChange != nil {
		if err := h.onConfigChange(h.config); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"config":  h.config,
	})
}

// EnableCLIResponse represents the enable CLI response.
type EnableCLIResponse struct {
	Success  bool     `json:"success"`
	Message  string   `json:"message"`
	Features []string `json:"features_enabled,omitempty"`
}

// EnableCLI enables CLI integration.
func (h *CLIHandler) EnableCLI(c echo.Context) error {
	if h.config.Enabled {
		return c.JSON(http.StatusOK, &EnableCLIResponse{
			Success: true,
			Message: "CLI is already enabled",
		})
	}

	h.config.Enabled = true

	// Notify config change
	if h.onConfigChange != nil {
		if err := h.onConfigChange(h.config); err != nil {
			h.config.Enabled = false // Rollback
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
		}
	}

	// Get enabled features
	var enabledFeatures []string
	cliFeatures := h.featureGate.GetCLIDependentFeatures()
	for _, f := range cliFeatures {
		enabledFeatures = append(enabledFeatures, f.Name)
	}

	return c.JSON(http.StatusOK, &EnableCLIResponse{
		Success:  true,
		Message:  "CLI enabled successfully",
		Features: enabledFeatures,
	})
}

// DisableCLIRequest represents the disable CLI request.
type DisableCLIRequest struct {
	Confirm bool `json:"confirm"`
}

// DisableCLIResponse represents the disable CLI response.
type DisableCLIResponse struct {
	Success          bool     `json:"success"`
	Message          string   `json:"message"`
	Warning          string   `json:"warning,omitempty"`
	FeaturesDisabled []string `json:"features_disabled,omitempty"`
}

// DisableCLI disables CLI integration.
func (h *CLIHandler) DisableCLI(c echo.Context) error {
	var req DisableCLIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	// Get features that will be disabled
	var disabledFeatures []string
	cliFeatures := h.featureGate.GetCLIDependentFeatures()
	for _, f := range cliFeatures {
		disabledFeatures = append(disabledFeatures, f.Name)
	}

	// Require confirmation
	if !req.Confirm {
		return c.JSON(http.StatusOK, &DisableCLIResponse{
			Success:          false,
			Message:          "Confirmation required",
			Warning:          "Disabling CLI will remove access to advanced features. Set 'confirm: true' to proceed.",
			FeaturesDisabled: disabledFeatures,
		})
	}

	if !h.config.Enabled {
		return c.JSON(http.StatusOK, &DisableCLIResponse{
			Success: true,
			Message: "CLI is already disabled",
		})
	}

	h.config.Enabled = false

	// Notify config change
	if h.onConfigChange != nil {
		if err := h.onConfigChange(h.config); err != nil {
			h.config.Enabled = true // Rollback
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
		}
	}

	return c.JSON(http.StatusOK, &DisableCLIResponse{
		Success:          true,
		Message:          "CLI disabled successfully",
		Warning:          "Advanced features are now unavailable. Enable CLI to restore functionality.",
		FeaturesDisabled: disabledFeatures,
	})
}

// GetCLIFeatureMatrix returns the feature availability matrix.
func (h *CLIHandler) GetCLIFeatureMatrix(c echo.Context) error {
	type FeatureMatrixEntry struct {
		Feature     string `json:"feature"`
		WithCLI     string `json:"with_cli"`
		WithoutCLI  string `json:"without_cli"`
		RequiresCLI bool   `json:"requires_cli"`
		Notes       string `json:"notes,omitempty"`
	}

	matrix := []FeatureMatrixEntry{
		{Feature: "Chat with Claude", WithCLI: "Full", WithoutCLI: "Full", RequiresCLI: false, Notes: "Direct API, no CLI needed"},
		{Feature: "Chat with Ollama", WithCLI: "Full", WithoutCLI: "Full", RequiresCLI: false, Notes: "Direct API, no CLI needed"},
		{Feature: "Chat with OpenAI", WithCLI: "Full", WithoutCLI: "Full", RequiresCLI: false, Notes: "Direct API, no CLI needed"},
		{Feature: "Skills", WithCLI: "Full", WithoutCLI: "None", RequiresCLI: true, Notes: "Requires CLI"},
		{Feature: "Tool Calling (Native)", WithCLI: "Full", WithoutCLI: "Limited", RequiresCLI: false, Notes: "Only providers with native support"},
		{Feature: "Tool Calling (Adapter)", WithCLI: "Full", WithoutCLI: "None", RequiresCLI: true, Notes: "CLIProxy adapter requires CLI"},
		{Feature: "File Operations", WithCLI: "Full", WithoutCLI: "None", RequiresCLI: true, Notes: "Requires CLI"},
		{Feature: "Terminal Commands", WithCLI: "Full", WithoutCLI: "None", RequiresCLI: true, Notes: "Requires CLI"},
		{Feature: "MCP Tools", WithCLI: "Full", WithoutCLI: "Full", RequiresCLI: false, Notes: "Native MCP server works without CLI"},
		{Feature: "Code Execution", WithCLI: "Full", WithoutCLI: "None", RequiresCLI: true, Notes: "Requires CLI"},
		{Feature: "Agent Mode", WithCLI: "Full", WithoutCLI: "Full", RequiresCLI: false, Notes: "Native agent runner works without CLI"},
		{Feature: "Project Context", WithCLI: "Full", WithoutCLI: "None", RequiresCLI: true, Notes: "Requires CLI"},
		{Feature: "Usage Statistics", WithCLI: "Full", WithoutCLI: "Basic", RequiresCLI: false, Notes: "Basic stats still available"},
		{Feature: "Provider Switching", WithCLI: "Full", WithoutCLI: "Full", RequiresCLI: false, Notes: "No CLI needed"},
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"cli_enabled": h.config.Enabled,
		"matrix":      matrix,
	})
}
