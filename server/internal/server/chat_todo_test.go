package server

import "testing"

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
}
