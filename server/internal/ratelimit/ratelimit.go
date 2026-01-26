// Package ratelimit provides per-client rate limiting middleware.
package ratelimit

import (
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

// Config holds rate limiter configuration.
type Config struct {
	// Rate is the number of requests allowed per window.
	Rate int
	// Window is the time window for rate limiting.
	Window time.Duration
	// KeyFunc extracts the client identifier from the request.
	// If nil, uses the client IP address.
	KeyFunc func(c echo.Context) string
	// ExceededHandler is called when rate limit is exceeded.
	// If nil, returns 429 Too Many Requests.
	ExceededHandler echo.HandlerFunc
	// CleanupInterval is how often to clean up expired entries.
	CleanupInterval time.Duration
}

// DefaultConfig returns a default rate limiter configuration.
func DefaultConfig() Config {
	return Config{
		Rate:            100,
		Window:          time.Minute,
		CleanupInterval: 5 * time.Minute,
	}
}

// clientState tracks request count for a client.
type clientState struct {
	count     int
	windowEnd time.Time
}

// Limiter implements per-client rate limiting.
type Limiter struct {
	config  Config
	clients map[string]*clientState
	mu      sync.RWMutex
	stopCh  chan struct{}
}

// New creates a new rate limiter.
func New(cfg Config) *Limiter {
	if cfg.Rate <= 0 {
		cfg.Rate = 100
	}
	if cfg.Window <= 0 {
		cfg.Window = time.Minute
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = 5 * time.Minute
	}

	l := &Limiter{
		config:  cfg,
		clients: make(map[string]*clientState),
		stopCh:  make(chan struct{}),
	}

	// Start cleanup goroutine
	go l.cleanup()

	return l
}

// Stop stops the rate limiter cleanup goroutine.
func (l *Limiter) Stop() {
	close(l.stopCh)
}

// cleanup periodically removes expired client entries.
func (l *Limiter) cleanup() {
	ticker := time.NewTicker(l.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-l.stopCh:
			return
		case <-ticker.C:
			l.mu.Lock()
			now := time.Now()
			for key, state := range l.clients {
				if now.After(state.windowEnd) {
					delete(l.clients, key)
				}
			}
			l.mu.Unlock()
		}
	}
}

// Allow checks if a request is allowed for the given key.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	state, exists := l.clients[key]

	if !exists || now.After(state.windowEnd) {
		// New window
		l.clients[key] = &clientState{
			count:     1,
			windowEnd: now.Add(l.config.Window),
		}
		return true
	}

	if state.count >= l.config.Rate {
		return false
	}

	state.count++
	return true
}

// Remaining returns the number of remaining requests for the given key.
func (l *Limiter) Remaining(key string) int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	now := time.Now()
	state, exists := l.clients[key]

	if !exists || now.After(state.windowEnd) {
		return l.config.Rate
	}

	remaining := l.config.Rate - state.count
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Reset returns the time when the rate limit resets for the given key.
func (l *Limiter) Reset(key string) time.Time {
	l.mu.RLock()
	defer l.mu.RUnlock()

	state, exists := l.clients[key]
	if !exists {
		return time.Now().Add(l.config.Window)
	}
	return state.windowEnd
}

// Middleware returns an Echo middleware for rate limiting.
func (l *Limiter) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get client key
			var key string
			if l.config.KeyFunc != nil {
				key = l.config.KeyFunc(c)
			} else {
				key = c.RealIP()
			}

			// Check rate limit
			if !l.Allow(key) {
				// Set rate limit headers
				c.Response().Header().Set("X-RateLimit-Limit", itoa(l.config.Rate))
				c.Response().Header().Set("X-RateLimit-Remaining", "0")
				c.Response().Header().Set("X-RateLimit-Reset", itoa64(l.Reset(key).Unix()))
				c.Response().Header().Set("Retry-After", itoa(int(l.config.Window.Seconds())))

				if l.config.ExceededHandler != nil {
					return l.config.ExceededHandler(c)
				}

				return c.JSON(http.StatusTooManyRequests, map[string]interface{}{
					"error":       "rate limit exceeded",
					"retry_after": int(l.config.Window.Seconds()),
				})
			}

			// Set rate limit headers
			c.Response().Header().Set("X-RateLimit-Limit", itoa(l.config.Rate))
			c.Response().Header().Set("X-RateLimit-Remaining", itoa(l.Remaining(key)))
			c.Response().Header().Set("X-RateLimit-Reset", itoa64(l.Reset(key).Unix()))

			return next(c)
		}
	}
}

// MiddlewareWithConfig creates a rate limiting middleware with the given config.
func MiddlewareWithConfig(cfg Config) echo.MiddlewareFunc {
	limiter := New(cfg)
	return limiter.Middleware()
}

// itoa converts int to string without importing strconv.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	var neg bool
	if i < 0 {
		neg = true
		i = -i
	}

	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}

	if neg {
		pos--
		buf[pos] = '-'
	}

	return string(buf[pos:])
}

// itoa64 converts int64 to string.
func itoa64(i int64) string {
	if i == 0 {
		return "0"
	}

	var neg bool
	if i < 0 {
		neg = true
		i = -i
	}

	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}

	if neg {
		pos--
		buf[pos] = '-'
	}

	return string(buf[pos:])
}
