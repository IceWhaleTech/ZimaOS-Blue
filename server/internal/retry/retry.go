// Package retry provides retry logic with exponential backoff for external API calls.
package retry

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"net/http"
	"time"
)

// Config holds retry configuration.
type Config struct {
	// MaxAttempts is the maximum number of attempts (including the first one).
	MaxAttempts int
	// InitialDelay is the initial delay before the first retry.
	InitialDelay time.Duration
	// MaxDelay is the maximum delay between retries.
	MaxDelay time.Duration
	// Multiplier is the factor by which the delay increases after each retry.
	Multiplier float64
	// Jitter adds randomness to the delay (0.0 to 1.0).
	Jitter float64
}

// DefaultConfig returns a default retry configuration.
func DefaultConfig() Config {
	return Config{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.1,
	}
}

// RetryableError indicates an error that can be retried.
type RetryableError struct {
	Err        error
	StatusCode int
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// IsRetryable checks if an error should be retried.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for RetryableError
	var retryableErr *RetryableError
	if errors.As(err, &retryableErr) {
		return true
	}

	// Check for context errors (not retryable)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Default: retry on network errors
	return true
}

// IsRetryableStatusCode checks if an HTTP status code should be retried.
func IsRetryableStatusCode(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests, // 429
		http.StatusInternalServerError,     // 500
		http.StatusBadGateway,              // 502
		http.StatusServiceUnavailable,      // 503
		http.StatusGatewayTimeout:          // 504
		return true
	default:
		return false
	}
}

// Func is the function type that can be retried.
type Func func(ctx context.Context) error

// Do executes the function with retry logic.
func Do(ctx context.Context, cfg Config, fn Func) error {
	var lastErr error

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		// Check context before attempting
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Execute the function
		err := fn(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if we should retry
		if !IsRetryable(err) {
			return err
		}

		// Don't sleep after the last attempt
		if attempt == cfg.MaxAttempts {
			break
		}

		// Calculate delay with exponential backoff
		delay := calculateDelay(cfg, attempt)

		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return lastErr
}

// DoWithResult executes a function that returns a result with retry logic.
func DoWithResult[T any](ctx context.Context, cfg Config, fn func(ctx context.Context) (T, error)) (T, error) {
	var result T
	var lastErr error

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		// Check context before attempting
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		// Execute the function
		res, err := fn(ctx)
		if err == nil {
			return res, nil
		}

		lastErr = err

		// Check if we should retry
		if !IsRetryable(err) {
			return result, err
		}

		// Don't sleep after the last attempt
		if attempt == cfg.MaxAttempts {
			break
		}

		// Calculate delay with exponential backoff
		delay := calculateDelay(cfg, attempt)

		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(delay):
		}
	}

	return result, lastErr
}

// calculateDelay calculates the delay for a given attempt.
func calculateDelay(cfg Config, attempt int) time.Duration {
	// Exponential backoff: initialDelay * multiplier^(attempt-1)
	delay := float64(cfg.InitialDelay) * math.Pow(cfg.Multiplier, float64(attempt-1))

	// Apply jitter
	if cfg.Jitter > 0 {
		jitterRange := delay * cfg.Jitter
		delay = delay - jitterRange + (rand.Float64() * 2 * jitterRange)
	}

	// Cap at max delay
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}

	return time.Duration(delay)
}

// HTTPClient wraps an http.Client with retry logic.
type HTTPClient struct {
	client *http.Client
	config Config
}

// NewHTTPClient creates a new HTTP client with retry support.
func NewHTTPClient(client *http.Client, cfg Config) *HTTPClient {
	if client == nil {
		client = http.DefaultClient
	}
	return &HTTPClient{
		client: client,
		config: cfg,
	}
}

// Do executes an HTTP request with retry logic.
func (c *HTTPClient) Do(req *http.Request) (*http.Response, error) {
	return DoWithResult(req.Context(), c.config, func(ctx context.Context) (*http.Response, error) {
		// Clone the request for retry
		reqClone := req.Clone(ctx)

		resp, err := c.client.Do(reqClone)
		if err != nil {
			return nil, err
		}

		// Check if status code is retryable
		if IsRetryableStatusCode(resp.StatusCode) {
			resp.Body.Close()
			return nil, &RetryableError{
				Err:        errors.New("retryable HTTP status"),
				StatusCode: resp.StatusCode,
			}
		}

		return resp, nil
	})
}
