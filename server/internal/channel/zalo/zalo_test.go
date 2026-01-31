package zalo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel"
)

func TestNew(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.ZaloConfig{
		Enabled:     true,
		OAID:        "123456789",
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
		OAID:        "123456789",
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
		OAID:        "123456789",
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
		OAID:        "123456789",
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
		OAID:        "123456789",
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2.0/oa/getoa":
			response := map[string]interface{}{
				"error":   0,
				"message": "Success",
				"data": map[string]interface{}{
					"oa_id":       "123456789",
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
		OAID:        "123456789",
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		OAID:        "123456789",
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
		OAID:        "123456789",
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
		OAID:        "123456789",
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
