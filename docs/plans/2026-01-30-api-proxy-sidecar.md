# API Proxy Sidecar Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement a local API Proxy Sidecar that intercepts and manages all Claude Code CLI requests, providing high availability, session monitoring, prompt injection interception, and usage statistics.

**Architecture:** The proxy sidecar runs as an embedded component within ZimaOS-Echo, dynamically allocating a port and acting as a gateway for all LLM API traffic. It implements a request pipeline with auth, prompt guard, masking (reserved), route selection, and upstream forwarding.

**Tech Stack:** Go, Echo framework, net/http/httputil (reverse proxy), sync (concurrency), regexp (pattern matching)

---

## File Structure

```
server/internal/apiproxy/
├── config.go           # Configuration types
├── proxy.go            # Core proxy server
├── router.go           # Route selection and failover
├── session.go          # Session monitoring
├── guard.go            # Prompt injection detection
├── metrics.go          # Proxy-specific metrics
├── pool.go             # Connection pooling
├── compat.go           # Model compatibility layer
├── handler.go          # HTTP handlers for management API
├── middleware.go       # Request/response middleware
├── types.go            # Domain models
└── proxy_test.go       # Tests
```

---

## Task 1: Configuration Types

**Files:**
- Create: `server/internal/apiproxy/config.go`
- Modify: `server/internal/config/config.go`

**Step 1: Create config.go with all configuration types**

```go
// server/internal/apiproxy/config.go
package apiproxy

import "time"

// Config holds the complete API proxy sidecar configuration.
type Config struct {
	Enabled    bool              `json:"enabled" mapstructure:"enabled"`
	Port       PortConfig        `json:"port" mapstructure:"port"`
	Routing    RoutingConfig     `json:"routing" mapstructure:"routing"`
	Sessions   SessionConfig     `json:"sessions" mapstructure:"sessions"`
	PromptGuard PromptGuardConfig `json:"prompt_guard" mapstructure:"prompt_guard"`
	Masking    MaskingConfig     `json:"masking" mapstructure:"masking"`
	Auth       AuthConfig        `json:"auth" mapstructure:"auth"`
	Connection ConnectionConfig  `json:"connection" mapstructure:"connection"`
	ModelCompat ModelCompatConfig `json:"model_compat" mapstructure:"model_compat"`
	Mock       MockConfig        `json:"mock" mapstructure:"mock"`
	ConfigWatch ConfigWatchConfig `json:"config_watch" mapstructure:"config_watch"`
	Metrics    MetricsConfig     `json:"metrics" mapstructure:"metrics"`
}

// PortConfig holds port configuration.
type PortConfig struct {
	Value       int    `json:"value" mapstructure:"value"`
	Range       string `json:"range" mapstructure:"range"`
	BindAddress string `json:"bind_address" mapstructure:"bind_address"`
	PortFile    string `json:"port_file" mapstructure:"port_file"`
}

// RoutingConfig holds route selection configuration.
type RoutingConfig struct {
	DefaultProvider string           `json:"default_provider" mapstructure:"default_provider"`
	Providers       []ProviderConfig `json:"providers" mapstructure:"providers"`
	Failover        FailoverConfig   `json:"failover" mapstructure:"failover"`
}

// ProviderConfig holds provider configuration.
type ProviderConfig struct {
	Name        string `json:"name" mapstructure:"name"`
	Endpoint    string `json:"endpoint" mapstructure:"endpoint"`
	APIKey      string `json:"api_key" mapstructure:"api_key"`
	Priority    int    `json:"priority" mapstructure:"priority"`
	Weight      int    `json:"weight" mapstructure:"weight"`
	Enabled     bool   `json:"enabled" mapstructure:"enabled"`
	HealthCheck string `json:"health_check" mapstructure:"health_check"`
}

// FailoverConfig holds failover configuration.
type FailoverConfig struct {
	Enabled          bool          `json:"enabled" mapstructure:"enabled"`
	MaxRetries       int           `json:"max_retries" mapstructure:"max_retries"`
	RetryDelay       time.Duration `json:"retry_delay" mapstructure:"retry_delay"`
	CircuitBreaker   bool          `json:"circuit_breaker" mapstructure:"circuit_breaker"`
	FailureThreshold int           `json:"failure_threshold" mapstructure:"failure_threshold"`
	RecoveryTimeout  time.Duration `json:"recovery_timeout" mapstructure:"recovery_timeout"`
}

// SessionConfig holds session monitoring configuration.
type SessionConfig struct {
	MaxSessions     int           `json:"max_sessions" mapstructure:"max_sessions"`
	IdleTimeout     time.Duration `json:"idle_timeout" mapstructure:"idle_timeout"`
	CleanupInterval time.Duration `json:"cleanup_interval" mapstructure:"cleanup_interval"`
}

// PromptGuardConfig holds prompt injection guard configuration.
type PromptGuardConfig struct {
	Enabled   bool            `json:"enabled" mapstructure:"enabled"`
	Mode      string          `json:"mode" mapstructure:"mode"` // block, warn, log
	Rules     []InjectionRule `json:"rules" mapstructure:"rules"`
	Whitelist []string        `json:"whitelist" mapstructure:"whitelist"`
}

// InjectionRule defines a detection rule.
type InjectionRule struct {
	ID          string `json:"id" mapstructure:"id"`
	Name        string `json:"name" mapstructure:"name"`
	Description string `json:"description" mapstructure:"description"`
	Pattern     string `json:"pattern" mapstructure:"pattern"`
	Severity    string `json:"severity" mapstructure:"severity"` // low, medium, high, critical
	Action      string `json:"action" mapstructure:"action"`     // block, warn, log
	Enabled     bool   `json:"enabled" mapstructure:"enabled"`
}

// MaskingConfig holds data masking configuration (reserved for future).
type MaskingConfig struct {
	Enabled bool          `json:"enabled" mapstructure:"enabled"`
	Rules   []MaskingRule `json:"rules" mapstructure:"rules"`
}

// MaskingRule defines what to mask.
type MaskingRule struct {
	ID          string `json:"id" mapstructure:"id"`
	Name        string `json:"name" mapstructure:"name"`
	Pattern     string `json:"pattern" mapstructure:"pattern"`
	Replacement string `json:"replacement" mapstructure:"replacement"`
	Direction   string `json:"direction" mapstructure:"direction"` // request, response, both
	Enabled     bool   `json:"enabled" mapstructure:"enabled"`
}

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	Enabled   bool             `json:"enabled" mapstructure:"enabled"`
	Type      string           `json:"type" mapstructure:"type"` // none, bearer, api_key, basic
	Tokens    []AuthToken      `json:"tokens" mapstructure:"tokens"`
	RateLimit *RateLimitConfig `json:"rate_limit" mapstructure:"rate_limit"`
}

// AuthToken represents an authentication token.
type AuthToken struct {
	ID          string    `json:"id" mapstructure:"id"`
	Name        string    `json:"name" mapstructure:"name"`
	Token       string    `json:"token" mapstructure:"token"`
	Permissions []string  `json:"permissions" mapstructure:"permissions"`
	ExpiresAt   time.Time `json:"expires_at,omitempty" mapstructure:"expires_at"`
	Enabled     bool      `json:"enabled" mapstructure:"enabled"`
}

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	Enabled        bool `json:"enabled" mapstructure:"enabled"`
	RequestsPerMin int  `json:"requests_per_min" mapstructure:"requests_per_min"`
	TokensPerMin   int  `json:"tokens_per_min" mapstructure:"tokens_per_min"`
	BurstSize      int  `json:"burst_size" mapstructure:"burst_size"`
}

// ConnectionConfig holds connection pool configuration.
type ConnectionConfig struct {
	MaxIdleConns          int           `json:"max_idle_conns" mapstructure:"max_idle_conns"`
	MaxIdleConnsPerHost   int           `json:"max_idle_conns_per_host" mapstructure:"max_idle_conns_per_host"`
	MaxConnsPerHost       int           `json:"max_conns_per_host" mapstructure:"max_conns_per_host"`
	IdleConnTimeout       time.Duration `json:"idle_conn_timeout" mapstructure:"idle_conn_timeout"`
	KeepAlive             bool          `json:"keep_alive" mapstructure:"keep_alive"`
	KeepAliveInterval     time.Duration `json:"keep_alive_interval" mapstructure:"keep_alive_interval"`
	DialTimeout           time.Duration `json:"dial_timeout" mapstructure:"dial_timeout"`
	TLSHandshakeTimeout   time.Duration `json:"tls_handshake_timeout" mapstructure:"tls_handshake_timeout"`
	ResponseHeaderTimeout time.Duration `json:"response_header_timeout" mapstructure:"response_header_timeout"`
	ForceHTTP2            bool          `json:"force_http2" mapstructure:"force_http2"`
}

// ModelCompatConfig holds model compatibility configuration.
type ModelCompatConfig struct {
	AutoDetect       bool                      `json:"auto_detect" mapstructure:"auto_detect"`
	DetectionCache   time.Duration             `json:"detection_cache" mapstructure:"detection_cache"`
	ToolCallFallback string                    `json:"tool_call_fallback" mapstructure:"tool_call_fallback"` // error, prompt, skip
	ModelOverrides   map[string]*ModelFeatures `json:"model_overrides" mapstructure:"model_overrides"`
}

// ModelFeatures describes model capabilities.
type ModelFeatures struct {
	ToolCalling      bool `json:"tool_calling" mapstructure:"tool_calling"`
	Vision           bool `json:"vision" mapstructure:"vision"`
	Streaming        bool `json:"streaming" mapstructure:"streaming"`
	SystemPrompt     bool `json:"system_prompt" mapstructure:"system_prompt"`
	MaxContextTokens int  `json:"max_context_tokens" mapstructure:"max_context_tokens"`
	MaxOutputTokens  int  `json:"max_output_tokens" mapstructure:"max_output_tokens"`
}

// MockConfig holds mock endpoint configuration.
type MockConfig struct {
	Enabled   bool           `json:"enabled" mapstructure:"enabled"`
	Endpoints []MockEndpoint `json:"endpoints" mapstructure:"endpoints"`
}

// MockEndpoint defines a mock endpoint.
type MockEndpoint struct {
	Path       string      `json:"path" mapstructure:"path"`
	Method     string      `json:"method" mapstructure:"method"`
	Response   interface{} `json:"response" mapstructure:"response"`
	StatusCode int         `json:"status_code" mapstructure:"status_code"`
	Delay      string      `json:"delay" mapstructure:"delay"`
	Enabled    bool        `json:"enabled" mapstructure:"enabled"`
}

// ConfigWatchConfig holds configuration watching settings.
type ConfigWatchConfig struct {
	Enabled      bool          `json:"enabled" mapstructure:"enabled"`
	PollInterval time.Duration `json:"poll_interval" mapstructure:"poll_interval"`
}

// MetricsConfig holds metrics configuration.
type MetricsConfig struct {
	Enabled        bool          `json:"enabled" mapstructure:"enabled"`
	PrometheusPath string        `json:"prometheus_path" mapstructure:"prometheus_path"`
	Retention      time.Duration `json:"retention" mapstructure:"retention"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Enabled: true,
		Port: PortConfig{
			Value:       0, // Dynamic allocation
			Range:       "9000-9100",
			BindAddress: "127.0.0.1",
		},
		Routing: RoutingConfig{
			DefaultProvider: "anthropic",
			Failover: FailoverConfig{
				Enabled:          true,
				MaxRetries:       3,
				RetryDelay:       time.Second,
				CircuitBreaker:   true,
				FailureThreshold: 5,
				RecoveryTimeout:  30 * time.Second,
			},
		},
		Sessions: SessionConfig{
			MaxSessions:     100,
			IdleTimeout:     30 * time.Minute,
			CleanupInterval: 5 * time.Minute,
		},
		PromptGuard: PromptGuardConfig{
			Enabled: true,
			Mode:    "block",
			Rules:   DefaultInjectionRules(),
		},
		Auth: AuthConfig{
			Enabled: false,
			Type:    "bearer",
		},
		Connection: ConnectionConfig{
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
		},
		ModelCompat: ModelCompatConfig{
			AutoDetect:       true,
			DetectionCache:   time.Hour,
			ToolCallFallback: "prompt",
		},
		ConfigWatch: ConfigWatchConfig{
			Enabled:      true,
			PollInterval: 10 * time.Second,
		},
		Metrics: MetricsConfig{
			Enabled:        true,
			PrometheusPath: "/metrics",
			Retention:      7 * 24 * time.Hour,
		},
	}
}

// DefaultInjectionRules returns the default prompt injection detection rules.
func DefaultInjectionRules() []InjectionRule {
	return []InjectionRule{
		{
			ID:          "INJ-001",
			Name:        "System Prompt Override",
			Description: "Detects attempts to override system prompts",
			Pattern:     `(?i)ignore.*previous.*instructions`,
			Severity:    "critical",
			Action:      "block",
			Enabled:     true,
		},
		{
			ID:          "INJ-002",
			Name:        "Role Manipulation",
			Description: "Detects attempts to manipulate AI role",
			Pattern:     `(?i)(you are now|pretend to be|act as if you)`,
			Severity:    "high",
			Action:      "block",
			Enabled:     true,
		},
		{
			ID:          "INJ-003",
			Name:        "Jailbreak Attempt",
			Description: "Detects common jailbreak patterns",
			Pattern:     `(?i)(DAN|do anything now|jailbreak)`,
			Severity:    "critical",
			Action:      "block",
			Enabled:     true,
		},
		{
			ID:          "INJ-004",
			Name:        "Instruction Injection",
			Description: "Detects instruction delimiter injection",
			Pattern:     `(?i)(\[INST\]|\[/INST\]|<<SYS>>|<</SYS>>)`,
			Severity:    "high",
			Action:      "block",
			Enabled:     true,
		},
		{
			ID:          "INJ-005",
			Name:        "Delimiter Injection",
			Description: "Detects delimiter-based injection",
			Pattern:     `(?i)(###.*instruction|---.*system)`,
			Severity:    "medium",
			Action:      "warn",
			Enabled:     true,
		},
		{
			ID:          "INJ-006",
			Name:        "Encoding Bypass",
			Description: "Detects encoding-based bypass attempts",
			Pattern:     `(?i)(base64.*decode|rot13|hex.*decode)`,
			Severity:    "medium",
			Action:      "warn",
			Enabled:     true,
		},
	}
}
```

**Step 2: Run go build to verify syntax**

Run: `cd "g:\GitHub\ZimaOS-Echo\server" && go build ./internal/apiproxy/...`
Expected: Build succeeds (or package not found, which is fine at this stage)

---

## Task 2: Domain Types

**Files:**
- Create: `server/internal/apiproxy/types.go`

**Step 1: Create types.go with domain models**

```go
// server/internal/apiproxy/types.go
package apiproxy

import (
	"sync"
	"time"
)

// SessionStatus represents the status of a session.
type SessionStatus string

const (
	SessionActive    SessionStatus = "active"
	SessionIdle      SessionStatus = "idle"
	SessionCompleted SessionStatus = "completed"
	SessionError     SessionStatus = "error"
)

// Session represents an active API session.
type Session struct {
	ID           string            `json:"id"`
	StartTime    time.Time         `json:"start_time"`
	LastActivity time.Time         `json:"last_activity"`
	Provider     string            `json:"provider"`
	Model        string            `json:"model"`
	RequestCount int64             `json:"request_count"`
	TokensIn     int64             `json:"tokens_in"`
	TokensOut    int64             `json:"tokens_out"`
	Status       SessionStatus     `json:"status"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	mu           sync.RWMutex
}

// Update updates session statistics.
func (s *Session) Update(tokensIn, tokensOut int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastActivity = time.Now()
	s.RequestCount++
	s.TokensIn += tokensIn
	s.TokensOut += tokensOut
	s.Status = SessionActive
}

// SetStatus sets the session status.
func (s *Session) SetStatus(status SessionStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = status
}

// InjectionAlert represents a detected injection attempt.
type InjectionAlert struct {
	Timestamp   time.Time `json:"timestamp"`
	SessionID   string    `json:"session_id"`
	RuleID      string    `json:"rule_id"`
	RuleName    string    `json:"rule_name"`
	Severity    string    `json:"severity"`
	MatchedText string    `json:"matched_text"`
	Action      string    `json:"action_taken"`
	Blocked     bool      `json:"blocked"`
}

// CircuitState represents the state of a circuit breaker.
type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half-open"
)

// CircuitBreaker implements the circuit breaker pattern.
type CircuitBreaker struct {
	mu               sync.RWMutex
	state            CircuitState
	failures         int
	successes        int
	lastFailure      time.Time
	failureThreshold int
	recoveryTimeout  time.Duration
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(failureThreshold int, recoveryTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            CircuitClosed,
		failureThreshold: failureThreshold,
		recoveryTimeout:  recoveryTimeout,
	}
}

// Allow checks if a request should be allowed.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(cb.lastFailure) > cb.recoveryTimeout {
			return true // Allow one request to test
		}
		return false
	case CircuitHalfOpen:
		return true
	}
	return true
}

// RecordSuccess records a successful request.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == CircuitHalfOpen {
		cb.successes++
		if cb.successes >= 2 {
			cb.state = CircuitClosed
			cb.failures = 0
			cb.successes = 0
		}
	}
}

// RecordFailure records a failed request.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()

	if cb.failures >= cb.failureThreshold {
		cb.state = CircuitOpen
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// ProxyStatus represents the proxy status.
type ProxyStatus struct {
	Status           string    `json:"status"`
	Port             int       `json:"port"`
	BindAddress      string    `json:"bind_address"`
	Endpoint         string    `json:"endpoint"`
	Uptime           string    `json:"uptime"`
	Version          string    `json:"version"`
	ActiveSessions   int       `json:"active_sessions"`
	TotalRequests    int64     `json:"total_requests"`
	ConfigLastReload time.Time `json:"config_last_reload"`
}

// ProviderStatus represents a provider's status.
type ProviderStatus struct {
	Name         string       `json:"name"`
	Endpoint     string       `json:"endpoint"`
	Enabled      bool         `json:"enabled"`
	Priority     int          `json:"priority"`
	Healthy      bool         `json:"healthy"`
	CircuitState CircuitState `json:"circuit_state"`
	LastCheck    time.Time    `json:"last_check,omitempty"`
	LastError    string       `json:"last_error,omitempty"`
}

// TokenStats holds token statistics.
type TokenStats struct {
	InputTokens      int64   `json:"input_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	CacheReadTokens  int64   `json:"cache_read_tokens"`
	CacheWriteTokens int64   `json:"cache_write_tokens"`
	EstimatedCost    float64 `json:"estimated_cost"`
}

// LatencyStats holds latency statistics.
type LatencyStats struct {
	Min time.Duration `json:"min_ms"`
	Max time.Duration `json:"max_ms"`
	Avg time.Duration `json:"avg_ms"`
	P50 time.Duration `json:"p50_ms"`
	P95 time.Duration `json:"p95_ms"`
	P99 time.Duration `json:"p99_ms"`
}

// ProviderStats holds per-provider statistics.
type ProviderStats struct {
	Provider      string        `json:"provider"`
	Requests      int64         `json:"requests"`
	Failures      int64         `json:"failures"`
	SuccessRate   float64       `json:"success_rate"`
	AvgLatency    time.Duration `json:"avg_latency_ms"`
	CircuitState  CircuitState  `json:"circuit_state"`
	LastError     string        `json:"last_error,omitempty"`
	LastErrorTime time.Time     `json:"last_error_time,omitempty"`
}

// ProxyMetrics holds proxy-specific metrics.
type ProxyMetrics struct {
	TotalRequests      int64                    `json:"total_requests"`
	SuccessfulRequests int64                    `json:"successful_requests"`
	FailedRequests     int64                    `json:"failed_requests"`
	BlockedRequests    int64                    `json:"blocked_requests"`
	TokensByModel      map[string]*TokenStats   `json:"tokens_by_model"`
	TTFT               *LatencyStats            `json:"ttft"`
	TotalLatency       *LatencyStats            `json:"total_latency"`
	ProxyOverhead      *LatencyStats            `json:"proxy_overhead"`
	TokensPerSecond    float64                  `json:"tokens_per_second"`
	RequestsPerMinute  float64                  `json:"requests_per_minute"`
	ProviderStats      map[string]*ProviderStats `json:"provider_stats"`
	ActiveSessions     int64                    `json:"active_sessions"`
	TotalSessions      int64                    `json:"total_sessions"`
	AvgSessionDuration time.Duration            `json:"avg_session_duration"`
}
```

---

## Task 3: Core Proxy Server

**Files:**
- Create: `server/internal/apiproxy/proxy.go`
