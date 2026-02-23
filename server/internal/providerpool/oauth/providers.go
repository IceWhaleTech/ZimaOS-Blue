package oauth

// ProviderConfig holds OAuth configuration for a specific LLM provider.
type ProviderConfig struct {
	ID           string   // "antigravity", "gemini-cli", "copilot"
	Name         string   // Display name
	FlowType     FlowType // authorization_code or device_code
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	Scopes       []string
	RedirectPort int    // Local callback port (for auth code flow)
	RedirectPath string // Callback path

	// Google Cloud Code specific
	CloudCodeEndpoints []string // Ordered list of endpoints to try
	LoadCodeAssistURL  string   // Project discovery endpoint

	// GitHub specific
	DeviceCodeURL string // For device code flow

	// API endpoint (for providers like Codex where API URL differs from auth URL)
	APIEndpoint string
}

// FlowType represents the OAuth flow type.
type FlowType string

const (
	FlowTypeAuthCode   FlowType = "authorization_code"
	FlowTypeDeviceCode FlowType = "device_code"
)

// AntigravityConfig returns the OAuth config for Google Antigravity.
func AntigravityConfig() *ProviderConfig {
	return &ProviderConfig{
		ID:           "antigravity",
		Name:         "Google Cloud Code",
		FlowType:     FlowTypeAuthCode,
		ClientID:     "1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com",
		ClientSecret: "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",
		AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:     "https://oauth2.googleapis.com/token",
		UserInfoURL:  "https://www.googleapis.com/oauth2/v1/userinfo?alt=json",
		Scopes: []string{
			"https://www.googleapis.com/auth/cloud-platform",
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
			"https://www.googleapis.com/auth/cclog",
			"https://www.googleapis.com/auth/experimentsandconfigs",
		},
		RedirectPort: 51121,
		RedirectPath: "/oauth-callback",
		CloudCodeEndpoints: []string{
			"https://cloudcode-pa.googleapis.com",
			"https://daily-cloudcode-pa.sandbox.googleapis.com",
			"https://autopush-cloudcode-pa.sandbox.googleapis.com",
		},
		LoadCodeAssistURL: "/v1internal:loadCodeAssist",
	}
}

// GeminiCLIConfig returns the OAuth config for Google Gemini CLI.
func GeminiCLIConfig() *ProviderConfig {
	return &ProviderConfig{
		ID:           "gemini-cli",
		Name:         "Google Cloud Code",
		FlowType:     FlowTypeAuthCode,
		ClientID:     "681255809395-oo8ft2oprdrnp9e3aqf6av3hmdib135j.apps.googleusercontent.com",
		ClientSecret: "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",
		AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:     "https://oauth2.googleapis.com/token",
		UserInfoURL:  "https://www.googleapis.com/oauth2/v1/userinfo?alt=json",
		Scopes: []string{
			"https://www.googleapis.com/auth/cloud-platform",
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		RedirectPort: 8085,
		RedirectPath: "/oauth2callback",
		CloudCodeEndpoints: []string{
			"https://cloudcode-pa.googleapis.com",
			"https://daily-cloudcode-pa.sandbox.googleapis.com",
		},
		LoadCodeAssistURL: "/v1internal:loadCodeAssist",
	}
}

// CopilotConfig returns the OAuth config for GitHub Copilot.
func CopilotConfig() *ProviderConfig {
	return &ProviderConfig{
		ID:            "copilot",
		Name:          "GitHub Copilot",
		FlowType:      FlowTypeDeviceCode,
		ClientID:      "Iv1.b507a08c87ecfe98", // GitHub Copilot CLI client ID
		AuthURL:       "https://github.com/login/device/code",
		TokenURL:      "https://github.com/login/oauth/access_token",
		UserInfoURL:   "https://api.github.com/user",
		DeviceCodeURL: "https://github.com/login/device/code",
		Scopes:        []string{"read:user"},
	}
}

// CodexConfig returns the OAuth config for OpenAI Codex CLI.
func CodexConfig() *ProviderConfig {
	return &ProviderConfig{
		ID:           "codex",
		Name:         "Codex CLI (OpenAI)",
		FlowType:     FlowTypeAuthCode,
		ClientID:     "app_EMoamEEZ73f0CkXaXp7hrann",
		AuthURL:      "https://auth.openai.com/oauth/authorize",
		TokenURL:     "https://auth.openai.com/oauth/token",
		RedirectPort: 1455,
		RedirectPath: "/auth/callback",
		Scopes:       []string{"openid", "profile", "email", "offline_access"},
		APIEndpoint:  "https://chatgpt.com/backend-api/codex/responses",
	}
}

// AllProviderConfigs returns all supported OAuth provider configs.
func AllProviderConfigs() map[string]*ProviderConfig {
	return map[string]*ProviderConfig{
		"antigravity": AntigravityConfig(),
		"gemini-cli":  GeminiCLIConfig(),
		"copilot":     CopilotConfig(),
		"codex":       CodexConfig(),
	}
}

// GetProviderConfig returns the OAuth config for a provider type.
func GetProviderConfig(providerType string) *ProviderConfig {
	configs := AllProviderConfigs()
	return configs[providerType]
}
