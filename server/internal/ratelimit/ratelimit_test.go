package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Rate != 100 {
		t.Errorf("expected Rate=100, got %d", cfg.Rate)
	}
	if cfg.Window != time.Minute {
		t.Errorf("expected Window=1m, got %v", cfg.Window)
	}
	if cfg.CleanupInterval != 5*time.Minute {
		t.Errorf("expected CleanupInterval=5m, got %v", cfg.CleanupInterval)
	}
}

func TestLimiter_Allow(t *testing.T) {
	cfg := Config{
		Rate:            3,
		Window:          time.Second,
		CleanupInterval: time.Hour, // Long interval to avoid cleanup during test
	}
	limiter := New(cfg)
	defer limiter.Stop()

	key := "test-client"

	// First 3 requests should be allowed
	for i := 0; i < 3; i++ {
		if !limiter.Allow(key) {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 4th request should be denied
	if limiter.Allow(key) {
		t.Error("4th request should be denied")
	}
}

func TestLimiter_WindowReset(t *testing.T) {
	cfg := Config{
		Rate:            2,
		Window:          100 * time.Millisecond,
		CleanupInterval: time.Hour,
	}
	limiter := New(cfg)
	defer limiter.Stop()

	key := "test-client"

	// Use up the rate limit
	limiter.Allow(key)
	limiter.Allow(key)

	// Should be denied
	if limiter.Allow(key) {
		t.Error("should be denied after rate limit")
	}

	// Wait for window to reset
	time.Sleep(150 * time.Millisecond)

	// Should be allowed again
	if !limiter.Allow(key) {
		t.Error("should be allowed after window reset")
	}
}

func TestLimiter_Remaining(t *testing.T) {
	cfg := Config{
		Rate:            5,
		Window:          time.Second,
		CleanupInterval: time.Hour,
	}
	limiter := New(cfg)
	defer limiter.Stop()

	key := "test-client"

	// Initially should have full rate
	if remaining := limiter.Remaining(key); remaining != 5 {
		t.Errorf("expected remaining=5, got %d", remaining)
	}

	// After one request
	limiter.Allow(key)
	if remaining := limiter.Remaining(key); remaining != 4 {
		t.Errorf("expected remaining=4, got %d", remaining)
	}

	// After all requests
	limiter.Allow(key)
	limiter.Allow(key)
	limiter.Allow(key)
	limiter.Allow(key)
	if remaining := limiter.Remaining(key); remaining != 0 {
		t.Errorf("expected remaining=0, got %d", remaining)
	}
}

func TestLimiter_MultipleClients(t *testing.T) {
	cfg := Config{
		Rate:            2,
		Window:          time.Second,
		CleanupInterval: time.Hour,
	}
	limiter := New(cfg)
	defer limiter.Stop()

	// Each client should have their own limit
	client1 := "client-1"
	client2 := "client-2"

	// Client 1 uses up their limit
	limiter.Allow(client1)
	limiter.Allow(client1)
	if limiter.Allow(client1) {
		t.Error("client1 should be denied")
	}

	// Client 2 should still have their limit
	if !limiter.Allow(client2) {
		t.Error("client2 should be allowed")
	}
	if !limiter.Allow(client2) {
		t.Error("client2 should be allowed")
	}
	if limiter.Allow(client2) {
		t.Error("client2 should be denied")
	}
}

func TestMiddleware_AllowedRequest(t *testing.T) {
	e := echo.New()
	cfg := Config{
		Rate:            10,
		Window:          time.Minute,
		CleanupInterval: time.Hour,
	}
	limiter := New(cfg)
	defer limiter.Stop()

	e.Use(limiter.Middleware())
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Real-IP", "192.168.1.1")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	// Check rate limit headers
	if rec.Header().Get("X-RateLimit-Limit") != "10" {
		t.Errorf("expected X-RateLimit-Limit=10, got %s", rec.Header().Get("X-RateLimit-Limit"))
	}
	if rec.Header().Get("X-RateLimit-Remaining") != "9" {
		t.Errorf("expected X-RateLimit-Remaining=9, got %s", rec.Header().Get("X-RateLimit-Remaining"))
	}
}

func TestMiddleware_RateLimitExceeded(t *testing.T) {
	e := echo.New()
	cfg := Config{
		Rate:            2,
		Window:          time.Minute,
		CleanupInterval: time.Hour,
	}
	limiter := New(cfg)
	defer limiter.Stop()

	e.Use(limiter.Middleware())
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	// Make requests until rate limit is exceeded
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Real-IP", "192.168.1.1")
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		if i < 2 {
			if rec.Code != http.StatusOK {
				t.Errorf("request %d: expected status 200, got %d", i+1, rec.Code)
			}
		} else {
			if rec.Code != http.StatusTooManyRequests {
				t.Errorf("request %d: expected status 429, got %d", i+1, rec.Code)
			}
			if rec.Header().Get("X-RateLimit-Remaining") != "0" {
				t.Errorf("expected X-RateLimit-Remaining=0, got %s", rec.Header().Get("X-RateLimit-Remaining"))
			}
		}
	}
}

func TestMiddleware_CustomKeyFunc(t *testing.T) {
	e := echo.New()
	cfg := Config{
		Rate:            2,
		Window:          time.Minute,
		CleanupInterval: time.Hour,
		KeyFunc: func(c echo.Context) string {
			return c.Request().Header.Get("X-API-Key")
		},
	}
	limiter := New(cfg)
	defer limiter.Stop()

	e.Use(limiter.Middleware())
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	// Different API keys should have separate limits
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-API-Key", "key-1")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("key-1 request %d: expected 200, got %d", i+1, rec.Code)
		}
	}

	// key-1 should be rate limited now
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "key-1")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("key-1 should be rate limited, got %d", rec.Code)
	}

	// key-2 should still work
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "key-2")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("key-2 should be allowed, got %d", rec.Code)
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{10, "10"},
		{100, "100"},
		{-1, "-1"},
		{-100, "-100"},
		{12345, "12345"},
	}

	for _, tt := range tests {
		result := itoa(tt.input)
		if result != tt.expected {
			t.Errorf("itoa(%d) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}
