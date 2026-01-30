package providerpool

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// HTTPHealthChecker performs HTTP-based health checks
type HTTPHealthChecker struct {
	client *http.Client
}

// NewHTTPHealthChecker creates a new HTTPHealthChecker
func NewHTTPHealthChecker(timeout time.Duration) *HTTPHealthChecker {
	return &HTTPHealthChecker{
		client: &http.Client{
			Timeout: timeout,
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

	// Build health check URL
	healthURL := getHealthCheckURL(provider)

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		result.Healthy = false
		result.Error = fmt.Sprintf("failed to create request: %v", err)
		return result
	}

	// Add authentication if available
	if len(provider.APIKeys) > 0 && provider.APIKeys[0].Key != "" {
		addAuthHeader(req, provider)
	}

	// Perform request
	resp, err := c.client.Do(req)
	if err != nil {
		result.Healthy = false
		result.Error = fmt.Sprintf("request failed: %v", err)
		result.Latency = time.Since(start)
		return result
	}
	defer resp.Body.Close()

	result.Latency = time.Since(start)

	// Check status code
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		result.Healthy = true
	} else if resp.StatusCode == 401 || resp.StatusCode == 403 {
		// Auth error - provider is reachable but credentials may be invalid
		result.Healthy = false
		result.Error = fmt.Sprintf("authentication error: %d", resp.StatusCode)
	} else {
		result.Healthy = false
		result.Error = fmt.Sprintf("unexpected status: %d", resp.StatusCode)
	}

	return result
}

// getHealthCheckURL returns the appropriate health check URL for a provider
func getHealthCheckURL(provider *Provider) string {
	baseURL := provider.BaseURL

	switch provider.ID {
	case "openai", "deepseek", "moonshot", "openrouter", "aihubmix":
		return baseURL + "/models"
	case "anthropic":
		// Anthropic doesn't have a dedicated health endpoint
		// We'll use the messages endpoint with a minimal request
		return baseURL + "/v1/messages"
	case "google":
		return baseURL + "/v1beta/models"
	case "ollama":
		return baseURL + "/api/tags"
	case "azure-openai":
		// Azure requires deployment-specific endpoint
		return baseURL + "/openai/deployments?api-version=" + provider.APIVersion
	default:
		// For custom providers, try /models endpoint
		return baseURL + "/models"
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
