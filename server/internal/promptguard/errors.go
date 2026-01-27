// Package promptguard provides protection against prompt injection attacks.
package promptguard

import "errors"

var (
	// ErrPromptInjectionDetected is returned when a prompt injection is detected.
	ErrPromptInjectionDetected = errors.New("prompt injection detected")

	// ErrInputTooLong is returned when input exceeds the maximum length.
	ErrInputTooLong = errors.New("input exceeds maximum length")

	// ErrInvalidPattern is returned when a pattern cannot be compiled.
	ErrInvalidPattern = errors.New("invalid pattern")

	// ErrOutputFiltered is returned when output was filtered.
	ErrOutputFiltered = errors.New("output contained filtered content")
)
