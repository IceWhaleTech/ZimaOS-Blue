package proxy

import (
	"errors"
	"net/url"
	"strings"
)

// SanitizeError converts raw Go network/TLS errors into user-friendly messages,
// stripping internal URLs and technical details.
func SanitizeError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	lower := strings.ToLower(msg)

	// Network unreachable / DNS failure
	if strings.Contains(lower, "no such host") ||
		strings.Contains(lower, "dial tcp") ||
		strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "network is unreachable") {
		return "Network connection failed. Please check your internet connection and try again."
	}

	// TLS / x509 certificate errors
	if strings.Contains(lower, "x509") ||
		strings.Contains(lower, "certificate") ||
		strings.Contains(lower, "tls") {
		return "Secure connection failed. This may be caused by a proxy, firewall, or network configuration issue."
	}

	// Timeout
	if strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "deadline exceeded") {
		return "Request timed out. The server may be busy — please try again later."
	}

	// EOF / connection reset
	if strings.Contains(lower, "eof") ||
		strings.Contains(lower, "connection reset") ||
		strings.Contains(lower, "broken pipe") {
		return "Connection was interrupted. Please try again."
	}

	// Strip any URLs from the message to avoid leaking internal endpoints
	return stripURLs(msg)
}

// stripURLs removes http(s) URLs from a string, replacing them with "[server]".
func stripURLs(s string) string {
	var result strings.Builder
	remaining := s
	for {
		idx := strings.Index(remaining, "http://")
		if idx == -1 {
			idx = strings.Index(remaining, "https://")
		}
		if idx == -1 {
			result.WriteString(remaining)
			break
		}
		result.WriteString(remaining[:idx])
		remaining = remaining[idx:]
		// Find end of URL
		end := strings.IndexAny(remaining, " \t\n\r\"'>)")
		if end == -1 {
			end = len(remaining)
		}
		urlStr := remaining[:end]
		if u, err := url.Parse(urlStr); err == nil && u.Host != "" {
			result.WriteString("[server]")
		} else {
			result.WriteString("[server]")
		}
		remaining = remaining[end:]
	}
	return result.String()
}

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
