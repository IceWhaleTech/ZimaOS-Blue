package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestChatToolExposureHTTPE2E_FirstTurnStaticAllowlistAndCapabilityToggles(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create memory store: %v", err)
	}
	defer store.Close()

	registry := llm.NewProviderRegistry()
	capture := &requestCaptureProvider{}
	registry.Register(capture)

	toolRegistry := tools.NewRegistry()
	for _, def := range []tools.ToolDefinition{
		{Name: "calendar", Description: "Calendar scheduling and agenda"},
		{Name: "deep_research", Description: "Run deep research"},
		{Name: "email", Description: "Email inbox search and triage"},
		{Name: "plan_append", Description: "Append checklist items"},
		{Name: "plan_create", Description: "Create a checklist"},
		{Name: "plan_update", Description: "Update checklist item states"},
		{Name: "read", Description: "Read workspace files"},
		{Name: "web_query", Description: "Search the web"},
		{Name: "write", Description: "Write workspace files"},
		{Name: "process", Description: "Inspect long-running processes"},
	} {
		toolRegistry.ExposeDefinition(def)
	}

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))
	handler.SetToolPolicyResolver(tools.NewToolPolicyResolver(&config.Config{
		ToolCalling: *config.DefaultToolCallingConfig(),
		Agents:      *config.DefaultAgentsConfig(),
	}))

	e := echo.New()
	api := e.Group("/api/v1")
	conversations := api.Group("/conversations")
	conversations.POST("/:id/messages", handler.SendMessage)

	server := httptest.NewServer(e)
	defer server.Close()

	t.Run("full static allowlist on first turn", func(t *testing.T) {
		conv, err := store.CreateConversation(context.Background(), "tool exposure e2e")
		if err != nil {
			t.Fatalf("create conversation: %v", err)
		}

		body := map[string]any{
			"message":  "Archive unread emails from Alice",
			"provider": "capture",
			"model":    "capture-model",
		}
		reqBody, _ := json.Marshal(body)
		resp, err := http.Post(server.URL+"/api/v1/conversations/"+conv.ID+"/messages", "application/json", bytes.NewReader(reqBody))
		if err != nil {
			t.Fatalf("post send message: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}

		names := make(map[string]struct{}, len(capture.LastRequest().Tools))
		for _, tool := range capture.LastRequest().Tools {
			names[tool.Name] = struct{}{}
		}
		for _, required := range []string{
			"calendar",
			"deep_research",
			"email",
			"plan_append",
			"plan_create",
			"plan_update",
			"read",
			"web_query",
			"write",
		} {
			if _, ok := names[required]; !ok {
				t.Fatalf("expected %q in first-turn tool set, got=%v", required, capture.LastRequest().Tools)
			}
		}
		if _, ok := names["process"]; ok {
			t.Fatalf("expected non-allowlisted tool to stay hidden, got=%v", capture.LastRequest().Tools)
		}
	})

	t.Run("legacy capability toggles no longer filter first turn", func(t *testing.T) {
		conv, err := store.CreateConversation(context.Background(), "tool exposure toggles e2e")
		if err != nil {
			t.Fatalf("create conversation: %v", err)
		}

		body := map[string]any{
			"message":               "Look up the latest updates and create a checklist",
			"provider":              "capture",
			"model":                 "capture-model",
			"web_search_enabled":    false,
			"deep_research_enabled": false,
		}
		reqBody, _ := json.Marshal(body)
		resp, err := http.Post(server.URL+"/api/v1/conversations/"+conv.ID+"/messages", "application/json", bytes.NewReader(reqBody))
		if err != nil {
			t.Fatalf("post send message: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}

		names := make(map[string]struct{}, len(capture.LastRequest().Tools))
		for _, tool := range capture.LastRequest().Tools {
			names[tool.Name] = struct{}{}
		}
		for _, required := range []string{"deep_research", "plan_create", "read", "web_query", "write"} {
			if _, ok := names[required]; !ok {
				t.Fatalf("expected %q to remain visible despite legacy request flags, got=%v", required, capture.LastRequest().Tools)
			}
		}
	})

	t.Run("persisted command-state toggles no longer filter first turn", func(t *testing.T) {
		conv, err := store.CreateConversation(context.Background(), "tool exposure persisted toggles e2e")
		if err != nil {
			t.Fatalf("create conversation: %v", err)
		}
		if err := store.UpsertConversationCommandState(context.Background(), memory.ConversationCommandState{
			ConversationID:      conv.ID,
			WebSearchEnabled:    false,
			DeepResearchEnabled: false,
		}); err != nil {
			t.Fatalf("UpsertConversationCommandState: %v", err)
		}

		body := map[string]any{
			"message":  "Look up the latest updates and create a checklist",
			"provider": "capture",
			"model":    "capture-model",
		}
		reqBody, _ := json.Marshal(body)
		resp, err := http.Post(server.URL+"/api/v1/conversations/"+conv.ID+"/messages", "application/json", bytes.NewReader(reqBody))
		if err != nil {
			t.Fatalf("post send message: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}

		names := make(map[string]struct{}, len(capture.LastRequest().Tools))
		for _, tool := range capture.LastRequest().Tools {
			names[tool.Name] = struct{}{}
		}
		for _, required := range []string{"deep_research", "plan_create", "read", "web_query", "write"} {
			if _, ok := names[required]; !ok {
				t.Fatalf("expected %q to remain visible despite persisted legacy toggles, got=%v", required, capture.LastRequest().Tools)
			}
		}
	})
}

func TestChatToolExposureHTTPE2E_DiscoverFirstCutoverExecOnly(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create memory store: %v", err)
	}
	defer store.Close()

	registry := llm.NewProviderRegistry()
	capture := &requestCaptureProvider{}
	registry.Register(capture)

	toolRegistry := tools.NewRegistry()
	for _, def := range []tools.ToolDefinition{
		{Name: "browser", Description: "Open and interact with web pages"},
		{Name: "deep_research", Description: "Run deep research"},
		{Name: "exec", Description: "Execute skill and shell commands"},
		{Name: "read", Description: "Read workspace files"},
		{Name: "web_query", Description: "Search the web"},
		{Name: "write", Description: "Write workspace files"},
	} {
		toolRegistry.ExposeDefinition(def)
	}

	handler := NewChatHandler(store, registry, toolRegistry)
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	handler.SetSettingsHandler(settings)
	handler.SetToolSelector(tools.DefaultToolSelector())
	handler.SetToolRouter(tools.DefaultToolRouter())
	handler.SetToolPolicyResolver(tools.NewToolPolicyResolver(&config.Config{
		ToolCalling: *config.DefaultToolCallingConfig(),
		Agents:      *config.DefaultAgentsConfig(),
	}))

	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	writeSettingsSelectorCanonicalWebQuerySkill(t, workspaceDir, "search the web for latest docs and official references", `blue web_query input="OpenAI Responses API docs"`, "search", "web", "docs", "latest")
	writeSettingsSelectorSkill(t, workspaceDir, "browser", "browse urls and interact with web pages after login or click flows", "blue browser.navigate url=https://example.com", "browser", "login", "click")
	writeSettingsSelectorSkill(t, workspaceDir, "analyze", "analyze multiple links and synthesize a report", `blue analyze topic="multi-link report" --json`, "analysis", "report", "summary", "link", "url")
	writeSettingsSelectorSkill(t, workspaceDir, "deep_research", "perform cited timeline comparisons and deep research", `blue deep_research query="OpenAI vs Anthropic agent runtime"`, "research", "citation", "timeline", "compare")
	writeSettingsSelectorSkill(t, workspaceDir, "ui_reviewer", "review screenshots and UI layouts for accessibility and visual issues", `blue ui_reviewer target="https://example.com"`, "ui", "review", "screenshot", "layout", "accessibility")
	handler.SetSkillSelector(agentcore.NewSkillSelector(workspaceDir, agentcore.NewHeuristicSkillReranker()))

	e := echo.New()
	api := e.Group("/api/v1")
	conversations := api.Group("/conversations")
	conversations.POST("/:id/messages", handler.SendMessage)

	server := httptest.NewServer(e)
	defer server.Close()

	t.Run("cutover collapses latest docs to exec only", func(t *testing.T) {
		conv, err := store.CreateConversation(context.Background(), "discover-first latest docs")
		if err != nil {
			t.Fatalf("create conversation: %v", err)
		}

		body := map[string]any{
			"message":  "搜索最新 OpenAI Responses API 文档。",
			"provider": "capture",
			"model":    "capture-model",
		}
		reqBody, _ := json.Marshal(body)
		resp, err := http.Post(server.URL+"/api/v1/conversations/"+conv.ID+"/messages", "application/json", bytes.NewReader(reqBody))
		if err != nil {
			t.Fatalf("post send message: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}

		got := capture.LastRequest().Tools
		if len(got) != 1 || got[0].Name != "exec" {
			t.Fatalf("provider tools = %#v, want exec-only cutover", got)
		}
	})

	t.Run("legacy capability toggle no longer blocks cutover", func(t *testing.T) {
		conv, err := store.CreateConversation(context.Background(), "discover-first toggle blocked")
		if err != nil {
			t.Fatalf("create conversation: %v", err)
		}

		body := map[string]any{
			"message":            "搜索最新 OpenAI Responses API 文档。",
			"provider":           "capture",
			"model":              "capture-model",
			"web_search_enabled": false,
		}
		reqBody, _ := json.Marshal(body)
		resp, err := http.Post(server.URL+"/api/v1/conversations/"+conv.ID+"/messages", "application/json", bytes.NewReader(reqBody))
		if err != nil {
			t.Fatalf("post send message: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}

		got := capture.LastRequest().Tools
		if len(got) != 1 || got[0].Name != "exec" {
			t.Fatalf("provider tools = %#v, want exec-only cutover even when legacy flags are false", got)
		}
	})

	t.Run("merged research family cutovers stay exec only even when legacy toggles are false", func(t *testing.T) {
		tests := []struct {
			name    string
			message string
		}{
			{
				name:    "analyze",
				message: "汇总这几个链接并给我一份报告：https://example.com/a https://example.com/b",
			},
			{
				name:    "deep_research",
				message: "Investigate https://example.com/pricing and compare the claims with citations, evidence, and a timeline.",
			},
			{
				name:    "ui_review",
				message: "Review the UI of https://example.com/pricing for accessibility and layout issues.",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				conv, err := store.CreateConversation(context.Background(), "discover-first research family "+tc.name)
				if err != nil {
					t.Fatalf("create conversation: %v", err)
				}

				body := map[string]any{
					"message":               tc.message,
					"provider":              "capture",
					"model":                 "capture-model",
					"web_search_enabled":    false,
					"deep_research_enabled": false,
				}
				reqBody, _ := json.Marshal(body)
				resp, err := http.Post(server.URL+"/api/v1/conversations/"+conv.ID+"/messages", "application/json", bytes.NewReader(reqBody))
				if err != nil {
					t.Fatalf("post send message: %v", err)
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("status = %d, want 200", resp.StatusCode)
				}

				got := capture.LastRequest().Tools
				if len(got) != 1 || got[0].Name != "exec" {
					t.Fatalf("provider tools = %#v, want exec-only research-family cutover", got)
				}
			})
		}
	})

	t.Run("clarify then follow-up transitions to canonical cutover", func(t *testing.T) {
		conv, err := store.CreateConversation(context.Background(), "discover-first clarify follow-up")
		if err != nil {
			t.Fatalf("create conversation: %v", err)
		}

		firstBody := map[string]any{
			"message":  "看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？",
			"provider": "capture",
			"model":    "capture-model",
		}
		reqBody, _ := json.Marshal(firstBody)
		resp, err := http.Post(server.URL+"/api/v1/conversations/"+conv.ID+"/messages", "application/json", bytes.NewReader(reqBody))
		if err != nil {
			t.Fatalf("post send message: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		if len(capture.LastRequest().Tools) != 0 {
			t.Fatalf("expected no native tools while clarifying, got=%v", capture.LastRequest().Tools)
		}

		secondBody := map[string]any{
			"message":  "先搜最新 OpenAI Responses API 文档。",
			"provider": "capture",
			"model":    "capture-model",
		}
		reqBody, _ = json.Marshal(secondBody)
		resp, err = http.Post(server.URL+"/api/v1/conversations/"+conv.ID+"/messages", "application/json", bytes.NewReader(reqBody))
		if err != nil {
			t.Fatalf("post follow-up message: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("follow-up status = %d, want 200", resp.StatusCode)
		}
		got := capture.LastRequest().Tools
		if len(got) != 1 || got[0].Name != "exec" {
			t.Fatalf("follow-up provider tools = %#v, want exec-only cutover", got)
		}
	})
}

func TestSettingsRemovedKeysHTTPE2E_Return400(t *testing.T) {
	jwtSvc := auth.NewJWTService(&auth.JWTConfig{
		Secret:            "12345678901234567890123456789012",
		Expiration:        time.Hour,
		RefreshExpiration: 24 * time.Hour,
		Issuer:            "settings-e2e",
	})
	token, err := jwtSvc.GenerateAccessToken(&auth.UserClaims{
		UserID:   "settings-e2e-user",
		Username: "settings-e2e",
		Role:     "admin",
	})
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}

	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())

	e := echo.New()
	authMW := auth.NewAuthMiddleware(jwtSvc, nil)
	api := e.Group("/api/v1", authMW.Authenticate())
	settingsHandler.RegisterRoutes(api)

	server := httptest.NewServer(e)
	defer server.Close()

	doJSON := func(method string, body string) *http.Response {
		req, err := http.NewRequest(method, server.URL+"/api/v1/settings", strings.NewReader(body))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := server.Client().Do(req)
		if err != nil {
			t.Fatalf("do request: %v", err)
		}
		return resp
	}

	t.Run("patch rejects removed key", func(t *testing.T) {
		resp := doJSON(http.MethodPatch, `{"smart_tool_selection":true}`)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", resp.StatusCode)
		}
		var payload map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if !strings.Contains(payload["error"], "smart_tool_selection") {
			t.Fatalf("expected removed key in error, got=%v", payload)
		}
	})

	t.Run("put rejects removed key", func(t *testing.T) {
		resp := doJSON(http.MethodPut, `{"locale":"en-US","small_model_route_tool_dispatch_enabled":true}`)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", resp.StatusCode)
		}
		var payload map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if !strings.Contains(payload["error"], "small_model_route_tool_dispatch_enabled") {
			t.Fatalf("expected removed key in error, got=%v", payload)
		}
	})
}
