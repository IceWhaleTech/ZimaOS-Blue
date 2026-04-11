package providerpool

import (
	"net/url"
	"strings"
)

// UsesResponsesIntegration reports whether a provider targets the Codex/Responses-native
// integration path. This is used for routing heuristics and warmup behavior, not gating.
func UsesResponsesIntegration(provider *Provider) bool {
	if provider == nil {
		return false
	}
	if provider.APIFormat == APIFormatResponses || provider.DetectedFormat == APIFormatResponses {
		return true
	}
	if provider.OAuth != nil && strings.EqualFold(strings.TrimSpace(provider.OAuth.ProviderType), "codex") {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(provider.ID), "openai-codex") {
		return true
	}
	return isCodexResponsesIntegrationBaseURL(provider.BaseURL) ||
		isCodexResponsesIntegrationBaseURL(provider.EffectiveBaseURL()) ||
		isCodexResponsesIntegrationBaseURL(provider.DetectedEndpoint) ||
		isResponsesEndpointBaseURL(provider.BaseURL) ||
		isResponsesEndpointBaseURL(provider.EffectiveBaseURL()) ||
		isResponsesEndpointBaseURL(provider.DetectedEndpoint)
}

func isCodexResponsesIntegrationBaseURL(raw string) bool {
	if strings.TrimSpace(raw) == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		path := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(raw)), "/")
		return path == "/backend-api/codex/responses" || strings.HasSuffix(path, "/backend-api/codex/responses")
	}
	path := strings.TrimSuffix(strings.ToLower(u.Path), "/")
	return path == "/backend-api/codex/responses" || strings.HasSuffix(path, "/backend-api/codex/responses")
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
