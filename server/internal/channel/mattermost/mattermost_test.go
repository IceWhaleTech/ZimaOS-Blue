package mattermost

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

func TestNew(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.MattermostConfig{
		Enabled:   true,
		ServerURL: "https://mattermost.example.com",
		BotToken:  "test-bot-token",
	}

	ch := New(cfg, logger)

	if ch == nil {
		t.Fatal("expected channel to be created")
	}
	if ch.Name() != "mattermost" {
		t.Errorf("expected name 'mattermost', got '%s'", ch.Name())
	}
	if ch.Type() != "mattermost" {
		t.Errorf("expected type 'mattermost', got '%s'", ch.Type())
	}
}

func TestChannel_Info(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.MattermostConfig{
		Enabled:   true,
		ServerURL: "https://mattermost.example.com",
		BotToken:  "test-bot-token",
	}

	ch := New(cfg, logger)
	info := ch.Info()

	if info.Name != "mattermost" {
		t.Errorf("expected name 'mattermost', got '%s'", info.Name)
	}
	if info.Type != "mattermost" {
		t.Errorf("expected type 'mattermost', got '%s'", info.Type)
	}
	if info.Status != channel.StatusDisconnected {
		t.Errorf("expected status 'disconnected', got '%s'", info.Status)
	}
	if !info.Enabled {
		t.Error("expected enabled to be true")
	}
}

func TestChannel_IsConnected(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.MattermostConfig{
		Enabled:   true,
		ServerURL: "https://mattermost.example.com",
		BotToken:  "test-bot-token",
	}

	ch := New(cfg, logger)

	if ch.IsConnected() {
		t.Error("expected channel to not be connected initially")
	}
}

func TestChannel_Messages(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.MattermostConfig{
		Enabled:   true,
		ServerURL: "https://mattermost.example.com",
		BotToken:  "test-bot-token",
	}

	ch := New(cfg, logger)
	messages := ch.Messages()

	if messages == nil {
		t.Error("expected messages channel to not be nil")
	}
}

func TestChannel_Stop_NotStarted(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.MattermostConfig{
		Enabled:   true,
		ServerURL: "https://mattermost.example.com",
		BotToken:  "test-bot-token",
	}

	ch := New(cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ch.Stop(ctx)
	if err != nil {
		t.Errorf("expected no error stopping non-started channel, got: %v", err)
	}
}

func TestChannel_Start_Success(t *testing.T) {
	// Create mock Mattermost server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v4/users/me":
			response := map[string]interface{}{
				"id":       "user-123",
				"username": "testbot",
				"email":    "bot@example.com",
			}
			json.NewEncoder(w).Encode(response)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	logger := zap.NewNop()
	cfg := channel.MattermostConfig{
		Enabled:   true,
		ServerURL: server.URL,
		BotToken:  "test-bot-token",
	}

	ch := New(cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ch.Start(ctx)
	if err != nil {
		t.Fatalf("expected no error starting channel, got: %v", err)
	}

	if !ch.IsConnected() {
		t.Error("expected channel to be connected after start")
	}

	// Clean up
	ch.Stop(ctx)
}

func TestChannel_Start_AuthFailure(t *testing.T) {
	// Create mock server that returns unauthorized
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		response := map[string]interface{}{
			"status_code": 401,
			"message":     "Invalid or expired session",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	logger := zap.NewNop()
	cfg := channel.MattermostConfig{
		Enabled:   true,
		ServerURL: server.URL,
		BotToken:  "invalid-token",
	}

	ch := New(cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ch.Start(ctx)
	if err == nil {
		t.Fatal("expected error starting channel with invalid token")
	}

	if ch.IsConnected() {
		t.Error("expected channel to not be connected after auth failure")
	}
}

func TestChannel_Send_NotInitialized(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.MattermostConfig{
		Enabled:   true,
		ServerURL: "https://mattermost.example.com",
		BotToken:  "test-bot-token",
	}

	ch := New(cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ch.Send(ctx, channel.OutgoingMessage{
		ChatID:  "test-channel",
		Content: "Hello",
	})

	if err == nil {
		t.Error("expected error sending message when not initialized")
	}
}

func TestChannel_SendStreaming_NotInitialized(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.MattermostConfig{
		Enabled:   true,
		ServerURL: "https://mattermost.example.com",
		BotToken:  "test-bot-token",
	}

	ch := New(cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	content := make(chan string)
	done := make(chan struct{})

	err := ch.SendStreaming(ctx, "test-channel", "", content, done)

	if err == nil {
		t.Error("expected error sending streaming message when not initialized")
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
			channelID:       "any-channel",
			expected:        true,
		},
		{
			name:            "channel ID in allowed list",
			allowedChannels: []string{"channel-1", "channel-2"},
			channelID:       "channel-1",
			expected:        true,
		},
		{
			name:            "channel ID not in allowed list",
			allowedChannels: []string{"channel-1", "channel-2"},
			channelID:       "channel-3",
			expected:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			cfg := channel.MattermostConfig{
				Enabled:         true,
				ServerURL:       "https://mattermost.example.com",
				BotToken:        "test-bot-token",
				AllowedChannels: tt.allowedChannels,
			}

			ch := New(cfg, logger)
			result := ch.isChannelAllowed(tt.channelID)

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
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
			userID:       "any-user",
			expected:     true,
		},
		{
			name:         "user ID in allowed list",
			allowedUsers: []string{"user-1", "user-2"},
			userID:       "user-1",
			expected:     true,
		},
		{
			name:         "user ID not in allowed list",
			allowedUsers: []string{"user-1", "user-2"},
			userID:       "user-3",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			cfg := channel.MattermostConfig{
				Enabled:      true,
				ServerURL:    "https://mattermost.example.com",
				BotToken:     "test-bot-token",
				AllowedUsers: tt.allowedUsers,
			}

			ch := New(cfg, logger)
			result := ch.isUserAllowed(tt.userID)

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
