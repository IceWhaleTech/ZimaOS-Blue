package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

func TestNewService(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()

	s := NewService(cfg, logger)
	if s == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestCreate(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	webhook, err := s.Create("test-webhook", TypeWake, "Test webhook")
	if err != nil {
		t.Fatalf("failed to create webhook: %v", err)
	}

	if webhook.ID == "" {
		t.Error("expected non-empty ID")
	}

	if webhook.Name != "test-webhook" {
		t.Errorf("expected name 'test-webhook', got '%s'", webhook.Name)
	}

	if webhook.Type != TypeWake {
		t.Errorf("expected type %s, got %s", TypeWake, webhook.Type)
	}

	if webhook.Secret == "" {
		t.Error("expected non-empty secret")
	}

	if !webhook.Enabled {
		t.Error("expected webhook to be enabled")
	}
}

func TestGet(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	webhook, _ := s.Create("test", TypeWake, "")

	// Get existing
	found, exists := s.Get(webhook.ID)
	if !exists {
		t.Fatal("webhook should exist")
	}

	if found.ID != webhook.ID {
		t.Errorf("expected ID %s, got %s", webhook.ID, found.ID)
	}

	// Get non-existing
	_, exists = s.Get("non-existent")
	if exists {
		t.Error("webhook should not exist")
	}
}

func TestList(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	// Create some webhooks
	s.Create("webhook1", TypeWake, "")
	s.Create("webhook2", TypeAgent, "")
	s.Create("webhook3", TypeCustom, "")

	webhooks := s.List()
	if len(webhooks) != 3 {
		t.Errorf("expected 3 webhooks, got %d", len(webhooks))
	}
}

func TestUpdate(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	webhook, _ := s.Create("original", TypeWake, "original desc")

	err := s.Update(webhook.ID, "updated", "updated desc", false)
	if err != nil {
		t.Fatalf("failed to update: %v", err)
	}

	updated, _ := s.Get(webhook.ID)
	if updated.Name != "updated" {
		t.Errorf("expected name 'updated', got '%s'", updated.Name)
	}

	if updated.Description != "updated desc" {
		t.Errorf("expected description 'updated desc', got '%s'", updated.Description)
	}

	if updated.Enabled {
		t.Error("expected webhook to be disabled")
	}
}

func TestDelete(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	webhook, _ := s.Create("to-delete", TypeWake, "")

	err := s.Delete(webhook.ID)
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	_, exists := s.Get(webhook.ID)
	if exists {
		t.Error("webhook should have been deleted")
	}

	// Delete non-existent
	err = s.Delete("non-existent")
	if err == nil {
		t.Error("expected error when deleting non-existent webhook")
	}
}

func TestRegenerateSecret(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	webhook, _ := s.Create("test", TypeWake, "")
	originalSecret := webhook.Secret

	newSecret, err := s.RegenerateSecret(webhook.ID)
	if err != nil {
		t.Fatalf("failed to regenerate secret: %v", err)
	}

	if newSecret == originalSecret {
		t.Error("expected new secret to be different")
	}

	updated, _ := s.Get(webhook.ID)
	if updated.Secret != newSecret {
		t.Error("secret was not updated")
	}
}

func TestHandleRequest(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	// Register handler
	var receivedEvent *WebhookEvent
	s.RegisterHandler(TypeWake, func(ctx context.Context, event *WebhookEvent) (interface{}, error) {
		receivedEvent = event
		return map[string]string{"status": "ok"}, nil
	})

	webhook, _ := s.Create("test", TypeWake, "")

	// Create request
	payload := map[string]string{"message": "hello"}
	body, _ := json.Marshal(payload)

	// Calculate signature
	mac := hmac.New(sha256.New, []byte(webhook.Secret))
	mac.Write(body)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/hooks/"+webhook.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", signature)

	w := httptest.NewRecorder()
	s.HandleRequest(w, req, webhook.ID)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if receivedEvent == nil {
		t.Fatal("handler was not called")
	}

	if receivedEvent.WebhookID != webhook.ID {
		t.Errorf("expected webhook ID %s, got %s", webhook.ID, receivedEvent.WebhookID)
	}
}

func TestHandleRequestInvalidSignature(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	webhook, _ := s.Create("test", TypeWake, "")

	payload := map[string]string{"message": "hello"}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/hooks/"+webhook.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", "sha256=invalid")

	w := httptest.NewRecorder()
	s.HandleRequest(w, req, webhook.ID)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestHandleRequestDisabled(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	webhook, _ := s.Create("test", TypeWake, "")
	s.Update(webhook.ID, webhook.Name, webhook.Description, false)

	req := httptest.NewRequest(http.MethodPost, "/hooks/"+webhook.ID, nil)
	w := httptest.NewRecorder()
	s.HandleRequest(w, req, webhook.ID)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

func TestHandleRequestNotFound(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	req := httptest.NewRequest(http.MethodPost, "/hooks/non-existent", nil)
	w := httptest.NewRecorder()
	s.HandleRequest(w, req, "non-existent")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetEvents(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	s.RegisterHandler(TypeWake, func(ctx context.Context, event *WebhookEvent) (interface{}, error) {
		return nil, nil
	})

	webhook, _ := s.Create("test", TypeWake, "")

	// Send some requests
	for i := 0; i < 5; i++ {
		payload := map[string]int{"count": i}
		body, _ := json.Marshal(payload)

		mac := hmac.New(sha256.New, []byte(webhook.Secret))
		mac.Write(body)
		signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

		req := httptest.NewRequest(http.MethodPost, "/hooks/"+webhook.ID, bytes.NewReader(body))
		req.Header.Set("X-Webhook-Signature", signature)

		w := httptest.NewRecorder()
		s.HandleRequest(w, req, webhook.ID)
	}

	// Get events
	events, err := s.GetEvents(webhook.ID, 3)
	if err != nil {
		t.Fatalf("failed to get events: %v", err)
	}

	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}

	// Get all events
	allEvents, _ := s.GetEvents(webhook.ID, 0)
	if len(allEvents) != 5 {
		t.Errorf("expected 5 events, got %d", len(allEvents))
	}
}

func TestVerifySignature(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	secret := "test-secret"
	payload := []byte("test payload")

	// Calculate correct signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	correctSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	// Test correct signature
	if !s.verifySignature(payload, correctSig, secret) {
		t.Error("expected signature to be valid")
	}

	// Test without prefix
	if !s.verifySignature(payload, hex.EncodeToString(mac.Sum(nil)), secret) {
		t.Error("expected signature without prefix to be valid")
	}

	// Test incorrect signature
	if s.verifySignature(payload, "sha256=invalid", secret) {
		t.Error("expected signature to be invalid")
	}

	// Test empty signature
	if s.verifySignature(payload, "", secret) {
		t.Error("expected empty signature to be invalid")
	}
}
