package bootstrap

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
)

var errRuntimeLLMUnavailable = fmt.Errorf("runtime llm backend not configured")

type runtimeLLMProviderRef struct {
	mu       sync.RWMutex
	provider llm.Provider
}

func newRuntimeLLMProviderRef() *runtimeLLMProviderRef {
	return &runtimeLLMProviderRef{}
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
	return provider.Chat(ctx, req)
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

type proxyBridgeProvider struct {
	bridge       *proxybridge.Bridge
	providerPool *providerpool.Pool
}

func newProxyBridgeProvider(bridge *proxybridge.Bridge, pool *providerpool.Pool) *proxyBridgeProvider {
	return &proxyBridgeProvider{
		bridge:       bridge,
		providerPool: pool,
	}
}

func (p *proxyBridgeProvider) Name() string {
	return "proxy"
}

func (p *proxyBridgeProvider) Models() []string {
	return nil
}

func (p *proxyBridgeProvider) normalize(req *llm.ChatRequest) {
	if p == nil {
		return
	}
	req.Model = resolveDefaultRuntimeModel(req.Model, p.providerPool)
}

func (p *proxyBridgeProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if p == nil || p.bridge == nil {
		return nil, fmt.Errorf("proxy bridge is not configured")
	}
	p.normalize(&req)
	return p.bridge.Chat(ctx, req)
}

func (p *proxyBridgeProvider) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	if p == nil || p.bridge == nil {
		return nil, fmt.Errorf("proxy bridge is not configured")
	}
	p.normalize(&req)
	ch := make(chan llm.StreamChunk, 64)
	go func() {
		defer close(ch)
		_ = p.bridge.ChatStream(ctx, req, func(chunk llm.StreamChunk) error {
			ch <- chunk
			return nil
		})
	}()
	return ch, nil
}

func (p *proxyBridgeProvider) ChatStreamCallback(ctx context.Context, req llm.ChatRequest, callback llm.StreamCallback) error {
	if p == nil || p.bridge == nil {
		return fmt.Errorf("proxy bridge is not configured")
	}
	p.normalize(&req)
	return p.bridge.ChatStream(ctx, req, callback)
}

const defaultRuntimeModel = "gpt-5.3-codex-spark"

func shouldUseDefaultRuntimeModel(pool *providerpool.Pool, modelID string) bool {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return false
	}
	if pool == nil || pool.Discovery == nil {
		return true
	}
	m, _, err := pool.Discovery.FindModel(modelID)
	return err == nil && m != nil && m.Enabled
}

func resolveDefaultRuntimeModel(model string, pool *providerpool.Pool) string {
	normalized := strings.TrimSpace(model)
	if normalized == "" {
		if shouldUseDefaultRuntimeModel(pool, defaultRuntimeModel) {
			return defaultRuntimeModel
		}
		return "auto"
	}
	if strings.EqualFold(normalized, "auto") {
		return "auto"
	}
	return model
}
