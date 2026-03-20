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

func TestBuildPostEmptyResearchResultNudge_ForZeroEvidenceResearch(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "research_run"},
	}
	toolResults := []llm.Message{
		{Role: llm.RoleTool, ToolCallID: "call-1", Content: `{"status":"completed","evidence_count":0,"answer":"No sufficient evidence was collected."}`},
	}

	nudge := buildPostEmptyResearchResultNudge(
		"Create a competitive market report and save it to market_research.md with sources.",
		toolCalls,
		toolResults,
	)
	if nudge == "" {
		t.Fatal("expected empty-research recovery nudge")
	}
	if want := `market_research.md`; !containsSubstring(nudge, want) {
		t.Fatalf("expected nudge to mention target path %q, got=%q", want, nudge)
	}
	if want := `do not switch to interactive browser navigation`; !containsSubstring(nudge, want) {
		t.Fatalf("expected nudge to steer away from browser, got=%q", nudge)
	}
}

func TestBuildEmptyResearchResultRecoveryTools_PrefersSearchAndWriteWithoutBrowser(t *testing.T) {
	tools := []llm.Tool{
		{Name: "browser"},
		{Name: "web_search"},
		{Name: "web_fetch"},
		{Name: "write"},
		{Name: "read"},
	}

	reduced := buildEmptyResearchResultRecoveryTools(tools, "Write the report to market_research.md after research.")
	if len(reduced) == 0 {
		t.Fatal("expected reduced toolset")
	}
	if got := reduced[0].Name; got != "web_search" {
		t.Fatalf("expected web_search to lead reduced toolset, got=%q", got)
	}
	for _, tool := range reduced {
		if tool.Name == "browser" {
			t.Fatalf("expected browser to be removed from empty-research recovery toolset, got=%v", reduced)
		}
	}
}

func TestBuildResearchFailureRecoveryTools_ReducesToWriteWorkflow(t *testing.T) {
	tools := []llm.Tool{
		{Name: "research_run"},
		{Name: "browser"},
		{Name: "web_search"},
		{Name: "write"},
		{Name: "read"},
		{Name: "find"},
	}

	reduced := buildResearchFailureRecoveryTools(tools, "Write the report to market_research.md after the research step.")
	if len(reduced) == 0 {
		t.Fatal("expected reduced toolset")
	}
	if got := reduced[0].Name; got != "write" {
		t.Fatalf("expected write to be first reduced tool, got=%q", got)
	}
	for _, tool := range reduced {
		if tool.Name == "web_search" || tool.Name == "browser" || tool.Name == "research_run" {
			t.Fatalf("expected search tools to be removed from recovery toolset, got=%v", reduced)
		}
	}
}

func TestBuildPostResearchFailureRecoveryRetryNudge_ForFailedWriteFollowUp(t *testing.T) {
	nudge := buildPostResearchFailureRecoveryRetryNudge(
		"Create a competitive market report with sources and save it to market_research.md.",
	)
	if nudge == "" {
		t.Fatal("expected retry nudge")
	}
	if want := `market_research.md`; !containsSubstring(nudge, want) {
		t.Fatalf("expected retry nudge to mention target path %q, got=%q", want, nudge)
	}
	if want := `ended before the report was saved`; !containsSubstring(nudge, want) {
		t.Fatalf("expected retry nudge to mention incomplete save, got=%q", nudge)
	}
}

func TestShouldRetryPendingResearchWrite_WhenReportStillMissing(t *testing.T) {
	if !shouldRetryPendingResearchWrite(
		"Create a competitive market report with sources and save it to market_research.md.",
		"Here are the key takeaways from the completed tools.",
		true,
		0,
	) {
		t.Fatal("expected pending research write to force another continuation")
	}
}

func TestShouldRetryPendingResearchWrite_SkipsWhenAwaitingUserInput(t *testing.T) {
	if shouldRetryPendingResearchWrite(
		"Create a competitive market report with sources and save it to market_research.md.",
		"Which file path should I use for the report?",
		true,
		0,
	) {
		t.Fatal("expected no forced continuation when assistant is explicitly awaiting user input")
	}
}

func TestShouldRetryPendingResearchWrite_SkipsAfterRetryBudget(t *testing.T) {
	if shouldRetryPendingResearchWrite(
		"Create a competitive market report with sources and save it to market_research.md.",
		"Here are the key takeaways from the completed tools.",
		true,
		maxResearchFailureWriteRecoveryRetries,
	) {
		t.Fatal("expected retry budget to stop forced continuation")
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
