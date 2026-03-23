package server

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type fileWriteCaptureTool struct {
	mu      sync.Mutex
	path    string
	content string
	calls   int
}

func (t *fileWriteCaptureTool) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{
		Name:        "file_write",
		Description: "capture file writes",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path":    map[string]interface{}{"type": "string"},
				"content": map[string]interface{}{"type": "string"},
			},
			"required":             []string{"path", "content"},
			"additionalProperties": true,
		},
	}
}

func (t *fileWriteCaptureTool) Execute(_ context.Context, args map[string]interface{}) (interface{}, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.calls++
	t.path = strings.TrimSpace(anyToStringForLLM(args["path"]))
	t.content = anyToStringForLLM(args["content"])
	return map[string]interface{}{
		"path":    t.path,
		"success": true,
		"append":  false,
	}, nil
}

func (t *fileWriteCaptureTool) Captured() (path, content string, calls int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.path, t.content, t.calls
}

func TestTryLLMWorkspaceArtifactOrchestration_WritesRecoveredArtifactFromLocalEvidence(t *testing.T) {
	registry := llm.NewProviderRegistry()
	provider := &scriptedChatProvider{
		name:   "stub",
		models: []string{"claude-opus-4-6"},
		responses: []llm.ChatResponse{{
			Model:      "claude-opus-4-6",
			Provider:   "stub",
			ProviderID: "stub",
			Message: llm.Message{
				Role:    llm.RoleAssistant,
				Content: "Here are the answers:\n```text\n5705\n2999\nAI & LLMs: 287\nSearch & Research: 253\nSKILL.md\ntyped WebSocket API\nFebruary 7, 2026\n6\n```",
			},
		}},
	}
	registry.Register(provider)

	writeTool := &fileWriteCaptureTool{}
	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(writeTool)

	handler := NewChatHandler(nil, registry, toolRegistry)
	toolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "file_read"},
	}
	toolResults := []llm.Message{
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-1",
			Content:    `{"path":"openclaw_report.pdf","selected_pages":[1,2,3,4,5,6,7,8],"text":"The public registry had 5,705 community-built skills and 2,999 remained after filtering. AI & LLMs had 287 skills, while Search & Research had 253. Each OpenClaw skill is defined by SKILL.md. The gateway exposes a typed WebSocket API. The registry data was collected on February 7, 2026. The paper proposes 6 benchmark tasks."}`,
		},
	}

	result, ok := handler.tryLLMWorkspaceArtifactOrchestration(
		context.Background(),
		"claude-opus-4-6",
		"I have a research report about OpenClaw agent use cases in my workspace as `openclaw_report.pdf`. I need you to extract several pieces of information from it and write them to `answer.txt`. Please answer the following questions, one answer per line:\n\n1. How many community-built skills were in the public registry before filtering?\n2. How many skills remained after filtering out spam, duplicates, non-English, crypto/finance/trading, and malicious content?\n3. What is the largest skill category by count, and how many skills does it have? (format: \"Category Name: count\")\n4. What is the second-largest skill category by count, and how many skills does it have? (format: \"Category Name: count\")\n5. What is the name of the file that defines an OpenClaw skill?\n6. What type of API does the OpenClaw gateway expose?\n7. What date was the skills registry data collected?\n8. How many new benchmark tasks does the paper propose? (just the number)",
		toolCalls,
		toolResults,
	)
	if !ok {
		t.Fatal("expected workspace artifact orchestration to succeed")
	}
	if result == nil || !strings.Contains(result.Content, `answer.txt`) {
		t.Fatalf("expected confirmation mentioning answer.txt, got=%+v", result)
	}

	path, content, calls := writeTool.Captured()
	if calls != 1 {
		t.Fatalf("expected one deterministic write, got %d", calls)
	}
	if path != "answer.txt" {
		t.Fatalf("captured path = %q, want answer.txt", path)
	}
	if strings.Contains(content, "```") {
		t.Fatalf("expected orchestration to strip outer code fences, got=%q", content)
	}
	for _, needle := range []string{"5705", "2999", "AI & LLMs: 287", "Search & Research: 253", "SKILL.md", "typed WebSocket API", "February 7, 2026", "6"} {
		if !strings.Contains(content, needle) {
			t.Fatalf("expected saved content to contain %q, got=%q", needle, content)
		}
	}

	req, ok := provider.RequestAt(0)
	if !ok {
		t.Fatal("expected synthesis request to be recorded")
	}
	if req.Model != "claude-opus-4-6" {
		t.Fatalf("orchestration model = %q, want claude-opus-4-6", req.Model)
	}
	if len(req.Tools) != 0 {
		t.Fatalf("expected synthesis orchestration to run without tools, got=%v", req.Tools)
	}
	if len(req.Messages) < 2 || !strings.Contains(req.Messages[1].Content, "Relevant local evidence") {
		t.Fatalf("expected synthesis prompt to include recovered evidence, got=%v", req.Messages)
	}
	if !strings.Contains(req.Messages[1].Content, "Return exactly 8 non-empty lines") {
		t.Fatalf("expected numbered-question prompt to enforce exact line count, got=%v", req.Messages[1].Content)
	}
	if !strings.Contains(req.Messages[1].Content, "count the distinct items across that full bounded set") {
		t.Fatalf("expected numbered-question prompt to include bounded-list counting guidance, got=%v", req.Messages[1].Content)
	}
}

func TestShouldUseLLMWorkspaceArtifactOrchestration_SkipsDiscoveryOnlyRounds(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "ls"},
	}
	toolResults := []llm.Message{
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-1",
			Content:    `{"path":"research","entries":[{"name":"openclaw_report.pdf","type":"file"}],"status":"success"}`,
		},
	}

	if shouldUseLLMWorkspaceArtifactOrchestration(
		"I have a report in openclaw_report.pdf in my workspace. Extract the answers and write them one per line to answer.txt.",
		toolCalls,
		toolResults,
	) {
		t.Fatal("expected workspace artifact orchestration to skip discovery-only evidence")
	}
}

func TestShouldUseLLMWorkspaceArtifactOrchestration_SkipsWhenExhaustiveCoverageStillPending(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "ls"},
		{ID: "call-2", Name: "file_read"},
	}
	toolResults := []llm.Message{
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-1",
			Content:    `{"base_path":"emails","entries":[{"path":"alpha_01.txt","type":"file"},{"path":"alpha_02.txt","type":"file"}],"status":"success"}`,
		},
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-2",
			Content:    `{"path":"emails/alpha_01.txt","content":"Project Alpha kickoff and budget approval."}`,
		},
	}

	if shouldUseLLMWorkspaceArtifactOrchestration(
		"Review all files in the emails/ folder and write a summary to alpha_summary.md.",
		toolCalls,
		toolResults,
	) {
		t.Fatal("expected workspace artifact orchestration to wait until all discovered files are covered")
	}
}

func TestShouldUseLLMWorkspaceArtifactOrchestration_SkipsReadOnlyMemoryRecallPrompt(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "file_read"},
	}
	toolResults := []llm.Message{
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-1",
			Content:    `{"path":"memory/MEMORY.md","content":"Rust\nJanuary 15, 2024\nDr. Elena Vasquez from Stanford\nNeonDB\npurple elephant sunrise"}`,
		},
	}

	if shouldUseLLMWorkspaceArtifactOrchestration(
		"I previously saved some personal information in a file called `memory/MEMORY.md`. Please read that file and answer these questions based on what you find in the file.",
		toolCalls,
		toolResults,
	) {
		t.Fatal("expected workspace artifact orchestration to skip read-only memory recall prompts")
	}
}

func TestShouldUseLLMWorkspaceArtifactOrchestration_SkipsWorkspaceEditTask(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "file_read"},
		{ID: "call-2", Name: "file_read"},
	}
	toolResults := []llm.Message{
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-1",
			Content:    `{"path":"docs/spec.md","content":"Add a quickstart section and a troubleshooting section."}`,
		},
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-2",
			Content:    `{"path":"README.md","content":"# Project\n\nOld introduction."}`,
		},
	}

	if shouldUseLLMWorkspaceArtifactOrchestration(
		"Read `docs/spec.md` and update `README.md` accordingly.",
		toolCalls,
		toolResults,
	) {
		t.Fatal("expected workspace artifact orchestration to skip direct edit tasks")
	}
}

func TestShouldUseLLMWorkspaceArtifactOrchestration_AllowsRepairWhenStructuredWriteMissesRequestedSections(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "file_read"},
		{ID: "call-2", Name: "file_write", Arguments: `{"path":"alpha_summary.md","content":"Plain paragraphs without the requested headings."}`},
	}
	toolResults := []llm.Message{
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-1",
			Content:    `{"path":"emails/alpha_01.txt","content":"Subject: Project Alpha - Kickoff and Timeline\nThis is our analytics dashboard.\nBudget $340K.\nBeta Launch: Apr 21"}`,
		},
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-2",
			Content:    `{"path":"alpha_summary.md","success":true}`,
		},
	}

	if !shouldUseLLMWorkspaceArtifactOrchestration(
		"You have access to a collection of emails in the emails/ folder in your workspace. Save the summary to alpha_summary.md with the following sections:\n1. **Project Overview**: What is Project Alpha?\n2. **Timeline**: Original timeline and any changes\n3. **Key Risks and Issues**: Budget concerns and security findings\n4. **Client/Business Impact**: Pipeline and revenue projections\n5. **Current Status**: Latest status",
		toolCalls,
		toolResults,
	) {
		t.Fatal("expected orchestration repair to remain allowed when the saved artifact misses the requested section structure")
	}
}

func TestShouldUseLLMWorkspaceArtifactOrchestration_AllowsFolderSummaryWithExplicitSections(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "file_read"},
	}
	toolResults := []llm.Message{
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-1",
			Content:    `{"path":"emails/2026-01-15_project_alpha_kickoff.txt","content":"Subject: Project Alpha - Kickoff and Timeline\nThis is our analytics dashboard.\nBudget $340K.\nBeta Launch: Apr 21"}`,
		},
	}

	if !shouldUseLLMWorkspaceArtifactOrchestration(
		"You have access to a collection of emails in the emails/ folder in your workspace. Save the summary to alpha_summary.md with the following sections:\n1. **Project Overview**: What is Project Alpha?\n2. **Timeline**: Original timeline and any changes\n3. **Key Risks and Issues**: Budget concerns and security findings\n4. **Client/Business Impact**: Pipeline and revenue projections\n5. **Current Status**: Latest status",
		toolCalls,
		toolResults,
	) {
		t.Fatal("expected orchestration to support folder-based local summaries with explicit section structure")
	}
}

func TestExtractRequestedSectionTitles_PreservesExplicitStructuredHeadings(t *testing.T) {
	titles := extractRequestedSectionTitles("Save the summary to alpha_summary.md with the following sections:\n\n1. **Project Overview**: What is Project Alpha?\n2. **Timeline**: Original timeline and current expected dates\n3. **Key Risks and Issues**: Budget concerns and security findings\n4. **Client/Business Impact**: Pipeline and revenue projections\n5. **Current Status**: Where the project stands right now")
	want := []string{
		"Project Overview",
		"Timeline",
		"Key Risks and Issues",
		"Client/Business Impact",
		"Current Status",
	}
	if fmt.Sprint(titles) != fmt.Sprint(want) {
		t.Fatalf("extractRequestedSectionTitles() = %v, want %v", titles, want)
	}
}

func TestBuildWorkspaceArtifactOrchestrationMessages_PreservesSectionTitlesAndConcreteFacts(t *testing.T) {
	msgs := buildWorkspaceArtifactOrchestrationMessages(
		"You have access to a collection of emails in the emails/ folder in your workspace. Save the summary to alpha_summary.md with the following sections:\n\n1. **Project Overview**: What is Project Alpha, what technology is being used, and what is the budget?\n2. **Timeline**: Original timeline and any changes, including current expected dates\n3. **Key Risks and Issues**: Budget concerns, security findings, technical challenges\n4. **Client/Business Impact**: Sales pipeline, client feedback, and revenue projections\n5. **Current Status**: Where the project stands right now based on the most recent updates",
		"alpha_summary.md",
		[]string{"FILE_READ | emails/alpha.txt\nBudget moved from $340K to $410K. Beta moved from Apr 21 to May 6 because security fixes and a WebSocket gateway added time."},
		nil,
	)
	if len(msgs) < 2 {
		t.Fatalf("expected orchestration messages, got=%v", msgs)
	}
	body := msgs[1].Content
	for _, needle := range []string{
		"Requested section titles (preserve exactly in this order)",
		"1. Project Overview",
		"5. Current Status",
		"Reproduce any explicitly requested section titles exactly and in order",
		"Preserve concrete named entities, exact numbers, exact dates, exact money figures, exact technology names",
		"show both versions explicitly and make the cause of the change clear",
		"original-versus-updated budget and timeline values side by side",
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("expected prompt to contain %q, got=%q", needle, body)
		}
	}
	if !strings.Contains(msgs[0].Content, "requested structure and the source evidence's concrete names, numbers, dates, and labels") {
		t.Fatalf("expected system prompt to reinforce structure and evidence fidelity, got=%q", msgs[0].Content)
	}
}

func TestBuildDeterministicWorkspaceArtifactOrchestrationDraft_ForProjectStatusSummaryTask(t *testing.T) {
	evidence := []string{
		"FILE_READ | emails/2026-01-15_project_alpha_kickoff.txt\nFrom: sarah.chen@mycompany.com\nSubject: Project Alpha - Kickoff and Timeline\nThis is our new customer-facing analytics dashboard that will replace the legacy reporting system.\n- We're going with PostgreSQL + TimescaleDB\n- API will be built with FastAPI\n- Frontend will use React with Recharts\nBudget has been approved for $340K total.\n- Beta Launch: Apr 21\n- GA Release: May 12",
		"FILE_READ | emails/2026-01-22_alpha_data_pipeline.txt\nSubject: Re: Project Alpha - Data Pipeline Architecture Proposal\n- Apache Kafka for real-time event streaming\n- Apache Flink for stream processing\n- dbt for batch transformations\n- Redis for caching",
		"FILE_READ | emails/2026-02-03_alpha_budget_concern.txt\nSubject: Re: Project Alpha - Budget Overrun Risk\nThis would push us from $340K to potentially $432K.",
		"FILE_READ | emails/2026-02-10_alpha_security_review.txt\nSubject: Project Alpha - Mandatory Security Review Findings\ncross-tenant data access if metric_id is guessable\nWebSocket connections must implement per-message authentication tokens\nRate limiting should be per-tenant AND per-user\nSSRF\nAudit logging is missing",
		"FILE_READ | emails/2026-02-12_alpha_phase1_complete.txt\nSubject: Project Alpha - Phase 1 Complete! Data Pipeline Live\nAfter the meeting with Linda, we got approval for the expanded budget ($410K)\n- Kafka cluster (6 brokers)\n- Flink stream processing\n- TimescaleDB populated\n- dbt models running",
		"FILE_READ | emails/2026-02-14_alpha_client_feedback.txt\nSubject: Project Alpha - Early Client Feedback from Beta Waitlist\nAcme Corp ($500K ARR potential)\nGlobalTech ($350K ARR)\nTotal pipeline from these 5 alone: $1.85M ARR. Combined with existing prospects, we're tracking toward $2.8M ARR - ahead of our $2.1M projection.",
		"FILE_READ | emails/2026-02-18_alpha_timeline_slip.txt\nSubject: Project Alpha - Updated Timeline (Phase 2 delay)\nSecurity critical items add ~1.5 weeks\nWebSocket gateway service adds ~2 weeks\n- Beta Launch: May 6\n- GA Release: May 27",
		"FILE_READ | emails/2026-02-25_alpha_frontend_progress.txt\nSubject: Project Alpha - Frontend Early Progress Update\n- Chart components using Recharts\n- Live metric widgets need the WebSocket API endpoint\n- Custom metric builder needs the metric definition API",
	}

	draft, ok := buildDeterministicWorkspaceArtifactOrchestrationDraft(
		"You have access to a collection of emails in the emails/ folder in your workspace. Save the summary to alpha_summary.md with the following sections:\n1. **Project Overview**: What is Project Alpha, what technology is being used, and what is the budget?\n2. **Timeline**: Original timeline and any changes, including current expected dates\n3. **Key Risks and Issues**: Budget concerns, security findings, technical challenges\n4. **Client/Business Impact**: Sales pipeline, client feedback, and revenue projections\n5. **Current Status**: Where the project stands right now based on the most recent updates",
		"alpha_summary.md",
		evidence,
		nil,
	)
	if !ok {
		t.Fatal("expected deterministic workspace orchestration draft")
	}
	for _, needle := range []string{
		"## Project Overview",
		"## Timeline",
		"## Key Risks and Issues",
		"## Client/Business Impact",
		"## Current Status",
		"PostgreSQL",
		"TimescaleDB",
		"FastAPI",
		"React",
		"Kafka",
		"Flink",
		"dbt",
		"Redis",
		"$340K",
		"$410K",
		"$432K",
		"Apr 21",
		"May 6",
		"May 27",
		"cross-tenant",
		"SSRF",
		"audit logging",
		"$1.85M",
		"$2.8M",
	} {
		if !strings.Contains(draft, needle) {
			t.Fatalf("expected deterministic draft to contain %q, got=%q", needle, draft)
		}
	}
}

func TestSanitizeWorkspaceArtifactOrchestrationDraft_PrefersExactAnswerBlock(t *testing.T) {
	content := "I found the answers below.\n```text\n1. 5705\n2. 2999\n3. AI & LLMs: 287\n4. Search & Research: 253\n5. SKILL.md\n6. typed WebSocket API\n7. February 7, 2026\n8. 6\n```"

	got := sanitizeWorkspaceArtifactOrchestrationDraft(content, 8)
	want := strings.Join([]string{
		"5705",
		"2999",
		"AI & LLMs: 287",
		"Search & Research: 253",
		"SKILL.md",
		"typed WebSocket API",
		"February 7, 2026",
		"6",
	}, "\n")

	if got != want {
		t.Fatalf("sanitizeWorkspaceArtifactOrchestrationDraft() = %q, want %q", got, want)
	}
}

func TestCollectWorkspaceArtifactEvidence_PreservesTailContent(t *testing.T) {
	tail := "The paper proposes 6 benchmark tasks."
	longText := strings.Repeat("filler ", 3500) + tail

	toolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "file_read"},
	}
	toolResults := []llm.Message{
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-1",
			Content:    fmt.Sprintf(`{"path":"openclaw_report.pdf","text":%q}`, longText),
		},
	}

	evidence := collectWorkspaceArtifactEvidence(toolCalls, toolResults)
	if len(evidence) == 0 {
		t.Fatal("expected evidence to be collected")
	}
	if !strings.Contains(strings.Join(evidence, "\n"), tail) {
		t.Fatalf("expected collected evidence to preserve tail marker %q", tail)
	}
}

func TestCollectWorkspaceArtifactEvidence_IncludesRawPDFEvidence(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "file_read"},
	}
	toolResults := []llm.Message{
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-1",
			Content:    `{"path":"openclaw_report.pdf","selected_pages":[1,2],"text":"AI & LLMs: 287\nSearch & Research: 253","raw_text":"[Page 1]\nTop Categories\nAI & LLMs    287\nSearch & Research    253\n\n[Page 2]\nCollected: February 7, 2026"}`,
		},
	}

	evidence := collectWorkspaceArtifactEvidence(toolCalls, toolResults)
	if len(evidence) == 0 {
		t.Fatal("expected evidence to be collected")
	}

	joined := strings.Join(evidence, "\n\n")
	if !strings.Contains(joined, "FILE_READ | RAW | openclaw_report.pdf | pages=1,2") {
		t.Fatalf("expected raw pdf header in evidence, got=%q", joined)
	}
	if !strings.Contains(joined, "AI & LLMs    287") {
		t.Fatalf("expected raw pdf layout-preserving text in evidence, got=%q", joined)
	}
	if !strings.Contains(joined, "FILE_READ | RAW | openclaw_report.pdf | page=1") {
		t.Fatalf("expected per-page raw pdf evidence block, got=%q", joined)
	}
	if !strings.Contains(joined, "FILE_READ | RAW | openclaw_report.pdf | page=2") {
		t.Fatalf("expected second per-page raw pdf evidence block, got=%q", joined)
	}
}

func TestScoreWorkspaceQuestionEvidence_PrefersExactAPIPhrase(t *testing.T) {
	question := "What type of API does the OpenClaw gateway expose?"
	generic := "The system exposes an API and several clients."
	exact := "Operationally, the Gateway is a long-lived daemon exposing a typed WebSocket API."

	if exactScore, genericScore := scoreWorkspaceQuestionEvidence(question, exact), scoreWorkspaceQuestionEvidence(question, generic); exactScore <= genericScore {
		t.Fatalf("expected exact API phrase to outrank generic API mention, exact=%d generic=%d", exactScore, genericScore)
	}
}

func TestScoreWorkspaceQuestionEvidence_PrefersExactBenchmarkTaskSentence(t *testing.T) {
	question := "How many new benchmark tasks does the paper propose?"
	generic := "The benchmark compares impact and effort across many workflow categories."
	exact := "The paper proposes 6 benchmark tasks."

	if exactScore, genericScore := scoreWorkspaceQuestionEvidence(question, exact), scoreWorkspaceQuestionEvidence(question, generic); exactScore <= genericScore {
		t.Fatalf("expected exact benchmark-task sentence to outrank generic benchmark mention, exact=%d generic=%d", exactScore, genericScore)
	}
}

func TestCollectWorkspaceQuestionEvidence_ReservesRoomForLaterQuestions(t *testing.T) {
	questions := []string{
		"How many community-built skills were in the public registry before filtering?",
		"How many skills remained after filtering?",
		"What is the largest skill category by count?",
		"What is the second-largest skill category by count?",
		"What is the name of the file that defines an OpenClaw skill?",
		"What type of API does the OpenClaw gateway expose?",
		"What date was the skills registry data collected?",
		"How many new benchmark tasks does the paper propose?",
	}
	evidence := []string{
		"FILE_READ | RAW | openclaw_report.pdf | page=1\nThe public registry had 5,705 community-built skills.\nThe paper proposes 6 benchmark tasks.",
		"FILE_READ | RAW | openclaw_report.pdf | page=2\nThe filtered list contains 2,999 skills.\nOperationally, the Gateway is a long-lived daemon exposing a typed WebSocket API.\nThe registry data was collected on February 7, 2026.",
		"FILE_READ | RAW | openclaw_report.pdf | page=3\nAI & LLMs 287\nSearch & Research 253\nSKILL.md defines an OpenClaw skill.",
	}

	selected := strings.Join(collectWorkspaceQuestionEvidence(questions, evidence), "\n\n")
	if !strings.Contains(selected, "typed WebSocket API") {
		t.Fatalf("expected later API question to retain focused evidence, got=%q", selected)
	}
	if !strings.Contains(selected, "The paper proposes 6 benchmark tasks.") {
		t.Fatalf("expected later benchmark-task question to retain focused evidence, got=%q", selected)
	}
}

func TestCollectWorkspaceQuestionEvidence_KeepsLateCriticalFactsFromLargePageBlocks(t *testing.T) {
	questions := []string{
		"What type of API does the OpenClaw gateway expose?",
		"How many new benchmark tasks does the paper propose?",
	}

	apiFiller := strings.Repeat("Gateway background detail. ", 70)
	taskFiller := strings.Repeat("Benchmark design rationale. ", 90)
	evidence := []string{
		"FILE_READ | RAW | openclaw_report.pdf | page=2\n" +
			apiFiller +
			"Operationally, the Gateway is a long-lived daemon exposing a typed WebSocket API.\n" +
			"The registry data was collected on February 7, 2026.",
		"FILE_READ | RAW | openclaw_report.pdf | page=6\n" +
			"Recommended new tasks and task modifications for PinchBench\n" +
			taskFiller +
			"Proposed tasks\n" +
			"Secure skill installation and safe configuration\n" +
			"Browser automation with no API constraints and recovery\n" +
			"Multi-channel routing and session isolation\n" +
			"Long-horizon automation and scheduling\n" +
			"Workspace-native artifact synthesis\n" +
			"Deep research with citations and synthesis\n" +
			"Difficulty: Hard\n6\n",
	}

	selected := strings.Join(collectWorkspaceQuestionEvidence(questions, evidence), "\n\n")
	if !strings.Contains(selected, "typed WebSocket API") {
		t.Fatalf("expected large page snippet to retain late API phrase, got=%q", selected)
	}
	if !strings.Contains(selected, "Deep research with citations and synthesis") || !strings.Contains(selected, "\n6") {
		t.Fatalf("expected large page snippet to retain proposed-task tail and count, got=%q", selected)
	}
}
