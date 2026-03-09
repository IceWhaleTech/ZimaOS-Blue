package tools

import (
	"context"
	"testing"
)

func TestExecutorTraceStoreRecordsCalls(t *testing.T) {
	registry := NewRegistry()
	mock := NewMockTool("hello", "hello tool")
	mock.SetResult(map[string]interface{}{"ok": true})
	registry.Register(mock)

	executor := NewExecutor(registry)
	store := NewToolTraceStore(10)
	executor.SetTraceStore(store)

	if _, err := executor.Execute(context.Background(), "hello", map[string]interface{}{"name": "blue"}); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	records := store.List(1)
	if len(records) != 1 {
		t.Fatalf("expected 1 trace record, got %d", len(records))
	}
	if records[0].RequestedTool != "hello" || records[0].ActualTool != "hello" {
		t.Fatalf("unexpected trace record: %#v", records[0])
	}
}
