package bootstrap

import (
	"context"
	"errors"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
)

var (
	errAuxiliaryLLMUnavailable = errors.New("auxiliary llm backend not available")
	errSmallModelUnsupported   = errors.New("small model request unsupported")
)

type llmChatCaller interface {
	Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)
}

type auxiliaryLLMCaller struct {
	mu         sync.RWMutex
	smallModel smallmodel.Runtime
	fallback   llmChatCaller
}

func newAuxiliaryLLMCaller() *auxiliaryLLMCaller {
	return &auxiliaryLLMCaller{}
}

func (c *auxiliaryLLMCaller) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	c.mu.RLock()
	runtime := c.smallModel
	fallback := c.fallback
	c.mu.RUnlock()

	pinnedProviderID := normalizedPinnedProviderID(ctx)
	switch {
	case pinnedProviderID == smallmodelProviderID:
		if runtime == nil || !runtime.Ready() {
			return nil, smallmodel.ErrNotReady
		}
		return callAuxiliarySmallModel(ctx, runtime, req)
	case pinnedProviderID != "":
		if fallback == nil {
			return nil, errAuxiliaryLLMUnavailable
		}
		return fallback.Chat(ctx, req)
	}

	if runtime != nil && runtime.Ready() {
		resp, err := callAuxiliarySmallModel(ctx, runtime, req)
		if err == nil {
			return resp, nil
		}
		if !shouldFallbackFromSmallModel(err) || fallback == nil {
			return nil, err
		}
	}

	if fallback == nil {
		return nil, errAuxiliaryLLMUnavailable
	}
	return fallback.Chat(ctx, req)
}
