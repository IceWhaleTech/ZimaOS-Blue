package builtin

import "testing"

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
