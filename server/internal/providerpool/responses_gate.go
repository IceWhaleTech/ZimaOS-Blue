package providerpool

import (
	"errors"
	"net/url"
	"strings"
	"sync/atomic"
)

const responsesIntegrationDisabledReason = "responses compatibility/integration is temporarily disabled; planned to move into Codex CLI later"

var responsesIntegrationEnabled atomic.Bool

func init() {
	responsesIntegrationEnabled.Store(true)
}

func SetResponsesIntegrationEnabled(enabled bool) {
	responsesIntegrationEnabled.Store(enabled)
}

func ResponsesIntegrationEnabled() bool {
	return responsesIntegrationEnabled.Load()
}

func ResponsesIntegrationDisabledReason() string {
	return responsesIntegrationDisabledReason
}

func UsesResponsesIntegration(provider *Provider) bool {
	if provider == nil {
		return false
	}
	if provider.OAuth != nil && strings.EqualFold(strings.TrimSpace(provider.OAuth.ProviderType), "codex") {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(provider.ID), "openai-codex") {
		return true
	}
	return isCodexResponsesIntegrationBaseURL(provider.BaseURL) ||
		isCodexResponsesIntegrationBaseURL(provider.EffectiveBaseURL()) ||
		isCodexResponsesIntegrationBaseURL(provider.DetectedEndpoint)
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

func validateResponsesIntegrationAllowed(provider *Provider) error {
	if ResponsesIntegrationEnabled() || !UsesResponsesIntegration(provider) {
		return nil
	}
	return errors.New(responsesIntegrationDisabledReason)
}

func (p *Pool) DisableResponsesProviders() int {
	if p == nil || p.Registry == nil {
		return 0
	}

	disabled := 0
	for _, provider := range p.Registry.List() {
		if !UsesResponsesIntegration(provider) {
			continue
		}
		changed := false
		if provider.Enabled {
			provider.Enabled = false
			changed = true
		}
		if provider.Status != ProviderStatusInactive {
			provider.Status = ProviderStatusInactive
			changed = true
		}
		if !changed {
			continue
		}
		if err := p.Registry.Update(provider); err == nil {
			disabled++
		}
	}

	return disabled
}
