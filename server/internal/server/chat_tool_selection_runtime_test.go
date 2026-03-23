package server

import (
	"context"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestSelectTools_DefaultRuntimePrefersRelevantSubset(t *testing.T) {
	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "email", Description: "Email inbox search and triage"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "calendar", Description: "Calendar scheduling and agenda"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "deep_research", Description: "Run deep research or check an existing research job status"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "web_search", Description: "Search the web"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "process", Description: "Inspect long-running processes"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "exec", Description: "Run terminal commands"})

	handler := NewChatHandler(nil, nil, registry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))
	handler.SetToolSelector(tools.DefaultToolSelector())
	handler.SetToolRouter(tools.DefaultToolRouter())

	got := handler.selectTools("Archive unread emails from Alice", tools.ToolPolicyRequest{
		Model:     "claude-3-5-haiku-20241022",
		RouteKind: tools.ToolRouteKindChat,
	})
	if len(got) == 0 {
		t.Fatal("expected non-empty tool selection")
	}
	if len(got) >= len(registry.Definitions()) {
		t.Fatalf("expected compact tool subset, got %d tools from %d defs", len(got), len(registry.Definitions()))
	}

	names := make(map[string]bool, len(got))
	for _, def := range got {
		names[def.Name] = true
	}
	if !names["email"] {
		t.Fatalf("expected email tool in selected set, got=%v", got)
	}
	if names["process"] {
		t.Fatalf("expected process tool to stay hidden for email request, got=%v", got)
	}
}

func TestSelectTools_DefaultRuntimeSkipsToolsForPlainReply(t *testing.T) {
	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "exec", Description: "Run terminal commands"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "gateway", Description: "Inspect gateway status"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "ppt", Description: "Generate presentation slide assets"})

	handler := NewChatHandler(nil, nil, registry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))
	handler.SetToolSelector(tools.DefaultToolSelector())
	handler.SetToolRouter(tools.DefaultToolRouter())

	got := handler.selectTools(`Say "Hello, I'm ready!" to confirm you can respond.`, tools.ToolPolicyRequest{
		Model:     "claude-3-5-haiku-20241022",
		RouteKind: tools.ToolRouteKindChat,
	})
	if len(got) != 0 {
		t.Fatalf("expected plain reply prompt to avoid tool exposure, got=%v", got)
	}
}

func TestSelectTools_DefaultRuntimePrefersWorkspaceFileWorkflow(t *testing.T) {
	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "calendar", Description: "Calendar scheduling and agenda"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "email", Description: "Email inbox search and triage"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "deep_research", Description: "Run deep research or check an existing research job status"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "ls", Description: "List workspace directories"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "find", Description: "Find text in files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "convert", Description: "Parse CSV and XLSX spreadsheets"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "pdf", Description: "Read PDF files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "exec", Description: "Run terminal commands"})

	handler := NewChatHandler(nil, nil, registry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))
	handler.SetToolSelector(tools.DefaultToolSelector())
	handler.SetToolRouter(tools.DefaultToolRouter())

	got := handler.selectTools("Review all files in the research/ folder and write a daily summary to daily_briefing.md.", tools.ToolPolicyRequest{
		Model:     "claude-3-5-haiku-20241022",
		RouteKind: tools.ToolRouteKindChat,
	})
	if len(got) == 0 {
		t.Fatal("expected non-empty tool selection")
	}

	names := make(map[string]bool, len(got))
	for _, def := range got {
		names[def.Name] = true
	}
	if !names["read"] || !names["write"] || !names["ls"] || !names["find"] {
		t.Fatalf("expected file workflow tools in selected set, got=%v", got)
	}
	if names["calendar"] || names["email"] || names["deep_research"] || names["exec"] {
		t.Fatalf("expected workspace file task to avoid calendar/email/research tools, got=%v", got)
	}
}

func TestSelectTools_ResearchReportKeepsResearchAndWriteTools(t *testing.T) {
	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "deep_research", Description: "Run deep research or check an existing research job status"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "browser", Description: "Fallback browser"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "web_search", Description: "Search the web"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "ls", Description: "List workspace directories"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "find", Description: "Find text in files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "grep", Description: "Search file content"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "convert", Description: "Convert local files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "exec", Description: "Run terminal commands"})

	handler := NewChatHandler(nil, nil, registry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))
	handler.SetToolSelector(tools.DefaultToolSelector())
	handler.SetToolRouter(tools.DefaultToolRouter())

	got := handler.selectTools("Create a competitive market research report with sources and save it to market_research.md.", tools.ToolPolicyRequest{
		Model:     "claude-3-5-haiku-20241022",
		RouteKind: tools.ToolRouteKindChat,
	})
	if len(got) == 0 {
		t.Fatal("expected non-empty tool selection")
	}

	names := make(map[string]bool, len(got))
	for _, def := range got {
		names[def.Name] = true
	}
	if !names["write"] {
		t.Fatalf("expected research report flow to keep write available, got=%v", got)
	}
	if !names["deep_research"] && !names["web_search"] {
		t.Fatalf("expected research report flow to keep research discovery tools, got=%v", got)
	}
}

func TestSelectTools_WorkspaceCodingTaskKeepsExec(t *testing.T) {
	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "ls", Description: "List workspace directories"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "find", Description: "Find text in files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "exec", Description: "Run terminal commands"})

	handler := NewChatHandler(nil, nil, registry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))
	handler.SetToolSelector(tools.DefaultToolSelector())
	handler.SetToolRouter(tools.DefaultToolRouter())

	got := handler.selectTools("In this workspace, run the test suite and debug the failing build.", tools.ToolPolicyRequest{
		Model:     "claude-3-5-haiku-20241022",
		RouteKind: tools.ToolRouteKindChat,
	})
	if len(got) == 0 {
		t.Fatal("expected non-empty tool selection")
	}

	names := make(map[string]bool, len(got))
	for _, def := range got {
		names[def.Name] = true
	}
	if !names["exec"] {
		t.Fatalf("expected coding task to keep exec available, got=%v", got)
	}
}

func TestSelectTools_DeepResearchToggleForceExposesResearchAskAndFileOps(t *testing.T) {
	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "ask", Description: "Ask user preference questions"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "memory", Description: "Recall prior context"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "exec", Description: "Run terminal commands"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "deep_research", Description: "Run deep research or check an existing research job status"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "web_search", Description: "Search the web"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "web_fetch", Description: "Fetch a known page"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "web_read", Description: "Read a normalized page"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "web_crawl", Description: "Crawl web sources"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "browser", Description: "Fallback browser"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "ls", Description: "List workspace directories"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "find", Description: "Find text in files"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "grep", Description: "Search file content"})

	handler := NewChatHandler(nil, nil, registry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))
	handler.SetToolSelector(tools.DefaultToolSelector())
	handler.SetToolRouter(tools.DefaultToolRouter())

	enabled := true
	got := handler.selectTools("投资人问，你们blue在memory layer做了哪些创新？为什么它的上下文治理能力比openclaw好？我们具体现有方案和它的区别是什么？Agent行业SOTA的方案是什么？", tools.ToolPolicyRequest{
		Model:               "claude-3-5-haiku-20241022",
		RouteKind:           tools.ToolRouteKindChat,
		DeepResearchEnabled: &enabled,
	})
	names := make(map[string]bool, len(got))
	for _, def := range got {
		names[def.Name] = true
	}
	for _, required := range []string{"ask", "deep_research", "read", "write", "ls", "find"} {
		if !names[required] {
			t.Fatalf("expected %s in selected set, got=%v", required, got)
		}
	}
	if !names["browser"] && !names["web_search"] && !names["web_fetch"] && !names["web_read"] && !names["web_crawl"] {
		t.Fatalf("expected at least one web research tool in selected set, got=%v", got)
	}
}

func TestResolveSkillSelectionForRequest_DeepResearchToggleForcesSkill(t *testing.T) {
	handler := NewChatHandler(nil, nil, tools.NewRegistry())

	enabled := true
	prompt, selectedSkill := handler.resolveSkillSelectionForRequest(
		context.Background(),
		"投资人问，你们blue在memory layer做了哪些创新？为什么它的上下文治理能力比openclaw好？我们具体现有方案和它的区别是什么？Agent行业SOTA的方案是什么？",
		&enabled,
	)
	if selectedSkill != "deep_research" {
		t.Fatalf("expected deep_research skill, got %q", selectedSkill)
	}
	if !strings.Contains(prompt, `stage="forced"`) || !strings.Contains(prompt, `selected="deep_research"`) {
		t.Fatalf("expected forced deep_research skill hint, got %q", prompt)
	}
}

func TestShouldForceRequestAgentMode_DeepResearchToggleRequiresStructuredResearchCue(t *testing.T) {
	enabled := true
	disabled := false

	if !shouldForceRequestAgentMode("What are the current best practices for agent memory systems?", &enabled) {
		t.Fatal("expected strong deep research prompt to force request agent mode")
	}
	if shouldForceRequestAgentMode("请用一句话比较 A 和 B。", &enabled) {
		t.Fatal("expected short comparative prompt not to force request agent mode")
	}
	if shouldForceRequestAgentMode("What are the current best practices for agent memory systems?", &disabled) {
		t.Fatal("expected disabled deep research toggle not to force request agent mode")
	}
}
