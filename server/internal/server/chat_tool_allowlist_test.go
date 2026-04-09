package server

import (
	"encoding/json"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

func TestNormalizeAssistantToolCallNameForAllowedSet_RewritesCompatAlias(t *testing.T) {
	allowed := []llm.Tool{{Name: "file_write"}}
	if got := normalizeAssistantToolCallNameForAllowedSet(" write ", allowed); got != "file_write" {
		t.Fatalf("normalizeAssistantToolCallNameForAllowedSet() = %q, want %q", got, "file_write")
	}
}

func TestNormalizeAssistantToolCallNameForAllowedSet_RewritesDeepResearchAliases(t *testing.T) {
	allowed := []llm.Tool{{Name: "research"}}
	for _, raw := range []string{"research_run", "research_status", "deep-research", "deep_research"} {
		if got := normalizeAssistantToolCallNameForAllowedSet(raw, allowed); got != "research" {
			t.Fatalf("normalizeAssistantToolCallNameForAllowedSet(%q) = %q, want %q", raw, got, "research")
		}
	}
}

func TestSanitizeAssistantToolCallsForAllowedSet_DropsCallsOutsideAllowedSet(t *testing.T) {
	allowed := []llm.Tool{{Name: "file_write"}}
	calls := []llm.ToolCall{
		{ID: "call-read", Name: " file_read ", Arguments: `{"path":"source.txt"}`},
		{ID: "", Name: " write ", Arguments: `{"path":"answer.txt","content":"done"}`},
	}

	got, dropped := sanitizeAssistantToolCallsForAllowedSet(calls, allowed)
	if len(dropped) != 1 || dropped[0] != "file_read" {
		t.Fatalf("dropped = %#v, want %#v", dropped, []string{"file_read"})
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].Name != "file_write" {
		t.Fatalf("tool name = %q, want %q", got[0].Name, "file_write")
	}
	if got[0].ID != "call_auto_1" {
		t.Fatalf("tool id = %q, want %q", got[0].ID, "call_auto_1")
	}
}

func TestSanitizeAssistantToolCallsForAllowedSet_RewritesDisallowedToToolSearchWhenPresent(t *testing.T) {
	allowed := []llm.Tool{{Name: "exec"}, {Name: "tool_search"}}
	calls := []llm.ToolCall{
		{ID: "call-1", Name: "deep_research", Arguments: `{"query":"x"}`},
	}

	got, dropped := sanitizeAssistantToolCallsForAllowedSet(calls, allowed)
	if len(dropped) != 0 {
		t.Fatalf("dropped = %#v, want none", dropped)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].Name != "tool_search" {
		t.Fatalf("tool name = %q, want %q", got[0].Name, "tool_search")
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(got[0].Arguments), &args); err != nil {
		t.Fatalf("failed to decode tool_search args: %v", err)
	}
	if args["query"] != "select:research" {
		t.Fatalf("args.query = %#v, want %#v", args["query"], "select:research")
	}
}

func TestSanitizeAssistantToolCallsForAllowedSet_DropsAllCallsWhenAllowedSetEmpty(t *testing.T) {
	calls := []llm.ToolCall{
		{ID: "call-1", Name: "exec", Arguments: `{"cmd":"pwd"}`},
		{ID: "call-2", Name: "web_query", Arguments: `{"query":"hello"}`},
	}

	got, dropped := sanitizeAssistantToolCallsForAllowedSet(calls, nil)
	if len(got) != 0 {
		t.Fatalf("len(got) = %d, want 0", len(got))
	}
	if len(dropped) != 2 || dropped[0] != "exec" || dropped[1] != "web_query" {
		t.Fatalf("dropped = %#v, want %#v", dropped, []string{"exec", "web_query"})
	}
}
