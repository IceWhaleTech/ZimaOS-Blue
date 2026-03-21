package server

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

func TestNormalizeAssistantToolCallNameForAllowedSet_RewritesCompatAlias(t *testing.T) {
	allowed := []llm.Tool{{Name: "file_write"}}
	if got := normalizeAssistantToolCallNameForAllowedSet(" write ", allowed); got != "file_write" {
		t.Fatalf("normalizeAssistantToolCallNameForAllowedSet() = %q, want %q", got, "file_write")
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
