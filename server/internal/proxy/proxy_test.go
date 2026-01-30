package proxy

import (
	"context"
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
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "hello"}`))
	}))
	defer upstream.Close()

	config := &RouteConfig{
		DefaultProvider: "test",
		LoadBalancing:   "priority",
		Providers: []*ProviderConfig{
			{Name: "test", Endpoint: upstream.URL, Priority: 1, Enabled: true},
		},
		Failover: FailoverConfig{
			Enabled:    true,
			MaxRetries: 1,
		},
	}

	router := NewRouter(config)
	connPool := NewConnectionPool(DefaultConnectionConfig())
	failover := NewFailoverHandler(&config.Failover, router)
	handler := NewProxyHandler(router, connPool, failover)

	t.Run("forward request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		if w.Header().Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
		}
	})
}

func TestProxyServer(t *testing.T) {
	config := &ProxyConfig{
		Enabled: true,
		Port: PortConfig{
			Value:       0,
			Range:       "19200-19300",
			BindAddress: "127.0.0.1",
		},
		Routing: RouteConfig{
			DefaultProvider: "test",
			LoadBalancing:   "priority",
			Providers:       []*ProviderConfig{},
			Failover: FailoverConfig{
				Enabled:          true,
				MaxRetries:       1,
				CircuitBreaker:   true,
				FailureThreshold: 5,
				RecoveryTimeout:  30 * time.Second,
			},
		},
		Connection: *DefaultConnectionConfig(),
		HealthCheck: HealthCheckConfig{
			Enabled:  false,
			Interval: 30 * time.Second,
			Timeout:  10 * time.Second,
		},
	}

	server, err := NewProxyServer(config)
	if err != nil {
		t.Fatalf("failed to create proxy server: %v", err)
	}

	t.Run("start and stop", func(t *testing.T) {
		err := server.Start()
		if err != nil {
			t.Fatalf("failed to start server: %v", err)
		}

		port := server.GetPort()
		if port < 19200 || port > 19300 {
			t.Errorf("port %d is outside expected range", port)
		}

		endpoint := server.GetEndpoint()
		if endpoint == "" {
			t.Error("GetEndpoint returned empty string")
		}

		// Test status endpoint
		resp, err := http.Get(endpoint + "/api/v1/proxy/status")
		if err != nil {
			t.Fatalf("failed to get status: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		// Stop server
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = server.Stop(ctx)
		if err != nil {
			t.Fatalf("failed to stop server: %v", err)
		}
	})
}
