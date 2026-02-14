package matrix

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

func TestChannel_Name(t *testing.T) {
	cfg := channel.MatrixConfig{
		Enabled:     true,
		Homeserver:  "https://matrix.org",
		UserID:      "@bot:matrix.org",
		AccessToken: "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	if ch.Name() != "matrix" {
		t.Errorf("expected name 'matrix', got %s", ch.Name())
	}
}

func TestChannel_Type(t *testing.T) {
	cfg := channel.MatrixConfig{
		Enabled:     true,
		Homeserver:  "https://matrix.org",
		UserID:      "@bot:matrix.org",
		AccessToken: "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	if ch.Type() != "matrix" {
		t.Errorf("expected type 'matrix', got %s", ch.Type())
	}
}

func TestChannel_Info(t *testing.T) {
	cfg := channel.MatrixConfig{
		Enabled:     true,
		Homeserver:  "https://matrix.org",
		UserID:      "@bot:matrix.org",
		AccessToken: "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	info := ch.Info()
	if info.Name != "matrix" {
		t.Errorf("expected name 'matrix', got %s", info.Name)
	}
	if info.Type != "matrix" {
		t.Errorf("expected type 'matrix', got %s", info.Type)
	}
	if info.Status != channel.StatusDisconnected {
		t.Errorf("expected status 'disconnected', got %s", info.Status)
	}
	if !info.Enabled {
		t.Error("expected enabled to be true")
	}
	if info.Metadata["homeserver"] != "https://matrix.org" {
		t.Errorf("expected homeserver 'https://matrix.org', got %v", info.Metadata["homeserver"])
	}
	if info.Metadata["user_id"] != "@bot:matrix.org" {
		t.Errorf("expected user_id '@bot:matrix.org', got %v", info.Metadata["user_id"])
	}
}

func TestChannel_IsConnected(t *testing.T) {
	cfg := channel.MatrixConfig{
		Enabled:     true,
		Homeserver:  "https://matrix.org",
		UserID:      "@bot:matrix.org",
		AccessToken: "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	if ch.IsConnected() {
		t.Error("expected channel to not be connected initially")
	}
}

func TestChannel_Messages(t *testing.T) {
	cfg := channel.MatrixConfig{
		Enabled:     true,
		Homeserver:  "https://matrix.org",
		UserID:      "@bot:matrix.org",
		AccessToken: "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	messages := ch.Messages()
	if messages == nil {
		t.Error("expected messages channel to not be nil")
	}
}

func TestChannel_isRoomAllowed(t *testing.T) {
	tests := []struct {
		name         string
		allowedRooms []string
		roomID       string
		expected     bool
	}{
		{
			name:         "empty allowed list allows all",
			allowedRooms: []string{},
			roomID:       "!room:matrix.org",
			expected:     true,
		},
		{
			name:         "room ID in allowed list",
			allowedRooms: []string{"!room:matrix.org", "!other:matrix.org"},
			roomID:       "!room:matrix.org",
			expected:     true,
		},
		{
			name:         "room ID not in allowed list",
			allowedRooms: []string{"!other:matrix.org", "!another:matrix.org"},
			roomID:       "!room:matrix.org",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := channel.MatrixConfig{
				Enabled:      true,
				Homeserver:   "https://matrix.org",
				UserID:       "@bot:matrix.org",
				AccessToken:  "test-token",
				AllowedRooms: tt.allowedRooms,
			}
			logger := zap.NewNop()
			ch := New(cfg, logger)

			result := ch.isRoomAllowed(tt.roomID)
			if result != tt.expected {
				t.Errorf("isRoomAllowed(%s) = %v, want %v", tt.roomID, result, tt.expected)
			}
		})
	}
}

func TestChannel_Stop_NotStarted(t *testing.T) {
	cfg := channel.MatrixConfig{
		Enabled:     true,
		Homeserver:  "https://matrix.org",
		UserID:      "@bot:matrix.org",
		AccessToken: "test-token",
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
	cfg := channel.MatrixConfig{
		Enabled:     true,
		Homeserver:  "https://matrix.org",
		UserID:      "@bot:matrix.org",
		AccessToken: "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	ctx := context.Background()
	msg := channel.OutgoingMessage{
		ChatID:  "!room:matrix.org",
		Content: "Hello",
	}

	err := ch.Send(ctx, msg)
	if err == nil {
		t.Error("expected error when sending without initialization")
	}
}

func TestChannel_SendStreaming_NotInitialized(t *testing.T) {
	cfg := channel.MatrixConfig{
		Enabled:     true,
		Homeserver:  "https://matrix.org",
		UserID:      "@bot:matrix.org",
		AccessToken: "test-token",
	}
	logger := zap.NewNop()
	ch := New(cfg, logger)

	ctx := context.Background()
	content := make(chan string)
	done := make(chan struct{})

	close(content) // Close immediately

	err := ch.SendStreaming(ctx, "!room:matrix.org", "", content, done)
	if err == nil {
		t.Error("expected error when streaming without initialization")
	}
}

func TestMarkdownToHTML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"**bold**", "<strong>bold<strong>"},
		{"*italic*", "<em>italic<em>"},
		{"`code`", "<code>code<code>"},
		{"plain text", "plain text"},
	}

	for _, tt := range tests {
		result := markdownToHTML(tt.input)
		if result != tt.expected {
			t.Errorf("markdownToHTML(%s) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}
