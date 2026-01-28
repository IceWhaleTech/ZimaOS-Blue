package claudecode

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// ErrorCategory represents the category of an error for retry decisions.
type ErrorCategory int

const (
	// CategoryRetryable indicates the error is transient and can be retried.
	CategoryRetryable ErrorCategory = iota
	// CategoryNonRetryable indicates the error is permanent and should not be retried.
	CategoryNonRetryable
	// CategoryRateLimited indicates the error is due to rate limiting.
	CategoryRateLimited
	// CategoryTimeout indicates the error is due to timeout.
	CategoryTimeout
)

// String returns the string representation of the error category.
func (c ErrorCategory) String() string {
	switch c {
	case CategoryRetryable:
		return "retryable"
	case CategoryNonRetryable:
		return "non_retryable"
	case CategoryRateLimited:
		return "rate_limited"
	case CategoryTimeout:
		return "timeout"
	default:
		return "unknown"
	}
}

// ClassifyError categorizes an error for retry decisions.
func ClassifyError(err error) ErrorCategory {
	if err == nil {
		return CategoryNonRetryable
	}

	// Check for timeout errors
	var timeoutErr ErrTimeout
	if errors.As(err, &timeoutErr) {
		return CategoryTimeout
	}

	// Check for context errors
	if errors.Is(err, context.DeadlineExceeded) {
		return CategoryTimeout
	}
	if errors.Is(err, context.Canceled) {
		return CategoryNonRetryable
	}

	// Check for parse errors (non-retryable)
	var parseErr ErrParseOutput
	if errors.As(err, &parseErr) {
		return CategoryNonRetryable
	}

	// Check for config errors (non-retryable)
	var configErr ErrInvalidConfig
	if errors.As(err, &configErr) {
		return CategoryNonRetryable
	}

	// Check for CLI execution errors
	var cliErr ErrCliExecution
	if errors.As(err, &cliErr) {
		return classifyCliError(cliErr)
	}

	// Default to retryable for unknown errors
	return CategoryRetryable
}

// classifyCliError categorizes CLI execution errors.
func classifyCliError(err ErrCliExecution) ErrorCategory {
	stderr := strings.ToLower(err.Stderr)

	// Rate limiting indicators
	if strings.Contains(stderr, "rate limit") ||
		strings.Contains(stderr, "too many requests") ||
		strings.Contains(stderr, "429") ||
		err.ExitCode == 429 {
		return CategoryRateLimited
	}

	// Authentication errors (non-retryable)
	if strings.Contains(stderr, "authentication") ||
		strings.Contains(stderr, "unauthorized") ||
		strings.Contains(stderr, "invalid api key") ||
		strings.Contains(stderr, "401") ||
		err.ExitCode == 401 {
		return CategoryNonRetryable
	}

	// Permission errors (non-retryable)
	if strings.Contains(stderr, "permission denied") ||
		strings.Contains(stderr, "forbidden") ||
		strings.Contains(stderr, "403") ||
		err.ExitCode == 403 {
		return CategoryNonRetryable
	}

	// Network errors (retryable)
	if strings.Contains(stderr, "connection") ||
		strings.Contains(stderr, "network") ||
		strings.Contains(stderr, "dns") ||
		strings.Contains(stderr, "timeout") ||
		strings.Contains(stderr, "econnrefused") ||
		strings.Contains(stderr, "econnreset") {
		return CategoryRetryable
	}

	// Server errors (retryable)
	if err.ExitCode >= 500 && err.ExitCode < 600 {
		return CategoryRetryable
	}

	// Exit codes 1-2 are often transient
	if err.ExitCode == 1 || err.ExitCode == 2 {
		return CategoryRetryable
	}

	// Default to non-retryable for other CLI errors
	return CategoryNonRetryable
}

// RetryConfig contains configuration for retry behavior.
type RetryConfig struct {
	// Enabled indicates whether retry is enabled.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int `json:"max_retries" yaml:"max_retries"`
	// InitialDelay is the initial delay before the first retry.
	InitialDelay time.Duration `json:"initial_delay" yaml:"initial_delay"`
	// MaxDelay is the maximum delay between retries.
	MaxDelay time.Duration `json:"max_delay" yaml:"max_delay"`
	// Multiplier is the factor by which the delay increases.
	Multiplier float64 `json:"multiplier" yaml:"multiplier"`
	// Jitter is the random factor (0-1) added to delays.
	Jitter float64 `json:"jitter" yaml:"jitter"`
}

// DefaultRetryConfig returns the default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		Enabled:      true,
		MaxRetries:   3,
		InitialDelay: 500 * time.Millisecond,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.1,
	}
}

// RetryStrategy defines how to calculate delay between retries.
type RetryStrategy int

const (
	// StrategyExponential uses exponential backoff.
	StrategyExponential RetryStrategy = iota
	// StrategyLinear uses linear backoff.
	StrategyLinear
	// StrategyFixed uses fixed delay.
	StrategyFixed
)

// CalculateDelay calculates the delay for a given retry attempt.
func (c *RetryConfig) CalculateDelay(attempt int, strategy RetryStrategy) time.Duration {
	var delay time.Duration

	switch strategy {
	case StrategyExponential:
		delay = time.Duration(float64(c.InitialDelay) * math.Pow(c.Multiplier, float64(attempt)))
	case StrategyLinear:
		delay = c.InitialDelay * time.Duration(attempt+1)
	case StrategyFixed:
		delay = c.InitialDelay
	}

	// Apply max delay cap
	if delay > c.MaxDelay {
		delay = c.MaxDelay
	}

	// Apply jitter
	if c.Jitter > 0 {
		jitterRange := float64(delay) * c.Jitter
		jitter := (rand.Float64()*2 - 1) * jitterRange
		delay = time.Duration(float64(delay) + jitter)
	}

	return delay
}

// GetStrategyForCategory returns the appropriate retry strategy for an error category.
func GetStrategyForCategory(category ErrorCategory) RetryStrategy {
	switch category {
	case CategoryRateLimited:
		return StrategyFixed
	case CategoryTimeout:
		return StrategyLinear
	default:
		return StrategyExponential
	}
}

// GetMaxRetriesForCategory returns the maximum retries for an error category.
func GetMaxRetriesForCategory(category ErrorCategory, defaultMax int) int {
	switch category {
	case CategoryRateLimited:
		return 5 // More retries for rate limiting
	case CategoryTimeout:
		return 2 // Fewer retries for timeout
	case CategoryNonRetryable:
		return 0
	default:
		return defaultMax
	}
}

// Retryer handles retry logic for CLI operations.
type Retryer struct {
	config  RetryConfig
	metrics *ReliabilityMetrics
	mu      sync.RWMutex
}

// NewRetryer creates a new Retryer with the given configuration.
func NewRetryer(config RetryConfig) *Retryer {
	return &Retryer{
		config:  config,
		metrics: &ReliabilityMetrics{},
	}
}

// RetryFunc is a function that can be retried.
type RetryFunc func(ctx context.Context) error

// Do executes the function with retry logic.
func (r *Retryer) Do(ctx context.Context, fn RetryFunc) error {
	if !r.config.Enabled {
		return fn(ctx)
	}

	var lastErr error
	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		// Execute the function
		err := fn(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		// Classify the error
		category := ClassifyError(err)

		// Check if we should retry
		maxRetries := GetMaxRetriesForCategory(category, r.config.MaxRetries)
		if category == CategoryNonRetryable || attempt >= maxRetries {
			return err
		}

		// Record retry attempt
		r.recordRetry()

		// Calculate delay
		strategy := GetStrategyForCategory(category)
		delay := r.config.CalculateDelay(attempt, strategy)

		// Wait before retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return lastErr
}

// recordRetry records a retry attempt in metrics.
func (r *Retryer) recordRetry() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metrics.RetryCount++
}

// GetMetrics returns the current retry metrics.
func (r *Retryer) GetMetrics() ReliabilityMetrics {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return *r.metrics
}

// ResetMetrics resets the retry metrics.
func (r *Retryer) ResetMetrics() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metrics = &ReliabilityMetrics{}
}

// ReliabilityMetrics contains metrics for reliability features.
type ReliabilityMetrics struct {
	// RetryCount is the total number of retry attempts.
	RetryCount int64 `json:"retry_count"`
	// RetrySuccessCount is the number of successful retries.
	RetrySuccessCount int64 `json:"retry_success_count"`
	// CircuitBreakerOpenCount is the number of times the circuit breaker opened.
	CircuitBreakerOpenCount int64 `json:"circuit_breaker_open_count"`
	// CacheHits is the number of cache hits.
	CacheHits int64 `json:"cache_hits"`
	// CacheMisses is the number of cache misses.
	CacheMisses int64 `json:"cache_misses"`
}

// CircuitBreakerConfig contains configuration for circuit breaker.
type CircuitBreakerConfig struct {
	// Enabled indicates whether circuit breaker is enabled.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// FailureThreshold is the number of failures before opening the circuit.
	FailureThreshold int `json:"failure_threshold" yaml:"failure_threshold"`
	// SuccessThreshold is the number of successes before closing the circuit.
	SuccessThreshold int `json:"success_threshold" yaml:"success_threshold"`
	// Timeout is the duration the circuit stays open before half-opening.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
	// HalfOpenMaxCalls is the maximum calls allowed in half-open state.
	HalfOpenMaxCalls int `json:"half_open_max_calls" yaml:"half_open_max_calls"`
}

// DefaultCircuitBreakerConfig returns the default circuit breaker configuration.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		Enabled:          false,
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
		HalfOpenMaxCalls: 3,
	}
}

// HealthCheckConfig contains configuration for health checks.
type HealthCheckConfig struct {
	// Enabled indicates whether health checks are enabled.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// Interval is the interval between health checks.
	Interval time.Duration `json:"interval" yaml:"interval"`
	// Timeout is the timeout for each health check.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

// DefaultHealthCheckConfig returns the default health check configuration.
func DefaultHealthCheckConfig() HealthCheckConfig {
	return HealthCheckConfig{
		Enabled:  false,
		Interval: 30 * time.Second,
		Timeout:  5 * time.Second,
	}
}

// ReliabilityConfig contains all reliability-related configuration.
type ReliabilityConfig struct {
	// Retry contains retry configuration.
	Retry RetryConfig `json:"retry" yaml:"retry"`
	// Cache contains cache configuration.
	Cache CacheConfig `json:"cache" yaml:"cache"`
	// CircuitBreaker contains circuit breaker configuration.
	CircuitBreaker CircuitBreakerConfig `json:"circuit_breaker" yaml:"circuit_breaker"`
	// HealthCheck contains health check configuration.
	HealthCheck HealthCheckConfig `json:"health_check" yaml:"health_check"`
}

// DefaultReliabilityConfig returns the default reliability configuration.
func DefaultReliabilityConfig() ReliabilityConfig {
	return ReliabilityConfig{
		Retry:          DefaultRetryConfig(),
		Cache:          DefaultCacheConfig(),
		CircuitBreaker: DefaultCircuitBreakerConfig(),
		HealthCheck:    DefaultHealthCheckConfig(),
	}
}
