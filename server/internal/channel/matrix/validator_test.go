package matrix

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMatrixValidator_MissingHomeserver(t *testing.T) {
	v := NewValidator()
	result := v.Validate(context.Background(), map[string]string{})

	if result.Success {
		t.Error("expected failure for missing homeserver")
	}
	if result.MessageKey != "homeserverRequired" {
		t.Errorf("expected MessageKey 'homeserverRequired', got '%s'", result.MessageKey)
	}
}

func TestMatrixValidator_MissingAccessToken(t *testing.T) {
	v := NewValidator()
	result := v.Validate(context.Background(), map[string]string{"homeserver": "https://matrix.org"})

	if result.Success {
		t.Error("expected failure for missing access_token")
	}
	if result.MessageKey != "accessTokenRequired" {
		t.Errorf("expected MessageKey 'accessTokenRequired', got '%s'", result.MessageKey)
	}
}

func TestMatrixValidator_ValidCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path
		if r.URL.Path != "/_matrix/client/v3/account/whoami" {
			t.Errorf("unexpected path: got %s", r.URL.Path)
		}

		// Verify authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-access-token-123" {
			t.Errorf("unexpected Authorization header: got %s", auth)
		}

		response := map[string]interface{}{
			"user_id":   "@testuser:matrix.org",
			"device_id": "TESTDEVICE",
			"is_guest":  false,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithTimeout(10 * time.Second)
	result := v.Validate(context.Background(), map[string]string{
		"homeserver":   server.URL,
		"access_token": "test-access-token-123",
	})

	if !result.Success {
		t.Errorf("expected success, got error: %s - %s", result.MessageKey, result.Error)
	}
	if result.MessageKey != "testSuccess" {
		t.Errorf("expected MessageKey 'testSuccess', got '%s'", result.MessageKey)
	}
	if result.Data["user_id"] != "@testuser:matrix.org" {
		t.Errorf("expected user_id '@testuser:matrix.org', got '%v'", result.Data["user_id"])
	}
	if result.Data["device_id"] != "TESTDEVICE" {
		t.Errorf("expected device_id 'TESTDEVICE', got '%v'", result.Data["device_id"])
	}
	if result.Data["is_guest"] != false {
		t.Errorf("expected is_guest false, got '%v'", result.Data["is_guest"])
	}
}

func TestMatrixValidator_UnknownToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"errcode": "M_UNKNOWN_TOKEN",
			"error":   "Unknown token",
		}
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithTimeout(10 * time.Second)
	result := v.Validate(context.Background(), map[string]string{
		"homeserver":   server.URL,
		"access_token": "invalid-token",
	})

	if result.Success {
		t.Error("expected failure for unknown token")
	}
	if result.MessageKey != "invalidToken" {
		t.Errorf("expected MessageKey 'invalidToken', got '%s'", result.MessageKey)
	}
}

func TestMatrixValidator_MissingToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"errcode": "M_MISSING_TOKEN",
			"error":   "Missing access token",
		}
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithTimeout(10 * time.Second)
	result := v.Validate(context.Background(), map[string]string{
		"homeserver":   server.URL,
		"access_token": "some-token",
	})

	if result.Success {
		t.Error("expected failure for missing token")
	}
	if result.MessageKey != "invalidToken" {
		t.Errorf("expected MessageKey 'invalidToken', got '%s'", result.MessageKey)
	}
}

func TestMatrixValidator_Forbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"errcode": "M_FORBIDDEN",
			"error":   "Forbidden",
		}
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithTimeout(10 * time.Second)
	result := v.Validate(context.Background(), map[string]string{
		"homeserver":   server.URL,
		"access_token": "forbidden-token",
	})

	if result.Success {
		t.Error("expected failure for forbidden")
	}
	if result.MessageKey != "invalidToken" {
		t.Errorf("expected MessageKey 'invalidToken', got '%s'", result.MessageKey)
	}
}

func TestMatrixValidator_UserDeactivated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"errcode": "M_USER_DEACTIVATED",
			"error":   "User has been deactivated",
		}
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithTimeout(10 * time.Second)
	result := v.Validate(context.Background(), map[string]string{
		"homeserver":   server.URL,
		"access_token": "deactivated-token",
	})

	if result.Success {
		t.Error("expected failure for deactivated user")
	}
	if result.MessageKey != "invalidToken" {
		t.Errorf("expected MessageKey 'invalidToken', got '%s'", result.MessageKey)
	}
}

func TestMatrixValidator_OtherError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"errcode": "M_UNKNOWN",
			"error":   "Unknown error",
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithTimeout(10 * time.Second)
	result := v.Validate(context.Background(), map[string]string{
		"homeserver":   server.URL,
		"access_token": "test-token",
	})

	if result.Success {
		t.Error("expected failure for other error")
	}
	if result.MessageKey != "connectionFailed" {
		t.Errorf("expected MessageKey 'connectionFailed', got '%s'", result.MessageKey)
	}
}

func TestMatrixValidator_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		json.NewEncoder(w).Encode(map[string]interface{}{"user_id": "@test:matrix.org"})
	}))
	defer server.Close()

	v := NewValidatorWithTimeout(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := v.Validate(ctx, map[string]string{
		"homeserver":   server.URL,
		"access_token": "test-token",
	})

	if result.Success {
		t.Error("expected timeout or connection failure")
	}
}

func TestMatrixValidator_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	v := NewValidatorWithTimeout(10 * time.Second)
	result := v.Validate(context.Background(), map[string]string{
		"homeserver":   server.URL,
		"access_token": "test-token",
	})

	if result.Success {
		t.Error("expected failure for invalid JSON")
	}
	if result.MessageKey != "connectionFailed" {
		t.Errorf("expected MessageKey 'connectionFailed', got '%s'", result.MessageKey)
	}
}

func TestMatrixValidator_NetworkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	serverURL := server.URL
	server.Close()

	v := NewValidatorWithTimeout(1 * time.Second)
	result := v.Validate(context.Background(), map[string]string{
		"homeserver":   serverURL,
		"access_token": "test-token",
	})

	if result.Success {
		t.Error("expected connection failure")
	}
	if result.MessageKey != "connectionFailed" {
		t.Errorf("expected MessageKey 'connectionFailed', got '%s'", result.MessageKey)
	}
}

func TestMatrixValidator_HomeserverWithTrailingSlash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should not have double slashes
		if r.URL.Path != "/_matrix/client/v3/account/whoami" {
			t.Errorf("unexpected path: got %s", r.URL.Path)
		}

		response := map[string]interface{}{
			"user_id":   "@testuser:matrix.org",
			"device_id": "TESTDEVICE",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithTimeout(10 * time.Second)
	result := v.Validate(context.Background(), map[string]string{
		"homeserver":   server.URL + "/", // With trailing slash
		"access_token": "test-token",
	})

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
}

func TestMatrixValidator_GuestUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"user_id":   "@guest123:matrix.org",
			"device_id": "GUESTDEVICE",
			"is_guest":  true,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithTimeout(10 * time.Second)
	result := v.Validate(context.Background(), map[string]string{
		"homeserver":   server.URL,
		"access_token": "guest-token",
	})

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Data["is_guest"] != true {
		t.Errorf("expected is_guest true, got '%v'", result.Data["is_guest"])
	}
}
