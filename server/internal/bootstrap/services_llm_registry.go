package bootstrap

import (
	"os"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

func registerLLMProviders(registry *llm.ProviderRegistry, cfg *config.Config) {
	registry.Register(llm.NewOpenAIProvider(os.Getenv("OPENAI_API_KEY"), ""))

	claudeKey := os.Getenv("ANTHROPIC_API_KEY")
	registry.Register(llm.NewClaudeProvider(claudeKey, ""))

	ollamaURL := strings.TrimSpace(os.Getenv("OLLAMA_URL"))
	if ollamaURL != "" {
		registry.Register(llm.NewOllamaProvider(ollamaURL))
	}
	registry.Register(llm.NewCustomProvider(os.Getenv("CUSTOM_API_KEY"), os.Getenv("CUSTOM_API_URL")))
	registry.Register(llm.NewGrokProvider(os.Getenv("GROK_API_KEY"), ""))
	registry.Register(llm.NewQwenProvider(os.Getenv("QWEN_API_KEY"), ""))
	registry.Register(llm.NewSiliconFlowProvider(os.Getenv("SILICONFLOW_API_KEY"), ""))
}
