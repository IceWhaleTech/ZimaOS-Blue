package llm

import (
	"time"
)

const (
	defaultGrokBaseURL = "https://api.x.ai"
	grokTimeout        = 120 * time.Second
)

// GrokProvider implements the Provider interface for xAI's Grok.
// Grok uses an OpenAI-compatible API, so we embed OpenAIProvider.
type GrokProvider struct {
	*OpenAIProvider
}

// NewGrokProvider creates a new Grok provider.
func NewGrokProvider(apiKey, baseURL string) *GrokProvider {
	if baseURL == "" {
		baseURL = defaultGrokBaseURL
	}

	openaiProvider := NewOpenAIProvider(apiKey, baseURL)

	return &GrokProvider{
		OpenAIProvider: openaiProvider,
	}
}

// Name returns the provider name.
func (p *GrokProvider) Name() string {
	return "grok"
}

// getDefaultModels returns the default model list for Grok.
func (p *GrokProvider) getDefaultModels() []string {
	return []string{
		"grok-beta",
		"grok-vision-beta",
	}
}

// Models returns the list of available models.
func (p *GrokProvider) Models() []string {
	// Try to fetch models from API
	models := p.fetchModels()
	if len(models) > 0 {
		return models
	}
	// Return default fallback list for Grok
	return p.getDefaultModels()
}
