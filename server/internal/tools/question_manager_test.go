package tools

import (
	"context"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
)

func TestQuestionManager_TimeoutActionDefault(t *testing.T) {
	mgr := NewQuestionManager(sse.NewBroker(), func() bool { return false }, 20*time.Millisecond)
	mgr.SetTimeoutActionFunc(func() string { return "default" })

	questions := []QuestionItem{{
		ID:       "q1",
		Question: "Pick one",
		Options:  []QuestionOption{{Label: "A", Value: "a"}, {Label: "B", Value: "b"}},
	}}

	ans, silent, err := mgr.AskQuestions(context.Background(), "u1", "s1", questions)
	if err != nil {
		t.Fatalf("AskQuestions error: %v", err)
	}
	if !silent {
		t.Fatal("expected silent=true when timeout falls back to default answers")
	}
	if len(ans) != 1 || len(ans[0].Selected) != 1 || ans[0].Selected[0] != "a" {
		t.Fatalf("unexpected default answers: %+v", ans)
	}
}

func TestQuestionManager_TimeoutActionError(t *testing.T) {
	mgr := NewQuestionManager(sse.NewBroker(), func() bool { return false }, 20*time.Millisecond)
	mgr.SetTimeoutActionFunc(func() string { return "error" })

	questions := []QuestionItem{{
		ID:       "q1",
		Question: "Pick one",
		Options:  []QuestionOption{{Label: "A", Value: "a"}, {Label: "B", Value: "b"}},
	}}

	ans, silent, err := mgr.AskQuestions(context.Background(), "u1", "s1", questions)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if silent {
		t.Fatal("silent should be false on timeout error mode")
	}
	if ans != nil {
		t.Fatalf("answers should be nil on timeout error mode, got %+v", ans)
	}
}

func TestQuestionManager_DynamicTimeoutOverride(t *testing.T) {
	mgr := NewQuestionManager(sse.NewBroker(), func() bool { return false }, 2*time.Minute)
	mgr.SetTimeoutFunc(func() time.Duration { return 25 * time.Millisecond })

	questions := []QuestionItem{{
		ID:       "q1",
		Question: "Pick one",
		Options:  []QuestionOption{{Label: "A", Value: "a"}},
	}}

	start := time.Now()
	_, _, _ = mgr.AskQuestions(context.Background(), "u1", "s1", questions)
	elapsed := time.Since(start)
	if elapsed > 300*time.Millisecond {
		t.Fatalf("dynamic timeout override not applied, elapsed=%s", elapsed)
	}
}
