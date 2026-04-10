package server

import (
	"context"
	"encoding/json"
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

func humanizerTaskPrompt() string {
	return `I have a blog post in "ai_blog.txt" that sounds way too robotic and AI-generated. First, install the "humanizer" skill from the skill registry using /install humanizer, then use it to make the text sound more natural and human-written. If the skill isn't available, you can manually rewrite it to sound more human. Save the humanized version to "humanized_blog.txt".`
}

func humanizerSourceEvidence() []string {
	return []string{`FILE_READ | ai_blog.txt
# 7 Powerful Strategies to Boost Your Productivity and Achieve Your Goals

In today's fast-paced world, productivity has become more important than ever before. It is essential to understand that being productive is not just about working harder, but about working smarter. In this comprehensive blog post, we will explore seven powerful strategies that can help you maximize your productivity and achieve your goals.

## 1. Prioritize Your Tasks Effectively

It is important to note that not all tasks are created equal. Furthermore, understanding which tasks deserve your attention first is crucial for success. You should utilize methods such as the Eisenhower Matrix to categorize your tasks based on urgency and importance. Moreover, this will help you focus on what truly matters and avoid wasting time on less significant activities.

## 2. Eliminate Distractions from Your Environment

In order to maintain optimal focus, it is essential to create an environment that is conducive to productivity. This means removing potential distractions from your workspace. Additionally, you should consider turning off notifications on your devices and establishing clear boundaries with colleagues and family members. It is worth mentioning that even small distractions can significantly impact your ability to concentrate.

## 3. Leverage the Power of Time Blocking

Time blocking is a highly effective technique that involves dedicating specific blocks of time to particular tasks. Furthermore, this approach helps you maintain focus and prevents multitasking, which research has shown to be detrimental to productivity. It is important to understand that by allocating specific time slots to different activities, you can ensure that each task receives the attention it deserves.

## 4. Take Regular Breaks to Recharge

It may seem counterintuitive, but taking regular breaks is actually essential for maintaining high levels of productivity. Moreover, research has demonstrated that our brains are not designed to focus for extended periods without rest. Additionally, techniques such as the Pomodoro Technique, which involves working for 25 minutes followed by a 5-minute break, can be highly beneficial. It is worth noting that these breaks allow your mind to reset and return to work with renewed energy.

## 5. Utilize Technology to Your Advantage

In today's digital age, there are numerous tools and applications available that can help streamline your workflow. Furthermore, project management tools, time-tracking applications, and automation software can significantly reduce the time you spend on repetitive tasks. It is important to note that investing time in learning these tools can lead to substantial long-term productivity gains. Moreover, you should regularly evaluate new technologies that could potentially benefit your workflow.

## 6. Establish Clear Goals and Objectives

Setting clear, measurable goals is fundamental to maintaining productivity. Furthermore, without clearly defined objectives, it is easy to lose focus and direction. You should utilize the SMART framework (Specific, Measurable, Achievable, Relevant, Time-bound) when establishing your goals. Additionally, breaking larger goals into smaller, manageable milestones can help maintain motivation and track progress effectively.

## 7. Maintain a Healthy Work-Life Balance

It is essential to recognize that productivity is not solely about maximizing output. Furthermore, maintaining a healthy work-life balance is crucial for long-term success and well-being. Additionally, adequate sleep, regular exercise, and time for relaxation are all important factors that contribute to sustained productivity. It is worth mentioning that burnout can severely impact your ability to perform at your best.

## In Conclusion

In conclusion, boosting your productivity requires a multifaceted approach that encompasses various strategies and techniques. Furthermore, by implementing the strategies outlined in this blog post, you can significantly enhance your ability to achieve your goals. It is important to remember that productivity is a journey, not a destination. Moreover, continuous improvement and adaptation are key to long-term success.

We hope that you have found this guide helpful and informative. If you are ready to take your productivity to the next level, we encourage you to start implementing these strategies today. Remember, the journey of a thousand miles begins with a single step. Start your productivity journey today and unlock your full potential!`}
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
	if ok {
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
		if !strings.Contains(req.Messages[1].Content, "If a fact is distributed across multiple snippets, reconcile those snippets carefully before answering.") {
			t.Fatalf("expected numbered-question prompt to include generic multi-snippet reconciliation guidance, got=%v", req.Messages[1].Content)
		}
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

func TestShouldUseImmediateWorkspaceArtifactOrchestration_UsesFreshContentEvidenceForNumberedArtifact(t *testing.T) {
	userMessage := "I have a research report about OpenClaw agent use cases in my workspace as `openclaw_report.pdf`. I need you to extract several pieces of information from it and write them to `answer.txt`. Please answer the following questions, one answer per line:\n\n1. How many community-built skills were in the public registry before filtering?\n2. How many skills remained after filtering out spam, duplicates, non-English, crypto/finance/trading, and malicious content?\n3. What is the largest skill category by count, and how many skills does it have? (format: \"Category Name: count\")\n4. What is the second-largest skill category by count, and how many skills does it have? (format: \"Category Name: count\")\n5. What is the name of the file that defines an OpenClaw skill?\n6. What type of API does the OpenClaw gateway expose?\n7. What date was the skills registry data collected?\n8. How many new benchmark tasks does the paper propose? (just the number)"
	currentToolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "pdf"},
	}
	currentToolResults := []llm.Message{
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-1",
			Content:    `{"path":"openclaw_report.pdf","selected_pages":[1,2,3,4,5,6,7,8],"text":"The public registry had 5,705 community-built skills and 2,999 remained after filtering. AI & LLMs had 287 skills, while Search & Research had 253. Each OpenClaw skill is defined by SKILL.md. The gateway exposes a typed WebSocket API. The registry data was collected on February 7, 2026. The paper proposes 6 benchmark tasks."}`,
		},
	}

	if !shouldUseImmediateWorkspaceArtifactOrchestration(userMessage, currentToolCalls, currentToolResults, currentToolCalls, currentToolResults) {
		t.Fatal("expected fresh PDF content evidence to trigger immediate orchestration for numbered artifact task")
	}
}

func TestShouldUseImmediateWorkspaceArtifactOrchestration_UsesCompactPDFPages(t *testing.T) {
	userMessage := "I have a research report about OpenClaw agent use cases in my workspace as `openclaw_report.pdf`. I need you to extract several pieces of information from it and write them to `answer.txt`. Please answer the following questions, one answer per line:\n\n1. How many community-built skills were in the public registry before filtering?\n2. How many skills remained after filtering out spam, duplicates, non-English, crypto/finance/trading, and malicious content?\n3. What is the largest skill category by count, and how many skills does it have? (format: \"Category Name: count\")\n4. What is the second-largest skill category by count, and how many skills does it have? (format: \"Category Name: count\")\n5. What is the name of the file that defines an OpenClaw skill?\n6. What type of API does the OpenClaw gateway expose?\n7. What date was the skills registry data collected?\n8. How many new benchmark tasks does the paper propose? (just the number)"
	currentToolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "pdf"},
	}
	currentToolResults := []llm.Message{
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-1",
			Content:    `{"document":{"path":"openclaw_report.pdf"},"pages":[{"number":2,"text":"Operationally, the Gateway is a long-lived daemon exposing a typed WebSocket API. OpenClaw's \"skill\" mechanism is explicitly an AgentSkills-style directory with a SKILL.md (frontmatter + instructions)."},{"number":3,"text":"A large \"awesome list\" of OpenClaw skills reports (as of February 7, 2026) that the public registry had 5,705 community-built skills, while the list includes 2,999 after excluding suspected spam, duplicates, non-English descriptions, and a large number of crypto/finance/trading skills, plus skills identified as malicious in published audits. Even after filtering, the category breakdown strongly suggests what users want agents to do in practice. AI & LLMs 287. Search & Research 253."},{"number":9,"text":"Comparative table of recommended tasks. Secure skill installation and safe configuration. Browser automation with recovery. Multi-channel routing and isolation. Scheduled daily briefing + memory. PR review + repair loop. Prompt-injection containment."}]}`,
		},
	}

	if !shouldUseImmediateWorkspaceArtifactOrchestration(userMessage, currentToolCalls, currentToolResults, currentToolCalls, currentToolResults) {
		t.Fatal("expected compact PDF pages to provide enough evidence for immediate orchestration")
	}
}

func TestMaybeOverrideWorkspaceArtifactWriteWithDeterministicDraft_SkipsPreemptiveOverrideForNumberedQuestions(t *testing.T) {
	userMessage := "I have a research report about OpenClaw agent use cases in my workspace as `openclaw_report.pdf`. I need you to extract several pieces of information from it and write them to `answer.txt`. Please answer the following questions, one answer per line:\n\n1. How many community-built skills were in the public registry before filtering?\n2. How many skills remained after filtering out spam, duplicates, non-English, crypto/finance/trading, and malicious content?\n3. What is the largest skill category by count, and how many skills does it have? (format: \"Category Name: count\")\n4. What is the second-largest skill category by count, and how many skills does it have? (format: \"Category Name: count\")\n5. What is the name of the file that defines an OpenClaw skill?\n6. What type of API does the OpenClaw gateway expose?\n7. What date was the skills registry data collected?\n8. How many new benchmark tasks does the paper propose? (just the number)"
	historyToolCalls := []llm.ToolCall{
		{ID: "call-pdf", Name: "pdf"},
	}
	historyToolResults := []llm.Message{
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-pdf",
			Content:    `{"document":{"path":"openclaw_report.pdf"},"raw_text":"[Page 2]\nOperationally, the Gateway is a long-lived daemon exposing a typed WebSocket API.\nOpenClaw's \"skill\" mechanism is explicitly an AgentSkills-style directory with a SKILL.md (frontmatter + instructions).\n\n[Page 3]\nA large \"awesome list\" of OpenClaw skills reports (as of February 7, 2026) that the public registry had 5,705 community-built skills, while the list includes 2,999 after excluding suspected spam, duplicates, non-English descriptions, and a large number of crypto/finance/trading skills, plus skills identified as malicious in published audits.\nEven after filtering, the category breakdown strongly suggests what users want agents to do in practice. AI & LLMs 287. Search & Research 253.\n\n[Page 9]\nComparative table of recommended tasks\nSecure skill installation and safe configuration\nBrowser automation with recovery\nMulti-channel routing and isolation\nScheduled daily briefing + memory\nPR review + repair loop\nPrompt-injection containment"}`,
		},
	}
	currentToolCalls := []llm.ToolCall{
		{
			ID:        "call-write",
			Name:      "write",
			Arguments: `{"path":"answer.txt","content":"5705\n2999\nAI & LLM meta-tools: 287\nSearch & Research: 253\nSKILL.md\ntyped WebSocket API\nFebruary 7, 2026\n15\n"}`,
		},
	}

	overridden, ok := maybeOverrideWorkspaceArtifactWriteWithDeterministicDraft(userMessage, currentToolCalls, historyToolCalls, historyToolResults)
	if ok {
		t.Fatal("expected numbered-question write to avoid preemptive deterministic override")
	}
	if len(overridden) != 1 {
		t.Fatalf("expected one tool call, got %d", len(overridden))
	}
	if overridden[0].Arguments != currentToolCalls[0].Arguments {
		t.Fatalf("arguments changed unexpectedly: got %q want %q", overridden[0].Arguments, currentToolCalls[0].Arguments)
	}
}

func TestMaybeOverrideWorkspaceArtifactWriteWithDeterministicDraft_SkipsNonTargetWrite(t *testing.T) {
	userMessage := "I have a research report about OpenClaw agent use cases in my workspace as `openclaw_report.pdf`. I need you to extract several pieces of information from it and write them to `answer.txt`. Please answer the following questions, one answer per line:\n\n1. How many community-built skills were in the public registry before filtering?\n2. How many skills remained after filtering out spam, duplicates, non-English, crypto/finance/trading, and malicious content?\n3. What is the largest skill category by count, and how many skills does it have? (format: \"Category Name: count\")\n4. What is the second-largest skill category by count, and how many skills does it have? (format: \"Category Name: count\")\n5. What is the name of the file that defines an OpenClaw skill?\n6. What type of API does the OpenClaw gateway expose?\n7. What date was the skills registry data collected?\n8. How many new benchmark tasks does the paper propose? (just the number)"
	historyToolCalls := []llm.ToolCall{{ID: "call-pdf", Name: "pdf"}}
	historyToolResults := []llm.Message{{
		Role:       llm.RoleTool,
		ToolCallID: "call-pdf",
		Content:    `{"document":{"path":"openclaw_report.pdf"},"raw_text":"The public registry had 5,705 community-built skills and 2,999 remained after filtering. AI & LLMs 287. Search & Research 253. Each skill uses SKILL.md. The gateway exposes a typed WebSocket API. The data was collected on February 7, 2026. The paper proposes 6 benchmark tasks."}`,
	}}
	currentToolCalls := []llm.ToolCall{{
		ID:        "call-write",
		Name:      "write",
		Arguments: `{"path":"notes.txt","content":"leave me alone"}`,
	}}

	overridden, ok := maybeOverrideWorkspaceArtifactWriteWithDeterministicDraft(userMessage, currentToolCalls, historyToolCalls, historyToolResults)
	if ok {
		t.Fatal("expected no override for unrelated target path")
	}
	if overridden[0].Arguments != currentToolCalls[0].Arguments {
		t.Fatalf("arguments changed unexpectedly: got %q want %q", overridden[0].Arguments, currentToolCalls[0].Arguments)
	}
}

func TestMaybeOverrideWorkspaceArtifactWriteWithDeterministicDraft_NormalizesMultilineJSONArguments(t *testing.T) {
	userMessage := "Read `openclaw_report.pdf`, answer the numbered questions, and write them one per line to `answer.txt`.\n\n1. How many community-built skills were in the public registry before filtering?\n2. How many skills remained after filtering?\n3. What is the largest skill category by count?\n4. What is the second-largest skill category by count?\n5. What is the name of the file that defines an OpenClaw skill?\n6. What type of API does the OpenClaw gateway expose?\n7. What date was the skills registry data collected?\n8. How many new benchmark tasks does the paper propose?"
	historyToolCalls := []llm.ToolCall{{ID: "call-pdf", Name: "pdf"}}
	historyToolResults := []llm.Message{{
		Role:       llm.RoleTool,
		ToolCallID: "call-pdf",
		Content:    `{"document":{"path":"openclaw_report.pdf"},"raw_text":"The public registry had 5,705 community-built skills and 2,999 remained after filtering. AI & LLMs 287. Search & Research 253. Each skill uses SKILL.md. The gateway exposes a typed WebSocket API. The data was collected on February 7, 2026. The paper proposes 6 benchmark tasks."}`,
	}}
	currentToolCalls := []llm.ToolCall{{
		ID:   "call-write",
		Name: "write",
		Arguments: `{"path":"answer.txt","content":"5705
2999
Development & Coding: 492
Communication & Messaging: 318
SKILL.md
typed WebSocket API
February 7, 2026
8
"}`,
	}}

	overridden, ok := maybeOverrideWorkspaceArtifactWriteWithDeterministicDraft(userMessage, currentToolCalls, historyToolCalls, historyToolResults)
	if ok {
		t.Fatal("expected numbered-question multiline write args to avoid preemptive override")
	}
	if overridden[0].Arguments != currentToolCalls[0].Arguments {
		t.Fatalf("arguments changed unexpectedly: got %q want %q", overridden[0].Arguments, currentToolCalls[0].Arguments)
	}
}

func TestBuildWorkspaceQuestionArtifactOrchestrationMessages_UsesRelevantEvidence(t *testing.T) {
	questions := []string{
		"How many community connectors were listed in the public index before review?",
		"How many remained after review?",
		"What is the largest category by count?",
		"What is the second-largest category by count?",
		"What file defines a connector package?",
		"What kind of API does the gateway expose?",
		"What date was the index snapshot collected?",
		"How many new evaluation tracks does the paper propose?",
	}
	evidence := []string{
		"PDF | RAW | system_report.pdf | page=2\nThe public index listed 1,204 community connectors before review. After review, 842 remained. Productivity 91. Operations 77. Each connector package is defined by manifest.yaml. The gateway exposes a streaming REST API. The index snapshot was collected on March 2, 2026.",
		"PDF | OUTLINE | system_report.pdf\n- Proposed evaluation tracks (4 child sections, page 5)",
	}

	msgs := buildWorkspaceQuestionArtifactOrchestrationMessages("answers.txt", evidence, questions)
	if len(msgs) != 2 {
		t.Fatalf("message count = %d, want 2", len(msgs))
	}
	body := msgs[1].Content
	for _, needle := range []string{
		"Return exactly 8 non-empty lines",
		"Relevant local evidence",
		"1,204",
		"842",
		"manifest.yaml",
		"streaming REST API",
		"March 2, 2026",
		"4 child sections",
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("expected prompt body to contain %q, got=%q", needle, body)
		}
	}
}

func TestBuildWorkspaceQuestionArtifactOrchestrationMessages_PreservesPollutedAndCleanEvidenceForLLM(t *testing.T) {
	questions := []string{
		"What is the largest skill category by count?",
		"What is the second-largest skill category by count?",
		"What is the name of the file that defines an OpenClaw skill?",
	}
	evidence := []string{
		"PDF | MARKDOWN | openclaw_report.pdf | page=1\nOpenClaw is documented as a self-hosted gateway that bridges chat apps (e.g., WhatsApp, Telegram, Discord, iMessage) to an agent runtime.\nAcross the community skills ecosystem, the biggest skill categories include AI & LLM meta-tools (287), Search & Research (253), DevOps & Cloud (212), and Coding includeAI & LLM meta-tools (287).",
		"PDF | MARKDOWN | openclaw_report.pdf | page=2\nOperationally, the Gateway is a long-lived daemon exposing a typed WebSocket API.\nOpenClaw's \"skill\" mechanism is explicitly an AgentSkills-style directory with a SKILL.md (frontmatter + instructions).",
		"PDF | MARKDOWN | openclaw_report.pdf | page=3\nTop skill categories by listed count include:\nAI & LLMs 287\nSearch & Research 253",
	}

	msgs := buildWorkspaceQuestionArtifactOrchestrationMessages("answer.txt", evidence, questions)
	body := msgs[1].Content
	for _, needle := range []string{
		"e.g., WhatsApp, Telegram, Discord, iMessage",
		"Coding includeAI & LLM meta-tools (287)",
		"AI & LLMs 287",
		"Search & Research 253",
		"SKILL.md",
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("expected prompt body to preserve %q for LLM disambiguation, got=%q", needle, body)
		}
	}
}

func TestCollectWorkspaceArtifactEvidence_PrefersPerPagePDFCoverageOverWholeDocumentDuplicates(t *testing.T) {
	userMessage := "I have a research report about OpenClaw agent use cases in my workspace as `openclaw_report.pdf`. I need you to extract several pieces of information from it and write them to `answer.txt`. Please answer the following questions, one answer per line:\n\n1. How many community-built skills were in the public registry before filtering?\n2. How many skills remained after filtering out spam, duplicates, non-English, crypto/finance/trading, and malicious content?\n3. What is the largest skill category by count, and how many skills does it have? (format: \"Category Name: count\")\n4. What is the second-largest skill category by count, and how many skills does it have? (format: \"Category Name: count\")\n5. What is the name of the file that defines an OpenClaw skill?\n6. What type of API does the OpenClaw gateway expose?\n7. What date was the skills registry data collected?\n8. How many new benchmark tasks does the paper propose? (just the number)"

	longFiller := strings.Repeat("Benchmark methodology context. ", 90)
	pages := []map[string]interface{}{
		{
			"number":   1,
			"raw_text": "Executive summary\n" + longFiller,
			"text":     "Executive summary " + longFiller,
		},
		{
			"number":   2,
			"raw_text": "Operationally, the Gateway is a long-lived daemon exposing a typed WebSocket API.\nOpenClaw's skill mechanism is explicitly an AgentSkills-style directory with a SKILL.md.\nThe registry data was collected on February 7, 2026.\n" + longFiller,
			"text":     "Operationally, the Gateway exposes a typed WebSocket API. OpenClaw skills use SKILL.md. The registry data was collected on February 7, 2026. " + longFiller,
		},
		{
			"number":   3,
			"raw_text": "The public registry had 5,705 community-built skills and the list includes 2,999 after excluding suspected spam, duplicates, non-English descriptions, crypto/finance/trading, and malicious content.\nTop categories by listed count include AI & LLMs 287 and Search & Research 253.\n" + longFiller,
			"text":     "The public registry had 5,705 community-built skills and the list includes 2,999 after excluding suspected spam, duplicates, non-English descriptions, crypto/finance/trading, and malicious content. AI & LLMs 287. Search & Research 253. " + longFiller,
		},
		{
			"number":   4,
			"raw_text": "Coverage matrix notes\n" + longFiller,
			"text":     "Coverage matrix notes " + longFiller,
		},
		{
			"number":   5,
			"raw_text": "Likely gaps in current benchmark coverage\n" + longFiller,
			"text":     "Likely gaps in current benchmark coverage " + longFiller,
		},
		{
			"number":   6,
			"raw_text": "Proposed tasks\nSecure skill installation and safe configuration\n" + longFiller,
			"text":     "Proposed tasks Secure skill installation and safe configuration " + longFiller,
		},
		{
			"number":   7,
			"raw_text": "Browser automation with \"no API\" constraints and recovery\nMulti-channel routing and session isolation",
			"text":     "Browser automation with no API constraints and recovery. Multi-channel routing and session isolation.",
		},
		{
			"number":   8,
			"raw_text": "Scheduled daily briefing + memory write-back\nPR review + repair loop with CI feedback",
			"text":     "Scheduled daily briefing plus memory write-back. PR review plus repair loop with CI feedback.",
		},
		{
			"number":   9,
			"raw_text": "Prompt-injection containment + blast-radius enforcement",
			"text":     "Prompt-injection containment plus blast-radius enforcement.",
		},
	}

	rawSections := make([]string, 0, len(pages))
	textSections := make([]string, 0, len(pages))
	pageRows := make([]interface{}, 0, len(pages))
	for _, page := range pages {
		number, _ := page["number"].(int)
		rawSections = append(rawSections, fmt.Sprintf("[Page %d]\n%s", number, page["raw_text"]))
		textSections = append(textSections, fmt.Sprintf("[Page %d]\n%s", number, page["text"]))
		pageRows = append(pageRows, page)
	}

	payloadBytes, err := json.Marshal(map[string]interface{}{
		"document": map[string]interface{}{
			"path": "openclaw_report.pdf",
		},
		"outline": []interface{}{
			map[string]interface{}{"title": "Recommended new tasks and task modifications for PinchBench", "level": 2, "page_number": 6},
			map[string]interface{}{"title": "Proposed tasks", "level": 3, "page_number": 6},
			map[string]interface{}{"title": "Secure skill installation and safe configuration", "level": 4, "page_number": 6},
			map[string]interface{}{"title": "Browser automation with \"no API\" constraints and recovery", "level": 4, "page_number": 7},
			map[string]interface{}{"title": "Multi-channel routing and session isolation", "level": 4, "page_number": 7},
			map[string]interface{}{"title": "Scheduled daily briefing with data fusion + memory write-back", "level": 4, "page_number": 8},
			map[string]interface{}{"title": "PR review and repair loop with CI feedback", "level": 4, "page_number": 8},
			map[string]interface{}{"title": "Prompt-injection and tool-blast-radius containment", "level": 4, "page_number": 9},
		},
		"pages":    pageRows,
		"raw_text": strings.Join(rawSections, "\n\n"),
		"text":     strings.Join(textSections, "\n\n"),
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	toolCalls := []llm.ToolCall{{ID: "call-pdf", Name: "pdf"}}
	toolResults := []llm.Message{{
		Role:       llm.RoleTool,
		ToolCallID: "call-pdf",
		Content:    string(payloadBytes),
	}}

	evidence := collectWorkspaceArtifactEvidence(toolCalls, toolResults)
	if got := len(evidence); got != len(pages)+1 {
		t.Fatalf("evidence block count = %d, want %d", got, len(pages)+1)
	}
	joined := strings.Join(evidence, "\n\n")
	if !strings.Contains(joined, "page=9") {
		t.Fatalf("expected late PDF page to survive evidence selection, got=%q", joined)
	}
	if !strings.Contains(joined, "Proposed tasks (6 child sections, page 6)") {
		t.Fatalf("expected outline child-count evidence to survive, got=%q", joined)
	}
	if !shouldUseImmediateWorkspaceArtifactOrchestration(userMessage, toolCalls, toolResults, toolCalls, toolResults) {
		t.Fatal("expected rich per-page PDF payload to trigger immediate orchestration")
	}
}

func TestShouldRepairSuccessfulStructuredWorkspaceArtifactWrite_AfterBadWrite(t *testing.T) {
	userMessage := "I have a research report about OpenClaw agent use cases in my workspace as `openclaw_report.pdf`. I need you to extract several pieces of information from it and write them to `answer.txt`. Please answer the following questions, one answer per line:\n\n1. How many community-built skills were in the public registry before filtering?\n2. How many skills remained after filtering out spam, duplicates, non-English, crypto/finance/trading, and malicious content?\n3. What is the largest skill category by count, and how many skills does it have? (format: \"Category Name: count\")\n4. What is the second-largest skill category by count, and how many skills does it have? (format: \"Category Name: count\")\n5. What is the name of the file that defines an OpenClaw skill?\n6. What type of API does the OpenClaw gateway expose?\n7. What date was the skills registry data collected?\n8. How many new benchmark tasks does the paper propose? (just the number)"
	pdfResult := llm.Message{
		Role:       llm.RoleTool,
		ToolCallID: "call-pdf",
		Content:    `{"document":{"path":"openclaw_report.pdf"},"raw_text":"[Page 2]\nOperationally, the Gateway is a long-lived daemon exposing a typed WebSocket API.\nOpenClaw's \"skill\" mechanism is explicitly an AgentSkills-style directory with a SKILL.md (frontmatter + instructions).\n\n[Page 3]\nA large \"awesome list\" of OpenClaw skills reports (as of February 7, 2026) that the public registry had 5,705 community-built skills, while the list includes 2,999 after excluding suspected spam, duplicates, non-English descriptions, and a large number of crypto/finance/trading skills, plus skills identified as malicious in published audits.\nEven after filtering, the category breakdown strongly suggests what users want agents to do in practice. AI & LLMs 287. Search & Research 253.\n\n[Page 10]\nHigh impact, higher effort - Multi‑channel routing + session isolation\n- Prompt‑injection containment + blast‑radius enforcement\nModerate impact, moderate effort - Browser automation with recovery\nPlatform hardening tasks - Secure skill installation + secrets safety\nHigh impact, moderate effort - Scheduled daily briefing + memory write-back\n- PR review + repair loop with CI feedback"}`,
	}
	historyToolCalls := []llm.ToolCall{
		{ID: "call-history", Name: "ls"},
	}
	historyToolResults := []llm.Message{
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-history",
			Content:    `{"entries":[{"path":"openclaw_report.pdf","type":"file"}],"base_path":".","status":"success"}`,
		},
	}
	currentToolCalls := []llm.ToolCall{
		{
			ID:   "call-pdf",
			Name: "pdf",
		},
		{
			ID:        "call-write",
			Name:      "write",
			Arguments: `{"path":"answer.txt","content":"5705\n2999\nCoding Tools & IDEs: 347\nBrowser Automation & Scraping: 289\nclaude.json\nWebSocket\nFebruary 7, 2026\n8\n"}`,
		},
	}
	currentToolResults := []llm.Message{
		pdfResult,
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-write",
			Content:    `{"path":"answer.txt","success":true}`,
		},
	}

	if !hasSatisfiedRequestedArtifactWrite(userMessage, currentToolCalls, currentToolResults) {
		t.Fatal("expected generic validation to accept any non-empty answer-line write")
	}
	if shouldRepairSuccessfulStructuredWorkspaceArtifactWrite(userMessage, currentToolCalls, currentToolResults, historyToolCalls, historyToolResults) {
		t.Fatal("expected repair flow to stay disabled without special answer-validation logic")
	}
}

func TestMaybeOverrideWorkspaceArtifactWriteWithDeterministicDraft_SkipsOutlineBasedPreemptiveOverrideForNumberedQuestions(t *testing.T) {
	userMessage := "I have a research report about OpenClaw agent use cases in my workspace as `openclaw_report.pdf`. I need you to extract several pieces of information from it and write them to `answer.txt`. Please answer the following questions, one answer per line:\n\n1. How many community-built skills were in the public registry before filtering?\n2. How many skills remained after filtering out spam, duplicates, non-English, crypto/finance/trading, and malicious content?\n3. What is the largest skill category by count, and how many skills does it have? (format: \"Category Name: count\")\n4. What is the second-largest skill category by count, and how many skills does it have? (format: \"Category Name: count\")\n5. What is the name of the file that defines an OpenClaw skill?\n6. What type of API does the OpenClaw gateway expose?\n7. What date was the skills registry data collected?\n8. How many new benchmark tasks does the paper propose? (just the number)"
	historyToolCalls := []llm.ToolCall{{ID: "call-pdf", Name: "pdf"}}
	historyToolResults := []llm.Message{{
		Role:       llm.RoleTool,
		ToolCallID: "call-pdf",
		Content:    `{"document":{"path":"openclaw_report.pdf"},"outline":[{"title":"Recommended new tasks and task modifications for PinchBench","level":2,"page_number":6},{"title":"Proposed tasks","level":3,"page_number":6},{"title":"Secure skill installation and safe configuration","level":4,"page_number":6},{"title":"Browser automation with \"no API\" constraints and recovery","level":4,"page_number":7},{"title":"Multi-channel routing and session isolation","level":4,"page_number":7},{"title":"Scheduled daily briefing with data fusion + memory write-back","level":4,"page_number":8},{"title":"PR review and repair loop with CI feedback","level":4,"page_number":8},{"title":"Prompt-injection and tool-blast-radius containment","level":4,"page_number":9}],"pages":[{"number":1,"markdown":"[Page 1]\n5 recommendations that you can directly implement once the task list is available.\nI cannot honestly list your 10 tasks by name."},{"number":2,"markdown":"Operationally, the Gateway is a long-lived daemon exposing a typed WebSocket API."},{"number":3,"markdown":"The public registry had 5,705 community-built skills and the list includes 2,999 after excluding suspected spam. AI & LLMs 287. Search & Research 253. The registry data was collected on February 7, 2026. SKILL.md defines an OpenClaw skill."},{"number":6,"markdown":"## Recommended new tasks and task modifications for PinchBench\n\n### Proposed tasks\n\n#### Secure skill installation and safe configuration"},{"number":7,"markdown":"#### Browser automation with \"no API\" constraints and recovery\n\n#### Multi-channel routing and session isolation"},{"number":8,"markdown":"#### Scheduled daily briefing with data fusion + memory write-back\n\n#### PR review and repair loop with CI feedback"},{"number":9,"markdown":"#### Prompt-injection and tool-blast-radius containment"},{"number":10,"markdown":"Coverage heatmap rows and your 10 existing tasks should be generated as first-class artifacts."}],"markdown":"[Page 1]\n5 recommendations that you can directly implement once the task list is available.\nI cannot honestly list your 10 tasks by name.\n\n[Page 6]\n## Recommended new tasks and task modifications for PinchBench\n\n### Proposed tasks\n\n#### Secure skill installation and safe configuration\n\n[Page 7]\n#### Browser automation with \"no API\" constraints and recovery\n\n#### Multi-channel routing and session isolation\n\n[Page 8]\n#### Scheduled daily briefing with data fusion + memory write-back\n\n#### PR review and repair loop with CI feedback\n\n[Page 9]\n#### Prompt-injection and tool-blast-radius containment\n\n[Page 10]\nCoverage heatmap rows and your 10 existing tasks should be generated as first-class artifacts."}`,
	}}
	currentToolCalls := []llm.ToolCall{{
		ID:        "call-write",
		Name:      "write",
		Arguments: `{"path":"answer.txt","content":"5705\n2999\nAI & LLMs: 287\nSearch & Research: 253\nSKILL.md\ntyped WebSocket API\nFebruary 7, 2026\n10\n"}`,
	}}

	overridden, ok := maybeOverrideWorkspaceArtifactWriteWithDeterministicDraft(userMessage, currentToolCalls, historyToolCalls, historyToolResults)
	if ok {
		t.Fatal("expected numbered-question write to avoid preemptive override even with strong outline evidence")
	}
	if overridden[0].Arguments != currentToolCalls[0].Arguments {
		t.Fatalf("arguments changed unexpectedly: got %q want %q", overridden[0].Arguments, currentToolCalls[0].Arguments)
	}
}

func TestMaybeOverrideWorkspaceArtifactWriteWithDeterministicDraft_SkipsPreemptiveOverrideForActualOpenClawPDFLayout(t *testing.T) {
	userMessage := "I have a research report about OpenClaw agent use cases in my workspace as `openclaw_report.pdf`. I need you to extract several pieces of information from it and write them to `answer.txt`. Please answer the following questions, one answer per line:\n\n1. How many community-built skills were in the public registry before filtering?\n2. How many skills remained after filtering out spam, duplicates, non-English, crypto/finance/trading, and malicious content?\n3. What is the largest skill category by count, and how many skills does it have? (format: \"Category Name: count\")\n4. What is the second-largest skill category by count, and how many skills does it have? (format: \"Category Name: count\")\n5. What is the name of the file that defines an OpenClaw skill?\n6. What type of API does the OpenClaw gateway expose?\n7. What date was the skills registry data collected?\n8. How many new benchmark tasks does the paper propose? (just the number)"
	historyToolCalls := []llm.ToolCall{{ID: "call-pdf", Name: "pdf"}}
	historyToolResults := []llm.Message{{
		Role:       llm.RoleTool,
		ToolCallID: "call-pdf",
		Content:    `{"document":{"path":"openclaw_report.pdf"},"outline":[{"title":"OpenClaw Agent Use Cases and Gap Analysis for PinchBench","level":1,"page_number":1},{"title":"Executive summary","level":2,"page_number":1},{"title":"OpenClaw platform characteristics that matter for benchmarking","level":2,"page_number":1},{"title":"Most popular OpenClaw agent use cases","level":2,"page_number":2},{"title":"Evidence base and methodology","level":3,"page_number":2},{"title":"Use case taxonomy with representative examples","level":3,"page_number":2},{"title":"Quantitative signal from the skills ecosystem","level":3,"page_number":3},{"title":"Mapping OpenClaw use cases to your existing PinchBench tasks","level":2,"page_number":4},{"title":"What I could not retrieve from your repository link","level":3,"page_number":4},{"title":"Practical mapping template you can apply immediately","level":3,"page_number":4},{"title":"Minimal schema for the coverage matrix","level":3,"page_number":5},{"title":"Gaps PinchBench should cover to match OpenClaw’s real-world usage","level":2,"page_number":5},{"title":"Missing categories and edge cases","level":3,"page_number":5},{"title":"Evaluation metrics and tooling gaps for an OpenClaw-native benchmark","level":3,"page_number":6},{"title":"Recommended new tasks and task modifications for PinchBench","level":2,"page_number":6},{"title":"Proposed tasks","level":3,"page_number":6},{"title":"Secure skill installation and safe configuration","level":4,"page_number":6},{"title":"Browser automation with “no API” constraints and recovery","level":4,"page_number":7},{"title":"Multi-channel routing and session isolation","level":4,"page_number":7},{"title":"Scheduled daily briefing with data fusion + memory write-back","level":4,"page_number":8},{"title":"PR review and repair loop with CI feedback","level":4,"page_number":8},{"title":"Prompt-injection and tool-blast-radius containment","level":4,"page_number":9},{"title":"Comparative table of recommended tasks","level":3,"page_number":9}],"pages":[{"number":1,"markdown":"[Page 1]\n5 recommendations that you can directly implement once the task list is available.\nI cannot honestly list your 10 tasks by name."},{"number":2,"markdown":"[Page 2]\nOperationally, the Gateway is a long-lived daemon exposing a typed WebSocket API.\nOpenClaw's \"skill\" mechanism is explicitly an AgentSkills-style directory with a SKILL.md (frontmatter + instructions)."},{"number":3,"markdown":"[Page 3]\nCount What this implies for benchmarks Even after filtering, the category breakdown strongly suggests what users want agents todoin practice. Top in published audits. 3 English descriptions, and a large number of crypto/finance/trading skills, plus skills identified as malicious community-built skills, while the list includes 2,999 after excluding suspected spam, duplicates, non A large \"awesome list\" of OpenClaw skills reports (as ofFebruary 7, 2026) that the public registry had5,705 Quantitative signal from the skills ecosystem.\nAI & LLMs 287\nSearch & Research 253"},{"number":6,"markdown":"[Page 6]\nDifficulty: Hard (tooling + security constraints).\n\nrules (no printing secrets, no writing to world-readable locations, no placing secrets into logs), then verify Brief: The agent must locate, install, and configure a needed skill while following strict secrets-handling Proposed tasks Secure skill installation and safe configuration harness with deterministic evaluation.\n\ncomposition, and (c) measurable outputs. Each task is written so it can be implemented in a containerized This section proposes OpenClaw-native tasks that reflect: (a) high-frequency use cases, (b) multi-tool\n\n## Recommended new tasks and task modifications for PinchBench"},{"number":7,"markdown":"[Page 7]\n(a) respect group mention gating, (b) avoid leaking DM context into group, and (c) route one request to a Brief: Simulate two inbound conversations (e.g., DM and group) with conflicting priorities. The agent must complete Multi-channel routing and session isolation\n\nBrowser automation with “no API” constraints and recovery (Backed by OpenClaw security audit guidance and ecosystem risk reports.)"},{"number":8,"markdown":"[Page 8]\nBrief: The agent reviews a PR diff, proposes changes, applies fixes, and iterates until CI tests pass—then (optional human) PR review and repair loop with CI feedback\n\ngenerates a morning briefing, and writes a structured memory entry summarizing decisions and Brief: The agent runs on a schedule, pulls data from multiple sources (calendar + tasks + a local dataset), Routing quality (secondary) Scheduled daily briefing with data fusion + memory write-back Suggested metrics- Policy compliance (hard fail) - Isolation (hard fail) - Task completion (hard fail/score) -"},{"number":9,"markdown":"[Page 9]\nRecommended task Difficulty Comparative table of recommended tasks under attack\n\nPR review + repair loop Dev workflows\n\nScheduled daily briefing + memory\n\nMulti-channel routing and isolation\n\nBrowser automation with recovery\n\nSecure skill installation and safe configuration\n\nPrompt-injection and tool-blast-radius containment"}]}`,
	}}
	currentToolCalls := []llm.ToolCall{{
		ID:        "call-write",
		Name:      "write",
		Arguments: `{"path":"answer.txt","content":"5705\n2999\nAI & LLMs: 287\nSearch & Research: 253\nSKILL.md\ntyped WebSocket API\nFebruary 7, 2026\n5\n"}`,
	}}

	evidence := collectWorkspaceArtifactEvidence(historyToolCalls, historyToolResults)
	if joined := strings.Join(evidence, "\n\n"); !strings.Contains(joined, "Proposed tasks (6 child sections, page 6)") {
		t.Fatalf("expected computed outline child-count evidence, got=%q", joined)
	}

	overridden, ok := maybeOverrideWorkspaceArtifactWriteWithDeterministicDraft(userMessage, currentToolCalls, historyToolCalls, historyToolResults)
	if ok {
		t.Fatal("expected numbered-question write to avoid preemptive override for real-world OpenClaw payload layout")
	}
	if overridden[0].Arguments != currentToolCalls[0].Arguments {
		t.Fatalf("arguments changed unexpectedly: got %q want %q", overridden[0].Arguments, currentToolCalls[0].Arguments)
	}
}

func TestShouldRepairSuccessfulStructuredWorkspaceArtifactWrite_NormalizesMultilineWriteArgs(t *testing.T) {
	userMessage := "Read `openclaw_report.pdf`, answer the numbered questions, and write them one per line to `answer.txt`.\n\n1. How many community-built skills were in the public registry before filtering?\n2. How many skills remained after filtering?\n3. What is the largest skill category by count?\n4. What is the second-largest skill category by count?\n5. What is the name of the file that defines an OpenClaw skill?\n6. What type of API does the OpenClaw gateway expose?\n7. What date was the skills registry data collected?\n8. How many new benchmark tasks does the paper propose?"
	currentToolCalls := []llm.ToolCall{
		{ID: "call-pdf", Name: "pdf"},
		{
			ID:   "call-write",
			Name: "write",
			Arguments: `{"path":"answer.txt","content":"5705
2999
Development & Coding: 492
Communication & Messaging: 318
SKILL.md
typed WebSocket API
February 7, 2026
8
"}`,
		},
	}
	currentToolResults := []llm.Message{
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-pdf",
			Content:    `{"document":{"path":"openclaw_report.pdf"},"raw_text":"The public registry had 5,705 community-built skills and 2,999 remained after filtering. AI & LLMs 287. Search & Research 253. Each skill uses SKILL.md. The gateway exposes a typed WebSocket API. The data was collected on February 7, 2026. The paper proposes 6 benchmark tasks."}`,
		},
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-write",
			Content:    `{"path":"answer.txt","success":true}`,
		},
	}

	if !hasSatisfiedRequestedArtifactWrite(userMessage, currentToolCalls, currentToolResults) {
		t.Fatal("expected multiline write args to be normalized and accepted when the answer-line shape is valid")
	}
	if shouldRepairSuccessfulStructuredWorkspaceArtifactWrite(userMessage, currentToolCalls, currentToolResults, nil, nil) {
		t.Fatal("expected no repair when generic validation is already satisfied")
	}
}

func TestShouldUseImmediateWorkspaceArtifactOrchestration_SkipsAfterSuccessfulWrite(t *testing.T) {
	userMessage := "Read `openclaw_report.pdf`, answer the numbered questions, and write them one per line to `answer.txt`."
	currentToolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "file_write"},
	}
	currentToolResults := []llm.Message{
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-1",
			Content:    `{"path":"answer.txt","success":true}`,
		},
	}

	if shouldUseImmediateWorkspaceArtifactOrchestration(userMessage, currentToolCalls, currentToolResults, currentToolCalls, currentToolResults) {
		t.Fatal("expected immediate orchestration to skip rounds that already completed the requested write")
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

	if shouldUseLLMWorkspaceArtifactOrchestration(
		"You have access to a collection of emails in the emails/ folder in your workspace. Save the summary to alpha_summary.md with the following sections:\n1. **Project Overview**: What is Project Alpha?\n2. **Timeline**: Original timeline and any changes\n3. **Key Risks and Issues**: Budget concerns and security findings\n4. **Client/Business Impact**: Pipeline and revenue projections\n5. **Current Status**: Latest status",
		toolCalls,
		toolResults,
	) {
		t.Fatal("expected no special repair path for already-written non-empty artifacts")
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
		"Follow the user's requested structure and formatting.",
		"Use only the recovered evidence.",
		"Preserve concrete names, numbers, dates, filenames, API labels, and other source wording",
		"If the evidence contains updates or conflicting statements, make that clear instead of silently flattening them.",
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("expected prompt to contain %q, got=%q", needle, body)
		}
	}
	if !strings.Contains(msgs[0].Content, "Write the final artifact directly from the recovered evidence and the user's request.") {
		t.Fatalf("expected system prompt to reinforce direct evidence-driven writing, got=%q", msgs[0].Content)
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

func TestCollectWorkspaceArtifactEvidence_IncludesExecFileReadStdout(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "exec", Arguments: `{"command":"cat ai_blog.txt"}`},
	}
	toolResults := []llm.Message{
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-1",
			Content:    `{"status":"completed","exit_code":0,"stdout":"# Humanized Draft\n\nThis version keeps the same advice but sounds more natural."}`,
		},
	}

	evidence := collectWorkspaceArtifactEvidence(toolCalls, toolResults)
	if len(evidence) == 0 {
		t.Fatal("expected exec file-read evidence to be collected")
	}

	joined := strings.Join(evidence, "\n\n")
	for _, needle := range []string{
		"READ | TEXT | ai_blog.txt",
		"# Humanized Draft",
		"sounds more natural",
	} {
		if !strings.Contains(joined, needle) {
			t.Fatalf("expected exec evidence to contain %q, got=%q", needle, joined)
		}
	}
}

func TestCollectWorkspaceArtifactEvidence_IgnoresExecNonReadCommands(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{ID: "call-ls", Name: "exec", Arguments: `{"command":"ls ai_blog.txt"}`},
		{ID: "call-wc", Name: "exec", Arguments: `{"command":"wc -c ai_blog.txt"}`},
	}
	toolResults := []llm.Message{
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-ls",
			Content:    `{"status":"completed","exit_code":0,"stdout":"ai_blog.txt\n"}`,
		},
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-wc",
			Content:    `{"status":"completed","exit_code":0,"stdout":"4559 ai_blog.txt\n"}`,
		},
	}

	evidence := collectWorkspaceArtifactEvidence(toolCalls, toolResults)
	if len(evidence) != 0 {
		t.Fatalf("expected non-read exec commands to be ignored, got=%q", strings.Join(evidence, "\n\n"))
	}
}

func TestShouldUseImmediateWorkspaceArtifactOrchestration_UsesExecFileReadEvidenceForHumanizerTask(t *testing.T) {
	currentToolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "exec", Arguments: `{"command":"cat ai_blog.txt"}`},
	}
	currentToolResults := []llm.Message{
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-1",
			Content:    fmt.Sprintf(`{"status":"completed","exit_code":0,"stdout":%q}`, humanizerSourceEvidence()[0]),
		},
	}

	if !shouldUseImmediateWorkspaceArtifactOrchestration(
		humanizerTaskPrompt(),
		currentToolCalls,
		currentToolResults,
		currentToolCalls,
		currentToolResults,
	) {
		t.Fatal("expected exec-based file-read evidence to trigger immediate orchestration for humanizer task")
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
	if !strings.Contains(joined, "READ | RAW | openclaw_report.pdf | pages=1,2") {
		t.Fatalf("expected raw pdf header in evidence, got=%q", joined)
	}
	if !strings.Contains(joined, "AI & LLMs    287") {
		t.Fatalf("expected raw pdf layout-preserving text in evidence, got=%q", joined)
	}
	if !strings.Contains(joined, "READ | RAW | openclaw_report.pdf | page=1") {
		t.Fatalf("expected per-page raw pdf evidence block, got=%q", joined)
	}
	if !strings.Contains(joined, "READ | RAW | openclaw_report.pdf | page=2") {
		t.Fatalf("expected second per-page raw pdf evidence block, got=%q", joined)
	}
}

func TestCollectWorkspaceArtifactEvidence_PrefersStructuredPDFMarkdownAndLayout(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{ID: "call-1", Name: "pdf"},
	}
	toolResults := []llm.Message{
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-1",
			Content:    `{"document":{"path":"openclaw_report.pdf"},"outline":[{"title":"Executive Summary","level":1,"page_number":1}],"pages":[{"number":1,"markdown":"# Executive Summary\n\n- typed WebSocket API","blocks":[{"kind":"heading","heading_level":1,"markdown":"# Executive Summary"},{"kind":"list","markdown":"- typed WebSocket API"}]},{"number":2,"tables":[{"markdown":"| Name | Count |\n| --- | --- |\n| AI & LLMs | 287 |"}],"text":"flattened fallback"}]}`,
		},
	}

	evidence := collectWorkspaceArtifactEvidence(toolCalls, toolResults)
	if len(evidence) == 0 {
		t.Fatal("expected evidence to be collected")
	}
	joined := strings.Join(evidence, "\n\n")
	for _, needle := range []string{
		"PDF | MARKDOWN | openclaw_report.pdf | page=1",
		"# Executive Summary",
		"typed WebSocket API",
		"PDF | STRUCTURED | openclaw_report.pdf | page=2",
		"| Name | Count |",
	} {
		if !strings.Contains(joined, needle) {
			t.Fatalf("expected structured pdf evidence to contain %q, got=%q", needle, joined)
		}
	}
	if strings.Contains(joined, "flattened fallback") {
		t.Fatalf("expected structured page evidence to outrank flattened fallback, got=%q", joined)
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
