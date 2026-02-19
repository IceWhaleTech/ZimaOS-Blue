package proxy

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// FailoverAPIHandler provides HTTP handlers for failover management
type FailoverAPIHandler struct {
	smartFailover *SmartFailoverHandler
	config        *FailoverConfig
}

// NewFailoverAPIHandler creates a new failover API handler
func NewFailoverAPIHandler(smartFailover *SmartFailoverHandler, config *FailoverConfig) *FailoverAPIHandler {
	return &FailoverAPIHandler{
		smartFailover: smartFailover,
		config:        config,
	}
}

// RegisterRoutes registers failover API routes
func (h *FailoverAPIHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/metrics", h.GetMetrics)
	g.GET("/config", h.GetConfig)
	g.PUT("/config", h.UpdateConfig)
	g.POST("/reset", h.ResetCircuitBreakers)
	g.GET("/breakers", h.GetCircuitBreakerStatus)
}

// GetMetrics returns failover metrics
// GET /api/v1/proxy/failover/metrics
func (h *FailoverAPIHandler) GetMetrics(c echo.Context) error {
	if h.smartFailover == nil {
		// Return empty metrics if smart failover is not initialized
		return c.JSON(http.StatusOK, &FailoverMetrics{
			ErrorsByType:      make(map[RetryableErrorType]int64),
			ProviderErrors:    make(map[string]map[RetryableErrorType]int64),
			ProviderFailovers: make(map[string]int64),
		})
	}

	metrics := h.smartFailover.GetMetrics()
	return c.JSON(http.StatusOK, metrics.GetStats())
}

// GetConfig returns failover configuration
// GET /api/v1/proxy/failover/config
func (h *FailoverAPIHandler) GetConfig(c echo.Context) error {
	if h.config == nil {
		return c.JSON(http.StatusOK, DefaultProxyConfig().Routing.Failover)
	}
	return c.JSON(http.StatusOK, h.config)
}

// UpdateConfig updates failover configuration (partial merge).
// PUT /api/v1/proxy/failover/config
func (h *FailoverAPIHandler) UpdateConfig(c echo.Context) error {
	// Read raw JSON so we know which fields were actually sent.
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(c.Request().Body).Decode(&raw); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
	}

	if h.config == nil {
		return c.JSON(http.StatusOK, h.config)
	}

	// Helper: unmarshal a top-level field only if present in the request.
	has := func(key string) bool { _, ok := raw[key]; return ok }

	if has("enabled") {
		var v bool
		json.Unmarshal(raw["enabled"], &v)
		h.config.Enabled = v
	}
	if has("max_retries") {
		var v int
		json.Unmarshal(raw["max_retries"], &v)
		if v > 0 {
			h.config.MaxRetries = v
		}
	}
	if has("retry_delay") {
		var v float64
		json.Unmarshal(raw["retry_delay"], &v)
		if v > 0 {
			h.config.RetryDelay = time.Duration(v)
		}
	}
	if has("circuit_breaker") {
		var v bool
		json.Unmarshal(raw["circuit_breaker"], &v)
		h.config.CircuitBreaker = v
	}
	if has("failure_threshold") {
		var v int
		json.Unmarshal(raw["failure_threshold"], &v)
		if v > 0 {
			h.config.FailureThreshold = v
		}
	}
	if has("recovery_timeout") {
		var v float64
		json.Unmarshal(raw["recovery_timeout"], &v)
		if v > 0 {
			h.config.RecoveryTimeout = time.Duration(v)
		}
	}

	// Error classification — partial merge into nested struct.
	if has("error_classification") {
		var ecRaw map[string]json.RawMessage
		json.Unmarshal(raw["error_classification"], &ecRaw)
		if _, ok := ecRaw["enabled"]; ok {
			var v bool
			json.Unmarshal(ecRaw["enabled"], &v)
			h.config.ErrorClassification.Enabled = v
		}
		if _, ok := ecRaw["failover_errors"]; ok {
			var v []string
			json.Unmarshal(ecRaw["failover_errors"], &v)
			if len(v) > 0 {
				h.config.ErrorClassification.FailoverErrors = v
			}
		}
		if _, ok := ecRaw["retryable_errors"]; ok {
			var v []string
			json.Unmarshal(ecRaw["retryable_errors"], &v)
			if len(v) > 0 {
				h.config.ErrorClassification.RetryableErrors = v
			}
		}
	}

	// Streaming anomaly — partial merge into nested struct.
	if has("streaming_anomaly") {
		var saRaw map[string]json.RawMessage
		json.Unmarshal(raw["streaming_anomaly"], &saRaw)
		if _, ok := saRaw["enabled"]; ok {
			var v bool
			json.Unmarshal(saRaw["enabled"], &v)
			h.config.StreamingAnomaly.Enabled = v
		}
		if _, ok := saRaw["window_size"]; ok {
			var v int
			json.Unmarshal(saRaw["window_size"], &v)
			if v > 0 {
				h.config.StreamingAnomaly.WindowSize = v
			}
		}
		if _, ok := saRaw["repeat_threshold"]; ok {
			var v int
			json.Unmarshal(saRaw["repeat_threshold"], &v)
			if v > 0 {
				h.config.StreamingAnomaly.RepeatThreshold = v
			}
		}
		if _, ok := saRaw["min_pattern_length"]; ok {
			var v int
			json.Unmarshal(saRaw["min_pattern_length"], &v)
			if v > 0 {
				h.config.StreamingAnomaly.MinPatternLength = v
			}
		}
		if _, ok := saRaw["recovery_strategy"]; ok {
			var v string
			json.Unmarshal(saRaw["recovery_strategy"], &v)
			if v != "" {
				h.config.StreamingAnomaly.RecoveryStrategy = v
			}
		}
	}

	// Context window check.
	if has("context_window_check") {
		var v bool
		json.Unmarshal(raw["context_window_check"], &v)
		h.config.ContextWindowCheck = v
	}
	if has("quota_cooldown") {
		var v float64
		json.Unmarshal(raw["quota_cooldown"], &v)
		if v > 0 {
			h.config.QuotaCooldown = time.Duration(v)
		}
	}
	if has("context_window_override") {
		var v map[string]int
		json.Unmarshal(raw["context_window_override"], &v)
		if len(v) > 0 {
			h.config.ContextWindowOverride = v
		}
	}

	return c.JSON(http.StatusOK, h.config)
}

// ResetCircuitBreakers resets all circuit breakers
// POST /api/v1/proxy/failover/reset
func (h *FailoverAPIHandler) ResetCircuitBreakers(c echo.Context) error {
	if h.smartFailover != nil {
		h.smartFailover.ResetAllBreakers()
	}
	return c.JSON(http.StatusOK, map[string]string{
		"message": "All circuit breakers have been reset",
	})
}

// GetCircuitBreakerStatus returns status of all circuit breakers
// GET /api/v1/proxy/failover/breakers
func (h *FailoverAPIHandler) GetCircuitBreakerStatus(c echo.Context) error {
	if h.smartFailover == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{})
	}

	stats := h.smartFailover.GetBreakerStats()
	result := make(map[string]interface{})

	for name, stat := range stats {
		if statMap, ok := stat.(map[string]interface{}); ok {
			result[name] = map[string]interface{}{
				"state":        statMap["state"],
				"failures":     statMap["failures"],
				"last_failure": statMap["last_failure"],
			}
		} else {
			result[name] = map[string]interface{}{
				"state":    "unknown",
				"failures": 0,
			}
		}
	}

	return c.JSON(http.StatusOK, result)
}

// FailoverMetricsResponse is the response for metrics endpoint
type FailoverMetricsResponse struct {
	ErrorsByType      map[string]int64            `json:"errors_by_type"`
	FailoverTotal     int64                       `json:"failover_total"`
	FailoverSuccess   int64                       `json:"failover_success"`
	FailoverFailure   int64                       `json:"failover_failure"`
	ProviderErrors    map[string]map[string]int64 `json:"provider_errors"`
	ProviderFailovers map[string]int64            `json:"provider_failovers"`
	StreamAnomalies   int64                       `json:"stream_anomalies"`
}

// ServeHTTP implements http.Handler for standalone use
func (h *FailoverAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.URL.Path {
	case "/metrics":
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if h.smartFailover == nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"errors_by_type":     map[string]int64{},
				"failover_total":     0,
				"failover_success":   0,
				"failover_failure":   0,
				"provider_errors":    map[string]map[string]int64{},
				"provider_failovers": map[string]int64{},
				"stream_anomalies":   0,
			})
			return
		}
		json.NewEncoder(w).Encode(h.smartFailover.GetMetrics().GetStats())

	case "/config":
		if r.Method == http.MethodGet {
			if h.config == nil {
				json.NewEncoder(w).Encode(DefaultProxyConfig().Routing.Failover)
			} else {
				json.NewEncoder(w).Encode(h.config)
			}
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}

	case "/breakers":
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if h.smartFailover == nil {
			json.NewEncoder(w).Encode(map[string]interface{}{})
			return
		}
		json.NewEncoder(w).Encode(h.smartFailover.GetBreakerStats())

	case "/reset":
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if h.smartFailover != nil {
			h.smartFailover.ResetAllBreakers()
		}
		json.NewEncoder(w).Encode(map[string]string{
			"message": "All circuit breakers have been reset",
		})

	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}
