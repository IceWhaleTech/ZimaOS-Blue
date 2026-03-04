package providerpool

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPHealthChecker_CloudCode401WithoutAPIKeyIsReachable(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
