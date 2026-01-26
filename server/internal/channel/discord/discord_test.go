package discord

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel"
)

func TestChannel_Name(t *testing.T) {
	cfg := channel.DiscordConfig{
		Enabled:  true,
		BotToken: "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	if ch.Name() != "discord" {
		t.Errorf("expected name 'discord', got %s", ch.Name())
	}
}

func TestChannel_Type(t *testing.T) {
	cfg := channel.DiscordConfig{
		Enabled:  true,
		BotToken: "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	if ch.Type() != "discord" {
		t.Errorf("expected type 'discord', got %s", ch.Type())
	}
}

func TestChannel_Info(t *testing.T) {
	cfg := channel.DiscordConfig{
		Enabled:  true,
		BotToken: "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	info := ch.Info()
	if info.Name != "discord" {
		t.Errorf("expected name 'discord', got %s", info.Name)
	}
	if info.Type != "discord" {
		t.Errorf("expected type 'discord', got %s", info.Type)
	}
	if info.Status != channel.StatusDisconnected {
		t.Errorf("expected status 'disconnected', got %s", info.Status)
	}
	if !info.Enabled {
		t.Error("expected enabled to be true")
	}
}

func TestChannel_IsConnected(t *testing.T) {
	cfg := channel.DiscordConfig{
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
	cfg := channel.DiscordConfig{
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

func TestChannel_isGuildAllowed(t *testing.T) {
	tests := []struct {
		name          string
		allowedGuilds []string
		guildID       string
		expected      bool
	}{
		{
			name:          "empty allowed list allows all",
			allowedGuilds: []string{},
			guildID:       "123456",
			expected:      true,
		},
		{
			name:          "guild ID in allowed list",
			allowedGuilds: []string{"123456", "789"},
			guildID:       "123456",
			expected:      true,
		},
		{
			name:          "guild ID not in allowed list",
			allowedGuilds: []string{"789", "999"},
			guildID:       "123456",
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := channel.DiscordConfig{
				Enabled:       true,
				BotToken:      "test-token",
				AllowedGuilds: tt.allowedGuilds,
			}
			logger := zap.NewNop()
			ch := New(cfg, logger)

			result := ch.isGuildAllowed(tt.guildID)
			if result != tt.expected {
				t.Errorf("isGuildAllowed(%s) = %v, want %v", tt.guildID, result, tt.expected)
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
			userID:       "123456",
			expected:     true,
		},
		{
			name:         "user ID in allowed list",
			allowedUsers: []string{"123456", "789"},
			userID:       "123456",
			expected:     true,
		},
		{
			name:         "user ID not in allowed list",
			allowedUsers: []string{"789", "999"},
			userID:       "123456",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := channel.DiscordConfig{
				Enabled:      true,
				BotToken:     "test-token",
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
	cfg := channel.DiscordConfig{
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
	cfg := channel.DiscordConfig{
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
	cfg := channel.DiscordConfig{
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
