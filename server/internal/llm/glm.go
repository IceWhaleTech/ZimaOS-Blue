package llm

import (
	"time"
)

const (
	defaultGLMBaseURL = "https://open.bigmodel.cn/api/paas"
	glmTimeout        = 120 * time.Second
)

// GLMProvider implements the Provider interface for Zhipu AI's GLM.
// GLM uses an OpenAI-compatible API, so we embed OpenAIProvider.
type GLMProvider struct {
	*OpenAIProvider
}

// NewGLMProvider creates a new GLM provider.
func NewGLMProvider(apiKey, baseURL string) *GLMProvider {
	if baseURL == "" {
		baseURL = defaultGLMBaseURL
	}

	openaiProvider := NewOpenAIProvider(apiKey, baseURL)

	return &GLMProvider{
		OpenAIProvider: openaiProvider,
	}
}

// Name returns the provider name.
func (p *GLMProvider) Name() string {
	return "glm"
}

// getDefaultModels returns the default model list for GLM.
func (p *GLMProvider) getDefaultModels() []string {
	return []string{
		"glm-4-plus",
		"glm-4-air",
		"glm-4-airx",
		"glm-4-flash",
		"glm-4-long",
		"glm-4v-plus",
		"glm-4v",
		"glm-4-0520",
		"glm-4",
	}
}

// Models returns the list of available models.
func (p *GLMProvider) Models() []string {
	// Try to fetch models from API
	models := p.fetchModels()
	if len(models) > 0 {
		return models
	}
	// Return default fallback list for GLM
	return p.getDefaultModels()
}
