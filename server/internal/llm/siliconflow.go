package llm

import (
	"time"
)

const (
	defaultSiliconFlowBaseURL = "https://api.siliconflow.cn/v1"
	siliconFlowTimeout        = 120 * time.Second
)

// SiliconFlowProvider implements the Provider interface for SiliconFlow (硅基流动).
// SiliconFlow uses an OpenAI-compatible API, so we embed OpenAIProvider.
type SiliconFlowProvider struct {
	*OpenAIProvider
}

// NewSiliconFlowProvider creates a new SiliconFlow provider.
func NewSiliconFlowProvider(apiKey, baseURL string) *SiliconFlowProvider {
	if baseURL == "" {
		baseURL = defaultSiliconFlowBaseURL
	}

	openaiProvider := NewOpenAIProvider(apiKey, baseURL)

	return &SiliconFlowProvider{
		OpenAIProvider: openaiProvider,
	}
}

// Name returns the provider name.
func (p *SiliconFlowProvider) Name() string {
	return "siliconflow"
}

// getDefaultModels returns the default model list for SiliconFlow.
func (p *SiliconFlowProvider) getDefaultModels() []string {
	return []string{
		"deepseek-ai/DeepSeek-V3",
		"deepseek-ai/DeepSeek-R1",
		"Qwen/Qwen2.5-72B-Instruct",
		"Qwen/Qwen2.5-32B-Instruct",
		"Qwen/Qwen2.5-7B-Instruct",
		"Qwen/QwQ-32B",
		"Pro/deepseek-ai/DeepSeek-V3",
		"Pro/deepseek-ai/DeepSeek-R1",
	}
}

// Models returns the list of available models.
func (p *SiliconFlowProvider) Models() []string {
	// Try to fetch models from API
	models := p.fetchModels()
	if len(models) > 0 {
		return models
	}
	// Return default fallback list for SiliconFlow
	return p.getDefaultModels()
}
