package mattermost

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMattermostValidator_MissingServerURL(t *testing.T) {
	v := NewValidator()
	result := v.Validate(context.Background(), map[string]string{})

	if result.Success {
		t.Error("expected failure for missing server_url")
	}
	if result.MessageKey != "serverUrlRequired" {
		t.Errorf("expected MessageKey 'serverUrlRequired', got '%s'", result.MessageKey)
	}
}

func TestMattermostValidator_MissingBotToken(t *testing.T) {
	v := NewValidator()
	result := v.Validate(context.Background(), map[string]string{"server_url": "https://mattermost.example.com"})

	if result.Success {
		t.Error("expected failure for missing bot_token")
	}
	if result.MessageKey != "botTokenRequired" {
		t.Errorf("expected MessageKey 'botTokenRequired', got '%s'", result.MessageKey)
	}
}

func TestMattermostValidator_ValidCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path
		if r.URL.Path != "/api/v4/users/me" {
			t.Errorf("unexpected path: got %s", r.URL.Path)
		}

		// Verify authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-bot-token" {
			t.Errorf("unexpected Authorization header: got %s", auth)
		}

		response := map[string]interface{}{
			"id":         "bot123456",
			"username":   "testbot",
			"email":      "bot@example.com",
			"nickname":   "Test Bot",
			"first_name": "Test",
			"last_name":  "Bot",
			"is_bot":     true,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithTimeout(10 * time.Second)
	result := v.Validate(context.Background(), map[string]string{
		"server_url": server.URL,
		"bot_token":  "test-bot-token",
	})

	if !result.Success {
		t.Errorf("expected success, got error: %s - %s", result.MessageKey, result.Error)
	}
	if result.MessageKey != "testSuccess" {
		t.Errorf("expected MessageKey 'testSuccess', got '%s'", result.MessageKey)
	}
	if result.Data["bot_id"] != "bot123456" {
		t.Errorf("expected bot_id 'bot123456', got '%v'", result.Data["bot_id"])
	}
	if result.Data["bot_username"] != "testbot" {
		t.Errorf("expected bot_username 'testbot', got '%v'", result.Data["bot_username"])
	}
	if result.Data["is_bot"] != true {
		t.Errorf("expected is_bot true, got '%v'", result.Data["is_bot"])
	}
}

func TestMattermostValidator_ServerURLWithTrailingSlash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should not have double slashes
		if r.URL.Path != "/api/v4/users/me" {
			t.Errorf("unexpected path: got %s", r.URL.Path)
		}

		response := map[string]interface{}{
			"id":       "bot123",
			"username": "testbot",
			"is_bot":   true,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithTimeout(10 * time.Second)
	result := v.Validate(context.Background(), map[string]string{
		"server_url": server.URL + "/", // With trailing slash
		"bot_token":  "test-token",
	})

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
}

func TestMattermostValidator_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id":          "api.context.session_expired.app_error",
			"message":     "Invalid or expired session",
			"status_code": 401,
		}
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithTimeout(10 * time.Second)
	result := v.Validate(context.Background(), map[string]string{
		"server_url": server.URL,
		"bot_token":  "invalid-token",
	})

	if result.Success {
		t.Error("expected failure for unauthorized")
	}
	if result.MessageKey != "invalidToken" {
		t.Errorf("expected MessageKey 'invalidToken', got '%s'", result.MessageKey)
	}
}

func TestMattermostValidator_Forbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id":          "api.context.permissions.app_error",
			"message":     "You do not have the appropriate permissions",
			"status_code": 403,
		}
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithTimeout(10 * time.Second)
	result := v.Validate(context.Background(), map[string]string{
		"server_url": server.URL,
		"bot_token":  "forbidden-token",
	})

	if result.Success {
		t.Error("expected failure for forbidden")
	}
	if result.MessageKey != "invalidToken" {
		t.Errorf("expected MessageKey 'invalidToken', got '%s'", result.MessageKey)
	}
}

func TestMattermostValidator_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id":          "api.context.internal_error",
			"message":     "Internal server error",
			"status_code": 500,
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithTimeout(10 * time.Second)
	result := v.Validate(context.Background(), map[string]string{
		"server_url": server.URL,
		"bot_token":  "test-token",
	})

	if result.Success {
		t.Error("expected failure for server error")
	}
	if result.MessageKey != "connectionFailed" {
		t.Errorf("expected MessageKey 'connectionFailed', got '%s'", result.MessageKey)
	}
}

func TestMattermostValidator_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		json.NewEncoder(w).Encode(map[string]interface{}{"id": "123"})
	}))
	defer server.Close()

	v := NewValidatorWithTimeout(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := v.Validate(ctx, map[string]string{
		"server_url": server.URL,
		"bot_token":  "test-token",
	})

	if result.Success {
		t.Error("expected timeout or connection failure")
	}
}

func TestMattermostValidator_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	v := NewValidatorWithTimeout(10 * time.Second)
	result := v.Validate(context.Background(), map[string]string{
		"server_url": server.URL,
		"bot_token":  "test-token",
	})

	if result.Success {
		t.Error("expected failure for invalid JSON")
	}
	if result.MessageKey != "connectionFailed" {
		t.Errorf("expected MessageKey 'connectionFailed', got '%s'", result.MessageKey)
	}
}

func TestMattermostValidator_NetworkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	serverURL := server.URL
	server.Close()

	v := NewValidatorWithTimeout(1 * time.Second)
	result := v.Validate(context.Background(), map[string]string{
		"server_url": serverURL,
		"bot_token":  "test-token",
	})

	if result.Success {
		t.Error("expected connection failure")
	}
	if result.MessageKey != "connectionFailed" {
		t.Errorf("expected MessageKey 'connectionFailed', got '%s'", result.MessageKey)
	}
}
