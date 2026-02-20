package providerpool

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// HTTPHealthChecker performs HTTP-based health checks
type HTTPHealthChecker struct {
	client         *http.Client
	insecureClient *http.Client
}

// NewHTTPHealthChecker creates a new HTTPHealthChecker
func NewHTTPHealthChecker(timeout time.Duration) *HTTPHealthChecker {
	return &HTTPHealthChecker{
		client: &http.Client{
			Timeout: timeout,
		},
		insecureClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // user-opted skip for self-signed certs
			},
		},
	}
}

// Check performs an HTTP health check on a provider
func (c *HTTPHealthChecker) Check(ctx context.Context, provider *Provider) *HealthCheckResult {
	start := time.Now()
	result := &HealthCheckResult{
		ProviderID: provider.ID,
		CheckedAt:  start,
	}

	// Skip if no base URL — give a helpful message for known providers that need configuration
	if provider.BaseURL == "" {
		result.Healthy = false
		switch provider.ID {
		case "azure-openai":
			result.Error = "base_url_not_configured:azure"
		case "bedrock":
			result.Error = "base_url_not_configured:bedrock"
		default:
			result.Error = "base_url_not_configured"
		}
		return result
	}

	// Determine HTTP method — Anthropic format only accepts POST on /messages
	method, healthURLs := getHealthCheckMethod(provider)

	// Pick client based on provider TLS setting
	httpClient := c.client
	if provider.SkipTLSVerify {
		httpClient = c.insecureClient
	}

	var lastErr string
	for _, healthURL := range healthURLs {
		req, err := http.NewRequestWithContext(ctx, method, healthURL, nil)
		if err != nil {
			lastErr = fmt.Sprintf("failed to create request: %v", err)
			continue
		}

		// Always add authentication if available
		if len(provider.APIKeys) > 0 && provider.APIKeys[0].Key != "" {
			addAuthHeader(req, provider)
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = sanitizeHealthError(err)
			result.Latency = time.Since(start)
			continue
		}
		resp.Body.Close()

		result.Latency = time.Since(start)

		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 400:
			result.Healthy = true
			return result
		case resp.StatusCode == 401 || resp.StatusCode == 403:
			// Server is reachable but API key is wrong/missing
			result.Healthy = false
			result.Error = fmt.Sprintf("auth_error:%d", resp.StatusCode)
			return result
		case resp.StatusCode == 400 || resp.StatusCode == 405:
			// 400: server validated the request and rejected it (e.g. missing body) — reachable
			// 405: Method Not Allowed — server is reachable, endpoint just doesn't support this method
			result.Healthy = true
			return result
		case resp.StatusCode == 404:
			// Try next URL
			lastErr = "endpoint_not_found"
		default:
			lastErr = fmt.Sprintf("unexpected_status:%d", resp.StatusCode)
		}
	}

	result.Healthy = false
	result.Error = lastErr
	return result
}

// sanitizeHealthError converts raw network errors to error codes for frontend i18n.
func sanitizeHealthError(err error) string {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "no such host") || strings.Contains(msg, "dial tcp") ||
		strings.Contains(msg, "connection refused") {
		return "network_error"
	}
	if strings.Contains(msg, "x509") || strings.Contains(msg, "certificate") {
		return "certificate_error"
	}
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded") {
		return "timeout_error"
	}
	return "connection_error"
}

// getHealthCheckMethod returns the HTTP method and URLs to use for health checking a provider.
// It uses provider ID for known providers, then falls back to API format for unknown/custom ones.
func getHealthCheckMethod(provider *Provider) (string, []string) {
	baseURL := strings.TrimSuffix(provider.BaseURL, "/")

	// Provider-specific overrides first
	switch provider.ID {
	case "openai", "deepseek", "moonshot", "openrouter", "aihubmix":
		return http.MethodGet, []string{baseURL + "/models"}
	case "google":
		return http.MethodGet, []string{baseURL + "/v1beta/models"}
	case "ollama":
		return http.MethodGet, []string{baseURL + "/api/tags"}
	case "azure-openai":
		apiVer := provider.APIVersion
		if apiVer == "" {
			apiVer = "2024-02-01"
		}
		return http.MethodGet, []string{baseURL + "/openai/deployments?api-version=" + apiVer}
	}

	// Format-based fallback — handles trial provider, custom providers, and any new providers
	switch provider.APIFormat {
	case APIFormatAnthropic:
		// Anthropic /v1/messages only accepts POST; use HEAD to avoid 405.
		// Even without a body, the server returns 401/403 (auth check) which we treat as "reachable".
		return http.MethodPost, []string{baseURL + "/v1/messages"}
	case APIFormatGoogle:
		return http.MethodGet, []string{baseURL + "/v1beta/models"}
	case APIFormatOllama:
		return http.MethodGet, []string{baseURL + "/api/tags"}
	default:
		// OpenAI-compatible: most providers have /models endpoint
		if strings.HasSuffix(baseURL, "/v1") || strings.HasSuffix(baseURL, "/v4") {
			return http.MethodGet, []string{baseURL + "/models"}
		}
		return http.MethodGet, []string{baseURL + "/v1/models", baseURL + "/models"}
	}
}

// addAuthHeader adds the appropriate authentication header for a provider.
// Uses provider ID for known providers, then falls back to API format.
func addAuthHeader(req *http.Request, provider *Provider) {
	if len(provider.APIKeys) == 0 {
		return
	}

	key := provider.APIKeys[0].Key

	// Provider-specific auth first
	switch provider.ID {
	case "anthropic":
		req.Header.Set("x-api-key", key)
		req.Header.Set("anthropic-version", provider.APIVersion)
	case "azure-openai":
		req.Header.Set("api-key", key)
	case "google":
		q := req.URL.Query()
		q.Set("key", key)
		req.URL.RawQuery = q.Encode()
	default:
		// Format-based fallback for trial and custom providers
		switch provider.APIFormat {
		case APIFormatAnthropic:
			req.Header.Set("x-api-key", key)
			if provider.APIVersion != "" {
				req.Header.Set("anthropic-version", provider.APIVersion)
			}
		case APIFormatGoogle:
			q := req.URL.Query()
			q.Set("key", key)
			req.URL.RawQuery = q.Encode()
		default:
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}

	// Add custom headers if configured
	for k, v := range provider.Headers {
		req.Header.Set(k, v)
	}
}

// CompositeHealthChecker combines multiple health check strategies
type CompositeHealthChecker struct {
	checkers []HealthChecker
}

// NewCompositeHealthChecker creates a new CompositeHealthChecker
func NewCompositeHealthChecker(checkers ...HealthChecker) *CompositeHealthChecker {
	return &CompositeHealthChecker{
		checkers: checkers,
	}
}

// Check runs all checkers and returns the first successful result
func (c *CompositeHealthChecker) Check(ctx context.Context, provider *Provider) *HealthCheckResult {
	var lastResult *HealthCheckResult

	for _, checker := range c.checkers {
		result := checker.Check(ctx, provider)
		if result.Healthy {
			return result
		}
		lastResult = result
	}

	if lastResult != nil {
		return lastResult
	}

	return &HealthCheckResult{
		ProviderID: provider.ID,
		Healthy:    false,
		Error:      "no health checkers configured",
		CheckedAt:  time.Now(),
	}
}
