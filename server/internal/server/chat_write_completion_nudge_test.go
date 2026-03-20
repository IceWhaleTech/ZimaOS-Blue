package server

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

func TestBuildPostWriteCompletionNudge_ForWorkspaceSummaryWrite(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "write"},
	}
	toolResults := []llm.Message{
		{Role: llm.RoleTool, ToolCallID: "call-1", Content: `{"path":"daily_briefing.md","size":2168,"success":true,"append":false}`},
	}

	nudge := buildPostWriteCompletionNudge(
		"Review all files in the research/ folder and write a daily summary to daily_briefing.md.",
		toolCalls,
		toolResults,
	)
	if nudge == "" {
		t.Fatal("expected non-empty nudge for successful workspace write")
	}
	if want := `daily_briefing.md`; !containsSubstring(nudge, want) {
		t.Fatalf("expected nudge to mention target path %q, got=%q", want, nudge)
	}
}

func TestBuildPostWriteCompletionNudge_SkipsCodingFlow(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "write"},
	}
	toolResults := []llm.Message{
		{Role: llm.RoleTool, ToolCallID: "call-1", Content: `{"path":"main.go","size":128,"success":true,"append":false}`},
	}

	nudge := buildPostWriteCompletionNudge(
		"In this workspace, update main.go and then run the test suite to fix the failing build.",
		toolCalls,
		toolResults,
	)
	if nudge != "" {
		t.Fatalf("expected no nudge for coding flow, got=%q", nudge)
	}
}

func TestCollectSuccessfulWriteTargets_IgnoresAppendWrites(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "write"},
		{ID: "call-2", Name: "write_commit"},
	}
	toolResults := []llm.Message{
		{Role: llm.RoleTool, ToolCallID: "call-1", Content: `{"path":"draft.md","size":20,"success":true,"append":true}`},
		{Role: llm.RoleTool, ToolCallID: "call-2", Content: `{"path":"final.md","size":120,"success":true,"transactional":true}`},
	}

	targets := collectSuccessfulWriteTargets(toolCalls, toolResults)
	if len(targets) != 1 || targets[0] != "final.md" {
		t.Fatalf("expected only committed write target, got=%v", targets)
	}
}

func TestBuildPostResearchFailureRecoveryNudge_ForFailedResearchReport(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "web_search"},
	}
	toolResults := []llm.Message{
		{Role: llm.RoleTool, ToolCallID: "call-1", Content: `{"error":"upstream timeout","status":"timeout"}`},
	}

	nudge := buildPostResearchFailureRecoveryNudge(
		"Research the latest AI browser launches with sources and write the report to market_report.md.",
		toolCalls,
		toolResults,
	)
	if nudge == "" {
		t.Fatal("expected non-empty recovery nudge for failed research tool")
	}
	if want := `market_report.md`; !containsSubstring(nudge, want) {
		t.Fatalf("expected nudge to mention target path %q, got=%q", want, nudge)
	}
	if want := `write the requested report`; !containsSubstring(nudge, want) {
		t.Fatalf("expected nudge to steer the model back to writing, got=%q", nudge)
	}
}

func TestBuildPostResearchFailureRecoveryNudge_RecognizesExecWrappedSearchFailure(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "exec", Arguments: `{"command":"blue web_search query=\"latest AI browser launches\""}`},
	}
	toolResults := []llm.Message{
		{Role: llm.RoleTool, ToolCallID: "call-1", Content: `{"error":"provider unavailable","status":"failed","exit_code":1}`},
	}

	nudge := buildPostResearchFailureRecoveryNudge(
		"Please do a comprehensive research report and save it to browser_landscape.md.",
		toolCalls,
		toolResults,
	)
	if nudge == "" {
		t.Fatal("expected recovery nudge for exec-wrapped web search failure")
	}
	if want := `browser_landscape.md`; !containsSubstring(nudge, want) {
		t.Fatalf("expected nudge to mention target path %q, got=%q", want, nudge)
	}
}

func containsSubstring(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && (func() bool {
		return stringIndex(haystack, needle) >= 0
	})()
}

func stringIndex(s, sep string) int {
	for i := 0; i+len(sep) <= len(s); i++ {
		if s[i:i+len(sep)] == sep {
			return i
		}
	}
	return -1
}
