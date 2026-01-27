package llm

import (
	"context"
)

// CustomProvider implements the Provider interface for custom OpenAI-compatible APIs.
// It wraps OpenAIProvider but uses a different name.
type CustomProvider struct {
	*OpenAIProvider
}

// NewCustomProvider creates a new custom OpenAI-compatible provider.
func NewCustomProvider(apiKey, baseURL string) *CustomProvider {
	return &CustomProvider{
		OpenAIProvider: NewOpenAIProvider(apiKey, baseURL),
	}
}

// Name returns the provider name.
func (p *CustomProvider) Name() string {
	return "custom"
}

// Models returns the list of available models.
// For custom providers, we return common model names that users might use.
func (p *CustomProvider) Models() []string {
	return []string{
		"default",
		"gpt-4o",
		"gpt-4o-mini",
		"gpt-4-turbo",
		"gpt-4",
		"gpt-3.5-turbo",
		"llama3",
		"llama3.1",
		"llama3.2",
		"mistral",
		"mixtral",
		"qwen2.5",
		"deepseek-chat",
		"deepseek-coder",
	}
}

// Chat sends a chat completion request.
func (p *CustomProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	return p.OpenAIProvider.Chat(ctx, req)
}

// ChatStream sends a streaming chat completion request.
func (p *CustomProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	return p.OpenAIProvider.ChatStream(ctx, req)
}
