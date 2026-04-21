package wechatilink

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestValidator_MissingAPIBaseURL(t *testing.T) {
	v := NewValidator()
	result := v.Validate(context.Background(), map[string]string{
		"bot_token": "bot-token",
	})

	if result.Success {
		t.Error("expected failure for missing api_base_url")
	}
	if result.MessageKey != "apiBaseURLRequired" {
		t.Errorf("expected MessageKey 'apiBaseURLRequired', got '%s'", result.MessageKey)
	}
}

func TestValidator_MissingBotToken(t *testing.T) {
	v := NewValidator()
	result := v.Validate(context.Background(), map[string]string{
		"api_base_url": "https://ilink.example.com",
	})

	if result.Success {
		t.Error("expected failure for missing bot_token")
	}
	if result.MessageKey != "botTokenRequired" {
		t.Errorf("expected MessageKey 'botTokenRequired', got '%s'", result.MessageKey)
	}
}

func TestValidator_ValidCredentials(t *testing.T) {
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/getupdates" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if got := r.Header.Get("AuthorizationType"); got != "ilink_bot_token" {
			t.Fatalf("AuthorizationType = %q, want %q", got, "ilink_bot_token")
		}
		if got := r.Header.Get("Authorization"); got != "Bearer bot-token" {
			t.Fatalf("Authorization = %q, want Bearer bot-token", got)
		}
		if got := r.Header.Get("X-WECHAT-UIN"); strings.TrimSpace(got) == "" {
			t.Fatal("expected X-WECHAT-UIN header to be set")
		}
		if got := r.Header.Get("iLink-App-Id"); got != "bot" {
			t.Fatalf("iLink-App-Id = %q, want %q", got, "bot")
		}
		if got := r.Header.Get("iLink-App-ClientVersion"); strings.TrimSpace(got) == "" {
			t.Fatal("expected iLink-App-ClientVersion header to be set")
		}

		var req struct {
			GetUpdatesBuf string `json:"get_updates_buf"`
			BaseInfo      struct {
				ChannelVersion string `json:"channel_version"`
			} `json:"base_info"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.GetUpdatesBuf != "" {
			t.Fatalf("get_updates_buf = %q, want empty", req.GetUpdatesBuf)
		}
		if strings.TrimSpace(req.BaseInfo.ChannelVersion) == "" {
			t.Fatal("expected base_info.channel_version to be set")
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"ret":             0,
			"msgs":            []any{},
			"get_updates_buf": "cursor-1",
		})
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"api_base_url": server.URL,
		"bot_token":    "bot-token",
	})

	if !result.Success {
		t.Errorf("expected success, got error: %s - %s", result.MessageKey, result.Error)
	}
	if result.MessageKey != "testSuccess" {
		t.Errorf("expected MessageKey 'testSuccess', got '%s'", result.MessageKey)
	}
	if result.Data["api_base_url"] != server.URL {
		t.Errorf("expected api_base_url %q, got '%v'", server.URL, result.Data["api_base_url"])
	}
}

func TestValidator_Validate_UsesDecimalStringUINHeader(t *testing.T) {
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/getupdates" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		_ = decodeILinkUINHeader(t, r.Header.Get("X-WECHAT-UIN"))

		_ = json.NewEncoder(w).Encode(map[string]any{
			"ret":             0,
			"msgs":            []any{},
			"get_updates_buf": "cursor-1",
		})
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"api_base_url": server.URL,
		"bot_token":    "bot-token",
	})

	if !result.Success {
		t.Fatalf("expected success, got error: %s - %s", result.MessageKey, result.Error)
	}
}
