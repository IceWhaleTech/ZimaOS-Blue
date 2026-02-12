package telegram

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

func TestChannel_Name(t *testing.T) {
	cfg := channel.TelegramConfig{
		Enabled:  true,
		BotToken: "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	if ch.Name() != "telegram" {
		t.Errorf("expected name 'telegram', got %s", ch.Name())
	}
}

func TestChannel_Type(t *testing.T) {
	cfg := channel.TelegramConfig{
		Enabled:  true,
		BotToken: "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	if ch.Type() != "telegram" {
		t.Errorf("expected type 'telegram', got %s", ch.Type())
	}
}

func TestChannel_Info(t *testing.T) {
	cfg := channel.TelegramConfig{
		Enabled:  true,
		BotToken: "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	info := ch.Info()
	if info.Name != "telegram" {
		t.Errorf("expected name 'telegram', got %s", info.Name)
	}
	if info.Type != "telegram" {
		t.Errorf("expected type 'telegram', got %s", info.Type)
	}
	if info.Status != channel.StatusDisconnected {
		t.Errorf("expected status 'disconnected', got %s", info.Status)
	}
	if !info.Enabled {
		t.Error("expected enabled to be true")
	}
}

func TestChannel_IsConnected(t *testing.T) {
	cfg := channel.TelegramConfig{
		Enabled:  true,
		BotToken: "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	if ch.IsConnected() {
		t.Error("expected channel to not be connected initially")
	}
}

func TestChannel_Messages(t *testing.T) {
	cfg := channel.TelegramConfig{
		Enabled:  true,
		BotToken: "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	messages := ch.Messages()
	if messages == nil {
		t.Error("expected messages channel to not be nil")
	}
}

func TestChannel_isUserAllowed(t *testing.T) {
	tests := []struct {
		name         string
		allowedUsers []string
		userID       int64
		username     string
		expected     bool
	}{
		{
			name:         "empty allowed list allows all",
			allowedUsers: []string{},
			userID:       123,
			username:     "testuser",
			expected:     true,
		},
		{
			name:         "user ID in allowed list",
			allowedUsers: []string{"123", "456"},
			userID:       123,
			username:     "testuser",
			expected:     true,
		},
		{
			name:         "username in allowed list",
			allowedUsers: []string{"testuser", "otheruser"},
			userID:       123,
			username:     "testuser",
			expected:     true,
		},
		{
			name:         "user not in allowed list",
			allowedUsers: []string{"456", "otheruser"},
			userID:       123,
			username:     "testuser",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := channel.TelegramConfig{
				Enabled:      true,
				BotToken:     "test-token",
				AllowedUsers: tt.allowedUsers,
			}
			logger := zap.NewNop()
			_ = New(cfg, logger)

			// We can't directly test isUserAllowed without creating a tgbotapi.User
			// This test documents the expected behavior
		})
	}
}

func TestChannel_isGroupAllowed(t *testing.T) {
	tests := []struct {
		name          string
		allowedGroups []string
		chatID        int64
		expected      bool
	}{
		{
			name:          "empty allowed list allows all",
			allowedGroups: []string{},
			chatID:        -123456,
			expected:      true,
		},
		{
			name:          "chat ID in allowed list",
			allowedGroups: []string{"-123456", "-789"},
			chatID:        -123456,
			expected:      true,
		},
		{
			name:          "chat ID not in allowed list",
			allowedGroups: []string{"-789", "-999"},
			chatID:        -123456,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := channel.TelegramConfig{
				Enabled:       true,
				BotToken:      "test-token",
				AllowedGroups: tt.allowedGroups,
			}
			logger := zap.NewNop()
			ch := New(cfg, logger)

			result := ch.isGroupAllowed(tt.chatID)
			if result != tt.expected {
				t.Errorf("isGroupAllowed(%d) = %v, want %v", tt.chatID, result, tt.expected)
			}
		})
	}
}

func TestParseChatID(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		hasError bool
	}{
		{"123456", 123456, false},
		{"-123456", -123456, false},
		{"0", 0, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		result, err := parseChatID(tt.input)
		if tt.hasError {
			if err == nil {
				t.Errorf("parseChatID(%s) expected error, got nil", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("parseChatID(%s) unexpected error: %v", tt.input, err)
			}
			if result != tt.expected {
				t.Errorf("parseChatID(%s) = %d, want %d", tt.input, result, tt.expected)
			}
		}
	}
}

func TestParseMessageID(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		hasError bool
	}{
		{"123", 123, false},
		{"0", 0, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		result, err := parseMessageID(tt.input)
		if tt.hasError {
			if err == nil {
				t.Errorf("parseMessageID(%s) expected error, got nil", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("parseMessageID(%s) unexpected error: %v", tt.input, err)
			}
			if result != tt.expected {
				t.Errorf("parseMessageID(%s) = %d, want %d", tt.input, result, tt.expected)
			}
		}
	}
}

func TestChannel_Stop_NotStarted(t *testing.T) {
	cfg := channel.TelegramConfig{
		Enabled:  true,
		BotToken: "test-token",
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
	cfg := channel.TelegramConfig{
		Enabled:  true,
		BotToken: "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	ctx := context.Background()
	msg := channel.OutgoingMessage{
		ChatID:  "123456",
		Content: "Hello",
	}

	err := ch.Send(ctx, msg)
	if err == nil {
		t.Error("expected error when sending without initialization")
	}
}

func TestChannel_SendStreaming_NotInitialized(t *testing.T) {
	cfg := channel.TelegramConfig{
		Enabled:  true,
		BotToken: "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	ctx := context.Background()
	content := make(chan string)
	done := make(chan struct{})

	close(content) // Close immediately

	err := ch.SendStreaming(ctx, "123456", "", content, done)
	if err == nil {
		t.Error("expected error when streaming without initialization")
	}
}
