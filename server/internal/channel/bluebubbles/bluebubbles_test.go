package bluebubbles

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

func TestNew(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.BlueBubblesConfig{
		Enabled:   true,
		ServerURL: "http://localhost:1234",
		Password:  "test-password",
	}

	ch := New(cfg, logger)

	if ch == nil {
		t.Fatal("expected channel to be created")
	}
	if ch.Name() != "bluebubbles" {
		t.Errorf("expected name 'bluebubbles', got '%s'", ch.Name())
	}
	if ch.Type() != "bluebubbles" {
		t.Errorf("expected type 'bluebubbles', got '%s'", ch.Type())
	}
}

func TestChannel_Info(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.BlueBubblesConfig{
		Enabled:   true,
		ServerURL: "http://localhost:1234",
		Password:  "test-password",
	}

	ch := New(cfg, logger)
	info := ch.Info()

	if info.Name != "bluebubbles" {
		t.Errorf("expected name 'bluebubbles', got '%s'", info.Name)
	}
	if info.Type != "bluebubbles" {
		t.Errorf("expected type 'bluebubbles', got '%s'", info.Type)
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
	cfg := channel.BlueBubblesConfig{
		Enabled:   true,
		ServerURL: "http://localhost:1234",
		Password:  "test-password",
	}

	ch := New(cfg, logger)

	if ch.IsConnected() {
		t.Error("expected channel to not be connected initially")
	}
}

func TestChannel_Messages(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.BlueBubblesConfig{
		Enabled:   true,
		ServerURL: "http://localhost:1234",
		Password:  "test-password",
	}

	ch := New(cfg, logger)
	messages := ch.Messages()

	if messages == nil {
		t.Error("expected messages channel to not be nil")
	}
}

func TestChannel_Stop_NotStarted(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.BlueBubblesConfig{
		Enabled:   true,
		ServerURL: "http://localhost:1234",
		Password:  "test-password",
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
	// Create mock BlueBubbles server
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/server/info":
			response := map[string]interface{}{
				"status": 200,
				"data": map[string]interface{}{
					"os_version":       "13.0",
					"server_version":   "1.9.0",
					"private_api":      true,
					"helper_connected": true,
				},
			}
			json.NewEncoder(w).Encode(response)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	logger := zap.NewNop()
	cfg := channel.BlueBubblesConfig{
		Enabled:   true,
		ServerURL: server.URL,
		Password:  "test-password",
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
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		response := map[string]interface{}{
			"status":  401,
			"message": "Unauthorized",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	logger := zap.NewNop()
	cfg := channel.BlueBubblesConfig{
		Enabled:   true,
		ServerURL: server.URL,
		Password:  "wrong-password",
	}

	ch := New(cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ch.Start(ctx)
	if err == nil {
		t.Fatal("expected error starting channel with invalid password")
	}

	if ch.IsConnected() {
		t.Error("expected channel to not be connected after auth failure")
	}
}

func TestChannel_Send_NotInitialized(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.BlueBubblesConfig{
		Enabled:   true,
		ServerURL: "http://localhost:1234",
		Password:  "test-password",
	}

	ch := New(cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ch.Send(ctx, channel.OutgoingMessage{
		ChatID:  "test-chat",
		Content: "Hello",
	})

	if err == nil {
		t.Error("expected error sending message when not initialized")
	}
}

func TestChannel_SendStreaming_NotInitialized(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.BlueBubblesConfig{
		Enabled:   true,
		ServerURL: "http://localhost:1234",
		Password:  "test-password",
	}

	ch := New(cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	content := make(chan string)
	done := make(chan struct{})

	err := ch.SendStreaming(ctx, "test-chat", "", content, done)

	if err == nil {
		t.Error("expected error sending streaming message when not initialized")
	}
}

func TestChannel_isChatAllowed(t *testing.T) {
	tests := []struct {
		name         string
		allowedChats []string
		chatGUID     string
		expected     bool
	}{
		{
			name:         "empty allowed list allows all",
			allowedChats: []string{},
			chatGUID:     "any-chat",
			expected:     true,
		},
		{
			name:         "chat GUID in allowed list",
			allowedChats: []string{"chat-1", "chat-2"},
			chatGUID:     "chat-1",
			expected:     true,
		},
		{
			name:         "chat GUID not in allowed list",
			allowedChats: []string{"chat-1", "chat-2"},
			chatGUID:     "chat-3",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			cfg := channel.BlueBubblesConfig{
				Enabled:      true,
				ServerURL:    "http://localhost:1234",
				Password:     "test-password",
				AllowedChats: tt.allowedChats,
			}

			ch := New(cfg, logger)
			result := ch.isChatAllowed(tt.chatGUID)

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
