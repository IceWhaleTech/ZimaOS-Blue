package bootstrap

import (
	"context"
	"fmt"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
)

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

func (c *auxiliaryLLMCaller) ChatStream(_ context.Context, _ llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	return nil, fmt.Errorf("auxiliary llm does not support streaming")
}

func (c *auxiliaryLLMCaller) ChatStreamCallback(_ context.Context, _ llm.ChatRequest, _ llm.StreamCallback) error {
	return fmt.Errorf("auxiliary llm does not support streaming")
}
