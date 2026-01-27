package llm

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// ErrorType classifies LLM errors for retry and failover decisions.
type ErrorType int

const (
	ErrorTypeUnknown ErrorType = iota
	ErrorTypeRetryable        // Rate limits, temporary failures, network issues
	ErrorTypeNonRetryable     // Auth errors, invalid requests, bad parameters
	ErrorTypeProviderDown     // Provider completely unavailable
	ErrorTypeModelUnavailable // Specific model unavailable
	ErrorTypeQuotaExceeded    // Quota/billing issues
	ErrorTypeContextTooLong   // Context length exceeded
	ErrorTypeContentFiltered  // Content filtered by safety systems
)

// String returns the string representation of ErrorType.
func (e ErrorType) String() string {
	switch e {
	case ErrorTypeRetryable:
		return "retryable"
	case ErrorTypeNonRetryable:
		return "non_retryable"
	case ErrorTypeProviderDown:
		return "provider_down"
	case ErrorTypeModelUnavailable:
		return "model_unavailable"
	case ErrorTypeQuotaExceeded:
		return "quota_exceeded"
	case ErrorTypeContextTooLong:
		return "context_too_long"
	case ErrorTypeContentFiltered:
		return "content_filtered"
	default:
		return "unknown"
	}
}

// ErrorClassifier classifies errors for retry and failover decisions.
type ErrorClassifier interface {
	// Classify returns the error type for the given error.
	Classify(err error) ErrorType

	// ShouldRetry returns true if the error should be retried with the same provider.
	ShouldRetry(err error) bool

	// ShouldFailover returns true if the error should trigger failover to another provider.
	ShouldFailover(err error) bool

	// GetRetryDelay returns the recommended delay before retrying.
	GetRetryDelay(err error, attempt int) time.Duration
}

// LLMError represents an error from an LLM provider with additional context.
type LLMError struct {
	Provider   string
	Model      string
	StatusCode int
	Message    string
	Type       ErrorType
	Retryable  bool
	Cause      error
}

func (e *LLMError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *LLMError) Unwrap() error {
	return e.Cause
}

// NewLLMError creates a new LLMError.
func NewLLMError(provider, model string, statusCode int, message string, cause error) *LLMError {
	err := &LLMError{
		Provider:   provider,
		Model:      model,
		StatusCode: statusCode,
		Message:    message,
		Cause:      cause,
	}
	// Auto-classify based on status code and message
	classifier := NewDefaultErrorClassifier()
	err.Type = classifier.Classify(err)
	err.Retryable = classifier.ShouldRetry(err)
	return err
}

// DefaultErrorClassifier implements ErrorClassifier with common patterns.
type DefaultErrorClassifier struct {
	retryablePatterns      []*regexp.Regexp
	nonRetryablePatterns   []*regexp.Regexp
	quotaPatterns          []*regexp.Regexp
	contextTooLongPatterns []*regexp.Regexp
	contentFilterPatterns  []*regexp.Regexp
	modelUnavailPatterns   []*regexp.Regexp
	baseRetryDelay         time.Duration
	maxRetryDelay          time.Duration
}

// NewDefaultErrorClassifier creates a new DefaultErrorClassifier with common patterns.
func NewDefaultErrorClassifier() *DefaultErrorClassifier {
	return &DefaultErrorClassifier{
		retryablePatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)rate.?limit`),
			regexp.MustCompile(`(?i)too.?many.?requests`),
			regexp.MustCompile(`(?i)temporarily.?unavailable`),
			regexp.MustCompile(`(?i)service.?unavailable`),
			regexp.MustCompile(`(?i)timeout`),
			regexp.MustCompile(`(?i)connection.?refused`),
			regexp.MustCompile(`(?i)connection.?reset`),
			regexp.MustCompile(`(?i)network.?error`),
			regexp.MustCompile(`(?i)server.?error`),
			regexp.MustCompile(`(?i)internal.?error`),
			regexp.MustCompile(`(?i)overloaded`),
			regexp.MustCompile(`(?i)capacity`),
		},
		nonRetryablePatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)invalid.?api.?key`),
			regexp.MustCompile(`(?i)authentication`),
			regexp.MustCompile(`(?i)unauthorized`),
			regexp.MustCompile(`(?i)forbidden`),
			regexp.MustCompile(`(?i)invalid.?request`),
			regexp.MustCompile(`(?i)bad.?request`),
			regexp.MustCompile(`(?i)malformed`),
			regexp.MustCompile(`(?i)not.?found`),
		},
		quotaPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)quota`),
			regexp.MustCompile(`(?i)billing`),
			regexp.MustCompile(`(?i)insufficient.?funds`),
			regexp.MustCompile(`(?i)payment.?required`),
			regexp.MustCompile(`(?i)credit`),
			regexp.MustCompile(`(?i)limit.?exceeded`),
		},
		contextTooLongPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)context.?length`),
			regexp.MustCompile(`(?i)token.?limit`),
			regexp.MustCompile(`(?i)max.?tokens`),
			regexp.MustCompile(`(?i)too.?long`),
			regexp.MustCompile(`(?i)maximum.?context`),
		},
		contentFilterPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)content.?filter`),
			regexp.MustCompile(`(?i)safety`),
			regexp.MustCompile(`(?i)moderation`),
			regexp.MustCompile(`(?i)policy.?violation`),
			regexp.MustCompile(`(?i)harmful`),
		},
		modelUnavailPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)model.?not.?found`),
			regexp.MustCompile(`(?i)model.?unavailable`),
			regexp.MustCompile(`(?i)model.?does.?not.?exist`),
			regexp.MustCompile(`(?i)invalid.?model`),
		},
		baseRetryDelay: 1 * time.Second,
		maxRetryDelay:  30 * time.Second,
	}
}

// Classify returns the error type for the given error.
func (c *DefaultErrorClassifier) Classify(err error) ErrorType {
	if err == nil {
		return ErrorTypeUnknown
	}

	// Check if it's an LLMError with status code
	var llmErr *LLMError
	if errors.As(err, &llmErr) {
		// Classify by status code first
		switch llmErr.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return ErrorTypeNonRetryable
		case http.StatusPaymentRequired:
			return ErrorTypeQuotaExceeded
		case http.StatusNotFound:
			return ErrorTypeModelUnavailable
		case http.StatusTooManyRequests:
			return ErrorTypeRetryable
		case http.StatusServiceUnavailable, http.StatusBadGateway, http.StatusGatewayTimeout:
			return ErrorTypeProviderDown
		case http.StatusBadRequest:
			// Check message for more specific classification
			msg := strings.ToLower(llmErr.Message)
			if c.matchesAny(msg, c.contextTooLongPatterns) {
				return ErrorTypeContextTooLong
			}
			if c.matchesAny(msg, c.contentFilterPatterns) {
				return ErrorTypeContentFiltered
			}
			return ErrorTypeNonRetryable
		}
	}

	// Classify by error message patterns
	msg := strings.ToLower(err.Error())

	if c.matchesAny(msg, c.quotaPatterns) {
		return ErrorTypeQuotaExceeded
	}
	if c.matchesAny(msg, c.contextTooLongPatterns) {
		return ErrorTypeContextTooLong
	}
	if c.matchesAny(msg, c.contentFilterPatterns) {
		return ErrorTypeContentFiltered
	}
	if c.matchesAny(msg, c.modelUnavailPatterns) {
		return ErrorTypeModelUnavailable
	}
	if c.matchesAny(msg, c.nonRetryablePatterns) {
		return ErrorTypeNonRetryable
	}
	if c.matchesAny(msg, c.retryablePatterns) {
		return ErrorTypeRetryable
	}

	return ErrorTypeUnknown
}

// ShouldRetry returns true if the error should be retried with the same provider.
func (c *DefaultErrorClassifier) ShouldRetry(err error) bool {
	errType := c.Classify(err)
	switch errType {
	case ErrorTypeRetryable:
		return true
	case ErrorTypeUnknown:
		// Unknown errors might be transient, allow one retry
		return true
	default:
		return false
	}
}

// ShouldFailover returns true if the error should trigger failover to another provider.
func (c *DefaultErrorClassifier) ShouldFailover(err error) bool {
	errType := c.Classify(err)
	switch errType {
	case ErrorTypeProviderDown, ErrorTypeModelUnavailable, ErrorTypeQuotaExceeded:
		return true
	case ErrorTypeRetryable:
		// After retries exhausted, failover
		return true
	case ErrorTypeContextTooLong:
		// Failover might help if another model has larger context
		return true
	case ErrorTypeNonRetryable, ErrorTypeContentFiltered:
		// These won't be fixed by failover
		return false
	default:
		return true
	}
}

// GetRetryDelay returns the recommended delay before retrying.
func (c *DefaultErrorClassifier) GetRetryDelay(err error, attempt int) time.Duration {
	// Exponential backoff with jitter
	delay := c.baseRetryDelay * time.Duration(1<<uint(attempt))
	if delay > c.maxRetryDelay {
		delay = c.maxRetryDelay
	}

	// Check for rate limit with specific retry-after
	var llmErr *LLMError
	if errors.As(err, &llmErr) {
		if llmErr.StatusCode == http.StatusTooManyRequests {
			// Use longer delay for rate limits
			if delay < 5*time.Second {
				delay = 5 * time.Second
			}
		}
	}

	return delay
}

// matchesAny returns true if the message matches any of the patterns.
func (c *DefaultErrorClassifier) matchesAny(msg string, patterns []*regexp.Regexp) bool {
	for _, p := range patterns {
		if p.MatchString(msg) {
			return true
		}
	}
	return false
}
