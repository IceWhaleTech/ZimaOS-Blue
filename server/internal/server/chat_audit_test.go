package server

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sessionaudit"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type auditEchoTool struct{}

func (t *auditEchoTool) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{
		Name:        "audit_echo",
		Description: "echo test tool",
	}
}

func (t *auditEchoTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{
		"ok": true,
		"q":  args["q"],
	}, nil
}

func TestExecuteToolCalls_PersistsRawPayloadAudit(t *testing.T) {
	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(&auditEchoTool{})

	handler := NewChatHandler(nil, llm.NewProviderRegistry(), toolRegistry)
	defer handler.Shutdown()

	auditStore, err := sessionaudit.NewSQLiteStore(filepath.Join(t.TempDir(), "session_audit.db"), sessionaudit.StoreConfig{
		RetentionDays:    30,
		CleanupInterval:  0,
		CleanupBatchSize: 100,
	})
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	defer auditStore.Close()

	handler.SetSessionAuditStore(auditStore)

	ctx := tools.WithSessionID(context.Background(), "conv-audit")
	ctx = tools.WithUserID(ctx, "user-audit")
	ctx = tools.WithChannel(ctx, "web")

	tc := llm.ToolCall{
		ID:        "tc-1",
		Name:      "audit_echo",
		Arguments: `{"q":"ping"}`,
	}
	results := handler.executeToolCalls(ctx, []llm.ToolCall{tc})
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}

	entries, err := auditStore.Recent(context.Background(), "conv-audit", 10)
	if err != nil {
		t.Fatalf("Recent() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}

	var (
		callEntry   sessionaudit.Entry
		resultEntry sessionaudit.Entry
		hasCall     bool
		hasResult   bool
	)
	for i := range entries {
		switch entries[i].EventType {
		case "assistant_tool_call":
			callEntry = entries[i]
			hasCall = true
		case "tool_result":
			resultEntry = entries[i]
			hasResult = true
		}
	}
	if !hasCall {
		t.Fatalf("assistant_tool_call audit entry missing")
	}
	if !hasResult {
		t.Fatalf("tool_result audit entry missing")
	}

	if callEntry.Payload != tc.Arguments {
		t.Fatalf("tool call payload = %q, want %q", callEntry.Payload, tc.Arguments)
	}
	if resultEntry.Payload != results[0].Content {
		t.Fatalf("tool result payload = %q, want %q", resultEntry.Payload, results[0].Content)
	}
	if resultEntry.Role != string(llm.RoleTool) {
		t.Fatalf("tool result role = %q, want %q", resultEntry.Role, llm.RoleTool)
	}
}
