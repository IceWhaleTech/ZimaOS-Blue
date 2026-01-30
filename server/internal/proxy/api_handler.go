package proxy

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ProxyAPIHandler provides HTTP handlers for proxy management APIs
type ProxyAPIHandler struct {
	sessionMonitor   *SessionMonitor
	metricsCollector *MetricsCollector
	promptGuard      *PromptGuard
	configWatcher    *ConfigWatcher
	modelCompat      *ModelCompatLayer
	mockHandler      *MockHandler
	authenticator    *Authenticator
	dataMasker       *DataMasker
}

// NewProxyAPIHandler creates a new proxy API handler
func NewProxyAPIHandler(
	sessionMonitor *SessionMonitor,
	metricsCollector *MetricsCollector,
	promptGuard *PromptGuard,
	configWatcher *ConfigWatcher,
	modelCompat *ModelCompatLayer,
	mockHandler *MockHandler,
	authenticator *Authenticator,
) *ProxyAPIHandler {
	return &ProxyAPIHandler{
		sessionMonitor:   sessionMonitor,
		metricsCollector: metricsCollector,
		promptGuard:      promptGuard,
		configWatcher:    configWatcher,
		modelCompat:      modelCompat,
		mockHandler:      mockHandler,
		authenticator:    authenticator,
		dataMasker:       NewDataMasker(nil), // Initialize with default config
	}
}

// RegisterRoutes registers all proxy API routes
func (h *ProxyAPIHandler) RegisterRoutes(mux *http.ServeMux) {
	// Session endpoints
	mux.HandleFunc("/api/v1/proxy/sessions", h.handleSessions)
	mux.HandleFunc("/api/v1/proxy/sessions/", h.handleSessionByID)

	// Metrics endpoints
	mux.HandleFunc("/api/v1/proxy/metrics", h.handleMetrics)
	mux.HandleFunc("/api/v1/proxy/metrics/providers", h.handleMetricsProviders)
	mux.HandleFunc("/api/v1/proxy/metrics/latency", h.handleMetricsLatency)
	mux.HandleFunc("/api/v1/proxy/metrics/timeseries", h.handleMetricsTimeSeries)

	// Guard endpoints
	mux.HandleFunc("/api/v1/proxy/guard/stats", h.handleGuardStats)
	mux.HandleFunc("/api/v1/proxy/guard/rules", h.handleGuardRules)

	// Auth endpoints
	mux.HandleFunc("/api/v1/proxy/auth/keys", h.handleAuthKeys)
	mux.HandleFunc("/api/v1/proxy/auth/stats", h.handleAuthStats)

	// Model compatibility endpoints
	mux.HandleFunc("/api/v1/proxy/models/compat", h.handleModels)

	// Mock endpoints
	mux.HandleFunc("/api/v1/proxy/mock", h.handleMock)

	// Data masking endpoints (reserved for future implementation)
	mux.HandleFunc("/api/v1/proxy/masking/stats", h.handleMaskingStats)
	mux.HandleFunc("/api/v1/proxy/masking/rules", h.handleMaskingRules)

	// Config reload endpoint
	mux.HandleFunc("/api/v1/proxy/config/reload", h.handleConfigReload)
}

// handleSessions handles GET /api/v1/proxy/sessions
func (h *ProxyAPIHandler) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.sessionMonitor == nil {
		h.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"sessions": []interface{}{},
			"stats":    map[string]interface{}{},
		})
		return
	}

	// Check for active-only filter
	activeOnly := r.URL.Query().Get("active") == "true"

	var sessions []*Session
	if activeOnly {
		sessions = h.sessionMonitor.ListActiveSessions()
	} else {
		sessions = h.sessionMonitor.ListSessions()
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"sessions": sessions,
		"stats":    h.sessionMonitor.Stats(),
	})
}

// handleSessionByID handles GET /api/v1/proxy/sessions/:id
func (h *ProxyAPIHandler) handleSessionByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract session ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/proxy/sessions/")
	sessionID := strings.TrimSuffix(path, "/")

	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	if h.sessionMonitor == nil {
		http.Error(w, "Session monitoring not enabled", http.StatusNotFound)
		return
	}

	session, found := h.sessionMonitor.GetSession(sessionID)
	if !found {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	h.jsonResponse(w, http.StatusOK, session)
}

// handleMetrics handles GET /api/v1/proxy/metrics
func (h *ProxyAPIHandler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.metricsCollector == nil {
		h.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"enabled": false,
		})
		return
	}

	h.jsonResponse(w, http.StatusOK, h.metricsCollector.Summary())
}

// handleMetricsProviders handles GET /api/v1/proxy/metrics/providers
func (h *ProxyAPIHandler) handleMetricsProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.metricsCollector == nil {
		h.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"providers": map[string]interface{}{},
		})
		return
	}

	// Check for specific provider
	providerName := r.URL.Query().Get("provider")
	if providerName != "" {
		pm, found := h.metricsCollector.GetProviderMetrics(providerName)
		if !found {
			http.Error(w, "Provider not found", http.StatusNotFound)
			return
		}
		h.jsonResponse(w, http.StatusOK, pm)
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"providers": h.metricsCollector.GetAllProviderMetrics(),
	})
}

// handleMetricsLatency handles GET /api/v1/proxy/metrics/latency
func (h *ProxyAPIHandler) handleMetricsLatency(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.metricsCollector == nil {
		h.jsonResponse(w, http.StatusOK, map[string]interface{}{})
		return
	}

	h.jsonResponse(w, http.StatusOK, h.metricsCollector.LatencyStats())
}

// handleMetricsTimeSeries handles GET /api/v1/proxy/metrics/timeseries
func (h *ProxyAPIHandler) handleMetricsTimeSeries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.metricsCollector == nil {
		h.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"buckets": []interface{}{},
		})
		return
	}

	// Parse time range from query params
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	var start, end time.Time
	var err error

	if startStr != "" {
		start, err = time.Parse(time.RFC3339, startStr)
		if err != nil {
			http.Error(w, "Invalid start time format", http.StatusBadRequest)
			return
		}
	} else {
		start = time.Now().Add(-24 * time.Hour)
	}

	if endStr != "" {
		end, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			http.Error(w, "Invalid end time format", http.StatusBadRequest)
			return
		}
	} else {
		end = time.Now()
	}

	buckets := h.metricsCollector.GetTimeSeries(start, end)
	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"buckets": buckets,
		"start":   start.Format(time.RFC3339),
		"end":     end.Format(time.RFC3339),
	})
}

// handleGuardStats handles GET /api/v1/proxy/guard/stats
func (h *ProxyAPIHandler) handleGuardStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.promptGuard == nil {
		h.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"enabled": false,
		})
		return
	}

	h.jsonResponse(w, http.StatusOK, h.promptGuard.Stats())
}

// handleGuardRules handles GET/POST /api/v1/proxy/guard/rules
func (h *ProxyAPIHandler) handleGuardRules(w http.ResponseWriter, r *http.Request) {
	if h.promptGuard == nil {
		http.Error(w, "Prompt guard not enabled", http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"rules": h.promptGuard.GetRules(),
		})

	case http.MethodPost:
		var rule GuardRule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if err := h.promptGuard.AddRule(rule); err != nil {
			h.jsonResponse(w, http.StatusBadRequest, map[string]interface{}{
				"error": err.Error(),
			})
			return
		}
		h.jsonResponse(w, http.StatusCreated, map[string]interface{}{
			"message": "Rule added successfully",
			"rule":    rule,
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleModels handles GET /api/v1/proxy/models
func (h *ProxyAPIHandler) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.modelCompat == nil {
		h.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"models": map[string]interface{}{},
		})
		return
	}

	// Get specific model if requested
	modelName := r.URL.Query().Get("model")
	if modelName != "" {
		features := h.modelCompat.GetFeatures(modelName)
		h.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"model":    modelName,
			"features": features,
		})
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"models": h.modelCompat.GetAllFeatures(),
	})
}

// handleMock handles GET/POST /api/v1/proxy/mock
func (h *ProxyAPIHandler) handleMock(w http.ResponseWriter, r *http.Request) {
	if h.mockHandler == nil {
		http.Error(w, "Mock handler not enabled", http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"endpoints": h.mockHandler.ListEndpoints(),
			"enabled":   h.mockHandler.IsEnabled(),
		})

	case http.MethodPost:
		var endpoint MockEndpoint
		if err := json.NewDecoder(r.Body).Decode(&endpoint); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		h.mockHandler.AddEndpoint(&endpoint)
		h.jsonResponse(w, http.StatusCreated, map[string]interface{}{
			"message":  "Mock endpoint added successfully",
			"endpoint": endpoint,
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleConfigReload handles POST /api/v1/proxy/config/reload
func (h *ProxyAPIHandler) handleConfigReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.configWatcher == nil {
		http.Error(w, "Config watcher not enabled", http.StatusNotFound)
		return
	}

	// Force reload
	if err := h.configWatcher.ForceReload(); err != nil {
		h.jsonResponse(w, http.StatusInternalServerError, map[string]interface{}{
			"error":   "Failed to reload configuration",
			"details": err.Error(),
		})
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"message":     "Configuration reloaded successfully",
		"reloaded_at": time.Now().Format(time.RFC3339),
	})
}

// handleAuthKeys handles GET/POST/DELETE /api/v1/proxy/auth/keys
func (h *ProxyAPIHandler) handleAuthKeys(w http.ResponseWriter, r *http.Request) {
	if h.authenticator == nil {
		http.Error(w, "Authentication not enabled", http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"keys": h.authenticator.ListAPIKeys(),
		})

	case http.MethodPost:
		var req struct {
			Key string `json:"key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Key == "" {
			http.Error(w, "API key is required", http.StatusBadRequest)
			return
		}

		h.authenticator.AddAPIKey(req.Key)
		h.jsonResponse(w, http.StatusCreated, map[string]interface{}{
			"message": "API key added successfully",
		})

	case http.MethodDelete:
		key := r.URL.Query().Get("key")
		if key == "" {
			http.Error(w, "API key is required", http.StatusBadRequest)
			return
		}

		h.authenticator.RemoveAPIKey(key)
		h.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"message": "API key removed successfully",
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAuthStats handles GET /api/v1/proxy/auth/stats
func (h *ProxyAPIHandler) handleAuthStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.authenticator == nil {
		h.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"enabled": false,
		})
		return
	}

	h.jsonResponse(w, http.StatusOK, h.authenticator.Stats())
}

// handleMaskingStats handles GET /api/v1/proxy/masking/stats
func (h *ProxyAPIHandler) handleMaskingStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.dataMasker == nil {
		h.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"enabled": false,
			"status":  "not_initialized",
		})
		return
	}

	h.jsonResponse(w, http.StatusOK, h.dataMasker.Stats())
}

// handleMaskingRules handles GET/POST/DELETE /api/v1/proxy/masking/rules
func (h *ProxyAPIHandler) handleMaskingRules(w http.ResponseWriter, r *http.Request) {
	if h.dataMasker == nil {
		http.Error(w, "Data masking not enabled", http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Return all rules including default rules
		h.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"rules":         h.dataMasker.ListRules(),
			"default_rules": GetDefaultRules(),
		})

	case http.MethodPost:
		var rule MaskingRule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if err := h.dataMasker.AddRule(&rule); err != nil {
			h.jsonResponse(w, http.StatusBadRequest, map[string]interface{}{
				"error": err.Error(),
			})
			return
		}
		h.jsonResponse(w, http.StatusCreated, map[string]interface{}{
			"message": "Masking rule added successfully",
			"rule":    rule,
		})

	case http.MethodDelete:
		ruleID := r.URL.Query().Get("id")
		if ruleID == "" {
			http.Error(w, "Rule ID is required", http.StatusBadRequest)
			return
		}

		if h.dataMasker.RemoveRule(ruleID) {
			h.jsonResponse(w, http.StatusOK, map[string]interface{}{
				"message": "Masking rule removed successfully",
			})
		} else {
			http.Error(w, "Rule not found", http.StatusNotFound)
		}

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// jsonResponse writes a JSON response
func (h *ProxyAPIHandler) jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// parseIntQuery parses an integer query parameter with a default value
func parseIntQuery(r *http.Request, key string, defaultVal int) int {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultVal
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return i
}
