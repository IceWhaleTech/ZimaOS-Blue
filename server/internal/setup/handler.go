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

	// Step 2: Security Settings
	AdminUsername string `json:"admin_username"`
	AdminPassword string `json:"admin_password"`
	EnableMFA     bool   `json:"enable_mfa"`

	// Step 3: Integration Settings
	EnableHomeAssistant bool   `json:"enable_home_assistant"`
	HomeAssistantURL    string `json:"home_assistant_url,omitempty"`
	HomeAssistantToken  string `json:"home_assistant_token,omitempty"`
}

// LLMConfig holds the LLM provider configuration.
type LLMConfig struct {
	Provider string `json:"provider"`
	APIKey   string `json:"api_key,omitempty"`
	BaseURL  string `json:"base_url,omitempty"`
	Model    string `json:"model,omitempty"`
}

// ValidationResult contains validation results for a step.
type ValidationResult struct {
	Valid   bool              `json:"valid"`
	Errors  map[string]string `json:"errors,omitempty"`
	Message string            `json:"message,omitempty"`
}

// UserCreator is a function that creates a user during setup.
type UserCreator func(username, password string, isAdmin bool) error

// UserChecker is a function that checks if a username exists.
type UserChecker func(username string) (bool, error)

// Handler handles setup wizard API requests.
type Handler struct {
	dataDir     string
	configPath  string
	mu          sync.RWMutex
	status      *SetupStatus
	createUser  UserCreator
	checkUser   UserChecker
	version     string
}

// NewHandler creates a new setup handler.
func NewHandler(dataDir string) *Handler {
	return NewHandlerWithVersion(dataDir, "0.0.0")
}

// NewHandlerWithVersion creates a new setup handler with a specific version.
func NewHandlerWithVersion(dataDir string, version string) *Handler {
	h := &Handler{
		dataDir:    dataDir,
		configPath: filepath.Join(dataDir, "setup_complete.json"),
		version:    version,
		status: &SetupStatus{
			Completed:   false,
			CurrentStep: 1,
			TotalSteps:  3,
			Version:     version,
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

// SetUserChecker sets the user checker function.
func (h *Handler) SetUserChecker(fn UserChecker) {
	h.checkUser = fn
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
	g.POST("/check-username", h.CheckUsername)
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

// CheckUsername checks if a username is available.
func (h *Handler) CheckUsername(c echo.Context) error {
	var req struct {
		Username string `json:"username"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"available": false,
			"message":   "Invalid request body",
		})
	}

	if req.Username == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"available": false,
			"message":   "Username is required",
		})
	}

	if len(req.Username) < 3 {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"available": false,
			"message":   "Username must be at least 3 characters",
		})
	}

	// Check if user checker is set
	if h.checkUser == nil {
		// If no checker is set, assume username is available
		return c.JSON(http.StatusOK, map[string]interface{}{
			"available": true,
			"message":   "Username is available",
		})
	}

	exists, err := h.checkUser(req.Username)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"available": false,
			"message":   "Failed to check username",
		})
	}

	if exists {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"available": false,
			"message":   "Username already exists",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"available": true,
		"message":   "Username is available",
	})
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
	case "slack":
		return h.testSlackConnection(c, req.Config)
	case "whatsapp":
		return h.testWhatsAppConnection(c, req.Config)
	case "signal":
		return h.testSignalConnection(c, req.Config)
	case "teams":
		return h.testTeamsConnection(c, req.Config)
	case "googlechat":
		return h.testGoogleChatConnection(c, req.Config)
	case "feishu":
		return h.testFeishuConnection(c, req.Config)
	case "dingtalk":
		return h.testDingTalkConnection(c, req.Config)
	case "qq":
		return h.testQQConnection(c, req.Config)
	case "wechat":
		return h.testWeChatConnection(c, req.Config)
	case "matrix":
		return h.testMatrixConnection(c, req.Config)
	case "imessage":
		return h.testIMessageConnection(c, req.Config)
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
	for step := 1; step <= 3; step++ {
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

	case 2: // Security Settings
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

	case 3: // Integration Settings
		if config.EnableHomeAssistant {
			if config.HomeAssistantURL == "" {
				errors["home_assistant_url"] = "Home Assistant URL is required"
			}
			if config.HomeAssistantToken == "" {
				errors["home_assistant_token"] = "Home Assistant token is required"
			}
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
			"success":    false,
			"messageKey": "providerRequired",
		})
	}

	if provider != "ollama" && apiKey == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "apiKeyRequired",
		})
	}

	if provider == "custom" && baseURL == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "baseUrlRequired",
		})
	}

	// Simulate successful connection test
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":    true,
		"messageKey": "testSuccess",
	})
}

func (h *Handler) testHomeAssistantConnection(c echo.Context, config map[string]string) error {
	url := config["url"]
	token := config["token"]

	if url == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "homeAssistantUrlRequired",
		})
	}

	if token == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "accessTokenRequired",
		})
	}

	// In a real implementation, this would make an API call to Home Assistant
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":    true,
		"messageKey": "testSuccess",
	})
}

func (h *Handler) testFeishuConnection(c echo.Context, config map[string]string) error {
	appID := config["app_id"]
	appSecret := config["app_secret"]

	if appID == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "appIdRequired",
		})
	}

	if appSecret == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "appSecretRequired",
		})
	}

	// In a real implementation, this would make an API call to Feishu to get tenant_access_token
	// POST https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":    true,
		"messageKey": "testSuccess",
	})
}

func (h *Handler) testTelegramConnection(c echo.Context, config map[string]string) error {
	botToken := config["bot_token"]

	if botToken == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "botTokenRequired",
		})
	}

	// In a real implementation, this would call Telegram's getMe API
	// GET https://api.telegram.org/bot<token>/getMe
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":    true,
		"messageKey": "testSuccess",
	})
}

func (h *Handler) testDiscordConnection(c echo.Context, config map[string]string) error {
	botToken := config["bot_token"]

	if botToken == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "botTokenRequired",
		})
	}

	// In a real implementation, this would call Discord's /users/@me API
	// GET https://discord.com/api/v10/users/@me
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":    true,
		"messageKey": "testSuccess",
	})
}

func (h *Handler) testSlackConnection(c echo.Context, config map[string]string) error {
	botToken := config["bot_token"]
	appToken := config["app_token"]

	if botToken == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "botTokenRequired",
		})
	}

	if appToken == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "appTokenRequired",
		})
	}

	// In a real implementation, this would call Slack's auth.test API
	// POST https://slack.com/api/auth.test
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":    true,
		"messageKey": "testSuccess",
	})
}

func (h *Handler) testWhatsAppConnection(c echo.Context, config map[string]string) error {
	phoneNumber := config["phone_number"]

	if phoneNumber == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "phoneNumberRequired",
		})
	}

	// WhatsApp Business API requires additional setup
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":    true,
		"messageKey": "configValidated",
	})
}

func (h *Handler) testSignalConnection(c echo.Context, config map[string]string) error {
	phoneNumber := config["phone_number"]

	if phoneNumber == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "phoneNumberRequired",
		})
	}

	// Signal requires signal-cli or similar setup
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":    true,
		"messageKey": "configValidated",
	})
}

func (h *Handler) testTeamsConnection(c echo.Context, config map[string]string) error {
	appID := config["app_id"]
	appPassword := config["app_password"]

	if appID == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "appIdRequired",
		})
	}

	if appPassword == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "appPasswordRequired",
		})
	}

	// In a real implementation, this would authenticate with Microsoft Bot Framework
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":    true,
		"messageKey": "testSuccess",
	})
}

func (h *Handler) testGoogleChatConnection(c echo.Context, config map[string]string) error {
	credentialsJSON := config["credentials_json"]

	if credentialsJSON == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "serviceAccountJsonRequired",
		})
	}

	// In a real implementation, this would validate the service account credentials
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":    true,
		"messageKey": "testSuccess",
	})
}

func (h *Handler) testWeChatConnection(c echo.Context, config map[string]string) error {
	corpID := config["corp_id"]
	agentID := config["agent_id"]
	secret := config["secret"]

	if corpID == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "corpIdRequired",
		})
	}

	if agentID == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "agentIdRequired",
		})
	}

	if secret == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "secretRequired",
		})
	}

	// In a real implementation, this would call WeChat Work's gettoken API
	// GET https://qyapi.weixin.qq.com/cgi-bin/gettoken
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":    true,
		"messageKey": "testSuccess",
	})
}

func (h *Handler) testMatrixConnection(c echo.Context, config map[string]string) error {
	homeserver := config["homeserver"]
	userID := config["user_id"]
	accessToken := config["access_token"]

	if homeserver == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "homeserverUrlRequired",
		})
	}

	if userID == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "userIdRequired",
		})
	}

	if accessToken == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "accessTokenRequired",
		})
	}

	// In a real implementation, this would call Matrix's whoami API
	// GET /_matrix/client/v3/account/whoami
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":    true,
		"messageKey": "testSuccess",
	})
}

func (h *Handler) testIMessageConnection(c echo.Context, config map[string]string) error {
	// iMessage doesn't require configuration fields, it uses system integration
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":    true,
		"messageKey": "iMessageReady",
	})
}

func (h *Handler) testDingTalkConnection(c echo.Context, config map[string]string) error {
	appKey := config["app_key"]
	appSecret := config["app_secret"]
	robotCode := config["robot_code"]

	if appKey == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "appKeyRequired",
		})
	}

	if appSecret == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "appSecretRequired",
		})
	}

	if robotCode == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "robotCodeRequired",
		})
	}

	// In a real implementation, this would call DingTalk's gettoken API
	// POST https://api.dingtalk.com/v1.0/oauth2/accessToken
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":    true,
		"messageKey": "testSuccess",
	})
}

func (h *Handler) testQQConnection(c echo.Context, config map[string]string) error {
	appID := config["app_id"]
	appSecret := config["app_secret"]
	token := config["token"]

	if appID == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "appIdRequired",
		})
	}

	if appSecret == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "appSecretRequired",
		})
	}

	if token == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "botTokenRequired",
		})
	}

	// In a real implementation, this would call QQ Bot's API to verify credentials
	// GET https://api.sgroup.qq.com/users/@me
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":    true,
		"messageKey": "testSuccess",
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
		"language":              config.Language,
		"timezone":              config.Timezone,
		"enable_mfa":            config.EnableMFA,
		"enable_home_assistant": config.EnableHomeAssistant,
		"home_assistant_url":    config.HomeAssistantURL,
		"admin_username":        config.AdminUsername,
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

// LoadLLMConfig loads the saved LLM configuration from disk.
func LoadLLMConfig(dataDir string) (*LLMConfig, error) {
	configPath := filepath.Join(dataDir, "llm_config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No config file, return nil without error
		}
		return nil, err
	}

	var config LLMConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
