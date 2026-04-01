package agentcore

import (
	"context"
	"testing"
)

func TestAutoSkillReranker_NilReceiverFallsBackToHeuristic(t *testing.T) {
	var reranker *AutoSkillReranker

	reranker.SetSwitchFuncs(func() bool { return true }, func() bool { return true })
	reranker.WarmupAsync()

	if mgr := reranker.ModelManager(); mgr != nil {
		t.Fatalf("expected nil model manager for nil reranker receiver, got %#v", mgr)
	}

	result, err := reranker.Rerank(context.Background(), "calendar reminders", []SkillDoc{
		{
			Name:        "calendar",
			Description: "Manage reminders and calendar events.",
			Tags:        []string{"calendar", "reminder"},
		},
	})
	if err != nil {
		t.Fatalf("expected heuristic fallback to handle nil reranker receiver, got error %v", err)
	}
	if result.SelectedSkill != "calendar" {
		t.Fatalf("selected skill = %q, want %q", result.SelectedSkill, "calendar")
	}
}

func TestAutoSkillReranker_ZeroValueFallsBackToHeuristic(t *testing.T) {
	reranker := &AutoSkillReranker{}

	result, err := reranker.Rerank(context.Background(), "calendar reminders", []SkillDoc{
		{
			Name:        "calendar",
			Description: "Manage reminders and calendar events.",
			Tags:        []string{"calendar", "reminder"},
		},
	})
	if err != nil {
		t.Fatalf("expected zero-value reranker to fall back to heuristic, got error %v", err)
	}
	if result.SelectedSkill != "calendar" {
		t.Fatalf("selected skill = %q, want %q", result.SelectedSkill, "calendar")
	}
}
