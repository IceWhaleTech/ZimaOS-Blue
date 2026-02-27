package proxy

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/resilience"
)

// FailoverHandler handles request failover
type FailoverHandler struct {
	config   *FailoverConfig
	router   *Router
	breakers map[string]*resilience.CircuitBreaker
	mu       sync.RWMutex
}

// NewFailoverHandler creates a new failover handler
func NewFailoverHandler(config *FailoverConfig, router *Router) *FailoverHandler {
	return &FailoverHandler{
		config:   config,
		router:   router,
		breakers: make(map[string]*resilience.CircuitBreaker),
	}
}

// Config returns the failover configuration pointer.
func (fh *FailoverHandler) Config() *FailoverConfig {
	return fh.config
}

func (fh *FailoverHandler) isSingleProviderMode() bool {
	if fh.router == nil {
		return false
	}
	providers := fh.router.GetAllProviders()
	enabled := 0
	for _, p := range providers {
		if p != nil && p.Config != nil && p.Config.Enabled {
			enabled++
			if enabled > 1 {
				return false
			}
		}
	}
	return enabled == 1
}

// getBreaker returns or creates a circuit breaker for a provider
func (fh *FailoverHandler) getBreaker(name string) *resilience.CircuitBreaker {
	fh.mu.RLock()
	breaker, ok := fh.breakers[name]
	fh.mu.RUnlock()

	if ok {
		return breaker
	}

	fh.mu.Lock()
	defer fh.mu.Unlock()

	// Double-check after acquiring write lock
	if breaker, ok = fh.breakers[name]; ok {
		return breaker
	}

	breaker = resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
		Name:                name,
		MaxFailures:         fh.config.FailureThreshold,
		Timeout:             fh.config.RecoveryTimeout,
		MaxHalfOpenRequests: 1,
	})
	fh.breakers[name] = breaker
	return breaker
}

// Execute executes request with failover support
func (fh *FailoverHandler) Execute(
	ctx context.Context,
	provider *Provider,
	fn func(*Provider) (*http.Response, error),
) (*http.Response, error) {
	if !fh.config.Enabled {
		return fn(provider)
	}

	// Single-provider mode: don't short-circuit or isolate the only provider.
	// Keep retrying the same provider to maximize recovery chance.
	if fh.isSingleProviderMode() {
		var lastErr error
		var lastResp *http.Response
		for attempt := 0; attempt <= fh.config.MaxRetries; attempt++ {
			if attempt > 0 {
				delay := fh.config.RetryDelay * time.Duration(1<<(attempt-1))
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(delay):
				}
			}

			resp, err := fn(provider)
			if err == nil && resp.StatusCode < 500 {
				return resp, nil
			}
			lastErr = err
			lastResp = resp
			if err == nil {
				lastErr = ErrUpstreamError
			}
		}
		if lastResp != nil {
			return lastResp, lastErr
		}
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, ErrUpstreamError
	}

	// Get circuit breaker for provider
	breaker := fh.getBreaker(provider.Config.Name)

	// Check circuit breaker
	if fh.config.CircuitBreaker && breaker.State() == resilience.StateOpen {
		// Try next provider
		return fh.tryNextProvider(ctx, provider, fn)
	}

	// Execute with retries
	var lastErr error
	var lastResp *http.Response

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
			if fh.config.CircuitBreaker {
				breaker.Execute(func() error { return nil }) // Record success
			}
			return resp, nil
		}

		lastErr = err
		lastResp = resp
		if err == nil {
			lastErr = ErrUpstreamError
		}
	}

	// All retries failed
	if fh.config.CircuitBreaker {
		breaker.Execute(func() error { return lastErr }) // Record failure
	}

	// Try next provider
	nextResp, nextErr := fh.tryNextProvider(ctx, provider, fn)
	if nextErr == nil {
		return nextResp, nil
	}

	// Return original error if all providers failed
	if lastResp != nil {
		return lastResp, lastErr
	}
	return nil, ErrAllProvidersFailed
}

// tryNextProvider attempts to use the next available provider
func (fh *FailoverHandler) tryNextProvider(
	ctx context.Context,
	failed *Provider,
	fn func(*Provider) (*http.Response, error),
) (*http.Response, error) {
	providers := fh.router.GetAvailableProviders()

	for _, p := range providers {
		if p.Config.Name == failed.Config.Name {
			continue
		}

		breaker := fh.getBreaker(p.Config.Name)
		if fh.config.CircuitBreaker && breaker.State() == resilience.StateOpen {
			continue
		}

		resp, err := fn(p)
		if err == nil && resp.StatusCode < 500 {
			if fh.config.CircuitBreaker {
				breaker.Execute(func() error { return nil }) // Record success
			}
			return resp, nil
		}

		if fh.config.CircuitBreaker {
			breaker.Execute(func() error { return err }) // Record failure
		}
	}

	return nil, ErrAllProvidersFailed
}

// GetBreakerState returns the circuit breaker state for a provider
func (fh *FailoverHandler) GetBreakerState(name string) string {
	fh.mu.RLock()
	breaker, ok := fh.breakers[name]
	fh.mu.RUnlock()

	if !ok {
		return "closed"
	}

	return breaker.State().String()
}

// GetBreakerStats returns circuit breaker statistics
func (fh *FailoverHandler) GetBreakerStats() map[string]interface{} {
	fh.mu.RLock()
	defer fh.mu.RUnlock()

	stats := make(map[string]interface{})
	for name, breaker := range fh.breakers {
		stats[name] = breaker.Stats()
	}
	return stats
}

// ResetBreaker resets a circuit breaker
func (fh *FailoverHandler) ResetBreaker(name string) {
	fh.mu.RLock()
	breaker, ok := fh.breakers[name]
	fh.mu.RUnlock()

	if ok {
		breaker.Reset()
	}
}

// ResetAllBreakers resets all circuit breakers
func (fh *FailoverHandler) ResetAllBreakers() {
	fh.mu.RLock()
	defer fh.mu.RUnlock()

	for _, breaker := range fh.breakers {
		breaker.Reset()
	}
}

// BreakerSnapshot holds persisted circuit breaker state for a single provider.
type BreakerSnapshot struct {
	Name            string
	State           string
	Failures        int
	Successes       int
	LastFailureTime time.Time
	LastStateChange time.Time
}

// SnapshotBreakers returns a snapshot of all circuit breaker states for persistence.
func (fh *FailoverHandler) SnapshotBreakers() []BreakerSnapshot {
	fh.mu.RLock()
	defer fh.mu.RUnlock()

	snaps := make([]BreakerSnapshot, 0, len(fh.breakers))
	for name, cb := range fh.breakers {
		stats := cb.Stats()
		snap := BreakerSnapshot{Name: name}
		if s, ok := stats["state"].(string); ok {
			snap.State = s
		}
		if f, ok := stats["failures"].(int); ok {
			snap.Failures = f
		}
		if s, ok := stats["successes"].(int); ok {
			snap.Successes = s
		}
		if t, ok := stats["last_failure"].(time.Time); ok {
			snap.LastFailureTime = t
		}
		if t, ok := stats["last_state_change"].(time.Time); ok {
			snap.LastStateChange = t
		}
		snaps = append(snaps, snap)
	}
	return snaps
}

// LoadBreakerState restores a circuit breaker's state from persisted data.
// Creates the breaker if it doesn't exist yet.
func (fh *FailoverHandler) LoadBreakerState(snap BreakerSnapshot) {
	breaker := fh.getBreaker(snap.Name)
	breaker.LoadState(
		resilience.ParseState(snap.State),
		snap.Failures,
		snap.Successes,
		snap.LastFailureTime,
		snap.LastStateChange,
	)
}
