package tools

import (
	"context"
	"testing"
	"time"
)

func TestRegisterMemoryToolsRegistersCompatWrappers(t *testing.T) {
	registry := NewRegistry()
	service := &mockMemoryService{backend: "local"}

	RegisterMemoryTools(registry, service)

	if registry.Get("memory") == nil {
		t.Fatal("expected unified memory tool to be registered")
	}
	for _, name := range []string{"memory_search", "memory_get", "memory_write", "memory_forget"} {
		if registry.Get(name) == nil {
			t.Fatalf("expected compat wrapper %q to be registered", name)
		}
		if !registry.IsDisabled(name) {
			t.Fatalf("expected compat wrapper %q to be hidden", name)
		}
	}
}

func TestMemoryCompatSearchToolReturnsStructuredPayload(t *testing.T) {
	now := time.Now()
	service := &mockMemoryService{
		recallResults: []MemorySearchResult{{
			Chunk: MemoryChunkResult{
				ID:        "m1",
				Content:   "Remember this",
				Metadata:  map[string]string{"tag": "test"},
				CreatedAt: now,
				UpdatedAt: now,
			},
			CombinedScore: 0.91,
			MatchTypes:    []string{"keyword"},
		}},
		backend: "local",
	}
	tool := newMemoryCompatTool("memory_search", "Search memory snippets by query.", service)

	result, err := tool.Execute(context.Background(), map[string]interface{}{"query": "remember", "limit": 5})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want map", result)
	}
	if payload["query"] != "remember" {
		t.Fatalf("query = %v, want remember", payload["query"])
	}
	if payload["backend"] != "local" {
		t.Fatalf("backend = %v, want local", payload["backend"])
	}
}

func TestMemoryCompatExecutorUsesNativeWrapper(t *testing.T) {
	registry := NewRegistry()
	service := &mockMemoryService{backend: "local", rememberResult: &MemoryChunkResult{ID: "m1", Content: "hello", CreatedAt: time.Now(), UpdatedAt: time.Now()}}
	RegisterMemoryTools(registry, service)
	executor := NewExecutor(registry)

	result, err := executor.Execute(context.Background(), "memory_write", map[string]interface{}{"content": "hello", "category": "prefs"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want map", result)
	}
	if payload["success"] != true {
		t.Fatalf("success = %v, want true", payload["success"])
	}
	if service.lastRememberContent != "hello" {
		t.Fatalf("content = %q, want hello", service.lastRememberContent)
	}
	if len(service.lastRememberTags) != 1 || service.lastRememberTags[0] != "prefs" {
		t.Fatalf("tags = %#v, want [prefs]", service.lastRememberTags)
	}
}

func TestMemoryCompatWriteSupportsNestedTagsArgs(t *testing.T) {
	service := &mockMemoryService{backend: "local", rememberResult: &MemoryChunkResult{ID: "m3", Content: "nested", CreatedAt: time.Now(), UpdatedAt: time.Now()}}
	tool := newMemoryCompatTool("memory_write", "Write a memory entry.", service)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"text": "nested note",
			"tags": []interface{}{"prefs", "work"},
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if service.lastRememberContent != "nested note" {
		t.Fatalf("content = %q, want nested note", service.lastRememberContent)
	}
	if len(service.lastRememberTags) != 2 || service.lastRememberTags[0] != "prefs" || service.lastRememberTags[1] != "work" {
		t.Fatalf("tags = %#v, want [prefs work]", service.lastRememberTags)
	}
}

func TestMemoryCompatLegacyAliasesExecuteNatively(t *testing.T) {
	registry := NewRegistry()
	service := &mockMemoryService{backend: "local", rememberResult: &MemoryChunkResult{ID: "m2", Content: "legacy", CreatedAt: time.Now(), UpdatedAt: time.Now()}}
	RegisterMemoryTools(registry, service)
	executor := NewExecutor(registry)

	if _, err := executor.Execute(context.Background(), "memory_store", map[string]interface{}{"text": "legacy note", "tag": "prefs"}); err != nil {
		t.Fatalf("memory_store execute returned error: %v", err)
	}
	if service.lastRememberContent != "legacy note" {
		t.Fatalf("content = %q, want legacy note", service.lastRememberContent)
	}
	if len(service.lastRememberTags) != 1 || service.lastRememberTags[0] != "prefs" {
		t.Fatalf("tags = %#v, want [prefs]", service.lastRememberTags)
	}

	service.getResult = &MemoryChunkResult{ID: "m2", Content: "legacy", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if _, err := executor.Execute(context.Background(), "memory_read", map[string]interface{}{"path": "m2"}); err != nil {
		t.Fatalf("memory_read execute returned error: %v", err)
	}
	if service.lastGetID != "m2" {
		t.Fatalf("lastGetID = %q, want m2", service.lastGetID)
	}

	if _, err := executor.Execute(context.Background(), "memory_delete", map[string]interface{}{"id": "m2"}); err != nil {
		t.Fatalf("memory_delete execute returned error: %v", err)
	}
	if service.lastForgetID != "m2" {
		t.Fatalf("lastForgetID = %q, want m2", service.lastForgetID)
	}
}
