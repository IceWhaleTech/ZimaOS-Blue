package resilience

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// TimeoutConfig holds configuration for timeout middleware
type TimeoutConfig struct {
	// DefaultTimeout is the default timeout for all requests
	DefaultTimeout time.Duration

	// LLMTimeout is the timeout for LLM-related requests (typically longer)
	LLMTimeout time.Duration

	// UploadTimeout is the timeout for file upload requests
	UploadTimeout time.Duration

	// Skipper defines a function to skip middleware
	Skipper func(c echo.Context) bool

	// OnTimeout is called when a request times out
	OnTimeout func(c echo.Context, timeout time.Duration)

	// ErrorMessage is the message returned when timeout occurs
	ErrorMessage string

	// PathTimeouts allows setting custom timeouts for specific paths
	PathTimeouts map[string]time.Duration
}

// TimeoutMiddleware provides request timeout enforcement
type TimeoutMiddleware struct {
	config TimeoutConfig
}

// NewTimeoutMiddleware creates a new timeout middleware
func NewTimeoutMiddleware(cfg TimeoutConfig) *TimeoutMiddleware {
	if cfg.DefaultTimeout <= 0 {
		cfg.DefaultTimeout = 30 * time.Second
	}
	if cfg.LLMTimeout <= 0 {
		cfg.LLMTimeout = 120 * time.Second
	}
	if cfg.UploadTimeout <= 0 {
		cfg.UploadTimeout = 300 * time.Second
	}
	if cfg.ErrorMessage == "" {
		cfg.ErrorMessage = "Request timeout"
	}
	if cfg.PathTimeouts == nil {
		cfg.PathTimeouts = make(map[string]time.Duration)
	}

	return &TimeoutMiddleware{
		config: cfg,
	}
}

// Middleware returns the Echo middleware function
func (m *TimeoutMiddleware) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if m.config.Skipper != nil && m.config.Skipper(c) {
				return next(c)
			}

			timeout := m.getTimeout(c)

			ctx, cancel := context.WithTimeout(c.Request().Context(), timeout)
			defer cancel()

			c.SetRequest(c.Request().WithContext(ctx))

			// Channel to receive the result
			done := make(chan error, 1)

			go func() {
				done <- next(c)
			}()

			select {
			case err := <-done:
				return err
			case <-ctx.Done():
				if m.config.OnTimeout != nil {
					m.config.OnTimeout(c, timeout)
				}
				return echo.NewHTTPError(http.StatusGatewayTimeout, m.config.ErrorMessage)
			}
		}
	}
}

func (m *TimeoutMiddleware) getTimeout(c echo.Context) time.Duration {
	path := c.Path()

	// Check path-specific timeouts
	if timeout, ok := m.config.PathTimeouts[path]; ok {
		return timeout
	}

	// Check for LLM-related paths
	if isLLMPath(path) {
		return m.config.LLMTimeout
	}

	// Check for upload paths
	if isUploadPath(c) {
		return m.config.UploadTimeout
	}

	return m.config.DefaultTimeout
}

func isLLMPath(path string) bool {
	llmPaths := []string{
		"/api/v1/chat",
		"/api/v1/chat/stream",
		"/api/v1/completions",
		"/api/v1/agent",
	}

	for _, p := range llmPaths {
		if path == p || len(path) > len(p) && path[:len(p)+1] == p+"/" {
			return true
		}
	}
	return false
}

func isUploadPath(c echo.Context) bool {
	// Check content type for multipart form data
	contentType := c.Request().Header.Get(echo.HeaderContentType)
	return len(contentType) >= 19 && contentType[:19] == "multipart/form-data"
}

// SetPathTimeout sets a custom timeout for a specific path
func (m *TimeoutMiddleware) SetPathTimeout(path string, timeout time.Duration) {
	m.config.PathTimeouts[path] = timeout
}

// RemovePathTimeout removes a custom timeout for a specific path
func (m *TimeoutMiddleware) RemovePathTimeout(path string) {
	delete(m.config.PathTimeouts, path)
}
