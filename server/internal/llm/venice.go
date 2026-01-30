package llm

import (
	"time"
)

const (
	defaultVeniceBaseURL = "https://api.venice.ai/api"
	veniceTimeout        = 120 * time.Second
)

// VeniceProvider implements the Provider interface for Venice AI.
// Venice AI uses an OpenAI-compatible API, so we embed OpenAIProvider.
type VeniceProvider struct {
	*OpenAIProvider
}

// NewVeniceProvider creates a new Venice AI provider.
func NewVeniceProvider(apiKey, baseURL string) *VeniceProvider {
	if baseURL == "" {
		baseURL = defaultVeniceBaseURL
	}

	openaiProvider := NewOpenAIProvider(apiKey, baseURL)

	return &VeniceProvider{
		OpenAIProvider: openaiProvider,
	}
}

// Name returns the provider name.
func (p *VeniceProvider) Name() string {
	return "venice"
}

// getDefaultModels returns the default model list for Venice AI.
func (p *VeniceProvider) getDefaultModels() []string {
	return []string{
		"llama-3.3-70b",
		"deepseek-r1-llama-70b",
		"qwen-2.5-72b",
		"llama-3.2-3b",
		"dolphin-2.9.2-qwen2-72b",
		"llama-3.1-405b",
	}
}

// Models returns the list of available models.
func (p *VeniceProvider) Models() []string {
	// Try to fetch models from API
	models := p.fetchModels()
	if len(models) > 0 {
		return models
	}
	// Return default fallback list for Venice AI
	return p.getDefaultModels()
}
