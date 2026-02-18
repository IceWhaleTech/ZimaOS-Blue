package discord

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDiscordValidator_MissingBotToken(t *testing.T) {
	v := NewValidator()
	result := v.Validate(context.Background(), map[string]string{})

	if result.Success {
		t.Error("expected failure for missing bot token")
	}
	if result.MessageKey != "botTokenRequired" {
		t.Errorf("expected MessageKey 'botTokenRequired', got '%s'", result.MessageKey)
	}
}

func TestDiscordValidator_EmptyBotToken(t *testing.T) {
	v := NewValidator()
	result := v.Validate(context.Background(), map[string]string{"bot_token": ""})

	if result.Success {
		t.Error("expected failure for empty bot token")
	}
	if result.MessageKey != "botTokenRequired" {
		t.Errorf("expected MessageKey 'botTokenRequired', got '%s'", result.MessageKey)
	}
}

func TestDiscordValidator_ValidToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path
		if r.URL.Path != "/users/@me" {
			t.Errorf("unexpected path: got %s, want /users/@me", r.URL.Path)
		}

		// Verify authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bot test-token-123" {
			t.Errorf("unexpected Authorization header: got %s", auth)
		}

		response := map[string]interface{}{
			"id":            "180789018078",
			"username":      "TestBot",
			"discriminator": "0",
			"global_name":   "Test Bot",
			"bot":           true,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{"bot_token": "test-token-123"})

	if !result.Success {
		t.Errorf("expected success, got error: %s - %s", result.MessageKey, result.Error)
	}
	if result.MessageKey != "testSuccess" {
		t.Errorf("expected MessageKey 'testSuccess', got '%s'", result.MessageKey)
	}
	if result.Data["bot_name"] != "TestBot" {
		t.Errorf("expected bot_name 'TestBot', got '%v'", result.Data["bot_name"])
	}
	if result.Data["bot_id"] != "180789018078" {
		t.Errorf("expected bot_id '180789018078', got '%v'", result.Data["bot_id"])
	}
	if result.Data["is_bot"] != true {
		t.Errorf("expected is_bot true, got '%v'", result.Data["is_bot"])
	}
}

func TestDiscordValidator_InvalidToken_401(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"code":    0,
			"message": "401: Unauthorized",
		}
		w.WriteHeader(http.StatusUnauthorized)
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

func TestDiscordValidator_Forbidden_403(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"code":    50001,
			"message": "Missing Access",
		}
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{"bot_token": "forbidden-token"})

	if result.Success {
		t.Error("expected failure for forbidden token")
	}
	if result.MessageKey != "invalidToken" {
		t.Errorf("expected MessageKey 'invalidToken', got '%s'", result.MessageKey)
	}
}

func TestDiscordValidator_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"code":    0,
			"message": "Internal Server Error",
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{"bot_token": "test-token"})

	if result.Success {
		t.Error("expected failure for server error")
	}
	if result.MessageKey != "connectionFailed" {
		t.Errorf("expected MessageKey 'connectionFailed', got '%s'", result.MessageKey)
	}
}

func TestDiscordValidator_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		json.NewEncoder(w).Encode(map[string]interface{}{"id": "123"})
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

func TestDiscordValidator_InvalidJSON(t *testing.T) {
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

func TestDiscordValidator_NetworkError(t *testing.T) {
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

func TestDiscordValidator_BotInfoParsing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id":            "987654321098765432",
			"username":      "AwesomeBot",
			"discriminator": "1234",
			"global_name":   "Awesome Bot",
			"avatar":        "abc123",
			"bot":           true,
			"verified":      true,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{"bot_token": "test-token"})

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Data["bot_name"] != "AwesomeBot" {
		t.Errorf("expected bot_name 'AwesomeBot', got '%v'", result.Data["bot_name"])
	}
	if result.Data["global_name"] != "Awesome Bot" {
		t.Errorf("expected global_name 'Awesome Bot', got '%v'", result.Data["global_name"])
	}
	if result.Data["is_bot"] != true {
		t.Errorf("expected is_bot true, got '%v'", result.Data["is_bot"])
	}
}
