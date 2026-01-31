package bluebubbles

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBlueBubblesValidator_MissingServerURL(t *testing.T) {
	v := NewValidator()
	result := v.Validate(context.Background(), map[string]string{})

	if result.Success {
		t.Error("expected failure for missing server_url")
	}
	if result.MessageKey != "serverUrlRequired" {
		t.Errorf("expected MessageKey 'serverUrlRequired', got '%s'", result.MessageKey)
	}
}

func TestBlueBubblesValidator_MissingPassword(t *testing.T) {
	v := NewValidator()
	result := v.Validate(context.Background(), map[string]string{"server_url": "http://localhost:1234"})

	if result.Success {
		t.Error("expected failure for missing password")
	}
	if result.MessageKey != "passwordRequired" {
		t.Errorf("expected MessageKey 'passwordRequired', got '%s'", result.MessageKey)
	}
}

func TestBlueBubblesValidator_ValidCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path
		if r.URL.Path != "/api/v1/server/info" {
			t.Errorf("unexpected path: got %s", r.URL.Path)
		}

		// Verify password parameter
		password := r.URL.Query().Get("password")
		if password != "test-password" {
			t.Errorf("unexpected password: got %s", password)
		}

		response := map[string]interface{}{
			"status":  200,
			"message": "Success",
			"data": map[string]interface{}{
				"os_version":        "14.0",
				"server_version":    "1.9.0",
				"private_api_mode":  true,
				"helper_connected":  true,
				"proxy_service":     "Cloudflare",
				"detected_icloud":   "user@icloud.com",
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithTimeout(10 * time.Second)
	result := v.Validate(context.Background(), map[string]string{
		"server_url": server.URL,
		"password":   "test-password",
	})

	if !result.Success {
		t.Errorf("expected success, got error: %s - %s", result.MessageKey, result.Error)
	}
	if result.MessageKey != "testSuccess" {
		t.Errorf("expected MessageKey 'testSuccess', got '%s'", result.MessageKey)
	}
	if result.Data["server_version"] != "1.9.0" {
		t.Errorf("expected server_version '1.9.0', got '%v'", result.Data["server_version"])
	}
	if result.Data["private_api_mode"] != true {
		t.Errorf("expected private_api_mode true, got '%v'", result.Data["private_api_mode"])
	}
}

func TestBlueBubblesValidator_ServerURLWithTrailingSlash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should not have double slashes
		if r.URL.Path != "/api/v1/server/info" {
			t.Errorf("unexpected path: got %s", r.URL.Path)
		}

		response := map[string]interface{}{
			"status":  200,
			"message": "Success",
			"data": map[string]interface{}{
				"server_version": "1.9.0",
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithTimeout(10 * time.Second)
	result := v.Validate(context.Background(), map[string]string{
		"server_url": server.URL + "/", // With trailing slash
		"password":   "test-password",
	})

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
}

func TestBlueBubblesValidator_InvalidPassword(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"status":  401,
			"message": "Unauthorized",
			"error": map[string]interface{}{
				"type":    "Authentication Error",
				"message": "Invalid password",
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithTimeout(10 * time.Second)
	result := v.Validate(context.Background(), map[string]string{
		"server_url": server.URL,
		"password":   "wrong-password",
	})

	if result.Success {
		t.Error("expected failure for invalid password")
	}
	if result.MessageKey != "invalidToken" {
		t.Errorf("expected MessageKey 'invalidToken', got '%s'", result.MessageKey)
	}
}

func TestBlueBubblesValidator_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"status":  500,
			"message": "Internal Server Error",
			"error": map[string]interface{}{
				"type":    "Server Error",
				"message": "Something went wrong",
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithTimeout(10 * time.Second)
	result := v.Validate(context.Background(), map[string]string{
		"server_url": server.URL,
		"password":   "test-password",
	})

	if result.Success {
		t.Error("expected failure for server error")
	}
	if result.MessageKey != "connectionFailed" {
		t.Errorf("expected MessageKey 'connectionFailed', got '%s'", result.MessageKey)
	}
}

func TestBlueBubblesValidator_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": 200})
	}))
	defer server.Close()

	v := NewValidatorWithTimeout(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := v.Validate(ctx, map[string]string{
		"server_url": server.URL,
		"password":   "test-password",
	})

	if result.Success {
		t.Error("expected timeout or connection failure")
	}
}

func TestBlueBubblesValidator_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	v := NewValidatorWithTimeout(10 * time.Second)
	result := v.Validate(context.Background(), map[string]string{
		"server_url": server.URL,
		"password":   "test-password",
	})

	if result.Success {
		t.Error("expected failure for invalid JSON")
	}
	if result.MessageKey != "connectionFailed" {
		t.Errorf("expected MessageKey 'connectionFailed', got '%s'", result.MessageKey)
	}
}

func TestBlueBubblesValidator_NetworkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	serverURL := server.URL
	server.Close()

	v := NewValidatorWithTimeout(1 * time.Second)
	result := v.Validate(context.Background(), map[string]string{
		"server_url": serverURL,
		"password":   "test-password",
	})

	if result.Success {
		t.Error("expected connection failure")
	}
	if result.MessageKey != "connectionFailed" {
		t.Errorf("expected MessageKey 'connectionFailed', got '%s'", result.MessageKey)
	}
}

func TestBlueBubblesValidator_HelperNotConnected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"status":  200,
			"message": "Success",
			"data": map[string]interface{}{
				"server_version":   "1.9.0",
				"helper_connected": false,
				"private_api_mode": false,
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithTimeout(10 * time.Second)
	result := v.Validate(context.Background(), map[string]string{
		"server_url": server.URL,
		"password":   "test-password",
	})

	// Should still succeed, but indicate helper is not connected
	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Data["helper_connected"] != false {
		t.Errorf("expected helper_connected false, got '%v'", result.Data["helper_connected"])
	}
}
