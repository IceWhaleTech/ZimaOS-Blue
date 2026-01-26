package resilience

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// ErrAllProvidersFailed is returned when all providers in the fallback chain fail
var ErrAllProvidersFailed = errors.New("all LLM providers failed")

// LLMProviderStatus represents the status of an LLM provider
type LLMProviderStatus int

const (
	// ProviderHealthy indicates the provider is working normally
	ProviderHealthy LLMProviderStatus = iota
	// ProviderDegraded indicates the provider is experiencing issues
	ProviderDegraded
	// ProviderUnhealthy indicates the provider is not working
	ProviderUnhealthy
)

func (s LLMProviderStatus) String() string {
	switch s {
	case ProviderHealthy:
		return "healthy"
	case ProviderDegraded:
		return "degraded"
	case ProviderUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}

// LLMProvider represents an LLM provider interface
type LLMProvider interface {
	Name() string
	Execute(ctx context.Context, request interface{}) (interface{}, error)
	HealthCheck(ctx context.Context) error
}

// LLMProviderHealth tracks the health of an LLM provider
type LLMProviderHealth struct {
	Provider     LLMProvider
	Status       LLMProviderStatus
	Priority     int
	Weight       int
	LastSuccess  time.Time
	LastFailure  time.Time
	SuccessCount int64
	FailureCount int64
	Latency      time.Duration
	CircuitBreaker *CircuitBreaker
}

// LLMFallbackConfig holds configuration for LLM fallback chain
type LLMFallbackConfig struct {
	// MaxRetries is the maximum number of retries per provider
	MaxRetries int

	// RetryDelay is the delay between retries
	RetryDelay time.Duration

	// HealthCheckInterval is how often to check provider health
	HealthCheckInterval time.Duration

	// UnhealthyThreshold is the number of failures before marking unhealthy
	UnhealthyThreshold int

	// RecoveryThreshold is the number of successes before marking healthy
	RecoveryThreshold int

	// CircuitBreakerConfig is the config for per-provider circuit breakers
	CircuitBreakerConfig CircuitBreakerConfig

	// OnProviderSwitch is called when switching to a different provider
	OnProviderSwitch func(from, to string)

	// OnAllFailed is called when all providers fail
	OnAllFailed func(err error)
}

// LLMFallbackChain manages LLM provider fallback
type LLMFallbackChain struct {
	config    LLMFallbackConfig
	mu        sync.RWMutex
	providers []*LLMProviderHealth
	primary   int // Index of primary provider
}

// NewLLMFallbackChain creates a new LLM fallback chain
func NewLLMFallbackChain(cfg LLMFallbackConfig) *LLMFallbackChain {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 2
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = 1 * time.Second
	}
	if cfg.HealthCheckInterval <= 0 {
		cfg.HealthCheckInterval = 30 * time.Second
	}
	if cfg.UnhealthyThreshold <= 0 {
		cfg.UnhealthyThreshold = 3
	}
	if cfg.RecoveryThreshold <= 0 {
		cfg.RecoveryThreshold = 2
	}

	return &LLMFallbackChain{
		config:    cfg,
		providers: make([]*LLMProviderHealth, 0),
	}
}

// AddProvider adds a provider to the fallback chain
func (c *LLMFallbackChain) AddProvider(provider LLMProvider, priority int, weight int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cbConfig := c.config.CircuitBreakerConfig
	cbConfig.Name = provider.Name()

	health := &LLMProviderHealth{
		Provider:       provider,
		Status:         ProviderHealthy,
		Priority:       priority,
		Weight:         weight,
		CircuitBreaker: NewCircuitBreaker(cbConfig),
	}

	c.providers = append(c.providers, health)
	c.sortProviders()
}

// RemoveProvider removes a provider from the fallback chain
func (c *LLMFallbackChain) RemoveProvider(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, p := range c.providers {
		if p.Provider.Name() == name {
			c.providers = append(c.providers[:i], c.providers[i+1:]...)
			break
		}
	}
}

// sortProviders sorts providers by priority and weight
func (c *LLMFallbackChain) sortProviders() {
	// Simple bubble sort for small number of providers
	for i := 0; i < len(c.providers)-1; i++ {
		for j := 0; j < len(c.providers)-i-1; j++ {
			if c.providers[j].Priority < c.providers[j+1].Priority ||
				(c.providers[j].Priority == c.providers[j+1].Priority &&
					c.providers[j].Weight < c.providers[j+1].Weight) {
				c.providers[j], c.providers[j+1] = c.providers[j+1], c.providers[j]
			}
		}
	}
}

// Execute executes a request with fallback support
func (c *LLMFallbackChain) Execute(ctx context.Context, request interface{}) (interface{}, error) {
	c.mu.RLock()
	providers := make([]*LLMProviderHealth, len(c.providers))
	copy(providers, c.providers)
	c.mu.RUnlock()

	if len(providers) == 0 {
		return nil, ErrAllProvidersFailed
	}

	var lastErr error
	var lastProvider string

	for _, health := range providers {
		// Skip unhealthy providers
		if health.Status == ProviderUnhealthy {
			continue
		}

		// Check circuit breaker
		if health.CircuitBreaker.State() == StateOpen {
			continue
		}

		// Try this provider with retries
		for retry := 0; retry <= c.config.MaxRetries; retry++ {
			if retry > 0 {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(c.config.RetryDelay):
				}
			}

			start := time.Now()
			result, err := c.executeWithCircuitBreaker(ctx, health, request)
			latency := time.Since(start)

			if err == nil {
				c.recordSuccess(health, latency)

				// Notify provider switch
				if lastProvider != "" && lastProvider != health.Provider.Name() {
					if c.config.OnProviderSwitch != nil {
						go c.config.OnProviderSwitch(lastProvider, health.Provider.Name())
					}
				}

				return result, nil
			}

			lastErr = err
			lastProvider = health.Provider.Name()
			c.recordFailure(health)
		}
	}

	if c.config.OnAllFailed != nil {
		go c.config.OnAllFailed(lastErr)
	}

	return nil, ErrAllProvidersFailed
}

func (c *LLMFallbackChain) executeWithCircuitBreaker(ctx context.Context, health *LLMProviderHealth, request interface{}) (interface{}, error) {
	var result interface{}
	var execErr error

	err := health.CircuitBreaker.Execute(func() error {
		var err error
		result, err = health.Provider.Execute(ctx, request)
		execErr = err
		return err
	})

	if err != nil {
		if errors.Is(err, ErrCircuitOpen) {
			return nil, err
		}
		return nil, execErr
	}

	return result, nil
}

func (c *LLMFallbackChain) recordSuccess(health *LLMProviderHealth, latency time.Duration) {
	atomic.AddInt64(&health.SuccessCount, 1)
	health.LastSuccess = time.Now()
	health.Latency = latency

	c.mu.Lock()
	defer c.mu.Unlock()

	// Update status based on success
	if health.Status == ProviderDegraded {
		successCount := atomic.LoadInt64(&health.SuccessCount)
		if successCount >= int64(c.config.RecoveryThreshold) {
			health.Status = ProviderHealthy
		}
	}
}

func (c *LLMFallbackChain) recordFailure(health *LLMProviderHealth) {
	atomic.AddInt64(&health.FailureCount, 1)
	health.LastFailure = time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Update status based on failures
	failureCount := atomic.LoadInt64(&health.FailureCount)
	if failureCount >= int64(c.config.UnhealthyThreshold) {
		health.Status = ProviderUnhealthy
	} else if failureCount > 0 {
		health.Status = ProviderDegraded
	}
}

// HealthCheck performs health checks on all providers
func (c *LLMFallbackChain) HealthCheck(ctx context.Context) {
	c.mu.RLock()
	providers := make([]*LLMProviderHealth, len(c.providers))
	copy(providers, c.providers)
	c.mu.RUnlock()

	for _, health := range providers {
		go func(h *LLMProviderHealth) {
			err := h.Provider.HealthCheck(ctx)
			if err == nil {
				c.recordSuccess(h, 0)
			} else {
				c.recordFailure(h)
			}
		}(health)
	}
}

// SetProviderStatus manually sets a provider's status
func (c *LLMFallbackChain) SetProviderStatus(name string, status LLMProviderStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, health := range c.providers {
		if health.Provider.Name() == name {
			health.Status = status
			break
		}
	}
}

// GetProviderStatus returns a provider's current status
func (c *LLMFallbackChain) GetProviderStatus(name string) (LLMProviderStatus, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, health := range c.providers {
		if health.Provider.Name() == name {
			return health.Status, true
		}
	}
	return ProviderUnhealthy, false
}

// Stats returns statistics for all providers
func (c *LLMFallbackChain) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := make(map[string]interface{})
	for _, health := range c.providers {
		stats[health.Provider.Name()] = map[string]interface{}{
			"status":         health.Status.String(),
			"priority":       health.Priority,
			"weight":         health.Weight,
			"success_count":  atomic.LoadInt64(&health.SuccessCount),
			"failure_count":  atomic.LoadInt64(&health.FailureCount),
			"last_success":   health.LastSuccess,
			"last_failure":   health.LastFailure,
			"latency_ms":     health.Latency.Milliseconds(),
			"circuit_breaker": health.CircuitBreaker.Stats(),
		}
	}
	return stats
}

// Reset resets all provider statistics and circuit breakers
func (c *LLMFallbackChain) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, health := range c.providers {
		health.Status = ProviderHealthy
		atomic.StoreInt64(&health.SuccessCount, 0)
		atomic.StoreInt64(&health.FailureCount, 0)
		health.CircuitBreaker.Reset()
	}
}

// GetActiveProvider returns the currently active (primary) provider name
func (c *LLMFallbackChain) GetActiveProvider() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, health := range c.providers {
		if health.Status != ProviderUnhealthy && health.CircuitBreaker.State() != StateOpen {
			return health.Provider.Name()
		}
	}
	return ""
}

// ProviderCount returns the number of registered providers
func (c *LLMFallbackChain) ProviderCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.providers)
}

// HealthyProviderCount returns the number of healthy providers
func (c *LLMFallbackChain) HealthyProviderCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	count := 0
	for _, health := range c.providers {
		if health.Status == ProviderHealthy {
			count++
		}
	}
	return count
}
