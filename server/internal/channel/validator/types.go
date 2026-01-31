// Package validator provides connection validation for messaging channels.
package validator

import (
	"context"
	"time"
)

// DefaultTimeout is the default timeout for connection validation.
const DefaultTimeout = 10 * time.Second

// Result represents the result of a connection validation.
type Result struct {
	// Success indicates if the connection was successful.
	Success bool `json:"success"`
	// MessageKey is the i18n key for the result message.
	MessageKey string `json:"messageKey"`
	// Error contains the error message if validation failed.
	Error string `json:"error,omitempty"`
	// Data contains additional information from the validation.
	Data map[string]interface{} `json:"data,omitempty"`
}

// Validator defines the interface for channel connection validators.
type Validator interface {
	// Validate tests the connection with the given configuration.
	// It returns a Result indicating success or failure with details.
	Validate(ctx context.Context, config map[string]string) Result
}

// ValidatorFunc is a function type that implements Validator.
type ValidatorFunc func(ctx context.Context, config map[string]string) Result

// Validate implements the Validator interface.
func (f ValidatorFunc) Validate(ctx context.Context, config map[string]string) Result {
	return f(ctx, config)
}

// NewSuccessResult creates a successful validation result.
func NewSuccessResult(messageKey string, data map[string]interface{}) Result {
	return Result{
		Success:    true,
		MessageKey: messageKey,
		Data:       data,
	}
}

// NewErrorResult creates a failed validation result.
func NewErrorResult(messageKey string, err string) Result {
	return Result{
		Success:    false,
		MessageKey: messageKey,
		Error:      err,
	}
}

// NewMissingFieldResult creates a result for missing required field.
func NewMissingFieldResult(fieldKey string) Result {
	return Result{
		Success:    false,
		MessageKey: fieldKey + "Required",
	}
}
