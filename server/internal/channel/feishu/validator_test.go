package feishu

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFeishuValidator_MissingAppID(t *testing.T) {
	v := NewValidator()
	result := v.Validate(context.Background(), map[string]string{})

	if result.Success {
		t.Error("expected failure for missing app_id")
	}
	if result.MessageKey != "appIdRequired" {
		t.Errorf("expected MessageKey 'appIdRequired', got '%s'", result.MessageKey)
	}
}

func TestFeishuValidator_MissingAppSecret(t *testing.T) {
	v := NewValidator()
	result := v.Validate(context.Background(), map[string]string{"app_id": "cli_test123"})

	if result.Success {
		t.Error("expected failure for missing app_secret")
	}
	if result.MessageKey != "appSecretRequired" {
		t.Errorf("expected MessageKey 'appSecretRequired', got '%s'", result.MessageKey)
	}
}

func TestFeishuValidator_ValidCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path
		if r.URL.Path != "/auth/v3/tenant_access_token/internal" {
			t.Errorf("unexpected path: got %s", r.URL.Path)
		}

		// Verify request body
		body, _ := io.ReadAll(r.Body)
		var reqBody map[string]string
		json.Unmarshal(body, &reqBody)

		if reqBody["app_id"] != "cli_test123" {
			t.Errorf("unexpected app_id: got %s", reqBody["app_id"])
		}
		if reqBody["app_secret"] != "secret456" {
			t.Errorf("unexpected app_secret: got %s", reqBody["app_secret"])
		}

		response := map[string]interface{}{
			"code":                0,
			"msg":                 "ok",
			"tenant_access_token": "t-test-token-abc123",
			"expire":              7200,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"app_id":     "cli_test123",
		"app_secret": "secret456",
	})

	if !result.Success {
		t.Errorf("expected success, got error: %s - %s", result.MessageKey, result.Error)
	}
	if result.MessageKey != "testSuccess" {
		t.Errorf("expected MessageKey 'testSuccess', got '%s'", result.MessageKey)
	}
	if result.Data["app_id"] != "cli_test123" {
		t.Errorf("expected app_id 'cli_test123', got '%v'", result.Data["app_id"])
	}
	if result.Data["token_expire"] != 7200 {
		t.Errorf("expected token_expire 7200, got '%v'", result.Data["token_expire"])
	}
	if result.Data["has_token"] != true {
		t.Errorf("expected has_token true, got '%v'", result.Data["has_token"])
	}
}

func TestFeishuValidator_InvalidAppID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"code": 10003,
			"msg":  "app_id not found",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"app_id":     "invalid_app_id",
		"app_secret": "secret456",
	})

	if result.Success {
		t.Error("expected failure for invalid app_id")
	}
	if result.MessageKey != "invalidToken" {
		t.Errorf("expected MessageKey 'invalidToken', got '%s'", result.MessageKey)
	}
}

func TestFeishuValidator_InvalidAppSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"code": 10014,
			"msg":  "app secret invalid",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"app_id":     "cli_test123",
		"app_secret": "invalid_secret",
	})

	if result.Success {
		t.Error("expected failure for invalid app_secret")
	}
	if result.MessageKey != "invalidToken" {
		t.Errorf("expected MessageKey 'invalidToken', got '%s'", result.MessageKey)
	}
}

func TestFeishuValidator_AppDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"code": 10015,
			"msg":  "app has been disabled",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"app_id":     "cli_disabled_app",
		"app_secret": "secret456",
	})

	if result.Success {
		t.Error("expected failure for disabled app")
	}
	if result.MessageKey != "invalidToken" {
		t.Errorf("expected MessageKey 'invalidToken', got '%s'", result.MessageKey)
	}
}

func TestFeishuValidator_OtherError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"code": 99999,
			"msg":  "unknown error",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"app_id":     "cli_test123",
		"app_secret": "secret456",
	})

	if result.Success {
		t.Error("expected failure for other error")
	}
	if result.MessageKey != "connectionFailed" {
		t.Errorf("expected MessageKey 'connectionFailed', got '%s'", result.MessageKey)
	}
}

func TestFeishuValidator_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 0})
	}))
	defer server.Close()

	v := NewValidatorWithOptions(100*time.Millisecond, server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := v.Validate(ctx, map[string]string{
		"app_id":     "cli_test123",
		"app_secret": "secret456",
	})

	if result.Success {
		t.Error("expected timeout or connection failure")
	}
}

func TestFeishuValidator_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"app_id":     "cli_test123",
		"app_secret": "secret456",
	})

	if result.Success {
		t.Error("expected failure for invalid JSON")
	}
	if result.MessageKey != "connectionFailed" {
		t.Errorf("expected MessageKey 'connectionFailed', got '%s'", result.MessageKey)
	}
}

func TestFeishuValidator_NetworkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	serverURL := server.URL
	server.Close()

	v := NewValidatorWithOptions(1*time.Second, serverURL)
	result := v.Validate(context.Background(), map[string]string{
		"app_id":     "cli_test123",
		"app_secret": "secret456",
	})

	if result.Success {
		t.Error("expected connection failure")
	}
	if result.MessageKey != "connectionFailed" {
		t.Errorf("expected MessageKey 'connectionFailed', got '%s'", result.MessageKey)
	}
}
