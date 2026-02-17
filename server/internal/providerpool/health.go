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

	// Skip if no base URL
	if provider.BaseURL == "" {
		result.Healthy = false
		result.Error = "no base URL configured"
		return result
	}

	// Build health check URLs (try in order, first success wins)
	healthURLs := getHealthCheckURLs(provider)

	// Pick client based on provider TLS setting
	httpClient := c.client
	if provider.SkipTLSVerify {
		httpClient = c.insecureClient
	}

	var lastErr string
	for _, healthURL := range healthURLs {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
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

		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			result.Healthy = true
			return result
		} else if resp.StatusCode == 401 || resp.StatusCode == 403 {
			result.Healthy = false
			result.Error = fmt.Sprintf("authentication error: %d", resp.StatusCode)
			return result
		}
		lastErr = fmt.Sprintf("unexpected status: %d", resp.StatusCode)
	}

	result.Healthy = false
	result.Error = lastErr
	return result
}

// sanitizeHealthError converts raw network errors to user-friendly messages.
func sanitizeHealthError(err error) string {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "no such host") || strings.Contains(msg, "dial tcp") ||
		strings.Contains(msg, "connection refused") {
		return "network connection failed"
	}
	if strings.Contains(msg, "x509") || strings.Contains(msg, "certificate") {
		return "secure connection failed (certificate error)"
	}
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded") {
		return "request timed out"
	}
	return "connection failed"
}

// getHealthCheckURLs returns the health check URLs to try for a provider (first match wins)
func getHealthCheckURLs(provider *Provider) []string {
	baseURL := strings.TrimSuffix(provider.BaseURL, "/")

	switch provider.ID {
	case "openai", "deepseek", "moonshot", "openrouter", "aihubmix":
		return []string{baseURL + "/models"}
	case "anthropic":
		return []string{baseURL + "/v1/messages"}
	case "google":
		return []string{baseURL + "/v1beta/models"}
	case "ollama":
		return []string{baseURL + "/api/tags"}
	case "azure-openai":
		return []string{baseURL + "/openai/deployments?api-version=" + provider.APIVersion}
	default:
		// For custom providers, try /models then /v1/models
		if strings.HasSuffix(baseURL, "/v1") {
			return []string{baseURL + "/models"}
		}
		return []string{baseURL + "/models", baseURL + "/v1/models"}
	}
}

// addAuthHeader adds the appropriate authentication header for a provider
func addAuthHeader(req *http.Request, provider *Provider) {
	if len(provider.APIKeys) == 0 {
		return
	}

	key := provider.APIKeys[0].Key

	switch provider.ID {
	case "anthropic":
		req.Header.Set("x-api-key", key)
		req.Header.Set("anthropic-version", provider.APIVersion)
	case "azure-openai":
		req.Header.Set("api-key", key)
	case "google":
		// Google uses query parameter for API key
		q := req.URL.Query()
		q.Set("key", key)
		req.URL.RawQuery = q.Encode()
	default:
		// Most providers use Bearer token
		req.Header.Set("Authorization", "Bearer "+key)
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
