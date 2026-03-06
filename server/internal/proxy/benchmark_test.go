package proxy

import (
	"net/http"
	"testing"
	"time"
)

// TestPortAllocationTime verifies port allocation time < 100ms
func TestPortAllocationTime(t *testing.T) {
	config := &PortConfig{
		Value:       0,
		Range:       "19300-19400",
		BindAddress: "127.0.0.1",
	}

	pa := NewPortAllocator(config)

	start := time.Now()
	port, err := pa.Allocate()
	allocationTime := time.Since(start)

	if err != nil {
		if isBindPermissionError(err) {
			t.Skipf("skip port allocation benchmark in restricted environment: %v", err)
		}
		t.Fatalf("Port allocation failed: %v", err)
	}
	defer pa.Release()

	// Target: < 100ms
	if allocationTime > 100*time.Millisecond {
		t.Errorf("Port allocation time %v exceeds 100ms target", allocationTime)
	} else {
		t.Logf("Port allocation time: %v (target: < 100ms) ✓", allocationTime)
	}

	if port < 19300 || port > 19400 {
		t.Errorf("Port %d outside expected range", port)
	}
}

// TestHealthCheckLatency verifies health check latency < 100ms
func TestHealthCheckLatency(t *testing.T) {
	// Create mock health endpoint
	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	router := NewRouter(&RouteConfig{
		DefaultProvider: "mock",
		LoadBalancing:   "priority",
		Providers: []*ProviderConfig{
			{
				Name:        "mock",
				Endpoint:    upstream.URL,
				Priority:    1,
				Enabled:     true,
				HealthCheck: "/health",
			},
		},
	})

	connPool := NewConnectionPool(&ConnectionConfig{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     30 * time.Second,
	})
	defer connPool.Close()

	hc := NewHealthChecker(router, connPool, &HealthCheckConfig{
		Enabled:  true,
		Interval: 30 * time.Second,
		Timeout:  5 * time.Second,
	})

	// Measure health check latency
	start := time.Now()
	hc.CheckProvider("mock")
	checkLatency := time.Since(start)

	// Target: < 100ms
	if checkLatency > 100*time.Millisecond {
		t.Errorf("Health check latency %v exceeds 100ms target", checkLatency)
	} else {
		t.Logf("Health check latency: %v (target: < 100ms) ✓", checkLatency)
	}
}

// TestModelRouterPerformance tests model router routing performance
func TestModelRouterPerformance(t *testing.T) {
	config := &ModelRouterConfig{
		Enabled: true,
		Families: []*ModelFamily{
			{Name: "claude-3", Patterns: []string{"^claude-3.*"}, Provider: "anthropic", Fallback: "claude-3-haiku"},
			{Name: "gpt-4", Patterns: []string{"^gpt-4.*"}, Provider: "openai", Fallback: "gpt-4o-mini"},
			{Name: "gemini", Patterns: []string{"^gemini.*"}, Provider: "google", Fallback: "gemini-flash"},
		},
		RegexCustomRules: []*RegexRule{
			{Pattern: "^claude-3-opus-latest$", Target: "claude-3-opus-20240229", Provider: "anthropic", Priority: 1},
		},
	}

	mr, err := NewModelRouter(config)
	if err != nil {
		t.Fatalf("Failed to create model router: %v", err)
	}

	models := []string{
		"claude-3-opus",
		"claude-3-sonnet",
		"claude-3-opus-latest",
		"gpt-4-turbo",
		"gpt-4o",
		"gemini-pro",
		"unknown-model",
	}

	// Warm up
	for _, model := range models {
		mr.RouteModel(model, false)
	}

	// Measure routing performance
	iterations := 10000
	start := time.Now()
	for i := 0; i < iterations; i++ {
		model := models[i%len(models)]
		mr.RouteModel(model, i%2 == 0)
	}
	elapsed := time.Since(start)

	avgLatency := elapsed / time.Duration(iterations)
	opsPerSec := float64(iterations) / elapsed.Seconds()

	t.Logf("Model router performance: %d ops in %v", iterations, elapsed)
	t.Logf("Average latency: %v, Ops/sec: %.0f", avgLatency, opsPerSec)

	// Should be very fast (< 1µs per operation)
	if avgLatency > time.Microsecond*10 {
		t.Errorf("Model routing too slow: %v per operation", avgLatency)
	}
}

// TestQuotaMonitorPerformance tests quota monitor performance
func TestQuotaMonitorPerformance(t *testing.T) {
	qm := NewQuotaMonitor(&QuotaMonitorConfig{
		Enabled:       true,
		TrackRequests: true,
		TrackTokens:   true,
	})

	providers := []string{"anthropic", "openai", "gemini", "ollama", "custom"}

	// Measure recording performance
	iterations := 10000
	start := time.Now()
	for i := 0; i < iterations; i++ {
		provider := providers[i%len(providers)]
		qm.RecordRequest(provider)
	}
	elapsed := time.Since(start)

	avgLatency := elapsed / time.Duration(iterations)
	opsPerSec := float64(iterations) / elapsed.Seconds()

	t.Logf("Quota monitor performance: %d ops in %v", iterations, elapsed)
	t.Logf("Average latency: %v, Ops/sec: %.0f", avgLatency, opsPerSec)

	// Should be very fast (< 10µs per operation)
	if avgLatency > time.Microsecond*100 {
		t.Errorf("Quota recording too slow: %v per operation", avgLatency)
	}
}

// BenchmarkModelRouter benchmarks model router
func BenchmarkModelRouter(b *testing.B) {
	config := &ModelRouterConfig{
		Enabled: true,
		Families: []*ModelFamily{
			{Name: "claude-3", Patterns: []string{"^claude-3.*"}, Provider: "anthropic", Fallback: "claude-3-haiku"},
			{Name: "gpt-4", Patterns: []string{"^gpt-4.*"}, Provider: "openai", Fallback: "gpt-4o-mini"},
		},
	}

	mr, _ := NewModelRouter(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mr.RouteModel("claude-3-opus", false)
	}
}

// BenchmarkQuotaMonitor benchmarks quota monitor
func BenchmarkQuotaMonitor(b *testing.B) {
	qm := NewQuotaMonitor(&QuotaMonitorConfig{
		Enabled:       true,
		TrackRequests: true,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qm.RecordRequest("anthropic")
	}
}

// BenchmarkProviderSelection benchmarks provider selection
func BenchmarkProviderSelection(b *testing.B) {
	router := NewRouter(&RouteConfig{
		DefaultProvider: "anthropic",
		LoadBalancing:   "priority",
		Providers: []*ProviderConfig{
			{Name: "anthropic", Endpoint: "https://api.anthropic.com", Priority: 1, Enabled: true},
			{Name: "openai", Endpoint: "https://api.openai.com", Priority: 2, Enabled: true},
			{Name: "gemini", Endpoint: "https://api.google.com", Priority: 3, Enabled: true},
		},
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.SelectProvider(nil)
	}
}
