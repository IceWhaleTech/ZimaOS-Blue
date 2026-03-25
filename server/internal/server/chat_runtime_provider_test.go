package server

import (
	"context"
	"net/http"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestChatOnce_PrefersRuntimeProviderOverProxyBridge(t *testing.T) {
	var proxyCalled bool
	bridge := proxybridge.NewBridge(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyCalled = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"resp_proxy","model":"auto","choices":[{"message":{"role":"assistant","content":"proxy"},"finish_reason":"stop"}]}`))
	}))

	runtimeProvider := &requestCaptureProvider{}
	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetProxyBridge(bridge)
	handler.SetRuntimeProvider(runtimeProvider)

	resp, err := handler.chatOnce(context.Background(), llm.ChatRequest{
		Model:    "auto",
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("chatOnce() error = %v", err)
	}
	if resp == nil || resp.Message.Content != "ok" {
		t.Fatalf("chatOnce() response = %+v, want runtime provider response", resp)
	}
	if proxyCalled {
		t.Fatal("proxy bridge should not be called when runtime provider is configured")
	}
	if got := runtimeProvider.LastRequest().Model; got != "auto" {
		t.Fatalf("runtime provider model = %q, want auto", got)
	}
}

func TestChatStreamCallback_PrefersRuntimeProviderOverProxyBridge(t *testing.T) {
	var proxyCalled bool
	bridge := proxybridge.NewBridge(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyCalled = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))

	runtimeProvider := &requestCaptureProvider{}
	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetProxyBridge(bridge)
	handler.SetRuntimeProvider(runtimeProvider)

	callbackCalls := 0
	err := handler.chatStreamCallback(context.Background(), llm.ChatRequest{
		Model:    "auto",
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}},
	}, func(chunk llm.StreamChunk) error {
		callbackCalls++
		return nil
	})
	if err != nil {
		t.Fatalf("chatStreamCallback() error = %v", err)
	}
	if callbackCalls == 0 {
		t.Fatal("expected runtime provider stream callback to be invoked")
	}
	if proxyCalled {
		t.Fatal("proxy bridge should not be called when runtime provider is configured")
	}
	if got := runtimeProvider.LastRequest().Model; got != "auto" {
		t.Fatalf("runtime provider model = %q, want auto", got)
	}
}
