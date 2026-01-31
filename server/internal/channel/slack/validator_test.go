package slack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSlackValidator_MissingBotToken(t *testing.T) {
	v := NewValidator()
	result := v.Validate(context.Background(), map[string]string{})

	if result.Success {
		t.Error("expected failure for missing bot token")
	}
	if result.MessageKey != "botTokenRequired" {
		t.Errorf("expected MessageKey 'botTokenRequired', got '%s'", result.MessageKey)
	}
}

func TestSlackValidator_EmptyBotToken(t *testing.T) {
	v := NewValidator()
	result := v.Validate(context.Background(), map[string]string{"bot_token": ""})

	if result.Success {
		t.Error("expected failure for empty bot token")
	}
	if result.MessageKey != "botTokenRequired" {
		t.Errorf("expected MessageKey 'botTokenRequired', got '%s'", result.MessageKey)
	}
}

func TestSlackValidator_ValidToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path
		if r.URL.Path != "/auth.test" {
			t.Errorf("unexpected path: got %s, want /auth.test", r.URL.Path)
		}

		// Verify authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer xoxb-test-token-123" {
			t.Errorf("unexpected Authorization header: got %s", auth)
		}

		response := map[string]interface{}{
			"ok":      true,
			"url":     "https://testworkspace.slack.com/",
			"team":    "Test Workspace",
			"user":    "testbot",
			"team_id": "T12345678",
			"user_id": "U12345678",
			"bot_id":  "B12345678",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{"bot_token": "xoxb-test-token-123"})

	if !result.Success {
		t.Errorf("expected success, got error: %s - %s", result.MessageKey, result.Error)
	}
	if result.MessageKey != "testSuccess" {
		t.Errorf("expected MessageKey 'testSuccess', got '%s'", result.MessageKey)
	}
	if result.Data["team_name"] != "Test Workspace" {
		t.Errorf("expected team_name 'Test Workspace', got '%v'", result.Data["team_name"])
	}
	if result.Data["bot_name"] != "testbot" {
		t.Errorf("expected bot_name 'testbot', got '%v'", result.Data["bot_name"])
	}
	if result.Data["team_id"] != "T12345678" {
		t.Errorf("expected team_id 'T12345678', got '%v'", result.Data["team_id"])
	}
}

func TestSlackValidator_InvalidAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"ok":    false,
			"error": "invalid_auth",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{"bot_token": "invalid-token"})

	if result.Success {
		t.Error("expected failure for invalid token")
	}
	if result.MessageKey != "invalidToken" {
		t.Errorf("expected MessageKey 'invalidToken', got '%s'", result.MessageKey)
	}
}

func TestSlackValidator_TokenRevoked(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"ok":    false,
			"error": "token_revoked",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{"bot_token": "revoked-token"})

	if result.Success {
		t.Error("expected failure for revoked token")
	}
	if result.MessageKey != "invalidToken" {
		t.Errorf("expected MessageKey 'invalidToken', got '%s'", result.MessageKey)
	}
}

func TestSlackValidator_AccountInactive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"ok":    false,
			"error": "account_inactive",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{"bot_token": "inactive-token"})

	if result.Success {
		t.Error("expected failure for inactive account")
	}
	if result.MessageKey != "invalidToken" {
		t.Errorf("expected MessageKey 'invalidToken', got '%s'", result.MessageKey)
	}
}

func TestSlackValidator_OtherError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"ok":    false,
			"error": "some_other_error",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{"bot_token": "test-token"})

	if result.Success {
		t.Error("expected failure for other error")
	}
	if result.MessageKey != "connectionFailed" {
		t.Errorf("expected MessageKey 'connectionFailed', got '%s'", result.MessageKey)
	}
}

func TestSlackValidator_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	}))
	defer server.Close()

	v := NewValidatorWithOptions(100*time.Millisecond, server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := v.Validate(ctx, map[string]string{"bot_token": "test-token"})

	if result.Success {
		t.Error("expected timeout or connection failure")
	}
}

func TestSlackValidator_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{"bot_token": "test-token"})

	if result.Success {
		t.Error("expected failure for invalid JSON")
	}
	if result.MessageKey != "connectionFailed" {
		t.Errorf("expected MessageKey 'connectionFailed', got '%s'", result.MessageKey)
	}
}

func TestSlackValidator_NetworkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	serverURL := server.URL
	server.Close()

	v := NewValidatorWithOptions(1*time.Second, serverURL)
	result := v.Validate(context.Background(), map[string]string{"bot_token": "test-token"})

	if result.Success {
		t.Error("expected connection failure")
	}
	if result.MessageKey != "connectionFailed" {
		t.Errorf("expected MessageKey 'connectionFailed', got '%s'", result.MessageKey)
	}
}

func TestSlackValidator_WithAppToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"ok":      true,
			"url":     "https://testworkspace.slack.com/",
			"team":    "Test Workspace",
			"user":    "testbot",
			"team_id": "T12345678",
			"user_id": "U12345678",
			"bot_id":  "B12345678",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"bot_token": "xoxb-test-token-123",
		"app_token": "xapp-test-token-456",
	})

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Data["app_token_provided"] != true {
		t.Errorf("expected app_token_provided true, got '%v'", result.Data["app_token_provided"])
	}
}

func TestSlackValidator_EnterpriseInstall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"ok":                    true,
			"url":                   "https://enterprise.slack.com/",
			"team":                  "Enterprise Workspace",
			"user":                  "enterprisebot",
			"team_id":               "E12345678",
			"user_id":               "U12345678",
			"bot_id":                "B12345678",
			"is_enterprise_install": true,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{"bot_token": "xoxb-enterprise-token"})

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Data["team_name"] != "Enterprise Workspace" {
		t.Errorf("expected team_name 'Enterprise Workspace', got '%v'", result.Data["team_name"])
	}
}
