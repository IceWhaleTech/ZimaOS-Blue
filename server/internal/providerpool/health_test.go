package providerpool

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPHealthChecker_CloudCode401WithoutAPIKeyIsReachable(t *testing.T) {
	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer upstream.Close()

	checker := NewHTTPHealthChecker(2 * time.Second)
	provider := &Provider{
		ID:        "google-antigravity",
		BaseURL:   upstream.URL,
		APIFormat: APIFormatCloudCode,
		APIKeys:   nil,
	}

	result := checker.Check(context.Background(), provider)
	if !result.Healthy {
		t.Fatalf("expected healthy for cloudcode 401 without api key, got unhealthy: %s", result.Error)
	}
}

func TestHTTPHealthChecker_NonCloudCode401IsAuthError(t *testing.T) {
	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer upstream.Close()

	checker := NewHTTPHealthChecker(2 * time.Second)
	provider := &Provider{
		ID:        "custom-openai",
		BaseURL:   upstream.URL + "/v1",
		APIFormat: APIFormatOpenAI,
		APIKeys:   nil,
	}

	result := checker.Check(context.Background(), provider)
	if result.Healthy {
		t.Fatal("expected unhealthy for non-cloudcode 401")
	}
	if result.Error != "auth_error:401" {
		t.Fatalf("expected auth_error:401, got %q", result.Error)
	}
}

func TestHTTPHealthChecker_CustomOpenAIUsesChatEndpointWithJSONProbeBody(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotContentType string
	var gotAuthorization string
	var gotBody string

	checker := NewHTTPHealthChecker(2 * time.Second)
	checker.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			gotContentType = r.Header.Get("Content-Type")
			gotAuthorization = r.Header.Get("Authorization")
			payload, _ := io.ReadAll(r.Body)
			gotBody = string(payload)
			return &http.Response{
				StatusCode: http.StatusForbidden,
				Header:     make(http.Header),
				Body:       http.NoBody,
				Request:    r,
			}, nil
		}),
	}
	provider := &Provider{
		ID:        "custom-openai",
		Type:      ProviderTypeCustom,
		BaseURL:   "https://relay.example.com/v1",
		APIFormat: APIFormatOpenAI,
		APIKeys: []APIKey{
			{ID: "key-1", Key: "sk-test", Enabled: true},
		},
	}

	result := checker.Check(context.Background(), provider)
	if result.Healthy {
		t.Fatal("expected unhealthy custom relay for 403 auth block")
	}
	if result.Error != "auth_error:403" {
		t.Fatalf("result.Error = %q, want auth_error:403", result.Error)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want %q", gotMethod, http.MethodPost)
	}
	if gotPath != "/v1/chat/completions" {
		t.Fatalf("path = %q, want %q", gotPath, "/v1/chat/completions")
	}
	if gotContentType != "application/json" {
		t.Fatalf("content-type = %q, want application/json", gotContentType)
	}
	if gotAuthorization != "Bearer sk-test" {
		t.Fatalf("authorization = %q, want %q", gotAuthorization, "Bearer sk-test")
	}
	if gotBody != "{}" {
		t.Fatalf("body = %q, want %q", gotBody, "{}")
	}
}

func TestHTTPHealthChecker_CustomOpenAI400ProbeIsReachable(t *testing.T) {
	checker := NewHTTPHealthChecker(2 * time.Second)
	checker.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     make(http.Header),
				Body:       http.NoBody,
				Request:    r,
			}, nil
		}),
	}
	provider := &Provider{
		ID:        "custom-openai",
		Type:      ProviderTypeCustom,
		BaseURL:   "https://relay.example.com/v1",
		APIFormat: APIFormatOpenAI,
	}

	result := checker.Check(context.Background(), provider)
	if !result.Healthy {
		t.Fatalf("expected healthy for reachable 400 probe, got unhealthy: %s", result.Error)
	}
}

func TestGetHealthCheckMethod_ResponsesBaseURLVariants(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		wantURL []string
	}{
		{
			name:    "fixed endpoint base",
			baseURL: "https://chatgpt.com/backend-api/codex/responses",
			wantURL: []string{"https://chatgpt.com/backend-api/codex/responses"},
		},
		{
			name:    "base with v1 prefix",
			baseURL: "https://relay.example.com/v1",
			wantURL: []string{"https://relay.example.com/v1/responses"},
		},
		{
			name:    "root base",
			baseURL: "https://relay.example.com",
			wantURL: []string{"https://relay.example.com/v1/responses", "https://relay.example.com/responses"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &Provider{
				ID:        "custom-responses",
				BaseURL:   tt.baseURL,
				APIFormat: APIFormatResponses,
			}
			method, urls := getHealthCheckMethod(provider)
			if method != http.MethodPost {
				t.Fatalf("method = %q, want %q", method, http.MethodPost)
			}
			if len(urls) != len(tt.wantURL) {
				t.Fatalf("len(urls) = %d, want %d (urls=%v)", len(urls), len(tt.wantURL), urls)
			}
			for i := range urls {
				if urls[i] != tt.wantURL[i] {
					t.Fatalf("urls[%d] = %q, want %q", i, urls[i], tt.wantURL[i])
				}
			}
		})
	}
}

func TestGetHealthCheckMethod_MiniMaxAnthropicEndpointUsesMessages(t *testing.T) {
	provider := &Provider{
		ID:        "minimax",
		BaseURL:   "https://api.minimaxi.com/anthropic",
		APIFormat: APIFormatAnthropic,
	}

	method, urls := getHealthCheckMethod(provider)
	if method != http.MethodPost {
		t.Fatalf("method = %q, want %q", method, http.MethodPost)
	}
	want := []string{
		"https://api.minimaxi.com/anthropic/v1/messages",
		"https://api.minimax.io/anthropic/v1/messages",
	}
	if len(urls) != len(want) {
		t.Fatalf("len(urls) = %d, want %d (urls=%v)", len(urls), len(want), urls)
	}
	for i := range want {
		if urls[i] != want[i] {
			t.Fatalf("urls[%d] = %q, want %q", i, urls[i], want[i])
		}
	}
}

func TestHTTPHealthChecker_MiniMaxAnthropicUsesMessagesAndBearer(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotVersion string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotVersion = r.Header.Get("anthropic-version")
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer upstream.Close()

	checker := NewHTTPHealthChecker(2 * time.Second)
	provider := &Provider{
		ID:        "minimax",
		BaseURL:   upstream.URL + "/anthropic",
		APIFormat: APIFormatAnthropic,
		APIKeys: []APIKey{
			{ID: "key-1", Key: "sk-test", Enabled: true},
		},
	}

	result := checker.Check(context.Background(), provider)
	if result.Error != "auth_error:401" {
		t.Fatalf("result.Error = %q, want auth_error:401", result.Error)
	}
	if gotPath != "/anthropic/v1/messages" {
		t.Fatalf("path = %q, want %q", gotPath, "/anthropic/v1/messages")
	}
	if gotAuth != "Bearer sk-test" {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "Bearer sk-test")
	}
	if gotVersion != "2023-06-01" {
		t.Fatalf("anthropic-version = %q, want %q", gotVersion, "2023-06-01")
	}
}

func TestHTTPHealthChecker_CustomMiniMaxAnthropicUsesBearer(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotXAPIKey string
	var gotVersion string

	checker := NewHTTPHealthChecker(2 * time.Second)
	checker.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			gotPath = r.URL.Path
			gotAuth = r.Header.Get("Authorization")
			gotXAPIKey = r.Header.Get("x-api-key")
			gotVersion = r.Header.Get("anthropic-version")
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header:     make(http.Header),
				Body:       http.NoBody,
				Request:    r,
			}, nil
		}),
	}
	provider := &Provider{
		ID:        "custom-minimax-relay",
		Type:      ProviderTypeCustom,
		BaseURL:   "https://api.minimaxi.com/anthropic",
		APIFormat: APIFormatAnthropic,
		APIKeys: []APIKey{
			{ID: "key-1", Key: "sk-test", Enabled: true},
		},
	}

	result := checker.Check(context.Background(), provider)
	if result.Error != "auth_error:401" {
		t.Fatalf("result.Error = %q, want auth_error:401", result.Error)
	}
	if gotPath != "/anthropic/v1/messages" {
		t.Fatalf("path = %q, want %q", gotPath, "/anthropic/v1/messages")
	}
	if gotAuth != "Bearer sk-test" {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "Bearer sk-test")
	}
	if gotXAPIKey != "" {
		t.Fatalf("x-api-key = %q, want empty", gotXAPIKey)
	}
	if gotVersion != "2023-06-01" {
		t.Fatalf("anthropic-version = %q, want %q", gotVersion, "2023-06-01")
	}
}

func TestAutoSwitchBaseURL_StripsAnthropicMessagesSuffix(t *testing.T) {
	provider := &Provider{ID: "minimax", BaseURL: "https://api.minimaxi.com/anthropic"}
	autoSwitchBaseURL(provider, "https://api.minimax.io/anthropic/v1/messages")
	if provider.BaseURL != "https://api.minimax.io/anthropic" {
		t.Fatalf("BaseURL = %q, want %q", provider.BaseURL, "https://api.minimax.io/anthropic")
	}
}

func TestFirstUsableAPIKey_PrefersEnabledKey(t *testing.T) {
	provider := &Provider{
		APIKeys: []APIKey{
			{ID: "k-disabled", Key: "sk-disabled", Enabled: false},
			{ID: "k-enabled", Key: "sk-enabled", Enabled: true},
		},
	}

	key := firstUsableAPIKey(provider)
	if key == nil {
		t.Fatal("expected key, got nil")
	}
	if key.ID != "k-enabled" {
		t.Fatalf("expected k-enabled, got %s", key.ID)
	}
}
