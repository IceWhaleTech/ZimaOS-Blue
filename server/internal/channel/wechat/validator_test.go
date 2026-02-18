package wechat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWeChatValidator_MissingCorpID(t *testing.T) {
	v := NewValidator()
	result := v.Validate(context.Background(), map[string]string{})

	if result.Success {
		t.Error("expected failure for missing corp_id")
	}
	if result.MessageKey != "corpIdRequired" {
		t.Errorf("expected MessageKey 'corpIdRequired', got '%s'", result.MessageKey)
	}
}

func TestWeChatValidator_MissingSecret(t *testing.T) {
	v := NewValidator()
	result := v.Validate(context.Background(), map[string]string{"corp_id": "ww180"})

	if result.Success {
		t.Error("expected failure for missing secret")
	}
	if result.MessageKey != "secretRequired" {
		t.Errorf("expected MessageKey 'secretRequired', got '%s'", result.MessageKey)
	}
}

func TestWeChatValidator_ValidCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path
		if !strings.HasPrefix(r.URL.Path, "/gettoken") {
			t.Errorf("unexpected path: got %s", r.URL.Path)
		}

		// Verify query parameters
		corpID := r.URL.Query().Get("corpid")
		secret := r.URL.Query().Get("corpsecret")

		if corpID != "ww180" {
			t.Errorf("unexpected corpid: got %s", corpID)
		}
		if secret != "secret789" {
			t.Errorf("unexpected corpsecret: got %s", secret)
		}

		response := map[string]interface{}{
			"errcode":      0,
			"errmsg":       "ok",
			"access_token": "test-access-token-abc123",
			"expires_in":   7200,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"corp_id": "ww180",
		"secret":  "secret789",
	})

	if !result.Success {
		t.Errorf("expected success, got error: %s - %s", result.MessageKey, result.Error)
	}
	if result.MessageKey != "testSuccess" {
		t.Errorf("expected MessageKey 'testSuccess', got '%s'", result.MessageKey)
	}
	if result.Data["corp_id"] != "ww180" {
		t.Errorf("expected corp_id 'ww180', got '%v'", result.Data["corp_id"])
	}
	if result.Data["token_expire"] != 7200 {
		t.Errorf("expected token_expire 7200, got '%v'", result.Data["token_expire"])
	}
	if result.Data["has_token"] != true {
		t.Errorf("expected has_token true, got '%v'", result.Data["has_token"])
	}
}

func TestWeChatValidator_InvalidSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"errcode": 40001,
			"errmsg":  "invalid credential",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"corp_id": "ww180",
		"secret":  "invalid_secret",
	})

	if result.Success {
		t.Error("expected failure for invalid secret")
	}
	if result.MessageKey != "invalidToken" {
		t.Errorf("expected MessageKey 'invalidToken', got '%s'", result.MessageKey)
	}
}

func TestWeChatValidator_InvalidCorpID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"errcode": 40013,
			"errmsg":  "invalid corpid",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"corp_id": "invalid_corp_id",
		"secret":  "secret789",
	})

	if result.Success {
		t.Error("expected failure for invalid corp_id")
	}
	if result.MessageKey != "invalidToken" {
		t.Errorf("expected MessageKey 'invalidToken', got '%s'", result.MessageKey)
	}
}

func TestWeChatValidator_NoPermission(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"errcode": 60011,
			"errmsg":  "no privilege to access/binduser",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"corp_id": "ww180",
		"secret":  "secret789",
	})

	if result.Success {
		t.Error("expected failure for no permission")
	}
	if result.MessageKey != "invalidToken" {
		t.Errorf("expected MessageKey 'invalidToken', got '%s'", result.MessageKey)
	}
}

func TestWeChatValidator_AgentNotEnabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"errcode": 60020,
			"errmsg":  "agent not enabled",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"corp_id": "ww180",
		"secret":  "secret789",
	})

	if result.Success {
		t.Error("expected failure for agent not enabled")
	}
	if result.MessageKey != "invalidToken" {
		t.Errorf("expected MessageKey 'invalidToken', got '%s'", result.MessageKey)
	}
}

func TestWeChatValidator_OtherError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"errcode": 99999,
			"errmsg":  "unknown error",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"corp_id": "ww180",
		"secret":  "secret789",
	})

	if result.Success {
		t.Error("expected failure for other error")
	}
	if result.MessageKey != "connectionFailed" {
		t.Errorf("expected MessageKey 'connectionFailed', got '%s'", result.MessageKey)
	}
}

func TestWeChatValidator_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		json.NewEncoder(w).Encode(map[string]interface{}{"errcode": 0})
	}))
	defer server.Close()

	v := NewValidatorWithOptions(100*time.Millisecond, server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := v.Validate(ctx, map[string]string{
		"corp_id": "ww180",
		"secret":  "secret789",
	})

	if result.Success {
		t.Error("expected timeout or connection failure")
	}
}

func TestWeChatValidator_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"corp_id": "ww180",
		"secret":  "secret789",
	})

	if result.Success {
		t.Error("expected failure for invalid JSON")
	}
	if result.MessageKey != "connectionFailed" {
		t.Errorf("expected MessageKey 'connectionFailed', got '%s'", result.MessageKey)
	}
}

func TestWeChatValidator_NetworkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	serverURL := server.URL
	server.Close()

	v := NewValidatorWithOptions(1*time.Second, serverURL)
	result := v.Validate(context.Background(), map[string]string{
		"corp_id": "ww180",
		"secret":  "secret789",
	})

	if result.Success {
		t.Error("expected connection failure")
	}
	if result.MessageKey != "connectionFailed" {
		t.Errorf("expected MessageKey 'connectionFailed', got '%s'", result.MessageKey)
	}
}
