package providerpool

import "errors"

var (
	// ErrProviderNotFound indicates the requested provider does not exist
	ErrProviderNotFound = errors.New("provider not found")

	// ErrProviderExists indicates a provider with the same ID already exists
	ErrProviderExists = errors.New("provider already exists")

	// ErrInvalidProviderID indicates the provider ID is malformed or unsafe.
	ErrInvalidProviderID = errors.New("invalid provider id")

	// ErrProviderDisabled indicates the provider is disabled
	ErrProviderDisabled = errors.New("provider is disabled")

	// ErrNoAvailableProvider indicates no healthy provider is available
	ErrNoAvailableProvider = errors.New("no available provider")

	// ErrModelNotFound indicates the requested model does not exist
	ErrModelNotFound = errors.New("model not found")

	// ErrModelDisabled indicates the model is disabled
	ErrModelDisabled = errors.New("model is disabled")

	// ErrAPIKeyNotFound indicates the requested API key does not exist
	ErrAPIKeyNotFound = errors.New("api key not found")

	// ErrAPIKeyInvalid indicates the API key is invalid or expired
	ErrAPIKeyInvalid = errors.New("api key is invalid")

	// ErrAuthRequired indicates authentication is required but no valid credentials provided
	ErrAuthRequired = errors.New("authentication required - please provide a valid API key")

	// ErrNoAPIKey indicates no API key is configured for the provider
	ErrNoAPIKey = errors.New("no api key configured")

	// ErrRateLimitExceeded indicates the rate limit has been exceeded
	ErrRateLimitExceeded = errors.New("rate limit exceeded")

	// ErrQuotaExceeded indicates the usage quota has been exceeded
	ErrQuotaExceeded = errors.New("quota exceeded")

	// ErrHealthCheckFailed indicates the health check failed
	ErrHealthCheckFailed = errors.New("health check failed")

	// ErrEncryptionFailed indicates encryption/decryption failed
	ErrEncryptionFailed = errors.New("encryption failed")

	// ErrStorageError indicates a storage operation failed
	ErrStorageError = errors.New("storage error")

	// ErrInvalidConfig indicates invalid configuration
	ErrInvalidConfig = errors.New("invalid configuration")

	// ErrIDENotFound indicates the IDE was not found
	ErrIDENotFound = errors.New("ide not found")

	// ErrIDEConnectionFailed indicates connection to IDE failed
	ErrIDEConnectionFailed = errors.New("ide connection failed")

	// ErrPricingNotFound indicates the pricing configuration was not found
	ErrPricingNotFound = errors.New("pricing not found")
)

// ProviderError wraps an error with provider context
type ProviderError struct {
	ProviderID string
	Operation  string
	Err        error
}

func (e *ProviderError) Error() string {
	if e.ProviderID != "" {
		return "provider " + e.ProviderID + ": " + e.Operation + ": " + e.Err.Error()
	}
	return e.Operation + ": " + e.Err.Error()
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}

// NewProviderError creates a new ProviderError
func NewProviderError(providerID, operation string, err error) *ProviderError {
	return &ProviderError{
		ProviderID: providerID,
		Operation:  operation,
		Err:        err,
	}
}

// ModelError wraps an error with model context
type ModelError struct {
	ProviderID string
	ModelID    string
	Operation  string
	Err        error
}

func (e *ModelError) Error() string {
	return "model " + e.ModelID + " (provider " + e.ProviderID + "): " + e.Operation + ": " + e.Err.Error()
}

func (e *ModelError) Unwrap() error {
	return e.Err
}

// NewModelError creates a new ModelError
func NewModelError(providerID, modelID, operation string, err error) *ModelError {
	return &ModelError{
		ProviderID: providerID,
		ModelID:    modelID,
		Operation:  operation,
		Err:        err,
	}
}
