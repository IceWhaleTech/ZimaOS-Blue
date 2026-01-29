package providers

import (
	"os"
	"strings"
)

// EnvConfig holds configuration read from environment variables.
type EnvConfig struct {
	// Anthropic
	AnthropicAPIKey string `json:"anthropic_api_key,omitempty"`

	// OpenAI
	OpenAIAPIKey  string `json:"openai_api_key,omitempty"`
	OpenAIBaseURL string `json:"openai_base_url,omitempty"`

	// Claude Code specific
	ClaudeCodeModel     string `json:"claude_code_model,omitempty"`
	ClaudeCodeMaxTokens string `json:"claude_code_max_tokens,omitempty"`

	// Proxy settings
	HTTPProxy  string `json:"http_proxy,omitempty"`
	HTTPSProxy string `json:"https_proxy,omitempty"`
	NoProxy    string `json:"no_proxy,omitempty"`

	// cc-switch
	CCSwitchProfile string `json:"cc_switch_profile,omitempty"`

	// Google
	GoogleAPIKey string `json:"google_api_key,omitempty"`

	// Azure OpenAI
	AzureOpenAIKey      string `json:"azure_openai_key,omitempty"`
	AzureOpenAIEndpoint string `json:"azure_openai_endpoint,omitempty"`
}

// EnvConfigDisplay is a safe version of EnvConfig for display (with masked keys).
type EnvConfigDisplay struct {
	AnthropicAPIKey     string `json:"anthropic_api_key,omitempty"`
	OpenAIAPIKey        string `json:"openai_api_key,omitempty"`
	OpenAIBaseURL       string `json:"openai_base_url,omitempty"`
	ClaudeCodeModel     string `json:"claude_code_model,omitempty"`
	ClaudeCodeMaxTokens string `json:"claude_code_max_tokens,omitempty"`
	HTTPProxy           string `json:"http_proxy,omitempty"`
	HTTPSProxy          string `json:"https_proxy,omitempty"`
	NoProxy             string `json:"no_proxy,omitempty"`
	CCSwitchProfile     string `json:"cc_switch_profile,omitempty"`
	GoogleAPIKey        string `json:"google_api_key,omitempty"`
	AzureOpenAIKey      string `json:"azure_openai_key,omitempty"`
	AzureOpenAIEndpoint string `json:"azure_openai_endpoint,omitempty"`

	// Flags indicating which keys are set
	HasAnthropicKey bool `json:"has_anthropic_key"`
	HasOpenAIKey    bool `json:"has_openai_key"`
	HasGoogleKey    bool `json:"has_google_key"`
	HasAzureKey     bool `json:"has_azure_key"`
}

// EnvironmentReader reads configuration from environment variables.
type EnvironmentReader struct {
	// Variables is the list of environment variables to read.
	Variables []string
}

// NewEnvironmentReader creates a new environment reader with default variables.
func NewEnvironmentReader() *EnvironmentReader {
	return &EnvironmentReader{
		Variables: []string{
			"ANTHROPIC_API_KEY",
			"OPENAI_API_KEY",
			"OPENAI_BASE_URL",
			"CLAUDE_CODE_MODEL",
			"CLAUDE_CODE_MAX_TOKENS",
			"HTTP_PROXY",
			"HTTPS_PROXY",
			"NO_PROXY",
			"CC_SWITCH_PROFILE",
			"GOOGLE_API_KEY",
			"AZURE_OPENAI_KEY",
			"AZURE_OPENAI_ENDPOINT",
		},
	}
}

// ReadConfig reads all environment variables and returns the configuration.
func (r *EnvironmentReader) ReadConfig() *EnvConfig {
	return &EnvConfig{
		AnthropicAPIKey:     os.Getenv("ANTHROPIC_API_KEY"),
		OpenAIAPIKey:        os.Getenv("OPENAI_API_KEY"),
		OpenAIBaseURL:       os.Getenv("OPENAI_BASE_URL"),
		ClaudeCodeModel:     os.Getenv("CLAUDE_CODE_MODEL"),
		ClaudeCodeMaxTokens: os.Getenv("CLAUDE_CODE_MAX_TOKENS"),
		HTTPProxy:           os.Getenv("HTTP_PROXY"),
		HTTPSProxy:          os.Getenv("HTTPS_PROXY"),
		NoProxy:             os.Getenv("NO_PROXY"),
		CCSwitchProfile:     os.Getenv("CC_SWITCH_PROFILE"),
		GoogleAPIKey:        os.Getenv("GOOGLE_API_KEY"),
		AzureOpenAIKey:      os.Getenv("AZURE_OPENAI_KEY"),
		AzureOpenAIEndpoint: os.Getenv("AZURE_OPENAI_ENDPOINT"),
	}
}

// ReadConfigDisplay reads environment variables and returns a display-safe version.
func (r *EnvironmentReader) ReadConfigDisplay() *EnvConfigDisplay {
	config := r.ReadConfig()
	return config.ToDisplay()
}

// ToDisplay converts EnvConfig to a display-safe version with masked API keys.
func (c *EnvConfig) ToDisplay() *EnvConfigDisplay {
	return &EnvConfigDisplay{
		AnthropicAPIKey:     MaskAPIKey(c.AnthropicAPIKey),
		OpenAIAPIKey:        MaskAPIKey(c.OpenAIAPIKey),
		OpenAIBaseURL:       c.OpenAIBaseURL,
		ClaudeCodeModel:     c.ClaudeCodeModel,
		ClaudeCodeMaxTokens: c.ClaudeCodeMaxTokens,
		HTTPProxy:           c.HTTPProxy,
		HTTPSProxy:          c.HTTPSProxy,
		NoProxy:             c.NoProxy,
		CCSwitchProfile:     c.CCSwitchProfile,
		GoogleAPIKey:        MaskAPIKey(c.GoogleAPIKey),
		AzureOpenAIKey:      MaskAPIKey(c.AzureOpenAIKey),
		AzureOpenAIEndpoint: c.AzureOpenAIEndpoint,
		HasAnthropicKey:     c.AnthropicAPIKey != "",
		HasOpenAIKey:        c.OpenAIAPIKey != "",
		HasGoogleKey:        c.GoogleAPIKey != "",
		HasAzureKey:         c.AzureOpenAIKey != "",
	}
}

// HasAnthropicKey returns true if an Anthropic API key is set.
func (c *EnvConfig) HasAnthropicKey() bool {
	return c.AnthropicAPIKey != ""
}

// HasOpenAIKey returns true if an OpenAI API key is set.
func (c *EnvConfig) HasOpenAIKey() bool {
	return c.OpenAIAPIKey != ""
}

// HasGoogleKey returns true if a Google API key is set.
func (c *EnvConfig) HasGoogleKey() bool {
	return c.GoogleAPIKey != ""
}

// HasAzureKey returns true if an Azure OpenAI key is set.
func (c *EnvConfig) HasAzureKey() bool {
	return c.AzureOpenAIKey != ""
}

// HasAnyAPIKey returns true if any API key is set.
func (c *EnvConfig) HasAnyAPIKey() bool {
	return c.HasAnthropicKey() || c.HasOpenAIKey() || c.HasGoogleKey() || c.HasAzureKey()
}

// GetConfiguredProviders returns a list of providers that have API keys configured.
func (c *EnvConfig) GetConfiguredProviders() []string {
	var providers []string
	if c.HasAnthropicKey() {
		providers = append(providers, "anthropic")
	}
	if c.HasOpenAIKey() {
		providers = append(providers, "openai")
	}
	if c.HasGoogleKey() {
		providers = append(providers, "google")
	}
	if c.HasAzureKey() {
		providers = append(providers, "azure")
	}
	return providers
}

// MaskAPIKey masks an API key for safe display.
// Shows first 4 and last 4 characters, with asterisks in between.
func MaskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}
