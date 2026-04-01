package bootstrap

import (
	"context"
	"errors"
	"fmt"
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

func (c *auxiliaryLLMCaller) Name() string {
	return "smallmodel"
}

func (c *auxiliaryLLMCaller) Models() []string {
	return []string{smallmodel.ModelID}
}

func (c *auxiliaryLLMCaller) SetSmallModel(rt smallmodel.Runtime) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.smallModel = rt
}

func (c *auxiliaryLLMCaller) SetFallback(fallback llmChatCaller) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fallback = fallback
}

func (c *auxiliaryLLMCaller) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	c.mu.RLock()
	runtime := c.smallModel
	fallback := c.fallback
	c.mu.RUnlock()

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

func (c *auxiliaryLLMCaller) ChatStream(_ context.Context, _ llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	return nil, fmt.Errorf("auxiliary llm does not support streaming")
}

func (c *auxiliaryLLMCaller) ChatStreamCallback(_ context.Context, _ llm.ChatRequest, _ llm.StreamCallback) error {
	return fmt.Errorf("auxiliary llm does not support streaming")
}
