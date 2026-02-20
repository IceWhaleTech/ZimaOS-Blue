package proxy

import (
	"net/http"

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
