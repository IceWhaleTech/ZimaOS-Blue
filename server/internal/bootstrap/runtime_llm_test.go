package bootstrap

import (
	"context"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
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

func TestRuntimeLLMProviderRef_DoesNotRetryAuthFailures(t *testing.T) {
	proxyProvider := &scriptedProvider{
		name:   "proxy",
		models: []string{"proxy-model"},
		results: []scriptedProviderResult{
			{err: &proxybridge.ProxyError{StatusCode: 401, Body: `{"error":"invalid_api_key"}`}},
		},
	}
	ref := newRuntimeLLMProviderRef()
	ref.retry.maxRetries = 2
	ref.retry.sleep = func(context.Context, time.Duration) error { return nil }
	ref.SetProvider(proxyProvider)

	_, err := ref.Chat(context.Background(), llm.ChatRequest{
		Model:    "proxy-model",
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}},
	})
	if err == nil {
		t.Fatal("expected auth failure to be returned")
	}
	if proxyProvider.calls != 1 {
		t.Fatalf("proxy calls = %d, want 1", proxyProvider.calls)
	}
}

func TestRuntimeLLMProviderRef_DefaultNameAndModelsWithoutProvider(t *testing.T) {
	ref := newRuntimeLLMProviderRef()
	if got := ref.Name(); got != "runtime" {
		t.Fatalf("Name() = %q, want runtime", got)
	}
	if got := ref.Models(); got != nil {
		t.Fatalf("Models() = %#v, want nil", got)
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

func TestShouldUseDefaultRuntimeModel_DefaultsTrueWithoutDiscovery(t *testing.T) {
	if !shouldUseDefaultRuntimeModel(&providerpool.Pool{}, defaultRuntimeModel) {
		t.Fatal("expected default runtime model to be allowed without discovery")
	}
	if shouldUseDefaultRuntimeModel(nil, "") {
		t.Fatal("expected empty model to be rejected")
	}
}
