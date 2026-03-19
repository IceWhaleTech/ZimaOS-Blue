package tools

import (
	"context"
	"testing"
)

type traceTestTool struct{}

func (traceTestTool) Definition() ToolDefinition {
	return ToolDefinition{Name: "trace_test", Parameters: map[string]interface{}{"type": "object"}}
}

func (traceTestTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func TestExecutorTraceIncludesRunMetadata(t *testing.T) {
	registry := NewRegistry()
	registry.Register(traceTestTool{})
	executor := NewExecutor(registry)
	store := NewToolTraceStore(10)
	executor.SetTraceStore(store)

	ctx := WithRunID(context.Background(), "run-123")
	ctx = WithRunStep(ctx, 7)
	if _, err := executor.Execute(ctx, "trace_test", map[string]interface{}{"command": "blue memory search"}); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	records := store.List(10)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].RunID != "run-123" {
		t.Fatalf("RunID = %q, want %q", records[0].RunID, "run-123")
	}
	if records[0].StepIndex != 7 {
		t.Fatalf("StepIndex = %d, want %d", records[0].StepIndex, 7)
	}
	if records[0].CapabilityKind != "tool" {
		t.Fatalf("CapabilityKind = %q, want %q", records[0].CapabilityKind, "tool")
	}
}
