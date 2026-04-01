package server

import (
	"context"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newChatToolSelectionTestHandler(registry *tools.Registry) *ChatHandler {
	handler := NewChatHandler(nil, nil, registry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))
	handler.SetToolSelector(tools.DefaultToolSelector())
	handler.SetToolRouter(tools.DefaultToolRouter())
	handler.SetToolPolicyResolver(tools.NewToolPolicyResolver(&config.Config{
		ToolCalling: *config.DefaultToolCallingConfig(),
		Agents:      *config.DefaultAgentsConfig(),
	}))
	return handler
}

func toolNameSet(defs []tools.ToolDefinition) map[string]struct{} {
	names := make(map[string]struct{}, len(defs))
	for _, def := range defs {
		names[def.Name] = struct{}{}
	}
	return names
}

func selectedToolNames(defs []tools.ToolDefinition) []string {
	names := make([]string, 0, len(defs))
	for _, def := range defs {
		names = append(names, def.Name)
	}
	return names
}

func attachTestProviderPool(t *testing.T, handler *ChatHandler, provider *providerpool.Provider) {
	t.Helper()

	storage, err := providerpool.NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStorage failed: %v", err)
	}
	registry, err := providerpool.NewRegistry(storage)
	if err != nil {
		t.Fatalf("NewRegistry failed: %v", err)
	}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	handler.SetProviderPool(&providerpool.Pool{Registry: registry})
}

func TestSelectTools_FirstTurnExposesFullStaticAllowlist(t *testing.T) {
	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "bash", Description: "Run real shell commands"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "calendar", Description: "Calendar scheduling and agenda"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "deep_research", Description: "Run deep research or check an existing research job status"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "email", Description: "Email inbox search and triage"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "plan_append", Description: "Append checklist items"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "plan_create", Description: "Create a checklist"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "plan_update", Description: "Update checklist item states"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "web_search", Description: "Search the web"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "process", Description: "Inspect long-running processes"})

	handler := newChatToolSelectionTestHandler(registry)

	got := handler.selectTools("Archive unread emails from Alice", tools.ToolPolicyRequest{
		Model:     "claude-3-5-haiku-20241022",
		RouteKind: tools.ToolRouteKindChat,
	})
	if len(got) == 0 {
		t.Fatal("expected non-empty tool selection")
	}

	names := toolNameSet(got)
	for _, required := range []string{
		"bash",
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
			t.Fatalf("expected %q in first-turn tool set, got=%v", required, got)
		}
	}
	if _, ok := names["process"]; ok {
		t.Fatalf("expected non-allowlisted tool to stay hidden, got=%v", got)
	}
}

func TestSelectTools_FirstTurnStillExposesToolsForPlainReply(t *testing.T) {
	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "plan_create", Description: "Create a checklist"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files"})

	handler := newChatToolSelectionTestHandler(registry)

	got := handler.selectTools(`Say "Hello, I'm ready!" to confirm you can respond.`, tools.ToolPolicyRequest{
		Model:     "claude-3-5-haiku-20241022",
		RouteKind: tools.ToolRouteKindChat,
	})
	if len(got) != 3 {
		t.Fatalf("expected full first-turn tool set for plain reply, got=%v", got)
	}
}

func TestSelectTools_WorkspaceWorkflowStillKeepsBashVisible(t *testing.T) {
	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "bash", Description: "Run real shell commands"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "edit", Description: "Edit workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "ls", Description: "List workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "find", Description: "Find workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "email", Description: "Search inbox messages"})

	handler := newChatToolSelectionTestHandler(registry)

	got := handler.selectTools("Review all files under notes/ and write a summary to out.md.", tools.ToolPolicyRequest{
		Model:     "claude-3-5-haiku-20241022",
		RouteKind: tools.ToolRouteKindChat,
	})

	names := toolNameSet(got)
	for _, required := range []string{"bash", "read", "write"} {
		if _, ok := names[required]; !ok {
			t.Fatalf("expected %q to remain exposed in workspace workflow, got=%v", required, got)
		}
	}
	if _, ok := names["email"]; ok {
		t.Fatalf("expected unrelated productivity tool to stay hidden, got=%v", got)
	}
}

func TestSelectChatToolsForRequest_ExplicitCapabilityTogglesFilterTools(t *testing.T) {
	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "deep_research", Description: "Run deep research"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "plan_create", Description: "Create a checklist"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "web_search", Description: "Search the web"})

	handler := newChatToolSelectionTestHandler(registry)

	webSearchEnabled := false
	deepResearchEnabled := false
	got := handler.selectChatToolsForRequest(
		context.Background(),
		"Look up the latest updates and create a checklist",
		"claude-3-5-haiku-20241022",
		"session-1",
		"",
		memory.ConversationCommandState{ConversationID: "session-1"},
		&webSearchEnabled,
		&deepResearchEnabled,
	)

	names := toolNameSet(got)
	for _, forbidden := range []string{"deep_research", "web_search"} {
		if _, ok := names[forbidden]; ok {
			t.Fatalf("expected %q to be removed by explicit capability toggle, got=%v", forbidden, got)
		}
	}
	for _, required := range []string{"plan_create", "read"} {
		if _, ok := names[required]; !ok {
			t.Fatalf("expected %q to remain visible, got=%v", required, got)
		}
	}
}

func TestSelectChatToolsForRequest_SmartSkillSelectionCollapsesToExec(t *testing.T) {
	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "exec", Description: "Execute skill and shell commands"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "web_search", Description: "Search the web"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files"})

	handler := newChatToolSelectionTestHandler(registry)
	smartSkill := true
	handler.GetSettingsHandler().settings.SmartSkillSelection = &smartSkill

	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	writeSettingsSelectorSkill(t, workspaceDir, "web_search", "search the web for latest docs and official references", `blue web_search query="OpenAI Responses API docs"`, "search", "web", "docs")
	writeSettingsSelectorSkill(t, workspaceDir, "analyze", "analyze reports and urls", `blue analyze topic="url report" --json`, "analysis", "report", "url")

	handler.SetSkillSelector(agentcore.NewSkillSelector(workspaceDir, agentcore.NewHeuristicSkillReranker()))

	got := handler.selectChatToolsForRequest(
		context.Background(),
		"搜索最新 OpenAI Responses API 文档。",
		"claude-3-5-haiku-20241022",
		"session-cutover",
		"",
		memory.ConversationCommandState{ConversationID: "session-cutover"},
		nil,
		nil,
	)

	if names := selectedToolNames(got); len(names) != 1 || names[0] != "exec" {
		t.Fatalf("selectChatToolsForRequest() = %v, want [exec]", names)
	}
}

func TestSelectChatToolsForRequest_SmartSkillClarifyHidesNativeTools(t *testing.T) {
	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "exec", Description: "Execute skill and shell commands"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "web_search", Description: "Search the web"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files"})

	handler := newChatToolSelectionTestHandler(registry)
	smartSkill := true
	handler.GetSettingsHandler().settings.SmartSkillSelection = &smartSkill

	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	writeSettingsSelectorSkill(t, workspaceDir, "web_search", "search the web for latest docs and official references", `blue web_search query="OpenAI Responses API docs"`, "search", "web", "docs")
	writeSettingsSelectorSkill(t, workspaceDir, "analyze", "analyze reports and urls", `blue analyze topic="url report" --json`, "analysis", "report", "url")

	handler.SetSkillSelector(agentcore.NewSkillSelector(workspaceDir, agentcore.NewHeuristicSkillReranker()))

	got := handler.selectChatToolsForRequest(
		context.Background(),
		"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？",
		"claude-3-5-haiku-20241022",
		"session-clarify",
		"",
		memory.ConversationCommandState{ConversationID: "session-clarify"},
		nil,
		nil,
	)

	if len(got) != 0 {
		t.Fatalf("selectChatToolsForRequest() = %v, want no native tools while clarifying", selectedToolNames(got))
	}
}

func TestSelectChatToolsForRequest_CutoverSkipsPromptCacheStickyUnion(t *testing.T) {
	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "exec", Description: "Execute skill and shell commands"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "web_search", Description: "Search the web"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files"})

	handler := newChatToolSelectionTestHandler(registry)
	smartSkill := true
	handler.GetSettingsHandler().settings.SmartSkillSelection = &smartSkill

	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	writeSettingsSelectorSkill(t, workspaceDir, "web_search", "search the web for latest docs and official references", `blue web_search query="OpenAI Responses API docs"`, "search", "web", "docs")
	writeSettingsSelectorSkill(t, workspaceDir, "analyze", "analyze reports and urls", `blue analyze topic="url report" --json`, "analysis", "report", "url")

	handler.SetSkillSelector(agentcore.NewSkillSelector(workspaceDir, agentcore.NewHeuristicSkillReranker()))
	attachTestProviderPool(t, handler, &providerpool.Provider{
		ID:        "anthropic-test",
		Name:      "Anthropic Test",
		Type:      providerpool.ProviderTypeCustom,
		Location:  providerpool.ProviderLocationCloud,
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		APIFormat: providerpool.APIFormatAnthropic,
	})

	handler.setPromptCacheToolSurface("conv-cutover", &promptCacheToolSurface{
		ProviderID:          "anthropic-test",
		WebSearchEnabled:    true,
		DeepResearchEnabled: false,
		Tools: []tools.ToolDefinition{
			{Name: "read"},
			{Name: "write"},
		},
		ExpiresAt: time.Now().Add(time.Minute),
	})

	got := handler.selectChatToolsForRequest(
		context.Background(),
		"搜索最新 OpenAI Responses API 文档。",
		"claude-3-5-haiku-20241022",
		"conv-cutover",
		"",
		memory.ConversationCommandState{
			ConversationID:     "conv-cutover",
			SelectedProviderID: "anthropic-test",
			WebSearchEnabled:   true,
		},
		nil,
		nil,
	)

	if names := selectedToolNames(got); len(names) != 1 || names[0] != "exec" {
		t.Fatalf("selectChatToolsForRequest() = %v, want sticky surface reset to [exec]", names)
	}
	if cached := handler.getPromptCacheToolSurface("conv-cutover"); cached != nil {
		t.Fatalf("prompt cache surface = %#v, want cleared after cutover-native selection", cached)
	}
}

func TestStabilizePromptCacheToolSurface_AnthropicKeepsStickyUnion(t *testing.T) {
	registry := tools.NewRegistry()
	handler := newChatToolSelectionTestHandler(registry)
	attachTestProviderPool(t, handler, &providerpool.Provider{
		ID:        "anthropic-test",
		Name:      "Anthropic Test",
		Type:      providerpool.ProviderTypeCustom,
		Location:  providerpool.ProviderLocationCloud,
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		APIFormat: providerpool.APIFormatAnthropic,
	})

	state := memory.ConversationCommandState{
		ConversationID:     "conv-1",
		SelectedProviderID: "anthropic-test",
	}

	first := handler.stabilizePromptCacheToolSurface("conv-1", "", state, nil, nil, []tools.ToolDefinition{
		{Name: "write"},
		{Name: "ask"},
	})
	if got := selectedToolNames(first); len(got) != 2 || got[0] != "ask" || got[1] != "write" {
		t.Fatalf("first stabilize = %v, want [ask write]", got)
	}

	second := handler.stabilizePromptCacheToolSurface("conv-1", "", state, nil, nil, []tools.ToolDefinition{
		{Name: "ask"},
	})
	if got := selectedToolNames(second); len(got) != 2 || got[0] != "ask" || got[1] != "write" {
		t.Fatalf("second stabilize = %v, want sticky union [ask write]", got)
	}
}

func TestStabilizePromptCacheToolSurface_ToggleChangeResetsStickyTools(t *testing.T) {
	registry := tools.NewRegistry()
	handler := newChatToolSelectionTestHandler(registry)
	attachTestProviderPool(t, handler, &providerpool.Provider{
		ID:        "anthropic-test",
		Name:      "Anthropic Test",
		Type:      providerpool.ProviderTypeCustom,
		Location:  providerpool.ProviderLocationCloud,
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		APIFormat: providerpool.APIFormatAnthropic,
	})

	state := memory.ConversationCommandState{
		ConversationID:     "conv-1",
		SelectedProviderID: "anthropic-test",
		WebSearchEnabled:   true,
	}

	first := handler.stabilizePromptCacheToolSurface("conv-1", "", state, nil, nil, []tools.ToolDefinition{
		{Name: "web_search"},
		{Name: "read"},
	})
	if got := selectedToolNames(first); len(got) != 2 || got[0] != "read" || got[1] != "web_search" {
		t.Fatalf("first stabilize = %v, want [read web_search]", got)
	}

	webSearchEnabled := false
	second := handler.stabilizePromptCacheToolSurface("conv-1", "", state, &webSearchEnabled, nil, []tools.ToolDefinition{
		{Name: "read"},
	})
	if got := selectedToolNames(second); len(got) != 1 || got[0] != "read" {
		t.Fatalf("toggle reset stabilize = %v, want [read]", got)
	}
}

func TestStabilizePromptCacheToolSurface_NonAnthropicDoesNotStick(t *testing.T) {
	registry := tools.NewRegistry()
	handler := newChatToolSelectionTestHandler(registry)
	attachTestProviderPool(t, handler, &providerpool.Provider{
		ID:        "openai-test",
		Name:      "OpenAI Test",
		Type:      providerpool.ProviderTypeCustom,
		Location:  providerpool.ProviderLocationCloud,
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		APIFormat: providerpool.APIFormatOpenAI,
	})

	state := memory.ConversationCommandState{
		ConversationID:     "conv-1",
		SelectedProviderID: "openai-test",
	}

	_ = handler.stabilizePromptCacheToolSurface("conv-1", "", state, nil, nil, []tools.ToolDefinition{
		{Name: "write"},
		{Name: "ask"},
	})
	second := handler.stabilizePromptCacheToolSurface("conv-1", "", state, nil, nil, []tools.ToolDefinition{
		{Name: "ask"},
	})
	if got := selectedToolNames(second); len(got) != 1 || got[0] != "ask" {
		t.Fatalf("non-anthropic stabilize = %v, want [ask]", got)
	}
	if cached := handler.getPromptCacheToolSurface("conv-1"); cached != nil {
		t.Fatalf("expected no cached sticky tool surface for non-anthropic provider, got=%v", selectedToolNames(cached.Tools))
	}
}
