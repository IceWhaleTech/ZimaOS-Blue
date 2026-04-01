package providerpool

import (
	"net/http"
	"net/url"
	"strings"
)

func isMiniMaxHost(host string) bool {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "api.minimaxi.com", "api.minimax.io":
		return true
	default:
		return false
	}
}

func isMiniMaxAnthropicEndpoint(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	if !isMiniMaxHost(u.Hostname()) {
		return false
	}
	path := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(u.Path)), "/")
	return strings.HasSuffix(path, "/messages") || strings.Contains(path, "/anthropic")
}

func usesMiniMaxAnthropicAuth(provider *Provider, endpoint string, format APIFormat) bool {
	if format != APIFormatAnthropic {
		return false
	}
	if provider != nil && provider.ID == "minimax" {
		return true
	}
	if isMiniMaxAnthropicEndpoint(endpoint) {
		return true
	}
	if provider != nil && isMiniMaxAnthropicEndpoint(provider.BaseURL) {
		return true
	}
	return false
}

func applyAnthropicProbeAuth(req *http.Request, apiKey string) {
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Del("Authorization")
}

func applyMiniMaxAnthropicProbeAuth(req *http.Request, apiKey string) {
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Del("x-api-key")
}
