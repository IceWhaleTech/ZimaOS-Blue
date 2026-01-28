package claudecode

import (
	"testing"
	"time"
)

func TestDefaultClaudeCodeBackend(t *testing.T) {
	backend := DefaultClaudeCodeBackend()

	if backend.Command != "claude" {
		t.Errorf("expected command 'claude', got '%s'", backend.Command)
	}

	// Output should be text for character-level streaming
	if backend.Output != "text" {
		t.Errorf("expected output 'text', got '%s'", backend.Output)
	}

	if backend.Input != "arg" {
		t.Errorf("expected input 'arg', got '%s'", backend.Input)
	}

	if backend.SessionMode != "always" {
		t.Errorf("expected session_mode 'always', got '%s'", backend.SessionMode)
	}

	if backend.SystemPromptWhen != "first" {
		t.Errorf("expected system_prompt_when 'first', got '%s'", backend.SystemPromptWhen)
	}

	if !backend.Serialize {
		t.Error("expected serialize to be true")
	}

	// Check model aliases
	if alias, ok := backend.ModelAliases["opus"]; !ok || alias != "opus" {
		t.Errorf("expected model alias 'opus' -> 'opus', got '%s'", alias)
	}

	// Check that Args contains text for streaming output
	hasTextFormat := false
	for i, arg := range backend.Args {
		if arg == "--output-format" && i+1 < len(backend.Args) && backend.Args[i+1] == "text" {
			hasTextFormat = true
			break
		}
	}
	if !hasTextFormat {
		t.Error("expected Args to contain '--output-format text' for streaming")
	}
}

func TestDefaultOpenCodeBackend(t *testing.T) {
	backend := DefaultOpenCodeBackend()

	if backend.Command != "opencode" {
		t.Errorf("expected command 'opencode', got '%s'", backend.Command)
	}

	if backend.Output != "json" {
		t.Errorf("expected output 'json', got '%s'", backend.Output)
	}
}

func TestDefaultClaudeCodeConfig(t *testing.T) {
	config := DefaultClaudeCodeConfig()

	if config.Enabled {
		t.Error("expected enabled to be false by default")
	}

	if config.Command != "claude" {
		t.Errorf("expected command 'claude', got '%s'", config.Command)
	}

	if config.DefaultModel != "sonnet" {
		t.Errorf("expected default_model 'sonnet', got '%s'", config.DefaultModel)
	}

	if config.Timeout != 5*time.Minute {
		t.Errorf("expected timeout 5m, got %v", config.Timeout)
	}

	if config.SessionTTL != 24*time.Hour {
		t.Errorf("expected session_ttl 24h, got %v", config.SessionTTL)
	}
}

func TestCliBackendConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  CliBackendConfig
		wantErr bool
	}{
		{
			name:    "empty command",
			config:  CliBackendConfig{},
			wantErr: true,
		},
		{
			name: "valid config",
			config: CliBackendConfig{
				Command: "claude",
			},
			wantErr: false,
		},
		{
			name: "invalid output",
			config: CliBackendConfig{
				Command: "claude",
				Output:  "invalid",
			},
			wantErr: true,
		},
		{
			name: "invalid input",
			config: CliBackendConfig{
				Command: "claude",
				Input:   "invalid",
			},
			wantErr: true,
		},
		{
			name: "invalid session_mode",
			config: CliBackendConfig{
				Command:     "claude",
				SessionMode: "invalid",
			},
			wantErr: true,
		},
		{
			name: "invalid system_prompt_mode",
			config: CliBackendConfig{
				Command:          "claude",
				SystemPromptMode: "invalid",
			},
			wantErr: true,
		},
		{
			name: "invalid system_prompt_when",
			config: CliBackendConfig{
				Command:          "claude",
				SystemPromptWhen: "invalid",
			},
			wantErr: true,
		},
		{
			name: "invalid image_mode",
			config: CliBackendConfig{
				Command:   "claude",
				ImageMode: "invalid",
			},
			wantErr: true,
		},
		{
			name: "valid full config",
			config: CliBackendConfig{
				Command:          "claude",
				Output:           "json",
				Input:            "arg",
				SessionMode:      "always",
				SystemPromptMode: "append",
				SystemPromptWhen: "first",
				ImageMode:        "repeat",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClaudeCodeConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  ClaudeCodeConfig
		wantErr bool
	}{
		{
			name: "disabled config",
			config: ClaudeCodeConfig{
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name: "enabled without command",
			config: ClaudeCodeConfig{
				Enabled: true,
			},
			wantErr: true,
		},
		{
			name: "enabled with invalid timeout",
			config: ClaudeCodeConfig{
				Enabled: true,
				Command: "claude",
				Timeout: 0,
			},
			wantErr: true,
		},
		{
			name: "enabled with invalid session_ttl",
			config: ClaudeCodeConfig{
				Enabled:    true,
				Command:    "claude",
				Timeout:    5 * time.Minute,
				SessionTTL: 0,
			},
			wantErr: true,
		},
		{
			name: "valid enabled config",
			config: ClaudeCodeConfig{
				Enabled:    true,
				Command:    "claude",
				Timeout:    5 * time.Minute,
				SessionTTL: 24 * time.Hour,
				Backend: CliBackendConfig{
					Command: "claude",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCliBackendConfigWithDefaults(t *testing.T) {
	config := CliBackendConfig{}
	result := config.WithDefaults()

	if result.Command != "claude" {
		t.Errorf("expected command 'claude', got '%s'", result.Command)
	}

	// Default output is now text for character-level streaming
	if result.Output != "text" {
		t.Errorf("expected output 'text', got '%s'", result.Output)
	}

	if result.Input != "arg" {
		t.Errorf("expected input 'arg', got '%s'", result.Input)
	}

	if result.SessionMode != "always" {
		t.Errorf("expected session_mode 'always', got '%s'", result.SessionMode)
	}
}

func TestClaudeCodeConfigWithDefaults(t *testing.T) {
	config := ClaudeCodeConfig{}
	result := config.WithDefaults()

	if result.Command != "claude" {
		t.Errorf("expected command 'claude', got '%s'", result.Command)
	}

	if result.DefaultModel != "sonnet" {
		t.Errorf("expected default_model 'sonnet', got '%s'", result.DefaultModel)
	}

	if result.Timeout != 5*time.Minute {
		t.Errorf("expected timeout 5m, got %v", result.Timeout)
	}
}

func TestClaudeCodeConfigWithAPIKeyAndBaseURL(t *testing.T) {
	// Test configuration matching user's setup:
	// ANTHROPIC_BASE_URL: https://paid.tribiosapi.top/
	// ANTHROPIC_AUTH_TOKEN: sk-ZAFeyOQ06TUW9qKlolQsKFtaePRsZYRzS1YOrYtx5n44X8zE
	config := ClaudeCodeConfig{
		APIKey:  "sk-ZAFeyOQ06TUW9qKlolQsKFtaePRsZYRzS1YOrYtx5n44X8zE",
		BaseURL: "https://paid.tribiosapi.top/",
	}
	result := config.WithDefaults()

	// Verify ANTHROPIC_AUTH_TOKEN is set correctly (not ANTHROPIC_API_KEY)
	if result.Backend.Env == nil {
		t.Fatal("expected Backend.Env to be initialized")
	}

	authToken, ok := result.Backend.Env["ANTHROPIC_AUTH_TOKEN"]
	if !ok {
		t.Error("expected ANTHROPIC_AUTH_TOKEN to be set in environment")
	}
	if authToken != "sk-ZAFeyOQ06TUW9qKlolQsKFtaePRsZYRzS1YOrYtx5n44X8zE" {
		t.Errorf("expected ANTHROPIC_AUTH_TOKEN to be 'sk-ZAFeyOQ06TUW9qKlolQsKFtaePRsZYRzS1YOrYtx5n44X8zE', got '%s'", authToken)
	}

	// Verify ANTHROPIC_BASE_URL is set correctly
	baseURL, ok := result.Backend.Env["ANTHROPIC_BASE_URL"]
	if !ok {
		t.Error("expected ANTHROPIC_BASE_URL to be set in environment")
	}
	if baseURL != "https://paid.tribiosapi.top/" {
		t.Errorf("expected ANTHROPIC_BASE_URL to be 'https://paid.tribiosapi.top/', got '%s'", baseURL)
	}

	// Verify ANTHROPIC_API_KEY is NOT set (we use AUTH_TOKEN instead)
	if _, ok := result.Backend.Env["ANTHROPIC_API_KEY"]; ok {
		t.Error("ANTHROPIC_API_KEY should not be set, use ANTHROPIC_AUTH_TOKEN instead")
	}

	// Verify ANTHROPIC_AUTH_TOKEN is removed from ClearEnv
	for _, key := range result.Backend.ClearEnv {
		if key == "ANTHROPIC_AUTH_TOKEN" {
			t.Error("ANTHROPIC_AUTH_TOKEN should be removed from ClearEnv when API key is set")
		}
	}
}

func TestClaudeCodeConfigWithAPIKeyOnly(t *testing.T) {
	config := ClaudeCodeConfig{
		APIKey: "test-api-key",
	}
	result := config.WithDefaults()

	// Verify ANTHROPIC_AUTH_TOKEN is set
	if result.Backend.Env == nil {
		t.Fatal("expected Backend.Env to be initialized")
	}

	authToken, ok := result.Backend.Env["ANTHROPIC_AUTH_TOKEN"]
	if !ok {
		t.Error("expected ANTHROPIC_AUTH_TOKEN to be set in environment")
	}
	if authToken != "test-api-key" {
		t.Errorf("expected ANTHROPIC_AUTH_TOKEN to be 'test-api-key', got '%s'", authToken)
	}

	// Verify ANTHROPIC_BASE_URL is NOT set when not provided
	if _, ok := result.Backend.Env["ANTHROPIC_BASE_URL"]; ok {
		t.Error("ANTHROPIC_BASE_URL should not be set when BaseURL is empty")
	}
}

func TestClaudeCodeConfigWithBaseURLOnly(t *testing.T) {
	config := ClaudeCodeConfig{
		BaseURL: "https://custom.api.com/",
	}
	result := config.WithDefaults()

	// Verify ANTHROPIC_BASE_URL is set
	if result.Backend.Env == nil {
		t.Fatal("expected Backend.Env to be initialized")
	}

	baseURL, ok := result.Backend.Env["ANTHROPIC_BASE_URL"]
	if !ok {
		t.Error("expected ANTHROPIC_BASE_URL to be set in environment")
	}
	if baseURL != "https://custom.api.com/" {
		t.Errorf("expected ANTHROPIC_BASE_URL to be 'https://custom.api.com/', got '%s'", baseURL)
	}

	// Verify ANTHROPIC_AUTH_TOKEN is NOT set when not provided
	if _, ok := result.Backend.Env["ANTHROPIC_AUTH_TOKEN"]; ok {
		t.Error("ANTHROPIC_AUTH_TOKEN should not be set when APIKey is empty")
	}

	// Verify ClearEnv still contains auth-related keys when no API key is set
	hasAuthToken := false
	for _, key := range result.Backend.ClearEnv {
		if key == "ANTHROPIC_AUTH_TOKEN" {
			hasAuthToken = true
			break
		}
	}
	if !hasAuthToken {
		t.Error("ANTHROPIC_AUTH_TOKEN should remain in ClearEnv when no API key is set")
	}
}

func TestClaudeCodeConfigClearEnvRemovesAuthKeys(t *testing.T) {
	config := ClaudeCodeConfig{
		APIKey: "test-key",
	}
	result := config.WithDefaults()

	// Verify both ANTHROPIC_API_KEY and ANTHROPIC_AUTH_TOKEN are removed from ClearEnv
	for _, key := range result.Backend.ClearEnv {
		if key == "ANTHROPIC_API_KEY" {
			t.Error("ANTHROPIC_API_KEY should be removed from ClearEnv when API key is set")
		}
		if key == "ANTHROPIC_AUTH_TOKEN" {
			t.Error("ANTHROPIC_AUTH_TOKEN should be removed from ClearEnv when API key is set")
		}
	}

	// Verify ANTHROPIC_API_KEY_OLD is still in ClearEnv (we don't use it)
	hasOldKey := false
	for _, key := range result.Backend.ClearEnv {
		if key == "ANTHROPIC_API_KEY_OLD" {
			hasOldKey = true
			break
		}
	}
	if !hasOldKey {
		t.Error("ANTHROPIC_API_KEY_OLD should remain in ClearEnv")
	}
}
