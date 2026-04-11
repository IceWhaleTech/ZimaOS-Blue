package builtin

import (
	"context"
	"testing"
)

type mockAdvisorExecutor struct {
	lastArgs map[string]interface{}
}

func (m *mockAdvisorExecutor) Execute(_ context.Context, args map[string]interface{}) (interface{}, error) {
	m.lastArgs = make(map[string]interface{}, len(args))
	for k, v := range args {
		m.lastArgs[k] = v
	}
	return map[string]any{
		"question":  args["question"],
		"category":  args["category"],
		"grounding": args["grounding"],
		"depth":     args["depth"],
		"output":    args["output"],
	}, nil
}

func TestAdvisorSkill_ValidateAcceptsQuestionAliasesAndDefaults(t *testing.T) {
	s := NewAdvisor()
	input := map[string]any{
		"query": "Go vs Python",
	}

	if err := s.Validate(input); err != nil {
		t.Fatalf("expected alias to pass validation: %v", err)
	}
	if got, _ := input["question"].(string); got != "Go vs Python" {
		t.Fatalf("question = %q, want promoted query", got)
	}
	if got, _ := input["grounding"].(string); got != "auto" {
		t.Fatalf("grounding = %q, want auto", got)
	}
	if got, _ := input["depth"].(string); got != "standard" {
		t.Fatalf("depth = %q, want standard", got)
	}
	if got, _ := input["output"].(string); got != "decision_memo" {
		t.Fatalf("output = %q, want decision_memo", got)
	}
}

func TestAdvisorSkill_ExecuteForwardsNormalizedArgs(t *testing.T) {
	exec := &mockAdvisorExecutor{}
	s := NewAdvisor()
	s.SetExecutor(exec)

	res, err := s.Execute(context.Background(), map[string]any{
		"query":    "OnlyOffice vs LibreOffice",
		"category": "library",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got error=%s", res.Error)
	}
	if got, _ := exec.lastArgs["question"].(string); got != "OnlyOffice vs LibreOffice" {
		t.Fatalf("question = %q, want normalized alias", got)
	}
	if got, _ := exec.lastArgs["category"].(string); got != "library" {
		t.Fatalf("category = %q, want library", got)
	}
	if got, _ := exec.lastArgs["grounding"].(string); got != "auto" {
		t.Fatalf("grounding = %q, want auto", got)
	}
}
