package proxy

import (
	"bytes"
	"io"
	"net/http"
	"time"
)

// MiddlewareFunc is a function that wraps an http.Handler
type MiddlewareFunc func(http.Handler) http.Handler

// Pipeline represents a request processing pipeline
type Pipeline struct {
	auth           *Authenticator
	guard          *PromptGuard
	sessionMonitor *SessionMonitor
	metrics        *MetricsCollector
	middlewares    []MiddlewareFunc
}

// PipelineConfig holds pipeline configuration
type PipelineConfig struct {
	AuthConfig      *AuthConfig      `json:"auth"`
	RateLimitConfig *RateLimitConfig `json:"rate_limit"`
	GuardConfig     *GuardConfig     `json:"guard"`
	SessionConfig   *SessionConfig   `json:"session"`
	MetricsConfig   *MetricsConfig   `json:"metrics"`
}

// DefaultPipelineConfig returns default pipeline configuration
func DefaultPipelineConfig() *PipelineConfig {
	return &PipelineConfig{
		AuthConfig:      DefaultAuthConfig(),
		RateLimitConfig: DefaultRateLimitConfig(),
		GuardConfig:     DefaultGuardConfig(),
		SessionConfig:   DefaultSessionConfig(),
		MetricsConfig:   DefaultMetricsConfig(),
	}
}

// NewPipeline creates a new request pipeline
func NewPipeline(config *PipelineConfig) *Pipeline {
	if config == nil {
		config = DefaultPipelineConfig()
	}

	return &Pipeline{
		auth:           NewAuthenticator(config.AuthConfig, config.RateLimitConfig),
		guard:          NewPromptGuard(config.GuardConfig),
		sessionMonitor: NewSessionMonitor(config.SessionConfig),
		metrics:        NewMetricsCollector(config.MetricsConfig),
		middlewares:    make([]MiddlewareFunc, 0),
	}
}

// Use adds a middleware to the pipeline
func (p *Pipeline) Use(mw MiddlewareFunc) {
	p.middlewares = append(p.middlewares, mw)
}

// Wrap wraps a handler with the pipeline middlewares
func (p *Pipeline) Wrap(handler http.Handler) http.Handler {
	// Apply custom middlewares in reverse order
	for i := len(p.middlewares) - 1; i >= 0; i-- {
		handler = p.middlewares[i](handler)
	}

	// Apply built-in middlewares
	handler = p.metricsMiddleware(handler)
	handler = p.sessionMiddleware(handler)
	handler = p.guardMiddleware(handler)
	handler = p.rateLimitMiddleware(handler)
	handler = p.authMiddleware(handler)

	return handler
}

// authMiddleware handles authentication
func (p *Pipeline) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ok, reason := p.auth.Authenticate(r)
		if !ok {
			http.Error(w, reason, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// rateLimitMiddleware handles rate limiting
func (p *Pipeline) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ok, reason := p.auth.CheckRateLimit(r)
		if !ok {
			w.Header().Set("Retry-After", "60")
			http.Error(w, reason, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// guardMiddleware handles prompt injection detection
func (p *Pipeline) guardMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only check POST requests with JSON body
		if r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}

		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			next.ServeHTTP(w, r)
			return
		}

		// Read body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}
		r.Body.Close()

		// Extract prompt from request
		prompt := extractPromptFromBody(body)
		if prompt != "" {
			result := p.guard.Check(prompt)
			if result.Blocked {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":       "prompt injection detected",
					"risk_level":  result.RiskLevel,
					"suggestions": result.Suggestions,
				})
				return
			}
		}

		// Restore body for downstream handlers
		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}

// sessionMiddleware handles session tracking
func (p *Pipeline) sessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Start session
		clientIP := p.auth.getClientIP(r)
		session := p.sessionMonitor.StartSession(clientIP, r.UserAgent())

		// Store session ID in context via header
		if session != nil {
			w.Header().Set("X-Session-ID", session.ID)
		}

		// Create response wrapper to capture status
		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(rw, r)

		// Update session on completion
		if session != nil {
			status := SessionStatusCompleted
			if rw.statusCode >= 400 {
				status = SessionStatusFailed
				p.sessionMonitor.RecordError(session.ID)
			}
			p.sessionMonitor.CompleteSession(session.ID, status)
		}
	})
}

// metricsMiddleware handles metrics collection
func (p *Pipeline) metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create response wrapper
		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(rw, r)

		// Record metrics
		latency := time.Since(start)
		p.metrics.Record(RequestMetrics{
			Timestamp:    start,
			StatusCode:   rw.statusCode,
			Latency:      latency,
			RequestSize:  r.ContentLength,
			ResponseSize: int64(rw.bytesWritten),
			Success:      rw.statusCode < 400,
		})
	})
}

// responseWriter wraps http.ResponseWriter to capture response info
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}

// Flush implements http.Flusher
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// extractPromptFromBody extracts prompt content from request body
func extractPromptFromBody(body []byte) string {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return ""
	}

	// Check common prompt fields
	promptFields := []string{"prompt", "content", "text", "input", "query"}
	for _, field := range promptFields {
		if val, ok := data[field]; ok {
			if str, ok := val.(string); ok {
				return str
			}
		}
	}

	// Check messages array (OpenAI/Anthropic format)
	if messages, ok := data["messages"].([]interface{}); ok {
		var prompts []string
		for _, msg := range messages {
			if m, ok := msg.(map[string]interface{}); ok {
				if content, ok := m["content"].(string); ok {
					prompts = append(prompts, content)
				}
			}
		}
		if len(prompts) > 0 {
			return prompts[len(prompts)-1] // Return last message
		}
	}

	return ""
}

// GetAuth returns the authenticator
func (p *Pipeline) GetAuth() *Authenticator {
	return p.auth
}

// GetGuard returns the prompt guard
func (p *Pipeline) GetGuard() *PromptGuard {
	return p.guard
}

// GetSessionMonitor returns the session monitor
func (p *Pipeline) GetSessionMonitor() *SessionMonitor {
	return p.sessionMonitor
}

// GetMetrics returns the metrics collector
func (p *Pipeline) GetMetrics() *MetricsCollector {
	return p.metrics
}

// Stats returns pipeline statistics
func (p *Pipeline) Stats() map[string]interface{} {
	return map[string]interface{}{
		"auth":     p.auth.Stats(),
		"guard":    p.guard.Stats(),
		"sessions": p.sessionMonitor.Stats(),
		"metrics":  p.metrics.Summary(),
	}
}
