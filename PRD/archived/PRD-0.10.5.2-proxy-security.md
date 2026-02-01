# v0.10.5.2 API Proxy Security & Monitoring

## Overview

This is the second phase of the API Proxy Sidecar implementation, building on v0.10.5.1 to add session monitoring, prompt injection guard, usage statistics, and authentication.

## Goals

1. **Session Monitoring** - Track and monitor all active API sessions
2. **Prompt Injection Guard** - Detect and block prompt injection attacks
3. **Usage Statistics** - Token consumption, latency, and performance metrics
4. **Simple Authentication** - Bearer token and API key authentication
5. **Rate Limiting** - Request and token rate limiting

## Non-Goals (This Version)

- Model compatibility layer (v0.10.5.3)
- Mock endpoints (v0.10.5.3)
- Configuration hot-reload (v0.10.5.3)
- UI integration (v0.10.5.3)
- Full data masking (future)

## Prerequisites

- v0.10.5.1 Core Proxy Infrastructure (completed)

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                API Proxy Security & Monitoring (v0.10.5.2)      │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐   │
│  │   Auth       │  │   Session    │  │   Prompt Guard       │   │
│  │   Gate       │  │   Monitor    │  │   (Injection         │   │
│  │              │  │              │  │    Detection)        │   │
│  └──────┬───────┘  └──────┬───────┘  └──────────┬───────────┘   │
│         │                 │                      │               │
│  ┌──────┴─────────────────┴──────────────────────┴──────────┐   │
│  │                    Request Pipeline                       │   │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────────────┐ │   │
│  │  │  Auth   │→│ Prompt  │→│ Session │→│ Forward         │ │   │
│  │  │  Check  │ │ Guard   │ │ Track   │ │ (v0.10.5.1)     │ │   │
│  │  └─────────┘ └─────────┘ └─────────┘ └─────────────────┘ │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐   │
│  │   Metrics    │  │   Rate       │  │   Alert              │   │
│  │   Collector  │  │   Limiter    │  │   Manager            │   │
│  └──────────────┘  └──────────────┘  └──────────────────────┘   │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Core Features

### 1. Session Monitoring

```go
// internal/proxy/session.go

// Session represents an active API session
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
    Metadata     map[string]string `json:"metadata"`
}

type SessionStatus string

const (
    SessionActive    SessionStatus = "active"
    SessionIdle      SessionStatus = "idle"
    SessionCompleted SessionStatus = "completed"
    SessionError     SessionStatus = "error"
)

// SessionMonitor monitors active sessions
type SessionMonitor struct {
    sessions    map[string]*Session
    maxSessions int
    idleTimeout time.Duration
    mu          sync.RWMutex

    // Callbacks
    OnSessionStart func(session *Session)
    OnSessionEnd   func(session *Session)
}

// NewSessionMonitor creates a new session monitor
func NewSessionMonitor(maxSessions int, idleTimeout time.Duration) *SessionMonitor {
    return &SessionMonitor{
        sessions:    make(map[string]*Session),
        maxSessions: maxSessions,
        idleTimeout: idleTimeout,
    }
}

// StartSession starts a new session
func (sm *SessionMonitor) StartSession(id, provider, model string) (*Session, error) {
    sm.mu.Lock()
    defer sm.mu.Unlock()

    if len(sm.sessions) >= sm.maxSessions {
        return nil, ErrMaxSessionsReached
    }

    session := &Session{
        ID:           id,
        StartTime:    time.Now(),
        LastActivity: time.Now(),
        Provider:     provider,
        Model:        model,
        Status:       SessionActive,
        Metadata:     make(map[string]string),
    }

    sm.sessions[id] = session

    if sm.OnSessionStart != nil {
        sm.OnSessionStart(session)
    }

    return session, nil
}

// UpdateSession updates session activity
func (sm *SessionMonitor) UpdateSession(id string, tokensIn, tokensOut int64) {
    sm.mu.Lock()
    defer sm.mu.Unlock()

    if session, ok := sm.sessions[id]; ok {
        session.LastActivity = time.Now()
        session.RequestCount++
        session.TokensIn += tokensIn
        session.TokensOut += tokensOut
        session.Status = SessionActive
    }
}

// GetSession returns a session by ID
func (sm *SessionMonitor) GetSession(id string) (*Session, bool) {
    sm.mu.RLock()
    defer sm.mu.RUnlock()
    session, ok := sm.sessions[id]
    return session, ok
}

// ListSessions returns all sessions
func (sm *SessionMonitor) ListSessions() []*Session {
    sm.mu.RLock()
    defer sm.mu.RUnlock()

    sessions := make([]*Session, 0, len(sm.sessions))
    for _, s := range sm.sessions {
        sessions = append(sessions, s)
    }
    return sessions
}

// CleanupIdleSessions removes idle sessions
func (sm *SessionMonitor) CleanupIdleSessions() {
    sm.mu.Lock()
    defer sm.mu.Unlock()

    now := time.Now()
    for id, session := range sm.sessions {
        if now.Sub(session.LastActivity) > sm.idleTimeout {
            session.Status = SessionCompleted
            if sm.OnSessionEnd != nil {
                sm.OnSessionEnd(session)
            }
            delete(sm.sessions, id)
        }
    }
}
```

### 2. Prompt Injection Guard

```go
// internal/proxy/guard.go

// GuardMode defines how to handle detected injections
type GuardMode string

const (
    GuardModeBlock GuardMode = "block"  // Block and return error
    GuardModeWarn  GuardMode = "warn"   // Allow but log warning
    GuardModeLog   GuardMode = "log"    // Log only, no action
)

// InjectionRule defines a detection rule
type InjectionRule struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
    Pattern     string `json:"pattern"`      // Regex pattern
    Severity    string `json:"severity"`     // low, medium, high, critical
    Action      string `json:"action"`       // block, warn, log
    Enabled     bool   `json:"enabled"`
}

// InjectionAlert represents a detected injection attempt
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

// PromptGuard detects and blocks prompt injection attacks
type PromptGuard struct {
    enabled   bool
    mode      GuardMode
    rules     []*InjectionRule
    compiled  map[string]*regexp.Regexp
    whitelist []string
    alerts    []*InjectionAlert
    mu        sync.RWMutex

    OnDetection func(alert *InjectionAlert)
}

// NewPromptGuard creates a new prompt guard
func NewPromptGuard(enabled bool, mode GuardMode) *PromptGuard {
    pg := &PromptGuard{
        enabled:  enabled,
        mode:     mode,
        rules:    make([]*InjectionRule, 0),
        compiled: make(map[string]*regexp.Regexp),
        alerts:   make([]*InjectionAlert, 0),
    }
    pg.loadDefaultRules()
    return pg
}

// loadDefaultRules loads built-in detection rules
func (pg *PromptGuard) loadDefaultRules() {
    defaultRules := []*InjectionRule{
        {
            ID:          "INJ-001",
            Name:        "System Prompt Override",
            Description: "Attempts to override system instructions",
            Pattern:     `(?i)ignore.*previous.*instructions`,
            Severity:    "critical",
            Action:      "block",
            Enabled:     true,
        },
        {
            ID:          "INJ-002",
            Name:        "Role Manipulation",
            Description: "Attempts to change AI role",
            Pattern:     `(?i)(you are now|pretend to be|act as if)`,
            Severity:    "high",
            Action:      "block",
            Enabled:     true,
        },
        {
            ID:          "INJ-003",
            Name:        "Jailbreak Attempt",
            Description: "Known jailbreak patterns",
            Pattern:     `(?i)(DAN|do anything now|jailbreak)`,
            Severity:    "critical",
            Action:      "block",
            Enabled:     true,
        },
        {
            ID:          "INJ-004",
            Name:        "Instruction Injection",
            Description: "Instruction delimiter injection",
            Pattern:     `(\[INST\]|\[/INST\]|<<SYS>>|<</SYS>>)`,
            Severity:    "high",
            Action:      "block",
            Enabled:     true,
        },
        {
            ID:          "INJ-005",
            Name:        "Delimiter Injection",
            Description: "Section delimiter injection",
            Pattern:     `(?i)(###.*instruction|---.*system)`,
            Severity:    "medium",
            Action:      "warn",
            Enabled:     true,
        },
    }

    for _, rule := range defaultRules {
        pg.AddRule(rule)
    }
}

// AddRule adds a detection rule
func (pg *PromptGuard) AddRule(rule *InjectionRule) error {
    compiled, err := regexp.Compile(rule.Pattern)
    if err != nil {
        return fmt.Errorf("invalid pattern for rule %s: %w", rule.ID, err)
    }

    pg.mu.Lock()
    defer pg.mu.Unlock()

    pg.rules = append(pg.rules, rule)
    pg.compiled[rule.ID] = compiled
    return nil
}

// Check checks content for injection attempts
func (pg *PromptGuard) Check(sessionID, content string) (*InjectionAlert, bool) {
    if !pg.enabled {
        return nil, false
    }

    pg.mu.RLock()
    defer pg.mu.RUnlock()

    for _, rule := range pg.rules {
        if !rule.Enabled {
            continue
        }

        compiled := pg.compiled[rule.ID]
        if compiled == nil {
            continue
        }

        if match := compiled.FindString(content); match != "" {
            alert := &InjectionAlert{
                Timestamp:   time.Now(),
                SessionID:   sessionID,
                RuleID:      rule.ID,
                RuleName:    rule.Name,
                Severity:    rule.Severity,
                MatchedText: pg.truncateMatch(match, 100),
                Action:      rule.Action,
                Blocked:     rule.Action == "block",
            }

            pg.alerts = append(pg.alerts, alert)

            if pg.OnDetection != nil {
                pg.OnDetection(alert)
            }

            return alert, alert.Blocked
        }
    }

    return nil, false
}

// truncateMatch truncates matched text for logging
func (pg *PromptGuard) truncateMatch(match string, maxLen int) string {
    if len(match) <= maxLen {
        return match
    }
    return match[:maxLen] + "..."
}

// GetAlerts returns recent alerts
func (pg *PromptGuard) GetAlerts(limit int) []*InjectionAlert {
    pg.mu.RLock()
    defer pg.mu.RUnlock()

    if limit <= 0 || limit > len(pg.alerts) {
        limit = len(pg.alerts)
    }

    // Return most recent alerts
    start := len(pg.alerts) - limit
    if start < 0 {
        start = 0
    }

    result := make([]*InjectionAlert, limit)
    copy(result, pg.alerts[start:])
    return result
}
```

### 3. Usage Statistics

```go
// internal/proxy/metrics.go

// ProxyMetrics proxy-specific metrics
type ProxyMetrics struct {
    // Request metrics
    TotalRequests      int64 `json:"total_requests"`
    SuccessfulRequests int64 `json:"successful_requests"`
    FailedRequests     int64 `json:"failed_requests"`
    BlockedRequests    int64 `json:"blocked_requests"`

    // Token metrics (per model)
    TokensByModel map[string]*TokenStats `json:"tokens_by_model"`

    // Latency metrics
    TTFT            *LatencyStats `json:"ttft"`
    TotalLatency    *LatencyStats `json:"total_latency"`
    UpstreamLatency *LatencyStats `json:"upstream_latency"`
    ProxyOverhead   *LatencyStats `json:"proxy_overhead"`

    // Throughput
    TokensPerSecond   float64 `json:"tokens_per_second"`
    RequestsPerMinute float64 `json:"requests_per_minute"`

    // Provider metrics
    ProviderStats map[string]*ProviderStats `json:"provider_stats"`

    // Session metrics
    ActiveSessions     int64         `json:"active_sessions"`
    TotalSessions      int64         `json:"total_sessions"`
    AvgSessionDuration time.Duration `json:"avg_session_duration"`
}

// TokenStats token statistics
type TokenStats struct {
    InputTokens      int64   `json:"input_tokens"`
    OutputTokens     int64   `json:"output_tokens"`
    CacheReadTokens  int64   `json:"cache_read_tokens"`
    CacheWriteTokens int64   `json:"cache_write_tokens"`
    EstimatedCost    float64 `json:"estimated_cost"`
}

// LatencyStats latency statistics
type LatencyStats struct {
    samples []time.Duration
    Min     time.Duration `json:"min_ms"`
    Max     time.Duration `json:"max_ms"`
    Avg     time.Duration `json:"avg_ms"`
    P50     time.Duration `json:"p50_ms"`
    P95     time.Duration `json:"p95_ms"`
    P99     time.Duration `json:"p99_ms"`
}

// ProviderStats per-provider statistics
type ProviderStats struct {
    Provider      string        `json:"provider"`
    Requests      int64         `json:"requests"`
    Failures      int64         `json:"failures"`
    SuccessRate   float64       `json:"success_rate"`
    AvgLatency    time.Duration `json:"avg_latency_ms"`
    CircuitState  string        `json:"circuit_state"`
    LastError     string        `json:"last_error,omitempty"`
    LastErrorTime time.Time     `json:"last_error_time,omitempty"`
}

// MetricsCollector collects proxy metrics
type MetricsCollector struct {
    metrics   *ProxyMetrics
    startTime time.Time
    mu        sync.RWMutex

    // Sliding window for throughput calculation
    requestWindow []time.Time
    tokenWindow   []tokenSample
}

type tokenSample struct {
    timestamp time.Time
    count     int64
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
    return &MetricsCollector{
        metrics: &ProxyMetrics{
            TokensByModel: make(map[string]*TokenStats),
            ProviderStats: make(map[string]*ProviderStats),
            TTFT:          &LatencyStats{},
            TotalLatency:  &LatencyStats{},
            UpstreamLatency: &LatencyStats{},
            ProxyOverhead: &LatencyStats{},
        },
        startTime:     time.Now(),
        requestWindow: make([]time.Time, 0),
        tokenWindow:   make([]tokenSample, 0),
    }
}

// RecordRequest records a request
func (mc *MetricsCollector) RecordRequest(provider, model string, success bool, blocked bool) {
    mc.mu.Lock()
    defer mc.mu.Unlock()

    mc.metrics.TotalRequests++
    if success {
        mc.metrics.SuccessfulRequests++
    } else {
        mc.metrics.FailedRequests++
    }
    if blocked {
        mc.metrics.BlockedRequests++
    }

    // Update provider stats
    if _, ok := mc.metrics.ProviderStats[provider]; !ok {
        mc.metrics.ProviderStats[provider] = &ProviderStats{Provider: provider}
    }
    ps := mc.metrics.ProviderStats[provider]
    ps.Requests++
    if !success {
        ps.Failures++
    }
    ps.SuccessRate = float64(ps.Requests-ps.Failures) / float64(ps.Requests) * 100

    // Update request window for throughput
    mc.requestWindow = append(mc.requestWindow, time.Now())
    mc.cleanupWindow()
}

// RecordTokens records token usage
func (mc *MetricsCollector) RecordTokens(model string, input, output, cacheRead, cacheWrite int64) {
    mc.mu.Lock()
    defer mc.mu.Unlock()

    if _, ok := mc.metrics.TokensByModel[model]; !ok {
        mc.metrics.TokensByModel[model] = &TokenStats{}
    }

    ts := mc.metrics.TokensByModel[model]
    ts.InputTokens += input
    ts.OutputTokens += output
    ts.CacheReadTokens += cacheRead
    ts.CacheWriteTokens += cacheWrite

    // Estimate cost (simplified pricing)
    ts.EstimatedCost = mc.estimateCost(model, ts)

    // Update token window for throughput
    mc.tokenWindow = append(mc.tokenWindow, tokenSample{
        timestamp: time.Now(),
        count:     input + output,
    })
}

// RecordLatency records latency metrics
func (mc *MetricsCollector) RecordLatency(ttft, total, upstream, overhead time.Duration) {
    mc.mu.Lock()
    defer mc.mu.Unlock()

    mc.metrics.TTFT.addSample(ttft)
    mc.metrics.TotalLatency.addSample(total)
    mc.metrics.UpstreamLatency.addSample(upstream)
    mc.metrics.ProxyOverhead.addSample(overhead)
}

// addSample adds a latency sample and recalculates stats
func (ls *LatencyStats) addSample(d time.Duration) {
    ls.samples = append(ls.samples, d)

    // Keep only last 1000 samples
    if len(ls.samples) > 1000 {
        ls.samples = ls.samples[len(ls.samples)-1000:]
    }

    // Recalculate stats
    ls.recalculate()
}

// recalculate recalculates latency statistics
func (ls *LatencyStats) recalculate() {
    if len(ls.samples) == 0 {
        return
    }

    sorted := make([]time.Duration, len(ls.samples))
    copy(sorted, ls.samples)
    sort.Slice(sorted, func(i, j int) bool {
        return sorted[i] < sorted[j]
    })

    ls.Min = sorted[0]
    ls.Max = sorted[len(sorted)-1]

    var sum time.Duration
    for _, d := range sorted {
        sum += d
    }
    ls.Avg = sum / time.Duration(len(sorted))

    ls.P50 = sorted[len(sorted)*50/100]
    ls.P95 = sorted[len(sorted)*95/100]
    ls.P99 = sorted[len(sorted)*99/100]
}

// GetMetrics returns current metrics
func (mc *MetricsCollector) GetMetrics() *ProxyMetrics {
    mc.mu.RLock()
    defer mc.mu.RUnlock()

    mc.calculateThroughput()
    return mc.metrics
}

// calculateThroughput calculates throughput metrics
func (mc *MetricsCollector) calculateThroughput() {
    now := time.Now()
    oneMinuteAgo := now.Add(-time.Minute)

    // Requests per minute
    var recentRequests int
    for _, t := range mc.requestWindow {
        if t.After(oneMinuteAgo) {
            recentRequests++
        }
    }
    mc.metrics.RequestsPerMinute = float64(recentRequests)

    // Tokens per second
    var recentTokens int64
    for _, s := range mc.tokenWindow {
        if s.timestamp.After(oneMinuteAgo) {
            recentTokens += s.count
        }
    }
    mc.metrics.TokensPerSecond = float64(recentTokens) / 60.0
}

// cleanupWindow removes old entries from sliding windows
func (mc *MetricsCollector) cleanupWindow() {
    cutoff := time.Now().Add(-5 * time.Minute)

    // Cleanup request window
    newWindow := make([]time.Time, 0)
    for _, t := range mc.requestWindow {
        if t.After(cutoff) {
            newWindow = append(newWindow, t)
        }
    }
    mc.requestWindow = newWindow

    // Cleanup token window
    newTokenWindow := make([]tokenSample, 0)
    for _, s := range mc.tokenWindow {
        if s.timestamp.After(cutoff) {
            newTokenWindow = append(newTokenWindow, s)
        }
    }
    mc.tokenWindow = newTokenWindow
}

// estimateCost estimates cost based on model pricing
func (mc *MetricsCollector) estimateCost(model string, ts *TokenStats) float64 {
    // Simplified pricing (per 1M tokens)
    pricing := map[string]struct{ input, output float64 }{
        "claude-opus-4-5":   {15.0, 75.0},
        "claude-sonnet-4-5": {3.0, 15.0},
        "gpt-4o":            {5.0, 15.0},
        "gpt-4o-mini":       {0.15, 0.6},
    }

    p, ok := pricing[model]
    if !ok {
        p = struct{ input, output float64 }{3.0, 15.0} // Default
    }

    inputCost := float64(ts.InputTokens) / 1_000_000 * p.input
    outputCost := float64(ts.OutputTokens) / 1_000_000 * p.output

    return inputCost + outputCost
}
```

### 4. Authentication

```go
// internal/proxy/auth.go

// AuthType authentication type
type AuthType string

const (
    AuthTypeNone   AuthType = "none"
    AuthTypeBearer AuthType = "bearer"
    AuthTypeAPIKey AuthType = "api_key"
    AuthTypeBasic  AuthType = "basic"
)

// AuthConfig authentication configuration
type AuthConfig struct {
    Enabled   bool              `json:"enabled"`
    Type      AuthType          `json:"type"`
    Tokens    []*AuthToken      `json:"tokens"`
    RateLimit *RateLimitConfig  `json:"rate_limit,omitempty"`
}

// AuthToken represents an authentication token
type AuthToken struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    TokenHash   string    `json:"-"`              // Stored hashed
    Permissions []string  `json:"permissions"`    // e.g., ["read", "write", "admin"]
    ExpiresAt   time.Time `json:"expires_at,omitempty"`
    Enabled     bool      `json:"enabled"`
    CreatedAt   time.Time `json:"created_at"`
}

// RateLimitConfig rate limiting configuration
type RateLimitConfig struct {
    Enabled        bool `json:"enabled"`
    RequestsPerMin int  `json:"requests_per_min"`
    TokensPerMin   int  `json:"tokens_per_min"`
    BurstSize      int  `json:"burst_size"`
}

// AuthGate handles authentication
type AuthGate struct {
    config     *AuthConfig
    tokens     map[string]*AuthToken // tokenHash -> token
    rateLimits map[string]*rateLimiter
    mu         sync.RWMutex
}

// rateLimiter implements token bucket rate limiting
type rateLimiter struct {
    tokens     float64
    maxTokens  float64
    refillRate float64
    lastRefill time.Time
    mu         sync.Mutex
}

// NewAuthGate creates a new auth gate
func NewAuthGate(config *AuthConfig) *AuthGate {
    ag := &AuthGate{
        config:     config,
        tokens:     make(map[string]*AuthToken),
        rateLimits: make(map[string]*rateLimiter),
    }

    // Index tokens by hash
    for _, t := range config.Tokens {
        ag.tokens[t.TokenHash] = t
    }

    return ag
}

// Authenticate authenticates a request
func (ag *AuthGate) Authenticate(r *http.Request) (*AuthToken, error) {
    if !ag.config.Enabled {
        return nil, nil // Auth disabled
    }

    var token string

    switch ag.config.Type {
    case AuthTypeBearer:
        auth := r.Header.Get("Authorization")
        if !strings.HasPrefix(auth, "Bearer ") {
            return nil, ErrMissingToken
        }
        token = strings.TrimPrefix(auth, "Bearer ")

    case AuthTypeAPIKey:
        token = r.Header.Get("X-API-Key")
        if token == "" {
            token = r.URL.Query().Get("api_key")
        }
        if token == "" {
            return nil, ErrMissingToken
        }

    case AuthTypeBasic:
        username, password, ok := r.BasicAuth()
        if !ok {
            return nil, ErrMissingToken
        }
        token = username + ":" + password

    default:
        return nil, nil
    }

    // Hash and lookup token
    tokenHash := ag.hashToken(token)

    ag.mu.RLock()
    authToken, ok := ag.tokens[tokenHash]
    ag.mu.RUnlock()

    if !ok {
        return nil, ErrInvalidToken
    }

    if !authToken.Enabled {
        return nil, ErrTokenDisabled
    }

    if !authToken.ExpiresAt.IsZero() && time.Now().After(authToken.ExpiresAt) {
        return nil, ErrTokenExpired
    }

    // Check rate limit
    if ag.config.RateLimit != nil && ag.config.RateLimit.Enabled {
        if !ag.checkRateLimit(authToken.ID) {
            return nil, ErrRateLimitExceeded
        }
    }

    return authToken, nil
}

// hashToken hashes a token for storage/lookup
func (ag *AuthGate) hashToken(token string) string {
    h := sha256.Sum256([]byte(token))
    return hex.EncodeToString(h[:])
}

// checkRateLimit checks if request is within rate limit
func (ag *AuthGate) checkRateLimit(tokenID string) bool {
    ag.mu.Lock()
    rl, ok := ag.rateLimits[tokenID]
    if !ok {
        rl = &rateLimiter{
            tokens:     float64(ag.config.RateLimit.BurstSize),
            maxTokens:  float64(ag.config.RateLimit.BurstSize),
            refillRate: float64(ag.config.RateLimit.RequestsPerMin) / 60.0,
            lastRefill: time.Now(),
        }
        ag.rateLimits[tokenID] = rl
    }
    ag.mu.Unlock()

    return rl.allow()
}

// allow checks if a request is allowed
func (rl *rateLimiter) allow() bool {
    rl.mu.Lock()
    defer rl.mu.Unlock()

    // Refill tokens
    now := time.Now()
    elapsed := now.Sub(rl.lastRefill).Seconds()
    rl.tokens += elapsed * rl.refillRate
    if rl.tokens > rl.maxTokens {
        rl.tokens = rl.maxTokens
    }
    rl.lastRefill = now

    // Check if we have tokens
    if rl.tokens < 1 {
        return false
    }

    rl.tokens--
    return true
}

// CreateToken creates a new auth token
func (ag *AuthGate) CreateToken(name string, permissions []string, expiresAt time.Time) (string, *AuthToken, error) {
    // Generate random token
    tokenBytes := make([]byte, 32)
    if _, err := rand.Read(tokenBytes); err != nil {
        return "", nil, err
    }
    token := base64.URLEncoding.EncodeToString(tokenBytes)

    authToken := &AuthToken{
        ID:          uuid.New().String(),
        Name:        name,
        TokenHash:   ag.hashToken(token),
        Permissions: permissions,
        ExpiresAt:   expiresAt,
        Enabled:     true,
        CreatedAt:   time.Now(),
    }

    ag.mu.Lock()
    ag.tokens[authToken.TokenHash] = authToken
    ag.config.Tokens = append(ag.config.Tokens, authToken)
    ag.mu.Unlock()

    return token, authToken, nil
}

// RevokeToken revokes a token
func (ag *AuthGate) RevokeToken(tokenID string) error {
    ag.mu.Lock()
    defer ag.mu.Unlock()

    for hash, t := range ag.tokens {
        if t.ID == tokenID {
            delete(ag.tokens, hash)
            return nil
        }
    }

    return ErrTokenNotFound
}
```

## API Endpoints

### Session Management

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/proxy/sessions` | List active sessions |
| GET | `/api/v1/proxy/sessions/:id` | Get session details |
| DELETE | `/api/v1/proxy/sessions/:id` | Terminate session |
| GET | `/api/v1/proxy/sessions/:id/metrics` | Get session metrics |

### Security

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/proxy/security/alerts` | List security alerts |
| GET | `/api/v1/proxy/security/rules` | List prompt guard rules |
| PUT | `/api/v1/proxy/security/rules` | Update rules |
| GET | `/api/v1/proxy/security/blocked` | List blocked requests |

### Metrics

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/proxy/metrics` | Get proxy metrics |
| GET | `/api/v1/proxy/metrics/tokens` | Get token usage by model |
| GET | `/api/v1/proxy/metrics/latency` | Get latency statistics |
| GET | `/api/v1/proxy/metrics/providers` | Get per-provider metrics |
| GET | `/metrics` | Prometheus format metrics |

### Authentication

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/proxy/auth/tokens` | List auth tokens |
| POST | `/api/v1/proxy/auth/tokens` | Create new token |
| DELETE | `/api/v1/proxy/auth/tokens/:id` | Revoke token |

## Configuration

### config.yaml (v0.10.5.2 additions)

```yaml
proxy:
  # ... v0.10.5.1 config ...

  # Session monitoring
  sessions:
    max_sessions: 100
    idle_timeout: "30m"
    cleanup_interval: "5m"

  # Prompt injection guard
  prompt_guard:
    enabled: true
    mode: "block"               # block, warn, log
    rules:
      - id: "INJ-001"
        name: "System Prompt Override"
        pattern: "(?i)ignore.*previous.*instructions"
        severity: "critical"
        action: "block"
        enabled: true
    whitelist: []

  # Authentication
  auth:
    enabled: false
    type: "bearer"
    tokens: []
    rate_limit:
      enabled: false
      requests_per_min: 1000
      tokens_per_min: 1000000
      burst_size: 100

  # Metrics
  metrics:
    enabled: true
    prometheus_path: "/metrics"
    retention: "7d"
```

## File Structure

```
internal/proxy/
├── ... (v0.10.5.1 files)
├── session.go          # Session monitoring
├── guard.go            # Prompt injection guard
├── metrics.go          # Usage statistics
├── auth.go             # Authentication
└── middleware.go       # Request pipeline middleware
```

## Implementation Checklist

### Phase 1: Session Monitoring

- [ ] Implement `Session` struct
- [ ] Implement `SessionMonitor`
- [ ] Add session lifecycle management
- [ ] Add idle timeout cleanup
- [ ] Create session API endpoints
- [ ] Write session tests

### Phase 2: Prompt Injection Guard

- [ ] Implement `InjectionRule` struct
- [ ] Implement `PromptGuard`
- [ ] Add default detection rules
- [ ] Implement block/warn/log modes
- [ ] Create security API endpoints
- [ ] Write guard tests

### Phase 3: Usage Statistics

- [ ] Implement `ProxyMetrics` struct
- [ ] Implement `MetricsCollector`
- [ ] Add token counting per model
- [ ] Add latency tracking (TTFT, total, overhead)
- [ ] Implement throughput calculation
- [ ] Create metrics API endpoints
- [ ] Write metrics tests

### Phase 4: Authentication

- [ ] Implement `AuthConfig` and `AuthToken`
- [ ] Implement `AuthGate`
- [ ] Add bearer/API key/basic auth
- [ ] Implement rate limiting
- [ ] Create auth API endpoints
- [ ] Write auth tests

### Phase 5: Integration

- [ ] Integrate all components into request pipeline
- [ ] Add middleware chain
- [ ] Write integration tests
- [ ] Performance testing

## Non-Functional Requirements

| Requirement | Target |
|-------------|--------|
| Auth check latency | < 5ms |
| Prompt guard check | < 10ms |
| Metrics collection overhead | < 2ms |
| Session lookup | < 1ms |
| Alert detection latency | < 50ms |

## Success Metrics

| Metric | Target |
|--------|--------|
| Injection detection rate | > 95% |
| False positive rate | < 1% |
| Auth success rate | > 99.9% |
| Metrics accuracy | > 99% |

## Dependencies

- v0.10.5.1 Core Proxy Infrastructure
- v0.10.1 Metrics Monitoring (integration)

## Next Version

v0.10.5.3 will add:
- Model compatibility layer
- Mock endpoints
- Configuration hot-reload
- UI integration
