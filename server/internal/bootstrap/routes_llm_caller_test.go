package bootstrap

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
)

type recordingProvider struct {
	name   string
	models []string

	calls   int
	lastReq llm.ChatRequest
}

type scriptedProvider struct {
	name    string
	models  []string
	results []scriptedProviderResult

	calls   int
	lastReq llm.ChatRequest
}

type scriptedProviderResult struct {
	response *llm.ChatResponse
	err      error
}

func (p *recordingProvider) Name() string {
	return p.name
}

func (p *recordingProvider) Models() []string {
	out := make([]string, len(p.models))
	copy(out, p.models)
	return out
}

func (p *recordingProvider) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	p.calls++
	p.lastReq = req
	return &llm.ChatResponse{
		Model: req.Model,
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: p.name,
		},
	}, nil
}

func (p *scriptedProvider) Name() string {
	return p.name
}

func (p *scriptedProvider) Models() []string {
	out := make([]string, len(p.models))
	copy(out, p.models)
	return out
}

func (p *scriptedProvider) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	p.calls++
	p.lastReq = req
	if idx := p.calls - 1; idx >= 0 && idx < len(p.results) {
		result := p.results[idx]
		if result.err != nil {
			return nil, result.err
		}
		if result.response != nil {
			return result.response, nil
		}
	}
	return &llm.ChatResponse{
		Model: req.Model,
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: p.name,
		},
	}, nil
}

func (p *scriptedProvider) ChatStream(_ context.Context, _ llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	return nil, errors.New("not implemented")
}

func (p *scriptedProvider) ChatStreamCallback(_ context.Context, _ llm.ChatRequest, _ llm.StreamCallback) error {
	return errors.New("not implemented")
}

func (p *recordingProvider) ChatStream(_ context.Context, _ llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	return nil, errors.New("not implemented")
}

func (p *recordingProvider) ChatStreamCallback(_ context.Context, _ llm.ChatRequest, _ llm.StreamCallback) error {
	return errors.New("not implemented")
}

func TestProviderRegistryLLMCaller_AutoModelUsesFirstProviderModel(t *testing.T) {
	registry := llm.NewProviderRegistry()
	first := &recordingProvider{name: "first", models: []string{"first-model"}}
	second := &recordingProvider{name: "second", models: []string{"second-model"}}
	registry.Register(first)
	registry.Register(second)

	caller := newProviderRegistryLLMCaller(registry)
	resp, err := caller.Chat(context.Background(), llm.ChatRequest{
		Model:    "auto",
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil || resp.Message.Content != "first" {
		t.Fatalf("expected first provider response, got %+v", resp)
	}
	if first.calls != 1 {
		t.Fatalf("expected first provider to be called once, got %d", first.calls)
	}
	if first.lastReq.Model != "first-model" {
		t.Fatalf("expected auto model to be replaced with first-model, got %q", first.lastReq.Model)
	}
}

func TestProviderRegistryLLMCaller_SelectsProviderByModel(t *testing.T) {
	registry := llm.NewProviderRegistry()
	first := &recordingProvider{name: "first", models: []string{"first-model"}}
	second := &recordingProvider{name: "second", models: []string{"second-model"}}
	registry.Register(first)
	registry.Register(second)

	caller := newProviderRegistryLLMCaller(registry)
	resp, err := caller.Chat(context.Background(), llm.ChatRequest{
		Model:    "second-model",
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil || resp.Message.Content != "second" {
		t.Fatalf("expected second provider response, got %+v", resp)
	}
	if second.calls != 1 {
		t.Fatalf("expected second provider to be called once, got %d", second.calls)
	}
	if first.calls != 0 {
		t.Fatalf("expected first provider not to be called, got %d", first.calls)
	}
}

func TestProviderRegistryLLMCaller_NoProviders(t *testing.T) {
	caller := newProviderRegistryLLMCaller(llm.NewProviderRegistry())
	_, err := caller.Chat(context.Background(), llm.ChatRequest{
		Model:    "auto",
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}},
	})
	if err == nil {
		t.Fatal("expected error when no providers are configured")
	}
}

func TestProviderRegistryLLMCaller_RetriesRetryableProxyErrors(t *testing.T) {
	registry := llm.NewProviderRegistry()
	provider := &scriptedProvider{
		name:   "retryable",
		models: []string{"retry-model"},
		results: []scriptedProviderResult{
			{err: &proxybridge.ProxyError{StatusCode: 502, Body: "upstream overloaded"}},
			{response: &llm.ChatResponse{
				Model: "retry-model",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "recovered",
				},
			}},
		},
	}
	registry.Register(provider)

	caller := newProviderRegistryLLMCaller(registry)
	caller.retry.maxRetries = 1
	caller.retry.sleep = func(context.Context, time.Duration) error { return nil }

	resp, err := caller.Chat(context.Background(), llm.ChatRequest{
		Model:    "retry-model",
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil || resp.Message.Content != "recovered" {
		t.Fatalf("response = %+v, want recovered reply", resp)
	}
	if provider.calls != 2 {
		t.Fatalf("provider calls = %d, want 2", provider.calls)
	}
}

func TestProxyBridgeLLMCaller_NilBridge(t *testing.T) {
	caller := &proxyBridgeLLMCaller{}
	_, err := caller.Chat(context.Background(), llm.ChatRequest{Model: "auto"})
	if err == nil {
		t.Fatal("expected error when proxy bridge is nil")
	}
}

func TestResolveDefaultRuntimeModel_RespectsExplicitAuto(t *testing.T) {
	if got := resolveDefaultRuntimeModel("auto", nil); got != "auto" {
		t.Fatalf("resolveDefaultRuntimeModel(auto) = %q, want auto", got)
	}
	if got := resolveDefaultRuntimeModel("", nil); got != defaultRuntimeModel {
		t.Fatalf("resolveDefaultRuntimeModel(empty) = %q, want %q", got, defaultRuntimeModel)
	}
}
