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

func TestChannel_HandleWebhook_NilDataDoesNotPanic(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.BlueBubblesConfig{
		Enabled:   true,
		ServerURL: "http://localhost:1234",
		Password:  "test-password",
	}

	ch := New(cfg, logger)
	ch.HandleWebhook(&WebhookEvent{Type: "new-message"})

	select {
	case msg := <-ch.Messages():
		t.Fatalf("unexpected message: %#v", msg)
	default:
	}
}

func TestChannel_convertMessage_PreservesVisibleMetadata(t *testing.T) {
	ch := New(channel.BlueBubblesConfig{}, zap.NewNop())

	msg := ch.convertMessage(&MessageData{
		GUID:        "msg-1",
		ChatGUID:    "chat-1",
		Handle:      "+15551234567",
		Text:        "",
		Subject:     "Vacation photo",
		Service:     "iMessage",
		IsFromMe:    false,
		DateCreated: 1700000000000,
		DateRead:    1700000005000,
		Attachments: []AttachmentData{{
			GUID:         "att-1",
			TransferName: "beach.png",
			MimeType:     "image/png",
			TotalBytes:   12345,
		}},
		Chats: []ChatData{{
			GUID:         "chat-1",
			DisplayName:  "Family",
			Participants: 3,
		}},
	})

	if msg.Content != "Vacation photo" {
		t.Fatalf("Content = %q, want subject fallback", msg.Content)
	}
	if msg.Type != channel.MessageTypeText {
		t.Fatalf("Type = %q, want text when fallback content exists", msg.Type)
	}
	if !msg.IsGroup || msg.GroupName != "Family" {
		t.Fatalf("group = (%v, %q), want (true, Family)", msg.IsGroup, msg.GroupName)
	}
	if len(msg.Attachments) != 1 {
		t.Fatalf("Attachments len = %d, want 1", len(msg.Attachments))
	}
	if msg.Attachments[0].Type != channel.MessageTypeImage {
		t.Fatalf("attachment type = %q, want image", msg.Attachments[0].Type)
	}
	if msg.Attachments[0].Size != 12345 {
		t.Fatalf("attachment size = %d, want 12345", msg.Attachments[0].Size)
	}
	if got := msg.Metadata["subject"]; got != "Vacation photo" {
		t.Fatalf("subject metadata = %#v", got)
	}
	if got := msg.Metadata["date_read"]; got != int64(1700000005000) {
		t.Fatalf("date_read metadata = %#v", got)
	}
	if got := msg.Metadata["attachment_count"]; got != 1 {
		t.Fatalf("attachment_count = %#v", got)
	}
	rawAttachments, ok := msg.Metadata["attachments"].([]map[string]interface{})
	if !ok {
		t.Fatalf("attachments metadata type = %T", msg.Metadata["attachments"])
	}
	if len(rawAttachments) != 1 {
		t.Fatalf("raw attachments len = %d, want 1", len(rawAttachments))
	}
	if got := rawAttachments[0]["transfer_name"]; got != "beach.png" {
		t.Fatalf("transfer_name = %#v", got)
	}
	if got := rawAttachments[0]["total_bytes"]; got != int64(12345) {
		t.Fatalf("total_bytes = %#v", got)
	}
}
