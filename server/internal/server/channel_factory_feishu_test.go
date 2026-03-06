package server

import (
	"testing"

	"go.uber.org/zap"
)

func TestChannelFactory_CreateFeishu_TypingIndicatorConfig(t *testing.T) {
	f := NewChannelFactory(zap.NewNop())

	tests := []struct {
		name    string
		config  map[string]string
		enabled bool
	}{
		{
			name: "default enabled when unset",
			config: map[string]string{
				"app_id":     "app-id",
				"app_secret": "app-secret",
			},
			enabled: true,
		},
		{
			name: "typing_indicator false disables typing reaction",
			config: map[string]string{
				"app_id":                "app-id",
				"app_secret":            "app-secret",
				"typing_indicator":      "false",
				"typingIndicator":       "true",
				"disableTypingReaction": "true",
			},
			enabled: false,
		},
		{
			name: "typingIndicator false disables typing reaction",
			config: map[string]string{
				"app_id":          "app-id",
				"app_secret":      "app-secret",
				"typingIndicator": "false",
			},
			enabled: false,
		},
		{
			name: "disable_typing_reaction true disables typing reaction",
			config: map[string]string{
				"app_id":                  "app-id",
				"app_secret":              "app-secret",
				"disable_typing_reaction": "true",
			},
			enabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch, err := f.CreateChannel(&ChannelConfig{
				ID:      "feishu",
				Enabled: true,
				Config:  tt.config,
			})
			if err != nil {
				t.Fatalf("CreateChannel failed: %v", err)
			}
			if ch == nil {
				t.Fatal("CreateChannel returned nil channel")
			}
			info := ch.Info()
			raw, ok := info.Metadata["typing_reaction_enabled"]
			if !ok {
				t.Fatalf("typing_reaction_enabled missing in metadata: %+v", info.Metadata)
			}
			got, ok := raw.(bool)
			if !ok {
				t.Fatalf("typing_reaction_enabled should be bool, got %T", raw)
			}
			if got != tt.enabled {
				t.Fatalf("typing_reaction_enabled = %v, want %v", got, tt.enabled)
			}
		})
	}
}
