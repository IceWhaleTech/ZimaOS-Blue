package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

// LoadProvidersFromPool loads providers from Provider Pool and registers them in LLM registry
func LoadProvidersFromPool(pool *providerpool.Pool, llmRegistry *llm.ProviderRegistry) {
	if pool == nil || llmRegistry == nil {
		return
	}

	providers := pool.Registry.ListEnabled()
	for _, provider := range providers {
		var apiKey, baseURL string
		if len(provider.APIKeys) > 0 {
			for _, key := range provider.APIKeys {
				if key.Enabled && key.Key != "" {
					apiKey = key.Key
					break
				}
			}
		}
		baseURL = provider.BaseURL

		var llmProvider llm.Provider
		switch provider.ID {
		case "openai":
			llmProvider = llm.NewOpenAIProvider(apiKey, baseURL)
		case "anthropic":
			llmProvider = llm.NewClaudeProvider(apiKey, baseURL)
		case "ollama":
			if baseURL == "" {
				baseURL = "http://localhost:11434"
			}
			llmProvider = llm.NewOllamaProvider(baseURL)
		case "custom":
			llmProvider = llm.NewCustomProvider(apiKey, baseURL)
		case "grok":
			llmProvider = llm.NewGrokProvider(apiKey, baseURL)
		case "qwen":
			llmProvider = llm.NewQwenProvider(apiKey, baseURL)
		case "venice":
			llmProvider = llm.NewVeniceProvider(apiKey, baseURL)
		case "bedrock":
			llmProvider = llm.NewBedrockProvider(apiKey, baseURL)
		case "siliconflow":
			llmProvider = llm.NewSiliconFlowProvider(apiKey, baseURL)
		case "glm":
			llmProvider = llm.NewGLMProvider(apiKey, baseURL)
		}
		if llmProvider != nil {
			llmRegistry.Register(llmProvider)
		}
	}
}
