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

func TestQuestionManager_GetPending_StrictUserMatchAndNewest(t *testing.T) {
	mgr := NewQuestionManager(sse.NewBroker(), func() bool { return false }, 2*time.Minute)
	now := time.Now()

	mgr.pending["old"] = &pendingQuestion{
		request: QuestionRequest{ID: "old", UserID: "u1", SessionID: "s1"},
		created: now.Add(-2 * time.Second),
	}
	mgr.pending["new"] = &pendingQuestion{
		request: QuestionRequest{ID: "new", UserID: "u1", SessionID: "s2"},
		created: now,
	}
	mgr.pending["other"] = &pendingQuestion{
		request: QuestionRequest{ID: "other", UserID: "u2", SessionID: "s3"},
		created: now.Add(1 * time.Second),
	}

	got := mgr.GetPending("u1")
	if got == nil {
		t.Fatal("GetPending(u1) = nil, want non-nil")
	}
	if got.ID != "new" {
		t.Fatalf("GetPending(u1).ID = %q, want %q", got.ID, "new")
	}
	if got := mgr.GetPending("u3"); got != nil {
		t.Fatalf("GetPending(u3) = %+v, want nil", got)
	}
}

func TestQuestionManager_GetPendingBySession_ReturnsNewest(t *testing.T) {
	mgr := NewQuestionManager(sse.NewBroker(), func() bool { return false }, 2*time.Minute)
	now := time.Now()

	mgr.pending["s-old"] = &pendingQuestion{
		request: QuestionRequest{ID: "s-old", UserID: "u1", SessionID: "conv-1"},
		created: now.Add(-3 * time.Second),
	}
	mgr.pending["s-new"] = &pendingQuestion{
		request: QuestionRequest{ID: "s-new", UserID: "u2", SessionID: "conv-1"},
		created: now,
	}
	mgr.pending["other"] = &pendingQuestion{
		request: QuestionRequest{ID: "other", UserID: "u3", SessionID: "conv-2"},
		created: now.Add(1 * time.Second),
	}

	got := mgr.GetPendingBySession("conv-1")
	if got == nil {
		t.Fatal("GetPendingBySession(conv-1) = nil, want non-nil")
	}
	if got.ID != "s-new" {
		t.Fatalf("GetPendingBySession(conv-1).ID = %q, want %q", got.ID, "s-new")
	}
	if got := mgr.GetPendingBySession("missing"); got != nil {
		t.Fatalf("GetPendingBySession(missing) = %+v, want nil", got)
	}
}
