package slack

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

func TestChannel_Name(t *testing.T) {
	cfg := channel.SlackConfig{
		Enabled:  true,
		BotToken: "xoxb-test-token",
		AppToken: "xapp-test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	if ch.Name() != "slack" {
		t.Errorf("expected name 'slack', got %s", ch.Name())
	}
}

func TestChannel_Type(t *testing.T) {
	cfg := channel.SlackConfig{
		Enabled:  true,
		BotToken: "xoxb-test-token",
		AppToken: "xapp-test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	if ch.Type() != "slack" {
		t.Errorf("expected type 'slack', got %s", ch.Type())
	}
}

func TestChannel_Info(t *testing.T) {
	cfg := channel.SlackConfig{
		Enabled:  true,
		BotToken: "xoxb-test-token",
		AppToken: "xapp-test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	info := ch.Info()
	if info.Name != "slack" {
		t.Errorf("expected name 'slack', got %s", info.Name)
	}
	if info.Type != "slack" {
		t.Errorf("expected type 'slack', got %s", info.Type)
	}
	if info.Status != channel.StatusDisconnected {
		t.Errorf("expected status 'disconnected', got %s", info.Status)
	}
	if !info.Enabled {
		t.Error("expected enabled to be true")
	}
}

func TestChannel_IsConnected(t *testing.T) {
	cfg := channel.SlackConfig{
		Enabled:  true,
		BotToken: "xoxb-test-token",
		AppToken: "xapp-test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	if ch.IsConnected() {
		t.Error("expected channel to not be connected initially")
	}
}

func TestChannel_Messages(t *testing.T) {
	cfg := channel.SlackConfig{
		Enabled:  true,
		BotToken: "xoxb-test-token",
		AppToken: "xapp-test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	messages := ch.Messages()
	if messages == nil {
		t.Error("expected messages channel to not be nil")
	}
}

func TestChannel_isChannelAllowed(t *testing.T) {
	tests := []struct {
		name            string
		allowedChannels []string
		channelID       string
		expected        bool
	}{
		{
			name:            "empty allowed list allows all",
			allowedChannels: []string{},
			channelID:       "C123456",
			expected:        true,
		},
		{
			name:            "channel ID in allowed list",
			allowedChannels: []string{"C123456", "C789"},
			channelID:       "C123456",
			expected:        true,
		},
		{
			name:            "channel ID not in allowed list",
			allowedChannels: []string{"C789", "C999"},
			channelID:       "C123456",
			expected:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := channel.SlackConfig{
				Enabled:         true,
				BotToken:        "xoxb-test-token",
				AppToken:        "xapp-test-token",
				AllowedChannels: tt.allowedChannels,
			}
			logger := zap.NewNop()
			ch := New(cfg, logger)

			result := ch.isChannelAllowed(tt.channelID)
			if result != tt.expected {
				t.Errorf("isChannelAllowed(%s) = %v, want %v", tt.channelID, result, tt.expected)
			}
		})
	}
}

func TestChannel_isUserAllowed(t *testing.T) {
	tests := []struct {
		name         string
		allowedUsers []string
		userID       string
		expected     bool
	}{
		{
			name:         "empty allowed list allows all",
			allowedUsers: []string{},
			userID:       "U123456",
			expected:     true,
		},
		{
			name:         "user ID in allowed list",
			allowedUsers: []string{"U123456", "U789"},
			userID:       "U123456",
			expected:     true,
		},
		{
			name:         "user ID not in allowed list",
			allowedUsers: []string{"U789", "U999"},
			userID:       "U123456",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := channel.SlackConfig{
				Enabled:      true,
				BotToken:     "xoxb-test-token",
				AppToken:     "xapp-test-token",
				AllowedUsers: tt.allowedUsers,
			}
			logger := zap.NewNop()
			ch := New(cfg, logger)

			result := ch.isUserAllowed(tt.userID)
			if result != tt.expected {
				t.Errorf("isUserAllowed(%s) = %v, want %v", tt.userID, result, tt.expected)
			}
		})
	}
}

func TestChannel_Stop_NotStarted(t *testing.T) {
	cfg := channel.SlackConfig{
		Enabled:  true,
		BotToken: "xoxb-test-token",
		AppToken: "xapp-test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Stopping a channel that was never started should not error
	err := ch.Stop(ctx)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestChannel_Send_NotInitialized(t *testing.T) {
	cfg := channel.SlackConfig{
		Enabled:  true,
		BotToken: "xoxb-test-token",
		AppToken: "xapp-test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	ctx := context.Background()
	msg := channel.OutgoingMessage{
		ChatID:  "C123456",
		Content: "Hello",
	}

	err := ch.Send(ctx, msg)
	if err == nil {
		t.Error("expected error when sending without initialization")
	}
}

func TestChannel_SendStreaming_NotInitialized(t *testing.T) {
	cfg := channel.SlackConfig{
		Enabled:  true,
		BotToken: "xoxb-test-token",
		AppToken: "xapp-test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	ctx := context.Background()
	content := make(chan string)
	done := make(chan struct{})

	close(content) // Close immediately

	err := ch.SendStreaming(ctx, "C123456", "", content, done)
	if err == nil {
		t.Error("expected error when streaming without initialization")
	}
}

func TestParseSlackTimestamp(t *testing.T) {
	tests := []struct {
		input    string
		expected int64 // Unix timestamp
	}{
		{"1234567890.123456", 1234567890},
		{"1609459200.000000", 1609459200},
		{"invalid", 0}, // Will return current time, so we just check it doesn't panic
	}

	for _, tt := range tests {
		result := parseSlackTimestamp(tt.input)
		if tt.expected != 0 && result.Unix() != tt.expected {
			t.Errorf("parseSlackTimestamp(%s) = %d, want %d", tt.input, result.Unix(), tt.expected)
		}
	}
}
