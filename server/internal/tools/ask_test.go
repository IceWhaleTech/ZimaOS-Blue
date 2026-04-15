package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
)

func TestAskDefinition_ExposesCanonicalQuestionsOnly(t *testing.T) {
	tool := NewAskTool(nil)
	props := tool.Definition().Parameters["properties"].(map[string]interface{})
	if _, ok := props["questions"]; !ok {
		t.Fatal("expected questions property in ask schema")
	}
	for _, legacy := range []string{"q", "mq", "a", "detail"} {
		if _, ok := props[legacy]; ok {
			t.Fatalf("did not expect legacy %q in ask schema", legacy)
		}
	}
}

func TestParseQuestionOptions_ObjectOptions(t *testing.T) {
	in := []interface{}{
		map[string]interface{}{
			"label":       "Low error rate (Recommended)",
			"description": "Most stable",
			"value":       "stable",
		},
		map[string]interface{}{
			"text":  "Lowest latency",
			"hint":  "May be less stable",
			"value": "fast",
		},
		"Zero extra model cost",
	}

	got := parseQuestionOptions(in)
	if len(got) != 3 {
		t.Fatalf("len(options) = %d, want 3", len(got))
	}
	if got[0].Label != "Low error rate (Recommended)" || got[0].Description != "Most stable" || got[0].Value != "stable" {
		t.Fatalf("option[0] mismatch: %#v", got[0])
	}
	if got[1].Label != "Lowest latency" || got[1].Description != "May be less stable" || got[1].Value != "fast" {
		t.Fatalf("option[1] mismatch: %#v", got[1])
	}
	if got[2].Label != "Zero extra model cost" || got[2].Value != "Zero extra model cost" {
		t.Fatalf("option[2] mismatch: %#v", got[2])
	}
}

func TestAskExecute_UsesQuestionOptionLabelsInQA(t *testing.T) {
	mgr := NewQuestionManager(nil, func() bool { return true }, 0)
	tool := NewAskTool(mgr)
	args := map[string]interface{}{
		"q": "First priority?",
		"a": []interface{}{
			map[string]interface{}{
				"label":       "Low error rate (Recommended)",
				"description": "Most stable path",
				"value":       "stable",
			},
			map[string]interface{}{
				"label": "Lowest latency",
				"value": "fast",
			},
		},
	}

	raw, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	outStr, ok := raw.(string)
	if !ok {
		t.Fatalf("Execute() type = %T, want string", raw)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(outStr), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}

	qa, ok := out["qa"].([]interface{})
	if !ok || len(qa) != 1 {
		t.Fatalf("qa = %#v, want single item", out["qa"])
	}
	first, ok := qa[0].(map[string]interface{})
	if !ok {
		t.Fatalf("qa[0] type = %T, want object", qa[0])
	}
	options, ok := first["o"].([]interface{})
	if !ok || len(options) != 2 {
		t.Fatalf("qa[0].o = %#v, want 2 labels", first["o"])
	}
	if options[0] != "Low error rate (Recommended)" {
		t.Fatalf("qa[0].o[0] = %v, want recommended label", options[0])
	}

	selected, ok := out["a"].([]interface{})
	if !ok || len(selected) != 1 || selected[0] != "stable" {
		t.Fatalf("a = %#v, want [\"stable\"]", out["a"])
	}
}

func TestAskExecute_IncludesOtherTextInQAAnswers(t *testing.T) {
	broker := sse.NewBroker()
	ch := broker.Subscribe("user-ask-other")
	defer broker.Unsubscribe("user-ask-other", ch)

	mgr := NewQuestionManager(broker, func() bool { return false }, 2*time.Minute)
	tool := NewAskTool(mgr)

	ctx := WithChannel(WithUserID(context.Background(), "user-ask-other"), "web")
	done := make(chan struct {
		raw interface{}
		err error
	}, 1)

	go func() {
		raw, err := tool.Execute(ctx, map[string]interface{}{
			"questions": []interface{}{
				map[string]interface{}{
					"question": "How should we proceed?",
					"type":     "radio",
					"options":  []interface{}{"Option A", "Option B"},
				},
			},
		})
		done <- struct {
			raw interface{}
			err error
		}{raw: raw, err: err}
	}()

	deadline := time.Now().Add(2 * time.Second)
	var pending *QuestionRequest
	for time.Now().Before(deadline) {
		pending = mgr.GetPending("user-ask-other")
		if pending != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pending == nil {
		t.Fatal("expected pending question request")
	}

	if !mgr.ResolveAnswer(pending.ID, []QuestionAnswerResult{{
		QuestionID: "q0",
		Selected:   nil,
		OtherText:  "Custom plan",
	}}) {
		t.Fatalf("failed to resolve question id=%s", pending.ID)
	}

	select {
	case result := <-done:
		if result.err != nil {
			t.Fatalf("Execute() error = %v", result.err)
		}
		outStr, ok := result.raw.(string)
		if !ok {
			t.Fatalf("Execute() type = %T, want string", result.raw)
		}
		var out map[string]interface{}
		if err := json.Unmarshal([]byte(outStr), &out); err != nil {
			t.Fatalf("unmarshal output error = %v", err)
		}

		qa, ok := out["qa"].([]interface{})
		if !ok || len(qa) != 1 {
			t.Fatalf("qa = %#v, want single item", out["qa"])
		}
		first, ok := qa[0].(map[string]interface{})
		if !ok {
			t.Fatalf("qa[0] type = %T, want object", qa[0])
		}
		answers, ok := first["a"].([]interface{})
		if !ok || len(answers) != 1 || answers[0] != "Custom plan" {
			t.Fatalf("qa[0].a = %#v, want [\"Custom plan\"]", first["a"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ask execution result")
	}
}

func TestAskExecute_SupportsNestedCamelCaseArgs(t *testing.T) {
	mgr := NewQuestionManager(nil, func() bool { return true }, 0)
	tool := NewAskTool(mgr)
	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"question":    "Choose mode",
			"detail":      "Extra context",
			"multiSelect": true,
			"options":     []interface{}{"Fast", "Safe"},
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	outStr, ok := raw.(string)
	if !ok {
		t.Fatalf("Execute() type = %T, want string", raw)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(outStr), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if got := out["mq"]; got != "Choose mode" {
		t.Fatalf("mq = %v, want %q", got, "Choose mode")
	}
	selected, ok := out["a"].([]interface{})
	if !ok || len(selected) != 1 || selected[0] != "Fast" {
		t.Fatalf("a = %#v, want [\"Fast\"]", out["a"])
	}
}

func TestAskExecute_RejectsQAAliasesInsideQuestions(t *testing.T) {
	mgr := NewQuestionManager(nil, func() bool { return true }, 0)
	tool := NewAskTool(mgr)
	args := map[string]interface{}{
		"questions": []interface{}{
			map[string]interface{}{
				"q":    "How should I answer?",
				"type": "checkbox",
				"a":    []interface{}{"free", "template"},
			},
		},
	}

	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Fatalf("expected error for q/a aliases in questions")
	}
}

func TestParseAskArgs_ParsesTopLevelDetail(t *testing.T) {
	question, detail, multi, options, allowEmptyOptions := parseAskArgs(map[string]interface{}{
		"q":      "Choose one",
		"detail": "❕ 这是补充说明",
		"a":      []interface{}{"A", "B"},
	})
	if question != "Choose one" {
		t.Fatalf("question = %q, want %q", question, "Choose one")
	}
	if detail != "❕ 这是补充说明" {
		t.Fatalf("detail = %q, want %q", detail, "❕ 这是补充说明")
	}
	if multi {
		t.Fatalf("multi = true, want false")
	}
	if len(options) != 2 {
		t.Fatalf("len(options) = %d, want 2", len(options))
	}
	if allowEmptyOptions {
		t.Fatalf("allowEmptyOptions = true, want false")
	}
}

func TestParseAskArgs_ParsesQuestionsItemDetail(t *testing.T) {
	question, detail, multi, options, allowEmptyOptions := parseAskArgs(map[string]interface{}{
		"questions": []interface{}{
			map[string]interface{}{
				"question": "Select mode",
				"detail":   "❕ 该选择会影响后续执行速度",
				"type":     "checkbox",
				"options":  []interface{}{"Fast", "Safe"},
			},
		},
	})
	if question != "Select mode" {
		t.Fatalf("question = %q, want %q", question, "Select mode")
	}
	if detail != "❕ 该选择会影响后续执行速度" {
		t.Fatalf("detail = %q, want expected text", detail)
	}
	if !multi {
		t.Fatalf("multi = false, want true")
	}
	if len(options) != 2 {
		t.Fatalf("len(options) = %d, want 2", len(options))
	}
	if allowEmptyOptions {
		t.Fatalf("allowEmptyOptions = true, want false")
	}
}

func TestParseAskArgs_AllowsTextQuestionsWithoutOptions(t *testing.T) {
	question, detail, multi, options, allowEmptyOptions := parseAskArgs(map[string]interface{}{
		"questions": []interface{}{
			map[string]interface{}{
				"question": "请假时长？",
				"type":     "text",
				"detail":   "直接填写数字或日期范围",
			},
		},
	})
	if question != "请假时长？" {
		t.Fatalf("question = %q, want expected text", question)
	}
	if detail != "直接填写数字或日期范围" {
		t.Fatalf("detail = %q, want expected text", detail)
	}
	if multi {
		t.Fatalf("multi = true, want false")
	}
	if len(options) != 0 {
		t.Fatalf("len(options) = %d, want 0", len(options))
	}
	if !allowEmptyOptions {
		t.Fatalf("allowEmptyOptions = false, want true")
	}
}

func TestAskExecute_AcceptsTextQuestionWithoutOptions(t *testing.T) {
	mgr := NewQuestionManager(nil, func() bool { return true }, 0)
	tool := NewAskTool(mgr)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"questions": []interface{}{
			map[string]interface{}{
				"question": "请假时长？",
				"type":     "text",
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	outStr, ok := raw.(string)
	if !ok {
		t.Fatalf("Execute() type = %T, want string", raw)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(outStr), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}

	qa, ok := out["qa"].([]interface{})
	if !ok || len(qa) != 1 {
		t.Fatalf("qa = %#v, want single item", out["qa"])
	}
	first, ok := qa[0].(map[string]interface{})
	if !ok {
		t.Fatalf("qa[0] type = %T, want object", qa[0])
	}
	options, ok := first["o"].([]interface{})
	if !ok {
		t.Fatalf("qa[0].o = %#v, want empty array", first["o"])
	}
	if len(options) != 0 {
		t.Fatalf("qa[0].o len = %d, want 0", len(options))
	}
}
