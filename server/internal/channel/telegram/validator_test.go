package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestValidator_MissingBotToken(t *testing.T) {
	v := NewValidator()
	result := v.Validate(context.Background(), map[string]string{})

	if result.Success {
		t.Error("expected failure for missing bot token")
	}
	if result.MessageKey != "botTokenRequired" {
		t.Errorf("expected MessageKey 'botTokenRequired', got '%s'", result.MessageKey)
	}
}

func TestValidator_EmptyBotToken(t *testing.T) {
	v := NewValidator()
	result := v.Validate(context.Background(), map[string]string{"bot_token": ""})

	if result.Success {
		t.Error("expected failure for empty bot token")
	}
	if result.MessageKey != "botTokenRequired" {
		t.Errorf("expected MessageKey 'botTokenRequired', got '%s'", result.MessageKey)
	}
}

func TestValidator_ValidToken(t *testing.T) {
	// Create mock server
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path contains getMe
		expectedPath := "/bottest-token-123/getMe"
		if r.URL.Path != expectedPath {
			t.Errorf("unexpected path: got %s, want %s", r.URL.Path, expectedPath)
		}

		response := map[string]interface{}{
			"ok": true,
			"result": map[string]interface{}{
				"id":         180789,
				"is_bot":     true,
				"first_name": "TestBot",
				"username":   "test_bot",
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create validator with mock server URL
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
	if result.Data["bot_username"] != "test_bot" {
		t.Errorf("expected bot_username 'test_bot', got '%v'", result.Data["bot_username"])
	}
	// Check bot_id is correct (JSON numbers are float64)
	if botID, ok := result.Data["bot_id"].(int64); ok {
		if botID != 180789 {
			t.Errorf("expected bot_id 180789, got %v", botID)
		}
	}
}

func TestValidator_InvalidToken_401(t *testing.T) {
	// Create mock server that returns 401
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"ok":          false,
			"error_code":  401,
			"description": "Unauthorized",
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

func TestValidator_InvalidToken_404(t *testing.T) {
	// Create mock server that returns 404
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"ok":          false,
			"error_code":  404,
			"description": "Not Found",
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{"bot_token": "not-found-token"})

	if result.Success {
		t.Error("expected failure for not found token")
	}
	if result.MessageKey != "invalidToken" {
		t.Errorf("expected MessageKey 'invalidToken', got '%s'", result.MessageKey)
	}
}

func TestValidator_ServerError(t *testing.T) {
	// Create mock server that returns 500
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"ok":          false,
			"error_code":  500,
			"description": "Internal Server Error",
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

func TestValidator_Timeout(t *testing.T) {
	// Create a slow server
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	}))
	defer server.Close()

	// Create validator with short timeout
	v := NewValidatorWithOptions(100*time.Millisecond, server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := v.Validate(ctx, map[string]string{"bot_token": "test-token"})

	if result.Success {
		t.Error("expected timeout or connection failure")
	}
}

func TestValidator_InvalidJSON(t *testing.T) {
	// Create mock server that returns invalid JSON
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestValidator_NetworkError(t *testing.T) {
	// Use a closed server to simulate network error
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
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

func TestValidator_BotInfoParsing(t *testing.T) {
	// Test with full bot info
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"ok": true,
			"result": map[string]interface{}{
				"id":                          987654321,
				"is_bot":                      true,
				"first_name":                  "My Awesome Bot",
				"username":                    "awesome_bot",
				"can_join_groups":             true,
				"can_read_all_group_messages": false,
				"supports_inline_queries":     true,
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{"bot_token": "test-token"})

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Data["bot_name"] != "My Awesome Bot" {
		t.Errorf("expected bot_name 'My Awesome Bot', got '%v'", result.Data["bot_name"])
	}
	if result.Data["bot_username"] != "awesome_bot" {
		t.Errorf("expected bot_username 'awesome_bot', got '%v'", result.Data["bot_username"])
	}
	if result.Data["is_bot"] != true {
		t.Errorf("expected is_bot true, got '%v'", result.Data["is_bot"])
	}
}
