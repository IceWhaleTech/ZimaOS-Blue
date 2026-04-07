package server

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
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

func TestBuildPostWorkspaceArtifactContinuationNudge_AfterDiscoveryRequiresWrite(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "ls"},
	}
	toolResults := []llm.Message{
		{Role: llm.RoleTool, ToolCallID: "call-1", Content: `{"path":"research","entries":[{"name":"market.md"},{"name":"customer.txt"}],"status":"success"}`},
	}

	nudge := buildPostWorkspaceArtifactContinuationNudge(
		"Review all files in the research/ folder and write a daily summary to daily_briefing.md.",
		toolCalls,
		toolResults,
	)
	if nudge == "" {
		t.Fatal("expected continuation nudge after successful workspace discovery")
	}
	if want := `daily_briefing.md`; !containsSubstring(nudge, want) {
		t.Fatalf("expected nudge to mention target path %q, got=%q", want, nudge)
	}
	if want := `Do not stop after listing files`; !containsSubstring(nudge, want) {
		t.Fatalf("expected nudge to block discovery-only stopping, got=%q", nudge)
	}
}

func TestBuildPostWorkspaceArtifactCoverageContinuationNudge_WhenSomeFilesRemainUnread(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "ls"},
		{ID: "call-2", Name: "file_read"},
	}
	toolResults := []llm.Message{
		{Role: llm.RoleTool, ToolCallID: "call-1", Content: `{"base_path":"emails","entries":[{"path":"alpha_01.txt","type":"file"},{"path":"alpha_02.txt","type":"file"},{"path":"alpha_03.txt","type":"file"}],"status":"success"}`},
		{Role: llm.RoleTool, ToolCallID: "call-2", Content: `{"path":"emails/alpha_01.txt","content":"Project Alpha kickoff"}`},
	}

	nudge := buildPostWorkspaceArtifactCoverageContinuationNudge(
		"Review all files in the emails/ folder and write a summary to alpha_summary.md.",
		toolCalls,
		toolResults,
	)
	if nudge == "" {
		t.Fatal("expected coverage continuation nudge while files remain unread")
	}
	if want := `complete source coverage`; !containsSubstring(nudge, want) {
		t.Fatalf("expected coverage nudge to mention complete source coverage, got=%q", nudge)
	}
	if want := `alpha_02.txt`; !containsSubstring(nudge, want) {
		t.Fatalf("expected coverage nudge to list unread files, got=%q", nudge)
	}
}

func TestBuildPostWorkspaceArtifactContinuationNudge_NumberedQuestionsPreserveExactPhrases(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "pdf"},
	}
	toolResults := []llm.Message{
		{Role: llm.RoleTool, ToolCallID: "call-1", Content: `{"path":"openclaw_report.pdf","selected_pages":[1,2,3],"text":"typed WebSocket API"}`},
	}

	nudge := buildPostWorkspaceArtifactContinuationNudge(
		"I have a report in my workspace as openclaw_report.pdf. Write the answers one per line to answer.txt.\n1. What type of API does the gateway expose?\n2. What date was the registry collected?",
		toolCalls,
		toolResults,
	)
	if nudge == "" {
		t.Fatal("expected continuation nudge for numbered local QA task")
	}
	if want := `answer.txt`; !containsSubstring(nudge, want) {
		t.Fatalf("expected continuation nudge to mention the target artifact, got=%q", nudge)
	}
	if want := `Continue from the evidence you already gathered`; !containsSubstring(nudge, want) {
		t.Fatalf("expected continuation nudge to stay generic and evidence-driven, got=%q", nudge)
	}
}

func TestBuildPostWorkspaceArtifactContinuationNudge_SkipsAfterSuccessfulWrite(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "file_write"},
	}
	toolResults := []llm.Message{
		{Role: llm.RoleTool, ToolCallID: "call-1", Content: `{"path":"daily_briefing.md","size":2168,"success":true,"append":false}`},
	}

	nudge := buildPostWorkspaceArtifactContinuationNudge(
		"Review all files in the research/ folder and write a daily summary to daily_briefing.md.",
		toolCalls,
		toolResults,
	)
	if nudge != "" {
		t.Fatalf("expected no continuation nudge after successful write, got=%q", nudge)
	}
}

func TestBuildPostWorkspaceArtifactWriteRetryNudge_NumberedQuestionsPreserveExactPhrases(t *testing.T) {
	nudge := buildPostWorkspaceArtifactWriteRetryNudge(
		"I have a report in my workspace as openclaw_report.pdf. Write the answers one per line to answer.txt.\n1. What type of API does the gateway expose?\n2. What date was the registry collected?",
	)
	if nudge == "" {
		t.Fatal("expected retry nudge for numbered local QA task")
	}
	if want := `answer.txt`; !containsSubstring(nudge, want) {
		t.Fatalf("expected retry nudge to mention the target artifact, got=%q", nudge)
	}
	if want := `evidence already gathered in the conversation`; !containsSubstring(nudge, want) {
		t.Fatalf("expected retry nudge to stay generic and evidence-driven, got=%q", nudge)
	}
}

func TestWorkspaceArtifactWriteRecoveryThreshold_NumberedQuestionsRecoverEarlier(t *testing.T) {
	if got := workspaceArtifactWriteRecoveryThreshold("Write a summary to output.txt."); got != 3 {
		t.Fatalf("threshold for ordinary artifact = %d, want 3", got)
	}
	if got := workspaceArtifactWriteRecoveryThreshold("1. What is the date?\n2. What is the API type?\nWrite the answers to answer.txt."); got != 3 {
		t.Fatalf("threshold for numbered question artifact = %d, want 3", got)
	}
	if got := workspaceArtifactWriteRecoveryThreshold("Review all files in the emails/ folder and write a summary to alpha_summary.md."); got != 3 {
		t.Fatalf("threshold for exhaustive collection artifact = %d, want 3", got)
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

func TestBuildPostWriteCompletionNudge_ForDirectArtifactWriting(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "file_write"},
	}
	toolResults := []llm.Message{
		{Role: llm.RoleTool, ToolCallID: "call-1", Content: `{"path":"email_draft.txt","size":320,"success":true,"append":false}`},
	}

	nudge := buildPostWriteCompletionNudge(
		"Write a professional email declining a meeting request due to schedule conflicts. Save it to email_draft.txt.",
		toolCalls,
		toolResults,
	)
	if nudge == "" {
		t.Fatal("expected write-completion nudge for direct artifact writing")
	}
	if want := `email_draft.txt`; !containsSubstring(nudge, want) {
		t.Fatalf("expected nudge to mention direct-writing target %q, got=%q", want, nudge)
	}
}

func TestBuildPostWriteCompletionTools_RemovesWriteAfterSuccessfulArtifactWrite(t *testing.T) {
	toolsIn := []llm.Tool{
		{Name: "file_write"},
		{Name: "file_read"},
		{Name: "find"},
		{Name: "ls"},
		{Name: "web_query"},
	}

	reduced := buildPostWriteCompletionTools(
		toolsIn,
		"Write a professional email declining a meeting request due to schedule conflicts. Save it to email_draft.txt.",
	)
	if containsLLMToolName(reduced, "file_write") {
		t.Fatalf("expected write tool to be removed after successful artifact write, got=%v", reduced)
	}
	if !containsLLMToolName(reduced, "file_read") {
		t.Fatalf("expected file_read to remain for final verification, got=%v", reduced)
	}
	if containsLLMToolName(reduced, "web_query") {
		t.Fatalf("expected web_query tool to be removed after successful artifact write, got=%v", reduced)
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

func TestBuildPostEmptyResearchResultNudge_IgnoresPendingResearchJob(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "research_run"},
	}
	toolResults := []llm.Message{
		{Role: llm.RoleTool, ToolCallID: "call-1", Content: `{"job_id":"job-2","status":"pending","accepted":true,"terminal":false,"evidence_count":0}`},
	}

	nudge := buildPostEmptyResearchResultNudge(
		"Create a competitive market report and save it to market_research.md with sources.",
		toolCalls,
		toolResults,
	)
	if nudge != "" {
		t.Fatalf("expected pending research job to avoid empty-result recovery, got=%q", nudge)
	}
}

func TestBuildPostPendingResearchStatusNudge_ForAcceptedResearchJob(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "research_run"},
	}
	toolResults := []llm.Message{
		{Role: llm.RoleTool, ToolCallID: "call-1", Content: `{"job_id":"job-2","status":"pending","accepted":true,"terminal":false,"evidence_count":0}`},
	}

	nudge := buildPostPendingResearchStatusNudge(
		"Create a competitive market report and save it to market_research.md with sources.",
		toolCalls,
		toolResults,
	)
	if nudge == "" {
		t.Fatal("expected pending research status nudge")
	}
	if want := `job_id "job-2"`; !containsSubstring(nudge, want) {
		t.Fatalf("expected nudge to mention job id, got=%q", nudge)
	}
	if want := `market_research.md`; !containsSubstring(nudge, want) {
		t.Fatalf("expected nudge to mention target path %q, got=%q", want, nudge)
	}
	if want := `deep_research with action="status"`; !containsSubstring(nudge, want) {
		t.Fatalf("expected nudge to steer toward deep_research status action, got=%q", nudge)
	}
}

func TestBuildEmptyResearchResultRecoveryTools_PrefersSearchAndWriteWithoutBrowser(t *testing.T) {
	tools := []llm.Tool{
		{Name: "browser"},
		{Name: "web_search"},
		{Name: "web_query"},
		{Name: "web_fetch"},
		{Name: "write"},
		{Name: "file_delete"},
		{Name: "read"},
	}

	reduced := buildEmptyResearchResultRecoveryTools(tools, "Write the report to market_research.md after research.")
	if len(reduced) == 0 {
		t.Fatal("expected reduced toolset")
	}
	if got := reduced[0].Name; got != "web_query" {
		t.Fatalf("expected web_query to lead reduced toolset, got=%q", got)
	}
	for _, tool := range reduced {
		if tool.Name == "browser" {
			t.Fatalf("expected browser to be removed from empty-research recovery toolset, got=%v", reduced)
		}
		if tool.Name == "file_delete" {
			t.Fatalf("expected file_delete to be removed from empty-research recovery toolset, got=%v", reduced)
		}
	}
}

func TestBuildPendingResearchStatusTools_PrefersResearchStatusAndWrite(t *testing.T) {
	tools := []llm.Tool{
		{Name: "deep_research"},
		{Name: "browser"},
		{Name: "web_search"},
		{Name: "web_fetch"},
		{Name: "write"},
		{Name: "file_delete"},
		{Name: "read"},
	}

	reduced := buildPendingResearchStatusTools(tools, "Write the report to market_research.md after research.")
	if len(reduced) == 0 {
		t.Fatal("expected reduced toolset")
	}
	if got := reduced[0].Name; got != "deep_research" {
		t.Fatalf("expected deep_research to lead reduced toolset, got=%q", got)
	}
	for _, tool := range reduced {
		if tool.Name == "browser" || tool.Name == "web_search" || tool.Name == "file_delete" {
			t.Fatalf("expected pending-research toolset to drop duplicate broad-search/delete tools, got=%v", reduced)
		}
	}
}

func TestBuildPostSuccessfulResearchWriteNudge_PushesTowardReport(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "research_run"},
	}
	toolResults := []llm.Message{
		{Role: llm.RoleTool, ToolCallID: "call-1", Content: `{"status":"completed","evidence_count":8,"answer":"Datadog, New Relic, Dynatrace, Elastic, and Splunk are the top players."}`},
	}

	nudge := buildPostSuccessfulResearchWriteNudge(
		"Create a competitive market report and save it to market_research.md with sources.",
		toolCalls,
		toolResults,
	)
	if nudge == "" {
		t.Fatal("expected successful-research write nudge")
	}
	if want := `market_research.md`; !containsSubstring(nudge, want) {
		t.Fatalf("expected nudge to mention target path %q, got=%q", want, nudge)
	}
	if want := `Do not start another broad search pass`; !containsSubstring(nudge, want) {
		t.Fatalf("expected nudge to block another broad search pass, got=%q", nudge)
	}
}

func TestBuildSuccessfulResearchWriteTools_RemovesResearchRun(t *testing.T) {
	tools := []llm.Tool{
		{Name: "research_run"},
		{Name: "research_status"},
		{Name: "web_fetch"},
		{Name: "web_search"},
		{Name: "write"},
		{Name: "file_delete"},
		{Name: "read"},
	}

	reduced := buildSuccessfulResearchWriteTools(tools, "Write the report to market_research.md after research.")
	if len(reduced) == 0 {
		t.Fatal("expected reduced toolset")
	}
	if got := reduced[0].Name; got != "write" {
		t.Fatalf("expected write to lead reduced toolset, got=%q", got)
	}
	for _, tool := range reduced {
		if tool.Name == "research_run" || tool.Name == "research_status" || tool.Name == "web_search" || tool.Name == "file_delete" {
			t.Fatalf("expected full research and broad search tools to be removed after successful research, got=%v", reduced)
		}
	}
}

func TestBuildResearchFailureRecoveryTools_ReducesToWriteWorkflow(t *testing.T) {
	tools := []llm.Tool{
		{Name: "research_run"},
		{Name: "browser"},
		{Name: "web_search"},
		{Name: "write"},
		{Name: "file_delete"},
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
		if tool.Name == "web_search" || tool.Name == "browser" || tool.Name == "research_run" || tool.Name == "file_delete" {
			t.Fatalf("expected search tools to be removed from recovery toolset, got=%v", reduced)
		}
	}
}

func TestBuildPostWorkspaceArtifactContinuationTools_DropsFileDeleteDuringWriteFocusedContinuation(t *testing.T) {
	reduced := buildPostWorkspaceArtifactContinuationTools([]llm.Tool{
		{Name: "file_write"},
		{Name: "file_read"},
		{Name: "file_delete"},
		{Name: "edit"},
		{Name: "ls"},
		{Name: "find"},
		{Name: "grep"},
		{Name: "pdf"},
	}, "Review openclaw_report.pdf and write the answers to answer.txt.", []llm.ToolCall{
		{ID: "call-1", Name: "pdf"},
	}, []llm.Message{
		{Role: llm.RoleTool, ToolCallID: "call-1", Content: `{"path":"openclaw_report.pdf","selected_pages":[1],"text":"typed WebSocket API"}`},
	})

	if !containsLLMToolName(reduced, "file_write") {
		t.Fatalf("expected continuation tools to preserve file_write, got=%v", reduced)
	}
	if containsLLMToolName(reduced, "file_delete") {
		t.Fatalf("expected continuation tools to drop file_delete once evidence is sufficient, got=%v", reduced)
	}
	if !containsLLMToolName(reduced, "file_write") {
		t.Fatalf("expected continuation tools to keep the write path available, got=%v", reduced)
	}
	if containsLLMToolName(reduced, "pdf") || containsLLMToolName(reduced, "file_read") {
		t.Fatalf("expected continuation tools to avoid reopening evidence tools once write completion mode begins, got=%v", reduced)
	}
}

func TestBuildWorkspaceArtifactWriteRecoveryTools_DropsFileDelete(t *testing.T) {
	reduced := buildWorkspaceArtifactWriteRecoveryTools([]llm.Tool{
		{Name: "file_write"},
		{Name: "file_delete"},
		{Name: "edit"},
		{Name: "write_begin"},
		{Name: "write_chunk"},
		{Name: "write_commit"},
	}, "Write the answers to answer.txt.")

	if !containsLLMToolName(reduced, "file_write") {
		t.Fatalf("expected recovery tools to preserve file_write, got=%v", reduced)
	}
	if containsLLMToolName(reduced, "file_delete") {
		t.Fatalf("expected recovery tools to drop file_delete, got=%v", reduced)
	}
}

func TestBuildWorkspaceArtifactWriteRecoveryTools_PrefersOfficeForOfficeArtifacts(t *testing.T) {
	reduced := buildWorkspaceArtifactWriteRecoveryTools([]llm.Tool{
		{Name: "office"},
		{Name: "file_write"},
		{Name: "file_delete"},
		{Name: "edit"},
	}, "Write the styled report to ui_review.docx.")

	if !containsLLMToolName(reduced, "office") {
		t.Fatalf("expected recovery tools to preserve office, got=%v", reduced)
	}
	if got := reduced[0].Name; got != "office" {
		t.Fatalf("expected office to lead office artifact recovery, got=%q", got)
	}
	if containsLLMToolName(reduced, "file_delete") {
		t.Fatalf("expected recovery tools to drop file_delete, got=%v", reduced)
	}
}

func TestBuildArtifactWorkflowExecutionHint_PrefersOfficeForOfficeArtifacts(t *testing.T) {
	hint := buildArtifactWorkflowExecutionHint("Read findings.md and save the polished report to ui_review.docx.")
	if !containsSubstring(hint, "prefer the native office tool") {
		t.Fatalf("expected office hint, got=%q", hint)
	}
}

func TestExtractSuccessfulWriteTarget_Office(t *testing.T) {
	got := extractSuccessfulWriteTarget("office", `{"success":true,"path":"reports/ui_review.docx"}`)
	if got != "reports/ui_review.docx" {
		t.Fatalf("extractSuccessfulWriteTarget() = %q, want reports/ui_review.docx", got)
	}
}

func TestHasSavedWorkspaceArtifactOnDisk_UsesScopedWorkspacePath(t *testing.T) {
	workspaceDir := t.TempDir()
	answerPath := filepath.Join(workspaceDir, "answer.txt")
	if err := os.WriteFile(answerPath, []byte("ok\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	ctx := tools.WithFSRootOverride(context.Background(), []string{workspaceDir}, map[string]string{"workspace": workspaceDir})
	if !hasSavedWorkspaceArtifactOnDisk(ctx, "Write the answers to answer.txt.") {
		t.Fatal("expected scoped workspace artifact to be detected on disk")
	}
	if hasSavedWorkspaceArtifactOnDisk(ctx, "Write the answers to missing.txt.") {
		t.Fatal("expected missing scoped artifact to stay false")
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

func TestBuildResearchFailureRetryMessages_CompactsToFreshWritePrompt(t *testing.T) {
	messages := buildResearchFailureRetryMessages(
		"Create a competitive market report with sources and save it to market_research.md.",
	)
	if len(messages) != 2 {
		t.Fatalf("expected compact retry context with 2 messages, got=%d", len(messages))
	}
	if messages[0].Role != llm.RoleUser || !containsSubstring(messages[0].Content, "market_research.md") {
		t.Fatalf("expected original user task to be preserved, got=%+v", messages[0])
	}
	if messages[1].Role != llm.RoleUser || !containsSubstring(messages[1].Content, "write the requested report") {
		t.Fatalf("expected write-focused retry nudge, got=%+v", messages[1])
	}
}

func TestSelectResearchFailureWriteRecoveryModel_PrefersFastModel(t *testing.T) {
	got := selectResearchFailureWriteRecoveryModel(
		"claude-haiku-4-5-20251001",
		[]string{"claude-haiku-4-5-20251001", "gpt-4o-mini", "qwen-turbo"},
	)
	if got != "qwen-turbo" {
		t.Fatalf("expected qwen-turbo fast fallback, got=%q", got)
	}
}

func TestSelectResearchFailureWriteRecoveryModel_FallsBackToSiblingFamily(t *testing.T) {
	got := selectResearchFailureWriteRecoveryModel(
		"claude-haiku-4-5-20251001",
		[]string{"claude-haiku-4-5-20251001", "claude-3-5-haiku-20241022"},
	)
	if got != "claude-3-5-haiku-20241022" {
		t.Fatalf("expected sibling-family fallback model, got=%q", got)
	}
}

func TestSelectResearchFailureWriteRecoveryModel_DottedAliasUsesProviderScopedConcreteModel(t *testing.T) {
	got := selectResearchFailureWriteRecoveryModel(
		"claude-3.5-haiku",
		[]string{"claude-haiku-4-5-20251001", "claude-sonnet-4-6"},
	)
	if got != "claude-haiku-4-5-20251001" {
		t.Fatalf("expected provider-scoped concrete haiku fallback, got=%q", got)
	}
}

func TestRecoveryAvailableModelIDs_PrefersPinnedProviderModels(t *testing.T) {
	handler := &ChatHandler{
		providerPool: newProviderPoolWithContextWindowModels(t, []contextWindowModelSpec{
			{ProviderID: "api123-haiku45", ModelID: "claude-haiku-4-5-20251001", ContextWindow: 200000},
			{ProviderID: "anthropic", ModelID: "claude-3-5-haiku-20241022", ContextWindow: 200000},
		}),
	}

	ctx := proxy.WithPinnedProvider(context.Background(), "api123-haiku45")
	got := handler.recoveryAvailableModelIDs(ctx)
	want := []string{"claude-haiku-4-5-20251001"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("recoveryAvailableModelIDs() = %v, want %v", got, want)
	}
}

func TestRecoveryAvailableModelIDs_FallsBackToResolvedRouteProviderModels(t *testing.T) {
	handler := &ChatHandler{
		providerPool: newProviderPoolWithContextWindowModels(t, []contextWindowModelSpec{
			{ProviderID: "api123-haiku45", ModelID: "claude-haiku-4-5-20251001", ContextWindow: 200000},
			{ProviderID: "anthropic", ModelID: "claude-3-5-haiku-20241022", ContextWindow: 200000},
		}),
	}

	ctx := proxy.WithResolvedRoute(context.Background(), &proxy.ResolvedRoute{ProviderID: "api123-haiku45"})
	got := handler.recoveryAvailableModelIDs(ctx)
	want := []string{"claude-haiku-4-5-20251001"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("recoveryAvailableModelIDs() = %v, want %v", got, want)
	}
}

func TestBuildToolLoopArtifactRecoveryNudge_PushesSearchLoopTowardWrite(t *testing.T) {
	nudge := buildToolLoopArtifactRecoveryNudge(
		"Create a competitive market report and save it to market_research.md with sources.",
		tools.ToolLoopReasonPollingNoProgress,
		`web_fetch:{"url":"https://example.com/a"}|status:completed|url:https://example.com/a`,
	)
	if nudge == "" {
		t.Fatal("expected artifact recovery nudge")
	}
	if want := `market_research.md`; !containsSubstring(nudge, want) {
		t.Fatalf("expected artifact nudge to mention target path %q, got=%q", want, nudge)
	}
	if want := `Do not continue looping through more search, browsing, or repeated file discovery/reads`; !containsSubstring(nudge, want) {
		t.Fatalf("expected artifact nudge to stop loop, got=%q", nudge)
	}
}

func TestBuildToolLoopArtifactRecoveryTools_ReducesToWriteWorkflow(t *testing.T) {
	toolset := []llm.Tool{
		{Name: "web_fetch"},
		{Name: "web_search"},
		{Name: "browser"},
		{Name: "write"},
		{Name: "read"},
		{Name: "grep"},
	}

	reduced := buildToolLoopArtifactRecoveryTools(
		toolset,
		"Create a competitive market report and save it to market_research.md with sources.",
		`web_search:{"query":"apm vendors"} -> web_fetch:{"url":"https://example.com"}`,
	)
	if len(reduced) == 0 {
		t.Fatal("expected reduced toolset")
	}
	if got := reduced[0].Name; got != "write" {
		t.Fatalf("expected write-first recovery toolset, got=%q", got)
	}
	for _, tool := range reduced {
		if tool.Name == "web_fetch" || tool.Name == "web_search" || tool.Name == "browser" {
			t.Fatalf("expected search/browser tools to be removed, got=%v", reduced)
		}
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
