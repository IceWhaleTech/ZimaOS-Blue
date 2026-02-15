package proxy

import (
	"testing"
)

func TestOriginRegistryResolve(t *testing.T) {
	patterns := map[string]string{
		"claude-*":        "cloud",
		"gpt-*":           "cloud",
		"deepseek-r1*":    "cloud",
		"qwen*":           "local",
		"llama*":          "local",
		"deepseek-coder*": "local",
	}
	reg := NewOriginRegistry(patterns)

	tests := []struct {
		model  string
		origin ModelOrigin
	}{
		{"claude-sonnet-4-20250514", OriginCloud},
		{"claude-3-haiku", OriginCloud},
		{"gpt-4o", OriginCloud},
		{"gpt-4-turbo", OriginCloud},
		{"deepseek-r1-0528", OriginCloud},
		{"qwen2.5-7b-instruct", OriginLocal},
		{"qwen2.5-coder-7b", OriginLocal},
		{"llama-3.1-8b-instruct", OriginLocal},
		{"deepseek-coder-6.7b", OriginLocal},
	}

	for _, tt := range tests {
		got := reg.Resolve(tt.model)
		if got != tt.origin {
			t.Errorf("Resolve(%q) = %s, want %s", tt.model, got, tt.origin)
		}
	}
}

func TestOriginRegistryUnknownModel(t *testing.T) {
	reg := NewOriginRegistry(map[string]string{"claude-*": "cloud"})
	got := reg.Resolve("unknown-model-xyz")
	if got != OriginCloud {
		t.Errorf("unknown model should default to cloud, got %s", got)
	}
}

func TestOriginRegistryEmpty(t *testing.T) {
	reg := NewOriginRegistry(nil)
	got := reg.Resolve("anything")
	if got != OriginCloud {
		t.Errorf("empty registry should default to cloud, got %s", got)
	}
}

func TestOriginRegistryCaseInsensitive(t *testing.T) {
	reg := NewOriginRegistry(map[string]string{"Claude-*": "cloud"})
	got := reg.Resolve("claude-sonnet")
	if got != OriginCloud {
		t.Errorf("expected case-insensitive match, got %s", got)
	}
}
