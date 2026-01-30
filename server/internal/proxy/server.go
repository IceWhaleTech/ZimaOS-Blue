package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ProxyServer is the main proxy server
type ProxyServer struct {
	config       *ProxyConfig
	portAlloc    *PortAllocator
	connPool     *ConnectionPool
	router       *Router
	failover     *FailoverHandler
	healthCheck  *HealthChecker
	handler      *ProxyHandler
	httpServer   *http.Server

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

	// Initialize components
	portAlloc := NewPortAllocator(&config.Port)
	connPool := NewConnectionPool(&config.Connection)
	router := NewRouter(&config.Routing)
	failover := NewFailoverHandler(&config.Routing.Failover, router)
	healthCheck := NewHealthChecker(router, connPool, &config.HealthCheck)
	handler := NewProxyHandler(router, connPool, failover)

	return &ProxyServer{
		config:      config,
		portAlloc:   portAlloc,
		connPool:    connPool,
		router:      router,
		failover:    failover,
		healthCheck: healthCheck,
		handler:     handler,
		ctx:         ctx,
		cancel:      cancel,
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

	// Management API endpoints
	mux.HandleFunc("/api/v1/proxy/status", ps.handleStatus)
	mux.HandleFunc("/api/v1/proxy/health", ps.handleHealth)
	mux.HandleFunc("/api/v1/proxy/providers", ps.handleProviders)

	// Proxy handler for all other requests
	mux.Handle("/", ps.handler)

	ps.httpServer = &http.Server{
		Addr:    ps.portAlloc.GetBindAddress(),
		Handler: mux,
	}

	ps.startTime = time.Now()

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
		"timestamp": time.Now().Format(time.RFC3339),
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

	return map[string]interface{}{
		"status":          "running",
		"port":            ps.portAlloc.GetPort(),
		"endpoint":        ps.portAlloc.GetEndpoint(),
		"uptime":          time.Since(ps.startTime).String(),
		"router":          ps.router.Stats(),
		"connection_pool": ps.connPool.Stats(),
		"circuit_breakers": ps.failover.GetBreakerStats(),
	}
}
