package server

import (
	"testing"

	"go.uber.org/zap"
)

func TestChannelFactory_CreateFeishu_TypingIndicatorConfig(t *testing.T) {
	f := NewChannelFactory(zap.NewNop())

	tests := []struct {
		name              string
		config            map[string]string
		typingEnabled     bool
		expectedReplyMode string
	}{
		{
			name: "default enabled when unset",
			config: map[string]string{
				"app_id":     "app-id",
				"app_secret": "app-secret",
			},
			typingEnabled:     true,
			expectedReplyMode: "reply",
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
			typingEnabled:     false,
			expectedReplyMode: "reply",
		},
		{
			name: "typingIndicator false disables typing reaction",
			config: map[string]string{
				"app_id":          "app-id",
				"app_secret":      "app-secret",
				"typingIndicator": "false",
			},
			typingEnabled:     false,
			expectedReplyMode: "reply",
		},
		{
			name: "disable_typing_reaction true disables typing reaction",
			config: map[string]string{
				"app_id":                  "app-id",
				"app_secret":              "app-secret",
				"disable_typing_reaction": "true",
			},
			typingEnabled:     false,
			expectedReplyMode: "reply",
		},
		{
			name: "session_mode true switches to session reply mode",
			config: map[string]string{
				"app_id":       "app-id",
				"app_secret":   "app-secret",
				"session_mode": "true",
			},
			typingEnabled:     true,
			expectedReplyMode: "session",
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
			if got != tt.typingEnabled {
				t.Fatalf("typing_reaction_enabled = %v, want %v", got, tt.typingEnabled)
			}

			replyMode, ok := info.Metadata["reply_mode"].(string)
			if !ok {
				t.Fatalf("reply_mode should be string, got %T", info.Metadata["reply_mode"])
			}
			if replyMode != tt.expectedReplyMode {
				t.Fatalf("reply_mode = %q, want %q", replyMode, tt.expectedReplyMode)
			}
		})
	}
}
