package providers

import (
	"testing"
)

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty key",
			input:    "",
			expected: "",
		},
		{
			name:     "short key",
			input:    "abc",
			expected: "***",
		},
		{
			name:     "8 char key",
			input:    "12345678",
			expected: "********",
		},
		{
			name:     "normal key",
			input:    "sk-ant-api03-1234567890abcdef",
			expected: "sk-a*********************cdef",
		},
		{
			name:     "long key",
			input:    "sk-ant-api03-1234567890abcdefghijklmnopqrstuvwxyz",
			expected: "sk-a*****************************************wxyz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskAPIKey(tt.input)
			if result != tt.expected {
				t.Errorf("MaskAPIKey(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestEnvConfig_HasKeys(t *testing.T) {
	config := &EnvConfig{
		AnthropicAPIKey: "test-key",
	}

	if !config.HasAnthropicKey() {
		t.Error("HasAnthropicKey() should return true")
	}

	if config.HasOpenAIKey() {
		t.Error("HasOpenAIKey() should return false")
	}

	if !config.HasAnyAPIKey() {
		t.Error("HasAnyAPIKey() should return true")
	}
}

func TestEnvConfig_GetConfiguredProviders(t *testing.T) {
	config := &EnvConfig{
		AnthropicAPIKey: "test-key",
		OpenAIAPIKey:    "test-key",
	}

	providers := config.GetConfiguredProviders()
	if len(providers) != 2 {
		t.Errorf("GetConfiguredProviders() returned %d providers, want 2", len(providers))
	}

	// Check that both providers are present
	hasAnthropic := false
	hasOpenAI := false
	for _, p := range providers {
		if p == "anthropic" {
			hasAnthropic = true
		}
		if p == "openai" {
			hasOpenAI = true
		}
	}

	if !hasAnthropic {
		t.Error("GetConfiguredProviders() should include anthropic")
	}
	if !hasOpenAI {
		t.Error("GetConfiguredProviders() should include openai")
	}
}

func TestEnvConfig_ToDisplay(t *testing.T) {
	config := &EnvConfig{
		AnthropicAPIKey: "sk-ant-api03-1234567890",
		OpenAIAPIKey:    "",
		OpenAIBaseURL:   "https://api.openai.com",
	}

	display := config.ToDisplay()

	if display.AnthropicAPIKey == config.AnthropicAPIKey {
		t.Error("ToDisplay() should mask the API key")
	}

	if !display.HasAnthropicKey {
		t.Error("ToDisplay() should set HasAnthropicKey to true")
	}

	if display.HasOpenAIKey {
		t.Error("ToDisplay() should set HasOpenAIKey to false")
	}

	if display.OpenAIBaseURL != config.OpenAIBaseURL {
		t.Error("ToDisplay() should preserve non-sensitive values")
	}
}
