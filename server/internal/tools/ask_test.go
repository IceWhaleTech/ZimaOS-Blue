package tools

import (
	"context"
	"encoding/json"
	"testing"
)

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
