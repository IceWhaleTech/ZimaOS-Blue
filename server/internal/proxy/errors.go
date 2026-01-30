package proxy

import "errors"

// Proxy errors
var (
	// Port errors
	ErrPortAllocationFailed = errors.New("failed to allocate port")
	ErrPortRangeInvalid     = errors.New("invalid port range")
	ErrPortInUse            = errors.New("port already in use")

	// Provider errors
	ErrNoAvailableProvider = errors.New("no available provider")
	ErrAllProvidersFailed  = errors.New("all providers failed")
	ErrProviderNotFound    = errors.New("provider not found")
	ErrProviderDisabled    = errors.New("provider is disabled")

	// Request errors
	ErrRequestFailed    = errors.New("request failed")
	ErrRequestTimeout   = errors.New("request timeout")
	ErrUpstreamError    = errors.New("upstream error")
	ErrInvalidRequest   = errors.New("invalid request")

	// Circuit breaker errors
	ErrCircuitOpen = errors.New("circuit breaker is open")

	// Configuration errors
	ErrConfigInvalid   = errors.New("invalid configuration")
	ErrConfigNotFound  = errors.New("configuration not found")

	// Model compatibility errors
	ErrToolCallingNotSupported = errors.New("tool calling not supported by model")
	ErrVisionNotSupported      = errors.New("vision not supported by model")
	ErrContextTooLong          = errors.New("context exceeds model limit")

	// Mock errors
	ErrMockEndpointNotFound = errors.New("mock endpoint not found")
	ErrMockDisabled         = errors.New("mock endpoints disabled")
)
