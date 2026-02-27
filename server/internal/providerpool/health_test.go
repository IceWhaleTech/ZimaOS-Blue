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
