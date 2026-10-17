package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type parallelConfigTransport func(*http.Request) (*http.Response, error)

func (f parallelConfigTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestBuildBuiltinToolConfigs_ParallelSettings(t *testing.T) {
	disabled := false
	cfg := &config.Config{}
	cfg.ToolCalling.WebSearch.ProviderSettings = map[string]config.ToolCallingWebSearchProviderSetting{"parallel": {APIKey: "provider-key", BaseURL: "https://search.parallel.ai/mcp", Enabled: &disabled}}
	cfg.ToolCalling.WebSearch.BrowserFallback.Enabled = &disabled
	cfg.ToolCalling.WebSearch.BrowserFallback.Engine = "duckduckgo"
	mapped, _ := buildBuiltinToolConfigs(cfg)
	setting, ok := mapped.ProviderSettings["parallel"]
	if !ok || setting.APIKey != "provider-key" || setting.BaseURL != "https://search.parallel.ai/mcp" || setting.Enabled == nil || *setting.Enabled {
		t.Fatal("provider-specific settings were not preserved")
	}
	if mapped.BrowserFallback.Enabled == nil || *mapped.BrowserFallback.Enabled || mapped.BrowserFallback.Engine != "duckduckgo" {
		t.Fatal("explicit browser fallback settings were not preserved")
	}
}

func TestRegisterServerToolRegistry_ParallelSettings(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer provider-key" || !strings.Contains(r.UserAgent(), "ZimaOS-Blue") {
			t.Error("missing provider-scoped key or project identity")
		}
		var req struct {
			ID     int
			Method string
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		var result interface{}
		switch req.Method {
		case "initialize":
			result = map[string]interface{}{"protocolVersion": "2025-03-26", "capabilities": map[string]interface{}{}}
		case "notifications/initialized":
			w.WriteHeader(202)
			return
		case "tools/list":
			result = map[string]interface{}{"tools": []interface{}{map[string]interface{}{"name": "web_search"}}}
		case "tools/call":
			result = map[string]interface{}{"structuredContent": map[string]interface{}{"results": []interface{}{map[string]interface{}{"title": "Blue source", "url": "https://docs.example.com/blue", "excerpts": []string{"Registry caller source evidence"}}}}}
		default:
			t.Errorf("unexpected method %q", req.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer server.Close()
	enabled := true
	browserEnabled := false
	cfg := &config.Config{}
	cfg.ToolCalling.WebSearch.Provider = "parallel"
	cfg.ToolCalling.WebSearch.Providers = []string{"parallel"}
	cfg.ToolCalling.WebSearch.APIKey = "other-provider-key"
	cfg.ToolCalling.WebSearch.ProviderSettings = map[string]config.ToolCallingWebSearchProviderSetting{"parallel": {APIKey: "provider-key", BaseURL: server.URL, Enabled: &enabled}}
	cfg.ToolCalling.WebSearch.BrowserFallback.Enabled = &browserEnabled
	registry := tools.NewRegistry()
	registerServerToolRegistry(registry, cfg, t.TempDir())
	search, ok := registry.Get("web_search").(*tools.WebSearchTool)
	if !ok {
		t.Fatal("missing registered search tool")
	}
	search.SetHTTPClientForTest(&http.Client{Transport: parallelConfigTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != server.URL {
			return nil, errors.New("provider endpoint did not reach server registry")
		}
		return http.DefaultTransport.RoundTrip(r)
	})})
	gateway := tools.NewToolGateway(registry, tools.NewExecutor(registry))
	result, err := gateway.Execute(context.Background(), tools.ToolGatewayRequest{ToolName: "web_query", Arguments: `{"action":"search","query":"Blue docs","format":"json"}`, RouteKind: tools.ToolRouteKindChat})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.CompactLLMContent, "Registry caller source evidence") || !strings.Contains(result.CompactLLMContent, "https://docs.example.com/blue") {
		t.Fatalf("registry did not consume useful sources: %s", result.CompactLLMContent)
	}
	if calls.Load() != 4 {
		t.Fatalf("MCP request count = %d, want 4", calls.Load())
	}
	enabled = false
	registry = tools.NewRegistry()
	registerServerToolRegistry(registry, cfg, t.TempDir())
	search = registry.Get("web_search").(*tools.WebSearchTool)
	search.SetHTTPClientForTest(&http.Client{Transport: parallelConfigTransport(func(r *http.Request) (*http.Response, error) {
		t.Error("disabled provider made a network request")
		return &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader(""))}, nil
	})})
	gateway = tools.NewToolGateway(registry, tools.NewExecutor(registry))
	if _, err := gateway.Execute(context.Background(), tools.ToolGatewayRequest{ToolName: "web_query", Arguments: `{"action":"search","query":"Blue docs","provider":"parallel"}`, RouteKind: tools.ToolRouteKindChat}); err == nil {
		t.Fatal("disabled provider should fail through normal registry")
	}
}
