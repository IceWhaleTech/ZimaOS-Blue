package server

import (
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

func TestShouldAutoContinueForTodo(t *testing.T) {
	t.Run("skips auto continue when waiting for user question", func(t *testing.T) {
		current := "- [ ] 确认你要查询的城市/地区\n- [ ] 获取该地区实时天气\n\n你要查哪个城市？"
		if shouldAutoContinueForTodo(current, "") {
			t.Fatalf("expected auto-continue to be disabled while awaiting user input")
		}
	})

	t.Run("skips auto continue for chinese clarification prompt without question mark", func(t *testing.T) {
		current := "- [ ] confirm city\n- [ ] fetch weather\n\n你要查哪个城市（例如：北京、上海、深圳）"
		if shouldAutoContinueForTodo(current, "") {
			t.Fatalf("expected auto-continue to be disabled for clarification prompt")
		}
	})

	t.Run("continues when todo exists and no user input is needed", func(t *testing.T) {
		current := "- [ ] 调用 web_search 查询实时天气\n- [ ] 汇总结果并回答用户"
		if !shouldAutoContinueForTodo(current, "") {
			t.Fatalf("expected auto-continue for pending todo without clarification request")
		}
	})

	t.Run("continues from tracked todo when current content is empty", func(t *testing.T) {
		tracked := "- [x] step1\n- [ ] step2"
		if !shouldAutoContinueForTodo("", tracked) {
			t.Fatalf("expected auto-continue from tracked pending todo")
		}
	})

	t.Run("stops when current response is non-empty summary even if tracked todo still pending", func(t *testing.T) {
		current := "已完成执行并给出最终总结。"
		tracked := "- [x] step1\n- [ ] step2"
		if shouldAutoContinueForTodo(current, tracked) {
			t.Fatalf("expected no auto-continue when current response is non-empty and has no pending todo")
		}
	})

	t.Run("continues with star checklist and uppercase X", func(t *testing.T) {
		tracked := "* [X] step1\n* [ ] step2"
		if !shouldAutoContinueForTodo("", tracked) {
			t.Fatalf("expected auto-continue for star checklist with uppercase X")
		}
	})
}

func TestShouldAutoContinueForActionPledge(t *testing.T) {
	t.Run("continues for chinese action pledge", func(t *testing.T) {
		current := "我先给你结论：我这边需要联网检索一下最新动态。我现在就去查，稍等我几秒。"
		if !shouldAutoContinueForActionPledge(current) {
			t.Fatalf("expected auto-continue for chinese action pledge")
		}
	})

	t.Run("continues for english action pledge", func(t *testing.T) {
		current := "I need to verify this online. Let me check and I'll get back in a few seconds."
		if !shouldAutoContinueForActionPledge(current) {
			t.Fatalf("expected auto-continue for english action pledge")
		}
	})

	t.Run("does not continue for clarification question", func(t *testing.T) {
		current := "可以先告诉我你要看的时间范围吗？"
		if shouldAutoContinueForActionPledge(current) {
			t.Fatalf("expected no auto-continue when awaiting user input")
		}
	})
}

func TestShouldAutoContinueAfterToollessReply(t *testing.T) {
	t.Run("agent mode continues on pending todo", func(t *testing.T) {
		current := "- [ ] 查询最新新闻\n- [ ] 汇总回答"
		ok, reason := shouldAutoContinueAfterToollessReply(current, "", true)
		if !ok || reason != "pending_todo" {
			t.Fatalf("expected pending_todo auto-continue, got ok=%v reason=%q", ok, reason)
		}
	})

	t.Run("non-agent continues on action pledge", func(t *testing.T) {
		current := "我现在就去查，稍等我几秒。"
		ok, reason := shouldAutoContinueAfterToollessReply(current, "", false)
		if !ok || reason != "action_pledge" {
			t.Fatalf("expected action_pledge auto-continue, got ok=%v reason=%q", ok, reason)
		}
	})

	t.Run("continues on pseudo tool-call text", func(t *testing.T) {
		current := "{\"cmd\":\"ls -la\"}I’ll inspect now.{\"tool\":\"exec\",\"cmd\":\"ls -la\"}exec: ls -la\n```tool\n{\"name\":\"exec\",\"arguments\":{\"cmd\":\"ls -la\"}}\n```\n<exec>{\"cmd\":\"ls -la\"}</exec>"
		ok, reason := shouldAutoContinueAfterToollessReply(current, "", false)
		if !ok || reason != "pseudo_tool_call" {
			t.Fatalf("expected pseudo_tool_call auto-continue, got ok=%v reason=%q", ok, reason)
		}
	})

	t.Run("continues on pseudo tool-call command/workdir json text", func(t *testing.T) {
		current := "收到，开始设置提醒。\nWorking on task: add reminder for 10 seconds later.{\"command\":\"blue reminder.add message=\\\"喝水\\\" time=10s\",\"workdir\":\"/tmp/workspace\"}{\"command\":\"blue help reminder\",...}"
		ok, reason := shouldAutoContinueAfterToollessReply(current, "", false)
		if !ok || reason != "pseudo_tool_call" {
			t.Fatalf("expected pseudo_tool_call auto-continue, got ok=%v reason=%q", ok, reason)
		}
	})

	t.Run("continues on pseudo tool-call single placeholder command json text", func(t *testing.T) {
		current := "我来给你设一个 10 秒后的提醒。{\"command\":\"...\"}`"
		ok, reason := shouldAutoContinueAfterToollessReply(current, "", false)
		if !ok || reason != "pseudo_tool_call" {
			t.Fatalf("expected pseudo_tool_call auto-continue, got ok=%v reason=%q", ok, reason)
		}
	})

	t.Run("does not treat a single cmd json example as pseudo tool-call", func(t *testing.T) {
		current := "你可以在 shell 里执行这个 JSON 示例：{\"cmd\":\"ls -la\"}"
		ok, reason := shouldAutoContinueAfterToollessReply(current, "", false)
		if ok {
			t.Fatalf("expected no auto-continue for benign single cmd json, got reason=%q", reason)
		}
	})
}

func TestBuildAutoContinueNudges(t *testing.T) {
	if got := buildToollessAutoContinueNudge(false); got == "" {
		t.Fatalf("expected non-agent toolless nudge to be non-empty")
	}
	if got := buildToollessAutoContinueNudge(false); !strings.Contains(got, "Tool guidance constraints") || !strings.Contains(got, "structured tool_calls only") {
		t.Fatalf("expected non-agent toolless nudge to include tool guidance constraints, got=%q", got)
	}

	agentToolless := buildToollessAutoContinueNudge(true)
	if !strings.Contains(agentToolless, "continuous improvement loop") || !strings.Contains(agentToolless, "next concrete improvement") {
		t.Fatalf("expected agent toolless nudge to include loop guidance, got=%q", agentToolless)
	}
	if !strings.Contains(agentToolless, "existing canonical TODO checklist") || !strings.Contains(agentToolless, "avoid rewriting the full checklist") {
		t.Fatalf("expected agent toolless nudge to enforce stable TODO checklist updates, got=%q", agentToolless)
	}
	if !strings.Contains(agentToolless, "Tool guidance constraints") {
		t.Fatalf("expected agent toolless nudge to include tool guidance constraints, got=%q", agentToolless)
	}

	agentPostTool := buildPostToolAutoContinueNudge(true)
	if !strings.Contains(agentPostTool, "agent loop") || !strings.Contains(agentPostTool, "next concrete improvement") {
		t.Fatalf("expected agent post-tool nudge to include loop guidance, got=%q", agentPostTool)
	}
	if !strings.Contains(agentPostTool, "existing canonical TODO checklist") || !strings.Contains(agentPostTool, "mark completed items") {
		t.Fatalf("expected agent post-tool nudge to enforce TODO status updates first, got=%q", agentPostTool)
	}

	pseudo := buildToollessAutoContinueNudgeForReason(false, "pseudo_tool_call")
	if !strings.Contains(pseudo, "fake tool-call text") {
		t.Fatalf("expected pseudo-tool nudge to mention fake tool-call text, got=%q", pseudo)
	}
	if !strings.Contains(pseudo, "Tool guidance constraints") {
		t.Fatalf("expected pseudo-tool nudge to include tool guidance constraints, got=%q", pseudo)
	}
	pseudoAgent := buildToollessAutoContinueNudgeForReason(true, "pseudo_tool_call")
	if !strings.Contains(pseudoAgent, "continuous improvement loop") {
		t.Fatalf("expected agent pseudo-tool nudge to include loop guidance, got=%q", pseudoAgent)
	}
	if !strings.Contains(pseudoAgent, "existing canonical TODO checklist") {
		t.Fatalf("expected agent pseudo-tool nudge to include canonical TODO continuity guidance, got=%q", pseudoAgent)
	}
	if !strings.Contains(pseudoAgent, "Tool guidance constraints") {
		t.Fatalf("expected agent pseudo-tool nudge to include tool guidance constraints, got=%q", pseudoAgent)
	}

	if shouldPersistToollessRoundContent("pseudo_tool_call") {
		t.Fatal("expected pseudo_tool_call rounds not to be persisted")
	}
	if !shouldPersistToollessRoundContent("action_pledge") {
		t.Fatal("expected action_pledge rounds to be persisted")
	}

	assistantPseudo := buildToollessAutoContinueAssistantContent("{\"cmd\":\"ls -la\"}", "pseudo_tool_call")
	if strings.Contains(assistantPseudo, "{\"cmd\"") {
		t.Fatalf("expected pseudo assistant follow-up content to discard raw fake tool text, got=%q", assistantPseudo)
	}
	assistantNormal := buildToollessAutoContinueAssistantContent("normal content", "action_pledge")
	if assistantNormal != "normal content" {
		t.Fatalf("expected normal assistant follow-up content to pass through, got=%q", assistantNormal)
	}
}

func TestGetMaxAutoContinueForMode(t *testing.T) {
	h := &ChatHandler{}
	if got := h.getMaxAutoContinueForMode(false); got != maxAutoContinueDefault {
		t.Fatalf("non-agent max auto-continue = %d, want %d", got, maxAutoContinueDefault)
	}
	if got := h.getMaxAutoContinueForMode(true); got != maxAutoContinueAgent {
		t.Fatalf("agent max auto-continue = %d, want %d", got, maxAutoContinueAgent)
	}
}

func TestPseudoToolCallAutoContinueBudget(t *testing.T) {
	if !shouldAutoContinueForReasonWithinBudget("pending_todo", true, 999) {
		t.Fatal("expected non-pseudo reasons to bypass pseudo budget")
	}

	if !shouldAutoContinueForReasonWithinBudget("pseudo_tool_call", false, maxPseudoToolCallAutoContinueDefault-1) {
		t.Fatal("expected non-agent pseudo_tool_call within budget to continue")
	}
	if shouldAutoContinueForReasonWithinBudget("pseudo_tool_call", false, maxPseudoToolCallAutoContinueDefault) {
		t.Fatal("expected non-agent pseudo_tool_call at budget limit to stop")
	}

	if !shouldAutoContinueForReasonWithinBudget("pseudo_tool_call", true, maxPseudoToolCallAutoContinueAgent-1) {
		t.Fatal("expected agent pseudo_tool_call within budget to continue")
	}
	if shouldAutoContinueForReasonWithinBudget("pseudo_tool_call", true, maxPseudoToolCallAutoContinueAgent) {
		t.Fatal("expected agent pseudo_tool_call at budget limit to stop")
	}
}

func TestPseudoToolCallModelSwitchThreshold(t *testing.T) {
	if shouldSwitchModelAfterPseudoToolCall(pseudoToolCallModelSwitchThreshold - 1) {
		t.Fatal("expected below-threshold pseudo count not to switch model")
	}
	if !shouldSwitchModelAfterPseudoToolCall(pseudoToolCallModelSwitchThreshold) {
		t.Fatal("expected threshold pseudo count to switch model")
	}
}

func TestSelectPseudoToolCallFallbackModel(t *testing.T) {
	if got := selectPseudoToolCallFallbackModel("auto", nil); got != "" {
		t.Fatalf("expected auto model not to fallback, got %q", got)
	}

	if got := selectPseudoToolCallFallbackModel("gpt-5.3-codex-spark", nil); got != "gpt-5.3-codex" {
		t.Fatalf("expected spark fallback model, got %q", got)
	}

	models := []string{"gpt-5.3-codex-spark", "gpt-5.3-codex"}
	if got := selectPseudoToolCallFallbackModel("gpt-5.3-codex-spark", models); got != "gpt-5.3-codex" {
		t.Fatalf("expected fallback to available base model, got %q", got)
	}

	models = []string{"gpt-5.3-codex-spark"}
	if got := selectPseudoToolCallFallbackModel("gpt-5.3-codex-spark", models); got != "" {
		t.Fatalf("expected no fallback when candidate not in available model list, got %q", got)
	}

	models = []string{"qwen2.5-coder-32b", "qwen2.5-coder-14b", "claude-3-5-sonnet"}
	if got := selectPseudoToolCallFallbackModel("qwen2.5-coder-32b-preview", models); got != "qwen2.5-coder-32b" {
		t.Fatalf("expected closest-family fallback model, got %q", got)
	}

	if got := selectPseudoToolCallFallbackModel("codex", nil); got != "" {
		t.Fatalf("expected empty fallback for model without suffix segment, got %q", got)
	}
}

func TestChoosePseudoToolCallPrimaryToolIndex(t *testing.T) {
	tools := []llm.Tool{
		{Name: "ask"},
		{Name: "web_search"},
		{Name: "exec"},
	}
	if got := choosePseudoToolCallPrimaryToolIndex(tools); got != 2 {
		t.Fatalf("expected exec to be preferred, got index=%d", got)
	}

	tools = []llm.Tool{
		{Name: "ask"},
		{Name: "file_read"},
	}
	if got := choosePseudoToolCallPrimaryToolIndex(tools); got != 1 {
		t.Fatalf("expected non-ask tool fallback, got index=%d", got)
	}
}

func TestHardenPseudoToolCallRetryRequest(t *testing.T) {
	req := llm.ChatRequest{
		Model:              "gpt-5.3-codex-spark",
		Temperature:        0.7,
		PreviousResponseID: "resp_abc",
		Tools: []llm.Tool{
			{
				Name:       "web_search",
				Parameters: map[string]interface{}{"type": "object"},
			},
			{
				Name: "exec",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"command": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
	}

	actions := hardenPseudoToolCallRetryRequest(&req)
	if len(actions) == 0 {
		t.Fatal("expected hardening actions to be applied")
	}
	if req.Temperature != 0.2 {
		t.Fatalf("expected temperature to be hardened to 0.2, got %v", req.Temperature)
	}
	if req.PreviousResponseID != "" {
		t.Fatalf("expected previous_response_id to be cleared, got %q", req.PreviousResponseID)
	}
	if len(req.Tools) != 1 || req.Tools[0].Name != "exec" {
		t.Fatalf("expected tool scope to reduce to exec, got %+v", req.Tools)
	}
	if got, ok := req.Tools[0].Parameters["additionalProperties"].(bool); !ok || got {
		t.Fatalf("expected schema additionalProperties=false, got %v", req.Tools[0].Parameters["additionalProperties"])
	}
}
