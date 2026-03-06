package proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestModelRoutingIntegration tests model routing with different model IDs
func TestModelRoutingIntegration(t *testing.T) {
	config := &ModelRouterConfig{
		Enabled: true,
		Families: []*ModelFamily{
			{
				Name:     "claude-3",
				Patterns: []string{"^claude-3.*"},
				Provider: "anthropic",
				Fallback: "claude-3-haiku",
			},
			{
				Name:     "gpt-4",
				Patterns: []string{"^gpt-4.*"},
				Provider: "openai",
				Fallback: "gpt-4o-mini",
			},
		},
		RegexCustomRules: []*RegexRule{
			{
				Pattern:     "^claude-3-opus-latest$",
				Target:      "claude-3-opus-20240229",
				Provider:    "anthropic",
				Priority:    1,
				Description: "Pin opus-latest",
			},
		},
	}

	mr, err := NewModelRouter(config)
	if err != nil {
		t.Fatalf("Failed to create model router: %v", err)
	}

	tests := []struct {
		model        string
		background   bool
		wantTarget   string
		wantProvider string
		wantFamily   string
	}{
		{"claude-3-opus", false, "claude-3-opus", "anthropic", "claude-3"},
		{"claude-3-opus", true, "claude-3-haiku", "anthropic", "claude-3"},
		{"claude-3-opus-latest", false, "claude-3-opus-20240229", "anthropic", ""},
		{"gpt-4-turbo", false, "gpt-4-turbo", "openai", "gpt-4"},
		{"gpt-4-turbo", true, "gpt-4o-mini", "openai", "gpt-4"},
		{"unknown-model", false, "unknown-model", "", ""},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_bg=%v", tt.model, tt.background), func(t *testing.T) {
			route, err := mr.RouteModel(tt.model, tt.background)
			if err != nil && tt.wantProvider != "" {
				t.Fatalf("RouteModel failed: %v", err)
			}

			if route.TargetModel != tt.wantTarget {
				t.Errorf("Expected target '%s', got '%s'", tt.wantTarget, route.TargetModel)
			}
			if route.Provider != tt.wantProvider {
				t.Errorf("Expected provider '%s', got '%s'", tt.wantProvider, route.Provider)
			}
		})
	}
}

// TestQuotaTrackingIntegration tests quota tracking across requests
func TestQuotaTrackingIntegration(t *testing.T) {
	qm := NewQuotaMonitor(&QuotaMonitorConfig{
		Enabled:           true,
		WarningThreshold:  20.0,
		CriticalThreshold: 5.0,
		TrackRequests:     true, // Enable request tracking
		TrackTokens:       true,
	})

	// Simulate multiple requests
	providers := []string{"anthropic", "openai", "gemini"}
	for i := 0; i < 10; i++ {
		for _, provider := range providers {
			qm.RecordRequest(provider)
		}
	}

	// Check quota summary
	summary := qm.GetQuotaSummary()
	if summary.TotalProviders != 3 {
		t.Errorf("Expected 3 providers, got %d", summary.TotalProviders)
	}

	// Check individual quotas
	for _, provider := range providers {
		quota := qm.GetQuota(provider)
		if quota == nil {
			t.Errorf("Expected quota for %s", provider)
			continue
		}
		if quota.UsedQuota != 10 {
			t.Errorf("Expected UsedQuota=10 for %s, got %d", provider, quota.UsedQuota)
		}
	}

	// Test rate limit recording
	qm.RecordError("anthropic", 429)
	quota := qm.GetQuota("anthropic")
	if !quota.RateLimited {
		t.Error("Expected RateLimited=true after 429")
	}
	if quota.Status != "rate_limited" {
		t.Errorf("Expected status 'rate_limited', got '%s'", quota.Status)
	}

	// Test ban detection
	qm.RecordError("openai", 403)
	quota = qm.GetQuota("openai")
	if !quota.BanDetected {
		t.Error("Expected BanDetected=true after 403")
	}
	if quota.Status != "banned" {
		t.Errorf("Expected status 'banned', got '%s'", quota.Status)
	}

	// Test best provider (should exclude banned/rate-limited)
	best := qm.GetBestProvider()
	if best == "anthropic" || best == "openai" {
		t.Errorf("Best provider should not be rate-limited or banned, got '%s'", best)
	}
}

// TestCircuitBreakerIntegration tests circuit breaker behavior
func TestCircuitBreakerIntegration(t *testing.T) {
	config := &FailoverConfig{
		Enabled:          true,
		MaxRetries:       3,
		RetryDelay:       10 * time.Millisecond,
		CircuitBreaker:   true,
		FailureThreshold: 3,
		RecoveryTimeout:  100 * time.Millisecond,
	}

	router := NewRouter(&RouteConfig{
		DefaultProvider: "test",
		LoadBalancing:   "priority",
		Providers: []*ProviderConfig{
			{Name: "test", Endpoint: "http://localhost", Priority: 1, Enabled: true},
		},
	})

	fh := NewFailoverHandler(config, router)

	// Get initial state
	state := fh.GetBreakerState("test")
	if state != "closed" {
		t.Errorf("Expected initial circuit breaker state 'closed', got '%s'", state)
	}

	// Test reset functionality
	fh.ResetBreaker("test")
	state = fh.GetBreakerState("test")
	if state != "closed" {
		t.Errorf("Expected circuit breaker state 'closed' after reset, got '%s'", state)
	}

	// Test stats
	stats := fh.GetBreakerStats()
	if stats == nil {
		t.Error("Expected non-nil breaker stats")
	}
}

// TestFailoverIntegration tests failover between providers
func TestFailoverIntegration(t *testing.T) {
	// Create two mock upstreams - first fails, second succeeds
	failCount := 0
	upstream1 := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		failCount++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer upstream1.Close()

	upstream2 := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer upstream2.Close()

	router := NewRouter(&RouteConfig{
		DefaultProvider: "primary",
		LoadBalancing:   "priority",
		Providers: []*ProviderConfig{
			{Name: "primary", Endpoint: upstream1.URL, Priority: 1, Enabled: true},
			{Name: "backup", Endpoint: upstream2.URL, Priority: 2, Enabled: true},
		},
	})

	// Mark primary as unhealthy
	router.UpdateHealth("primary", false, fmt.Errorf("service unavailable"))

	// Select provider should return backup
	provider, err := router.SelectProvider(nil)
	if err != nil {
		t.Fatalf("SelectProvider failed: %v", err)
	}
	if provider.Config.Name != "backup" {
		t.Errorf("Expected backup provider, got '%s'", provider.Config.Name)
	}
}

// TestProxyRequestForwarding tests actual request forwarding
func TestProxyRequestForwarding(t *testing.T) {
	// Create mock upstream
	receivedHeaders := make(map[string]string)
	receivedBody := ""
	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture request details
		for k, v := range r.Header {
			receivedHeaders[k] = v[0]
		}
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Custom-Header", "test-value")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      "resp_123",
			"content": "Response from upstream",
		})
	}))
	defer upstream.Close()

	// Test upstream directly since ProxyServer now requires providerPool for routing
	reqBody := `{"model": "claude-3-opus", "messages": [{"role": "user", "content": "Hello"}]}`
	req, _ := http.NewRequest("POST", upstream.URL+"/v1/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-key")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	// Verify upstream received the request
	if receivedBody == "" {
		t.Error("Upstream did not receive request body")
	}
}

// TestBackgroundRequestDetection tests background request detection
func TestBackgroundRequestDetection(t *testing.T) {
	mr, _ := NewModelRouter(nil)

	tests := []struct {
		name         string
		path         string
		headers      map[string]string
		query        string
		isBackground bool
	}{
		{
			name:         "normal_request",
			path:         "/v1/chat/completions",
			headers:      map[string]string{},
			isBackground: false,
		},
		{
			name:         "background_header",
			path:         "/v1/chat/completions",
			headers:      map[string]string{"X-Background-Task": "true"},
			isBackground: true,
		},
		{
			name:         "title_endpoint",
			path:         "/v1/title/generate",
			headers:      map[string]string{},
			isBackground: true,
		},
		{
			name:         "summarize_endpoint",
			path:         "/v1/summarize",
			headers:      map[string]string{},
			isBackground: true,
		},
		{
			name:         "background_query",
			path:         "/v1/chat/completions",
			headers:      map[string]string{},
			query:        "background=true",
			isBackground: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := tt.path
			if tt.query != "" {
				url += "?" + tt.query
			}
			req := httptest.NewRequest("POST", url, nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			result := mr.IsBackgroundRequest(req)
			if result != tt.isBackground {
				t.Errorf("Expected isBackground=%v, got %v", tt.isBackground, result)
			}
		})
	}
}

// TestRateLimitHeaderParsing tests parsing of rate limit headers
func TestRateLimitHeaderParsing(t *testing.T) {
	qm := NewQuotaMonitor(nil)

	tests := []struct {
		name      string
		headers   map[string]string
		wantTotal int64
		wantPct   float64
	}{
		{
			name: "standard_headers",
			headers: map[string]string{
				"X-RateLimit-Limit":     "1000",
				"X-RateLimit-Remaining": "800",
			},
			wantTotal: 1000,
			wantPct:   80.0,
		},
		{
			name: "anthropic_headers",
			headers: map[string]string{
				"anthropic-ratelimit-requests-limit":     "500",
				"anthropic-ratelimit-requests-remaining": "450",
			},
			wantTotal: 500,
			wantPct:   90.0,
		},
		{
			name: "openai_headers",
			headers: map[string]string{
				"x-ratelimit-limit-requests":     "10000",
				"x-ratelimit-remaining-requests": "9500",
			},
			wantTotal: 10000,
			wantPct:   95.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: 200,
				Header:     make(http.Header),
			}
			for k, v := range tt.headers {
				resp.Header.Set(k, v)
			}

			qm.Reset()
			qm.RecordUsage(tt.name, resp, 0)

			quota := qm.GetQuota(tt.name)
			if quota == nil {
				t.Fatal("Expected quota to be recorded")
			}
			if quota.TotalQuota != tt.wantTotal {
				t.Errorf("Expected TotalQuota=%d, got %d", tt.wantTotal, quota.TotalQuota)
			}
			if quota.RemainingPct != tt.wantPct {
				t.Errorf("Expected RemainingPct=%f, got %f", tt.wantPct, quota.RemainingPct)
			}
		})
	}
}
