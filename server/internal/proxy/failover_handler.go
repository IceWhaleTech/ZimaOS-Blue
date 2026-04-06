package proxy

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/labstack/echo/v4"
)

// FailoverAPIHandler provides HTTP handlers for failover management
type FailoverAPIHandler struct {
	smartFailover *SmartFailoverHandler
	config        *FailoverConfig
	onConfigSave  func(*FailoverConfig) error
	onRaceChange  func(ProviderRaceConfig)
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
	g.GET("/overview", h.GetOverview)
	g.GET("/metrics", h.GetMetrics)
	g.GET("/config", h.GetConfig)
	g.PUT("/config", h.UpdateConfig)
	g.POST("/reset", h.ResetCircuitBreakers)
	g.GET("/breakers", h.GetCircuitBreakerStatus)
}

// SetOnConfigSave sets a callback invoked after config updates.
func (h *FailoverAPIHandler) SetOnConfigSave(fn func(*FailoverConfig) error) {
	h.onConfigSave = fn
}

// SetOnProviderRaceChange sets a callback for provider-race runtime updates.
func (h *FailoverAPIHandler) SetOnProviderRaceChange(fn func(ProviderRaceConfig)) {
	h.onRaceChange = fn
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

// GetOverview returns aggregated failover state for sparse dashboard/status surfaces.
// GET /api/v1/proxy/failover/overview
func (h *FailoverAPIHandler) GetOverview(c echo.Context) error {
	return c.JSON(http.StatusOK, FailoverOverviewResponse{
		Metrics:         h.failoverMetricsPayload(),
		Config:          h.failoverConfigPayload(),
		CircuitBreakers: h.failoverCircuitBreakerPayload(),
	})
}

// GetConfig returns failover configuration
// GET /api/v1/proxy/failover/config
func (h *FailoverAPIHandler) GetConfig(c echo.Context) error {
	if h.config == nil {
		return c.JSON(http.StatusOK, DefaultProxyConfig().Routing.Failover)
	}
	return c.JSON(http.StatusOK, h.config)
}

type failoverConfigPatch struct {
	Enabled          *bool `json:"enabled"`
	MaxRetries       *int  `json:"max_retries"`
	CircuitBreaker   *bool `json:"circuit_breaker"`
	FailureThreshold *int  `json:"failure_threshold"`
	ContextWindow    *bool `json:"context_window_check"`

	RetryDelay              *time.Duration `json:"retry_delay"`
	RecoveryTimeout         *time.Duration `json:"recovery_timeout"`
	TransientRecoveryTimout *time.Duration `json:"transient_recovery_timeout"`
	QuotaCooldown           *time.Duration `json:"quota_cooldown"`

	ProviderRace *providerRaceConfigPatch `json:"provider_race"`
}

type providerRaceConfigPatch struct {
	Enabled                    *bool          `json:"enabled"`
	MaxParallel                *int           `json:"max_parallel"`
	MinProviders               *int           `json:"min_providers"`
	EmptyRateMinSamples        *int           `json:"empty_rate_min_samples"`
	EmptyRateCooldownThreshold *float64       `json:"empty_rate_cooldown_threshold"`
	EmptyRateSinkThreshold     *float64       `json:"empty_rate_sink_threshold"`
	EmptyRateExcludeThreshold  *float64       `json:"empty_rate_exclude_threshold"`
	EmptyRateCooldown          *time.Duration `json:"empty_rate_cooldown"`
}

// UpdateConfig updates failover configuration.
// PUT /api/v1/proxy/failover/config
func (h *FailoverAPIHandler) UpdateConfig(c echo.Context) error {
	if h.config == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failover config unavailable"})
	}

	var req failoverConfigPatch
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if req.Enabled != nil {
		h.config.Enabled = *req.Enabled
	}
	if req.MaxRetries != nil {
		h.config.MaxRetries = *req.MaxRetries
	}
	if req.CircuitBreaker != nil {
		h.config.CircuitBreaker = *req.CircuitBreaker
	}
	if req.FailureThreshold != nil {
		h.config.FailureThreshold = *req.FailureThreshold
	}
	if req.ContextWindow != nil {
		h.config.ContextWindowCheck = *req.ContextWindow
	}
	if req.RetryDelay != nil {
		h.config.RetryDelay = *req.RetryDelay
	}
	if req.RecoveryTimeout != nil {
		h.config.RecoveryTimeout = *req.RecoveryTimeout
	}
	if req.TransientRecoveryTimout != nil {
		h.config.TransientRecoveryTimeout = *req.TransientRecoveryTimout
	}
	if req.QuotaCooldown != nil {
		h.config.QuotaCooldown = *req.QuotaCooldown
	}
	if req.ProviderRace != nil {
		pr := &h.config.ProviderRace
		if req.ProviderRace.Enabled != nil {
			pr.Enabled = *req.ProviderRace.Enabled
		}
		if req.ProviderRace.MaxParallel != nil {
			pr.MaxParallel = *req.ProviderRace.MaxParallel
		}
		if req.ProviderRace.MinProviders != nil {
			pr.MinProviders = *req.ProviderRace.MinProviders
		}
		if req.ProviderRace.EmptyRateMinSamples != nil {
			pr.EmptyRateMinSamples = *req.ProviderRace.EmptyRateMinSamples
		}
		if req.ProviderRace.EmptyRateCooldownThreshold != nil {
			pr.EmptyRateCooldownThreshold = *req.ProviderRace.EmptyRateCooldownThreshold
		}
		if req.ProviderRace.EmptyRateSinkThreshold != nil {
			pr.EmptyRateSinkThreshold = *req.ProviderRace.EmptyRateSinkThreshold
		}
		if req.ProviderRace.EmptyRateExcludeThreshold != nil {
			pr.EmptyRateExcludeThreshold = *req.ProviderRace.EmptyRateExcludeThreshold
		}
		if req.ProviderRace.EmptyRateCooldown != nil {
			pr.EmptyRateCooldown = *req.ProviderRace.EmptyRateCooldown
		}
		if h.onRaceChange != nil {
			h.onRaceChange(h.config.ProviderRace)
		}
	}

	if h.onConfigSave != nil {
		if err := h.onConfigSave(h.config); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to persist failover config"})
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

type FailoverCircuitBreakerStatusResponse struct {
	State       string `json:"state"`
	Failures    int    `json:"failures"`
	LastFailure string `json:"last_failure,omitempty"`
}

type FailoverOverviewResponse struct {
	Metrics         FailoverMetricsResponse                         `json:"metrics"`
	Config          FailoverConfig                                  `json:"config"`
	CircuitBreakers map[string]FailoverCircuitBreakerStatusResponse `json:"circuit_breakers"`
}

func (h *FailoverAPIHandler) failoverMetricsPayload() FailoverMetricsResponse {
	if h == nil || h.smartFailover == nil || h.smartFailover.metrics == nil {
		return FailoverMetricsResponse{
			ErrorsByType:      map[string]int64{},
			ProviderErrors:    map[string]map[string]int64{},
			ProviderFailovers: map[string]int64{},
		}
	}

	metrics := h.smartFailover.metrics
	metrics.mu.RLock()
	defer metrics.mu.RUnlock()

	errorsByType := make(map[string]int64, len(metrics.ErrorsByType))
	for errorType, count := range metrics.ErrorsByType {
		errorsByType[string(errorType)] = count
	}

	providerErrors := make(map[string]map[string]int64, len(metrics.ProviderErrors))
	for provider, counts := range metrics.ProviderErrors {
		providerErrors[provider] = make(map[string]int64, len(counts))
		for errorType, count := range counts {
			providerErrors[provider][string(errorType)] = count
		}
	}

	providerFailovers := make(map[string]int64, len(metrics.ProviderFailovers))
	for provider, count := range metrics.ProviderFailovers {
		providerFailovers[provider] = count
	}

	return FailoverMetricsResponse{
		ErrorsByType:      errorsByType,
		FailoverTotal:     metrics.FailoverTotal,
		FailoverSuccess:   metrics.FailoverSuccess,
		FailoverFailure:   metrics.FailoverFailure,
		ProviderErrors:    providerErrors,
		ProviderFailovers: providerFailovers,
		StreamAnomalies:   atomic.LoadInt64(&metrics.StreamAnomalies),
	}
}

func (h *FailoverAPIHandler) failoverConfigPayload() FailoverConfig {
	if h == nil || h.config == nil {
		return DefaultProxyConfig().Routing.Failover
	}
	return *h.config
}

func (h *FailoverAPIHandler) failoverCircuitBreakerPayload() map[string]FailoverCircuitBreakerStatusResponse {
	if h == nil || h.smartFailover == nil {
		return map[string]FailoverCircuitBreakerStatusResponse{}
	}

	stats := h.smartFailover.GetBreakerStats()
	result := make(map[string]FailoverCircuitBreakerStatusResponse, len(stats))
	for name, stat := range stats {
		payload := FailoverCircuitBreakerStatusResponse{
			State: "unknown",
		}
		if statMap, ok := stat.(map[string]interface{}); ok {
			if state, ok := statMap["state"].(string); ok && state != "" {
				payload.State = state
			}
			switch failures := statMap["failures"].(type) {
			case int:
				payload.Failures = failures
			case int64:
				payload.Failures = int(failures)
			case float64:
				payload.Failures = int(failures)
			}
			switch lastFailure := statMap["last_failure"].(type) {
			case time.Time:
				if !lastFailure.IsZero() {
					payload.LastFailure = lastFailure.UTC().Format(time.RFC3339)
				}
			case string:
				payload.LastFailure = lastFailure
			}
		}
		result[name] = payload
	}
	return result
}

// ServeHTTP implements http.Handler for standalone use
func (h *FailoverAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.URL.Path {
	case "/overview":
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		json.NewEncoder(w).Encode(FailoverOverviewResponse{
			Metrics:         h.failoverMetricsPayload(),
			Config:          h.failoverConfigPayload(),
			CircuitBreakers: h.failoverCircuitBreakerPayload(),
		})

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
		} else if r.Method == http.MethodPut {
			var req failoverConfigPatch
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid request", http.StatusBadRequest)
				return
			}
			if h.config == nil {
				http.Error(w, "failover config unavailable", http.StatusInternalServerError)
				return
			}
			// Reuse update logic by binding through Echo path in regular server;
			// standalone handler supports only provider race enable toggle for tests/debug.
			if req.ProviderRace != nil && req.ProviderRace.Enabled != nil {
				h.config.ProviderRace.Enabled = *req.ProviderRace.Enabled
				if h.onRaceChange != nil {
					h.onRaceChange(h.config.ProviderRace)
				}
			}
			if h.onConfigSave != nil {
				if err := h.onConfigSave(h.config); err != nil {
					http.Error(w, "failed to persist failover config", http.StatusInternalServerError)
					return
				}
			}
			json.NewEncoder(w).Encode(h.config)
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
