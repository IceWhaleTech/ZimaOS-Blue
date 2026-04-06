package proxy

import (
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrorCategory classifies API errors for failover decisions
type ErrorCategory string

const (
	// ErrorCategoryNonRetryable - do not failover or retry
	ErrorCategoryNonRetryable ErrorCategory = "non_retryable"

	// ErrorCategoryRetryable - retry with same provider (transient errors)
	ErrorCategoryRetryable ErrorCategory = "retryable"

	// ErrorCategoryFailover - retry with different provider (provider-specific limits)
	ErrorCategoryFailover ErrorCategory = "failover"

	// ErrorCategoryStreamAnomaly - force stop and recover
	ErrorCategoryStreamAnomaly ErrorCategory = "stream_anomaly"
)

// RetryableErrorType defines specific error types
type RetryableErrorType string

const (
	// Context/Token Limits - MUST failover to provider with larger context
	ErrorTypeContextTooLong    RetryableErrorType = "context_too_long"
	ErrorTypeMaxTokensExceeded RetryableErrorType = "max_tokens_exceeded"

	// Rate/Quota Limits - failover to different provider or API key
	ErrorTypeRateLimited      RetryableErrorType = "rate_limited"
	ErrorTypeQuotaExceeded    RetryableErrorType = "quota_exceeded"
	ErrorTypeConcurrencyLimit RetryableErrorType = "concurrency_limit"

	// Provider Issues - failover to different provider
	ErrorTypeModelOverloaded    RetryableErrorType = "model_overloaded"
	ErrorTypeServiceUnavailable RetryableErrorType = "service_unavailable"
	ErrorTypeTimeout            RetryableErrorType = "timeout"

	// Streaming Anomalies - force stop and attempt recovery
	ErrorTypeRepetitiveOutput RetryableErrorType = "repetitive_output"
	ErrorTypeInfiniteLoop     RetryableErrorType = "infinite_loop"

	// Model not configured — model appears in list but isn't usable on this provider.
	// Should failover to next provider (model may work elsewhere).
	ErrorTypeModelNotConfigured RetryableErrorType = "model_not_configured"

	// Non-retryable
	ErrorTypeInvalidRequest RetryableErrorType = "invalid_request"
	ErrorTypeAuthFailed     RetryableErrorType = "auth_failed"
	ErrorTypeModelNotFound  RetryableErrorType = "model_not_found"
	ErrorTypeNetworkError   RetryableErrorType = "network_error"
	ErrorTypeUnknown        RetryableErrorType = "unknown"
)

// ErrorPattern defines a pattern to match API errors
type ErrorPattern struct {
	Type            RetryableErrorType
	Category        ErrorCategory
	StatusCodes     []int
	MessagePatterns []*regexp.Regexp
}

// ErrorClassification result
type ErrorClassification struct {
	Type                   RetryableErrorType `json:"type"`
	Category               ErrorCategory      `json:"category"`
	Message                string             `json:"message"`
	Retryable              bool               `json:"retryable"`
	ShouldFailover         bool               `json:"should_failover"`
	SuggestedContextWindow int                `json:"suggested_context_window,omitempty"`
	RetryAfter             time.Duration      `json:"retry_after,omitempty"`
	OriginalStatusCode     int                `json:"original_status_code"`
}

// APIErrorClassifier classifies API errors from different providers
type APIErrorClassifier struct {
	patterns map[string][]ErrorPattern
}

var (
	defaultAPIErrorPatternsOnce sync.Once
	defaultAPIErrorPatterns     map[string][]ErrorPattern
)

// IsContextWindowExceededMessage returns true when an error message clearly
// indicates input/context overflow, including relay-wrapped variants that may
// be surfaced with a generic 5xx status code.
func IsContextWindowExceededMessage(msg string) bool {
	msg = strings.TrimSpace(strings.ToLower(msg))
	if msg == "" {
		return false
	}

	return strings.Contains(msg, "context too long") ||
		strings.Contains(msg, "maximum context length") ||
		strings.Contains(msg, "context_length_exceeded") ||
		strings.Contains(msg, "prompt is too long") ||
		strings.Contains(msg, "context window is full") ||
		strings.Contains(msg, "reduce conversation history")
}

// NewAPIErrorClassifier creates a new error classifier with default patterns
func NewAPIErrorClassifier() *APIErrorClassifier {
	return &APIErrorClassifier{}
}

func (c *APIErrorClassifier) ensurePatterns() {
	if c == nil || c.patterns != nil {
		return
	}
	defaultAPIErrorPatternsOnce.Do(func() {
		defaultAPIErrorPatterns = buildDefaultAPIErrorPatterns()
	})
	c.patterns = defaultAPIErrorPatterns
}

// buildDefaultAPIErrorPatterns initializes error patterns for known providers.
func buildDefaultAPIErrorPatterns() map[string][]ErrorPattern {
	patterns := make(map[string][]ErrorPattern)

	// Anthropic patterns
	patterns["anthropic"] = []ErrorPattern{
		{
			Type:        ErrorTypeContextTooLong,
			Category:    ErrorCategoryFailover,
			StatusCodes: []int{400},
			MessagePatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?i)context size \((\d+) tokens?\) exceeds maximum`),
				regexp.MustCompile(`(?i)request context size.*exceeds maximum allowed`),
				regexp.MustCompile(`(?i)prompt is too long`),
			},
		},
		{
			Type:        ErrorTypeRateLimited,
			Category:    ErrorCategoryFailover,
			StatusCodes: []int{429},
			MessagePatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?i)rate.?limit`),
				regexp.MustCompile(`(?i)too many requests`),
			},
		},
		{
			Type:        ErrorTypeQuotaExceeded,
			Category:    ErrorCategoryFailover,
			StatusCodes: []int{400, 429},
			MessagePatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?i)quota`),
				regexp.MustCompile(`(?i)credit`),
				regexp.MustCompile(`(?i)billing`),
				regexp.MustCompile(`(?i)insufficient.*balance`),
			},
		},
		{
			Type:        ErrorTypeModelOverloaded,
			Category:    ErrorCategoryFailover,
			StatusCodes: []int{529, 503},
			MessagePatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?i)overloaded`),
				regexp.MustCompile(`(?i)capacity`),
			},
		},
		{
			Type:        ErrorTypeAuthFailed,
			Category:    ErrorCategoryNonRetryable,
			StatusCodes: []int{401, 403},
			MessagePatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?i)invalid.*api.?key`),
				regexp.MustCompile(`(?i)authentication`),
				regexp.MustCompile(`(?i)unauthorized`),
			},
		},
		{
			Type:        ErrorTypeModelNotConfigured,
			Category:    ErrorCategoryFailover,
			StatusCodes: []int{400, 403, 404, 422},
			MessagePatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?i)not configured`),
				regexp.MustCompile(`(?i)not enabled`),
				regexp.MustCompile(`(?i)not available`),
				regexp.MustCompile(`(?i)not supported`),
				regexp.MustCompile(`(?i)no access`),
				regexp.MustCompile(`(?i)not authorized.*model`),
				regexp.MustCompile(`(?i)model.*disabled`),
				regexp.MustCompile(`(?i)model.*unavailable`),
			},
		},
		{
			Type:        ErrorTypeModelNotFound,
			Category:    ErrorCategoryFailover,
			StatusCodes: []int{404},
			MessagePatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?i)model.*not found`),
				regexp.MustCompile(`(?i)invalid.*model`),
			},
		},
	}

	// OpenAI patterns
	patterns["openai"] = []ErrorPattern{
		{
			Type:        ErrorTypeContextTooLong,
			Category:    ErrorCategoryFailover,
			StatusCodes: []int{400},
			MessagePatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?i)maximum context length`),
				regexp.MustCompile(`(?i)(\d+) tokens.*exceeds.*limit`),
				regexp.MustCompile(`(?i)context_length_exceeded`),
			},
		},
		{
			Type:        ErrorTypeRateLimited,
			Category:    ErrorCategoryFailover,
			StatusCodes: []int{429},
			MessagePatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?i)rate_limit_exceeded`),
				regexp.MustCompile(`(?i)too many requests`),
			},
		},
		{
			Type:        ErrorTypeQuotaExceeded,
			Category:    ErrorCategoryFailover,
			StatusCodes: []int{429},
			MessagePatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?i)insufficient_quota`),
				regexp.MustCompile(`(?i)billing`),
				regexp.MustCompile(`(?i)exceeded.*quota`),
			},
		},
		{
			Type:        ErrorTypeModelOverloaded,
			Category:    ErrorCategoryFailover,
			StatusCodes: []int{503},
			MessagePatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?i)overloaded`),
				regexp.MustCompile(`(?i)server.*busy`),
			},
		},
	}

	// DeepSeek patterns
	patterns["deepseek"] = []ErrorPattern{
		{
			Type:        ErrorTypeContextTooLong,
			Category:    ErrorCategoryFailover,
			StatusCodes: []int{400},
			MessagePatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?i)context.*too long`),
				regexp.MustCompile(`(?i)token.*limit`),
				regexp.MustCompile(`(?i)exceeds.*maximum`),
			},
		},
		{
			Type:        ErrorTypeRateLimited,
			Category:    ErrorCategoryFailover,
			StatusCodes: []int{429},
			MessagePatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?i)rate.*limit`),
			},
		},
	}

	// Google/Gemini patterns
	patterns["google"] = []ErrorPattern{
		{
			Type:        ErrorTypeContextTooLong,
			Category:    ErrorCategoryFailover,
			StatusCodes: []int{400},
			MessagePatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?i)exceeds.*token.*limit`),
				regexp.MustCompile(`(?i)input.*too.*long`),
			},
		},
		{
			Type:        ErrorTypeRateLimited,
			Category:    ErrorCategoryFailover,
			StatusCodes: []int{429},
			MessagePatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?i)quota.*exceeded`),
				regexp.MustCompile(`(?i)rate.*limit`),
			},
		},
	}

	// Generic patterns (fallback for unknown providers)
	patterns["generic"] = []ErrorPattern{
		{
			Type:        ErrorTypeContextTooLong,
			Category:    ErrorCategoryFailover,
			StatusCodes: []int{400},
			MessagePatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?i)context.*exceed`),
				regexp.MustCompile(`(?i)token.*limit`),
				regexp.MustCompile(`(?i)too.*long`),
				regexp.MustCompile(`(?i)maximum.*allowed`),
			},
		},
		{
			Type:        ErrorTypeRateLimited,
			Category:    ErrorCategoryFailover,
			StatusCodes: []int{429},
			MessagePatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?i)rate.*limit`),
				regexp.MustCompile(`(?i)too many`),
			},
		},
		{
			Type:        ErrorTypeQuotaExceeded,
			Category:    ErrorCategoryFailover,
			StatusCodes: []int{402, 429},
			MessagePatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?i)quota`),
				regexp.MustCompile(`(?i)credit`),
				regexp.MustCompile(`(?i)balance`),
			},
		},
		{
			Type:        ErrorTypeServiceUnavailable,
			Category:    ErrorCategoryFailover,
			StatusCodes: []int{500, 502, 503, 504},
			MessagePatterns: []*regexp.Regexp{
				regexp.MustCompile(`.*`), // Match any message for these status codes
			},
		},
		{
			Type:        ErrorTypeModelNotConfigured,
			Category:    ErrorCategoryFailover,
			StatusCodes: []int{400, 403, 404, 422},
			MessagePatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?i)not configured`),
				regexp.MustCompile(`(?i)not enabled`),
				regexp.MustCompile(`(?i)not available`),
				regexp.MustCompile(`(?i)not supported`),
				regexp.MustCompile(`(?i)no access`),
				regexp.MustCompile(`(?i)model.*disabled`),
				regexp.MustCompile(`(?i)model.*unavailable`),
			},
		},
	}

	return patterns
}

// ClassifyError analyzes response and returns error classification
func (c *APIErrorClassifier) ClassifyError(
	provider string,
	statusCode int,
	responseBody []byte,
) *ErrorClassification {
	c.ensurePatterns()

	// Parse error response - try multiple formats
	message := c.extractErrorMessage(responseBody)
	if statusCode >= 400 && IsContextWindowExceededMessage(message) {
		classification := &ErrorClassification{
			Type:               ErrorTypeContextTooLong,
			Category:           ErrorCategoryFailover,
			Message:            message,
			Retryable:          true,
			ShouldFailover:     true,
			OriginalStatusCode: statusCode,
		}
		c.extractContextSize(message, classification)
		c.extractRetryAfter(responseBody, classification)
		return classification
	}

	// Try provider-specific patterns first
	if patterns, ok := c.patterns[strings.ToLower(provider)]; ok {
		if classification := c.matchPatterns(patterns, statusCode, message); classification != nil {
			classification.OriginalStatusCode = statusCode
			c.extractContextSize(message, classification)
			c.extractRetryAfter(responseBody, classification)
			return classification
		}
	}

	// Fall back to generic patterns
	if classification := c.matchPatterns(c.patterns["generic"], statusCode, message); classification != nil {
		classification.OriginalStatusCode = statusCode
		c.extractContextSize(message, classification)
		return classification
	}

	// Default classification based on status code
	return c.defaultClassification(statusCode, message)
}

// extractErrorMessage extracts error message from various response formats
func (c *APIErrorClassifier) extractErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	// Try standard error format
	var errResp struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		return errResp.Error.Message
	}

	// Try OpenAI format
	var openaiResp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &openaiResp); err == nil && openaiResp.Error.Message != "" {
		return openaiResp.Error.Message
	}

	// Try simple message format
	var simpleResp struct {
		Message string `json:"message"`
		Msg     string `json:"msg"`
	}
	if err := json.Unmarshal(body, &simpleResp); err == nil {
		if simpleResp.Message != "" {
			return simpleResp.Message
		}
		if simpleResp.Msg != "" {
			return simpleResp.Msg
		}
	}

	// Return raw body as string (truncated)
	s := string(body)
	if len(s) > 500 {
		s = s[:500]
	}
	return s
}

// matchPatterns tries to match error against patterns
func (c *APIErrorClassifier) matchPatterns(patterns []ErrorPattern, statusCode int, message string) *ErrorClassification {
	for _, pattern := range patterns {
		// Check status code
		statusMatch := false
		for _, code := range pattern.StatusCodes {
			if code == statusCode {
				statusMatch = true
				break
			}
		}
		if !statusMatch {
			continue
		}

		// Check message patterns
		for _, re := range pattern.MessagePatterns {
			if re.MatchString(message) {
				return &ErrorClassification{
					Type:           pattern.Type,
					Category:       pattern.Category,
					Message:        message,
					Retryable:      pattern.Category != ErrorCategoryNonRetryable,
					ShouldFailover: pattern.Category == ErrorCategoryFailover,
				}
			}
		}
	}
	return nil
}

// extractContextSize extracts context window size from error message
func (c *APIErrorClassifier) extractContextSize(message string, classification *ErrorClassification) {
	if classification.Type != ErrorTypeContextTooLong {
		return
	}

	// Try to extract token count from message
	re := regexp.MustCompile(`(\d{4,})`)
	matches := re.FindAllString(message, -1)
	if len(matches) > 0 {
		// Take the largest number as the context size
		maxSize := 0
		for _, m := range matches {
			if size, err := strconv.Atoi(m); err == nil && size > maxSize {
				maxSize = size
			}
		}
		if maxSize > 0 {
			classification.SuggestedContextWindow = maxSize
		}
	}
}

// extractRetryAfter extracts retry-after from response
func (c *APIErrorClassifier) extractRetryAfter(body []byte, classification *ErrorClassification) {
	var resp struct {
		RetryAfter float64 `json:"retry_after"`
	}
	if err := json.Unmarshal(body, &resp); err == nil && resp.RetryAfter > 0 {
		classification.RetryAfter = time.Duration(resp.RetryAfter * float64(time.Second))
	}
}

// defaultClassification returns default classification based on status code
func (c *APIErrorClassifier) defaultClassification(statusCode int, message string) *ErrorClassification {
	classification := &ErrorClassification{
		Message:            message,
		OriginalStatusCode: statusCode,
	}

	switch {
	case statusCode >= 500:
		classification.Type = ErrorTypeServiceUnavailable
		classification.Category = ErrorCategoryFailover
		classification.Retryable = true
		classification.ShouldFailover = true
	case statusCode == 429:
		classification.Type = ErrorTypeRateLimited
		classification.Category = ErrorCategoryFailover
		classification.Retryable = true
		classification.ShouldFailover = true
	case statusCode == 401 || statusCode == 403:
		classification.Type = ErrorTypeAuthFailed
		classification.Category = ErrorCategoryNonRetryable
		classification.Retryable = false
		classification.ShouldFailover = false
	case statusCode == 404:
		classification.Type = ErrorTypeModelNotFound
		classification.Category = ErrorCategoryFailover
		classification.Retryable = true
		classification.ShouldFailover = true
	case statusCode >= 400:
		classification.Type = ErrorTypeInvalidRequest
		classification.Category = ErrorCategoryNonRetryable
		classification.Retryable = false
		classification.ShouldFailover = false
	default:
		classification.Type = ErrorTypeUnknown
		classification.Category = ErrorCategoryNonRetryable
		classification.Retryable = false
		classification.ShouldFailover = false
	}

	return classification
}

// AddProviderPatterns adds custom patterns for a provider
func (c *APIErrorClassifier) AddProviderPatterns(provider string, patterns []ErrorPattern) {
	c.patterns[strings.ToLower(provider)] = patterns
}
