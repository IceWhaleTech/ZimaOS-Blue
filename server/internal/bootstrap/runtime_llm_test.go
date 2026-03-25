package bootstrap

import (
	"context"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/claudecode"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

func TestRuntimeDispatchProvider_UsesProxyWhenClaudeCodeDisabled(t *testing.T) {
	proxyProvider := &recordingProvider{name: "proxy"}
	dispatch := newRuntimeDispatchProvider(proxyProvider, nil, nil)

	resp, err := dispatch.Chat(context.Background(), llm.ChatRequest{
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
	if proxyProvider.lastReq.Model != "auto" {
		t.Fatalf("proxy model = %q, want auto", proxyProvider.lastReq.Model)
	}
}

func TestRuntimeDispatchProvider_EnabledDoesNotSilentlyFallBackToProxy(t *testing.T) {
	cc := claudecode.NewHandlerWithDataDir(nil, "", kvstore.NewMemoryStore())
	proxyProvider := &recordingProvider{name: "proxy"}
	dispatch := newRuntimeDispatchProvider(proxyProvider, cc, nil)

	_, err := dispatch.Chat(context.Background(), llm.ChatRequest{
		Model:    "auto",
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}},
	})
	if err == nil {
		t.Fatal("expected error when Claude Code is enabled but runtime factory is missing")
	}
	if !strings.Contains(err.Error(), "claude code runtime is not configured") {
		t.Fatalf("error = %v, want Claude Code runtime configuration error", err)
	}
	if proxyProvider.calls != 0 {
		t.Fatalf("proxy calls = %d, want 0", proxyProvider.calls)
	}
}

func TestRuntimeDispatchProvider_ActiveProviderUsesClaudeCodeWhenEnabled(t *testing.T) {
	cc := claudecode.NewHandlerWithDataDir(nil, "", kvstore.NewMemoryStore())
	proxyProvider := &recordingProvider{name: "proxy"}
	factory := newClaudeCodeRuntimeFactory(cc, nil, 23456, nil, "", nil, nil)
	dispatch := newRuntimeDispatchProvider(proxyProvider, cc, factory)

	provider, err := dispatch.activeProvider()
	if err != nil {
		t.Fatalf("activeProvider() error = %v", err)
	}
	if provider == nil {
		t.Fatal("activeProvider() returned nil provider")
	}
	if got := provider.Name(); got != "claude-code" {
		t.Fatalf("activeProvider().Name() = %q, want claude-code", got)
	}
}

func TestRuntimeDispatchProvider_NormalizeRequestForClaudeCode(t *testing.T) {
	cc := claudecode.NewHandlerWithDataDir(nil, "", kvstore.NewMemoryStore())
	dispatch := newRuntimeDispatchProvider(nil, cc, nil)

	cases := []struct {
		name  string
		model string
		want  string
	}{
		{name: "empty", model: "", want: ""},
		{name: "auto", model: "auto", want: ""},
		{name: "default model", model: defaultCCCLIModel, want: ""},
		{name: "explicit model", model: "claude-sonnet-4-20250514", want: "claude-sonnet-4-20250514"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := dispatch.normalizeRequest(llm.ChatRequest{Model: tc.model})
			if req.Model != tc.want {
				t.Fatalf("normalizeRequest(%q) = %q, want %q", tc.model, req.Model, tc.want)
			}
		})
	}
}
