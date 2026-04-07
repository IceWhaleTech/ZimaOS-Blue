package bootstrap

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
)

type auxiliarySmallModelMock struct {
	ready   bool
	resp    string
	err     error
	calls   int
	lastReq smallmodel.GenerateRequest
}

func (m *auxiliarySmallModelMock) Generate(_ context.Context, req smallmodel.GenerateRequest) (*smallmodel.GenerateResponse, error) {
	m.calls++
	m.lastReq = req
	if m.err != nil {
		return nil, m.err
	}
	return &smallmodel.GenerateResponse{Text: m.resp}, nil
}

func (m *auxiliarySmallModelMock) Ready() bool { return m.ready }

type auxiliaryFallbackMock struct {
	calls   int
	lastReq llm.ChatRequest
	resp    *llm.ChatResponse
	err     error
}

func (m *auxiliaryFallbackMock) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	m.calls++
	m.lastReq = req
	if m.err != nil {
		return nil, m.err
	}
	if m.resp != nil {
		return m.resp, nil
	}
	return &llm.ChatResponse{Message: llm.Message{Role: llm.RoleAssistant, Content: "fallback"}}, nil
}

func TestAuxiliaryLLMCallerPrefersSmallModel(t *testing.T) {
	caller := newAuxiliaryLLMCaller()
	sm := &auxiliarySmallModelMock{ready: true, resp: "local summary"}
	fallback := &auxiliaryFallbackMock{}
	caller.SetSmallModel(sm)
	caller.SetFallback(fallback)

	resp, err := caller.Chat(context.Background(), llm.ChatRequest{
		Messages:  []llm.Message{{Role: llm.RoleUser, Content: "summarize this"}},
		MaxTokens: 42,
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if sm.calls != 1 {
		t.Fatalf("small model calls = %d, want 1", sm.calls)
	}
	if fallback.calls != 0 {
		t.Fatalf("fallback calls = %d, want 0", fallback.calls)
	}
	if resp == nil || resp.Message.Content != "local summary" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if !strings.Contains(sm.lastReq.Prompt, "User:\nsummarize this") {
		t.Fatalf("small model prompt = %q, want rendered user message", sm.lastReq.Prompt)
	}
}

func TestAuxiliaryLLMCallerFallsBackWhenSmallModelUnavailable(t *testing.T) {
	caller := newAuxiliaryLLMCaller()
	sm := &auxiliarySmallModelMock{ready: false}
	fallback := &auxiliaryFallbackMock{resp: &llm.ChatResponse{Message: llm.Message{Role: llm.RoleAssistant, Content: "proxy result"}}}
	caller.SetSmallModel(sm)
	caller.SetFallback(fallback)

	resp, err := caller.Chat(context.Background(), llm.ChatRequest{
		Model:    "auto",
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "reflect on this"}},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if sm.calls != 0 {
		t.Fatalf("small model calls = %d, want 0", sm.calls)
	}
	if fallback.calls != 1 {
		t.Fatalf("fallback calls = %d, want 1", fallback.calls)
	}
	if resp == nil || resp.Message.Content != "proxy result" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestAuxiliaryLLMCallerBypassesSmallModelWhenProviderPinned(t *testing.T) {
	caller := newAuxiliaryLLMCaller()
	sm := &auxiliarySmallModelMock{ready: true, resp: "local summary"}
	fallback := &auxiliaryFallbackMock{resp: &llm.ChatResponse{Message: llm.Message{Role: llm.RoleAssistant, Content: "provider result"}}}
	caller.SetSmallModel(sm)
	caller.SetFallback(fallback)

	resp, err := caller.Chat(proxy.WithPinnedProvider(context.Background(), "openai-prod"), llm.ChatRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "rewrite this page"}},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if sm.calls != 0 {
		t.Fatalf("small model calls = %d, want 0", sm.calls)
	}
	if fallback.calls != 1 {
		t.Fatalf("fallback calls = %d, want 1", fallback.calls)
	}
	if resp == nil || resp.Message.Content != "provider result" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestAuxiliaryLLMCallerDoesNotFallbackWhenSmallModelPinned(t *testing.T) {
	caller := newAuxiliaryLLMCaller()
	sm := &auxiliarySmallModelMock{ready: false}
	fallback := &auxiliaryFallbackMock{resp: &llm.ChatResponse{Message: llm.Message{Role: llm.RoleAssistant, Content: "provider result"}}}
	caller.SetSmallModel(sm)
	caller.SetFallback(fallback)

	_, err := caller.Chat(proxy.WithPinnedProvider(context.Background(), "smallmodel"), llm.ChatRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "rewrite this page"}},
	})
	if !errors.Is(err, smallmodel.ErrNotReady) {
		t.Fatalf("Chat() error = %v, want %v", err, smallmodel.ErrNotReady)
	}
	if sm.calls != 0 {
		t.Fatalf("small model calls = %d, want 0", sm.calls)
	}
	if fallback.calls != 0 {
		t.Fatalf("fallback calls = %d, want 0", fallback.calls)
	}
}

func TestAuxiliaryLLMCallerFallsBackOnUnsupportedSmallModelRequest(t *testing.T) {
	caller := newAuxiliaryLLMCaller()
	sm := &auxiliarySmallModelMock{ready: true, resp: "should not be used"}
	fallback := &auxiliaryFallbackMock{}
	caller.SetSmallModel(sm)
	caller.SetFallback(fallback)

	_, err := caller.Chat(context.Background(), llm.ChatRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "use tool"}},
		Tools:    []llm.Tool{{Name: "memory_search"}},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if sm.calls != 0 {
		t.Fatalf("small model calls = %d, want 0", sm.calls)
	}
	if fallback.calls != 1 {
		t.Fatalf("fallback calls = %d, want 1", fallback.calls)
	}
}

func TestAuxiliaryLLMCallerFallsBackOnSmallModelError(t *testing.T) {
	caller := newAuxiliaryLLMCaller()
	sm := &auxiliarySmallModelMock{ready: true, err: errors.New("local model failed")}
	fallback := &auxiliaryFallbackMock{}
	caller.SetSmallModel(sm)
	caller.SetFallback(fallback)

	_, err := caller.Chat(context.Background(), llm.ChatRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "extract memory"}},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if sm.calls != 1 {
		t.Fatalf("small model calls = %d, want 1", sm.calls)
	}
	if fallback.calls != 1 {
		t.Fatalf("fallback calls = %d, want 1", fallback.calls)
	}
}

func TestRegisterLLMProvidersRequiresExplicitOllamaConfig(t *testing.T) {
	t.Run("skips ollama by default", func(t *testing.T) {
		registry := llm.NewProviderRegistry()
		registerLLMProviders(registry, &config.Config{})
		for _, name := range registry.List() {
			if name == "ollama" {
				t.Fatalf("unexpected ollama registration without explicit config")
			}
		}
	})

	t.Run("registers ollama when OLLAMA_URL is set", func(t *testing.T) {
		t.Setenv("OLLAMA_URL", "http://localhost:11434")
		registry := llm.NewProviderRegistry()
		registerLLMProviders(registry, &config.Config{})
		found := false
		for _, name := range registry.List() {
			if name == "ollama" {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("expected ollama to be registered when OLLAMA_URL is set")
		}
	})
}
