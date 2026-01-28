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

	if backend.Output != "json" {
		t.Errorf("expected output 'json', got '%s'", backend.Output)
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

	if result.Output != "json" {
		t.Errorf("expected output 'json', got '%s'", result.Output)
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
