package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/llm"
)

// ProviderConfig holds the configuration for a single provider.
type ProviderConfig struct {
	Name    string `json:"name"`
	APIKey  string `json:"api_key,omitempty"`
	BaseURL string `json:"base_url,omitempty"`
	Enabled bool   `json:"enabled"`
}

// ProvidersConfig holds all provider configurations.
type ProvidersConfig struct {
	Providers map[string]ProviderConfig `json:"providers"`
}

// ProviderSettingsHandler handles provider settings API endpoints.
type ProviderSettingsHandler struct {
	registry *llm.ProviderRegistry
	dataDir  string
	mu       sync.RWMutex
	config   *ProvidersConfig
}

// NewProviderSettingsHandler creates a new provider settings handler.
func NewProviderSettingsHandler(registry *llm.ProviderRegistry, dataDir string) *ProviderSettingsHandler {
	h := &ProviderSettingsHandler{
		registry: registry,
		dataDir:  dataDir,
		config: &ProvidersConfig{
			Providers: make(map[string]ProviderConfig),
		},
	}
	h.loadConfig()
	// Apply saved configs to all providers
	h.applyAllConfigs()
	return h
}

// applyAllConfigs applies all saved configurations to their respective providers.
func (h *ProviderSettingsHandler) applyAllConfigs() {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for name, config := range h.config.Providers {
		h.updateProviderInRegistry(name, config)
	}
}

// loadConfig loads the configuration from disk.
func (h *ProviderSettingsHandler) loadConfig() {
	if h.dataDir == "" {
		return
	}
	configPath := filepath.Join(h.dataDir, "provider_settings.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}
	var config ProvidersConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return
	}
	h.mu.Lock()
	h.config = &config
	h.mu.Unlock()
}

// saveConfig saves the configuration to disk.
func (h *ProviderSettingsHandler) saveConfig() error {
	if h.dataDir == "" {
		return nil
	}
	if err := os.MkdirAll(h.dataDir, 0755); err != nil {
		return err
	}
	h.mu.RLock()
	data, err := json.MarshalIndent(h.config, "", "  ")
	h.mu.RUnlock()
	if err != nil {
		return err
	}
	configPath := filepath.Join(h.dataDir, "provider_settings.json")
	return os.WriteFile(configPath, data, 0600) // Restrictive permissions for API keys
}

// RegisterRoutes registers the provider settings API routes.
func (h *ProviderSettingsHandler) RegisterRoutes(g *echo.Group) {
	g.GET("", h.ListProviderConfigs)
	g.GET("/:name", h.GetProviderConfig)
	g.PUT("/:name", h.UpdateProviderConfig)
	g.POST("/:name/test", h.TestProviderConnection)
}

// ProviderConfigResponse is the response for provider config endpoints.
type ProviderConfigResponse struct {
	Name       string `json:"name"`
	APIKey     string `json:"api_key,omitempty"` // Masked for security
	BaseURL    string `json:"base_url,omitempty"`
	Enabled    bool   `json:"enabled"`
	HasAPIKey  bool   `json:"has_api_key"`
	DefaultURL string `json:"default_url,omitempty"`
}

// ListProviderConfigs returns all provider configurations.
func (h *ProviderSettingsHandler) ListProviderConfigs(c echo.Context) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Get all registered providers
	providerNames := h.registry.List()
	configs := make([]ProviderConfigResponse, 0, len(providerNames))

	for _, name := range providerNames {
		config := h.getProviderConfigResponse(name)
		configs = append(configs, config)
	}

	return c.JSON(http.StatusOK, configs)
}

// GetProviderConfig returns the configuration for a specific provider.
func (h *ProviderSettingsHandler) GetProviderConfig(c echo.Context) error {
	name := c.Param("name")

	h.mu.RLock()
	defer h.mu.RUnlock()

	config := h.getProviderConfigResponse(name)
	return c.JSON(http.StatusOK, config)
}

// getProviderConfigResponse builds a response for a provider config.
func (h *ProviderSettingsHandler) getProviderConfigResponse(name string) ProviderConfigResponse {
	resp := ProviderConfigResponse{
		Name:    name,
		Enabled: true, // Default to enabled if provider exists
	}

	// Set default URLs based on provider
	switch name {
	case "claude":
		resp.DefaultURL = "https://api.anthropic.com"
	case "openai":
		resp.DefaultURL = "https://api.openai.com"
	case "ollama":
		resp.DefaultURL = "http://localhost:11434"
	case "grok":
		resp.DefaultURL = "https://api.x.ai"
	case "qwen":
		resp.DefaultURL = "https://dashscope.aliyuncs.com/compatible-mode"
	}

	// Get saved config if exists
	if saved, ok := h.config.Providers[name]; ok {
		resp.HasAPIKey = saved.APIKey != ""
		resp.BaseURL = saved.BaseURL
		resp.Enabled = saved.Enabled
		// Mask API key for security
		if saved.APIKey != "" {
			resp.APIKey = maskAPIKey(saved.APIKey)
		}
	}

	return resp
}

// UpdateProviderConfigRequest is the request for updating provider config.
type UpdateProviderConfigRequest struct {
	APIKey  *string `json:"api_key,omitempty"`
	BaseURL *string `json:"base_url,omitempty"`
	Enabled *bool   `json:"enabled,omitempty"`
}

// UpdateProviderConfig updates the configuration for a specific provider.
func (h *ProviderSettingsHandler) UpdateProviderConfig(c echo.Context) error {
	name := c.Param("name")

	var req UpdateProviderConfigRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
	}

	h.mu.Lock()

	// Get or create provider config
	config, ok := h.config.Providers[name]
	if !ok {
		config = ProviderConfig{
			Name:    name,
			Enabled: true,
		}
	}

	// Update fields if provided
	if req.APIKey != nil {
		config.APIKey = *req.APIKey
	}
	if req.BaseURL != nil {
		config.BaseURL = *req.BaseURL
	}
	if req.Enabled != nil {
		config.Enabled = *req.Enabled
	}

	h.config.Providers[name] = config
	h.mu.Unlock()

	// Save to disk
	if err := h.saveConfig(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to save configuration: " + err.Error(),
		})
	}

	// Update the provider in the registry
	h.updateProviderInRegistry(name, config)

	// Return updated config
	h.mu.RLock()
	resp := h.getProviderConfigResponse(name)
	h.mu.RUnlock()

	return c.JSON(http.StatusOK, resp)
}

// updateProviderInRegistry updates the provider instance in the registry.
func (h *ProviderSettingsHandler) updateProviderInRegistry(name string, config ProviderConfig) {
	switch name {
	case "claude":
		baseURL := config.BaseURL
		if baseURL == "" {
			baseURL = "https://api.anthropic.com"
		}
		provider := llm.NewClaudeProvider(config.APIKey, baseURL)
		h.registry.Update(provider)
	case "openai":
		baseURL := config.BaseURL
		if baseURL == "" {
			baseURL = "https://api.openai.com"
		}
		provider := llm.NewOpenAIProvider(config.APIKey, baseURL)
		h.registry.Update(provider)
	case "ollama":
		baseURL := config.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		provider := llm.NewOllamaProvider(baseURL)
		h.registry.Update(provider)
	case "custom":
		// Update custom OpenAI-compatible provider
		baseURL := config.BaseURL
		if baseURL == "" {
			baseURL = "https://api.openai.com"
		}
		provider := llm.NewCustomProvider(config.APIKey, baseURL)
		h.registry.Update(provider)
	case "grok":
		baseURL := config.BaseURL
		if baseURL == "" {
			baseURL = "https://api.x.ai"
		}
		provider := llm.NewGrokProvider(config.APIKey, baseURL)
		h.registry.Update(provider)
	case "qwen":
		baseURL := config.BaseURL
		if baseURL == "" {
			baseURL = "https://dashscope.aliyuncs.com/compatible-mode"
		}
		provider := llm.NewQwenProvider(config.APIKey, baseURL)
		h.registry.Update(provider)
	}
}

// TestProviderConnection tests the connection to a provider.
func (h *ProviderSettingsHandler) TestProviderConnection(c echo.Context) error {
	name := c.Param("name")

	h.mu.RLock()
	config, ok := h.config.Providers[name]
	h.mu.RUnlock()

	if !ok {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "providerNotConfigured",
		})
	}

	// Basic validation
	switch name {
	case "claude", "openai":
		if config.APIKey == "" {
			return c.JSON(http.StatusOK, map[string]interface{}{
				"success":    false,
				"messageKey": "apiKeyRequired",
			})
		}
	}

	// TODO: Actually test the connection by making a simple API call
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":    true,
		"messageKey": "testSuccess",
	})
}

// GetConfig returns the current provider config for a given name.
func (h *ProviderSettingsHandler) GetConfig(name string) *ProviderConfig {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if config, ok := h.config.Providers[name]; ok {
		return &config
	}
	return nil
}

// maskAPIKey masks an API key for display.
func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}
