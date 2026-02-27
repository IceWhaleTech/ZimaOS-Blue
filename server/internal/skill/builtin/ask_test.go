package builtin

import "testing"

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

func TestParseAskQuestions_SQIsRejected(t *testing.T) {
	_, err := parseAskQuestions(map[string]any{
		"sq": "Pick one",
		"a":  `["A","B"]`,
	})
	if err == nil {
		t.Fatalf("expected error for sq input, got nil")
	}
}
