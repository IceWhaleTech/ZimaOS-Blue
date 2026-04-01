package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestProxyPrunerHTTPE2E_ContextTrimAndPerRequestDisable(t *testing.T) {
	var mu sync.Mutex
	var lastUpstreamBody []byte

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" && r.URL.Path != "/chat/completions" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"object":"ok"}`))
			return
		}

		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		lastUpstreamBody = body
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
		  "id":"resp-test",
		  "object":"chat.completion",
		  "created":0,
		  "model":"gpt-test",
		  "choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],
		  "usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}
		}`))
	}))
	defer upstream.Close()

	tmpDir, err := os.MkdirTemp("", "proxy-pruner-e2e-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := providerpool.NewFileStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStorage: %v", err)
	}
	registry, err := providerpool.NewRegistry(storage)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	discovery := providerpool.NewModelDiscovery(registry, storage, time.Hour)
	router := providerpool.NewRouter(registry, discovery, providerpool.RoutingStrategyPriority)

	provider := &providerpool.Provider{
		ID:        "openai-upstream",
		Name:      "openai-upstream",
		Type:      providerpool.ProviderTypeCustom,
		BaseURL:   upstream.URL,
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		Priority:  10,
		APIFormat: providerpool.APIFormatOpenAI,
		APIKeys: []providerpool.APIKey{
			{ID: "k-test", Key: "sk-test", Enabled: true},
		},
	}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("registry.Register: %v", err)
	}
	models := []*providerpool.Model{
		{
			ID:           "gpt-test",
			Name:         "gpt-test",
			ProviderID:   provider.ID,
			Enabled:      true,
			Capabilities: providerpool.ModelCapabilities{Chat: true, Streaming: false},
		},
	}
	if err := storage.SaveModels(provider.ID, models); err != nil {
		t.Fatalf("SaveModels: %v", err)
	}
	router.RebuildCandidates()

	proxyHandler := proxy.NewProxyHandler(nil, proxy.NewConnectionPool(proxy.DefaultConnectionConfig()), nil)
	proxyHandler.SetProviderPool(&providerpool.Pool{
		Registry:  registry,
		Discovery: discovery,
		Router:    router,
	})
	// Explicitly enable pruner middleware (local backend).
	prunerCfg := pruner.DefaultConfig()
	prunerMw := pruner.NewMiddleware(pruner.NewLocalBackend(prunerCfg), prunerCfg, pruner.NewStats())
	proxyHandler.SetPruner(prunerMw)

	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	settings := NewSettingsHandler(kvstore.NewMemoryStore())

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetProxyBridge(proxybridge.NewBridge(proxyHandler))
	handler.SetSettingsHandler(settings)

	// Build a conversation history containing a tool call + a large tool result (>= 50 lines).
	conv, err := store.CreateConversation(context.Background(), "proxy pruner e2e")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	toolCallID := "call-1"
	if _, err := store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role: "assistant",
		ToolCalls: []memory.ToolCall{{
			ID:        toolCallID,
			Name:      "read",
			Arguments: `{"path":"README.md"}`,
		}},
	}); err != nil {
		t.Fatalf("AddMessage assistant tool call: %v", err)
	}
	var toolOut strings.Builder
	toolOut.WriteString("```go\n")
	for i := 0; i < 80; i++ {
		toolOut.WriteString("x")
		toolOut.WriteString(strconv.Itoa(i))
		toolOut.WriteString(" := 1\n")
	}
	toolOut.WriteString("```\n")
	if _, err := store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:       "tool",
		ToolCallID: toolCallID,
		ToolName:   "read",
		Content:    toolOut.String(),
	}); err != nil {
		t.Fatalf("AddMessage tool result: %v", err)
	}

	e := echo.New()
	api := e.Group("/api/v1")
	conversations := api.Group("/conversations")
	conversations.POST("/:id/messages", handler.SendMessage)
	settings.RegisterRoutes(api)
	// Expose the pruner config endpoint so the test can toggle global enable/disable
	// and verify pruning effects end-to-end.
	prunerAPI := pruner.NewAPIHandler(prunerMw, &prunerCfg, nil)
	prunerAPI.RegisterRoutes(api.Group("/proxy/pruner"))
	srv := httptest.NewServer(e)
	defer srv.Close()

	patch := func(t *testing.T, body string) {
		t.Helper()
		req, err := http.NewRequest(http.MethodPatch, srv.URL+"/api/v1/settings", strings.NewReader(body))
		if err != nil {
			t.Fatalf("new PATCH request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatalf("PATCH /settings: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("PATCH /settings status=%d, want 200", resp.StatusCode)
		}
	}

	send := func(t *testing.T) SendMessageResponse {
		t.Helper()
		payload := []byte(`{"message":"hello","model":"gpt-test"}`)
		resp, err := http.Post(srv.URL+"/api/v1/conversations/"+conv.ID+"/messages", "application/json", bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("POST SendMessage: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("SendMessage status=%d, want 200", resp.StatusCode)
		}
		var out SendMessageResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode SendMessageResponse: %v", err)
		}
		return out
	}

	t.Run("pruner runs when small_model_context_prune_enabled=true (explicit)", func(t *testing.T) {
		patch(t, `{"small_model_context_prune_enabled":true}`)
		out := send(t)

		mu.Lock()
		body := string(lastUpstreamBody)
		mu.Unlock()

		if out.ContextTrim == nil || out.ContextTrim.Type != "pruned" {
			t.Fatalf("expected context_trim.type=pruned when pruner runs, got=%+v", out.ContextTrim)
		}
		if !strings.Contains(body, "(filtered") {
			t.Fatalf("expected upstream request body to include pruner markers, got body=%s", body)
		}
	})

	t.Run("pruner bypasses when small_model_context_prune_enabled=false (explicit)", func(t *testing.T) {
		patch(t, `{"small_model_context_prune_enabled":false}`)
		out := send(t)

		mu.Lock()
		body := string(lastUpstreamBody)
		mu.Unlock()

		if out.ContextTrim != nil && out.ContextTrim.Type == "pruned" {
			t.Fatalf("expected context_trim not pruned when pruner disabled per request, got=%+v", out.ContextTrim)
		}
		if strings.Contains(body, "(filtered") {
			t.Fatalf("expected upstream request body to be unpruned when disabled, got body=%s", body)
		}
	})

	t.Run("global pruner disable via /proxy/pruner/config stops pruning even when request allows it", func(t *testing.T) {
		patch(t, `{"small_model_context_prune_enabled":true}`)

		req, err := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/proxy/pruner/config", strings.NewReader(`{"enabled":false}`))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatalf("PUT pruner config: %v", err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("PUT pruner config status=%d, want 200", resp.StatusCode)
		}

		out := send(t)
		mu.Lock()
		body := string(lastUpstreamBody)
		mu.Unlock()

		if out.ContextTrim != nil && out.ContextTrim.Type == "pruned" {
			t.Fatalf("expected no pruning when globally disabled, got context_trim=%+v", out.ContextTrim)
		}
		if strings.Contains(body, "(filtered") {
			t.Fatalf("expected upstream body to be unpruned when globally disabled, got body=%s", body)
		}
	})

	t.Run("global pruner enable via /proxy/pruner/config resumes pruning", func(t *testing.T) {
		patch(t, `{"small_model_context_prune_enabled":true}`)

		req, err := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/proxy/pruner/config", strings.NewReader(`{"enabled":true}`))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatalf("PUT pruner config: %v", err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("PUT pruner config status=%d, want 200", resp.StatusCode)
		}

		out := send(t)
		mu.Lock()
		body := string(lastUpstreamBody)
		mu.Unlock()

		if out.ContextTrim == nil || out.ContextTrim.Type != "pruned" {
			t.Fatalf("expected pruning when globally enabled, got context_trim=%+v", out.ContextTrim)
		}
		if !strings.Contains(body, "(filtered") {
			t.Fatalf("expected upstream body to be pruned when globally enabled, got body=%s", body)
		}
	})
}
