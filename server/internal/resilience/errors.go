package resilience

import (
	"errors"
	"fmt"
	"net/http"
)

// ErrorCategory represents the category of an error
type ErrorCategory int

const (
	// CategoryUnknown is for unclassified errors
	CategoryUnknown ErrorCategory = iota
	// CategoryValidation is for input validation errors
	CategoryValidation
	// CategoryAuthentication is for authentication errors
	CategoryAuthentication
	// CategoryAuthorization is for authorization errors
	CategoryAuthorization
	// CategoryNotFound is for resource not found errors
	CategoryNotFound
	// CategoryConflict is for resource conflict errors
	CategoryConflict
	// CategoryRateLimit is for rate limiting errors
	CategoryRateLimit
	// CategoryTimeout is for timeout errors
	CategoryTimeout
	// CategoryUnavailable is for service unavailable errors
	CategoryUnavailable
	// CategoryInternal is for internal server errors
	CategoryInternal
	// CategoryExternal is for external service errors
	CategoryExternal
	// CategoryLLM is for LLM provider errors
	CategoryLLM
)

func (c ErrorCategory) String() string {
	switch c {
	case CategoryValidation:
		return "validation"
	case CategoryAuthentication:
		return "authentication"
	case CategoryAuthorization:
		return "authorization"
	case CategoryNotFound:
		return "not_found"
	case CategoryConflict:
		return "conflict"
	case CategoryRateLimit:
		return "rate_limit"
	case CategoryTimeout:
		return "timeout"
	case CategoryUnavailable:
		return "unavailable"
	case CategoryInternal:
		return "internal"
	case CategoryExternal:
		return "external"
	case CategoryLLM:
		return "llm"
	default:
		return "unknown"
	}
}

// HTTPStatus returns the appropriate HTTP status code for the category
func (c ErrorCategory) HTTPStatus() int {
	switch c {
	case CategoryValidation:
		return http.StatusBadRequest
	case CategoryAuthentication:
		return http.StatusUnauthorized
	case CategoryAuthorization:
		return http.StatusForbidden
	case CategoryNotFound:
		return http.StatusNotFound
	case CategoryConflict:
		return http.StatusConflict
	case CategoryRateLimit:
		return http.StatusTooManyRequests
	case CategoryTimeout:
		return http.StatusGatewayTimeout
	case CategoryUnavailable:
		return http.StatusServiceUnavailable
	case CategoryInternal:
		return http.StatusInternalServerError
	case CategoryExternal:
		return http.StatusBadGateway
	case CategoryLLM:
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

// AppError represents a structured application error
type AppError struct {
	Category  ErrorCategory `json:"category"`
	Code      string        `json:"code"`
	Message   string        `json:"message"`
	Details   interface{}   `json:"details,omitempty"`
	Cause     error         `json:"-"`
	Retryable bool          `json:"retryable"`
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

// HTTPStatus returns the HTTP status code for this error
func (e *AppError) HTTPStatus() int {
	return e.Category.HTTPStatus()
}

// NewAppError creates a new application error
func NewAppError(category ErrorCategory, code, message string) *AppError {
	return &AppError{
		Category: category,
		Code:     code,
		Message:  message,
	}
}

// WithCause adds a cause to the error
func (e *AppError) WithCause(cause error) *AppError {
	e.Cause = cause
	return e
}

// WithDetails adds details to the error
func (e *AppError) WithDetails(details interface{}) *AppError {
	e.Details = details
	return e
}

// WithRetryable marks the error as retryable
func (e *AppError) WithRetryable(retryable bool) *AppError {
	e.Retryable = retryable
	return e
}

// Common error constructors

// ErrValidation creates a validation error
func ErrValidation(code, message string) *AppError {
	return NewAppError(CategoryValidation, code, message)
}

// ErrAuthentication creates an authentication error
func ErrAuthentication(code, message string) *AppError {
	return NewAppError(CategoryAuthentication, code, message)
}

// ErrAuthorization creates an authorization error
func ErrAuthorization(code, message string) *AppError {
	return NewAppError(CategoryAuthorization, code, message)
}

// ErrNotFound creates a not found error
func ErrNotFound(code, message string) *AppError {
	return NewAppError(CategoryNotFound, code, message)
}

// ErrConflict creates a conflict error
func ErrConflict(code, message string) *AppError {
	return NewAppError(CategoryConflict, code, message)
}

// ErrRateLimit creates a rate limit error
func ErrRateLimit(code, message string) *AppError {
	return NewAppError(CategoryRateLimit, code, message).WithRetryable(true)
}

// ErrTimeout creates a timeout error
func ErrTimeout(code, message string) *AppError {
	return NewAppError(CategoryTimeout, code, message).WithRetryable(true)
}

// ErrUnavailable creates a service unavailable error
func ErrUnavailable(code, message string) *AppError {
	return NewAppError(CategoryUnavailable, code, message).WithRetryable(true)
}

// ErrInternal creates an internal error
func ErrInternal(code, message string) *AppError {
	return NewAppError(CategoryInternal, code, message)
}

// ErrExternal creates an external service error
func ErrExternal(code, message string) *AppError {
	return NewAppError(CategoryExternal, code, message).WithRetryable(true)
}

// ErrLLM creates an LLM provider error
func ErrLLM(code, message string) *AppError {
	return NewAppError(CategoryLLM, code, message).WithRetryable(true)
}

// ErrorClassifier classifies errors into categories
type ErrorClassifier struct {
	classifiers []func(error) *ErrorCategory
}

// NewErrorClassifier creates a new error classifier
func NewErrorClassifier() *ErrorClassifier {
	return &ErrorClassifier{
		classifiers: make([]func(error) *ErrorCategory, 0),
	}
}

// Register adds a classifier function
func (c *ErrorClassifier) Register(classifier func(error) *ErrorCategory) {
	c.classifiers = append(c.classifiers, classifier)
}

// Classify determines the category of an error
func (c *ErrorClassifier) Classify(err error) ErrorCategory {
	if err == nil {
		return CategoryUnknown
	}

	// Check if it's already an AppError
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Category
	}

	// Try registered classifiers
	for _, classifier := range c.classifiers {
		if category := classifier(err); category != nil {
			return *category
		}
	}

	// Check for common error types
	if errors.Is(err, ErrCircuitOpen) || errors.Is(err, ErrTooManyRequests) {
		return CategoryUnavailable
	}

	if errors.Is(err, ErrBulkheadFull) {
		return CategoryUnavailable
	}

	return CategoryUnknown
}

// ErrorResponse represents the JSON response for errors
type ErrorResponse struct {
	Error struct {
		Category  string      `json:"category"`
		Code      string      `json:"code"`
		Message   string      `json:"message"`
		Details   interface{} `json:"details,omitempty"`
		Retryable bool        `json:"retryable"`
	} `json:"error"`
}

// ToResponse converts an AppError to an ErrorResponse
func (e *AppError) ToResponse() ErrorResponse {
	var resp ErrorResponse
	resp.Error.Category = e.Category.String()
	resp.Error.Code = e.Code
	resp.Error.Message = e.Message
	resp.Error.Details = e.Details
	resp.Error.Retryable = e.Retryable
	return resp
}
