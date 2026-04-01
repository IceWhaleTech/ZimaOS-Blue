package bootstrap

import (
	"context"
	"fmt"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
)

type proxyBridgeProvider struct {
	bridge       *proxybridge.Bridge
	providerPool *providerpool.Pool
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
