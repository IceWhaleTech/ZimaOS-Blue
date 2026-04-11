package server

import (
	"context"
	"fmt"
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

func newDiscoverFirstSelectionHandler(t *testing.T, dynamicExposure bool) *ChatHandler {
	t.Helper()

	registry := tools.NewRegistry()
	registry.Register(tools.NewToolSearchTool(registry))
	registry.ExposeDefinition(tools.ToolDefinition{Name: "ask", Description: "Ask the user clarifying questions"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "browser", Description: "Open and interact with web pages"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "deep_research", Description: "Run deep research"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "exec", Description: "Execute skill and shell commands"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "config", Description: "Manage runtime settings and providers"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "web_query", Description: "Search the web"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files"})

	handler := newChatToolSelectionTestHandler(registry)

	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	writeSettingsSelectorCanonicalWebQuerySkill(t, workspaceDir, "search the web for latest docs and official references", `blue web_query input="OpenAI Responses API docs"`, "search", "web", "docs", "latest")
	writeSettingsSelectorSkill(t, workspaceDir, "ask", "ask the user clarifying questions and wait for an answer", `blue ask q="Choose a deploy strategy" a='["Canary","Blue-Green"]'`, "clarify", "interactive", "question")
	writeSettingsSelectorSkill(t, workspaceDir, "browser", "browse urls and interact with web pages after login or click flows", "blue browser.navigate url=https://example.com", "browser", "login", "click", "page")
	writeSettingsSelectorSkill(t, workspaceDir, "analyze", "analyze multiple links and synthesize a report", `blue analyze topic="multi-link report" --json`, "analysis", "report", "summary", "link", "url")
	writeSettingsSelectorSkill(t, workspaceDir, "deep_research", "perform cited timeline comparisons and deep research", `blue deep_research query="OpenAI vs Anthropic agent runtime"`, "research", "citation", "timeline", "compare")
	writeSettingsSelectorSkill(t, workspaceDir, "ui_reviewer", "review screenshots and UI layouts for accessibility and visual issues", `blue ui_reviewer target="https://example.com"`, "ui", "review", "screenshot", "layout", "accessibility")
	writeSettingsSelectorSkill(t, workspaceDir, "config", "manage providers settings channels skills tools health and proxy diagnostics", "blue config.providers.list", "admin", "settings", "providers", "diagnostics")

	handler.SetSkillSelector(agentcore.NewSkillSelector(workspaceDir, agentcore.NewHeuristicSkillReranker()))
	handler.ConfigureToolSearchRuntime(workspaceDir, &config.Config{
		ToolCalling: *config.DefaultToolCallingConfig(),
		Agents:      *config.DefaultAgentsConfig(),
	})
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

func TestBuildChatToolSurfaceLogSnapshot_UsesNativeDefsForSelected(t *testing.T) {
	snapshot := buildChatToolSurfaceLogSnapshot(chatToolSurfaceSelection{
		RoutedDefs: []tools.ToolDefinition{
			{Name: "ask"},
			{Name: "browser"},
			{Name: "web_query"},
		},
		NativeDefs: nil,
		NativeMode: chatNativeToolSurfaceModeClarifyNone,
		DiscoveryDecision: &agentcore.CapabilityDiscoveryDecision{
			NeedClarify: true,
		},
		SkillDecision: &agentcore.Decision{
			Reason:        "ir_no_match",
			ConflictFlags: []string{"question_prefix"},
		},
	})

	if snapshot.Routed != 3 {
		t.Fatalf("Routed = %d, want 3", snapshot.Routed)
	}
	if snapshot.Selected != 0 {
		t.Fatalf("Selected = %d, want 0", snapshot.Selected)
	}
	if snapshot.NativeMode != string(chatNativeToolSurfaceModeClarifyNone) {
		t.Fatalf("NativeMode = %q, want %q", snapshot.NativeMode, chatNativeToolSurfaceModeClarifyNone)
	}
	if !snapshot.NeedClarify {
		t.Fatal("NeedClarify = false, want true")
	}
	if snapshot.SelectedSkill != "" {
		t.Fatalf("SelectedSkill = %q, want empty", snapshot.SelectedSkill)
	}
	if snapshot.DecisionReason != "ir_no_match" {
		t.Fatalf("DecisionReason = %q, want ir_no_match", snapshot.DecisionReason)
	}
	if got := snapshot.ConflictFlags; len(got) != 1 || got[0] != "question_prefix" {
		t.Fatalf("ConflictFlags = %v, want [question_prefix]", got)
	}
	if got := snapshot.RoutedNames; len(got) != 3 || got[0] != "ask" || got[2] != "web_query" {
		t.Fatalf("RoutedNames = %v, want [ask browser web_query]", got)
	}
	if len(snapshot.SelectedNames) != 0 {
		t.Fatalf("SelectedNames = %v, want empty", snapshot.SelectedNames)
	}
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

func bindToolSearchTestRuntime(t *testing.T, handler *ChatHandler, cfg *config.Config, workspaceDir string) *tools.ToolSearchTool {
	t.Helper()
	if cfg == nil {
		cfg = &config.Config{
			ToolCalling: *config.DefaultToolCallingConfig(),
			Agents:      *config.DefaultAgentsConfig(),
		}
	}
	handler.SetToolPolicyResolver(tools.NewToolPolicyResolver(cfg))
	handler.ConfigureToolSearchRuntime(workspaceDir, cfg)
	searchTool := tools.GetToolSearchTool(handler.toolRegistry)
	if searchTool == nil {
		t.Fatal("expected tool_search to be registered")
	}
	return searchTool
}

func TestSelectTools_FirstTurnStaticAllowlistOmitsEmailWithoutEmailIntent(t *testing.T) {
	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "bash", Description: "Run real shell commands"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "calendar", Description: "Calendar scheduling and agenda"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "deep_research", Description: "Run deep research or check an existing research job status"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "email", Description: "Email inbox search and triage"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "plan_append", Description: "Append checklist items"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "plan_create", Description: "Create a checklist"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "plan_update", Description: "Update checklist item states"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "web_query", Description: "Search the web"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "process", Description: "Inspect long-running processes"})

	handler := newChatToolSelectionTestHandler(registry)

	got := handler.selectTools("Look up the latest updates and create a checklist.", tools.ToolPolicyRequest{
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
		"plan_append",
		"plan_create",
		"plan_update",
		"read",
		"web_query",
		"write",
	} {
		if _, ok := names[required]; !ok {
			t.Fatalf("expected %q in first-turn tool set, got=%v", required, got)
		}
	}
	if _, ok := names["email"]; ok {
		t.Fatalf("expected email to stay hidden without explicit email intent, got=%v", got)
	}
	if _, ok := names["process"]; ok {
		t.Fatalf("expected non-allowlisted tool to stay hidden, got=%v", got)
	}
}

func TestSelectTools_EmailIntentExpandsAllowlistAndNarrowsToEmail(t *testing.T) {
	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "bash", Description: "Run real shell commands"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "calendar", Description: "Calendar scheduling and agenda"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "deep_research", Description: "Run deep research or check an existing research job status"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "email", Description: "Email inbox search and triage"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "plan_append", Description: "Append checklist items"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "plan_create", Description: "Create a checklist"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "plan_update", Description: "Update checklist item states"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "web_query", Description: "Search the web"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files"})

	handler := newChatToolSelectionTestHandler(registry)

	got := handler.selectTools("Archive unread emails from Alice.", tools.ToolPolicyRequest{
		Model:     "claude-3-5-haiku-20241022",
		RouteKind: tools.ToolRouteKindChat,
	})
	if len(got) == 0 {
		t.Fatal("expected non-empty tool selection")
	}

	names := toolNameSet(got)
	if _, ok := names["email"]; !ok {
		t.Fatalf("expected email to be exposed for explicit email intent, got=%v", got)
	}
	if _, ok := names["calendar"]; ok {
		t.Fatalf("expected unrelated calendar tool to stay hidden for email intent, got=%v", got)
	}
	if _, ok := names["deep_research"]; ok {
		t.Fatalf("expected unrelated research tool to stay hidden for email intent, got=%v", got)
	}
}

func TestSelectTools_FirstTurnStillExposesToolsForPlainReply(t *testing.T) {
	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "tool_search", Description: "Search and defer additional tools", AlwaysLoad: true})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "plan_create", Description: "Create a checklist"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files"})

	handler := newChatToolSelectionTestHandler(registry)

	got := handler.selectTools(`Say "Hello, I'm ready!" to confirm you can respond.`, tools.ToolPolicyRequest{
		Model:     "claude-3-5-haiku-20241022",
		RouteKind: tools.ToolRouteKindChat,
	})
	if len(got) != 4 {
		t.Fatalf("expected full first-turn tool set for plain reply, got=%v", got)
	}
	if _, ok := toolNameSet(got)["tool_search"]; !ok {
		t.Fatalf("expected tool_search to remain exposed for plain reply, got=%v", got)
	}
}

func TestSelectChatToolsForRequest_ToolSearchHydratesDeferredTool(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(tools.NewToolSearchTool(registry))
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "process", Description: "Inspect long-running processes"})

	handler := newChatToolSelectionTestHandler(registry)
	searchTool := bindToolSearchTestRuntime(t, handler, nil, "")

	ctx := tools.WithSessionID(context.Background(), "conv-tool-hydrate")
	ctx = tools.WithRouteKind(ctx, tools.ToolRouteKindChat)
	if _, err := searchTool.Execute(ctx, map[string]interface{}{"query": "select:process"}); err != nil {
		t.Fatalf("tool_search Execute returned error: %v", err)
	}

	got := handler.selectChatToolsForRequest(
		context.Background(),
		`Say "Hello, I'm ready!" to confirm you can respond.`,
		"claude-3-5-haiku-20241022",
		"conv-tool-hydrate",
		"",
		memory.ConversationCommandState{ConversationID: "conv-tool-hydrate"},
		nil,
		nil,
	)

	names := toolNameSet(got)
	for _, required := range []string{"tool_search", "process"} {
		if _, ok := names[required]; !ok {
			t.Fatalf("expected %q after tool_search hydration, got=%v", required, selectedToolNames(got))
		}
	}
}

func TestSelectChatToolsForRequest_ToolSearchHydratesSkillViaExec(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(tools.NewToolSearchTool(registry))
	registry.ExposeDefinition(tools.ToolDefinition{Name: "bash", Description: "Run real shell commands", AlwaysLoad: true})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "exec", Description: "Execute skill and shell commands"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files"})

	handler := newChatToolSelectionTestHandler(registry)

	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	writeSettingsSelectorSkill(t, workspaceDir, "demo_skill", "run a demo skill", `blue demo_skill task=demo`, "demo", "task")
	handler.SetSkillSelector(agentcore.NewSkillSelector(workspaceDir, agentcore.NewHeuristicSkillReranker()))

	searchTool := bindToolSearchTestRuntime(t, handler, nil, workspaceDir)

	ctx := tools.WithSessionID(context.Background(), "conv-skill-hydrate")
	ctx = tools.WithRouteKind(ctx, tools.ToolRouteKindChat)
	if _, err := searchTool.Execute(ctx, map[string]interface{}{"query": "select:demo_skill"}); err != nil {
		t.Fatalf("tool_search Execute returned error: %v", err)
	}

	got := handler.selectChatToolsForRequest(
		context.Background(),
		"Review all files under notes/ and write a summary to out.md.",
		"claude-3-5-haiku-20241022",
		"conv-skill-hydrate",
		"",
		memory.ConversationCommandState{ConversationID: "conv-skill-hydrate"},
		nil,
		nil,
	)

	names := toolNameSet(got)
	for _, required := range []string{"tool_search", "exec"} {
		if _, ok := names[required]; !ok {
			t.Fatalf("expected %q after skill hydration, got=%v", required, selectedToolNames(got))
		}
	}
	if _, ok := names["bash"]; ok {
		t.Fatalf("expected bash to be replaced by exec after skill hydration, got=%v", selectedToolNames(got))
	}
}

func TestSelectChatToolsForRequest_ToolSearchHydratesAgentTools(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(tools.NewToolSearchTool(registry))
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "agents_list", Description: "List agents"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "subagents", Description: "Spawn subagents"})

	handler := newChatToolSelectionTestHandler(registry)
	cfg := &config.Config{
		ToolCalling: *config.DefaultToolCallingConfig(),
		Agents:      *config.DefaultAgentsConfig(),
	}
	cfg.Agents.List = []config.AgentConfig{
		{
			ID:          "worker",
			Enabled:     true,
			Description: "Bounded worker agent",
			Subagents: config.AgentSubagentPolicyConfig{
				Enabled: true,
			},
		},
	}
	searchTool := bindToolSearchTestRuntime(t, handler, cfg, "")

	ctx := tools.WithSessionID(context.Background(), "conv-agent-hydrate")
	ctx = tools.WithRouteKind(ctx, tools.ToolRouteKindChat)
	if _, err := searchTool.Execute(ctx, map[string]interface{}{"query": "select:worker"}); err != nil {
		t.Fatalf("tool_search Execute returned error: %v", err)
	}

	got := handler.selectChatToolsForRequest(
		context.Background(),
		`Say "Hello, I'm ready!" to confirm you can respond.`,
		"claude-3-5-haiku-20241022",
		"conv-agent-hydrate",
		"",
		memory.ConversationCommandState{ConversationID: "conv-agent-hydrate"},
		nil,
		nil,
	)

	names := toolNameSet(got)
	for _, required := range []string{"tool_search", "agents_list", "subagents"} {
		if _, ok := names[required]; !ok {
			t.Fatalf("expected %q after agent hydration, got=%v", required, selectedToolNames(got))
		}
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

func TestSelectTools_PublicStockArtifactKeepsWebQueryAndWrite(t *testing.T) {
	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "bash", Description: "Run real shell commands"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "browser", Description: "Open and interact with web pages"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "web_query", Description: "Research current public web information"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "edit", Description: "Edit workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "ls", Description: "List workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "find", Description: "Find workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "grep", Description: "Search workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "email", Description: "Search inbox messages"})

	handler := newChatToolSelectionTestHandler(registry)

	got := handler.selectTools("Research the current stock price of Apple (AAPL) and save it to stock_report.txt with the price, date, and a brief market summary.", tools.ToolPolicyRequest{
		Model:     "claude-sonnet-4-6",
		RouteKind: tools.ToolRouteKindChat,
	})

	names := toolNameSet(got)
	for _, required := range []string{"web_query", "write", "read"} {
		if _, ok := names[required]; !ok {
			t.Fatalf("expected %q in public stock artifact tool set, got=%v", required, got)
		}
	}
	if _, ok := names["email"]; ok {
		t.Fatalf("expected unrelated productivity tool to stay hidden, got=%v", got)
	}
}

func TestSelectChatToolSurfacesForRequest_GenericDocxResearchKeepsNativeDocxWorkflow(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(tools.NewToolSearchTool(registry))
	registry.ExposeDefinition(tools.ToolDefinition{Name: "browser", Description: "Open and interact with web pages"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "docx", Description: "Read, create, edit, validate, or template native .docx workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "web_query", Description: "Research current public web information"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files"})

	handler := newChatToolSelectionTestHandler(registry)

	selection := handler.selectChatToolSurfacesForRequest(context.Background(), "先做多轮资料研究，再整理成结构化结论，最后生成 .docx 并做一次文件校验。", tools.ToolPolicyRequest{
		Model:     "claude-3-5-haiku-20241022",
		RouteKind: tools.ToolRouteKindChat,
	}, nil, nil)

	if selection.NativeMode != chatNativeToolSurfaceModeLegacy {
		t.Fatalf("NativeMode = %q, want %q", selection.NativeMode, chatNativeToolSurfaceModeLegacy)
	}
	names := toolNameSet(selection.NativeDefs)
	for _, required := range []string{"docx", "read", "web_query", "write", "tool_search"} {
		if _, ok := names[required]; !ok {
			t.Fatalf("expected %q in generic .docx research workflow, got=%v", required, selectedToolNames(selection.NativeDefs))
		}
	}
	if _, ok := names["exec"]; ok {
		t.Fatalf("expected generic .docx research workflow to keep mixed tools instead of exec-only cutover, got=%v", selectedToolNames(selection.NativeDefs))
	}
}

func TestSelectChatToolsForRequest_ExplicitCapabilityTogglesNoLongerFilterTools(t *testing.T) {
	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "deep_research", Description: "Run deep research"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "plan_create", Description: "Create a checklist"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "web_query", Description: "Search the web"})

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
	for _, required := range []string{"deep_research", "plan_create", "read", "web_query"} {
		if _, ok := names[required]; !ok {
			t.Fatalf("expected %q to remain visible despite explicit legacy capability toggles, got=%v", required, got)
		}
	}
}

func TestSelectChatToolsForRequest_SmartSkillSelectionCollapsesToExec(t *testing.T) {
	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "exec", Description: "Execute skill and shell commands"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "web_query", Description: "Search the web"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files"})

	handler := newChatToolSelectionTestHandler(registry)

	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	writeSettingsSelectorCanonicalWebQuerySkill(t, workspaceDir, "search the web for latest docs and official references", `blue web_query input="OpenAI Responses API docs"`, "search", "web", "docs")
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

func TestSelectChatToolSurfacesForRequest_DiscoverFirstUsesRuntimeExecOverlay(t *testing.T) {
	registry := tools.NewRegistry()
	tools.RegisterExecTools(registry, tools.DefaultExecConfig(), nil, nil, nil)
	registry.Register(tools.NewToolSearchTool(registry))

	handler := newChatToolSelectionTestHandler(registry)

	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	writeSettingsSelectorCanonicalWebQuerySkill(t, workspaceDir, "search the web for latest docs and official references", `blue web_query input="OpenAI Responses API docs"`, "search", "web", "docs")
	handler.SetSkillSelector(agentcore.NewSkillSelector(workspaceDir, agentcore.NewHeuristicSkillReranker()))
	handler.ConfigureToolSearchRuntime(workspaceDir, &config.Config{
		ToolCalling: *config.DefaultToolCallingConfig(),
		Agents:      *config.DefaultAgentsConfig(),
	})

	selection := handler.selectChatToolSurfacesForRequest(context.Background(), "搜索最新 OpenAI Responses API 文档。", tools.ToolPolicyRequest{
		Model:     "claude-3-5-haiku-20241022",
		RouteKind: tools.ToolRouteKindChat,
	}, nil, nil)

	if selection.NativeMode != chatNativeToolSurfaceModeSkillExec {
		t.Fatalf("NativeMode = %q, want %q", selection.NativeMode, chatNativeToolSurfaceModeSkillExec)
	}
	if got := selectedToolNames(selection.NativeDefs); len(got) != 2 || got[0] != "exec" || got[1] != "tool_search" {
		t.Fatalf("NativeDefs = %v, want [exec tool_search]", got)
	}
}

func TestSelectChatToolSurfacesForRequest_DefaultSettingsUseDiscoverFirstCutover(t *testing.T) {
	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "exec", Description: "Execute skill and shell commands"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "web_query", Description: "Search the web"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files"})

	handler := newChatToolSelectionTestHandler(registry)

	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	writeSettingsSelectorCanonicalWebQuerySkill(t, workspaceDir, "search the web for latest docs and official references", `blue web_query input="OpenAI Responses API docs"`, "search", "web", "docs", "latest")
	writeSettingsSelectorSkill(t, workspaceDir, "analyze", "analyze reports and urls", `blue analyze topic="url report" --json`, "analysis", "report", "url")

	handler.SetSkillSelector(agentcore.NewSkillSelector(workspaceDir, agentcore.NewHeuristicSkillReranker()))

	selection := handler.selectChatToolSurfacesForRequest(context.Background(), "搜索最新 OpenAI Responses API 文档。", tools.ToolPolicyRequest{
		Model:     "claude-3-5-haiku-20241022",
		RouteKind: tools.ToolRouteKindChat,
	}, nil, nil)

	if selection.NativeMode != chatNativeToolSurfaceModeSkillExec {
		t.Fatalf("NativeMode = %q, want %q", selection.NativeMode, chatNativeToolSurfaceModeSkillExec)
	}
	if got := selectedToolNames(selection.NativeDefs); len(got) != 1 || got[0] != "exec" {
		t.Fatalf("NativeDefs = %v, want [exec]", got)
	}
	if selection.DiscoveryDecision == nil {
		t.Fatal("expected DiscoveryDecision")
	}
	if selection.DiscoveryDecision.CanonicalTarget != agentcore.CanonicalWebQuery {
		t.Fatalf("CanonicalTarget = %q, want %q", selection.DiscoveryDecision.CanonicalTarget, agentcore.CanonicalWebQuery)
	}
}

func TestSelectChatToolsForRequest_SmartSkillClarifyKeepsFallbackToolSearch(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(tools.NewToolSearchTool(registry))
	registry.ExposeDefinition(tools.ToolDefinition{Name: "exec", Description: "Execute skill and shell commands"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "web_query", Description: "Search the web"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files"})

	handler := newChatToolSelectionTestHandler(registry)

	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	writeSettingsSelectorCanonicalWebQuerySkill(t, workspaceDir, "search the web for latest docs and official references", `blue web_query input="OpenAI Responses API docs"`, "search", "web", "docs")
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

	if names := selectedToolNames(got); len(names) != 1 || names[0] != "tool_search" {
		t.Fatalf("selectChatToolsForRequest() = %v, want [tool_search] while clarifying", names)
	}
}

func TestSelectChatToolSurfacesForRequest_DiscoverFirstCanonicalCutovers(t *testing.T) {
	handler := newDiscoverFirstSelectionHandler(t, true)

	tests := []struct {
		name              string
		query             string
		wantCanonical     agentcore.CanonicalSkillID
		wantResearchMode  string
		wantProfile       agentcore.ExecutionProfile
		wantNativeMode    chatNativeToolSurfaceMode
		wantDiscoveryMode agentcore.NativeSurfaceMode
	}{
		{
			name:              "latest_docs_routes_to_web_query",
			query:             "搜索最新 OpenAI Responses API 文档。",
			wantCanonical:     agentcore.CanonicalWebQuery,
			wantProfile:       agentcore.ExecutionProfilePreferFork,
			wantNativeMode:    chatNativeToolSurfaceModeSkillExec,
			wantDiscoveryMode: agentcore.NativeSurfaceModeSkillExec,
		},
		{
			name:              "login_flow_routes_to_browser",
			query:             "打开 https://example.com，登录后再看页面内容。",
			wantCanonical:     agentcore.CanonicalBrowser,
			wantProfile:       agentcore.ExecutionProfileInline,
			wantNativeMode:    chatNativeToolSurfaceModeSkillExec,
			wantDiscoveryMode: agentcore.NativeSurfaceModeSkillExec,
		},
		{
			name:              "report_routes_to_analyze",
			query:             "汇总这几个链接并给我一份报告：https://example.com/a https://example.com/b",
			wantCanonical:     agentcore.CanonicalResearch,
			wantResearchMode:  "analyze",
			wantProfile:       agentcore.ExecutionProfileRequireFork,
			wantNativeMode:    chatNativeToolSurfaceModeSkillExec,
			wantDiscoveryMode: agentcore.NativeSurfaceModeSkillExec,
		},
		{
			name:              "cited_research_routes_to_deep_research",
			query:             "Investigate https://example.com/pricing and compare the claims with citations, evidence, and a timeline.",
			wantCanonical:     agentcore.CanonicalResearch,
			wantResearchMode:  "deep_research",
			wantProfile:       agentcore.ExecutionProfileRequireFork,
			wantNativeMode:    chatNativeToolSurfaceModeSkillExec,
			wantDiscoveryMode: agentcore.NativeSurfaceModeSkillExec,
		},
		{
			name:              "ui_review_routes_to_ui_reviewer",
			query:             "Review the UI of https://example.com/pricing for accessibility and layout issues.",
			wantCanonical:     agentcore.CanonicalResearch,
			wantResearchMode:  "ui_review",
			wantProfile:       agentcore.ExecutionProfileRequireFork,
			wantNativeMode:    chatNativeToolSurfaceModeSkillExec,
			wantDiscoveryMode: agentcore.NativeSurfaceModeSkillExec,
		},
		{
			name:              "workspace_readme_routes_to_exec",
			query:             "看下 workspace 里的 README，并总结一下项目在做什么。",
			wantCanonical:     agentcore.CanonicalExec,
			wantProfile:       agentcore.ExecutionProfileInline,
			wantNativeMode:    chatNativeToolSurfaceModeSkillExec,
			wantDiscoveryMode: agentcore.NativeSurfaceModeSkillExec,
		},
		{
			name:              "ask_routes_to_ask_skill",
			query:             "ask me two clarifying questions before you continue",
			wantCanonical:     agentcore.CanonicalAsk,
			wantProfile:       agentcore.ExecutionProfileInline,
			wantNativeMode:    chatNativeToolSurfaceModeSkillExec,
			wantDiscoveryMode: agentcore.NativeSurfaceModeSkillExec,
		},
		{
			name:              "config_routes_to_config_skill",
			query:             "config providers.list",
			wantCanonical:     agentcore.CanonicalConfig,
			wantProfile:       agentcore.ExecutionProfileInline,
			wantNativeMode:    chatNativeToolSurfaceModeSkillExec,
			wantDiscoveryMode: agentcore.NativeSurfaceModeSkillExec,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			selection := handler.selectChatToolSurfacesForRequest(context.Background(), tc.query, tools.ToolPolicyRequest{
				Model:     "claude-3-5-haiku-20241022",
				RouteKind: tools.ToolRouteKindChat,
			}, nil, nil)

			if selection.NativeMode != tc.wantNativeMode {
				t.Fatalf("NativeMode = %q, want %q", selection.NativeMode, tc.wantNativeMode)
			}
			if got := selectedToolNames(selection.NativeDefs); len(got) != 2 || got[0] != "exec" || got[1] != "tool_search" {
				t.Fatalf("NativeDefs = %v, want [exec tool_search]", got)
			}
			if selection.DiscoveryDecision == nil {
				t.Fatal("expected DiscoveryDecision")
			}
			if selection.DiscoveryDecision.CanonicalTarget != tc.wantCanonical {
				t.Fatalf("CanonicalTarget = %q, want %q", selection.DiscoveryDecision.CanonicalTarget, tc.wantCanonical)
			}
			if tc.wantResearchMode != "" {
				if selection.SkillDecision == nil {
					t.Fatal("expected SkillDecision for research-family selection")
				}
				if selection.SkillDecision.ResearchMode != tc.wantResearchMode {
					t.Fatalf("ResearchMode = %q, want %q", selection.SkillDecision.ResearchMode, tc.wantResearchMode)
				}
			}
			if selection.DiscoveryDecision.ExecutionProfile != tc.wantProfile {
				t.Fatalf("ExecutionProfile = %q, want %q", selection.DiscoveryDecision.ExecutionProfile, tc.wantProfile)
			}
			if selection.DiscoveryDecision.NativeSurfaceMode != tc.wantDiscoveryMode {
				t.Fatalf("Discovery NativeSurfaceMode = %q, want %q", selection.DiscoveryDecision.NativeSurfaceMode, tc.wantDiscoveryMode)
			}
		})
	}
}

func TestSelectChatToolSurfacesForRequest_DiscoverFirstClarifyAndLegacyFlagsDoNotBlockCutover(t *testing.T) {
	handler := newDiscoverFirstSelectionHandler(t, true)

	clarifySelection := handler.selectChatToolSurfacesForRequest(context.Background(), "看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？", tools.ToolPolicyRequest{
		Model:     "claude-3-5-haiku-20241022",
		RouteKind: tools.ToolRouteKindChat,
	}, nil, nil)
	if clarifySelection.NativeMode != chatNativeToolSurfaceModeClarifyNone {
		t.Fatalf("clarify NativeMode = %q, want %q", clarifySelection.NativeMode, chatNativeToolSurfaceModeClarifyNone)
	}
	if got := selectedToolNames(clarifySelection.NativeDefs); len(got) != 1 || got[0] != "tool_search" {
		t.Fatalf("clarify NativeDefs = %v, want [tool_search]", got)
	}
	if clarifySelection.DiscoveryDecision == nil || !clarifySelection.DiscoveryDecision.NeedClarify {
		t.Fatalf("clarify DiscoveryDecision = %#v, want clarify decision", clarifySelection.DiscoveryDecision)
	}

	webSearchEnabled := false
	fallbackSelection := handler.selectChatToolSurfacesForRequest(context.Background(), "搜索最新 OpenAI Responses API 文档。", tools.ToolPolicyRequest{
		Model:     "claude-3-5-haiku-20241022",
		RouteKind: tools.ToolRouteKindChat,
	}, &webSearchEnabled, nil)
	if fallbackSelection.NativeMode != chatNativeToolSurfaceModeSkillExec {
		t.Fatalf("legacy-flag NativeMode = %q, want discover-first skill exec cutover", fallbackSelection.NativeMode)
	}
	if got := selectedToolNames(fallbackSelection.NativeDefs); len(got) != 2 || got[0] != "exec" || got[1] != "tool_search" {
		t.Fatalf("legacy-flag NativeDefs = %v, want [exec tool_search]", got)
	}
	if fallbackSelection.DiscoveryDecision == nil || fallbackSelection.DiscoveryDecision.CanonicalTarget != agentcore.CanonicalWebQuery {
		t.Fatalf("legacy-flag DiscoveryDecision = %#v, want canonical web_query", fallbackSelection.DiscoveryDecision)
	}
}

func TestSelectChatToolSurfacesForRequest_DiscoverFirstStockArtifactStaysActionable(t *testing.T) {
	handler := newDiscoverFirstSelectionHandler(t, true)

	selection := handler.selectChatToolSurfacesForRequest(context.Background(), "Research the current stock price of Apple (AAPL) and save it to stock_report.txt with the price, date, and a brief market summary.", tools.ToolPolicyRequest{
		Model:     "claude-sonnet-4-6",
		RouteKind: tools.ToolRouteKindChat,
	}, nil, nil)

	if selection.NativeMode != chatNativeToolSurfaceModeLegacy {
		t.Fatalf("NativeMode = %q, want %q", selection.NativeMode, chatNativeToolSurfaceModeLegacy)
	}
	if len(selection.NativeDefs) == 0 {
		t.Fatalf("NativeDefs = %v, want actionable tool surface", selectedToolNames(selection.NativeDefs))
	}
	names := toolNameSet(selection.NativeDefs)
	for _, required := range []string{"write", "read"} {
		if _, ok := names[required]; !ok {
			t.Fatalf("NativeDefs = %v, want %q available for artifact workflow", selectedToolNames(selection.NativeDefs), required)
		}
	}
	if _, ok := names["web_query"]; !ok {
		t.Fatalf("NativeDefs = %v, want web retrieval tool for stock research", selectedToolNames(selection.NativeDefs))
	}
	if selection.DiscoveryDecision != nil && selection.DiscoveryDecision.NeedClarify {
		t.Fatalf("DiscoveryDecision = %#v, want no clarify gate for actionable stock artifact request", selection.DiscoveryDecision)
	}
}

func TestSelectChatToolSurfacesForRequest_LocalWorkspaceArtifactKeepsNativeFileWorkflow(t *testing.T) {
	handler := newDiscoverFirstSelectionHandler(t, true)

	selection := handler.selectChatToolSurfacesForRequest(context.Background(), "Review all files in the research/ folder and write a comprehensive daily summary to daily_briefing.md.", tools.ToolPolicyRequest{
		Model:     "claude-sonnet-4-6",
		RouteKind: tools.ToolRouteKindChat,
	}, nil, nil)

	if selection.NativeMode != chatNativeToolSurfaceModeLegacy {
		t.Fatalf("NativeMode = %q, want %q", selection.NativeMode, chatNativeToolSurfaceModeLegacy)
	}
	if len(selection.NativeDefs) == 0 {
		t.Fatalf("NativeDefs = %v, want actionable tool surface", selectedToolNames(selection.NativeDefs))
	}
	names := toolNameSet(selection.NativeDefs)
	for _, required := range []string{"read", "write"} {
		if _, ok := names[required]; !ok {
			t.Fatalf("NativeDefs = %v, want %q available for local workspace artifact workflow", selectedToolNames(selection.NativeDefs), required)
		}
	}
	if _, ok := names["exec"]; ok {
		t.Fatalf("NativeDefs = %v, want local workspace artifact workflow to avoid exec-only cutover", selectedToolNames(selection.NativeDefs))
	}
	if selection.DiscoveryDecision != nil && selection.DiscoveryDecision.NeedClarify {
		t.Fatalf("DiscoveryDecision = %#v, want no clarify gate for local workspace artifact request", selection.DiscoveryDecision)
	}
}

func TestSelectChatToolSurfacesForRequest_GeneralKnowledgeSynthesisDoesNotFallIntoClarifyNone(t *testing.T) {
	handler := newDiscoverFirstSelectionHandler(t, true)

	selection := handler.selectChatToolSurfacesForRequest(context.Background(), "请围绕亚里士多德的公开著作、可信史料和核心思想，整理一套可对话的知识容器。提炼他的概念体系、价值判断、论证方式和常用追问框架，在回答我问题时尽量保持他的思考风格，同时明确区分原典观点、合理推断和现代延伸。", tools.ToolPolicyRequest{
		Model:     "auto",
		RouteKind: tools.ToolRouteKindChat,
	}, nil, nil)

	if selection.NativeMode == chatNativeToolSurfaceModeClarifyNone {
		t.Fatalf("NativeMode = %q, want actionable surface for general knowledge synthesis", selection.NativeMode)
	}
	if len(selection.NativeDefs) == 0 {
		t.Fatalf("NativeDefs = %v, want non-empty actionable tool surface", selectedToolNames(selection.NativeDefs))
	}
	if selection.DiscoveryDecision != nil && selection.DiscoveryDecision.NeedClarify {
		t.Fatalf("DiscoveryDecision = %#v, want no clarify gate for general knowledge synthesis", selection.DiscoveryDecision)
	}
}

func TestSelectChatToolSurfacesForRequest_ImageGenerationPrefersNativeGenerateImageSurface(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(tools.NewToolSearchTool(registry))
	registry.ExposeDefinition(tools.ToolDefinition{Name: "bash", Description: "Run shell commands"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "edit", Description: "Edit workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "find", Description: "Find workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "generate_image", Description: "Generate an image from a prompt and optionally save it to a workspace path."})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "image", Description: "Legacy image review and generation surface"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "ls", Description: "List workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "ocr", Description: "Extract text from images"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files"})

	handler := newChatToolSelectionTestHandler(registry)

	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	writeSettingsSelectorSkill(t, workspaceDir, "mediagen", "Generate images or videos through the built-in media pipeline.", `blue media generate "Draw a cat on a rainbow" --category t2i`, "image", "video", "generate", "draw")

	handler.SetSkillSelector(agentcore.NewSkillSelector(workspaceDir, agentcore.NewHeuristicSkillReranker()))
	handler.ConfigureToolSearchRuntime(workspaceDir, &config.Config{
		ToolCalling: *config.DefaultToolCallingConfig(),
		Agents:      *config.DefaultAgentsConfig(),
	})

	selection := handler.selectChatToolSurfacesForRequest(context.Background(), `Generate an image of a friendly robot sitting in a cozy coffee shop, reading a book. Save it as "robot_cafe.png" in the current directory.`, tools.ToolPolicyRequest{
		Model:     "claude-sonnet-4-6",
		RouteKind: tools.ToolRouteKindChat,
	}, nil, nil)

	if selection.NativeMode != chatNativeToolSurfaceModeLegacy {
		t.Fatalf("NativeMode = %q, want %q", selection.NativeMode, chatNativeToolSurfaceModeLegacy)
	}
	names := toolNameSet(selection.NativeDefs)
	for _, required := range []string{"generate_image", "read", "write", "edit", "ls", "find", "tool_search", "bash"} {
		if _, ok := names[required]; !ok {
			t.Fatalf("NativeDefs = %v, want %q in native image-generation surface", selectedToolNames(selection.NativeDefs), required)
		}
	}
	if _, ok := names["ocr"]; ok {
		t.Fatalf("NativeDefs = %v, want OCR excluded from direct image-generation surface", selectedToolNames(selection.NativeDefs))
	}
	if selection.DiscoveryDecision != nil && selection.DiscoveryDecision.NeedClarify {
		t.Fatalf("DiscoveryDecision = %#v, want no clarify gate for native image generation", selection.DiscoveryDecision)
	}
	if selection.SkillDecision != nil {
		t.Fatalf("SkillDecision = %#v, want native image generation to bypass discover-first skill routing", selection.SkillDecision)
	}
}

func TestSelectChatToolSurfacesForRequest_LegacyExecCollapsePersistsWhenDynamicExposureDisabled(t *testing.T) {
	handler := newDiscoverFirstSelectionHandler(t, false)

	selection := handler.selectChatToolSurfacesForRequest(context.Background(), "搜索最新 OpenAI Responses API 文档。", tools.ToolPolicyRequest{
		Model:     "claude-3-5-haiku-20241022",
		RouteKind: tools.ToolRouteKindChat,
	}, nil, nil)
	if selection.NativeMode != chatNativeToolSurfaceModeSkillExec {
		t.Fatalf("NativeMode = %q, want skill_exec while dynamic exposure is disabled", selection.NativeMode)
	}
	if got := selectedToolNames(selection.NativeDefs); len(got) != 2 || got[0] != "exec" || got[1] != "tool_search" {
		t.Fatalf("NativeDefs = %v, want [exec tool_search]", got)
	}
	if selection.DiscoveryDecision == nil {
		t.Fatal("expected DiscoveryDecision")
	}
	if selection.DiscoveryDecision.CanonicalTarget != agentcore.CanonicalWebQuery {
		t.Fatalf("CanonicalTarget = %q, want %q", selection.DiscoveryDecision.CanonicalTarget, agentcore.CanonicalWebQuery)
	}
	if selection.DiscoveryDecision.ExecutionProfile != agentcore.ExecutionProfilePreferFork {
		t.Fatalf("ExecutionProfile = %q, want %q", selection.DiscoveryDecision.ExecutionProfile, agentcore.ExecutionProfilePreferFork)
	}
}

func TestSelectChatToolsForRequest_CutoverSkipsPromptCacheStickyUnion(t *testing.T) {
	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "exec", Description: "Execute skill and shell commands"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "web_query", Description: "Search the web"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files"})

	handler := newChatToolSelectionTestHandler(registry)

	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	writeSettingsSelectorCanonicalWebQuerySkill(t, workspaceDir, "search the web for latest docs and official references", `blue web_query input="OpenAI Responses API docs"`, "search", "web", "docs")
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
		ProviderID: "anthropic-test",
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

func TestSelectChatToolsForRequest_DiscoverFirstCutoverSkipsPromptCacheStickyUnion(t *testing.T) {
	handler := newDiscoverFirstSelectionHandler(t, true)
	attachTestProviderPool(t, handler, &providerpool.Provider{
		ID:        "anthropic-test",
		Name:      "Anthropic Test",
		Type:      providerpool.ProviderTypeCustom,
		Location:  providerpool.ProviderLocationCloud,
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		APIFormat: providerpool.APIFormatAnthropic,
	})

	handler.setPromptCacheToolSurface("conv-discover-cutover", &promptCacheToolSurface{
		ProviderID: "anthropic-test",
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
		"conv-discover-cutover",
		"",
		memory.ConversationCommandState{
			ConversationID:     "conv-discover-cutover",
			SelectedProviderID: "anthropic-test",
			WebSearchEnabled:   true,
		},
		nil,
		nil,
	)

	if names := selectedToolNames(got); len(names) != 2 || names[0] != "exec" || names[1] != "tool_search" {
		t.Fatalf("selectChatToolsForRequest() = %v, want sticky surface reset to [exec tool_search]", names)
	}
	if cached := handler.getPromptCacheToolSurface("conv-discover-cutover"); cached != nil {
		t.Fatalf("prompt cache surface = %#v, want cleared after discover-first cutover", cached)
	}
}

func TestSelectChatToolSurfacesForRequest_RecordsToolSurfaceAuditCounts(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(tools.NewToolSearchTool(registry))
	tools.RegisterExecTools(registry, tools.DefaultExecConfig(), nil, nil, nil)
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files"})

	handler := newChatToolSelectionTestHandler(registry)
	bindToolSearchTestRuntime(t, handler, nil, "")

	if _, ok := handler.deferredToolExposure.Apply("conv-audit", tools.DeferredToolExposureUpdate{
		NeedExec:         true,
		RegistryVersion:  registry.Version(),
		PromptPolicyHash: handler.resolvePromptPolicy().Hash,
	}); !ok {
		t.Fatal("expected deferred tool exposure update to apply")
	}

	first := handler.selectChatToolSurfacesForRequest(context.Background(), "Review the workspace and continue.", tools.ToolPolicyRequest{
		Model:     "claude-3-5-haiku-20241022",
		SessionID: "conv-audit",
		RouteKind: tools.ToolRouteKindChat,
	}, nil, nil)
	firstNames := toolNameSet(first.NativeDefs)
	for _, required := range []string{"exec", "read", "tool_search", "write"} {
		if _, ok := firstNames[required]; !ok {
			t.Fatalf("first native defs = %v, want exec/read/tool_search/write", selectedToolNames(first.NativeDefs))
		}
	}
	if _, ok := firstNames["bash"]; ok {
		t.Fatalf("first native defs = %v, want bash to be replaced during deferred exec cutover", selectedToolNames(first.NativeDefs))
	}
	firstSnapshot := handler.toolSurfaceAudit.Snapshot()
	if firstSnapshot.ExecCutoverCount != 1 {
		t.Fatalf("exec cutover count = %d, want 1", firstSnapshot.ExecCutoverCount)
	}
	if firstSnapshot.CacheInvalidationCount != 0 {
		t.Fatalf("cache invalidation count = %d, want 0", firstSnapshot.CacheInvalidationCount)
	}

	registry.ExposeDefinition(tools.ToolDefinition{Name: "calendar", Description: "Calendar scheduling"})
	second := handler.selectChatToolSurfacesForRequest(context.Background(), "Review the workspace and continue.", tools.ToolPolicyRequest{
		Model:     "claude-3-5-haiku-20241022",
		SessionID: "conv-audit",
		RouteKind: tools.ToolRouteKindChat,
	}, nil, nil)
	secondNames := toolNameSet(second.NativeDefs)
	for _, required := range []string{"bash", "read", "tool_search", "write"} {
		if _, ok := secondNames[required]; !ok {
			t.Fatalf("second native defs = %v, want bash/read/tool_search/write", selectedToolNames(second.NativeDefs))
		}
	}
	if _, ok := secondNames["exec"]; ok {
		t.Fatalf("second native defs = %v, want exec removed after cache invalidation", selectedToolNames(second.NativeDefs))
	}
	secondSnapshot := handler.toolSurfaceAudit.Snapshot()
	if secondSnapshot.CacheInvalidationCount != 1 {
		t.Fatalf("cache invalidation count = %d, want 1", secondSnapshot.CacheInvalidationCount)
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
		{Name: "web_query"},
		{Name: "read"},
	})
	if got := selectedToolNames(first); len(got) != 2 || got[0] != "read" || got[1] != "web_query" {
		t.Fatalf("first stabilize = %v, want [read web_query]", got)
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

func TestPromptCacheToolSurface_SharedAcrossConversations(t *testing.T) {
	handler := newChatToolSelectionTestHandler(tools.NewRegistry())
	defer handler.Close()

	surface := &promptCacheToolSurface{
		ProviderID:       "anthropic-test",
		RegistryVersion:  7,
		PromptPolicyHash: "policy-1",
		Tools: []tools.ToolDefinition{
			{Name: "read", Description: "Read files"},
			{Name: "write", Description: "Write files"},
		},
		ExpiresAt: time.Now().Add(time.Minute),
	}

	for i := 0; i < 100; i++ {
		handler.setPromptCacheToolSurface(fmt.Sprintf("conv-%03d", i), surface)
	}

	footprint := handler.ChatCacheFootprint()
	if footprint.PromptToolSurfaceRefs != 100 {
		t.Fatalf("prompt tool surface refs = %d, want 100", footprint.PromptToolSurfaceRefs)
	}
	if footprint.PromptToolSurfaceSharedEntries != 1 {
		t.Fatalf("prompt tool surface shared entries = %d, want 1", footprint.PromptToolSurfaceSharedEntries)
	}

	cached := handler.getPromptCacheToolSurface("conv-042")
	if cached == nil {
		t.Fatal("expected shared prompt tool surface to remain available")
	}
	if got := selectedToolNames(cached.Tools); len(got) != 2 || got[0] != "read" || got[1] != "write" {
		t.Fatalf("shared prompt tool surface tools = %v, want [read write]", got)
	}
}
