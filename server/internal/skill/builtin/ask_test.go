package builtin

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type captureAskQuestioner struct {
	lastUserID    string
	lastSessionID string
	lastQuestions []AskQuestionItem
	answers       []AskQuestionAnswerResult
	silent        bool
	err           error
}

func (c *captureAskQuestioner) AskQuestions(_ context.Context, userID, sessionID string, questions []AskQuestionItem) ([]AskQuestionAnswerResult, bool, error) {
	c.lastUserID = userID
	c.lastSessionID = sessionID
	c.lastQuestions = append([]AskQuestionItem(nil), questions...)
	return c.answers, c.silent, c.err
}

func TestAskExecute_PassesSessionIDToQuestioner(t *testing.T) {
	a := NewAsk()
	mock := &captureAskQuestioner{
		answers: []AskQuestionAnswerResult{{
			QuestionID: "q0",
			Selected:   []string{"continue"},
		}},
	}
	a.SetQuestioner(mock)

	ctx := tools.WithUserID(context.Background(), "user-123")
	ctx = tools.WithSessionID(ctx, "conv-456")

	res, err := a.Execute(ctx, map[string]any{
		"q": "continue?",
		"a": []any{"continue", "cancel"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !res.Success {
		t.Fatalf("Execute returned unsuccessful result: %+v", res)
	}
	if mock.lastUserID != "user-123" {
		t.Fatalf("user id = %q, want %q", mock.lastUserID, "user-123")
	}
	if mock.lastSessionID != "conv-456" {
		t.Fatalf("session id = %q, want %q", mock.lastSessionID, "conv-456")
	}
}

func TestAskValidate_AcceptsQuestionsArray(t *testing.T) {
	a := NewAsk()
	err := a.Validate(map[string]any{
		"questions": []any{
			map[string]any{
				"question": "How should I answer?",
				"type":     "checkbox",
				"options": []any{
					map[string]any{"label": "自由回答", "value": "free"},
					map[string]any{"label": "模板回答", "value": "template"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestAskExecute_AcceptsStringQuestionsArrayWithTopLevelOptionGroups(t *testing.T) {
	a := NewAsk()
	mock := &captureAskQuestioner{
		answers: []AskQuestionAnswerResult{
			{QuestionID: "q0", Selected: []string{"开发环境"}},
			{QuestionID: "q1", Selected: []string{"Go"}},
			{QuestionID: "q2", Selected: []string{"简洁回答"}},
		},
	}
	a.SetQuestioner(mock)

	res, err := a.Execute(context.Background(), map[string]any{
		"questions": `["你主要用 ZimaOS 做什么？","你最常用的编程语言是？","有没有希望我记住的工作习惯或偏好？"]`,
		"options":   `[["存储备份","媒体中心","开发环境","其他"],["Python","JavaScript/TypeScript","Go","Rust","其他"],["简洁回答","详细解释","多给代码示例","其他"]]`,
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !res.Success {
		t.Fatalf("Execute returned unsuccessful result: %+v", res)
	}
	if got := len(mock.lastQuestions); got != 3 {
		t.Fatalf("question count = %d, want 3", got)
	}
	if mock.lastQuestions[1].Question != "你最常用的编程语言是？" {
		t.Fatalf("question[1] = %q", mock.lastQuestions[1].Question)
	}
	if got := len(mock.lastQuestions[2].Options); got != 4 {
		t.Fatalf("question[2] options len = %d, want 4", got)
	}
	selected, _ := res.Data.(map[string]any)["selected"].([]string)
	if len(selected) != 1 || selected[0] != "开发环境" {
		t.Fatalf("selected = %#v, want first answer", selected)
	}
}

func TestParseAskQuestions_AcceptsOptionAlias(t *testing.T) {
	items, err := parseAskQuestions(map[string]any{
		"question": "Which topic?",
		"option":   "A,B,C",
	})
	if err != nil {
		t.Fatalf("parseAskQuestions returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	if got := len(items[0].Options); got != 3 {
		t.Fatalf("options len = %d, want 3", got)
	}
}

func TestParseAskQuestions_QIsSingleSelect(t *testing.T) {
	items, err := parseAskQuestions(map[string]any{
		"q": "Pick one",
		"a": `["A","B"]`,
	})
	if err != nil {
		t.Fatalf("parseAskQuestions returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	if items[0].MultiSelect {
		t.Fatalf("MultiSelect = true, want false")
	}
}

func TestParseAskQuestions_QAllowsSingleOption(t *testing.T) {
	items, err := parseAskQuestions(map[string]any{
		"q": "Pick one",
		"a": `["OnlyOne"]`,
	})
	if err != nil {
		t.Fatalf("parseAskQuestions returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	if got := len(items[0].Options); got != 1 {
		t.Fatalf("options len = %d, want 1", got)
	}
}

func TestParseAskQuestions_QSupportsObjectOptionsJSON(t *testing.T) {
	items, err := parseAskQuestions(map[string]any{
		"q": "First priority?",
		"a": `[{"label":"Low error rate (Recommended)","description":"Most stable","value":"stable"},{"label":"Lowest latency","value":"fast"}]`,
	})
	if err != nil {
		t.Fatalf("parseAskQuestions returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	if got := len(items[0].Options); got != 2 {
		t.Fatalf("options len = %d, want 2", got)
	}
	if items[0].Options[0].Label != "Low error rate (Recommended)" {
		t.Fatalf("option[0].Label = %q", items[0].Options[0].Label)
	}
	if items[0].Options[0].Description != "Most stable" {
		t.Fatalf("option[0].Description = %q", items[0].Options[0].Description)
	}
	if items[0].Options[0].Value != "stable" {
		t.Fatalf("option[0].Value = %q", items[0].Options[0].Value)
	}
}

func TestParseAskQuestions_QSupportsSingleQuotedObjectJSON(t *testing.T) {
	items, err := parseAskQuestions(map[string]any{
		"q": "First priority?",
		"a": `'[{"label":"Stable","value":"stable"},{"label":"Fast","value":"fast"}]'`,
	})
	if err != nil {
		t.Fatalf("parseAskQuestions returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	if got := len(items[0].Options); got != 2 {
		t.Fatalf("options len = %d, want 2", got)
	}
	if items[0].Options[0].Label != "Stable" || items[0].Options[0].Value != "stable" {
		t.Fatalf("option[0] = %+v", items[0].Options[0])
	}
}

func TestParseAskQuestions_QuestionsSupportsObjectOptions(t *testing.T) {
	items, err := parseAskQuestions(map[string]any{
		"questions": `[{"question":"How should I answer?","type":"checkbox","options":[{"label":"Use free-form answer (Recommended)","description":"Directly answer 1-4","value":"free"},{"label":"Use template","value":"template"}]}]`,
	})
	if err != nil {
		t.Fatalf("parseAskQuestions returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	if !items[0].MultiSelect {
		t.Fatalf("MultiSelect = false, want true")
	}
	if got := len(items[0].Options); got != 2 {
		t.Fatalf("options len = %d, want 2", got)
	}
	if items[0].Options[0].Label != "Use free-form answer (Recommended)" {
		t.Fatalf("option[0].Label = %q", items[0].Options[0].Label)
	}
	if items[0].Options[0].Description != "Directly answer 1-4" {
		t.Fatalf("option[0].Description = %q", items[0].Options[0].Description)
	}
}

func TestParseAskQuestions_QuestionsNativeArray(t *testing.T) {
	items, err := parseAskQuestions(map[string]any{
		"questions": []any{
			map[string]any{
				"question": "How should I answer?",
				"type":     "checkbox",
				"options": []any{
					map[string]any{
						"label":       "Use free-form answer (Recommended)",
						"description": "Directly answer 1-4",
						"value":       "free",
					},
					map[string]any{
						"label": "Use template",
						"value": "template",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("parseAskQuestions returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	if !items[0].MultiSelect {
		t.Fatalf("MultiSelect = false, want true")
	}
	if got := len(items[0].Options); got != 2 {
		t.Fatalf("options len = %d, want 2", got)
	}
	if items[0].Options[0].Label != "Use free-form answer (Recommended)" {
		t.Fatalf("option[0].Label = %q", items[0].Options[0].Label)
	}
	if items[0].Options[0].Description != "Directly answer 1-4" {
		t.Fatalf("option[0].Description = %q", items[0].Options[0].Description)
	}
}

func TestParseAskQuestions_StringQuestionsArrayUsesTopLevelOptionGroups(t *testing.T) {
	items, err := parseAskQuestions(map[string]any{
		"questions": `["你主要用 ZimaOS 做什么？","你最常用的编程语言是？","有没有希望我记住的工作习惯或偏好？"]`,
		"options":   `[["存储备份","媒体中心","开发环境","其他"],["Python","JavaScript/TypeScript","Go","Rust","其他"],["简洁回答","详细解释","多给代码示例","其他"]]`,
	})
	if err != nil {
		t.Fatalf("parseAskQuestions returned error: %v", err)
	}
	if got := len(items); got != 3 {
		t.Fatalf("items len = %d, want 3", got)
	}
	if items[0].Question != "你主要用 ZimaOS 做什么？" {
		t.Fatalf("question[0] = %q", items[0].Question)
	}
	if got := len(items[1].Options); got != 5 {
		t.Fatalf("question[1] options len = %d, want 5", got)
	}
	if items[2].Options[2].Label != "多给代码示例" {
		t.Fatalf("question[2] option[2] label = %q", items[2].Options[2].Label)
	}
	if items[2].MultiSelect {
		t.Fatalf("question[2] MultiSelect = true, want false")
	}
}

func TestParseAskQuestions_StringQuestionsArrayRejectsLengthMismatch(t *testing.T) {
	_, err := parseAskQuestions(map[string]any{
		"questions": `["Q1","Q2"]`,
		"options":   `[["A","B"]]`,
	})
	if err == nil {
		t.Fatalf("expected mismatch error, got nil")
	}
	if err.Error() != "questions/options length mismatch: 2 questions, 1 option groups" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseAskQuestions_QuestionsRejectsQAAliases(t *testing.T) {
	_, err := parseAskQuestions(map[string]any{
		"questions": []any{
			map[string]any{
				"q":    "How should I answer?",
				"type": "checkbox",
				"a":    []any{"free", "template"},
			},
		},
	})
	if err == nil {
		t.Fatalf("expected error for q/a aliases in questions")
	}
}

func TestParseAskQuestions_SQIsRejected(t *testing.T) {
	_, err := parseAskQuestions(map[string]any{
		"sq": "Pick one",
		"a":  `["A","B"]`,
	})
	if err == nil {
		t.Fatalf("expected error for sq input, got nil")
	}
}

func TestParseAskQuestions_QSupportsDetail(t *testing.T) {
	items, err := parseAskQuestions(map[string]any{
		"q":      "Choose one",
		"detail": "❕ 这是一条额外说明",
		"a":      `["A","B"]`,
	})
	if err != nil {
		t.Fatalf("parseAskQuestions returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	if items[0].Detail != "❕ 这是一条额外说明" {
		t.Fatalf("detail = %q, want expected text", items[0].Detail)
	}
}

func TestParseAskQuestions_QuestionsSupportsDetail(t *testing.T) {
	items, err := parseAskQuestions(map[string]any{
		"questions": []any{
			map[string]any{
				"question": "Choose mode",
				"detail":   "❕ 会影响后续步骤",
				"type":     "radio",
				"options":  []any{"fast", "safe"},
			},
		},
	})
	if err != nil {
		t.Fatalf("parseAskQuestions returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	if items[0].Detail != "❕ 会影响后续步骤" {
		t.Fatalf("detail = %q, want expected text", items[0].Detail)
	}
}

func TestParseAskQuestions_QuestionsSupportsTextInput(t *testing.T) {
	items, err := parseAskQuestions(map[string]any{
		"questions": []any{
			map[string]any{
				"question": "请假时长？",
				"type":     "text",
			},
		},
	})
	if err != nil {
		t.Fatalf("parseAskQuestions returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	if got := len(items[0].Options); got != 0 {
		t.Fatalf("options len = %d, want 0", got)
	}
	if items[0].MultiSelect {
		t.Fatalf("MultiSelect = true, want false")
	}
}

func TestParseAskQuestions_TopLevelTextQuestionAllowsNoOptions(t *testing.T) {
	items, err := parseAskQuestions(map[string]any{
		"question": "请假时长？",
		"type":     "text",
	})
	if err != nil {
		t.Fatalf("parseAskQuestions returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	if got := len(items[0].Options); got != 0 {
		t.Fatalf("options len = %d, want 0", got)
	}
}
