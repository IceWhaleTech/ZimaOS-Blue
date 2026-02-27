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
}
