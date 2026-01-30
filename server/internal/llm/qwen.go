package llm

import (
	"time"
)

const (
	defaultQwenBaseURL = "https://dashscope.aliyuncs.com/compatible-mode"
	qwenTimeout        = 120 * time.Second
)

// QwenProvider implements the Provider interface for Alibaba Cloud's Qwen.
// Qwen uses an OpenAI-compatible API, so we embed OpenAIProvider.
type QwenProvider struct {
	*OpenAIProvider
}

// NewQwenProvider creates a new Qwen provider.
func NewQwenProvider(apiKey, baseURL string) *QwenProvider {
	if baseURL == "" {
		baseURL = defaultQwenBaseURL
	}

	openaiProvider := NewOpenAIProvider(apiKey, baseURL)

	return &QwenProvider{
		OpenAIProvider: openaiProvider,
	}
}

// Name returns the provider name.
func (p *QwenProvider) Name() string {
	return "qwen"
}

// getDefaultModels returns the default model list for Qwen.
func (p *QwenProvider) getDefaultModels() []string {
	return []string{
		"qwen-turbo",
		"qwen-plus",
		"qwen-max",
		"qwen-max-longcontext",
		"qwen-vl-plus",
		"qwen-vl-max",
		"qwen2.5-72b-instruct",
		"qwen2.5-32b-instruct",
		"qwen2.5-14b-instruct",
		"qwen2.5-7b-instruct",
	}
}

// Models returns the list of available models.
func (p *QwenProvider) Models() []string {
	// Try to fetch models from API
	models := p.fetchModels()
	if len(models) > 0 {
		return models
	}
	// Return default fallback list for Qwen
	return p.getDefaultModels()
}
