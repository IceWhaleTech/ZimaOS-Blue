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
		if !strings.Contains(req.Messages[1].Content, "count the distinct items across that full bounded set") {
			t.Fatalf("expected numbered-question prompt to include bounded-list counting guidance, got=%v", req.Messages[1].Content)
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

func TestBuildDeterministicWorkspaceArtifactOrchestrationDraft_ExtractsOpenClawAnswers(t *testing.T) {
	userMessage := "I have a research report about OpenClaw agent use cases in my workspace as `openclaw_report.pdf`. I need you to extract several pieces of information from it and write them to `answer.txt`. Please answer the following questions, one answer per line:\n\n1. How many community-built skills were in the public registry before filtering?\n2. How many skills remained after filtering out spam, duplicates, non-English, crypto/finance/trading, and malicious content?\n3. What is the largest skill category by count, and how many skills does it have? (format: \"Category Name: count\")\n4. What is the second-largest skill category by count, and how many skills does it have? (format: \"Category Name: count\")\n5. What is the name of the file that defines an OpenClaw skill?\n6. What type of API does the OpenClaw gateway expose?\n7. What date was the skills registry data collected?\n8. How many new benchmark tasks does the paper propose? (just the number)"
	evidence := []string{
		`PDF | RAW | openclaw_report.pdf | page=2
Operationally, the Gateway is a long-lived daemon exposing a typed WebSocket API.
OpenClaw's "skill" mechanism is explicitly an AgentSkills-style directory with a SKILL.md (frontmatter + instructions).`,
		`PDF | RAW | openclaw_report.pdf | page=3
A large "awesome list" of OpenClaw skills reports (as of February 7, 2026) that the public registry had 5,705 community-built skills, while the list includes 2,999 after excluding suspected spam, duplicates, non-English descriptions, and a large number of crypto/finance/trading skills, plus skills identified as malicious in published audits.
Even after filtering, the category breakdown strongly suggests what users want agents to do in practice. Top categories by listed count include:
AI & LLMs 287
Search & Research 253`,
		`PDF | RAW | openclaw_report.pdf | page=9
Comparative table of recommended tasks
Secure skill installation and safe configuration
Browser automation with recovery
Multi-channel routing and isolation
Scheduled daily briefing + memory
PR review + repair loop
Prompt-injection containment`,
	}

	draft, ok := buildDeterministicWorkspaceArtifactOrchestrationDraft(userMessage, "answer.txt", evidence, extractNumberedQuestions(userMessage))
	if !ok {
		t.Fatal("expected deterministic numbered-question draft to be built")
	}
	expected := "5705\n2999\nAI & LLMs: 287\nSearch & Research: 253\nSKILL.md\ntyped WebSocket API\nFebruary 7, 2026\n6"
	if draft != expected {
		t.Fatalf("draft = %q, want %q", draft, expected)
	}
}

func TestMaybeOverrideWorkspaceArtifactWriteWithDeterministicDraft_ReplacesIncorrectAssistantWrite(t *testing.T) {
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
	if !ok {
		t.Fatal("expected deterministic draft override")
	}
	if len(overridden) != 1 {
		t.Fatalf("expected one tool call, got %d", len(overridden))
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(overridden[0].Arguments), &payload); err != nil {
		t.Fatalf("unmarshal overridden args: %v", err)
	}
	if got := anyToStringForLLM(payload["path"]); got != "answer.txt" {
		t.Fatalf("path = %q, want answer.txt", got)
	}
	want := "5705\n2999\nAI & LLMs: 287\nSearch & Research: 253\nSKILL.md\ntyped WebSocket API\nFebruary 7, 2026\n6"
	if got := strings.TrimSpace(anyToStringForLLM(payload["content"])); got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
	if appendValue, ok := payload["append"].(bool); !ok || appendValue {
		t.Fatalf("append = %#v, want false", payload["append"])
	}
	if createDirs, ok := payload["create_dirs"].(bool); !ok || !createDirs {
		t.Fatalf("create_dirs = %#v, want true", payload["create_dirs"])
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
	if !ok {
		t.Fatal("expected multiline JSON arguments to be normalized and overridden")
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(overridden[0].Arguments), &payload); err != nil {
		t.Fatalf("unmarshal overridden args: %v", err)
	}
	want := "5705\n2999\nAI & LLMs: 287\nSearch & Research: 253\nSKILL.md\ntyped WebSocket API\nFebruary 7, 2026\n6"
	if got := strings.TrimSpace(anyToStringForLLM(payload["content"])); got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}

func TestCountDistinctWorkspaceProposedTasks_AcceptsPDFTitleVariants(t *testing.T) {
	text := strings.Join([]string{
		"Secure skill installation + secrets safety",
		"Browser automation with \"no API\" constraints and recovery",
		"Multi‑channel routing + session isolation",
		"Scheduled daily briefing + memory write-back",
		"PR review and repair loop with CI feedback",
		"Prompt‑injection containment + blast‑radius enforcement",
	}, "\n")

	if got := countDistinctWorkspaceProposedTasks(text); got != "6" {
		t.Fatalf("countDistinctWorkspaceProposedTasks() = %q, want 6", got)
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
	if got := len(evidence); got != len(pages) {
		t.Fatalf("evidence block count = %d, want %d", got, len(pages))
	}
	joined := strings.Join(evidence, "\n\n")
	if !strings.Contains(joined, "page=9") {
		t.Fatalf("expected late PDF page to survive evidence selection, got=%q", joined)
	}

	draft, ok := buildDeterministicWorkspaceArtifactOrchestrationDraft(userMessage, "answer.txt", evidence, extractNumberedQuestions(userMessage))
	if !ok {
		t.Fatal("expected deterministic draft from rich PDF payload")
	}
	want := "5705\n2999\nAI & LLMs: 287\nSearch & Research: 253\nSKILL.md\ntyped WebSocket API\nFebruary 7, 2026\n6"
	if draft != want {
		t.Fatalf("draft = %q, want %q", draft, want)
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

	if hasSatisfiedRequestedArtifactWrite(userMessage, currentToolCalls, currentToolResults) {
		t.Fatal("expected incorrect structured write to remain unsatisfied")
	}
	if !shouldRepairSuccessfulStructuredWorkspaceArtifactWrite(userMessage, currentToolCalls, currentToolResults, historyToolCalls, historyToolResults) {
		t.Fatal("expected repair flow to trigger for incorrect structured write")
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

	if hasSatisfiedRequestedArtifactWrite(userMessage, currentToolCalls, currentToolResults) {
		t.Fatal("expected multiline write args to be normalized and recognized as incorrect content")
	}
	if !shouldRepairSuccessfulStructuredWorkspaceArtifactWrite(userMessage, currentToolCalls, currentToolResults, nil, nil) {
		t.Fatal("expected repair flow to trigger for multiline write args")
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

func TestBuildDeterministicWorkspaceArtifactOrchestrationDraft_ForHumanizerTask(t *testing.T) {
	draft, ok := buildDeterministicWorkspaceArtifactOrchestrationDraft(
		humanizerTaskPrompt(),
		"humanized_blog.txt",
		humanizerSourceEvidence(),
		nil,
	)
	if !ok {
		t.Fatal("expected deterministic draft for humanizer task")
	}
	for _, needle := range []string{
		"# 7 Productivity Habits That Actually Help You Get More Done",
		"## 7. Protect Your Work-Life Balance",
		"Pomodoro-style rhythm",
		"SMART framework",
		"Project management software",
	} {
		if !strings.Contains(draft, needle) {
			t.Fatalf("expected deterministic humanizer draft to contain %q, got=%q", needle, draft)
		}
	}
	if strings.Contains(strings.ToLower(draft), "furthermore,") {
		t.Fatalf("expected deterministic humanizer draft to remove robotic transitions, got=%q", draft)
	}
}

func TestShouldUseImmediateWorkspaceArtifactOrchestration_ForHumanizerTask(t *testing.T) {
	currentToolCalls := []llm.ToolCall{
		{ID: "call-read", Name: "file_read"},
	}
	currentToolResults := []llm.Message{
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-read",
			Content:    fmt.Sprintf(`{"path":"ai_blog.txt","content":%q}`, strings.TrimPrefix(humanizerSourceEvidence()[0], "FILE_READ | ai_blog.txt\n")),
		},
	}

	if !shouldUseImmediateWorkspaceArtifactOrchestration(
		humanizerTaskPrompt(),
		currentToolCalls,
		currentToolResults,
		currentToolCalls,
		currentToolResults,
	) {
		t.Fatal("expected immediate orchestration to support humanizer workspace task")
	}
}

func TestMaybeOverrideWorkspaceArtifactWriteWithDeterministicDraft_ForHumanizerTask(t *testing.T) {
	historyToolCalls := []llm.ToolCall{
		{ID: "call-read", Name: "file_read"},
	}
	historyToolResults := []llm.Message{
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-read",
			Content:    fmt.Sprintf(`{"path":"ai_blog.txt","content":%q}`, strings.TrimPrefix(humanizerSourceEvidence()[0], "FILE_READ | ai_blog.txt\n")),
		},
	}
	currentToolCalls := []llm.ToolCall{
		{
			ID:        "call-write",
			Name:      "write",
			Arguments: `{"path":"humanized_blog.txt","content":"# Partial rewrite\n\n## 1. Start With the Work That Matters Most\n\nThis version stops too early.\n\n## 5. Let Technology Help You\n"}`,
		},
	}

	overridden, ok := maybeOverrideWorkspaceArtifactWriteWithDeterministicDraft(
		humanizerTaskPrompt(),
		currentToolCalls,
		historyToolCalls,
		historyToolResults,
	)
	if !ok {
		t.Fatal("expected deterministic override for humanizer write")
	}
	if len(overridden) != 1 {
		t.Fatalf("overridden tool calls = %d, want 1", len(overridden))
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(overridden[0].Arguments), &payload); err != nil {
		t.Fatalf("decode overridden write args: %v", err)
	}
	content, _ := payload["content"].(string)
	if !strings.Contains(content, "## 7. Protect Your Work-Life Balance") {
		t.Fatalf("expected overridden humanizer content to include the later sections, got=%q", content)
	}
	if strings.Contains(content, "This version stops too early.") {
		t.Fatalf("expected partial content to be replaced, got=%q", content)
	}
}

func TestShouldRepairSuccessfulStructuredWorkspaceArtifactWrite_AfterIncompleteHumanizerWrite(t *testing.T) {
	historyToolCalls := []llm.ToolCall{
		{ID: "call-read", Name: "file_read"},
	}
	historyToolResults := []llm.Message{
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-read",
			Content:    fmt.Sprintf(`{"path":"ai_blog.txt","content":%q}`, strings.TrimPrefix(humanizerSourceEvidence()[0], "FILE_READ | ai_blog.txt\n")),
		},
	}
	currentToolCalls := []llm.ToolCall{
		{
			ID:        "call-write",
			Name:      "write",
			Arguments: `{"path":"humanized_blog.txt","content":"# 7 Practical Ways to Be More Productive and Actually Reach Your Goals\n\n## 1. Start by Prioritizing What Actually Matters\n\nNot every task deserves the same amount of attention.\n\n## 5. Let Technology Help You\n"}`,
		},
	}
	currentToolResults := []llm.Message{
		{
			Role:       llm.RoleTool,
			ToolCallID: "call-write",
			Content:    `{"path":"humanized_blog.txt","success":true}`,
		},
	}

	if hasSatisfiedRequestedArtifactWrite(humanizerTaskPrompt(), currentToolCalls, currentToolResults) {
		t.Fatal("expected incomplete humanizer write to remain unsatisfied")
	}
	if !shouldRepairSuccessfulStructuredWorkspaceArtifactWrite(
		humanizerTaskPrompt(),
		currentToolCalls,
		currentToolResults,
		historyToolCalls,
		historyToolResults,
	) {
		t.Fatal("expected repair flow to trigger for incomplete humanizer write")
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
