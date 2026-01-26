package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
	}{
		{
			name:   "nil config uses defaults",
			config: nil,
		},
		{
			name: "custom config",
			config: &Config{
				Enabled:        true,
				Endpoint:       "/custom-metrics",
				IncludeRuntime: false,
				IncludeHTTP:    true,
				IncludeLLM:     true,
			},
		},
		{
			name: "disabled metrics",
			config: &Config{
				Enabled:        false,
				IncludeRuntime: false,
				IncludeHTTP:    false,
				IncludeLLM:     false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(tt.config)
			if m == nil {
				t.Fatal("expected non-nil Metrics")
			}
			if m.registry == nil {
				t.Fatal("expected non-nil registry")
			}
		})
	}
}

func TestMetricsHandler(t *testing.T) {
	m := New(&Config{
		Enabled:        true,
		Endpoint:       "/metrics",
		IncludeRuntime: true,
		IncludeHTTP:    true,
		IncludeLLM:     true,
	})

	handler := m.Handler()
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "go_goroutines") {
		t.Error("expected go_goroutines metric in response")
	}
}

func TestMetricsMiddleware(t *testing.T) {
	m := New(&Config{
		Enabled:        true,
		Endpoint:       "/metrics",
		IncludeRuntime: false,
		IncludeHTTP:    true,
		IncludeLLM:     false,
	})

	e := echo.New()
	e.Use(m.Middleware())
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	// Check metrics were recorded
	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRec := httptest.NewRecorder()
	m.Handler().ServeHTTP(metricsRec, metricsReq)

	body := metricsRec.Body.String()
	if !strings.Contains(body, "http_requests_total") {
		t.Error("expected http_requests_total metric")
	}
}

func TestRecordLLMRequest(t *testing.T) {
	m := New(&Config{
		Enabled:        true,
		Endpoint:       "/metrics",
		IncludeRuntime: false,
		IncludeHTTP:    false,
		IncludeLLM:     true,
	})

	m.RecordLLMRequest("openai", "gpt-4", 2*time.Second, 100, 50)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "llm_requests_total") {
		t.Error("expected llm_requests_total metric")
	}
	if !strings.Contains(body, "llm_tokens_total") {
		t.Error("expected llm_tokens_total metric")
	}
}

func TestRecordLLMError(t *testing.T) {
	m := New(&Config{
		Enabled:        true,
		Endpoint:       "/metrics",
		IncludeRuntime: false,
		IncludeHTTP:    false,
		IncludeLLM:     true,
	})

	m.RecordLLMError("openai", "gpt-4", "rate_limit")

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "llm_errors_total") {
		t.Error("expected llm_errors_total metric")
	}
}

func TestWorkerMetrics(t *testing.T) {
	m := New(&Config{
		Enabled:        true,
		Endpoint:       "/metrics",
		IncludeRuntime: false,
		IncludeHTTP:    false,
		IncludeLLM:     false,
	})

	m.UpdateWorkerStats(5, 10)
	m.RecordWorkerCompleted()
	m.RecordWorkerError()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "worker_pool_active") {
		t.Error("expected worker_pool_active metric")
	}
	if !strings.Contains(body, "worker_pool_queued") {
		t.Error("expected worker_pool_queued metric")
	}
	if !strings.Contains(body, "worker_pool_completed_total") {
		t.Error("expected worker_pool_completed_total metric")
	}
}

func TestBusinessMetrics(t *testing.T) {
	m := New(&Config{
		Enabled:        true,
		Endpoint:       "/metrics",
		IncludeRuntime: false,
		IncludeHTTP:    false,
		IncludeLLM:     false,
	})

	m.RecordConversation()
	m.RecordMessage("user")
	m.RecordMessage("assistant")

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "conversations_total") {
		t.Error("expected conversations_total metric")
	}
	if !strings.Contains(body, "messages_total") {
		t.Error("expected messages_total metric")
	}
}

func TestGetRuntimeStats(t *testing.T) {
	stats := GetRuntimeStats()

	requiredKeys := []string{
		"goroutines",
		"num_cpu",
		"gomaxprocs",
		"alloc_bytes",
		"sys_bytes",
		"heap_alloc",
		"gc_cycles",
	}

	for _, key := range requiredKeys {
		if _, ok := stats[key]; !ok {
			t.Errorf("expected key %s in runtime stats", key)
		}
	}
}

func TestDisabledMetrics(t *testing.T) {
	m := New(&Config{
		Enabled:        false,
		IncludeRuntime: false,
		IncludeHTTP:    false,
		IncludeLLM:     false,
	})

	// These should not panic when metrics are disabled
	m.RecordLLMRequest("openai", "gpt-4", time.Second, 100, 50)
	m.RecordLLMError("openai", "gpt-4", "error")
	m.UpdateWorkerStats(1, 2)
	m.RecordWorkerCompleted()
	m.RecordWorkerError()
	m.RecordConversation()
	m.RecordMessage("user")
}

func TestRegisterRoutes(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		endpoint string
	}{
		{
			name:     "enabled metrics",
			enabled:  true,
			endpoint: "/metrics",
		},
		{
			name:     "disabled metrics",
			enabled:  false,
			endpoint: "/metrics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(&Config{
				Enabled:  tt.enabled,
				Endpoint: tt.endpoint,
			})

			e := echo.New()
			m.RegisterRoutes(e)

			if tt.enabled {
				req := httptest.NewRequest(http.MethodGet, tt.endpoint, nil)
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)

				if rec.Code != http.StatusOK {
					t.Errorf("expected status 200, got %d", rec.Code)
				}
			}
		})
	}
}
