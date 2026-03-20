package server

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestSelectTools_DefaultRuntimePrefersRelevantSubset(t *testing.T) {
	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "email", Description: "Email inbox search and triage"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "calendar", Description: "Calendar scheduling and agenda"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "research_run", Description: "Deep research with sources"})
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
	registry.ExposeDefinition(tools.ToolDefinition{Name: "research_run", Description: "Deep research with sources"})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "research_status", Description: "Deep research status"})
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
	if names["calendar"] || names["email"] || names["research_run"] || names["exec"] {
		t.Fatalf("expected workspace file task to avoid calendar/email/research tools, got=%v", got)
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
