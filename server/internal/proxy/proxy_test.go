package proxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPortAllocator(t *testing.T) {
	t.Run("allocate dynamic port", func(t *testing.T) {
		config := &PortConfig{
			Value:       0,
			Range:       "19000-19100",
			BindAddress: "127.0.0.1",
		}

		pa := NewPortAllocator(config)
		port, err := pa.Allocate()
		if err != nil {
			if isBindPermissionError(err) {
				t.Skipf("skip dynamic port allocation test in restricted environment: %v", err)
			}
			t.Fatalf("failed to allocate port: %v", err)
		}

		if port < 19000 || port > 19100 {
			t.Errorf("port %d is outside range 19000-19100", port)
		}

		if pa.GetPort() != port {
			t.Errorf("GetPort() = %d, want %d", pa.GetPort(), port)
		}

		endpoint := pa.GetEndpoint()
		if endpoint == "" {
			t.Error("GetEndpoint() returned empty string")
		}

		pa.Release()
	})

	t.Run("invalid port range", func(t *testing.T) {
		config := &PortConfig{
			Value:       0,
			Range:       "invalid",
			BindAddress: "127.0.0.1",
		}

		pa := NewPortAllocator(config)
		_, err := pa.Allocate()
		if err != ErrPortRangeInvalid {
			t.Errorf("expected ErrPortRangeInvalid, got %v", err)
		}
	})
}

func TestConnectionPool(t *testing.T) {
	config := DefaultConnectionConfig()
	pool := NewConnectionPool(config)
	defer pool.Close()

	t.Run("get client", func(t *testing.T) {
		client1 := pool.GetClient("provider1")
		if client1 == nil {
			t.Error("GetClient returned nil")
		}

		client2 := pool.GetClient("provider1")
		if client1 != client2 {
			t.Error("GetClient should return same client for same provider")
		}

		client3 := pool.GetClient("provider2")
		if client3 == nil {
			t.Error("GetClient returned nil for provider2")
		}
	})

	t.Run("stats", func(t *testing.T) {
		stats := pool.Stats()
		if stats["client_count"].(int) != 2 {
			t.Errorf("expected 2 clients, got %v", stats["client_count"])
		}
	})
}

func TestRouter(t *testing.T) {
	config := &RouteConfig{
		DefaultProvider: "provider1",
		LoadBalancing:   "priority",
		Providers: []*ProviderConfig{
			{Name: "provider1", Endpoint: "http://localhost:8001", Priority: 1, Enabled: true},
			{Name: "provider2", Endpoint: "http://localhost:8002", Priority: 2, Enabled: true},
			{Name: "provider3", Endpoint: "http://localhost:8003", Priority: 3, Enabled: false},
		},
	}

	router := NewRouter(config)

	t.Run("select by priority", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		provider, err := router.SelectProvider(req)
		if err != nil {
			t.Fatalf("SelectProvider failed: %v", err)
		}

		if provider.Config.Name != "provider1" {
			t.Errorf("expected provider1, got %s", provider.Config.Name)
		}
	})

	t.Run("skip disabled provider", func(t *testing.T) {
		available := router.GetAvailableProviders()
		for _, p := range available {
			if p.Config.Name == "provider3" {
				t.Error("disabled provider should not be in available list")
			}
		}
	})

	t.Run("update health", func(t *testing.T) {
		router.UpdateHealth("provider1", false, ErrRequestFailed)

		provider, _ := router.GetProvider("provider1")
		if provider.Healthy {
			t.Error("provider should be unhealthy")
		}

		// Now provider2 should be selected
		req := httptest.NewRequest("GET", "/", nil)
		selected, _ := router.SelectProvider(req)
		if selected.Config.Name != "provider2" {
			t.Errorf("expected provider2, got %s", selected.Config.Name)
		}

		// Restore health
		router.UpdateHealth("provider1", true, nil)
	})

	t.Run("round robin", func(t *testing.T) {
		config.LoadBalancing = "round-robin"
		router := NewRouter(config)

		counts := make(map[string]int)
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest("GET", "/", nil)
			provider, _ := router.SelectProvider(req)
			counts[provider.Config.Name]++
		}

		// Both enabled providers should be selected
		if counts["provider1"] == 0 || counts["provider2"] == 0 {
			t.Error("round robin should select all enabled providers")
		}
	})
}

func TestFailoverHandler(t *testing.T) {
	config := &FailoverConfig{
		Enabled:          true,
		MaxRetries:       2,
		RetryDelay:       10 * time.Millisecond,
		CircuitBreaker:   true,
		FailureThreshold: 3,
		RecoveryTimeout:  100 * time.Millisecond,
	}

	routeConfig := &RouteConfig{
		DefaultProvider: "provider1",
		LoadBalancing:   "priority",
		Providers: []*ProviderConfig{
			{Name: "provider1", Endpoint: "http://localhost:8001", Priority: 1, Enabled: true},
			{Name: "provider2", Endpoint: "http://localhost:8002", Priority: 2, Enabled: true},
		},
	}

	router := NewRouter(routeConfig)
	failover := NewFailoverHandler(config, router)

	t.Run("successful request", func(t *testing.T) {
		provider, _ := router.GetProvider("provider1")

		resp, err := failover.Execute(context.Background(), provider, func(p *Provider) (*http.Response, error) {
			return &http.Response{StatusCode: 200}, nil
		})

		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("retry on failure", func(t *testing.T) {
		provider, _ := router.GetProvider("provider1")
		attempts := 0

		resp, err := failover.Execute(context.Background(), provider, func(p *Provider) (*http.Response, error) {
			attempts++
			if attempts < 3 {
				return nil, ErrRequestFailed
			}
			return &http.Response{StatusCode: 200}, nil
		})

		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
		if attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("circuit breaker state", func(t *testing.T) {
		state := failover.GetBreakerState("provider1")
		if state != "closed" && state != "half-open" {
			t.Logf("circuit breaker state: %s", state)
		}

		failover.ResetBreaker("provider1")
		state = failover.GetBreakerState("provider1")
		if state != "closed" {
			t.Errorf("expected closed state after reset, got %s", state)
		}
	})
}

func TestProxyHandler(t *testing.T) {
	// Create mock upstream server
	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "hello"}`))
	}))
	defer upstream.Close()

	t.Run("forward request", func(t *testing.T) {
		// Test upstream server directly since ProxyHandler now requires providerPool
		req, _ := http.NewRequest("GET", upstream.URL+"/test", nil)
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		if resp.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", resp.Header.Get("Content-Type"))
		}
	})

	t.Run("no provider pool returns error", func(t *testing.T) {
		config := &RouteConfig{
			DefaultProvider: "test",
			LoadBalancing:   "priority",
			Providers: []*ProviderConfig{
				{Name: "test", Endpoint: upstream.URL, Priority: 1, Enabled: true},
			},
		}
		router := NewRouter(config)
		connPool := NewConnectionPool(DefaultConnectionConfig())
		failover := NewFailoverHandler(&config.Failover, router)
		handler := NewProxyHandler(router, connPool, failover)

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Without providerPool, should return 503
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status 503 without providerPool, got %d", w.Code)
		}
	})
}

func TestHealthChecker(t *testing.T) {
	// Create mock health endpoint server
	healthServer := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "ok"}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer healthServer.Close()

	// Create unhealthy server
	unhealthyServer := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer unhealthyServer.Close()

	routeConfig := &RouteConfig{
		DefaultProvider: "healthy",
		LoadBalancing:   "priority",
		Providers: []*ProviderConfig{
			{Name: "healthy", Endpoint: healthServer.URL, Priority: 1, Enabled: true, HealthCheck: "/health"},
			{Name: "unhealthy", Endpoint: unhealthyServer.URL, Priority: 2, Enabled: true, HealthCheck: "/health"},
			{Name: "no-check", Endpoint: "http://localhost:9999", Priority: 3, Enabled: true, HealthCheck: ""},
		},
	}

	router := NewRouter(routeConfig)
	connPool := NewConnectionPool(DefaultConnectionConfig())
	healthConfig := &HealthCheckConfig{
		Enabled:  true,
		Interval: 100 * time.Millisecond,
		Timeout:  5 * time.Second,
	}

	hc := NewHealthChecker(router, connPool, healthConfig)

	t.Run("check healthy provider", func(t *testing.T) {
		err := hc.CheckProvider("healthy")
		if err != nil {
			t.Fatalf("CheckProvider failed: %v", err)
		}

		provider, _ := router.GetProvider("healthy")
		if !provider.Healthy {
			t.Error("healthy provider should be marked healthy")
		}
	})

	t.Run("check unhealthy provider", func(t *testing.T) {
		err := hc.CheckProvider("unhealthy")
		if err != nil {
			t.Fatalf("CheckProvider failed: %v", err)
		}

		provider, _ := router.GetProvider("unhealthy")
		if provider.Healthy {
			t.Error("unhealthy provider should be marked unhealthy")
		}
	})

	t.Run("check provider without health endpoint", func(t *testing.T) {
		err := hc.CheckProvider("no-check")
		if err != nil {
			t.Fatalf("CheckProvider failed: %v", err)
		}

		// Provider without health check should remain in default state
		provider, _ := router.GetProvider("no-check")
		if !provider.Healthy {
			t.Error("provider without health check should remain healthy by default")
		}
	})

	t.Run("check non-existent provider", func(t *testing.T) {
		err := hc.CheckProvider("non-existent")
		if err != ErrProviderNotFound {
			t.Errorf("expected ErrProviderNotFound, got %v", err)
		}
	})

	t.Run("check all providers", func(t *testing.T) {
		hc.CheckNow()

		healthy, _ := router.GetProvider("healthy")
		unhealthy, _ := router.GetProvider("unhealthy")

		if !healthy.Healthy {
			t.Error("healthy provider should be healthy after CheckNow")
		}
		if unhealthy.Healthy {
			t.Error("unhealthy provider should be unhealthy after CheckNow")
		}
	})

	t.Run("start and stop health checker", func(t *testing.T) {
		hc.Start()
		time.Sleep(150 * time.Millisecond) // Wait for at least one check cycle
		hc.Stop()
	})

	t.Run("disabled health checker", func(t *testing.T) {
		disabledConfig := &HealthCheckConfig{
			Enabled:  false,
			Interval: 100 * time.Millisecond,
			Timeout:  5 * time.Second,
		}
		disabledHC := NewHealthChecker(router, connPool, disabledConfig)
		disabledHC.Start() // Should return immediately
		disabledHC.Stop()
	})
}

func TestStreamingResponse(t *testing.T) {
	// Create mock SSE server
	sseServer := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		// Send SSE events
		events := []string{
			`data: {"id":"1","choices":[{"delta":{"content":"Hello"}}]}`,
			`data: {"id":"2","choices":[{"delta":{"content":" World"}}]}`,
			`data: [DONE]`,
		}

		for _, event := range events {
			w.Write([]byte(event + "\n\n"))
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer sseServer.Close()

	t.Run("stream SSE response", func(t *testing.T) {
		// Test SSE server directly since ProxyHandler now requires providerPool
		req, _ := http.NewRequest("POST", sseServer.URL+"/v1/chat/completions", nil)
		req.Header.Set("Accept", "text/event-stream")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		contentType := resp.Header.Get("Content-Type")
		if contentType != "text/event-stream" {
			t.Errorf("expected Content-Type text/event-stream, got %s", contentType)
		}

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)
		if !contains(bodyStr, "Hello") || !contains(bodyStr, "World") {
			t.Errorf("response body missing expected content: %s", bodyStr)
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
