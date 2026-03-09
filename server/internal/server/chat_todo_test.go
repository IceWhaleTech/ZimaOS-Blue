package server

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
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

	t.Run("continues from tracked todo when current response is progress text", func(t *testing.T) {
		current := "我会继续执行下一步并补充验证。"
		tracked := "- [x] step1\n- [ ] step2"
		if !shouldAutoContinueForTodo(current, tracked) {
			t.Fatalf("expected auto-continue when tracked todo has pending items and current text is non-completion progress")
		}
	})

	t.Run("stops when plan completion is known even if tracked todo still pending", func(t *testing.T) {
		current := "继续执行中。"
		tracked := "- [x] step1\n- [ ] step2"
		if shouldAutoContinueForTodo(current, tracked, true) {
			t.Fatalf("expected no auto-continue when plan completion is known")
		}
	})

	t.Run("continues with star checklist and uppercase X", func(t *testing.T) {
		tracked := "* [X] step1\n* [ ] step2"
		if !shouldAutoContinueForTodo("", tracked) {
			t.Fatalf("expected auto-continue for star checklist with uppercase X")
		}
	})

	t.Run("stops when reply includes stale checklist but explicit completion deliverable", func(t *testing.T) {
		current := "- [ ] 实现2048网页游戏\n- [ ] 本地运行并给出localhost地址\n已完成，游戏可直接运行，地址：http://localhost:3000"
		if shouldAutoContinueForTodo(current, "") {
			t.Fatalf("expected no auto-continue when completion deliverable is explicit")
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

	t.Run("continues for chinese quick-check plus wait phrasing", func(t *testing.T) {
		current := "我先帮你快速查一下 BlueAgent 的最新相关新闻与动态。请稍等，我整理成要点给你。"
		if !shouldAutoContinueForActionPledge(current) {
			t.Fatalf("expected auto-continue for chinese quick-check wait phrasing")
		}
	})

	t.Run("continues for english action pledge", func(t *testing.T) {
		current := "I need to verify this online. Let me check and I'll get back in a few seconds."
		if !shouldAutoContinueForActionPledge(current) {
			t.Fatalf("expected auto-continue for english action pledge")
		}
	})

	t.Run("continues for spanish action pledge", func(t *testing.T) {
		current := "Necesito verificarlo en línea. Voy a revisar y te respondo enseguida."
		if !shouldAutoContinueForActionPledge(current) {
			t.Fatalf("expected auto-continue for spanish action pledge")
		}
	})

	t.Run("continues for japanese action pledge", func(t *testing.T) {
		current := "最新情報を確認します。少しお待ちください。"
		if !shouldAutoContinueForActionPledge(current) {
			t.Fatalf("expected auto-continue for japanese action pledge")
		}
	})

	t.Run("continues for soft-consent continuation offer", func(t *testing.T) {
		current := "为了不给你错误信息，我建议按高可信来源整理。如果你同意，我下一步会按这个范围整理（1-2分钟）：\n1) 官方渠道\n2) 主流媒体"
		if !shouldAutoContinueForActionPledge(current) {
			t.Fatalf("expected auto-continue for soft-consent continuation offer")
		}
	})

	t.Run("continues for french soft-consent continuation offer", func(t *testing.T) {
		current := "Pour éviter les erreurs, je propose de me limiter aux sources fiables. Si vous êtes d'accord, je vais résumer cela en 1-2 minutes."
		if !shouldAutoContinueForActionPledge(current) {
			t.Fatalf("expected auto-continue for french soft-consent continuation offer")
		}
	})

	t.Run("does not continue for clarification question", func(t *testing.T) {
		current := "可以先告诉我你要看的时间范围吗？"
		if shouldAutoContinueForActionPledge(current) {
			t.Fatalf("expected no auto-continue when awaiting user input")
		}
	})

	t.Run("does not continue for completed-result statement", func(t *testing.T) {
		current := "我已经帮你查好了，下面是结果要点。"
		if shouldAutoContinueForActionPledge(current) {
			t.Fatalf("expected no auto-continue for completed-result statement")
		}
	})
}

func TestShouldPreferDeepSearchReport(t *testing.T) {
	positive := []string{
		"帮我做一个 blueagent 最新新闻的深度搜索",
		"Please do deep research on BlueAgent latest updates with sources",
		"给我一份 BlueAgent 更新报告并附引用来源",
	}
	for _, msg := range positive {
		if !shouldPreferDeepSearchReport(msg) {
			t.Fatalf("expected deep-search preference for %q", msg)
		}
	}

	negative := []string{
		"hello",
		"写一行 golang 代码",
		"不要深度搜索，直接一句话回答",
	}
	for _, msg := range negative {
		if shouldPreferDeepSearchReport(msg) {
			t.Fatalf("expected no deep-search preference for %q", msg)
		}
	}
}

func TestBuildDeepSearchExecutionHint(t *testing.T) {
	hint := buildDeepSearchExecutionHint("帮我查 BlueAgent 最新动态并做完整报告")
	if hint == "" {
		t.Fatal("expected deep-search execution hint for latest/news request")
	}
	if !strings.Contains(hint, "at least 2 diverse web_search rounds") {
		t.Fatalf("expected hint to require multi-round search, got=%q", hint)
	}
	if !strings.Contains(hint, "complete report") {
		t.Fatalf("expected hint to require complete report, got=%q", hint)
	}

	if got := buildDeepSearchExecutionHint("你好"); got != "" {
		t.Fatalf("expected empty hint for non-research request, got=%q", got)
	}
}

func TestShouldEnforceDeepSearchMinRounds(t *testing.T) {
	if !shouldEnforceDeepSearchMinRounds("请做 BlueAgent 最新动态深度搜索，并给我完整报告附来源") {
		t.Fatal("expected hard deep-search gate for explicit deep-search report request")
	}
	if !shouldEnforceDeepSearchMinRounds("BlueAgent latest updates with sources and citations") {
		t.Fatal("expected hard deep-search gate for freshness + source requirement")
	}
	if shouldEnforceDeepSearchMinRounds("blueagent news") {
		t.Fatal("expected no hard deep-search gate for lightweight news lookup")
	}
}

func TestResolveToolRoundLimitForRequest(t *testing.T) {
	var h *ChatHandler

	if got := h.resolveToolRoundLimitForRequest(false, "hello", false); got != maxToolRounds {
		t.Fatalf("default non-agent tool rounds = %d, want %d", got, maxToolRounds)
	}

	researchMsg := "请做 BlueAgent 最新动态深度搜索，并给我完整报告附来源"
	gotResearch := h.resolveToolRoundLimitForRequest(false, researchMsg, false)
	if gotResearch <= maxToolRounds {
		t.Fatalf("expected research request to increase tool rounds, got=%d base=%d", gotResearch, maxToolRounds)
	}
	if gotResearch > maxToolRoundsNonAgentHardCap {
		t.Fatalf("research tool rounds exceeded hard cap: got=%d cap=%d", gotResearch, maxToolRoundsNonAgentHardCap)
	}

	gotDeepHint := h.resolveToolRoundLimitForRequest(false, "blueagent news", true)
	if gotDeepHint <= maxToolRounds {
		t.Fatalf("expected deep-search hint to increase tool rounds, got=%d base=%d", gotDeepHint, maxToolRounds)
	}
	if gotDeepHint > maxToolRoundsNonAgentHardCap {
		t.Fatalf("deep-search hinted tool rounds exceeded hard cap: got=%d cap=%d", gotDeepHint, maxToolRoundsNonAgentHardCap)
	}

	if got := h.resolveToolRoundLimitForRequest(true, researchMsg, true); got != maxToolRoundsAgent {
		t.Fatalf("agent mode should keep agent loop policy max, got=%d want=%d", got, maxToolRoundsAgent)
	}
}

func TestDeepSearchLoopState_ForceUntilMinRounds(t *testing.T) {
	state := newDeepSearchLoopState("请深度检索 BlueAgent 最新新闻并整理完整报告", []tools.ToolDefinition{
		{Name: "web_search"},
	})
	if state == nil || !state.enabled {
		t.Fatal("expected deep-search loop state enabled")
	}

	if force, reason := state.shouldForceAnotherSearch(0, 6); !force || reason != "min_rounds_not_met" {
		t.Fatalf("expected force before search rounds, got force=%v reason=%q", force, reason)
	}

	state.observeToolRound(
		[]llm.ToolCall{
			{ID: "s1", Name: "web_search", Arguments: `{"query":"BlueAgent release notes 2026"}`},
		},
		[]llm.Message{
			{
				Role:       llm.RoleTool,
				ToolCallID: "s1",
				Content:    `{"query":"BlueAgent release notes 2026","results":[{"title":"BlueAgent Releases","url":"https://github.com/blueagent/blueagent/releases","description":"release notes"}]}`,
			},
		},
	)
	if state.searchRounds != 1 {
		t.Fatalf("searchRounds=%d, want=1", state.searchRounds)
	}
	if force, _ := state.shouldForceAnotherSearch(1, 6); !force {
		t.Fatal("expected force after first search round")
	}

	state.observeToolRound(
		[]llm.ToolCall{
			{ID: "s2", Name: "web_search", Arguments: `{"query":"BlueAgent docs updates"}`},
		},
		[]llm.Message{
			{
				Role:       llm.RoleTool,
				ToolCallID: "s2",
				Content:    `{"query":"BlueAgent docs updates","results":[{"title":"BlueAgent Docs","url":"https://docs.blueagent.ai/tools/web","description":"web tool docs"}]}`,
			},
		},
	)
	if state.searchRounds != 2 {
		t.Fatalf("searchRounds=%d, want=2", state.searchRounds)
	}
	if force, reason := state.shouldForceAnotherSearch(2, 6); force || reason != "min_met" {
		t.Fatalf("expected no force when min met, got force=%v reason=%q", force, reason)
	}
}

func TestDeepSearchLoopState_FailOpenOnNoProgress(t *testing.T) {
	state := newDeepSearchLoopState("请深度检索 BlueAgent 最新新闻并整理完整报告", []tools.ToolDefinition{
		{Name: "web_search"},
	})
	if state == nil || !state.enabled {
		t.Fatal("expected deep-search loop state enabled")
	}

	state.observeToolRound(
		[]llm.ToolCall{
			{ID: "s1", Name: "web_search", Arguments: `{}`},
		},
		[]llm.Message{
			{
				Role:       llm.RoleTool,
				ToolCallID: "s1",
				Content:    `{"results":[]}`,
			},
		},
	)
	if state.searchRounds != 1 {
		t.Fatalf("searchRounds=%d, want=1", state.searchRounds)
	}
	if state.consecutiveNoProgress == 0 {
		t.Fatalf("expected no-progress streak > 0, got=%d", state.consecutiveNoProgress)
	}
	if force, reason := state.shouldForceAnotherSearch(1, 6); force || reason != "no_progress" {
		t.Fatalf("expected fail-open no_progress, got force=%v reason=%q", force, reason)
	}
}

func TestDeepSearchLoopState_FailOpenOnForceBudget(t *testing.T) {
	state := newDeepSearchLoopState("deep research on BlueAgent latest updates", []tools.ToolDefinition{
		{Name: "web_search"},
	})
	if state == nil || !state.enabled {
		t.Fatal("expected deep-search loop state enabled")
	}

	for i := 0; i < deepSearchForceContinueMaxRetries; i++ {
		if force, _ := state.shouldForceAnotherSearch(i, 6); !force {
			t.Fatalf("expected force within retry budget at i=%d", i)
		}
		state.markForcedContinuation()
	}
	if force, reason := state.shouldForceAnotherSearch(2, 6); force || reason != "force_budget" {
		t.Fatalf("expected fail-open force_budget, got force=%v reason=%q", force, reason)
	}
}

func TestNewDeepSearchLoopState_DisabledWithoutSearchCapability(t *testing.T) {
	state := newDeepSearchLoopState("帮我深度搜索 BlueAgent 最新消息", []tools.ToolDefinition{
		{Name: "reminder"},
		{Name: "scheduler"},
	})
	if state == nil {
		t.Fatal("expected non-nil state")
	}
	if state.enabled {
		t.Fatal("expected deep-search guard disabled when no search-capable tools are available")
	}
}

func TestIsShortQAShape_SkipsDeepSearchRequest(t *testing.T) {
	h := &ChatHandler{}
	req := SendMessageRequest{}
	if h.isShortQAShape(req, "帮我查一下 blueagent 的最新新闻") {
		t.Fatal("expected deep-search/news request not to route to short-qa")
	}
	if !h.isShortQAShape(req, "什么是 BlueAgent?") {
		t.Fatal("expected simple short question to keep short-qa shape")
	}
}

func TestCompactAdditionalSearchToolResultForLLM_KeptPreview(t *testing.T) {
	payload := map[string]interface{}{
		"query": "BlueAgent latest news",
		"results": []interface{}{
			map[string]interface{}{
				"title":       "BlueAgent release notes",
				"url":         "https://github.com/blueagent/blueagent/releases",
				"description": "official release notes",
			},
			map[string]interface{}{
				"title":       "BlueAgent docs",
				"url":         "https://docs.blueagent.ai/",
				"description": "official docs",
			},
			map[string]interface{}{
				"title":       "BlueAgent community report",
				"url":         "https://example.com/blueagent-report",
				"description": "community coverage",
			},
		},
		"total_count": 3,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	got := compactAdditionalSearchToolResultForLLM("web_search", string(raw))
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(got), &out); err != nil {
		t.Fatalf("unmarshal summary: %v", err)
	}
	if omitted, ok := out["omitted_from_llm_context"].(bool); !ok || !omitted {
		t.Fatalf("expected omitted_from_llm_context=true, got=%#v", out["omitted_from_llm_context"])
	}
	results, ok := out["results"].([]interface{})
	if !ok || len(results) == 0 {
		t.Fatalf("expected compacted result preview, got=%#v", out["results"])
	}
	if len(results) > maxLLMAdditionalSearchItems {
		t.Fatalf("preview result count = %d, want <= %d", len(results), maxLLMAdditionalSearchItems)
	}
}

func TestSanitizeResponseContent_StripsMalformedCommandWorkdirPrefix(t *testing.T) {
	leaked := "{\"command\":\"blue web_search query=\\\"BlueAgent GitHub release\\\"\"\"workdir\":\"/Users/orca/.zimaos-blue/data/workspace\"}我先帮你搜到一批 BlueAgent 相关最新结果（当前检索到 5 条）：\n- SecurityWeek"
	got := sanitizeResponseContentWithProvider(leaked, "MockProxy", "prov_auto_continue_scripted", "gpt-5.3-codex-spark")
	if strings.Contains(got, `"command":"blue web_search`) {
		t.Fatalf("expected leaked command json removed from sanitized content, got=%q", got)
	}
	if strings.Contains(got, `"workdir":`) {
		t.Fatalf("expected leaked workdir removed from sanitized content, got=%q", got)
	}
	if !strings.Contains(got, "我先帮你搜到一批 BlueAgent 相关最新结果") {
		t.Fatalf("expected user-facing text retained after sanitize, got=%q", got)
	}
}

func TestShouldAutoContinueAfterToollessReply(t *testing.T) {
	t.Run("agent mode continues on pending todo", func(t *testing.T) {
		current := "- [ ] 查询最新新闻\n- [ ] 汇总回答"
		ok, reason := shouldAutoContinueAfterToollessReply(current, "", true, false)
		if !ok || reason != "pending_todo" {
			t.Fatalf("expected pending_todo auto-continue, got ok=%v reason=%q", ok, reason)
		}
	})

	t.Run("non-agent continues on action pledge", func(t *testing.T) {
		current := "我现在就去查，稍等我几秒。"
		ok, reason := shouldAutoContinueAfterToollessReply(current, "", false, false)
		if !ok || reason != "action_pledge" {
			t.Fatalf("expected action_pledge auto-continue, got ok=%v reason=%q", ok, reason)
		}
	})

	t.Run("continues on pseudo tool-call text", func(t *testing.T) {
		current := "{\"cmd\":\"ls -la\"}I’ll inspect now.{\"tool\":\"exec\",\"cmd\":\"ls -la\"}exec: ls -la\n```tool\n{\"name\":\"exec\",\"arguments\":{\"cmd\":\"ls -la\"}}\n```\n<exec>{\"cmd\":\"ls -la\"}</exec>"
		ok, reason := shouldAutoContinueAfterToollessReply(current, "", false, false)
		if !ok || reason != "pseudo_tool_call" {
			t.Fatalf("expected pseudo_tool_call auto-continue, got ok=%v reason=%q", ok, reason)
		}
	})

	t.Run("continues on pseudo tool-call command/workdir json text", func(t *testing.T) {
		current := "收到，开始设置提醒。\nWorking on task: add reminder for 10 seconds later.{\"command\":\"blue reminder.add message=\\\"喝水\\\" time=10s\",\"workdir\":\"/tmp/workspace\"}{\"command\":\"blue help reminder\",...}"
		ok, reason := shouldAutoContinueAfterToollessReply(current, "", false, false)
		if !ok || reason != "pseudo_tool_call" {
			t.Fatalf("expected pseudo_tool_call auto-continue, got ok=%v reason=%q", ok, reason)
		}
	})

	t.Run("continues on pseudo tool-call single placeholder command json text", func(t *testing.T) {
		current := "我来给你设一个 10 秒后的提醒。{\"command\":\"...\"}`"
		ok, reason := shouldAutoContinueAfterToollessReply(current, "", false, false)
		if !ok || reason != "pseudo_tool_call" {
			t.Fatalf("expected pseudo_tool_call auto-continue, got ok=%v reason=%q", ok, reason)
		}
	})

	t.Run("continues on codex tool-directive leakage text", func(t *testing.T) {
		current := "to=functions.exec {\"command\":\"blue help browser\"} to=multi_tool_use.parallel {...}"
		ok, reason := shouldAutoContinueAfterToollessReply(current, "", false, false)
		if !ok || reason != "pseudo_tool_call" {
			t.Fatalf("expected pseudo_tool_call auto-continue, got ok=%v reason=%q", ok, reason)
		}
	})

	t.Run("continues on tool_uses plus recipient_name scaffold leakage", func(t *testing.T) {
		current := "{\"tool_uses\":[...]} with recipient_name and parameters to multi_tool_use.parallel. Let's do that."
		ok, reason := shouldAutoContinueAfterToollessReply(current, "", false, false)
		if !ok || reason != "pseudo_tool_call" {
			t.Fatalf("expected pseudo_tool_call auto-continue, got ok=%v reason=%q", ok, reason)
		}
	})

	t.Run("continues on leaked tool execution envelope text", func(t *testing.T) {
		current := "{\"tool_uses\":[...]} with recipient multi_tool_use.parallel. " +
			"{\"command\":\"blue web_search query=\\\"BlueAgent GitHub\\\"\"}" +
			"{\"data\":{\"format\":\"xml\",\"result\":\"<web_search>...</web_search>\",\"success\":true},\"duration_ms\":5,\"exit_code\":0,\"host\":\"local\",\"session_id\":\"16b745a8\",\"status\":\"completed\",\"stdout\":\"format: xml\"}"
		ok, reason := shouldAutoContinueAfterToollessReply(current, "", false, false)
		if !ok || reason != "pseudo_tool_call" {
			t.Fatalf("expected pseudo_tool_call auto-continue, got ok=%v reason=%q", ok, reason)
		}
	})

	t.Run("continues on protocol deliberation leakage text", func(t *testing.T) {
		current := `{"format":"..."}
No to field maybe automatically from functions.web_search?
Actually first call in transcript: assistant with commentary.
In this interface, I need specify function in message property maybe not possible manually?
Need include in assistant message header not possible in plaintext.
{"format":"...","max_results":10,"provider":"duckduckgo","query":"site:github.com/blueagent/blueagent/releases","region":"wt-wt"}`
		ok, reason := shouldAutoContinueAfterToollessReply(current, "", false, false)
		if !ok || reason != "pseudo_tool_call" {
			t.Fatalf("expected pseudo_tool_call auto-continue, got ok=%v reason=%q", ok, reason)
		}
	})

	t.Run("continues on reminder set claim without tool call when reminder is preferred", func(t *testing.T) {
		current := "已设置提醒：10秒后提醒你喝水"
		ok, reason := shouldAutoContinueAfterToollessReply(current, "", false, false, false, true)
		if !ok || reason != "pseudo_tool_call" {
			t.Fatalf("expected pseudo_tool_call auto-continue, got ok=%v reason=%q", ok, reason)
		}
	})

	t.Run("does not auto-continue on reminder set claim when reminder is not preferred", func(t *testing.T) {
		current := "已设置提醒：10秒后提醒你喝水"
		ok, reason := shouldAutoContinueAfterToollessReply(current, "", false, false, false, false)
		if ok {
			t.Fatalf("expected no auto-continue when reminder is not preferred, got reason=%q", reason)
		}
	})

	t.Run("does not treat a single cmd json example as pseudo tool-call", func(t *testing.T) {
		current := "你可以在 shell 里执行这个 JSON 示例：{\"cmd\":\"ls -la\"}"
		ok, reason := shouldAutoContinueAfterToollessReply(current, "", false, false)
		if ok {
			t.Fatalf("expected no auto-continue for benign single cmd json, got reason=%q", reason)
		}
	})

	t.Run("does not auto-continue for leaked command/workdir prefix when answer body is already concrete", func(t *testing.T) {
		current := "{\"command\":\"blue web_search query=\\\"BlueAgent GitHub release\\\"\"\"workdir\":\"/Users/orca/.zimaos-blue/data/workspace\"}我先帮你搜到一批 BlueAgent 相关最新结果（当前检索到 5 条）：\n- SecurityWeek"
		ok, reason := shouldAutoContinueAfterToollessReply(current, "", false, false)
		if ok {
			t.Fatalf("expected no auto-continue for leaked command/workdir prefix with concrete answer body, got reason=%q", reason)
		}
	})

	t.Run("agent mode bootstraps missing todo after prior rounds", func(t *testing.T) {
		current := "我会继续修改实现并补充验证。"
		ok, reason := shouldAutoContinueAfterToollessReply(current, "", true, true)
		if !ok || reason != "missing_todo" {
			t.Fatalf("expected missing_todo auto-continue, got ok=%v reason=%q", ok, reason)
		}
	})

	t.Run("agent mode completion reply without next-step guidance requests follow-up", func(t *testing.T) {
		current := "任务已完成。最终总结：功能可用。"
		ok, reason := shouldAutoContinueAfterToollessReply(current, "", true, true)
		if !ok || reason != "missing_next_steps" {
			t.Fatalf("expected missing_next_steps auto-continue on completion reply, got ok=%v reason=%q", ok, reason)
		}
	})

	t.Run("agent mode skips missing_todo when plan completion is known", func(t *testing.T) {
		current := "继续执行中。"
		ok, reason := shouldAutoContinueAfterToollessReply(current, "", true, true, true)
		if ok {
			t.Fatalf("expected no auto-continue when plan completion is known, got reason=%q", reason)
		}
	})

	t.Run("agent mode requests next-step guidance when completion misses it", func(t *testing.T) {
		current := "任务已完成。完成内容：已执行。使用方法：已验证。"
		ok, reason := shouldAutoContinueAfterToollessReply(current, "", true, true)
		if !ok || reason != "missing_next_steps" {
			t.Fatalf("expected missing_next_steps auto-continue, got ok=%v reason=%q", ok, reason)
		}
	})

	t.Run("agent mode completion with concrete localhost delivery does not force next steps", func(t *testing.T) {
		current := "已完成，游戏可直接运行，地址：http://localhost:3000"
		ok, reason := shouldAutoContinueAfterToollessReply(current, "", true, true)
		if ok {
			t.Fatalf("expected no auto-continue for concrete localhost delivery, got reason=%q", reason)
		}
	})

	t.Run("agent mode completion with pending tracked checklist does not force next steps", func(t *testing.T) {
		current := "任务已完成。最终总结：功能可用。"
		tracked := "- [x] 创建实现计划\n- [ ] 执行计划步骤"
		ok, reason := shouldAutoContinueAfterToollessReply(current, tracked, true, true)
		if ok {
			t.Fatalf("expected no auto-continue when tracked checklist is still pending, got reason=%q", reason)
		}
	})

	t.Run("agent mode completion after missing_todo bootstrap does not force next steps", func(t *testing.T) {
		current := "任务已完成。最终总结：实现可用并已验证。"
		ok, reason := shouldAutoContinueAfterToollessReply(current, "", true, true, false, false, false)
		if ok {
			t.Fatalf("expected no auto-continue when missing_next_steps is suppressed after missing_todo bootstrap, got reason=%q", reason)
		}
	})

	t.Run("agent mode does not request next-step guidance when already present", func(t *testing.T) {
		current := "任务已完成。完成内容：已执行。使用方法：已验证。下一步建议：1. 运行回归测试。"
		ok, reason := shouldAutoContinueAfterToollessReply(current, "", true, true)
		if ok {
			t.Fatalf("expected no auto-continue when next-step guidance already exists, got reason=%q", reason)
		}
	})
}

func TestBuildReducedContinuationRecoveryRequest_AggressiveFallbackShrinksWhenNeeded(t *testing.T) {
	chatReq := llm.ChatRequest{
		Model: "gpt-5.3-codex-spark",
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "system instructions"},
			{Role: llm.RoleAssistant, Content: "partial previous reply"},
			{Role: llm.RoleUser, Content: "continue please"},
		},
		Tools: []llm.Tool{
			{Name: "exec", Description: "execute command"},
			{Name: "web_search", Description: "search web"},
		},
	}

	reduced := buildReducedContinuationRecoveryRequest(chatReq)
	if len(reduced.Messages) >= len(chatReq.Messages) {
		t.Fatalf("reduced messages should shrink, got %d from %d", len(reduced.Messages), len(chatReq.Messages))
	}
	if len(reduced.Messages) == 0 {
		t.Fatal("reduced messages should not be empty")
	}
}

func TestExtractPlanChecklistFromToolRound(t *testing.T) {
	t.Run("extracts checklist from exec plan command result", func(t *testing.T) {
		calls := []llm.ToolCall{
			{
				Name:      "exec",
				Arguments: `{"command":"plan_update task_index=1 checked=true"}`,
			},
		}
		results := []llm.Message{
			{
				Role:    llm.RoleTool,
				Content: `{"status":"completed","data":{"checklist":"- [x] step1\n- [ ] step2"}}`,
			},
		}

		checklist, ok := extractPlanChecklistFromToolRound(calls, results)
		if !ok {
			t.Fatal("expected checklist extraction to succeed")
		}
		if checklist != "- [x] step1\n- [ ] step2" {
			t.Fatalf("unexpected checklist: %q", checklist)
		}
	})

	t.Run("ignores non-plan exec commands", func(t *testing.T) {
		calls := []llm.ToolCall{
			{
				Name:      "exec",
				Arguments: `{"command":"ls -la"}`,
			},
		}
		results := []llm.Message{
			{
				Role:    llm.RoleTool,
				Content: `{"status":"completed","data":{"checklist":"- [ ] should-not-be-used"}}`,
			},
		}

		if checklist, ok := extractPlanChecklistFromToolRound(calls, results); ok || checklist != "" {
			t.Fatalf("expected non-plan exec command to be ignored, got ok=%v checklist=%q", ok, checklist)
		}
	})

	t.Run("extracts checklist from direct plan tool result", func(t *testing.T) {
		calls := []llm.ToolCall{
			{
				Name:      "plan_create",
				Arguments: `{"tasks":["a","b"]}`,
			},
		}
		results := []llm.Message{
			{
				Role:    llm.RoleTool,
				Content: `{"checklist":"- [ ] a\n- [ ] b"}`,
			},
		}

		checklist, ok := extractPlanChecklistFromToolRound(calls, results)
		if !ok || checklist == "" {
			t.Fatalf("expected direct plan tool checklist extraction, got ok=%v checklist=%q", ok, checklist)
		}
	})
}

func TestExtractPlanCompletionFromToolRound(t *testing.T) {
	t.Run("extracts all_completed from exec plan command result", func(t *testing.T) {
		calls := []llm.ToolCall{
			{
				Name:      "exec",
				Arguments: `{"command":"plan_update task_index=2 checked=true"}`,
			},
		}
		results := []llm.Message{
			{
				Role:    llm.RoleTool,
				Content: `{"status":"completed","data":{"all_completed":"true","pending_count":"0","checklist":"- [x] a\n- [x] b"}}`,
			},
		}

		done, ok := extractPlanCompletionFromToolRound(calls, results)
		if !ok {
			t.Fatal("expected plan completion extraction to succeed")
		}
		if !done {
			t.Fatal("expected plan completion to be true")
		}
	})

	t.Run("extracts incomplete state from direct plan tool result", func(t *testing.T) {
		calls := []llm.ToolCall{
			{
				Name:      "plan_update",
				Arguments: `{"task_index":1,"checked":false}`,
			},
		}
		results := []llm.Message{
			{
				Role:    llm.RoleTool,
				Content: `{"operation":"update","task_count":2,"completed_count":1,"pending_count":1,"all_completed":false}`,
			},
		}

		done, ok := extractPlanCompletionFromToolRound(calls, results)
		if !ok {
			t.Fatal("expected plan completion extraction to succeed")
		}
		if done {
			t.Fatal("expected plan completion to be false")
		}
	})

	t.Run("ignores non-plan exec commands", func(t *testing.T) {
		calls := []llm.ToolCall{
			{
				Name:      "exec",
				Arguments: `{"command":"ls -la"}`,
			},
		}
		results := []llm.Message{
			{
				Role:    llm.RoleTool,
				Content: `{"status":"completed","data":{"all_completed":"true"}}`,
			},
		}

		if done, ok := extractPlanCompletionFromToolRound(calls, results); ok || done {
			t.Fatalf("expected non-plan exec completion to be ignored, got ok=%v done=%v", ok, done)
		}
	})
}

func TestFilterPseudoDirectiveDeltaForStreaming(t *testing.T) {
	t.Run("suppresses codex directive chunks and keeps suppression sticky in same round", func(t *testing.T) {
		suppressing := false
		suppressedChunks := 0

		first := filterPseudoDirectiveDeltaForStreaming(
			"先给结论：to=functions.exec {\"command\":\"blue help browser\"}",
			&suppressing,
			&suppressedChunks,
		)
		if first != "先给结论：" {
			t.Fatalf("unexpected first visible delta: %q", first)
		}
		if !suppressing {
			t.Fatal("expected suppressing=true after directive leakage")
		}

		noise := filterPseudoDirectiveDeltaForStreaming(
			"{\"command\":\"...\",\"workdir\":\"/tmp/workspace\"}",
			&suppressing,
			&suppressedChunks,
		)
		if noise != "" {
			t.Fatalf("expected noise chunk to be suppressed, got %q", noise)
		}

		recovery := filterPseudoDirectiveDeltaForStreaming(
			"这是最终答复。",
			&suppressing,
			&suppressedChunks,
		)
		if recovery != "" {
			t.Fatalf("expected recovery chunk to stay suppressed within same round, got %q", recovery)
		}
		if !suppressing {
			t.Fatal("expected suppressing=true after recovery chunk in sticky suppression mode")
		}
	})

	t.Run("detects recipient_name style directive leakage", func(t *testing.T) {
		suppressing := false
		suppressedChunks := 0
		visible := filterPseudoDirectiveDeltaForStreaming(
			"{\"recipient_name\":\"functions.exec\",\"parameters\":{\"command\":\"blue help browser\"}}",
			&suppressing,
			&suppressedChunks,
		)
		if visible != "" {
			t.Fatalf("expected recipient_name directive chunk to be fully suppressed, got %q", visible)
		}
		if !suppressing {
			t.Fatal("expected suppressing=true after recipient_name directive")
		}
	})

	t.Run("suppresses cmd plus tool-wrapper leakage", func(t *testing.T) {
		suppressing := false
		suppressedChunks := 0
		visible := filterPseudoDirectiveDeltaForStreaming(
			"{\"cmd\":\"ls -la\"}I’ll inspect now.{\"tool\":\"exec\",\"cmd\":\"ls -la\"}\n```tool\n{\"name\":\"exec\",\"arguments\":{\"cmd\":\"ls -la\"}}\n```\n<exec>{\"cmd\":\"ls -la\"}</exec>",
			&suppressing,
			&suppressedChunks,
		)
		if visible != "" {
			t.Fatalf("expected cmd/tool-wrapper leakage to be fully suppressed, got %q", visible)
		}
		if !suppressing {
			t.Fatal("expected suppressing=true after cmd/tool-wrapper leakage")
		}
	})

	t.Run("keeps suppressing across long noise bursts and suppresses clean text in same round", func(t *testing.T) {
		suppressing := false
		suppressedChunks := 0

		first := filterPseudoDirectiveDeltaForStreaming(
			"{\"tool_uses\":[...]}",
			&suppressing,
			&suppressedChunks,
		)
		if first != "" {
			t.Fatalf("expected initial tool_uses leakage to be fully suppressed, got %q", first)
		}
		if !suppressing {
			t.Fatal("expected suppressing=true after tool_uses leakage")
		}

		for i := 0; i < 120; i++ {
			noise := filterPseudoDirectiveDeltaForStreaming(
				"with recipient_name and parameters to multi_tool_use.parallel.",
				&suppressing,
				&suppressedChunks,
			)
			if noise != "" {
				t.Fatalf("expected long-burst scaffold chunk %d to stay suppressed, got %q", i, noise)
			}
		}
		if !suppressing {
			t.Fatal("expected suppressing=true after long noise burst")
		}

		recovery := filterPseudoDirectiveDeltaForStreaming(
			"这是最终答复。",
			&suppressing,
			&suppressedChunks,
		)
		if recovery != "" {
			t.Fatalf("expected recovery chunk to remain suppressed in same round, got %q", recovery)
		}
		if !suppressing {
			t.Fatal("expected suppressing=true after recovery chunk in sticky suppression mode")
		}
	})

	t.Run("suppresses leaked tool envelope burst with code fence and recipient phrase", func(t *testing.T) {
		suppressing := false
		suppressedChunks := 0

		first := filterPseudoDirectiveDeltaForStreaming(
			"{\"tool_uses\":[...]}",
			&suppressing,
			&suppressedChunks,
		)
		if first != "" {
			t.Fatalf("expected initial leakage to be fully suppressed, got %q", first)
		}
		if !suppressing {
			t.Fatal("expected suppressing=true after initial leakage")
		}

		fence := filterPseudoDirectiveDeltaForStreaming(
			"```",
			&suppressing,
			&suppressedChunks,
		)
		if fence != "" {
			t.Fatalf("expected code fence chunk to be suppressed, got %q", fence)
		}

		recipient := filterPseudoDirectiveDeltaForStreaming(
			"with recipient multi_tool_use.parallel. We can do that again.",
			&suppressing,
			&suppressedChunks,
		)
		if recipient != "" {
			t.Fatalf("expected recipient scaffold chunk to be suppressed, got %q", recipient)
		}

		envelope := filterPseudoDirectiveDeltaForStreaming(
			"{\"data\":{\"format\":\"xml\",\"result\":\"<web_search>...</web_search>\"},\"duration_ms\":5,\"exit_code\":0,\"host\":\"local\",\"session_id\":\"16b745a8\",\"status\":\"completed\",\"stdout\":\"format: xml\"}",
			&suppressing,
			&suppressedChunks,
		)
		if envelope != "" {
			t.Fatalf("expected leaked tool envelope chunk to be suppressed, got %q", envelope)
		}

		recovery := filterPseudoDirectiveDeltaForStreaming(
			"这是最终答复。",
			&suppressing,
			&suppressedChunks,
		)
		if recovery != "" {
			t.Fatalf("expected recovery chunk to remain suppressed in same round, got %q", recovery)
		}
		if !suppressing {
			t.Fatal("expected suppressing=true after recovery chunk in sticky suppression mode")
		}
	})

	t.Run("starts suppression when stream begins with leaked tool envelope", func(t *testing.T) {
		suppressing := false
		suppressedChunks := 0
		visible := filterPseudoDirectiveDeltaForStreaming(
			"{\"data\":{\"format\":\"xml\",\"result\":\"<web_search>...</web_search>\"},\"duration_ms\":5,\"exit_code\":0,\"host\":\"local\",\"session_id\":\"16b745a8\",\"status\":\"completed\",\"stdout\":\"format: xml\"}",
			&suppressing,
			&suppressedChunks,
		)
		if visible != "" {
			t.Fatalf("expected envelope-only initial chunk to be suppressed, got %q", visible)
		}
		if !suppressing {
			t.Fatal("expected suppressing=true after envelope-only initial chunk")
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
	if got := buildToollessAutoContinueNudge(false); !strings.Contains(got, "do not run `blue reminder --help`") {
		t.Fatalf("expected non-agent toolless nudge to block reminder help fallback, got=%q", got)
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
	if strings.Contains(pseudo, "fake tool-call text") {
		t.Fatalf("expected pseudo-tool nudge not to include dedicated fake-tool wording, got=%q", pseudo)
	}
	if !strings.Contains(pseudo, "Now actually execute by calling available tools") {
		t.Fatalf("expected pseudo-tool nudge to reuse generic execution nudge, got=%q", pseudo)
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
	missingTodo := buildToollessAutoContinueNudgeForReason(true, "missing_todo")
	if !strings.Contains(missingTodo, "checklist bootstrap required") || !strings.Contains(missingTodo, "`- [ ] step`") {
		t.Fatalf("expected missing_todo nudge to enforce checklist bootstrap format, got=%q", missingTodo)
	}
	if !strings.Contains(missingTodo, "Keep updating the same checklist") {
		t.Fatalf("expected missing_todo nudge to keep canonical checklist continuity, got=%q", missingTodo)
	}
	pendingTodo := buildToollessAutoContinueNudgeForReason(true, "pending_todo")
	if !strings.Contains(pendingTodo, "Do NOT output another TODO list") {
		t.Fatalf("expected pending_todo nudge to prevent checklist rewriting, got=%q", pendingTodo)
	}
	if !strings.Contains(pendingTodo, "at least one real tool call") {
		t.Fatalf("expected pending_todo nudge to enforce real execution, got=%q", pendingTodo)
	}
	missingNextSteps := buildToollessAutoContinueNudgeForReason(true, "missing_next_steps")
	if !strings.Contains(missingNextSteps, "WITHOUT calling tools") || !strings.Contains(missingNextSteps, "Suggested next steps") {
		t.Fatalf("expected missing_next_steps nudge to enforce completion + next-step guidance, got=%q", missingNextSteps)
	}

	if shouldPersistToollessRoundContent("pseudo_tool_call") {
		t.Fatal("expected pseudo_tool_call rounds not to be persisted")
	}
	if shouldPersistToollessRoundContent("missing_next_steps") {
		t.Fatal("expected missing_next_steps rounds not to be persisted")
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

func TestClassifyEmptyPostToolAutoContinueReason(t *testing.T) {
	if got := classifyEmptyPostToolAutoContinueReason("", false, false); got != "post_tool_summary" {
		t.Fatalf("non-agent reason = %q, want post_tool_summary", got)
	}
	if got := classifyEmptyPostToolAutoContinueReason("", true, false); got != "missing_todo" {
		t.Fatalf("agent missing TODO reason = %q, want missing_todo", got)
	}
	if got := classifyEmptyPostToolAutoContinueReason("- [ ] 搜索最近动态", true, false); got != "pending_todo" {
		t.Fatalf("agent pending TODO reason = %q, want pending_todo", got)
	}
	if got := classifyEmptyPostToolAutoContinueReason("- [x] 搜索最近动态", true, true); got != "post_tool_summary" {
		t.Fatalf("agent completed plan reason = %q, want post_tool_summary", got)
	}
}

func TestShouldCollapseToollessAutoContinueRound(t *testing.T) {
	if !shouldCollapseToollessAutoContinueRound("pending_todo", "- [ ] step 1") {
		t.Fatal("expected pending_todo rounds to be collapsed")
	}
	if !shouldCollapseToollessAutoContinueRound("missing_todo", "- [ ] step 1") {
		t.Fatal("expected missing_todo rounds to be collapsed")
	}
	if !shouldCollapseToollessAutoContinueRound("deep_search_min_rounds", "先给结论") {
		t.Fatal("expected deep_search_min_rounds rounds to be collapsed")
	}
	if !shouldCollapseToollessAutoContinueRound("action_pledge", "- [ ] step 1") {
		t.Fatal("expected checklist-style action_pledge rounds to be collapsed")
	}
	if shouldCollapseToollessAutoContinueRound("action_pledge", "我先去查，稍等几秒。") {
		t.Fatal("expected plain action_pledge rounds not to be collapsed")
	}
	if shouldCollapseToollessAutoContinueRound("pseudo_tool_call", "- [ ] step 1") {
		t.Fatal("expected pseudo_tool_call rounds not to be collapsed")
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

func TestBumpContinuationDegradationWindowAndThreshold(t *testing.T) {
	h := &ChatHandler{}
	base := time.Date(2026, 3, 2, 14, 30, 0, 0, time.UTC)

	for i := 1; i < continuationDegradeAlertThreshold; i++ {
		count, alert := h.bumpContinuationDegradation(base)
		if count != i {
			t.Fatalf("count = %d, want %d", count, i)
		}
		if alert {
			t.Fatalf("unexpected alert before threshold at count=%d", count)
		}
	}

	count, alert := h.bumpContinuationDegradation(base)
	if count != continuationDegradeAlertThreshold {
		t.Fatalf("count = %d, want threshold %d", count, continuationDegradeAlertThreshold)
	}
	if !alert {
		t.Fatal("expected alert at threshold")
	}

	count, alert = h.bumpContinuationDegradation(base.Add(continuationDegradeAlertWindow + time.Second))
	if count != 1 {
		t.Fatalf("count after window reset = %d, want 1", count)
	}
	if alert {
		t.Fatal("did not expect alert after window reset")
	}
}

func TestToollessAutoContinueBudget(t *testing.T) {
	if shouldAutoContinueForReasonWithinBudget("pending_todo", false, 0, 0, 0, 0) {
		t.Fatal("expected non-agent pending_todo to be disabled")
	}
	if !shouldAutoContinueForReasonWithinBudget("pending_todo", true, 0, 0, 0, maxPendingTodoAutoContinueAgent-1) {
		t.Fatal("expected agent pending_todo within budget to continue")
	}
	if shouldAutoContinueForReasonWithinBudget("pending_todo", true, 0, 0, 0, maxPendingTodoAutoContinueAgent) {
		t.Fatal("expected agent pending_todo at budget limit to stop")
	}

	if !shouldAutoContinueForReasonWithinBudget("pseudo_tool_call", false, maxPseudoToolCallAutoContinueDefault-1, 0, 0, 0) {
		t.Fatal("expected non-agent pseudo_tool_call within budget to continue")
	}
	if shouldAutoContinueForReasonWithinBudget("pseudo_tool_call", false, maxPseudoToolCallAutoContinueDefault, 0, 0, 0) {
		t.Fatal("expected non-agent pseudo_tool_call at budget limit to stop")
	}

	if !shouldAutoContinueForReasonWithinBudget("pseudo_tool_call", true, maxPseudoToolCallAutoContinueAgent-1, 0, 0, 0) {
		t.Fatal("expected agent pseudo_tool_call within budget to continue")
	}
	if shouldAutoContinueForReasonWithinBudget("pseudo_tool_call", true, maxPseudoToolCallAutoContinueAgent, 0, 0, 0) {
		t.Fatal("expected agent pseudo_tool_call at budget limit to stop")
	}

	if !shouldAutoContinueForReasonWithinBudget("action_pledge", false, 0, maxActionPledgeAutoContinueDefault-1, 0, 0) {
		t.Fatal("expected non-agent action_pledge within budget to continue")
	}
	if shouldAutoContinueForReasonWithinBudget("action_pledge", false, 0, maxActionPledgeAutoContinueDefault, 0, 0) {
		t.Fatal("expected non-agent action_pledge at budget limit to stop")
	}

	if !shouldAutoContinueForReasonWithinBudget("action_pledge", true, 0, maxActionPledgeAutoContinueAgent-1, 0, 0) {
		t.Fatal("expected agent action_pledge within budget to continue")
	}
	if shouldAutoContinueForReasonWithinBudget("action_pledge", true, 0, maxActionPledgeAutoContinueAgent, 0, 0) {
		t.Fatal("expected agent action_pledge at budget limit to stop")
	}

	if shouldAutoContinueForReasonWithinBudget("missing_todo", false, 0, 0, 0, 0) {
		t.Fatal("expected non-agent missing_todo to be disabled")
	}
	if !shouldAutoContinueForReasonWithinBudget("missing_todo", true, 0, 0, maxMissingTodoAutoContinueAgent-1, 0) {
		t.Fatal("expected agent missing_todo within budget to continue")
	}
	if shouldAutoContinueForReasonWithinBudget("missing_todo", true, 0, 0, maxMissingTodoAutoContinueAgent, 0) {
		t.Fatal("expected agent missing_todo at budget limit to stop")
	}

	if shouldAutoContinueForReasonWithinBudget("missing_next_steps", false, 0, 0, 0, 0) {
		t.Fatal("expected non-agent missing_next_steps to be disabled")
	}
	if !shouldAutoContinueForReasonWithinBudget("missing_next_steps", true, 0, 0, 0, maxPendingTodoAutoContinueAgent-1) {
		t.Fatal("expected agent missing_next_steps within budget to continue")
	}
	if shouldAutoContinueForReasonWithinBudget("missing_next_steps", true, 0, 0, 0, maxPendingTodoAutoContinueAgent) {
		t.Fatal("expected agent missing_next_steps at budget limit to stop")
	}
}

func TestActionPledgeDuplicateDebounce(t *testing.T) {
	if shouldStopForDuplicateActionPledge("action_pledge", maxConsecutiveDuplicateActionPledgeAutoContinue) {
		t.Fatal("expected first action_pledge duplicate count within threshold")
	}
	if !shouldStopForDuplicateActionPledge("action_pledge", maxConsecutiveDuplicateActionPledgeAutoContinue+1) {
		t.Fatal("expected action_pledge duplicate count over threshold to stop")
	}
	if shouldStopForDuplicateActionPledge("pseudo_tool_call", 10) {
		t.Fatal("expected duplicate stop gate to apply only to action_pledge")
	}
}

func TestBuildToolFallbackText_RedactsSensitiveOutput(t *testing.T) {
	msgs := []llm.Message{
		{
			Role:    llm.RoleTool,
			Content: `{"stdout":"AWS_SECRET_ACCESS_KEY=abc123","stderr":"password=very-secret","error":"token leaked","exit_code":1}`,
		},
		{
			Role:    llm.RoleTool,
			Content: `{"status":"completed","stdout":"safe output"}`,
		},
	}

	out, toolCount := buildToolFallbackText(msgs, 4096)
	if toolCount != 2 {
		t.Fatalf("toolCount = %d, want 2", toolCount)
	}
	if strings.Contains(out, "AWS_SECRET_ACCESS_KEY") || strings.Contains(out, "password=very-secret") || strings.Contains(out, "token leaked") {
		t.Fatalf("expected fallback text to redact raw sensitive output, got=%q", out)
	}
	if !strings.Contains(out, "Safe status: 1 succeeded, 1 failed, 0 unknown.") {
		t.Fatalf("expected fallback safe status summary, got=%q", out)
	}
	if !strings.Contains(out, "redacted for safety") {
		t.Fatalf("expected fallback redaction marker, got=%q", out)
	}
}

func TestBuildToolFallbackText_NoToolResults(t *testing.T) {
	out, toolCount := buildToolFallbackText([]llm.Message{
		{Role: llm.RoleAssistant, Content: "hello"},
	}, 4096)
	if toolCount != 0 {
		t.Fatalf("toolCount = %d, want 0", toolCount)
	}
	if !strings.Contains(out, "Raw tool output is hidden for safety") {
		t.Fatalf("expected safe no-tool fallback text, got=%q", out)
	}
}

func TestBuildToolFallbackText_UsesExtractedSafeSummaryWhenAvailable(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: "帮我调研一下最近一周 blueagent 的动向"},
		{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{ID: "call_web_1", Name: "web_search", Arguments: `{"query":"blueagent latest"}`},
			},
		},
		{
			Role:       llm.RoleTool,
			ToolCallID: "call_web_1",
			Content:    `{"query":"blueagent latest","results":[{"title":"BlueAgent Releases","url":"https://github.com/blueagent/blueagent/releases","description":"release notes"},{"title":"BlueAgent Docs","url":"https://docs.blueagent.ai","description":"documentation"}],"stdout":"secret-token"}`,
		},
	}

	out, toolCount := buildToolFallbackText(msgs, 4096)
	if toolCount != 1 {
		t.Fatalf("toolCount = %d, want 1", toolCount)
	}
	if !strings.Contains(out, "自动提炼的安全摘要") {
		t.Fatalf("expected extracted safe summary marker, got=%q", out)
	}
	if !strings.Contains(out, "BlueAgent Releases") {
		t.Fatalf("expected extracted web search title in fallback, got=%q", out)
	}
	if strings.Contains(out, "Safe status:") {
		t.Fatalf("expected extracted summary path, got generic safe status fallback=%q", out)
	}
	if strings.Contains(out, "secret-token") {
		t.Fatalf("expected raw tool output redacted in fallback, got=%q", out)
	}
}

func TestBuildToolFallbackText_UsesExtractedSummaryForWebSearchXML(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: "帮我看看 BlueAgent 最近一周动态"},
		{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{ID: "call_web_xml_1", Name: "web_search", Arguments: `{"query":"blueagent weekly"}`},
			},
		},
		{
			Role:       llm.RoleTool,
			ToolCallID: "call_web_xml_1",
			Content: `<web_search>
  <query>blueagent weekly</query>
  <provider>duckduckgo</provider>
  <total_count>2</total_count>
  <results>
    <result><title>BlueAgent Weekly Update</title><url>https://example.com/a</url></result>
    <result><title>BlueAgent Release Notes</title><url>https://example.com/b</url></result>
  </results>
</web_search>`,
		},
	}

	out, toolCount := buildToolFallbackText(msgs, 4096)
	if toolCount != 1 {
		t.Fatalf("toolCount = %d, want 1", toolCount)
	}
	if !strings.Contains(out, "自动提炼的安全摘要") {
		t.Fatalf("expected extracted safe summary marker, got=%q", out)
	}
	if !strings.Contains(out, "BlueAgent Weekly Update") {
		t.Fatalf("expected extracted xml web_search title in fallback, got=%q", out)
	}
	if strings.Contains(out, "Safe status:") {
		t.Fatalf("expected extracted summary path, got generic safe status fallback=%q", out)
	}
}

func TestBuildToolFallbackTextWithOptions_UsesConciseSummaryWhenCardsVisible(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: "帮我调研一下最近一周 OpenClaw 的动向"},
		{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{{
				ID:        "call_web_2",
				Name:      "web_search",
				Arguments: `{"query":"OpenClaw latest news 2025"}`,
			}},
		},
		{
			Role:       llm.RoleTool,
			ToolCallID: "call_web_2",
			Content:    `{"query":"OpenClaw latest news 2025","results":[{"title":"OpenClaw 发布周报","url":"https://example.com/openclaw-weekly"},{"title":"OpenClaw Roadmap Update","url":"https://example.com/openclaw-roadmap"}],"stdout":"secret-token"}`,
		},
	}

	out, toolCount := buildToolFallbackTextWithOptions(msgs, 4096, toolFallbackTextOptions{toolCardsVisible: true})
	if toolCount != 1 {
		t.Fatalf("toolCount = %d, want 1", toolCount)
	}
	if !strings.Contains(out, "我先根据已完成的工具结果，给你一个简要汇总") {
		t.Fatalf("expected concise cards-visible summary intro, got=%q", out)
	}
	if !strings.Contains(out, "详细执行记录见上方工具卡片") {
		t.Fatalf("expected cards-visible follow-up note, got=%q", out)
	}
	if !strings.Contains(out, "OpenClaw 发布周报") {
		t.Fatalf("expected extracted search result title, got=%q", out)
	}
	if strings.Contains(out, "最终总结生成失败") {
		t.Fatalf("expected softer fallback wording without failure phrasing, got=%q", out)
	}
	if strings.Contains(out, "secret-token") {
		t.Fatalf("expected raw tool output redacted in fallback, got=%q", out)
	}
}

func TestBuildToolFallbackText_UsesToolNameSummaryWhenExtractionUnavailable(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: "帮我查一下近期项目进展"},
		{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{ID: "call_1", Name: "web_search"},
				{ID: "call_2", Name: "exec_command"},
			},
		},
		// Simulate concurrent completion order: call_2 returns before call_1.
		{Role: llm.RoleTool, ToolCallID: "call_2", Content: "execution completed"},
		{Role: llm.RoleTool, ToolCallID: "call_1", Content: `{"status":"running","stdout":"secret"}`},
	}

	out, toolCount := buildToolFallbackText(msgs, 4096)
	if toolCount != 2 {
		t.Fatalf("toolCount = %d, want 2", toolCount)
	}
	if strings.Contains(out, "Safe status:") {
		t.Fatalf("expected tool-name fallback path instead of safe status diagnostics, got=%q", out)
	}
	if !strings.Contains(out, "exec_command") || !strings.Contains(out, "web_search") {
		t.Fatalf("expected tool names in fallback summary, got=%q", out)
	}
	if !strings.Contains(out, "stdout/stderr/error") {
		t.Fatalf("expected redaction notice in fallback, got=%q", out)
	}
	if strings.Contains(out, "secret") {
		t.Fatalf("expected raw tool output redacted in fallback, got=%q", out)
	}
}

func TestFormatProcessBlock_HidesSensitiveToolResultOutput(t *testing.T) {
	summary := []map[string]interface{}{
		{
			"name":   "exec",
			"args":   `{"command":"echo secret"}`,
			"result": `{"exit_code":1,"error":"password=abc123","stdout":"token=xyz","duration_ms":12}`,
		},
	}
	got := formatProcessBlock(summary)
	if strings.Contains(got, "password=abc123") || strings.Contains(got, "token=xyz") {
		t.Fatalf("expected process block to mask sensitive result text, got=%q", got)
	}
	if !strings.Contains(got, "password=[REDACTED]") {
		t.Fatalf("expected process block to include masked error details, got=%q", got)
	}
	if !strings.Contains(got, "token=[REDACTED]") {
		t.Fatalf("expected process block to include masked stdout text, got=%q", got)
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
	if got := choosePseudoToolCallPrimaryToolIndex(tools, false); got != 2 {
		t.Fatalf("expected exec to be preferred, got index=%d", got)
	}

	tools = []llm.Tool{
		{Name: "ask"},
		{Name: "file_read"},
	}
	if got := choosePseudoToolCallPrimaryToolIndex(tools, false); got != 1 {
		t.Fatalf("expected non-ask tool fallback, got index=%d", got)
	}

	tools = []llm.Tool{
		{Name: "exec"},
		{Name: "reminder"},
	}
	if got := choosePseudoToolCallPrimaryToolIndex(tools, true); got != 1 {
		t.Fatalf("expected reminder to be preferred when reminder intent is detected, got index=%d", got)
	}
}

func TestShouldPreferReminderToolForRetry(t *testing.T) {
	tools := []llm.Tool{
		{Name: "exec"},
		{Name: "reminder"},
	}
	messages := []llm.Message{
		{Role: llm.RoleUser, Content: "提醒我10秒钟以后喝水"},
	}
	if !shouldPreferReminderToolForRetry(messages, tools) {
		t.Fatal("expected reminder preference for reminder intent")
	}

	messages = []llm.Message{
		{Role: llm.RoleUser, Content: "帮我看一下这个目录里的文件"},
	}
	if shouldPreferReminderToolForRetry(messages, tools) {
		t.Fatal("expected no reminder preference for non-reminder intent")
	}

	if shouldPreferReminderToolForRetry([]llm.Message{{Role: llm.RoleUser, Content: "提醒我喝水"}}, []llm.Tool{{Name: "exec"}}) {
		t.Fatal("expected no reminder preference when reminder tool is unavailable")
	}
}

func TestApplyReminderToolPreference(t *testing.T) {
	defs := []tools.ToolDefinition{
		{Name: "web_search"},
		{Name: "reminder"},
		{Name: "notifications"},
	}

	filtered := applyReminderToolPreference(defs, "提醒我10秒后喝水")
	if len(filtered) != 1 || filtered[0].Name != "reminder" {
		t.Fatalf("expected reminder-only toolset for reminder intent, got=%v", toolNames(filtered))
	}

	kept := applyReminderToolPreference(defs, "帮我同步到系统提醒事项")
	if len(kept) != len(defs) {
		t.Fatalf("expected full toolset for explicit system reminder target, got=%v", toolNames(kept))
	}

	unchanged := applyReminderToolPreference(defs, "帮我查一下今天的新闻")
	if len(unchanged) != len(defs) {
		t.Fatalf("expected non-reminder query not to be filtered, got=%v", toolNames(unchanged))
	}

	noReminderDefs := []tools.ToolDefinition{
		{Name: "web_search"},
		{Name: "notifications"},
	}
	noFilter := applyReminderToolPreference(noReminderDefs, "提醒我十分钟后站起来")
	if len(noFilter) != len(noReminderDefs) {
		t.Fatalf("expected no filtering when built-in reminder is unavailable, got=%v", toolNames(noFilter))
	}
}

func toolNames(defs []tools.ToolDefinition) []string {
	names := make([]string, 0, len(defs))
	for _, def := range defs {
		names = append(names, def.Name)
	}
	return names
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

func TestHardenPseudoToolCallRetryRequest_PrefersReminderForReminderIntent(t *testing.T) {
	req := llm.ChatRequest{
		Temperature: 0.7,
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "remind me to drink water in 10 seconds"},
		},
		Tools: []llm.Tool{
			{
				Name: "exec",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"command": map[string]interface{}{"type": "string"},
					},
				},
			},
			{
				Name: "reminder",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"action":  map[string]interface{}{"type": "string"},
						"message": map[string]interface{}{"type": "string"},
						"time":    map[string]interface{}{"type": "string"},
					},
				},
			},
		},
	}

	actions := hardenPseudoToolCallRetryRequest(&req)
	if len(actions) == 0 {
		t.Fatal("expected hardening actions to be applied")
	}
	if len(req.Tools) != 1 || req.Tools[0].Name != "reminder" {
		t.Fatalf("expected tool scope to reduce to reminder, got %+v", req.Tools)
	}
}

func TestSanitizeResponseContent_StripsPseudoDirectiveArtifactsButKeepsAnswer(t *testing.T) {
	raw := `---
to=functions.exec  乐盈json ...
Let's do correctly.
to=functions.exec  菲娱json
{"command":"blue help browser","workdir":"/Users/orca/.zimaos-blue/data/workspace"}to=functions.exec d天天json
{"command":"blue help browser","workdir":"/Users/orca/.zimaos-blue/data/workspace"}收到，你选 **2**。

第 2 条是这篇：
- 标题：别再用旧版了！BlueAgent 2026.2.9 更新迁移避坑指南`

	got := sanitizeResponseContent(raw)
	if strings.Contains(strings.ToLower(got), "to=functions.exec") {
		t.Fatalf("expected pseudo directive token removed, got=%q", got)
	}
	if strings.Contains(got, `"command":"blue help browser"`) {
		t.Fatalf("expected leaked command json removed, got=%q", got)
	}
	if !strings.Contains(got, "收到，你选 **2**。") {
		t.Fatalf("expected user-facing answer retained, got=%q", got)
	}
	if !strings.Contains(got, "第 2 条是这篇：") {
		t.Fatalf("expected normal summary content retained, got=%q", got)
	}
}

func TestSanitizeResponseContent_DoesNotStripBenignJSONExample(t *testing.T) {
	raw := `你可以在 shell 里执行这个 JSON 示例：{"cmd":"ls -la"}`
	got := sanitizeResponseContent(raw)
	if got != raw {
		t.Fatalf("expected benign single JSON example preserved, got=%q", got)
	}
}

func TestSanitizeResponseContent_StripsLeakedCommandWorkdirPrefixButKeepsAnswer(t *testing.T) {
	raw := `{"command":"blue web_search query=\"BlueAgent GitHub release\"""workdir":"/Users/orca/.zimaos-blue/data/workspace"}我先帮你搜到一批 BlueAgent 相关最新结果（当前检索到 5 条）：`
	got := sanitizeResponseContent(raw)
	if strings.Contains(got, `"command":"blue web_search`) {
		t.Fatalf("expected leaked command json removed, got=%q", got)
	}
	if strings.Contains(got, `"workdir":`) {
		t.Fatalf("expected leaked workdir removed, got=%q", got)
	}
	if !strings.Contains(got, "我先帮你搜到一批 BlueAgent 相关最新结果") {
		t.Fatalf("expected user-facing answer retained, got=%q", got)
	}
}

func TestResolveResponseSanitizeProfile_Deterministic(t *testing.T) {
	if got := resolveResponseSanitizeProfile("openai", "openai", "gpt-4o"); got != responseSanitizeProfileBalanced {
		t.Fatalf("expected openai profile balanced, got=%s", got)
	}
	if got := resolveResponseSanitizeProfile("deepresearch", "deepresearch", "deepresearch-fallback"); got != responseSanitizeProfileMinimal {
		t.Fatalf("expected deepresearch profile minimal, got=%s", got)
	}
	if got := resolveResponseSanitizeProfile("openai", "openai", "gpt-5.3-codex-spark"); got != responseSanitizeProfileStrict {
		t.Fatalf("expected codex model profile strict, got=%s", got)
	}
	if got := resolveResponseSanitizeProfile("", "custom_vendor", "custom-model"); got != responseSanitizeProfileBalanced {
		t.Fatalf("expected unknown provider profile balanced, got=%s", got)
	}
}

func TestSanitizeResponseContentWithProvider_ProfileStrategy(t *testing.T) {
	raw := `{"command":"blue help browser"}这是正文`

	strict := sanitizeResponseContentWithProvider(raw, "openai", "openai", "gpt-5.3-codex-spark")
	if strings.Contains(strict, `"command":"blue help browser"`) {
		t.Fatalf("expected strict profile to strip leaked command json, got=%q", strict)
	}
	if !strings.Contains(strict, "这是正文") {
		t.Fatalf("expected strict profile to keep user-facing answer, got=%q", strict)
	}

	balanced := sanitizeResponseContentWithProvider(raw, "openai", "openai", "gpt-4o")
	if balanced != raw {
		t.Fatalf("expected balanced profile to keep benign single command example, got=%q", balanced)
	}

	minimal := sanitizeResponseContentWithProvider(raw, "deepresearch", "deepresearch", "deepresearch-fallback")
	if minimal != raw {
		t.Fatalf("expected minimal profile to keep benign single command example, got=%q", minimal)
	}
}

func TestSanitizeResponseContentWithProvider_StripsProtocolDeliberationLeakInStrictProfile(t *testing.T) {
	raw := `{"format":"..."}
No to field maybe automatically from functions.web_search? Actually first call in transcript: assistant with commentary.
In this interface, I need specify function in message property maybe not possible manually?
{"format":"...","max_results":10,"provider":"duckduckgo","query":"site:github.com/blueagent/blueagent/releases","region":"wt-wt"}
以下是最近一周 BlueAgent 动向：发布了新版本并修复了关键问题。`

	got := sanitizeResponseContentWithProvider(raw, "openai", "openai", "gpt-5.3-codex-spark")
	if strings.Contains(got, "functions.web_search") {
		t.Fatalf("expected protocol deliberation leakage removed, got=%q", got)
	}
	if strings.Contains(got, "assistant with commentary") {
		t.Fatalf("expected commentary scaffold removed, got=%q", got)
	}
	if strings.Contains(got, `{"format":"...","max_results":10`) {
		t.Fatalf("expected leaked protocol json removed, got=%q", got)
	}
	if !strings.Contains(got, "以下是最近一周 BlueAgent 动向") {
		t.Fatalf("expected user-facing summary retained, got=%q", got)
	}
}
