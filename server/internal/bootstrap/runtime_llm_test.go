package bootstrap

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

func TestRuntimeLLMProviderRef_UsesConfiguredProvider(t *testing.T) {
	proxyProvider := &recordingProvider{name: "proxy"}
	ref := newRuntimeLLMProviderRef()
	ref.SetProvider(proxyProvider)

	resp, err := ref.Chat(context.Background(), llm.ChatRequest{
		Model:    "auto",
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if resp == nil || resp.Message.Content != "proxy" {
		t.Fatalf("Chat() response = %+v, want proxy response", resp)
	}
	if proxyProvider.calls != 1 {
		t.Fatalf("proxy calls = %d, want 1", proxyProvider.calls)
	}
}

func TestRuntimeLLMProviderRef_ReturnsUnavailableWithoutProvider(t *testing.T) {
	ref := newRuntimeLLMProviderRef()
	if _, err := ref.Chat(context.Background(), llm.ChatRequest{}); err == nil {
		t.Fatal("expected error when runtime provider is missing")
	}
}

func TestResolveDefaultRuntimeModel_RespectsAutoAndFallback(t *testing.T) {
	if got := resolveDefaultRuntimeModel("auto", nil); got != "auto" {
		t.Fatalf("resolveDefaultRuntimeModel(auto) = %q, want auto", got)
	}
	if got := resolveDefaultRuntimeModel("", nil); got != defaultRuntimeModel {
		t.Fatalf("resolveDefaultRuntimeModel(empty) = %q, want %q", got, defaultRuntimeModel)
	}
	if got := resolveDefaultRuntimeModel("gpt-4.1", nil); got != "gpt-4.1" {
		t.Fatalf("resolveDefaultRuntimeModel(explicit) = %q, want explicit model", got)
	}
}
