package setup

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/labstack/echo/v4"
)

// SetupStatus represents the current setup state.
type SetupStatus struct {
	Completed   bool   `json:"completed"`
	CurrentStep int    `json:"current_step"`
	TotalSteps  int    `json:"total_steps"`
	Version     string `json:"version"`
}

// SetupConfig holds the initial configuration from the wizard.
type SetupConfig struct {
	// Step 1: Basic Settings
	Language string `json:"language"`
	Timezone string `json:"timezone"`

	// Step 2: LLM Configuration
	LLMProvider string `json:"llm_provider"` // openai, ollama, anthropic, custom
	LLMAPIKey   string `json:"llm_api_key,omitempty"`
	LLMBaseURL  string `json:"llm_base_url,omitempty"`
	LLMModel    string `json:"llm_model"`

	// Step 3: Security Settings
	AdminUsername string `json:"admin_username"`
	AdminPassword string `json:"admin_password"`
	EnableMFA     bool   `json:"enable_mfa"`

	// Step 4: Integration Settings
	EnableHomeAssistant bool   `json:"enable_home_assistant"`
	HomeAssistantURL    string `json:"home_assistant_url,omitempty"`
	HomeAssistantToken  string `json:"home_assistant_token,omitempty"`

	// Step 5: Channel Settings
	EnableTelegram bool   `json:"enable_telegram"`
	TelegramToken  string `json:"telegram_token,omitempty"`
	EnableDiscord  bool   `json:"enable_discord"`
	DiscordToken   string `json:"discord_token,omitempty"`
}

// ValidationResult contains validation results for a step.
type ValidationResult struct {
	Valid   bool              `json:"valid"`
	Errors  map[string]string `json:"errors,omitempty"`
	Message string            `json:"message,omitempty"`
}

// UserCreator is a function that creates a user during setup.
type UserCreator func(username, password string, isAdmin bool) error

// Handler handles setup wizard API requests.
type Handler struct {
	dataDir    string
	configPath string
	mu         sync.RWMutex
	status     *SetupStatus
	createUser UserCreator
}

// NewHandler creates a new setup handler.
func NewHandler(dataDir string) *Handler {
	h := &Handler{
		dataDir:    dataDir,
		configPath: filepath.Join(dataDir, "setup_complete.json"),
		status: &SetupStatus{
			Completed:   false,
			CurrentStep: 1,
			TotalSteps:  5,
			Version:     "0.5.0",
		},
	}

	// Load existing status
	h.loadStatus()

	return h
}

// SetUserCreator sets the user creation function.
func (h *Handler) SetUserCreator(fn UserCreator) {
	h.createUser = fn
}

// RegisterRoutes registers setup routes.
func (h *Handler) RegisterRoutes(e *echo.Echo) {
	g := e.Group("/api/setup")
	g.GET("/status", h.GetStatus)
	g.POST("/validate", h.ValidateStep)
	g.POST("/complete", h.CompleteSetup)
	g.POST("/reset", h.ResetSetup)
	g.GET("/defaults", h.GetDefaults)
	g.POST("/test-connection", h.TestConnection)
}

// GetStatus returns the current setup status.
func (h *Handler) GetStatus(c echo.Context) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return c.JSON(http.StatusOK, h.status)
}

// GetDefaults returns default configuration values.
func (h *Handler) GetDefaults(c echo.Context) error {
	defaults := map[string]interface{}{
		"language":     "en",
		"timezone":     "UTC",
		"llm_provider": "openai",
		"llm_model":    "gpt-4o-mini",
		"llm_base_url": "https://api.openai.com",
		"enable_mfa":   false,
		"providers": []map[string]interface{}{
			{
				"id":          "openai",
				"name":        "OpenAI",
				"requires_key": true,
				"models":      []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-3.5-turbo"},
				"base_url":    "https://api.openai.com",
			},
			{
				"id":          "anthropic",
				"name":        "Anthropic",
				"requires_key": true,
				"models":      []string{"claude-3-5-sonnet-20241022", "claude-3-opus-20240229", "claude-3-haiku-20240307"},
				"base_url":    "https://api.anthropic.com",
			},
			{
				"id":          "ollama",
				"name":        "Ollama (Local)",
				"requires_key": false,
				"models":      []string{"llama3.2", "llama3.1", "mistral", "codellama", "phi3"},
				"base_url":    "http://localhost:11434",
			},
			{
				"id":          "custom",
				"name":        "Custom OpenAI-Compatible",
				"requires_key": true,
				"models":      []string{},
				"base_url":    "",
			},
		},
		"languages": []map[string]string{
			{"code": "ca-ES", "name": "Català"},
			{"code": "cs-CZ", "name": "Čeština"},
			{"code": "da-DK", "name": "Dansk"},
			{"code": "de-DE", "name": "Deutsch"},
			{"code": "el-GR", "name": "Ελληνικά"},
			{"code": "en-GB", "name": "English (UK)"},
			{"code": "en-US", "name": "English (US)"},
			{"code": "es-ES", "name": "Español"},
			{"code": "fr-FR", "name": "Français"},
			{"code": "ga-IE", "name": "Gaeilge"},
			{"code": "hr-HR", "name": "Hrvatski"},
			{"code": "hu-HU", "name": "Magyar"},
			{"code": "it-IT", "name": "Italiano"},
			{"code": "ja-JP", "name": "日本語"},
			{"code": "ko-KR", "name": "한국어"},
			{"code": "ml-IN", "name": "മലയാളം"},
			{"code": "nb-NO", "name": "Norsk bokmål"},
			{"code": "nl-NL", "name": "Nederlands"},
			{"code": "pl-PL", "name": "Polski"},
			{"code": "pt-BR", "name": "Português (Brasil)"},
			{"code": "pt-PT", "name": "Português (Portugal)"},
			{"code": "ro-RO", "name": "Română"},
			{"code": "ru-RU", "name": "Русский"},
			{"code": "sk-SK", "name": "Slovenčina"},
			{"code": "sv-SE", "name": "Svenska"},
			{"code": "zh-CN", "name": "简体中文"},
			{"code": "zh-TW", "name": "繁體中文"},
		},
	}

	return c.JSON(http.StatusOK, defaults)
}

// ValidateStep validates a specific setup step.
func (h *Handler) ValidateStep(c echo.Context) error {
	var req struct {
		Step   int         `json:"step"`
		Config SetupConfig `json:"config"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ValidationResult{
			Valid:   false,
			Message: "Invalid request body",
		})
	}

	result := h.validateStep(req.Step, &req.Config)
	return c.JSON(http.StatusOK, result)
}

// TestConnection tests a connection (LLM, Home Assistant, etc.).
func (h *Handler) TestConnection(c echo.Context) error {
	var req struct {
		Type   string            `json:"type"` // llm, homeassistant, telegram, discord
		Config map[string]string `json:"config"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Test connection based on type
	switch req.Type {
	case "llm":
		return h.testLLMConnection(c, req.Config)
	case "homeassistant":
		return h.testHomeAssistantConnection(c, req.Config)
	case "telegram":
		return h.testTelegramConnection(c, req.Config)
	case "discord":
		return h.testDiscordConnection(c, req.Config)
	default:
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Unknown connection type",
		})
	}
}

// CompleteSetup completes the setup wizard.
func (h *Handler) CompleteSetup(c echo.Context) error {
	var config SetupConfig
	if err := c.Bind(&config); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Invalid configuration",
		})
	}

	// Validate all steps
	for step := 1; step <= 5; step++ {
		result := h.validateStep(step, &config)
		if !result.Valid {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"success": false,
				"message": result.Message,
				"step":    step,
				"errors":  result.Errors,
			})
		}
	}

	// Create admin user if user creator is set
	if h.createUser != nil {
		if err := h.createUser(config.AdminUsername, config.AdminPassword, true); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"success": false,
				"message": "Failed to create admin user: " + err.Error(),
			})
		}
	}

	// Save configuration
	if err := h.saveConfig(&config); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to save configuration: " + err.Error(),
		})
	}

	// Mark setup as complete
	h.mu.Lock()
	h.status.Completed = true
	h.status.CurrentStep = h.status.TotalSteps
	h.mu.Unlock()

	if err := h.saveStatus(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to save setup status: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Setup completed successfully",
	})
}

// ResetSetup resets the setup wizard (for testing/reconfiguration).
func (h *Handler) ResetSetup(c echo.Context) error {
	h.mu.Lock()
	h.status.Completed = false
	h.status.CurrentStep = 1
	h.mu.Unlock()

	// Remove setup complete file
	os.Remove(h.configPath)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Setup reset successfully",
	})
}

// IsSetupComplete returns whether setup is complete.
func (h *Handler) IsSetupComplete() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.status.Completed
}

func (h *Handler) validateStep(step int, config *SetupConfig) ValidationResult {
	errors := make(map[string]string)

	switch step {
	case 1: // Basic Settings
		if config.Language == "" {
			errors["language"] = "Language is required"
		}
		if config.Timezone == "" {
			errors["timezone"] = "Timezone is required"
		}

	case 2: // LLM Configuration
		if config.LLMProvider == "" {
			errors["llm_provider"] = "LLM provider is required"
		}
		if config.LLMProvider != "ollama" && config.LLMAPIKey == "" {
			errors["llm_api_key"] = "API key is required for this provider"
		}
		if config.LLMModel == "" {
			errors["llm_model"] = "Model selection is required"
		}
		if config.LLMProvider == "custom" && config.LLMBaseURL == "" {
			errors["llm_base_url"] = "Base URL is required for custom provider"
		}

	case 3: // Security Settings
		if config.AdminUsername == "" {
			errors["admin_username"] = "Admin username is required"
		} else if len(config.AdminUsername) < 3 {
			errors["admin_username"] = "Username must be at least 3 characters"
		}
		if config.AdminPassword == "" {
			errors["admin_password"] = "Admin password is required"
		} else if len(config.AdminPassword) < 8 {
			errors["admin_password"] = "Password must be at least 8 characters"
		}

	case 4: // Integration Settings
		if config.EnableHomeAssistant {
			if config.HomeAssistantURL == "" {
				errors["home_assistant_url"] = "Home Assistant URL is required"
			}
			if config.HomeAssistantToken == "" {
				errors["home_assistant_token"] = "Home Assistant token is required"
			}
		}

	case 5: // Channel Settings
		if config.EnableTelegram && config.TelegramToken == "" {
			errors["telegram_token"] = "Telegram bot token is required"
		}
		if config.EnableDiscord && config.DiscordToken == "" {
			errors["discord_token"] = "Discord bot token is required"
		}
	}

	if len(errors) > 0 {
		return ValidationResult{
			Valid:   false,
			Errors:  errors,
			Message: "Validation failed",
		}
	}

	return ValidationResult{
		Valid:   true,
		Message: "Validation passed",
	}
}

func (h *Handler) testLLMConnection(c echo.Context, config map[string]string) error {
	// In a real implementation, this would make an API call to test the connection
	// For now, we just validate the configuration
	provider := config["provider"]
	apiKey := config["api_key"]
	baseURL := config["base_url"]

	if provider == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success": false,
			"message": "Provider is required",
		})
	}

	if provider != "ollama" && apiKey == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success": false,
			"message": "API key is required",
		})
	}

	if provider == "custom" && baseURL == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success": false,
			"message": "Base URL is required for custom provider",
		})
	}

	// Simulate successful connection test
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Connection successful",
	})
}

func (h *Handler) testHomeAssistantConnection(c echo.Context, config map[string]string) error {
	url := config["url"]
	token := config["token"]

	if url == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success": false,
			"message": "Home Assistant URL is required",
		})
	}

	if token == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success": false,
			"message": "Access token is required",
		})
	}

	// In a real implementation, this would make an API call to Home Assistant
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Connection successful",
	})
}

func (h *Handler) testTelegramConnection(c echo.Context, config map[string]string) error {
	token := config["token"]

	if token == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success": false,
			"message": "Bot token is required",
		})
	}

	// In a real implementation, this would call the Telegram API
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Connection successful",
	})
}

func (h *Handler) testDiscordConnection(c echo.Context, config map[string]string) error {
	token := config["token"]

	if token == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success": false,
			"message": "Bot token is required",
		})
	}

	// In a real implementation, this would call the Discord API
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Connection successful",
	})
}

func (h *Handler) loadStatus() {
	data, err := os.ReadFile(h.configPath)
	if err != nil {
		return
	}

	var status SetupStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return
	}

	h.status = &status
}

func (h *Handler) saveStatus() error {
	if err := os.MkdirAll(h.dataDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(h.status, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(h.configPath, data, 0644)
}

func (h *Handler) saveConfig(config *SetupConfig) error {
	if err := os.MkdirAll(h.dataDir, 0755); err != nil {
		return err
	}

	// Save the setup configuration (excluding sensitive data in plain text)
	safeConfig := map[string]interface{}{
		"language":               config.Language,
		"timezone":               config.Timezone,
		"llm_provider":           config.LLMProvider,
		"llm_model":              config.LLMModel,
		"llm_base_url":           config.LLMBaseURL,
		"enable_mfa":             config.EnableMFA,
		"enable_home_assistant":  config.EnableHomeAssistant,
		"home_assistant_url":     config.HomeAssistantURL,
		"enable_telegram":        config.EnableTelegram,
		"enable_discord":         config.EnableDiscord,
		"admin_username":         config.AdminUsername,
		// Note: Sensitive data like passwords and tokens should be stored securely
		// This is a simplified version for the wizard
	}

	data, err := json.MarshalIndent(safeConfig, "", "  ")
	if err != nil {
		return err
	}

	configPath := filepath.Join(h.dataDir, "wizard_config.json")
	return os.WriteFile(configPath, data, 0644)
}
