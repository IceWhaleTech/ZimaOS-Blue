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
// For custom providers, we try to fetch from the API.
// Returns empty list if API is not configured or fetch fails.
func (p *CustomProvider) Models() []string {
	// Try to fetch models from API (inherited from OpenAIProvider)
	models := p.OpenAIProvider.fetchModels()
	if len(models) > 0 {
		return models
	}
	// Return empty list - user needs to configure API key and URL first
	return []string{}
}

// RefreshModels clears the cached models and fetches fresh list.
// Returns empty list if API fetch fails.
func (p *CustomProvider) RefreshModels() []string {
	// Clear cache and fetch directly, bypassing OpenAIProvider's fallback
	p.cachedModels = nil
	models := p.fetchModels()
	if len(models) > 0 {
		return models
	}
	// Return empty list - API fetch failed
	return []string{}
}

// Chat sends a chat completion request.
func (p *CustomProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	return p.OpenAIProvider.Chat(ctx, req)
}

// ChatStream sends a streaming chat completion request.
func (p *CustomProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	return p.OpenAIProvider.ChatStream(ctx, req)
}

// ChatStreamCallback sends a streaming chat completion request and calls the callback for each chunk.
func (p *CustomProvider) ChatStreamCallback(ctx context.Context, req ChatRequest, callback StreamCallback) error {
	return p.OpenAIProvider.ChatStreamCallback(ctx, req, callback)
}
