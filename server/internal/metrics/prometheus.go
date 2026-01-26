package metrics

import (
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Config holds metrics configuration
type Config struct {
	Enabled        bool   `mapstructure:"enabled"`
	Endpoint       string `mapstructure:"endpoint"`
	IncludeRuntime bool   `mapstructure:"include_runtime"`
	IncludeHTTP    bool   `mapstructure:"include_http"`
	IncludeLLM     bool   `mapstructure:"include_llm"`
}

// Metrics holds all prometheus metrics
type Metrics struct {
	config   *Config
	registry *prometheus.Registry

	// HTTP metrics
	httpRequestsTotal    *prometheus.CounterVec
	httpRequestDuration  *prometheus.HistogramVec
	httpRequestsInFlight prometheus.Gauge
	httpResponseSize     *prometheus.HistogramVec

	// LLM metrics
	llmRequestsTotal   *prometheus.CounterVec
	llmRequestDuration *prometheus.HistogramVec
	llmTokensTotal     *prometheus.CounterVec
	llmErrors          *prometheus.CounterVec

	// Worker pool metrics
	workerPoolActive    prometheus.Gauge
	workerPoolQueued    prometheus.Gauge
	workerPoolCompleted prometheus.Counter
	workerPoolErrors    prometheus.Counter

	// Custom business metrics
	conversationsTotal prometheus.Counter
	messagesTotal      *prometheus.CounterVec
}

// New creates a new Metrics instance
func New(cfg *Config) *Metrics {
	if cfg == nil {
		cfg = &Config{
			Enabled:        true,
			Endpoint:       "/metrics",
			IncludeRuntime: true,
			IncludeHTTP:    true,
			IncludeLLM:     true,
		}
	}

	registry := prometheus.NewRegistry()

	m := &Metrics{
		config:   cfg,
		registry: registry,
	}

	// Register runtime collectors if enabled
	if cfg.IncludeRuntime {
		registry.MustRegister(collectors.NewGoCollector())
		registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	}

	// Initialize HTTP metrics
	if cfg.IncludeHTTP {
		m.initHTTPMetrics()
	}

	// Initialize LLM metrics
	if cfg.IncludeLLM {
		m.initLLMMetrics()
	}

	// Initialize worker pool metrics
	m.initWorkerMetrics()

	// Initialize business metrics
	m.initBusinessMetrics()

	return m
}

func (m *Metrics) initHTTPMetrics() {
	m.httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	m.httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)

	m.httpRequestsInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Number of HTTP requests currently being processed",
		},
	)

	m.httpResponseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "HTTP response size in bytes",
			Buckets: []float64{100, 1000, 10000, 100000, 1000000},
		},
		[]string{"method", "path"},
	)

	m.registry.MustRegister(
		m.httpRequestsTotal,
		m.httpRequestDuration,
		m.httpRequestsInFlight,
		m.httpResponseSize,
	)
}

func (m *Metrics) initLLMMetrics() {
	m.llmRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llm_requests_total",
			Help: "Total number of LLM requests",
		},
		[]string{"provider", "model"},
	)

	m.llmRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "llm_request_duration_seconds",
			Help:    "LLM request duration in seconds",
			Buckets: []float64{.1, .5, 1, 2.5, 5, 10, 30, 60, 120},
		},
		[]string{"provider", "model"},
	)

	m.llmTokensTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llm_tokens_total",
			Help: "Total number of tokens used",
		},
		[]string{"provider", "model", "type"}, // type: input, output
	)

	m.llmErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llm_errors_total",
			Help: "Total number of LLM errors",
		},
		[]string{"provider", "model", "error_type"},
	)

	m.registry.MustRegister(
		m.llmRequestsTotal,
		m.llmRequestDuration,
		m.llmTokensTotal,
		m.llmErrors,
	)
}

func (m *Metrics) initWorkerMetrics() {
	m.workerPoolActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "worker_pool_active",
			Help: "Number of active workers",
		},
	)

	m.workerPoolQueued = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "worker_pool_queued",
			Help: "Number of queued tasks",
		},
	)

	m.workerPoolCompleted = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "worker_pool_completed_total",
			Help: "Total number of completed tasks",
		},
	)

	m.workerPoolErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "worker_pool_errors_total",
			Help: "Total number of worker errors",
		},
	)

	m.registry.MustRegister(
		m.workerPoolActive,
		m.workerPoolQueued,
		m.workerPoolCompleted,
		m.workerPoolErrors,
	)
}

func (m *Metrics) initBusinessMetrics() {
	m.conversationsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "conversations_total",
			Help: "Total number of conversations created",
		},
	)

	m.messagesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "messages_total",
			Help: "Total number of messages",
		},
		[]string{"role"}, // user, assistant, system
	)

	m.registry.MustRegister(
		m.conversationsTotal,
		m.messagesTotal,
	)
}

// Handler returns the prometheus HTTP handler
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// RegisterRoutes registers the metrics endpoint
func (m *Metrics) RegisterRoutes(e *echo.Echo) {
	if !m.config.Enabled {
		return
	}
	e.GET(m.config.Endpoint, echo.WrapHandler(m.Handler()))
}

// Middleware returns an Echo middleware for HTTP metrics
func (m *Metrics) Middleware() echo.MiddlewareFunc {
	if !m.config.IncludeHTTP {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return next
		}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Path() == m.config.Endpoint {
				return next(c)
			}

			start := time.Now()
			m.httpRequestsInFlight.Inc()

			err := next(c)

			m.httpRequestsInFlight.Dec()
			duration := time.Since(start).Seconds()

			status := c.Response().Status
			method := c.Request().Method
			path := c.Path()

			m.httpRequestsTotal.WithLabelValues(method, path, strconv.Itoa(status)).Inc()
			m.httpRequestDuration.WithLabelValues(method, path).Observe(duration)
			m.httpResponseSize.WithLabelValues(method, path).Observe(float64(c.Response().Size))

			return err
		}
	}
}

// RecordLLMRequest records an LLM request
func (m *Metrics) RecordLLMRequest(provider, model string, duration time.Duration, inputTokens, outputTokens int) {
	if !m.config.IncludeLLM {
		return
	}
	m.llmRequestsTotal.WithLabelValues(provider, model).Inc()
	m.llmRequestDuration.WithLabelValues(provider, model).Observe(duration.Seconds())
	m.llmTokensTotal.WithLabelValues(provider, model, "input").Add(float64(inputTokens))
	m.llmTokensTotal.WithLabelValues(provider, model, "output").Add(float64(outputTokens))
}

// RecordLLMError records an LLM error
func (m *Metrics) RecordLLMError(provider, model, errorType string) {
	if !m.config.IncludeLLM {
		return
	}
	m.llmErrors.WithLabelValues(provider, model, errorType).Inc()
}

// UpdateWorkerStats updates worker pool metrics
func (m *Metrics) UpdateWorkerStats(active, queued int) {
	m.workerPoolActive.Set(float64(active))
	m.workerPoolQueued.Set(float64(queued))
}

// RecordWorkerCompleted records a completed worker task
func (m *Metrics) RecordWorkerCompleted() {
	m.workerPoolCompleted.Inc()
}

// RecordWorkerError records a worker error
func (m *Metrics) RecordWorkerError() {
	m.workerPoolErrors.Inc()
}

// RecordConversation records a new conversation
func (m *Metrics) RecordConversation() {
	m.conversationsTotal.Inc()
}

// RecordMessage records a message
func (m *Metrics) RecordMessage(role string) {
	m.messagesTotal.WithLabelValues(role).Inc()
}

// GetRuntimeStats returns current runtime statistics
func GetRuntimeStats() map[string]interface{} {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return map[string]interface{}{
		"goroutines":     runtime.NumGoroutine(),
		"num_cpu":        runtime.NumCPU(),
		"gomaxprocs":     runtime.GOMAXPROCS(0),
		"alloc_bytes":    memStats.Alloc,
		"sys_bytes":      memStats.Sys,
		"heap_alloc":     memStats.HeapAlloc,
		"heap_sys":       memStats.HeapSys,
		"heap_idle":      memStats.HeapIdle,
		"heap_inuse":     memStats.HeapInuse,
		"heap_released":  memStats.HeapReleased,
		"heap_objects":   memStats.HeapObjects,
		"gc_cycles":      memStats.NumGC,
		"gc_pause_total": memStats.PauseTotalNs,
	}
}
