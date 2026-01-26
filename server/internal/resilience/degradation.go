package resilience

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// DegradationLevel represents the level of service degradation
type DegradationLevel int

const (
	// LevelNormal is the normal operation level
	LevelNormal DegradationLevel = iota
	// LevelPartial is partial degradation (some features disabled)
	LevelPartial
	// LevelMinimal is minimal service (only essential features)
	LevelMinimal
	// LevelEmergency is emergency mode (static responses only)
	LevelEmergency
)

func (l DegradationLevel) String() string {
	switch l {
	case LevelNormal:
		return "normal"
	case LevelPartial:
		return "partial"
	case LevelMinimal:
		return "minimal"
	case LevelEmergency:
		return "emergency"
	default:
		return "unknown"
	}
}

// DegradationConfig holds configuration for graceful degradation
type DegradationConfig struct {
	// HealthCheckInterval is how often to check system health
	HealthCheckInterval time.Duration

	// ThresholdPartial is the error rate threshold for partial degradation
	ThresholdPartial float64

	// ThresholdMinimal is the error rate threshold for minimal degradation
	ThresholdMinimal float64

	// ThresholdEmergency is the error rate threshold for emergency mode
	ThresholdEmergency float64

	// RecoveryTime is how long to wait before attempting recovery
	RecoveryTime time.Duration

	// OnLevelChange is called when degradation level changes
	OnLevelChange func(from, to DegradationLevel)
}

// DegradationManager manages graceful degradation
type DegradationManager struct {
	config DegradationConfig

	mu              sync.RWMutex
	level           DegradationLevel
	manualOverride  bool
	lastLevelChange time.Time

	// Metrics
	totalRequests int64
	failedRequests int64
	windowStart   time.Time
}

// NewDegradationManager creates a new degradation manager
func NewDegradationManager(cfg DegradationConfig) *DegradationManager {
	if cfg.HealthCheckInterval <= 0 {
		cfg.HealthCheckInterval = 10 * time.Second
	}
	if cfg.ThresholdPartial <= 0 {
		cfg.ThresholdPartial = 0.1 // 10% error rate
	}
	if cfg.ThresholdMinimal <= 0 {
		cfg.ThresholdMinimal = 0.3 // 30% error rate
	}
	if cfg.ThresholdEmergency <= 0 {
		cfg.ThresholdEmergency = 0.5 // 50% error rate
	}
	if cfg.RecoveryTime <= 0 {
		cfg.RecoveryTime = 60 * time.Second
	}

	return &DegradationManager{
		config:      cfg,
		level:       LevelNormal,
		windowStart: time.Now(),
	}
}

// Level returns the current degradation level
func (m *DegradationManager) Level() DegradationLevel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.level
}

// SetLevel manually sets the degradation level
func (m *DegradationManager) SetLevel(level DegradationLevel) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.level != level {
		from := m.level
		m.level = level
		m.manualOverride = true
		m.lastLevelChange = time.Now()

		if m.config.OnLevelChange != nil {
			go m.config.OnLevelChange(from, level)
		}
	}
}

// ClearOverride clears manual override and returns to automatic management
func (m *DegradationManager) ClearOverride() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.manualOverride = false
}

// RecordRequest records a request result for error rate calculation
func (m *DegradationManager) RecordRequest(success bool) {
	atomic.AddInt64(&m.totalRequests, 1)
	if !success {
		atomic.AddInt64(&m.failedRequests, 1)
	}
}

// ErrorRate returns the current error rate
func (m *DegradationManager) ErrorRate() float64 {
	total := atomic.LoadInt64(&m.totalRequests)
	if total == 0 {
		return 0
	}
	failed := atomic.LoadInt64(&m.failedRequests)
	return float64(failed) / float64(total)
}

// UpdateLevel updates the degradation level based on current metrics
func (m *DegradationManager) UpdateLevel() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.manualOverride {
		return
	}

	errorRate := m.ErrorRate()
	var newLevel DegradationLevel

	switch {
	case errorRate >= m.config.ThresholdEmergency:
		newLevel = LevelEmergency
	case errorRate >= m.config.ThresholdMinimal:
		newLevel = LevelMinimal
	case errorRate >= m.config.ThresholdPartial:
		newLevel = LevelPartial
	default:
		// Only recover if enough time has passed
		if m.level != LevelNormal && time.Since(m.lastLevelChange) >= m.config.RecoveryTime {
			newLevel = m.level - 1 // Step down one level
		} else {
			newLevel = m.level
		}
	}

	if newLevel != m.level {
		from := m.level
		m.level = newLevel
		m.lastLevelChange = time.Now()

		if m.config.OnLevelChange != nil {
			go m.config.OnLevelChange(from, newLevel)
		}
	}
}

// ResetMetrics resets the error rate metrics
func (m *DegradationManager) ResetMetrics() {
	atomic.StoreInt64(&m.totalRequests, 0)
	atomic.StoreInt64(&m.failedRequests, 0)
	m.mu.Lock()
	m.windowStart = time.Now()
	m.mu.Unlock()
}

// Stats returns degradation statistics
func (m *DegradationManager) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"level":             m.level.String(),
		"manual_override":   m.manualOverride,
		"error_rate":        m.ErrorRate(),
		"total_requests":    atomic.LoadInt64(&m.totalRequests),
		"failed_requests":   atomic.LoadInt64(&m.failedRequests),
		"last_level_change": m.lastLevelChange,
		"window_start":      m.windowStart,
	}
}

// IsFeatureEnabled checks if a feature should be enabled at the current degradation level
func (m *DegradationManager) IsFeatureEnabled(feature string, minLevel DegradationLevel) bool {
	return m.Level() <= minLevel
}

// FallbackResponse represents a static fallback response
type FallbackResponse struct {
	StatusCode int
	Body       interface{}
	Headers    map[string]string
}

// FallbackRegistry manages fallback responses
type FallbackRegistry struct {
	mu        sync.RWMutex
	fallbacks map[string]FallbackResponse
}

// NewFallbackRegistry creates a new fallback registry
func NewFallbackRegistry() *FallbackRegistry {
	return &FallbackRegistry{
		fallbacks: make(map[string]FallbackResponse),
	}
}

// Register registers a fallback response for a path
func (r *FallbackRegistry) Register(path string, response FallbackResponse) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fallbacks[path] = response
}

// Get returns the fallback response for a path
func (r *FallbackRegistry) Get(path string) (FallbackResponse, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	resp, ok := r.fallbacks[path]
	return resp, ok
}

// CachedResponse represents a cached response for degradation
type CachedResponse struct {
	Body      interface{}
	CachedAt  time.Time
	ExpiresAt time.Time
}

// ResponseCache provides caching for degradation scenarios
type ResponseCache struct {
	mu    sync.RWMutex
	cache map[string]CachedResponse
	ttl   time.Duration
}

// NewResponseCache creates a new response cache
func NewResponseCache(ttl time.Duration) *ResponseCache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &ResponseCache{
		cache: make(map[string]CachedResponse),
		ttl:   ttl,
	}
}

// Set stores a response in the cache
func (c *ResponseCache) Set(key string, body interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	c.cache[key] = CachedResponse{
		Body:      body,
		CachedAt:  now,
		ExpiresAt: now.Add(c.ttl),
	}
}

// Get retrieves a response from the cache
func (c *ResponseCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	resp, ok := c.cache[key]
	if !ok || time.Now().After(resp.ExpiresAt) {
		return nil, false
	}
	return resp.Body, true
}

// Clear removes all entries from the cache
func (c *ResponseCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]CachedResponse)
}

// Cleanup removes expired entries from the cache
func (c *ResponseCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, resp := range c.cache {
		if now.After(resp.ExpiresAt) {
			delete(c.cache, key)
		}
	}
}

// HealthBasedRouter routes requests based on backend health
type HealthBasedRouter struct {
	mu       sync.RWMutex
	backends map[string]*BackendHealth
}

// BackendHealth tracks the health of a backend
type BackendHealth struct {
	Name        string
	Healthy     bool
	LastCheck   time.Time
	FailCount   int
	SuccessCount int
	Weight      int
}

// NewHealthBasedRouter creates a new health-based router
func NewHealthBasedRouter() *HealthBasedRouter {
	return &HealthBasedRouter{
		backends: make(map[string]*BackendHealth),
	}
}

// RegisterBackend registers a backend for health tracking
func (r *HealthBasedRouter) RegisterBackend(name string, weight int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.backends[name] = &BackendHealth{
		Name:    name,
		Healthy: true,
		Weight:  weight,
	}
}

// RecordResult records a request result for a backend
func (r *HealthBasedRouter) RecordResult(name string, success bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	backend, ok := r.backends[name]
	if !ok {
		return
	}

	backend.LastCheck = time.Now()
	if success {
		backend.SuccessCount++
		backend.FailCount = 0
		backend.Healthy = true
	} else {
		backend.FailCount++
		if backend.FailCount >= 3 {
			backend.Healthy = false
		}
	}
}

// GetHealthyBackend returns a healthy backend
func (r *HealthBasedRouter) GetHealthyBackend() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var best *BackendHealth
	for _, backend := range r.backends {
		if backend.Healthy {
			if best == nil || backend.Weight > best.Weight {
				best = backend
			}
		}
	}

	if best != nil {
		return best.Name
	}
	return ""
}

// Stats returns health statistics for all backends
func (r *HealthBasedRouter) Stats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := make(map[string]interface{})
	for name, backend := range r.backends {
		stats[name] = map[string]interface{}{
			"healthy":       backend.Healthy,
			"last_check":    backend.LastCheck,
			"fail_count":    backend.FailCount,
			"success_count": backend.SuccessCount,
			"weight":        backend.Weight,
		}
	}
	return stats
}

// DegradationMiddleware provides middleware for graceful degradation
type DegradationMiddleware struct {
	manager   *DegradationManager
	fallbacks *FallbackRegistry
	cache     *ResponseCache
}

// NewDegradationMiddleware creates a new degradation middleware
func NewDegradationMiddleware(manager *DegradationManager) *DegradationMiddleware {
	return &DegradationMiddleware{
		manager:   manager,
		fallbacks: NewFallbackRegistry(),
		cache:     NewResponseCache(5 * time.Minute),
	}
}

// ExecuteWithDegradation executes a function with degradation support
func (m *DegradationMiddleware) ExecuteWithDegradation(ctx context.Context, key string, fn func() (interface{}, error)) (interface{}, error) {
	level := m.manager.Level()

	// In emergency mode, try fallback first
	if level == LevelEmergency {
		if fallback, ok := m.fallbacks.Get(key); ok {
			return fallback.Body, nil
		}
		if cached, ok := m.cache.Get(key); ok {
			return cached, nil
		}
	}

	// Execute the function
	result, err := fn()

	// Record the result
	m.manager.RecordRequest(err == nil)

	if err != nil {
		// Try cache on error
		if cached, ok := m.cache.Get(key); ok {
			return cached, nil
		}
		// Try fallback on error
		if fallback, ok := m.fallbacks.Get(key); ok {
			return fallback.Body, nil
		}
		return nil, err
	}

	// Cache successful result
	m.cache.Set(key, result)

	return result, nil
}

// RegisterFallback registers a fallback response
func (m *DegradationMiddleware) RegisterFallback(path string, response FallbackResponse) {
	m.fallbacks.Register(path, response)
}
