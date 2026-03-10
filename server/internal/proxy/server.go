package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// ProxyServer is the main proxy server
type ProxyServer struct {
	config      *ProxyConfig
	portAlloc   *PortAllocator
	connPool    *ConnectionPool
	router      *Router
	failover    *FailoverHandler
	healthCheck *HealthChecker
	handler     *ProxyHandler
	httpServer  *http.Server

	// v0.10.5.1+ components (Antigravity-inspired)
	modelRouter  *ModelRouter
	quotaMonitor *QuotaMonitor

	// v0.10.5.2+ components
	sessionMonitor   *SessionMonitor
	metricsCollector *MetricsCollector
	promptGuard      *PromptGuard
	authenticator    *Authenticator

	// v0.10.5.3+ components
	modelCompat   *ModelCompatLayer
	mockHandler   *MockHandler
	configWatcher *ConfigWatcher
	apiHandler    *ProxyAPIHandler

	startTime time.Time
	mu        sync.RWMutex

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewProxyServer creates a new proxy server
func NewProxyServer(config *ProxyConfig) (*ProxyServer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize core components
	portAlloc := NewPortAllocator(&config.Port)
	connPool := NewConnectionPool(&config.Connection)
	router := NewRouter(&config.Routing)
	failover := NewFailoverHandler(&config.Routing.Failover, router)
	healthCheck := NewHealthChecker(router, connPool, &config.HealthCheck)
	handler := NewProxyHandler(router, connPool, failover)
	handler.SetResponsesIntegrationEnabled(false)

	// Initialize v0.10.5.1+ components (Antigravity-inspired)
	var modelRouter *ModelRouter
	var err error
	if config.ModelRouter != nil {
		modelRouter, err = NewModelRouter(config.ModelRouter)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to create model router: %w", err)
		}
	} else {
		modelRouter, _ = NewModelRouter(nil) // Use defaults
	}

	quotaMonitor := NewQuotaMonitor(config.QuotaMonitor)

	// Initialize v0.10.5.2+ components
	sessionMonitor := NewSessionMonitor(nil) // Use default config
	metricsCollector := NewMetricsCollector(nil)
	promptGuard := NewPromptGuard(nil)
	authenticator := NewAuthenticator(nil, nil)
	handler.SetSessionMonitor(sessionMonitor)

	// Initialize v0.10.5.3+ components
	modelCompat := NewModelCompatLayer(nil)
	mockHandler := NewMockHandler(nil)

	// Create API handler
	apiHandler := NewProxyAPIHandler(
		sessionMonitor,
		metricsCollector,
		promptGuard,
		nil, // configWatcher will be set later if needed
		modelCompat,
		mockHandler,
		authenticator,
	)

	return &ProxyServer{
		config:           config,
		portAlloc:        portAlloc,
		connPool:         connPool,
		router:           router,
		failover:         failover,
		healthCheck:      healthCheck,
		handler:          handler,
		modelRouter:      modelRouter,
		quotaMonitor:     quotaMonitor,
		sessionMonitor:   sessionMonitor,
		metricsCollector: metricsCollector,
		promptGuard:      promptGuard,
		authenticator:    authenticator,
		modelCompat:      modelCompat,
		mockHandler:      mockHandler,
		apiHandler:       apiHandler,
		ctx:              ctx,
		cancel:           cancel,
	}, nil
}

// Start starts the proxy server
func (ps *ProxyServer) Start() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Allocate port
	port, err := ps.portAlloc.Allocate()
	if err != nil {
		return fmt.Errorf("failed to allocate port: %w", err)
	}

	// Create HTTP server
	mux := http.NewServeMux()

	// Core management API endpoints
	mux.HandleFunc("/api/v1/proxy/status", ps.handleStatus)
	mux.HandleFunc("/api/v1/proxy/health", ps.handleHealth)
	mux.HandleFunc("/api/v1/proxy/providers", ps.handleProviders)
	mux.HandleFunc("/api/v1/proxy/providers/", ps.handleProviderByName) // handles /providers/:name and /providers/:name/health

	// Model Router API endpoints (v0.10.5.1+)
	mux.HandleFunc("/api/v1/proxy/models", ps.handleModels)
	mux.HandleFunc("/api/v1/proxy/models/route", ps.handleModelRoute)
	mux.HandleFunc("/api/v1/proxy/models/rules", ps.handleModelRules)

	// Quota Monitor API endpoints (v0.10.5.1+)
	mux.HandleFunc("/api/v1/proxy/quotas", ps.handleQuotas)
	mux.HandleFunc("/api/v1/proxy/quotas/best", ps.handleQuotasBest)

	// Register v0.10.5.2+ API routes (sessions, metrics, guard, auth)
	// Register v0.10.5.3+ API routes (models, mock, config)
	if ps.apiHandler != nil {
		ps.apiHandler.RegisterRoutes(mux)
	}

	// Anthropic-compatible API endpoints (for Claude Code CLI)
	mux.Handle("/v1/messages", ps.wrapWithSessionTracking(http.HandlerFunc(ps.handleAnthropicMessages)))

	// OpenAI-compatible API endpoints
	mux.Handle("/v1/chat/completions", ps.wrapWithSessionTracking(ps.handler))

	// Proxy handler for all other requests
	mux.Handle("/", ps.wrapWithSessionTracking(ps.handler))

	ps.httpServer = &http.Server{
		Addr:    ps.portAlloc.GetBindAddress(),
		Handler: mux,
	}

	ps.startTime = timeutil.NowTime()

	// Start health checker
	ps.healthCheck.Start()

	// Start HTTP server
	ps.wg.Add(1)
	go func() {
		defer ps.wg.Done()
		if err := ps.httpServer.ListenAndServe(); err != http.ErrServerClosed {
			fmt.Printf("Proxy server error: %v\n", err)
		}
	}()

	fmt.Printf("Proxy server started on port %d\n", port)
	return nil
}

// wrapWithSessionTracking attaches real request-chain events to SessionMonitor.
func (ps *ProxyServer) wrapWithSessionTracking(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ps.sessionMonitor == nil {
			next.ServeHTTP(w, r)
			return
		}

		clientIP := r.RemoteAddr
		if ps.authenticator != nil {
			clientIP = ps.authenticator.getClientIP(r)
		}
		session := ps.sessionMonitor.StartSession(clientIP, r.UserAgent())
		if session == nil {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("X-Session-ID", session.ID)
		reqWithSession := r.WithContext(WithSessionID(r.Context(), session.ID))

		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}
		next.ServeHTTP(rw, reqWithSession)

		// Some flows may not call UpdateSession (e.g. fast failures); ensure request is counted.
		if current, ok := ps.sessionMonitor.GetSession(session.ID); ok && current.RequestCount == 0 {
			ps.sessionMonitor.UpdateSession(
				session.ID,
				rw.Header().Get("X-Actual-Provider"),
				rw.Header().Get("X-Actual-Model"),
				0,
				0,
			)
		}

		status := SessionStatusCompleted
		if rw.statusCode >= 400 {
			status = SessionStatusFailed
			ps.sessionMonitor.RecordError(session.ID)
		}
		ps.sessionMonitor.CompleteSession(session.ID, status)
	})
}

// Stop gracefully stops the proxy server
func (ps *ProxyServer) Stop(ctx context.Context) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Cancel context
	ps.cancel()

	// Stop health checker
	ps.healthCheck.Stop()

	// Shutdown HTTP server
	if ps.httpServer != nil {
		if err := ps.httpServer.Shutdown(ctx); err != nil {
			return err
		}
	}

	// Close connection pool
	ps.connPool.Close()

	// Release port
	ps.portAlloc.Release()

	// Wait for goroutines
	ps.wg.Wait()

	return nil
}

// GetEndpoint returns the proxy endpoint URL
func (ps *ProxyServer) GetEndpoint() string {
	return ps.portAlloc.GetEndpoint()
}

// GetPort returns the proxy port
func (ps *ProxyServer) GetPort() int {
	return ps.portAlloc.GetPort()
}

// handleStatus handles GET /api/v1/proxy/status
func (ps *ProxyServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ps.mu.RLock()
	status := map[string]interface{}{
		"status":       "running",
		"port":         ps.portAlloc.GetPort(),
		"bind_address": ps.config.Port.BindAddress,
		"endpoint":     ps.portAlloc.GetEndpoint(),
		"uptime":       time.Since(ps.startTime).String(),
		"version":      "0.10.5.1",
	}
	ps.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleHealth handles GET /api/v1/proxy/health
func (ps *ProxyServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": timeutil.NowTime().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// handleProviders handles GET /api/v1/proxy/providers
func (ps *ProxyServer) handleProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	providers := ps.router.GetAllProviders()
	result := make([]map[string]interface{}, 0, len(providers))

	for _, p := range providers {
		item := map[string]interface{}{
			"name":            p.Config.Name,
			"endpoint":        p.Config.Endpoint,
			"priority":        p.Config.Priority,
			"enabled":         p.Config.Enabled,
			"healthy":         p.Healthy,
			"last_check":      p.LastCheck.Format(time.RFC3339),
			"last_latency_ms": p.LastLatency.Milliseconds(),
			"circuit_state":   ps.failover.GetBreakerState(p.Config.Name),
		}
		if p.LastError != nil {
			item["last_error"] = p.LastError.Error()
		}
		result = append(result, item)
	}

	response := map[string]interface{}{
		"providers":        result,
		"default_provider": ps.config.Routing.DefaultProvider,
		"load_balancing":   ps.config.Routing.LoadBalancing,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Stats returns proxy server statistics
func (ps *ProxyServer) Stats() map[string]interface{} {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	stats := map[string]interface{}{
		"status":           "running",
		"port":             ps.portAlloc.GetPort(),
		"endpoint":         ps.portAlloc.GetEndpoint(),
		"uptime":           time.Since(ps.startTime).String(),
		"router":           ps.router.Stats(),
		"connection_pool":  ps.connPool.Stats(),
		"circuit_breakers": ps.failover.GetBreakerStats(),
	}

	// Add model router stats
	if ps.modelRouter != nil {
		stats["model_router"] = ps.modelRouter.Stats()
	}

	// Add quota monitor stats
	if ps.quotaMonitor != nil {
		stats["quota_monitor"] = ps.quotaMonitor.Stats()
	}

	return stats
}

// handleModels handles GET /api/v1/proxy/models
func (ps *ProxyServer) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if ps.modelRouter == nil {
		http.Error(w, "Model router not configured", http.StatusNotFound)
		return
	}

	families := ps.modelRouter.GetFamilies()
	familyList := make([]map[string]interface{}, 0, len(families))
	for _, f := range families {
		familyList = append(familyList, map[string]interface{}{
			"name":     f.Name,
			"patterns": f.Patterns,
			"provider": f.Provider,
			"fallback": f.Fallback,
		})
	}

	response := map[string]interface{}{
		"families":          familyList,
		"regex_rules_count": len(ps.modelRouter.GetRules()),
		"default_family":    ps.modelRouter.config.DefaultFamily,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleModelRoute handles GET /api/v1/proxy/models/route?model=xxx
func (ps *ProxyServer) handleModelRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if ps.modelRouter == nil {
		http.Error(w, "Model router not configured", http.StatusNotFound)
		return
	}

	model := r.URL.Query().Get("model")
	if model == "" {
		http.Error(w, "Missing 'model' query parameter", http.StatusBadRequest)
		return
	}

	background := r.URL.Query().Get("background") == "true"
	route, err := ps.modelRouter.RouteModel(model, background)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(route)
}

// handleModelRules handles PUT /api/v1/proxy/models/rules
func (ps *ProxyServer) handleModelRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if ps.modelRouter == nil {
		http.Error(w, "Model router not configured", http.StatusNotFound)
		return
	}

	var rule RegexRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := ps.modelRouter.AddRule(&rule); err != nil {
		http.Error(w, "Failed to add rule: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Rule added successfully",
	})
}

// handleQuotas handles GET /api/v1/proxy/quotas
func (ps *ProxyServer) handleQuotas(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if ps.quotaMonitor == nil {
		http.Error(w, "Quota monitor not configured", http.StatusNotFound)
		return
	}

	// Check if requesting specific provider
	provider := r.URL.Query().Get("provider")
	if provider != "" {
		quota := ps.quotaMonitor.GetQuota(provider)
		if quota == nil {
			http.Error(w, "Provider not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(quota)
		return
	}

	// Return full summary
	summary := ps.quotaMonitor.GetQuotaSummary()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

// handleQuotasBest handles GET /api/v1/proxy/quotas/best
func (ps *ProxyServer) handleQuotasBest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if ps.quotaMonitor == nil {
		http.Error(w, "Quota monitor not configured", http.StatusNotFound)
		return
	}

	best := ps.quotaMonitor.GetBestProvider()
	quota := ps.quotaMonitor.GetQuota(best)

	response := map[string]interface{}{
		"best_provider": best,
	}
	if quota != nil {
		response["remaining_pct"] = quota.RemainingPct
		response["status"] = quota.Status
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetModelRouter returns the model router
func (ps *ProxyServer) GetModelRouter() *ModelRouter {
	return ps.modelRouter
}

// GetQuotaMonitor returns the quota monitor
func (ps *ProxyServer) GetQuotaMonitor() *QuotaMonitor {
	return ps.quotaMonitor
}

// handleProviderByName handles GET /api/v1/proxy/providers/:name and POST /api/v1/proxy/providers/:name/health
func (ps *ProxyServer) handleProviderByName(w http.ResponseWriter, r *http.Request) {
	// Extract provider name from path: /api/v1/proxy/providers/{name} or /api/v1/proxy/providers/{name}/health
	path := r.URL.Path
	prefix := "/api/v1/proxy/providers/"
	if len(path) <= len(prefix) {
		http.Error(w, "Provider name required", http.StatusBadRequest)
		return
	}

	remaining := path[len(prefix):]
	// Check if it's a health check request
	isHealthCheck := false
	providerName := remaining
	if idx := len(remaining) - len("/health"); idx > 0 && remaining[idx:] == "/health" {
		isHealthCheck = true
		providerName = remaining[:idx]
	}

	// Get provider
	provider, ok := ps.router.GetProvider(providerName)
	if !ok {
		http.Error(w, "Provider not found", http.StatusNotFound)
		return
	}

	if isHealthCheck {
		// POST /api/v1/proxy/providers/:name/health - trigger health check
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ps.handleTriggerHealthCheck(w, r, providerName)
		return
	}

	// GET /api/v1/proxy/providers/:name - get provider details
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	result := map[string]interface{}{
		"name":            provider.Config.Name,
		"endpoint":        provider.Config.Endpoint,
		"priority":        provider.Config.Priority,
		"weight":          provider.Config.Weight,
		"enabled":         provider.Config.Enabled,
		"healthy":         provider.Healthy,
		"last_check":      provider.LastCheck.Format(time.RFC3339),
		"last_latency_ms": provider.LastLatency.Milliseconds(),
		"circuit_state":   ps.failover.GetBreakerState(provider.Config.Name),
	}
	if provider.LastError != nil {
		result["last_error"] = provider.LastError.Error()
	}
	if provider.Config.HealthCheck != "" {
		result["health_check_endpoint"] = provider.Config.HealthCheck
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleTriggerHealthCheck triggers an immediate health check for a provider
func (ps *ProxyServer) handleTriggerHealthCheck(w http.ResponseWriter, r *http.Request, providerName string) {
	// Trigger health check
	ps.healthCheck.CheckProvider(providerName)

	// Get updated provider status
	provider, ok := ps.router.GetProvider(providerName)
	if !ok {
		http.Error(w, "Provider not found", http.StatusNotFound)
		return
	}

	result := map[string]interface{}{
		"provider":        providerName,
		"healthy":         provider.Healthy,
		"last_check":      provider.LastCheck.Format(time.RFC3339),
		"last_latency_ms": provider.LastLatency.Milliseconds(),
		"message":         "Health check triggered",
	}
	if provider.LastError != nil {
		result["last_error"] = provider.LastError.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleAnthropicMessages handles POST /v1/messages (Anthropic-compatible API)
// This endpoint allows Claude Code CLI to use the proxy with Anthropic API format.
// Supports automatic failover to next provider on failure.
func (ps *ProxyServer) handleAnthropicMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	bodyBytes, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Parse Anthropic request to extract model
	var anthropicReq map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &anthropicReq); err != nil {
		http.Error(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}

	model, _ := anthropicReq["model"].(string)
	isStreaming, _ := anthropicReq["stream"].(bool)

	// Route through Provider Pool to get provider + API key
	if ps.handler.providerPool == nil {
		http.Error(w, "No provider pool configured", http.StatusServiceUnavailable)
		return
	}

	// Get all candidates for failover
	route, err := ps.handler.providerPool.Router.Route(&providerpool.RouteRequest{
		ModelID: model,
	})
	if err != nil {
		http.Error(w, "No available provider: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	// Try primary provider first, then fallbacks
	providers := []*providerpool.Provider{route.Provider}
	for _, fallback := range route.Fallbacks {
		providers = append(providers, fallback.Provider)
	}

	var lastErr error
	var lastStatusCode int
	for i, provider := range providers {
		// Get API key for this provider
		apiKey, _ := ps.handler.providerPool.Registry.GetAPIKey(provider.ID)

		resp, statusCode, err := ps.tryAnthropicProvider(r.Context(), provider, apiKey, bodyBytes, isStreaming)
		if err == nil && statusCode < 500 {
			// Success - copy response to client with actual provider/model info
			ps.copyAnthropicResponse(w, resp, isStreaming, provider.Name, model)
			return
		}

		// Record failure for metrics
		lastErr = err
		lastStatusCode = statusCode
		if resp != nil {
			resp.Body.Close()
		}

		// Log failover attempt
		if i < len(providers)-1 {
			fmt.Printf("Anthropic API: Provider %s failed (status=%d, err=%v), trying next provider\n",
				provider.Name, statusCode, err)
		}
	}

	// All providers failed
	if lastErr != nil {
		http.Error(w, "All providers failed: "+lastErr.Error(), http.StatusBadGateway)
	} else {
		http.Error(w, fmt.Sprintf("All providers failed with status %d", lastStatusCode), http.StatusBadGateway)
	}
}

// tryAnthropicProvider attempts to send request to a single provider
func (ps *ProxyServer) tryAnthropicProvider(
	ctx context.Context,
	provider *providerpool.Provider,
	apiKey *providerpool.APIKey,
	bodyBytes []byte,
	isStreaming bool,
) (*http.Response, int, error) {
	// Build upstream URL
	targetURL, err := url.Parse(provider.BaseURL)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid provider URL: %w", err)
	}

	upstreamURL := *targetURL
	upstreamURL.Path = singleJoiningSlash(targetURL.Path, "/v1/messages")

	// Create upstream request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if apiKey != nil && apiKey.Key != "" {
		req.Header.Set("x-api-key", apiKey.Key)
	}
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Host = targetURL.Host

	// Send request
	client := ps.connPool.GetClient(provider.Name)
	if provider.SkipTLSVerify {
		client = ps.connPool.GetInsecureClient(provider.Name)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}

	return resp, resp.StatusCode, nil
}

// copyAnthropicResponse copies the response to the client
func (ps *ProxyServer) copyAnthropicResponse(w http.ResponseWriter, resp *http.Response, isStreaming bool, providerName, model string) {
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		if !isHopByHopHeader(key) {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
	}

	// Add actual provider/model info headers
	w.Header().Set("X-Actual-Provider", providerName)
	w.Header().Set("X-Actual-Model", model)

	// Handle streaming vs non-streaming response
	if isStreaming || strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(resp.StatusCode)

		flusher, ok := w.(http.Flusher)
		if !ok {
			io.Copy(w, resp.Body)
			return
		}

		buf := make([]byte, 4096)
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				w.Write(buf[:n])
				flusher.Flush()
			}
			if err != nil {
				break
			}
		}
	} else {
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}
