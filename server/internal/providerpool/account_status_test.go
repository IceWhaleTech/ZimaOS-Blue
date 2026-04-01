package providerpool

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchProviderAccountStatus_OpenRouterParsesCredits(t *testing.T) {
	restore := installAccountStatusTransport(func(r *http.Request) (int, string) {
		if r.URL.Path != "/api/v1/key" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/api/v1/key")
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-openrouter" {
			t.Fatalf("authorization = %q, want %q", got, "Bearer sk-openrouter")
		}
		return http.StatusOK, `{"data":{"usage":12.5,"limit":25,"limit_remaining":12.5}}`
	})
	defer restore()

	status := FetchProviderAccountStatus(context.Background(), &Provider{
		ID:      "openrouter",
		BaseURL: "https://openrouter.ai/api",
		APIKeys: []APIKey{
			{ID: "key-1", Key: "sk-openrouter", KeyHash: "sk-or-1234", Enabled: true},
		},
	}, "")

	if status.Kind != ProviderAccountStatusKindCredits {
		t.Fatalf("kind = %q, want %q", status.Kind, ProviderAccountStatusKindCredits)
	}
	if status.KeyID != "key-1" {
		t.Fatalf("key_id = %q, want %q", status.KeyID, "key-1")
	}
	if status.KeyHash != "sk-or-1234" {
		t.Fatalf("key_hash = %q, want %q", status.KeyHash, "sk-or-1234")
	}
	if status.PrimaryItemKey != "remaining" {
		t.Fatalf("primary_item_key = %q, want %q", status.PrimaryItemKey, "remaining")
	}
	if len(status.Items) != 3 {
		t.Fatalf("items len = %d, want 3", len(status.Items))
	}
	if status.Items[0].Key != "remaining" || status.Items[0].Value != 12.5 {
		t.Fatalf("remaining item = %+v, want key=remaining value=12.5", status.Items[0])
	}
}

func TestFetchProviderAccountStatus_DeepSeekParsesBalance(t *testing.T) {
	restore := installAccountStatusTransport(func(r *http.Request) (int, string) {
		if r.URL.Path != "/user/balance" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/user/balance")
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-deepseek" {
			t.Fatalf("authorization = %q, want %q", got, "Bearer sk-deepseek")
		}
		return http.StatusOK, `{"is_available":true,"balance_infos":[{"currency":"CNY","total_balance":"15.500000","granted_balance":"2.000000","topped_up_balance":"13.500000"}]}`
	})
	defer restore()

	status := FetchProviderAccountStatus(context.Background(), &Provider{
		ID:      "deepseek",
		BaseURL: "https://api.deepseek.com",
		APIKeys: []APIKey{
			{ID: "key-1", Key: "sk-deepseek", KeyHash: "sk-ds-1234", Enabled: true},
		},
	}, "")

	if status.Kind != ProviderAccountStatusKindBalance {
		t.Fatalf("kind = %q, want %q", status.Kind, ProviderAccountStatusKindBalance)
	}
	if len(status.Items) != 3 {
		t.Fatalf("items len = %d, want 3", len(status.Items))
	}
	if status.Items[0].Key != "remaining" || status.Items[0].Value != 15.5 {
		t.Fatalf("remaining item = %+v, want key=remaining value=15.5", status.Items[0])
	}
	if status.Items[1].Key != "granted" || status.Items[1].Value != 2 {
		t.Fatalf("granted item = %+v, want key=granted value=2", status.Items[1])
	}
	if status.Items[2].Key != "topped_up" || status.Items[2].Value != 13.5 {
		t.Fatalf("topped_up item = %+v, want key=topped_up value=13.5", status.Items[2])
	}
}

func TestGetProviderAccountStatusUsesRequestedKey(t *testing.T) {
	restore := installAccountStatusTransport(func(r *http.Request) (int, string) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-second" {
			t.Fatalf("authorization = %q, want %q", got, "Bearer sk-second")
		}
		return http.StatusOK, `{"data":{"usage":3,"limit_remaining":9}}`
	})
	defer restore()

	h, e := newTestProviderHandler(t)
	if err := h.pool.Registry.Register(&Provider{
		ID:      "openrouter",
		Name:    "OpenRouter",
		Type:    ProviderTypePlatform,
		BaseURL: "https://openrouter.ai/api",
		APIKeys: []APIKey{
			{ID: "key-1", Key: "sk-first", KeyHash: "sk-or-first", Enabled: true},
			{ID: "key-2", Key: "sk-second", KeyHash: "sk-or-second", Enabled: true},
		},
	}); err != nil {
		t.Fatalf("register provider failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/providers/openrouter/account/status?key_id=key-2", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var status ProviderAccountStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if status.KeyID != "key-2" {
		t.Fatalf("key_id = %q, want %q", status.KeyID, "key-2")
	}
	if status.KeyHash != "sk-or-second" {
		t.Fatalf("key_hash = %q, want %q", status.KeyHash, "sk-or-second")
	}
}

func installAccountStatusTransport(fn func(*http.Request) (int, string)) func() {
	oldFactory := newAccountStatusHTTPClient
	newAccountStatusHTTPClient = func() *http.Client {
		return &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				status, body := fn(r)
				return &http.Response{
					StatusCode: status,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(body)),
					Request:    r,
				}, nil
			}),
		}
	}
	return func() {
		newAccountStatusHTTPClient = oldFactory
	}
}
