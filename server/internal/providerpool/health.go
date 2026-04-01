package providerpool

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
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
	start := timeutil.NowTime()
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
	for i, healthURL := range healthURLs {
		reqBody := healthCheckBody(provider)
		var bodyReader io.Reader
		if reqBody != "" {
			bodyReader = strings.NewReader(reqBody)
		}
		req, err := http.NewRequestWithContext(ctx, method, healthURL, bodyReader)
		if err != nil {
			lastErr = fmt.Sprintf("failed to create request: %v", err)
			continue
		}
		if reqBody != "" {
			req.Header.Set("Content-Type", "application/json")
		}

		// Always add authentication if available
		if len(provider.APIKeys) > 0 && provider.APIKeys[0].Key != "" {
			addAuthHeader(req, provider)
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = sanitizeHealthError(err)
			result.Latency = timeutil.SinceTime(start)
			continue
		}
		resp.Body.Close()

		result.Latency = timeutil.SinceTime(start)

		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 400:
			result.Healthy = true
			if i > 0 {
				// Alternate URL worked — auto-switch BaseURL for future requests
				autoSwitchBaseURL(provider, healthURL)
			}
			return result
		case resp.StatusCode == 401 || resp.StatusCode == 403:
			// Cloud Code providers are OAuth-first. Health checker only has API keys,
			// so 401/403 without an explicit key means endpoint is reachable.
			if provider.APIFormat == APIFormatCloudCode && len(provider.APIKeys) == 0 {
				result.Healthy = true
				if i > 0 {
					autoSwitchBaseURL(provider, healthURL)
				}
				return result
			}

			// Server is reachable but credentials are wrong/missing.
			result.Healthy = false
			result.Error = fmt.Sprintf("auth_error:%d", resp.StatusCode)
			if i > 0 {
				autoSwitchBaseURL(provider, healthURL)
			}
			return result
		case resp.StatusCode == 400 || resp.StatusCode == 405:
			// 400: server validated the request and rejected it (e.g. missing body) — reachable
			// 405: Method Not Allowed — server is reachable, endpoint just doesn't support this method
			result.Healthy = true
			if i > 0 {
				autoSwitchBaseURL(provider, healthURL)
			}
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
	case "openai", "deepseek", "openrouter", "openrouter-free", "aihubmix":
		return http.MethodGet, []string{baseURL + "/models"}
	case "nvidia":
		// NVIDIA NIM public integrate endpoint is centered on chat completions.
		// Some tenants do not expose /models, which can cause false 404 health failures.
		if strings.HasSuffix(baseURL, "/chat/completions") {
			return http.MethodPost, []string{baseURL}
		}
		if strings.HasSuffix(baseURL, "/v1") {
			return http.MethodPost, []string{baseURL + "/chat/completions"}
		}
		return http.MethodPost, []string{
			baseURL + "/v1/chat/completions",
			baseURL + "/chat/completions",
		}
	case "moonshot":
		// Moonshot (Kimi) has domestic (.cn) and international (.ai) domains.
		urls := []string{baseURL + "/models"}
		if alt := alternateRegionURL(baseURL); alt != "" {
			urls = append(urls, alt+"/models")
		}
		return http.MethodGet, urls
	case "minimax":
		// MiniMax supports both Anthropic-compatible and OpenAI-compatible endpoints.
		// Keep health checks aligned with the configured endpoint family so
		// built-in /anthropic bases are probed via /v1/messages.
		method, urls := minimaxHealthCheckTargets(baseURL, provider.APIFormat)
		if alt := alternateRegionURL(baseURL); alt != "" {
			_, altURLs := minimaxHealthCheckTargets(alt, provider.APIFormat)
			urls = append(urls, altURLs...)
		}
		return method, urls
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
	case APIFormatResponses:
		if isResponsesEndpointBaseURL(baseURL) {
			return http.MethodPost, []string{baseURL}
		}
		if strings.HasSuffix(baseURL, "/v1") || strings.HasSuffix(baseURL, "/v4") {
			return http.MethodPost, []string{baseURL + "/responses"}
		}
		return http.MethodPost, []string{baseURL + "/v1/responses", baseURL + "/responses"}
	case APIFormatAnthropic:
		// Anthropic /v1/messages only accepts POST; use HEAD to avoid 405.
		// Even without a body, the server returns 401/403 (auth check) which we treat as "reachable".
		return http.MethodPost, []string{baseURL + "/v1/messages"}
	case APIFormatGoogle:
		return http.MethodGet, []string{baseURL + "/v1beta/models"}
	case APIFormatOllama:
		return http.MethodGet, []string{baseURL + "/api/tags"}
	case APIFormatCloudCode:
		// Google Cloud Code Assist (Antigravity/Gemini CLI) uses /v1internal endpoints
		return http.MethodPost, []string{baseURL + "/v1internal:loadCodeAssist"}
	default:
		// Custom OpenAI relays are considered healthy only if a chat-completions
		// style endpoint is reachable. This catches HTML edge blocks and auth
		// errors that /models can mask.
		if provider.Type == ProviderTypeCustom {
			return http.MethodPost, openAIChatHealthCheckTargets(baseURL)
		}
		// OpenAI-compatible built-ins/platforms: prefer /models to avoid
		// unnecessary probe cost when the provider catalog is stable.
		if strings.HasSuffix(baseURL, "/v1") || strings.HasSuffix(baseURL, "/v4") {
			return http.MethodGet, []string{baseURL + "/models"}
		}
		return http.MethodGet, []string{baseURL + "/v1/models", baseURL + "/models"}
	}
}

func healthCheckBody(provider *Provider) string {
	if provider == nil {
		return ""
	}
	if provider.Type != ProviderTypeCustom {
		return ""
	}
	if provider.APIFormat != "" && provider.APIFormat != APIFormatOpenAI {
		return ""
	}
	return `{}`
}

func minimaxHealthCheckTargets(baseURL string, configuredFormat APIFormat) (string, []string) {
	if format, ok := DetectEndpointFixedFormat(baseURL); ok {
		switch format {
		case APIFormatAnthropic:
			return http.MethodPost, anthropicHealthCheckTargets(baseURL)
		case APIFormatResponses:
			return http.MethodPost, responsesHealthCheckTargets(baseURL)
		}
	}
	if configuredFormat == APIFormatAnthropic {
		return http.MethodPost, anthropicHealthCheckTargets(baseURL)
	}
	return http.MethodPost, openAIChatHealthCheckTargets(baseURL)
}

func anthropicHealthCheckTargets(baseURL string) []string {
	switch {
	case strings.HasSuffix(baseURL, "/messages"), strings.HasSuffix(baseURL, "/v1/messages"):
		return []string{baseURL}
	default:
		return []string{baseURL + "/v1/messages"}
	}
}

func responsesHealthCheckTargets(baseURL string) []string {
	switch {
	case strings.HasSuffix(baseURL, "/responses"), strings.HasSuffix(baseURL, "/v1/responses"):
		return []string{baseURL}
	case strings.HasSuffix(baseURL, "/v1"), strings.HasSuffix(baseURL, "/v4"):
		return []string{baseURL + "/responses"}
	default:
		return []string{baseURL + "/v1/responses", baseURL + "/responses"}
	}
}

func openAIChatHealthCheckTargets(baseURL string) []string {
	switch {
	case strings.HasSuffix(baseURL, "/chat/completions"), strings.HasSuffix(baseURL, "/v1/chat/completions"):
		return []string{baseURL}
	case strings.HasSuffix(baseURL, "/v1"), strings.HasSuffix(baseURL, "/v4"):
		return []string{baseURL + "/chat/completions"}
	default:
		return []string{baseURL + "/v1/chat/completions"}
	}
}

func isResponsesEndpointBaseURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		path := strings.TrimSuffix(raw, "/")
		return path != "" && strings.HasSuffix(path, "/responses")
	}
	path := strings.TrimSuffix(u.Path, "/")
	return path != "" && strings.HasSuffix(path, "/responses")
}

// addAuthHeader adds the appropriate authentication header for a provider.
// Uses provider ID for known providers, then falls back to API format.
func addAuthHeader(req *http.Request, provider *Provider) {
	key := firstUsableAPIKey(provider)
	if key == nil {
		return
	}

	// Provider-specific auth first
	switch provider.ID {
	case "anthropic":
		req.Header.Set("x-api-key", key.Key)
		req.Header.Set("anthropic-version", provider.APIVersion)
	case "minimax":
		applyMiniMaxAnthropicProbeAuth(req, key.Key)
	case "azure-openai":
		req.Header.Set("api-key", key.Key)
	case "google":
		q := req.URL.Query()
		q.Set("key", key.Key)
		req.URL.RawQuery = q.Encode()
	default:
		// Format-based fallback for trial and custom providers
		switch provider.APIFormat {
		case APIFormatAnthropic:
			if usesMiniMaxAnthropicAuth(provider, req.URL.String(), provider.APIFormat) {
				applyMiniMaxAnthropicProbeAuth(req, key.Key)
			} else {
				req.Header.Set("x-api-key", key.Key)
				if provider.APIVersion != "" {
					req.Header.Set("anthropic-version", provider.APIVersion)
				}
			}
		case APIFormatGoogle:
			q := req.URL.Query()
			q.Set("key", key.Key)
			req.URL.RawQuery = q.Encode()
		default:
			req.Header.Set("Authorization", "Bearer "+key.Key)
		}
	}

	// Add custom headers if configured
	for k, v := range provider.Headers {
		req.Header.Set(k, v)
	}
}

func firstUsableAPIKey(provider *Provider) *APIKey {
	if provider == nil || len(provider.APIKeys) == 0 {
		return nil
	}

	for i := range provider.APIKeys {
		key := &provider.APIKeys[i]
		if !key.Enabled {
			continue
		}
		if strings.TrimSpace(key.Key) == "" {
			continue
		}
		return key
	}

	for i := range provider.APIKeys {
		key := &provider.APIKeys[i]
		if strings.TrimSpace(key.Key) == "" {
			continue
		}
		return key
	}

	return nil
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
		CheckedAt:  timeutil.NowTime(),
	}
}

// alternateRegionURL returns the alternate regional API base URL for providers
// that have both domestic (China) and international domains.
// Returns "" if the base URL doesn't match any known regional domain.
func alternateRegionURL(baseURL string) string {
	// Each pair: [domestic, international]
	regionPairs := [][2]string{
		{"api.minimaxi.com", "api.minimax.io"},
		{"api.moonshot.cn", "api.moonshot.ai"},
	}
	for _, pair := range regionPairs {
		if strings.Contains(baseURL, pair[0]) {
			return strings.Replace(baseURL, pair[0], pair[1], 1)
		}
		if strings.Contains(baseURL, pair[1]) {
			return strings.Replace(baseURL, pair[1], pair[0], 1)
		}
	}
	return ""
}

// autoSwitchBaseURL updates the provider's BaseURL when an alternate health check URL succeeded.
// This is used for providers with regional domains (MiniMax, Moonshot).
func autoSwitchBaseURL(provider *Provider, healthURL string) {
	switch provider.ID {
	case "minimax", "moonshot":
	default:
		return
	}
	// Extract the base URL portion (strip the endpoint path suffix)
	for _, suffix := range []string{"/v1/messages", "/messages", "/v1/chat/completions", "/chat/completions", "/models"} {
		if strings.HasSuffix(healthURL, suffix) {
			provider.BaseURL = strings.TrimSuffix(healthURL, suffix)
			return
		}
	}
}
