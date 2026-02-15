package memory

import (
	"testing"
)

func TestProgressiveSearchToolDefinition(t *testing.T) {
	tool := &MemoryProgressiveSearchTool{}
	def := tool.Definition()

	if def.Name != "memory_search_progressive" {
		t.Errorf("expected name 'memory_search_progressive', got %s", def.Name)
	}
	if def.Description == "" {
		t.Error("expected non-empty description")
	}
	if def.Parameters == nil {
		t.Error("expected non-nil parameters")
	}

	props, ok := def.Parameters["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected properties map")
	}
	for _, required := range []string{"query", "depth", "ids", "limit"} {
		if _, ok := props[required]; !ok {
			t.Errorf("expected property %q", required)
		}
	}
}

func TestProgressiveSearchToolExecute_NilSearcher(t *testing.T) {
	tool := &MemoryProgressiveSearchTool{searcher: nil}
	_, err := tool.Execute(nil, map[string]interface{}{
		"query": "test",
		"depth": float64(1),
	})
	if err == nil {
		t.Error("expected error when searcher is nil")
	}
}
