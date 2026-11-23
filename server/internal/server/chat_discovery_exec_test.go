package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
)

func TestDiscoveryOnlyExecActivation(t *testing.T) {
	for _, installed := range []bool{false, true} {
		for _, stream := range []bool{false, true} {
			name := "sync"
			if stream {
				name = "stream"
			}
			if installed {
				name += "-installed-filens"
			}
			t.Run(name, func(t *testing.T) {
				store, err := memory.NewStore(":memory:")
				if err != nil {
					t.Fatal(err)
				}
				defer store.Close()
				conv, err := store.CreateConversation(context.Background(), "Discovery exec")
				if err != nil {
					t.Fatal(err)
				}
				provider := &scriptedChatProvider{name: "discovery-exec", responses: []llm.ChatResponse{
					{Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "discover", Name: "tool_search", Arguments: `{"query":"select:exec"}`}}}},
					{Message: llm.Message{Role: llm.RoleAssistant, Content: "Ready."}},
				}}
				registry := llm.NewProviderRegistry()
				registry.Register(provider)
				h := newDiscoveryOnlySelectionChatHandler(t, store, registry, tools.NewRegistry())
				query := "Handle the frobnicator."
				if installed {
					query = installedFiLensShellQuery(t, h)
				}
				body, _ := json.Marshal(map[string]interface{}{"message": query, "provider": provider.Name(), "model": "gpt-5.3-codex-spark"})
				if stream {
					runStreamTurn(t, h, conv.ID, string(body))
				} else {
					e := echo.New()
					req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
					req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
					c := e.NewContext(req, httptest.NewRecorder())
					c.SetParamNames("id")
					c.SetParamValues(conv.ID)
					if err := h.SendMessage(c); err != nil {
						t.Fatal(err)
					}
				}
				first, ok := provider.RequestAt(0)
				if !ok {
					t.Fatal("missing first request")
				}
				if len(first.Tools) == 0 {
					t.Fatal("weak installed skill match exposed no model tools")
				}
				if !llmToolsContain(first.Tools, "tool_search") {
					t.Fatal("missing policy-filtered discovery")
				}
				second, ok := provider.RequestAt(1)
				if !ok {
					t.Fatal("missing post-discovery request")
				}
				if installed {
					if !llmToolsContain(second.Tools, "exec") {
						t.Fatal("exec was not exposed after discovery")
					}
				} else {
					assertLLMRequestToolNames(t, first.Tools, "tool_search")
					assertLLMRequestToolNames(t, second.Tools, "exec", "tool_search")
				}
				found := false
				for _, m := range second.Messages {
					if m.Role == llm.RoleTool && strings.Contains(m.Content, `"availability":"deferred"`) {
						found = true
					}
				}
				if !found && !installed {
					t.Error("discovery must describe the actual initial tool surface, not policy membership")
				}
			})
		}
	}

}

func TestDiscoveryExecPolicyDeny(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	h := newClarifyNoneSelectionChatHandler(t, store, llm.NewProviderRegistry(), tools.NewRegistry())
	cfg := &config.Config{ToolCalling: *config.DefaultToolCallingConfig(), Agents: *config.DefaultAgentsConfig()}
	cfg.ToolCalling.Deny = []string{"exec"}
	h.SetToolPolicyResolver(tools.NewToolPolicyResolver(cfg))
	h.ConfigureToolSearchRuntime("", cfg)
	h.deferredToolExposure.Apply("denied", tools.DeferredToolExposureUpdate{ActivatedTools: []string{"exec"}, RegistryVersion: h.toolRegistry.Version(), PromptPolicyHash: h.resolvePromptPolicy().Hash})
	request := llm.ChatRequest{Model: "gpt-5.3-codex-spark", Tools: []llm.Tool{{Name: "tool_search"}}}
	request = h.refreshDiscoveredTools(context.Background(), request, []llm.ToolCall{{Name: "tool_search"}}, nil, "denied", nil, nil)
	assertLLMRequestToolNames(t, request.Tools, "tool_search")
	request = h.refreshDiscoveredTools(context.Background(), request, []llm.ToolCall{{ID: "preview", Name: "bash"}}, []llm.Message{{ToolCallID: "preview", Content: `{"session_id":"retained","llm_output_truncated":true,"output_retrieval":{"tool":"exec","action":"log"}}`}}, "denied", nil, nil)
	assertLLMRequestToolNames(t, request.Tools, "tool_search")
	for _, preview := range []bool{false, true} {
		policyReq := tools.ToolPolicyRequest{RouteKind: tools.ToolRouteKindChat}
		var selection chatToolSurfaceSelection
		if preview {
			selection = h.previewChatToolSurfacesForRequest(context.Background(), "Search the latest OpenAI Responses API documentation.", policyReq, nil, nil)
		} else {
			selection = h.selectChatToolSurfacesForRequest(context.Background(), "Search the latest OpenAI Responses API documentation.", policyReq, nil, nil)
		}
		if hasToolDefName(selection.NativeDefs, "exec") {
			t.Errorf("preview=%v: skill cutover exposed denied exec", preview)
		}
	}

}

func TestTruncatedBashExposesRecovery(t *testing.T) {
	for _, stream := range []bool{false, true} {
		name := "sync"
		if stream {
			name = "stream"
		}
		t.Run(name, func(t *testing.T) {
			store, err := memory.NewStore(":memory:")
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			conv, err := store.CreateConversation(context.Background(), "Recover output")
			if err != nil {
				t.Fatal(err)
			}
			provider := &scriptedChatProvider{name: "recovery", responses: []llm.ChatResponse{
				{Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "discover", Name: "tool_search", Arguments: `{"query":"select:bash"}`}}}},
				{Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "run", Name: "bash", Arguments: `{"command":"print fixture"}`}}}},
				{Message: llm.Message{Role: llm.RoleAssistant, Content: "Ready."}},
			}}
			registry := llm.NewProviderRegistry()
			registry.Register(provider)
			toolRegistry := tools.NewRegistry()
			toolRegistry.Register(&staticToolMock{def: tools.ToolDefinition{Name: "bash", AlwaysLoad: true}, result: map[string]interface{}{"stdout": strings.Repeat("context\n", 2500), "status": "completed", "session_id": "retained-output"}})
			h := newDiscoveryOnlySelectionChatHandler(t, store, registry, toolRegistry)
			body, _ := json.Marshal(map[string]interface{}{"message": "Handle the frobnicator.", "provider": provider.Name(), "model": "gpt-5.3-codex-spark"})
			if stream {
				runStreamTurn(t, h, conv.ID, string(body))
			} else {
				e := echo.New()
				req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
				req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
				c := e.NewContext(req, httptest.NewRecorder())
				c.SetParamNames("id")
				c.SetParamValues(conv.ID)
				if err := h.SendMessage(c); err != nil {
					t.Fatal(err)
				}
			}
			post, ok := provider.RequestAt(2)
			if !ok {
				t.Fatal("missing post-preview request")
			}
			found := false
			for _, tool := range post.Tools {
				if tool.Name == "exec" {
					found = true
				}
			}
			if !found {
				t.Error("incomplete bash evidence has a retained session but no callable exec recovery interface")
			}
		})
	}
}

func newDiscoveryOnlySelectionChatHandler(t *testing.T, store *memory.Store, registry *llm.ProviderRegistry, toolRegistry *tools.Registry) *ChatHandler {
	t.Helper()
	h := newClarifyNoneSelectionChatHandler(t, store, registry, toolRegistry)
	return h
}

func llmToolsContain(defs []llm.Tool, name string) bool {
	for _, def := range defs {
		if def.Name == name {
			return true
		}
	}
	return false
}
