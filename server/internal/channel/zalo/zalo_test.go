package zalo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

func TestNew(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.ZaloConfig{
		Enabled:     true,
		OAID:        "180789",
		AccessToken: "test-access-token",
	}

	ch := New(cfg, logger)

	if ch == nil {
		t.Fatal("expected channel to be created")
	}
	if ch.Name() != "zalo" {
		t.Errorf("expected name 'zalo', got '%s'", ch.Name())
	}
	if ch.Type() != "zalo" {
		t.Errorf("expected type 'zalo', got '%s'", ch.Type())
	}
}

func TestChannel_Info(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.ZaloConfig{
		Enabled:     true,
		OAID:        "180789",
		AccessToken: "test-access-token",
	}

	ch := New(cfg, logger)
	info := ch.Info()

	if info.Name != "zalo" {
		t.Errorf("expected name 'zalo', got '%s'", info.Name)
	}
	if info.Type != "zalo" {
		t.Errorf("expected type 'zalo', got '%s'", info.Type)
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
	cfg := channel.ZaloConfig{
		Enabled:     true,
		OAID:        "180789",
		AccessToken: "test-access-token",
	}

	ch := New(cfg, logger)

	if ch.IsConnected() {
		t.Error("expected channel to not be connected initially")
	}
}

func TestChannel_Messages(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.ZaloConfig{
		Enabled:     true,
		OAID:        "180789",
		AccessToken: "test-access-token",
	}

	ch := New(cfg, logger)
	messages := ch.Messages()

	if messages == nil {
		t.Error("expected messages channel to not be nil")
	}
}

func TestChannel_Stop_NotStarted(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.ZaloConfig{
		Enabled:     true,
		OAID:        "180789",
		AccessToken: "test-access-token",
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
	// Create mock Zalo server
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2.0/oa/getoa":
			response := map[string]interface{}{
				"error":   0,
				"message": "Success",
				"data": map[string]interface{}{
					"oa_id":       "180789",
					"name":        "Test OA",
					"is_verified": true,
				},
			}
			json.NewEncoder(w).Encode(response)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	logger := zap.NewNop()
	cfg := channel.ZaloConfig{
		Enabled:     true,
		OAID:        "180789",
		AccessToken: "test-access-token",
	}

	ch := NewWithOptions(cfg, logger, server.URL)

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
	// Create mock server that returns error
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"error":   -124,
			"message": "Invalid access token",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	logger := zap.NewNop()
	cfg := channel.ZaloConfig{
		Enabled:     true,
		OAID:        "180789",
		AccessToken: "invalid-token",
	}

	ch := NewWithOptions(cfg, logger, server.URL)

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
	cfg := channel.ZaloConfig{
		Enabled:     true,
		OAID:        "180789",
		AccessToken: "test-access-token",
	}

	ch := New(cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ch.Send(ctx, channel.OutgoingMessage{
		ChatID:  "user-123",
		Content: "Hello",
	})

	if err == nil {
		t.Error("expected error sending message when not initialized")
	}
}

func TestChannel_SendStreaming_NotInitialized(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.ZaloConfig{
		Enabled:     true,
		OAID:        "180789",
		AccessToken: "test-access-token",
	}

	ch := New(cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	content := make(chan string)
	done := make(chan struct{})

	err := ch.SendStreaming(ctx, "user-123", "", content, done)

	if err == nil {
		t.Error("expected error sending streaming message when not initialized")
	}
}

func TestChannel_Send_ImageAttachmentPreservesCaptionAsText(t *testing.T) {
	var (
		mu     sync.Mutex
		bodies []string
	)
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(body))
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": 0, "message": "Success"})
	}))
	defer server.Close()

	logger := zap.NewNop()
	cfg := channel.ZaloConfig{Enabled: true, OAID: "180789", AccessToken: "test-access-token"}
	ch := NewWithOptions(cfg, logger, server.URL)
	ch.mu.Lock()
	ch.status = channel.StatusConnected
	ch.mu.Unlock()

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "user-123",
		Content: "caption",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeImage,
			URL:  "https://example.com/image.png",
		}},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if len(bodies) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(bodies))
	}
	first := decodeZaloBody(t, bodies[0])
	message, _ := first["message"].(map[string]interface{})
	attachment, _ := message["attachment"].(map[string]interface{})
	payload, _ := attachment["payload"].(map[string]interface{})
	elements, _ := payload["elements"].([]interface{})
	if payload["template_type"] != "media" || len(elements) != 1 {
		t.Fatalf("expected first request to be image media payload, got %#v", first)
	}
	if el, _ := elements[0].(map[string]interface{}); el["url"] != "https://example.com/image.png" {
		t.Fatalf("expected image url in first request, got %#v", el)
	}
	if got := decodeZaloText(t, bodies[1]); got != "caption" {
		t.Fatalf("expected second request to send caption text, got %q", got)
	}
}

func TestChannel_Send_ImageAttachmentFallsBackToTextOnFailure(t *testing.T) {
	var (
		mu     sync.Mutex
		bodies []string
	)
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		payload := string(body)
		mu.Lock()
		bodies = append(bodies, payload)
		mu.Unlock()
		if strings.Contains(payload, "\"template_type\":\"media\"") {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": 0, "message": "Success"})
	}))
	defer server.Close()

	logger := zap.NewNop()
	cfg := channel.ZaloConfig{Enabled: true, OAID: "180789", AccessToken: "test-access-token"}
	ch := NewWithOptions(cfg, logger, server.URL)
	ch.mu.Lock()
	ch.status = channel.StatusConnected
	ch.mu.Unlock()

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "user-123",
		Content: "caption",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeImage,
			URL:  "https://example.com/image.png",
		}},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if len(bodies) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(bodies))
	}
	if got := decodeZaloText(t, bodies[1]); got != "caption\nhttps://example.com/image.png" {
		t.Fatalf("expected fallback text with caption and URL, got %q", got)
	}
}

func TestChannel_Send_AttachmentWithoutURLFallsBackToName(t *testing.T) {
	var (
		mu     sync.Mutex
		bodies []string
	)
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(body))
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": 0, "message": "Success"})
	}))
	defer server.Close()

	logger := zap.NewNop()
	cfg := channel.ZaloConfig{Enabled: true, OAID: "180789", AccessToken: "test-access-token"}
	ch := NewWithOptions(cfg, logger, server.URL)
	ch.mu.Lock()
	ch.status = channel.StatusConnected
	ch.mu.Unlock()

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "user-123",
		Content: "caption",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeFile,
			Name: "agenda.pdf",
		}},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if len(bodies) != 1 {
		t.Fatalf("expected 1 request, got %d", len(bodies))
	}
	if got := decodeZaloText(t, bodies[0]); got != "caption\nagenda.pdf" {
		t.Fatalf("expected fallback text with caption and file name, got %q", got)
	}
}

func decodeZaloBody(t *testing.T, body string) map[string]interface{} {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("failed to decode body %q: %v", body, err)
	}
	return payload
}

func decodeZaloText(t *testing.T, body string) string {
	t.Helper()
	payload := decodeZaloBody(t, body)
	message, _ := payload["message"].(map[string]interface{})
	text, _ := message["text"].(string)
	return text
}

func TestChannel_HandleWebhook_ImageEventEnqueuesAttachmentMessage(t *testing.T) {
	ch := New(channel.ZaloConfig{Enabled: true, OAID: "180789", AccessToken: "token"}, zap.NewNop())
	ch.HandleWebhook(&WebhookEvent{
		AppID:     "app-1",
		EventName: "user_send_image",
		MsgID:     "msg-1",
		Timestamp: time.Now().UnixMilli(),
		Sender:    WebhookSender{ID: "user-1"},
		Message: WebhookMessage{
			Attachments: []WebhookAttachment{{
				Type:    "image",
				Payload: AttachmentPayload{ID: "att-1", URL: "https://example.com/image.png"},
			}},
		},
	})

	select {
	case got := <-ch.Messages():
		if got.Type != channel.MessageTypeImage {
			t.Fatalf("Type = %q, want image", got.Type)
		}
		if len(got.Attachments) != 1 {
			t.Fatalf("Attachments len = %d, want 1", len(got.Attachments))
		}
		if got.Metadata["event_name"] != "user_send_image" {
			t.Fatalf("event_name = %v", got.Metadata["event_name"])
		}
		if got.Metadata["attachment_count"] != 1 {
			t.Fatalf("attachment_count = %v", got.Metadata["attachment_count"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for zalo message")
	}
}
