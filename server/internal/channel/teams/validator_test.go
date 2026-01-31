package teams

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTeamsValidator_MissingAppID(t *testing.T) {
	v := NewValidator()
	result := v.Validate(context.Background(), map[string]string{})

	if result.Success {
		t.Error("expected failure for missing app_id")
	}
	if result.MessageKey != "appIdRequired" {
		t.Errorf("expected MessageKey 'appIdRequired', got '%s'", result.MessageKey)
	}
}

func TestTeamsValidator_MissingAppPassword(t *testing.T) {
	v := NewValidator()
	result := v.Validate(context.Background(), map[string]string{"app_id": "test-app-id"})

	if result.Success {
		t.Error("expected failure for missing app_password")
	}
	if result.MessageKey != "appPasswordRequired" {
		t.Errorf("expected MessageKey 'appPasswordRequired', got '%s'", result.MessageKey)
	}
}

func TestTeamsValidator_ValidCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path contains token endpoint
		if r.URL.Path != "/botframework.com/oauth2/v2.0/token" {
			t.Errorf("unexpected path: got %s", r.URL.Path)
		}

		// Verify content type
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("unexpected Content-Type: got %s", r.Header.Get("Content-Type"))
		}

		// Parse form data
		r.ParseForm()
		if r.Form.Get("client_id") != "test-app-id" {
			t.Errorf("unexpected client_id: got %s", r.Form.Get("client_id"))
		}
		if r.Form.Get("client_secret") != "test-app-password" {
			t.Errorf("unexpected client_secret: got %s", r.Form.Get("client_secret"))
		}

		response := map[string]interface{}{
			"token_type":   "Bearer",
			"expires_in":   3600,
			"access_token": "test-access-token-abc123",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"app_id":       "test-app-id",
		"app_password": "test-app-password",
	})

	if !result.Success {
		t.Errorf("expected success, got error: %s - %s", result.MessageKey, result.Error)
	}
	if result.MessageKey != "testSuccess" {
		t.Errorf("expected MessageKey 'testSuccess', got '%s'", result.MessageKey)
	}
	if result.Data["app_id"] != "test-app-id" {
		t.Errorf("expected app_id 'test-app-id', got '%v'", result.Data["app_id"])
	}
	if result.Data["has_token"] != true {
		t.Errorf("expected has_token true, got '%v'", result.Data["has_token"])
	}
}

func TestTeamsValidator_WithTenantID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify tenant ID is in path
		if r.URL.Path != "/custom-tenant-id/oauth2/v2.0/token" {
			t.Errorf("unexpected path: got %s, want /custom-tenant-id/oauth2/v2.0/token", r.URL.Path)
		}

		response := map[string]interface{}{
			"token_type":   "Bearer",
			"expires_in":   3600,
			"access_token": "test-access-token",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"app_id":       "test-app-id",
		"app_password": "test-app-password",
		"tenant_id":    "custom-tenant-id",
	})

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Data["tenant_id"] != "custom-tenant-id" {
		t.Errorf("expected tenant_id 'custom-tenant-id', got '%v'", result.Data["tenant_id"])
	}
}

func TestTeamsValidator_InvalidClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"error":             "invalid_client",
			"error_description": "Invalid client credentials",
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"app_id":       "invalid-app-id",
		"app_password": "invalid-password",
	})

	if result.Success {
		t.Error("expected failure for invalid client")
	}
	if result.MessageKey != "invalidToken" {
		t.Errorf("expected MessageKey 'invalidToken', got '%s'", result.MessageKey)
	}
}

func TestTeamsValidator_UnauthorizedClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"error":             "unauthorized_client",
			"error_description": "Client is not authorized",
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"app_id":       "test-app-id",
		"app_password": "test-password",
	})

	if result.Success {
		t.Error("expected failure for unauthorized client")
	}
	if result.MessageKey != "invalidToken" {
		t.Errorf("expected MessageKey 'invalidToken', got '%s'", result.MessageKey)
	}
}

func TestTeamsValidator_InvalidGrant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"error":             "invalid_grant",
			"error_description": "Credentials expired",
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"app_id":       "test-app-id",
		"app_password": "expired-password",
	})

	if result.Success {
		t.Error("expected failure for invalid grant")
	}
	if result.MessageKey != "invalidToken" {
		t.Errorf("expected MessageKey 'invalidToken', got '%s'", result.MessageKey)
	}
}

func TestTeamsValidator_OtherError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"error":             "server_error",
			"error_description": "Internal server error",
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"app_id":       "test-app-id",
		"app_password": "test-password",
	})

	if result.Success {
		t.Error("expected failure for server error")
	}
	if result.MessageKey != "connectionFailed" {
		t.Errorf("expected MessageKey 'connectionFailed', got '%s'", result.MessageKey)
	}
}

func TestTeamsValidator_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "token"})
	}))
	defer server.Close()

	v := NewValidatorWithOptions(100*time.Millisecond, server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := v.Validate(ctx, map[string]string{
		"app_id":       "test-app-id",
		"app_password": "test-password",
	})

	if result.Success {
		t.Error("expected timeout or connection failure")
	}
}

func TestTeamsValidator_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"app_id":       "test-app-id",
		"app_password": "test-password",
	})

	if result.Success {
		t.Error("expected failure for invalid JSON")
	}
	if result.MessageKey != "connectionFailed" {
		t.Errorf("expected MessageKey 'connectionFailed', got '%s'", result.MessageKey)
	}
}

func TestTeamsValidator_NetworkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	serverURL := server.URL
	server.Close()

	v := NewValidatorWithOptions(1*time.Second, serverURL)
	result := v.Validate(context.Background(), map[string]string{
		"app_id":       "test-app-id",
		"app_password": "test-password",
	})

	if result.Success {
		t.Error("expected connection failure")
	}
	if result.MessageKey != "connectionFailed" {
		t.Errorf("expected MessageKey 'connectionFailed', got '%s'", result.MessageKey)
	}
}

func TestTeamsValidator_NoAccessToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"token_type": "Bearer",
			"expires_in": 3600,
			// No access_token
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"app_id":       "test-app-id",
		"app_password": "test-password",
	})

	if result.Success {
		t.Error("expected failure for no access token")
	}
	if result.MessageKey != "connectionFailed" {
		t.Errorf("expected MessageKey 'connectionFailed', got '%s'", result.MessageKey)
	}
}
