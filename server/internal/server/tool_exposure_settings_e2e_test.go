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
		{Name: "web_search", Description: "Search the web"},
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
			"web_search",
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

	t.Run("explicit capability toggles still filter first turn", func(t *testing.T) {
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
		for _, forbidden := range []string{"deep_research", "web_search"} {
			if _, ok := names[forbidden]; ok {
				t.Fatalf("expected %q to be removed, got=%v", forbidden, capture.LastRequest().Tools)
			}
		}
		for _, required := range []string{"plan_create", "read", "write"} {
			if _, ok := names[required]; !ok {
				t.Fatalf("expected %q to remain visible, got=%v", required, capture.LastRequest().Tools)
			}
		}
	})

	t.Run("persisted command-state toggles also filter first turn", func(t *testing.T) {
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
		for _, forbidden := range []string{"deep_research", "web_search"} {
			if _, ok := names[forbidden]; ok {
				t.Fatalf("expected %q to be removed by persisted command state, got=%v", forbidden, capture.LastRequest().Tools)
			}
		}
		for _, required := range []string{"plan_create", "read", "write"} {
			if _, ok := names[required]; !ok {
				t.Fatalf("expected %q to remain visible, got=%v", required, capture.LastRequest().Tools)
			}
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
