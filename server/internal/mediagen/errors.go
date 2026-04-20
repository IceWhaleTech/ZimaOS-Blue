package mediagen

import "errors"

var (
	ErrProviderNotFound    = errors.New("mediagen: no provider found for model")
	ErrModelUnavailable    = errors.New("mediagen: requested model is not available")
	ErrTaskNotFound        = errors.New("mediagen: task not found")
	ErrUnsupportedType     = errors.New("mediagen: unsupported media type")
	ErrGenerationFailed    = errors.New("mediagen: generation failed")
	ErrGenerationCancelled = errors.New("mediagen: generation cancelled")
	ErrTimeout             = errors.New("mediagen: generation timed out")
	ErrNoResults           = errors.New("mediagen: no results returned")
	ErrManagerClosed       = errors.New("mediagen: manager shutting down")
)
