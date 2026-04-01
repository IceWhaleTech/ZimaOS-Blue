package bootstrap

import (
	"context"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

type runtimeLLMProviderRef struct {
	mu       sync.RWMutex
	provider llm.Provider
	retry    llmChatRetryConfig
}

func newRuntimeLLMProviderRef() *runtimeLLMProviderRef {
	return &runtimeLLMProviderRef{retry: defaultLLMChatRetryConfig()}
}

func (r *runtimeLLMProviderRef) SetProvider(provider llm.Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.provider = provider
}

func (r *runtimeLLMProviderRef) currentProvider() llm.Provider {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.provider
}

func (r *runtimeLLMProviderRef) Name() string {
	if provider := r.currentProvider(); provider != nil {
		return provider.Name()
	}
	return "runtime"
}

func (r *runtimeLLMProviderRef) Models() []string {
	if provider := r.currentProvider(); provider != nil {
		return provider.Models()
	}
	return nil
}

func (r *runtimeLLMProviderRef) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	provider := r.currentProvider()
	if provider == nil {
		return nil, errRuntimeLLMUnavailable
	}
	return r.retry.withDefaults().chat(ctx, req, provider.Chat)
}

func (r *runtimeLLMProviderRef) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	provider := r.currentProvider()
	if provider == nil {
		return nil, errRuntimeLLMUnavailable
	}
	return provider.ChatStream(ctx, req)
}

func (r *runtimeLLMProviderRef) ChatStreamCallback(ctx context.Context, req llm.ChatRequest, callback llm.StreamCallback) error {
	provider := r.currentProvider()
	if provider == nil {
		return errRuntimeLLMUnavailable
	}
	return provider.ChatStreamCallback(ctx, req, callback)
}
