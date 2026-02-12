# v0.10.5.1 API Proxy Core Infrastructure

## Overview

This is the first phase of the API Proxy Sidecar implementation, focusing on the core proxy infrastructure including dynamic port binding, basic HTTP/HTTPS proxy, connection pooling, intelligent model routing, and failover mechanisms.

> **Design Reference**: This PRD incorporates best practices from [Antigravity-Manager](https://github.com/lbjlaq/Antigravity-Manager), including intelligent model routing, quota management, and multi-protocol support.

## Goals

1. **Dynamic Port Binding** - Allocate available port at startup with discovery mechanism
2. **Basic HTTP/HTTPS Proxy** - Forward requests to upstream LLM providers
3. **Connection Pooling** - Efficient connection management with keep-alive
4. **Intelligent Model Router** - Series-based model mapping with smart routing strategies
5. **Failover Mechanism** - Circuit breaker and retry with backoff
6. **Quota Monitoring** - Real-time quota tracking and display (Dashboard integration)

## Non-Goals (This Version)

- Session monitoring (v0.10.5.2)
- Prompt injection guard (v0.10.5.2)
- Usage statistics (v0.10.5.2)
- Authentication (v0.10.5.2)
- Model compatibility layer (v0.10.5.3)
- Mock endpoints (v0.10.5.3)
- UI integration (v0.10.5.3)
- Multimodal & Imagen support (v0.10.5.4)

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      API Proxy Core (v0.10.5.1)                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌──────────────┐  ┌──────────────────┐  ┌──────────────────────────────┐   │
│  │   Listener   │  │   Model Router   │  │      Connection Pool         │   │
│  │  (Dynamic    │  │  (Series-based   │  │   (Keep-Alive, HTTP/2)       │   │
│  │   Port)      │  │   Mapping)       │  │                              │   │
│  └──────┬───────┘  └──────┬───────────┘  └──────────┬───────────────────┘   │
│         │                 │                          │                       │
│  ┌──────┴─────────────────┴──────────────────────────┴──────────────────┐   │
│  │                       Request Pipeline                                │   │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────────────┐ │   │
│  │  │ Receive │→│ Model   │→│ Account │→│ Forward │→│ Response        │ │   │
│  │  │ Request │ │ Route   │ │ Select  │ │ Request │ │ Handler         │ │   │
│  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────────────┘ │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌────────────────┐   │
│  │   Failover   │  │   Circuit    │  │   Health     │  │    Quota       │   │
│  │   Handler    │  │   Breaker    │  │   Check      │  │    Monitor     │   │
│  └──────────────┘  └──────────────┘  └──────────────┘  └────────────────┘   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Upstream Providers                                   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────┐   │
│  │Anthropic │  │  OpenAI  │  │  Gemini  │  │  Ollama  │  │Custom Endpoint│   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘  └──────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Core Features

### 1. Dynamic Port Binding

```go
// internal/proxy/config.go

// PortConfig port configuration
type PortConfig struct {
    Port        int    `json:"port"`         // 0 = dynamic allocation
    PortRange   string `json:"port_range"`   // e.g., "9000-9100"
    BindAddress string `json:"bind_address"` // Default: "127.0.0.1"
    PortFile    string `json:"port_file"`    // Write allocated port to file
}

// DefaultPortConfig returns default port configuration
func DefaultPortConfig() *PortConfig {
    return &PortConfig{
        Port:        0,
        PortRange:   "9000-9100",
        BindAddress: "127.0.0.1",
        PortFile:    "", // Will use default: ~/.local/share/zimaos-blue/proxy.port
    }
}
```

**Port Discovery Methods:**

1. **Port File**: Write port to `~/.local/share/zimaos-blue/proxy.port`
2. **Environment Variable**: Export as `BLUE_PROXY_PORT`
3. **API Endpoint**: Query `/api/v1/proxy/status` for current port

```go
// internal/proxy/port.go

// PortAllocator handles dynamic port allocation
type PortAllocator struct {
    config *PortConfig
    port   int
    mu     sync.RWMutex
}

// Allocate finds and binds to an available port
func (pa *PortAllocator) Allocate() (int, error) {
    // Parse port range
    // Try ports in range until one is available
    // Write port to file
    // Export to environment variable
}

// GetPort returns the currently allocated port
func (pa *PortAllocator) GetPort() int {
    pa.mu.RLock()
    defer pa.mu.RUnlock()
    return pa.port
}

// GetEndpoint returns the full proxy endpoint URL
func (pa *PortAllocator) GetEndpoint() string {
    return fmt.Sprintf("http://%s:%d", pa.config.BindAddress, pa.GetPort())
}
```

### 2. Basic HTTP/HTTPS Proxy

```go
// internal/proxy/server.go

// ProxyServer is the main proxy server
type ProxyServer struct {
    config     *ProxyConfig
    router     *Router
    connPool   *ConnectionPool
    failover   *FailoverHandler
    httpServer *http.Server

    // Lifecycle
    ctx    context.Context
    cancel context.CancelFunc
    wg     sync.WaitGroup
}

// NewProxyServer creates a new proxy server
func NewProxyServer(config *ProxyConfig) (*ProxyServer, error) {
    // Initialize components
    // Setup HTTP server
    // Configure TLS if needed
}

// Start starts the proxy server
func (ps *ProxyServer) Start() error {
    // Allocate port
    // Start HTTP server
    // Start health check goroutine
}

// Stop gracefully stops the proxy server
func (ps *ProxyServer) Stop(ctx context.Context) error {
    // Cancel context
    // Shutdown HTTP server
    // Close connection pool
    // Wait for goroutines
}
```

**Request Handler:**

```go
// internal/proxy/handler.go

// ProxyHandler handles incoming proxy requests
type ProxyHandler struct {
    router   *Router
    connPool *ConnectionPool
    failover *FailoverHandler
}

// ServeHTTP implements http.Handler
func (ph *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 1. Select provider based on route
    provider, err := ph.router.SelectProvider(r)
    if err != nil {
        http.Error(w, "No available provider", http.StatusServiceUnavailable)
        return
    }

    // 2. Forward request with failover
    resp, err := ph.failover.Execute(r.Context(), func() (*http.Response, error) {
        return ph.forwardRequest(r, provider)
    })

    // 3. Copy response to client
    ph.copyResponse(w, resp)
}

// forwardRequest forwards the request to upstream provider
func (ph *ProxyHandler) forwardRequest(r *http.Request, provider *Provider) (*http.Response, error) {
    // Clone request
    // Update URL to provider endpoint
    // Add/modify headers (API key, etc.)
    // Send request via connection pool
}
```

### 3. Connection Pool

```go
// internal/proxy/pool.go

// ConnectionPoolConfig connection pool configuration
type ConnectionPoolConfig struct {
    MaxIdleConns          int           `json:"max_idle_conns"`
    MaxIdleConnsPerHost   int           `json:"max_idle_conns_per_host"`
    MaxConnsPerHost       int           `json:"max_conns_per_host"`
    IdleConnTimeout       time.Duration `json:"idle_conn_timeout"`
    KeepAlive             bool          `json:"keep_alive"`
    KeepAliveInterval     time.Duration `json:"keep_alive_interval"`
    DialTimeout           time.Duration `json:"dial_timeout"`
    TLSHandshakeTimeout   time.Duration `json:"tls_handshake_timeout"`
    ResponseHeaderTimeout time.Duration `json:"response_header_timeout"`
    ForceHTTP2            bool          `json:"force_http2"`
}

// DefaultConnectionPoolConfig returns default configuration
func DefaultConnectionPoolConfig() *ConnectionPoolConfig {
    return &ConnectionPoolConfig{
        MaxIdleConns:          100,
        MaxIdleConnsPerHost:   10,
        MaxConnsPerHost:       100,
        IdleConnTimeout:       90 * time.Second,
        KeepAlive:             true,
        KeepAliveInterval:     30 * time.Second,
        DialTimeout:           30 * time.Second,
        TLSHandshakeTimeout:   10 * time.Second,
        ResponseHeaderTimeout: 60 * time.Second,
        ForceHTTP2:            true,
    }
}

// ConnectionPool manages HTTP connections to upstream providers
type ConnectionPool struct {
    config    *ConnectionPoolConfig
    transport *http.Transport
    clients   map[string]*http.Client // Per-provider clients
    mu        sync.RWMutex
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(config *ConnectionPoolConfig) *ConnectionPool {
    transport := &http.Transport{
        MaxIdleConns:          config.MaxIdleConns,
        MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
        MaxConnsPerHost:       config.MaxConnsPerHost,
        IdleConnTimeout:       config.IdleConnTimeout,
        TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
        ResponseHeaderTimeout: config.ResponseHeaderTimeout,
        DialContext: (&net.Dialer{
            Timeout:   config.DialTimeout,
            KeepAlive: config.KeepAliveInterval,
        }).DialContext,
        ForceAttemptHTTP2: config.ForceHTTP2,
    }

    return &ConnectionPool{
        config:    config,
        transport: transport,
        clients:   make(map[string]*http.Client),
    }
}

// GetClient returns an HTTP client for the given provider
func (cp *ConnectionPool) GetClient(provider string) *http.Client {
    cp.mu.RLock()
    if client, ok := cp.clients[provider]; ok {
        cp.mu.RUnlock()
        return client
    }
    cp.mu.RUnlock()

    cp.mu.Lock()
    defer cp.mu.Unlock()

    // Double-check after acquiring write lock
    if client, ok := cp.clients[provider]; ok {
        return client
    }

    client := &http.Client{
        Transport: cp.transport,
        Timeout:   0, // No timeout, handled by context
    }
    cp.clients[provider] = client
    return client
}

// Close closes all connections
func (cp *ConnectionPool) Close() {
    cp.transport.CloseIdleConnections()
}
```

### 4. Model Router (Intelligent Routing Center)

> **Inspired by**: [Antigravity-Manager](https://github.com/lbjlaq/Antigravity-Manager)'s Model Router design

The Model Router provides intelligent request routing with series-based model mapping, regex customization, and automatic background task downgrading.

```go
// internal/proxy/model_router.go

// ModelFamily represents a model series/family
type ModelFamily struct {
    Name     string   `json:"name"`      // e.g., "claude-3", "gpt-4", "gemini-pro"
    Patterns []string `json:"patterns"`  // Regex patterns to match model IDs
    Provider string   `json:"provider"`  // Target provider for this family
    Fallback string   `json:"fallback"`  // Fallback model for background tasks
}

// ModelRouterConfig configuration for model routing
type ModelRouterConfig struct {
    Enabled           bool           `json:"enabled"`
    Families          []*ModelFamily `json:"families"`
    BackgroundModels  []string       `json:"background_models"`  // Models for background tasks
    DefaultFamily     string         `json:"default_family"`
    RegexCustomRules  []*RegexRule   `json:"regex_rules"`        // Expert-level regex rules
}

// RegexRule allows expert-level model redirection
type RegexRule struct {
    Pattern     string `json:"pattern"`      // Regex pattern to match
    Target      string `json:"target"`       // Target model ID
    Provider    string `json:"provider"`     // Target provider
    Priority    int    `json:"priority"`     // Rule priority (lower = higher)
    Description string `json:"description"`  // Human-readable description
}

// ModelRouter handles intelligent model-based routing
type ModelRouter struct {
    config   *ModelRouterConfig
    families map[string]*ModelFamily
    rules    []*compiledRule
    mu       sync.RWMutex
}

type compiledRule struct {
    rule    *RegexRule
    pattern *regexp.Regexp
}

// NewModelRouter creates a new model router
func NewModelRouter(config *ModelRouterConfig) (*ModelRouter, error) {
    mr := &ModelRouter{
        config:   config,
        families: make(map[string]*ModelFamily),
    }

    // Compile regex rules
    for _, rule := range config.RegexCustomRules {
        compiled, err := regexp.Compile(rule.Pattern)
        if err != nil {
            return nil, fmt.Errorf("invalid regex pattern %q: %w", rule.Pattern, err)
        }
        mr.rules = append(mr.rules, &compiledRule{rule: rule, pattern: compiled})
    }

    // Sort rules by priority
    sort.Slice(mr.rules, func(i, j int) bool {
        return mr.rules[i].rule.Priority < mr.rules[j].rule.Priority
    })

    // Index families
    for _, family := range config.Families {
        mr.families[family.Name] = family
    }

    return mr, nil
}

// RouteModel determines the target provider and model for a request
func (mr *ModelRouter) RouteModel(requestedModel string, isBackground bool) (*ModelRoute, error) {
    mr.mu.RLock()
    defer mr.mu.RUnlock()

    // 1. Check custom regex rules first
    for _, cr := range mr.rules {
        if cr.pattern.MatchString(requestedModel) {
            return &ModelRoute{
                OriginalModel: requestedModel,
                TargetModel:   cr.rule.Target,
                Provider:      cr.rule.Provider,
                RuleApplied:   cr.rule.Description,
            }, nil
        }
    }

    // 2. Match against model families
    for _, family := range mr.config.Families {
        for _, pattern := range family.Patterns {
            matched, _ := regexp.MatchString(pattern, requestedModel)
            if matched {
                targetModel := requestedModel
                // Downgrade to fallback for background tasks
                if isBackground && family.Fallback != "" {
                    targetModel = family.Fallback
                }
                return &ModelRoute{
                    OriginalModel: requestedModel,
                    TargetModel:   targetModel,
                    Provider:      family.Provider,
                    Family:        family.Name,
                    Downgraded:    isBackground && family.Fallback != "",
                }, nil
            }
        }
    }

    return nil, ErrModelNotFound
}

// ModelRoute represents the routing decision
type ModelRoute struct {
    OriginalModel string `json:"original_model"`
    TargetModel   string `json:"target_model"`
    Provider      string `json:"provider"`
    Family        string `json:"family,omitempty"`
    RuleApplied   string `json:"rule_applied,omitempty"`
    Downgraded    bool   `json:"downgraded,omitempty"`
}

// IsBackgroundRequest detects if request is a background task (e.g., title generation)
func (mr *ModelRouter) IsBackgroundRequest(r *http.Request) bool {
    // Check for common background task indicators
    // 1. X-Background-Task header
    if r.Header.Get("X-Background-Task") == "true" {
        return true
    }

    // 2. Check request path for known background endpoints
    backgroundPaths := []string{"/title", "/summarize", "/embed"}
    for _, path := range backgroundPaths {
        if strings.Contains(r.URL.Path, path) {
            return true
        }
    }

    return false
}
```

**Model Family Configuration Example:**

```yaml
model_router:
  enabled: true
  default_family: "claude-3"

  families:
    - name: "claude-3"
      patterns:
        - "^claude-3.*"
        - "^claude-opus.*"
        - "^claude-sonnet.*"
      provider: "anthropic"
      fallback: "claude-3-haiku-20240307"  # For background tasks

    - name: "gpt-4"
      patterns:
        - "^gpt-4.*"
        - "^chatgpt-4.*"
      provider: "openai"
      fallback: "gpt-4o-mini"

    - name: "gemini-pro"
      patterns:
        - "^gemini-.*-pro.*"
        - "^gemini-pro.*"
      provider: "google"
      fallback: "gemini-1.5-flash"

    - name: "gemini-flash"
      patterns:
        - "^gemini-.*-flash.*"
      provider: "google"
      fallback: ""  # Already a flash model

  # Expert-level regex rules (highest priority)
  regex_rules:
    - pattern: "^claude-3-opus-latest$"
      target: "claude-3-opus-20240229"
      provider: "anthropic"
      priority: 1
      description: "Pin opus-latest to specific version"

    - pattern: "^gpt-4-turbo$"
      target: "gpt-4-turbo-2024-04-09"
      provider: "openai"
      priority: 2
      description: "Pin gpt-4-turbo to specific version"

  # Models considered as background/lightweight tasks
  background_models:
    - "claude-3-haiku"
    - "gpt-4o-mini"
    - "gemini-1.5-flash"
```

### 5. Provider Selection

```go
// internal/proxy/router.go

// ProviderConfig provider configuration
type ProviderConfig struct {
    Name        string `json:"name"`
    Endpoint    string `json:"endpoint"`
    APIKey      string `json:"api_key"`
    Priority    int    `json:"priority"`      // Lower = higher priority
    Weight      int    `json:"weight"`        // For weighted load balancing
    Enabled     bool   `json:"enabled"`
    HealthCheck string `json:"health_check"`  // Health check endpoint
}

// RouteConfig route configuration
type RouteConfig struct {
    DefaultProvider string            `json:"default_provider"`
    Providers       []*ProviderConfig `json:"providers"`
    LoadBalancing   string            `json:"load_balancing"` // priority, round-robin, weighted
}

// Router handles provider selection
type Router struct {
    config    *RouteConfig
    providers map[string]*Provider
    healthy   map[string]bool
    mu        sync.RWMutex

    // Round-robin state
    rrIndex int
}

// Provider represents an upstream provider
type Provider struct {
    Config      *ProviderConfig
    Healthy     bool
    LastCheck   time.Time
    LastError   error
    LastLatency time.Duration
}

// SelectProvider selects the best available provider
func (r *Router) SelectProvider(req *http.Request) (*Provider, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    // Get enabled and healthy providers
    available := r.getAvailableProviders()
    if len(available) == 0 {
        return nil, ErrNoAvailableProvider
    }

    // Select based on load balancing strategy
    switch r.config.LoadBalancing {
    case "round-robin":
        return r.selectRoundRobin(available), nil
    case "weighted":
        return r.selectWeighted(available), nil
    default: // "priority"
        return r.selectByPriority(available), nil
    }
}

// selectByPriority selects provider with highest priority (lowest number)
func (r *Router) selectByPriority(providers []*Provider) *Provider {
    sort.Slice(providers, func(i, j int) bool {
        return providers[i].Config.Priority < providers[j].Config.Priority
    })
    return providers[0]
}

// UpdateHealth updates provider health status
func (r *Router) UpdateHealth(name string, healthy bool, err error) {
    r.mu.Lock()
    defer r.mu.Unlock()

    if provider, ok := r.providers[name]; ok {
        provider.Healthy = healthy
        provider.LastCheck = time.Now()
        provider.LastError = err
    }
}
```

### 5. Failover Mechanism

```go
// internal/proxy/failover.go

// FailoverConfig failover configuration
type FailoverConfig struct {
    Enabled          bool          `json:"enabled"`
    MaxRetries       int           `json:"max_retries"`
    RetryDelay       time.Duration `json:"retry_delay"`
    CircuitBreaker   bool          `json:"circuit_breaker"`
    FailureThreshold int           `json:"failure_threshold"`
    RecoveryTimeout  time.Duration `json:"recovery_timeout"`
}

// DefaultFailoverConfig returns default failover configuration
func DefaultFailoverConfig() *FailoverConfig {
    return &FailoverConfig{
        Enabled:          true,
        MaxRetries:       3,
        RetryDelay:       time.Second,
        CircuitBreaker:   true,
        FailureThreshold: 5,
        RecoveryTimeout:  30 * time.Second,
    }
}

// CircuitState represents circuit breaker state
type CircuitState string

const (
    CircuitClosed   CircuitState = "closed"    // Normal operation
    CircuitOpen     CircuitState = "open"      // Failing, reject requests
    CircuitHalfOpen CircuitState = "half-open" // Testing recovery
)

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
    config       *FailoverConfig
    state        CircuitState
    failures     int
    lastFailure  time.Time
    mu           sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config *FailoverConfig) *CircuitBreaker {
    return &CircuitBreaker{
        config: config,
        state:  CircuitClosed,
    }
}

// Allow checks if request should be allowed
func (cb *CircuitBreaker) Allow() bool {
    cb.mu.RLock()
    defer cb.mu.RUnlock()

    switch cb.state {
    case CircuitClosed:
        return true
    case CircuitOpen:
        // Check if recovery timeout has passed
        if time.Since(cb.lastFailure) > cb.config.RecoveryTimeout {
            cb.mu.RUnlock()
            cb.mu.Lock()
            cb.state = CircuitHalfOpen
            cb.mu.Unlock()
            cb.mu.RLock()
            return true
        }
        return false
    case CircuitHalfOpen:
        return true
    }
    return false
}

// RecordSuccess records a successful request
func (cb *CircuitBreaker) RecordSuccess() {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    cb.failures = 0
    cb.state = CircuitClosed
}

// RecordFailure records a failed request
func (cb *CircuitBreaker) RecordFailure() {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    cb.failures++
    cb.lastFailure = time.Now()

    if cb.failures >= cb.config.FailureThreshold {
        cb.state = CircuitOpen
    }
}

// FailoverHandler handles request failover
type FailoverHandler struct {
    config   *FailoverConfig
    router   *Router
    breakers map[string]*CircuitBreaker
    mu       sync.RWMutex
}

// Execute executes request with failover support
func (fh *FailoverHandler) Execute(ctx context.Context, provider *Provider, fn func(*Provider) (*http.Response, error)) (*http.Response, error) {
    if !fh.config.Enabled {
        return fn(provider)
    }

    // Get circuit breaker for provider
    breaker := fh.getBreaker(provider.Config.Name)

    // Check circuit breaker
    if !breaker.Allow() {
        // Try next provider
        return fh.tryNextProvider(ctx, provider, fn)
    }

    // Execute with retries
    var lastErr error
    for attempt := 0; attempt <= fh.config.MaxRetries; attempt++ {
        if attempt > 0 {
            // Wait before retry with exponential backoff
            delay := fh.config.RetryDelay * time.Duration(1<<(attempt-1))
            select {
            case <-ctx.Done():
                return nil, ctx.Err()
            case <-time.After(delay):
            }
        }

        resp, err := fn(provider)
        if err == nil && resp.StatusCode < 500 {
            breaker.RecordSuccess()
            return resp, nil
        }

        lastErr = err
        if err == nil {
            lastErr = fmt.Errorf("upstream error: %d", resp.StatusCode)
        }
    }

    // All retries failed
    breaker.RecordFailure()

    // Try next provider
    return fh.tryNextProvider(ctx, provider, fn)
}

// tryNextProvider attempts to use the next available provider
func (fh *FailoverHandler) tryNextProvider(ctx context.Context, failed *Provider, fn func(*Provider) (*http.Response, error)) (*http.Response, error) {
    providers := fh.router.GetAvailableProviders()

    for _, p := range providers {
        if p.Config.Name == failed.Config.Name {
            continue
        }

        breaker := fh.getBreaker(p.Config.Name)
        if !breaker.Allow() {
            continue
        }

        resp, err := fn(p)
        if err == nil && resp.StatusCode < 500 {
            breaker.RecordSuccess()
            return resp, nil
        }

        breaker.RecordFailure()
    }

    return nil, ErrAllProvidersFailed
}
```

### 6. Health Check

```go
// internal/proxy/health.go

// HealthChecker performs health checks on providers
type HealthChecker struct {
    router   *Router
    connPool *ConnectionPool
    interval time.Duration
    timeout  time.Duration

    ctx    context.Context
    cancel context.CancelFunc
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(router *Router, connPool *ConnectionPool, interval, timeout time.Duration) *HealthChecker {
    ctx, cancel := context.WithCancel(context.Background())
    return &HealthChecker{
        router:   router,
        connPool: connPool,
        interval: interval,
        timeout:  timeout,
        ctx:      ctx,
        cancel:   cancel,
    }
}

// Start starts the health check loop
func (hc *HealthChecker) Start() {
    go hc.run()
}

// Stop stops the health checker
func (hc *HealthChecker) Stop() {
    hc.cancel()
}

// run is the main health check loop
func (hc *HealthChecker) run() {
    ticker := time.NewTicker(hc.interval)
    defer ticker.Stop()

    // Initial check
    hc.checkAll()

    for {
        select {
        case <-hc.ctx.Done():
            return
        case <-ticker.C:
            hc.checkAll()
        }
    }
}

// checkAll checks all providers
func (hc *HealthChecker) checkAll() {
    providers := hc.router.GetAllProviders()

    var wg sync.WaitGroup
    for _, p := range providers {
        wg.Add(1)
        go func(provider *Provider) {
            defer wg.Done()
            hc.checkProvider(provider)
        }(p)
    }
    wg.Wait()
}

// checkProvider checks a single provider
func (hc *HealthChecker) checkProvider(provider *Provider) {
    if provider.Config.HealthCheck == "" {
        return
    }

    ctx, cancel := context.WithTimeout(hc.ctx, hc.timeout)
    defer cancel()

    url := provider.Config.Endpoint + provider.Config.HealthCheck
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        hc.router.UpdateHealth(provider.Config.Name, false, err)
        return
    }

    // Add API key if required
    if provider.Config.APIKey != "" {
        req.Header.Set("Authorization", "Bearer "+provider.Config.APIKey)
    }

    client := hc.connPool.GetClient(provider.Config.Name)
    start := time.Now()
    resp, err := client.Do(req)
    latency := time.Since(start)

    if err != nil {
        hc.router.UpdateHealth(provider.Config.Name, false, err)
        return
    }
    defer resp.Body.Close()

    healthy := resp.StatusCode >= 200 && resp.StatusCode < 300
    hc.router.UpdateHealthWithLatency(provider.Config.Name, healthy, nil, latency)
}
```

### 8. Quota Monitor (Usage Tracking & Display)

> **Inspired by**: [Antigravity-Manager](https://github.com/lbjlaq/Antigravity-Manager)'s real-time quota monitoring dashboard

The Quota Monitor tracks API usage and remaining quotas across all providers, enabling intelligent routing decisions and dashboard display.

```go
// internal/proxy/quota.go

// QuotaInfo represents quota information for a provider/account
type QuotaInfo struct {
    Provider      string    `json:"provider"`
    AccountID     string    `json:"account_id,omitempty"`
    TotalQuota    int64     `json:"total_quota"`      // Total allowed requests/tokens
    UsedQuota     int64     `json:"used_quota"`       // Used requests/tokens
    RemainingPct  float64   `json:"remaining_pct"`    // Remaining percentage (0-100)
    ResetTime     time.Time `json:"reset_time"`       // When quota resets
    LastSync      time.Time `json:"last_sync"`        // Last sync time
    Status        string    `json:"status"`           // "healthy", "warning", "critical", "banned"
    RateLimited   bool      `json:"rate_limited"`     // Currently rate limited (429)
    BanDetected   bool      `json:"ban_detected"`     // 403 ban detected
}

// QuotaMonitorConfig configuration for quota monitoring
type QuotaMonitorConfig struct {
    Enabled         bool          `json:"enabled"`
    SyncInterval    time.Duration `json:"sync_interval"`     // How often to sync quotas
    WarningThreshold float64      `json:"warning_threshold"` // Warn when below this % (default: 20)
    CriticalThreshold float64     `json:"critical_threshold"` // Critical when below this % (default: 5)
    TrackTokens     bool          `json:"track_tokens"`      // Track token usage
    TrackRequests   bool          `json:"track_requests"`    // Track request count
}

// DefaultQuotaMonitorConfig returns default configuration
func DefaultQuotaMonitorConfig() *QuotaMonitorConfig {
    return &QuotaMonitorConfig{
        Enabled:          true,
        SyncInterval:     5 * time.Minute,
        WarningThreshold: 20.0,
        CriticalThreshold: 5.0,
        TrackTokens:      true,
        TrackRequests:    true,
    }
}

// QuotaMonitor tracks and manages quota information
type QuotaMonitor struct {
    config    *QuotaMonitorConfig
    quotas    map[string]*QuotaInfo  // key: provider:account_id
    mu        sync.RWMutex
    ctx       context.Context
    cancel    context.CancelFunc
}

// NewQuotaMonitor creates a new quota monitor
func NewQuotaMonitor(config *QuotaMonitorConfig) *QuotaMonitor {
    ctx, cancel := context.WithCancel(context.Background())
    return &QuotaMonitor{
        config: config,
        quotas: make(map[string]*QuotaInfo),
        ctx:    ctx,
        cancel: cancel,
    }
}

// RecordUsage records usage from a response
func (qm *QuotaMonitor) RecordUsage(provider string, resp *http.Response, tokensUsed int64) {
    qm.mu.Lock()
    defer qm.mu.Unlock()

    key := provider
    quota, exists := qm.quotas[key]
    if !exists {
        quota = &QuotaInfo{
            Provider: provider,
            Status:   "healthy",
        }
        qm.quotas[key] = quota
    }

    // Update usage
    quota.UsedQuota += tokensUsed
    quota.LastSync = time.Now()

    // Parse rate limit headers if present
    qm.parseRateLimitHeaders(quota, resp)

    // Update status based on remaining quota
    qm.updateStatus(quota)
}

// parseRateLimitHeaders extracts quota info from response headers
func (qm *QuotaMonitor) parseRateLimitHeaders(quota *QuotaInfo, resp *http.Response) {
    // Common rate limit headers
    // X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset
    // anthropic-ratelimit-requests-limit, anthropic-ratelimit-requests-remaining
    // x-ratelimit-limit-requests, x-ratelimit-remaining-requests (OpenAI)

    if limit := resp.Header.Get("X-RateLimit-Limit"); limit != "" {
        if v, err := strconv.ParseInt(limit, 10, 64); err == nil {
            quota.TotalQuota = v
        }
    }

    if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
        if v, err := strconv.ParseInt(remaining, 10, 64); err == nil {
            quota.UsedQuota = quota.TotalQuota - v
            if quota.TotalQuota > 0 {
                quota.RemainingPct = float64(v) / float64(quota.TotalQuota) * 100
            }
        }
    }

    if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
        if v, err := strconv.ParseInt(reset, 10, 64); err == nil {
            quota.ResetTime = time.Unix(v, 0)
        }
    }

    // Detect rate limiting (429) or ban (403)
    quota.RateLimited = resp.StatusCode == 429
    quota.BanDetected = resp.StatusCode == 403
}

// updateStatus updates quota status based on remaining percentage
func (qm *QuotaMonitor) updateStatus(quota *QuotaInfo) {
    if quota.BanDetected {
        quota.Status = "banned"
    } else if quota.RateLimited {
        quota.Status = "rate_limited"
    } else if quota.RemainingPct <= qm.config.CriticalThreshold {
        quota.Status = "critical"
    } else if quota.RemainingPct <= qm.config.WarningThreshold {
        quota.Status = "warning"
    } else {
        quota.Status = "healthy"
    }
}

// GetQuotaSummary returns a summary of all quotas for dashboard display
func (qm *QuotaMonitor) GetQuotaSummary() *QuotaSummary {
    qm.mu.RLock()
    defer qm.mu.RUnlock()

    summary := &QuotaSummary{
        Providers:       make([]*QuotaInfo, 0, len(qm.quotas)),
        AverageRemaining: 0,
        TotalProviders:  len(qm.quotas),
        HealthyCount:    0,
        WarningCount:    0,
        CriticalCount:   0,
        BannedCount:     0,
    }

    var totalPct float64
    for _, quota := range qm.quotas {
        summary.Providers = append(summary.Providers, quota)
        totalPct += quota.RemainingPct

        switch quota.Status {
        case "healthy":
            summary.HealthyCount++
        case "warning":
            summary.WarningCount++
        case "critical", "rate_limited":
            summary.CriticalCount++
        case "banned":
            summary.BannedCount++
        }
    }

    if len(qm.quotas) > 0 {
        summary.AverageRemaining = totalPct / float64(len(qm.quotas))
    }

    // Find best provider (highest remaining quota, not banned)
    summary.BestProvider = qm.findBestProvider()

    return summary
}

// findBestProvider returns the provider with highest remaining quota
func (qm *QuotaMonitor) findBestProvider() string {
    var best string
    var bestPct float64 = -1

    for _, quota := range qm.quotas {
        if quota.Status != "banned" && quota.RemainingPct > bestPct {
            bestPct = quota.RemainingPct
            best = quota.Provider
        }
    }

    return best
}

// QuotaSummary provides dashboard-ready quota information
type QuotaSummary struct {
    Providers        []*QuotaInfo `json:"providers"`
    AverageRemaining float64      `json:"average_remaining_pct"`
    TotalProviders   int          `json:"total_providers"`
    HealthyCount     int          `json:"healthy_count"`
    WarningCount     int          `json:"warning_count"`
    CriticalCount    int          `json:"critical_count"`
    BannedCount      int          `json:"banned_count"`
    BestProvider     string       `json:"best_provider"`
}
```

**Dashboard Display Features:**

```
┌─────────────────────────────────────────────────────────────────┐
│                    Quota Monitor Dashboard                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Average Remaining: ████████████████░░░░ 78.5%                  │
│                                                                  │
│  ┌─────────────┬──────────┬──────────┬─────────────────────┐    │
│  │  Provider   │  Status  │ Remaining│     Reset Time      │    │
│  ├─────────────┼──────────┼──────────┼─────────────────────┤    │
│  │  Anthropic  │ ● Healthy│   85.2%  │ 2026-01-30 12:00    │    │
│  │  OpenAI     │ ● Healthy│   72.1%  │ 2026-01-30 11:30    │    │
│  │  Gemini     │ ◐ Warning│   18.5%  │ 2026-01-30 10:00    │    │
│  │  Ollama     │ ● Healthy│   100%   │ N/A (Local)         │    │
│  └─────────────┴──────────┴──────────┴─────────────────────┘    │
│                                                                  │
│  Best Provider: Anthropic (85.2% remaining)                     │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## API Endpoints

### Proxy Management

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/proxy/status` | Get proxy status and port |
| GET | `/api/v1/proxy/health` | Health check endpoint |
| GET | `/api/v1/proxy/providers` | List configured providers |
| GET | `/api/v1/proxy/providers/:name` | Get provider details |
| POST | `/api/v1/proxy/providers/:name/health` | Trigger health check |

### Model Router

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/proxy/models` | List all model families and mappings |
| GET | `/api/v1/proxy/models/route` | Test model routing (query: `?model=xxx`) |
| PUT | `/api/v1/proxy/models/rules` | Update custom regex rules |

### Quota Monitor

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/proxy/quotas` | Get quota summary for all providers |
| GET | `/api/v1/proxy/quotas/:provider` | Get quota details for specific provider |
| GET | `/api/v1/proxy/quotas/best` | Get recommended best provider |

### API Response Examples

#### GET /api/v1/proxy/status

```json
{
  "status": "running",
  "port": 9042,
  "bind_address": "127.0.0.1",
  "endpoint": "http://127.0.0.1:9042",
  "uptime": "2h15m30s",
  "version": "0.10.5.1"
}
```

#### GET /api/v1/proxy/providers

```json
{
  "providers": [
    {
      "name": "anthropic",
      "endpoint": "https://api.anthropic.com",
      "priority": 1,
      "enabled": true,
      "healthy": true,
      "last_check": "2026-01-30T10:30:00Z",
      "last_latency_ms": 150
    },
    {
      "name": "openai",
      "endpoint": "https://api.openai.com",
      "priority": 2,
      "enabled": true,
      "healthy": true,
      "last_check": "2026-01-30T10:30:00Z",
      "last_latency_ms": 120
    }
  ],
  "default_provider": "anthropic",
  "load_balancing": "priority"
}
```

#### GET /api/v1/proxy/models

```json
{
  "families": [
    {
      "name": "claude-3",
      "patterns": ["^claude-3.*", "^claude-opus.*"],
      "provider": "anthropic",
      "fallback": "claude-3-haiku-20240307"
    },
    {
      "name": "gpt-4",
      "patterns": ["^gpt-4.*"],
      "provider": "openai",
      "fallback": "gpt-4o-mini"
    }
  ],
  "regex_rules_count": 2,
  "default_family": "claude-3"
}
```

#### GET /api/v1/proxy/models/route?model=claude-3-opus

```json
{
  "original_model": "claude-3-opus",
  "target_model": "claude-3-opus",
  "provider": "anthropic",
  "family": "claude-3",
  "downgraded": false
}
```

#### GET /api/v1/proxy/quotas

```json
{
  "providers": [
    {
      "provider": "anthropic",
      "total_quota": 1000000,
      "used_quota": 148000,
      "remaining_pct": 85.2,
      "reset_time": "2026-01-30T12:00:00Z",
      "last_sync": "2026-01-30T10:30:00Z",
      "status": "healthy",
      "rate_limited": false,
      "ban_detected": false
    },
    {
      "provider": "openai",
      "total_quota": 500000,
      "used_quota": 139500,
      "remaining_pct": 72.1,
      "reset_time": "2026-01-30T11:30:00Z",
      "last_sync": "2026-01-30T10:30:00Z",
      "status": "healthy",
      "rate_limited": false,
      "ban_detected": false
    },
    {
      "provider": "gemini",
      "total_quota": 100000,
      "used_quota": 81500,
      "remaining_pct": 18.5,
      "reset_time": "2026-01-30T10:00:00Z",
      "last_sync": "2026-01-30T10:30:00Z",
      "status": "warning",
      "rate_limited": false,
      "ban_detected": false
    }
  ],
  "average_remaining_pct": 58.6,
  "total_providers": 3,
  "healthy_count": 2,
  "warning_count": 1,
  "critical_count": 0,
  "banned_count": 0,
  "best_provider": "anthropic"
}
```

## Configuration

### config.yaml (v0.10.5.1 subset)

```yaml
# API Proxy Core Configuration
proxy:
  enabled: true

  # Port configuration
  port:
    value: 0                    # 0 = dynamic allocation
    range: "9000-9100"          # Port range for dynamic allocation
    bind_address: "127.0.0.1"   # Bind address (localhost only by default)
    port_file: ""               # Write port to file (empty = default location)

  # Route selection
  routing:
    default_provider: "anthropic"
    load_balancing: "priority"  # priority, round-robin, weighted
    providers:
      - name: "anthropic"
        endpoint: "https://api.anthropic.com"
        api_key: "${ANTHROPIC_API_KEY}"
        priority: 1
        enabled: true
        health_check: "/v1/models"

      - name: "openai"
        endpoint: "https://api.openai.com"
        api_key: "${OPENAI_API_KEY}"
        priority: 2
        enabled: true
        health_check: "/v1/models"

      - name: "ollama"
        endpoint: "http://localhost:11434"
        priority: 3
        enabled: true
        health_check: "/api/tags"

    # Failover configuration
    failover:
      enabled: true
      max_retries: 3
      retry_delay: "1s"
      circuit_breaker: true
      failure_threshold: 5
      recovery_timeout: "30s"

  # Connection pool
  connection:
    max_idle_conns: 100
    max_idle_conns_per_host: 10
    max_conns_per_host: 100
    idle_conn_timeout: "90s"
    keep_alive: true
    keep_alive_interval: "30s"
    dial_timeout: "30s"
    tls_handshake_timeout: "10s"
    response_header_timeout: "60s"
    force_http2: true

  # Health check
  health_check:
    enabled: true
    interval: "30s"
    timeout: "10s"

  # Model Router (Intelligent Routing)
  model_router:
    enabled: true
    default_family: "claude-3"

    families:
      - name: "claude-3"
        patterns:
          - "^claude-3.*"
          - "^claude-opus.*"
          - "^claude-sonnet.*"
        provider: "anthropic"
        fallback: "claude-3-haiku-20240307"

      - name: "gpt-4"
        patterns:
          - "^gpt-4.*"
          - "^chatgpt-4.*"
        provider: "openai"
        fallback: "gpt-4o-mini"

      - name: "gemini-pro"
        patterns:
          - "^gemini-.*-pro.*"
          - "^gemini-pro.*"
        provider: "google"
        fallback: "gemini-1.5-flash"

    # Expert-level regex rules
    regex_rules:
      - pattern: "^claude-3-opus-latest$"
        target: "claude-3-opus-20240229"
        provider: "anthropic"
        priority: 1
        description: "Pin opus-latest to specific version"

  # Quota Monitor
  quota_monitor:
    enabled: true
    sync_interval: "5m"
    warning_threshold: 20.0    # Warn when below 20%
    critical_threshold: 5.0    # Critical when below 5%
    track_tokens: true
    track_requests: true
```

## File Structure

```
internal/proxy/
├── config.go           # Configuration types
├── server.go           # Main proxy server
├── handler.go          # Request handler
├── port.go             # Port allocation
├── pool.go             # Connection pool
├── router.go           # Provider selection
├── model_router.go     # Model-based intelligent routing
├── failover.go         # Failover & circuit breaker
├── health.go           # Health checker
├── quota.go            # Quota monitoring
├── errors.go           # Error definitions
└── api.go              # Management API handlers

internal/proxy/
└── proxy_test.go       # Unit tests
```

## Implementation Checklist

### Phase 1: Core Infrastructure

- [ ] Create `internal/proxy/` package structure
- [ ] Implement `PortConfig` and `PortAllocator`
- [ ] Implement port file writing and env var export
- [ ] Write port allocation tests

### Phase 2: Connection Pool

- [ ] Implement `ConnectionPoolConfig`
- [ ] Implement `ConnectionPool` with HTTP/2 support
- [ ] Add keep-alive configuration
- [ ] Write connection pool tests

### Phase 3: Provider Selection

- [ ] Implement `ProviderConfig` and `RouteConfig`
- [ ] Implement `Router` with priority selection
- [ ] Add round-robin and weighted selection
- [ ] Write router tests

### Phase 4: Model Router (NEW)

- [ ] Implement `ModelFamily` and `ModelRouterConfig`
- [ ] Implement `ModelRouter` with regex pattern matching
- [ ] Add series-based model mapping
- [ ] Implement background task detection and auto-downgrade
- [ ] Add custom regex rules support
- [ ] Write model router tests

### Phase 5: Failover Mechanism

- [ ] Implement `CircuitBreaker`
- [ ] Implement `FailoverHandler` with retry logic
- [ ] Add exponential backoff
- [ ] Write failover tests

### Phase 6: Health Check

- [ ] Implement `HealthChecker`
- [ ] Add periodic health check loop
- [ ] Integrate with router health status
- [ ] Write health check tests

### Phase 7: Quota Monitor (NEW)

- [ ] Implement `QuotaInfo` and `QuotaMonitorConfig`
- [ ] Implement `QuotaMonitor` with rate limit header parsing
- [ ] Add quota status tracking (healthy/warning/critical/banned)
- [ ] Implement `GetQuotaSummary` for dashboard display
- [ ] Add best provider recommendation
- [ ] Write quota monitor tests

### Phase 8: Proxy Server

- [ ] Implement `ProxyServer`
- [ ] Implement `ProxyHandler`
- [ ] Integrate Model Router into request pipeline
- [ ] Integrate Quota Monitor into response handling
- [ ] Add request forwarding logic
- [ ] Add streaming response support (SSE)
- [ ] Write integration tests

### Phase 9: Management API

- [ ] Implement `/api/v1/proxy/status`
- [ ] Implement `/api/v1/proxy/health`
- [ ] Implement `/api/v1/proxy/providers`
- [ ] Implement `/api/v1/proxy/models` (model router)
- [ ] Implement `/api/v1/proxy/models/route` (test routing)
- [ ] Implement `/api/v1/proxy/quotas` (quota summary)
- [ ] Implement `/api/v1/proxy/quotas/best` (best provider)
- [ ] Write API tests

## Non-Functional Requirements

| Requirement | Target |
|-------------|--------|
| Proxy startup time | < 500ms |
| Proxy overhead (latency) | < 20ms P95 |
| Memory footprint | < 30MB |
| Health check latency | < 100ms |
| Port allocation time | < 100ms |

## Success Metrics

| Metric | Target |
|--------|--------|
| Proxy uptime | > 99.9% |
| Request success rate | > 99% |
| Failover success rate | > 95% |
| Circuit breaker accuracy | > 99% |

## Dependencies

- v0.10.0 Claude Code CLI Bundling
- v0.10.3 CC CLI Integration

## Next Versions

### v0.10.5.2 - Security & Monitoring
- Session monitoring
- Prompt injection guard
- Usage statistics
- Authentication

### v0.10.5.3 - Compatibility & UI
- Model compatibility layer
- Mock endpoints
- UI integration (Dashboard widgets)

### v0.10.5.4 - Multimodal Support
> **Inspired by**: [Antigravity-Manager](https://github.com/lbjlaq/Antigravity-Manager)'s Imagen 3 support

- Multimodal request handling (images, audio)
- Imagen 3 integration with quality controls
- Automatic aspect ratio mapping (`WIDTHxHEIGHT` → standard ratios)
- Quality levels: `hd` (4K), `medium` (2K), `standard`
- Large payload support (up to 100MB for 4K image recognition)
- OpenAI Images API compatibility

## Design References

This PRD incorporates best practices from:

- **[Antigravity-Manager](https://github.com/lbjlaq/Antigravity-Manager)** - Model Router design, quota monitoring, intelligent routing strategies
- Key features borrowed:
  - Series-based model mapping with regex customization
  - Background task auto-downgrade to Flash models
  - Real-time quota monitoring with status indicators
  - Best provider recommendation based on remaining quota
  - 429/403 error detection and intelligent retry
