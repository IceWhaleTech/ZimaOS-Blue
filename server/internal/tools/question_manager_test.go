package tools

import (
	"context"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

func TestQuestionManager_TimeoutActionDefault(t *testing.T) {
	broker := sse.NewBroker()
	ch := broker.Subscribe("u1")
	defer broker.Unsubscribe("u1", ch)
	mgr := NewQuestionManager(broker, func() bool { return false }, 20*time.Millisecond)
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

func TestQuestionManager_ObserverReceivesLifecycleEvents(t *testing.T) {
	broker := sse.NewBroker()
	ch := broker.Subscribe("u1")
	defer broker.Unsubscribe("u1", ch)
	mgr := NewQuestionManager(broker, func() bool { return false }, 2*time.Second)
	observer := &runtimeObserverStub{}
	mgr.SetObserver(observer)

	ctx := WithRunID(context.Background(), "run-q1")
	ctx = WithRunStep(ctx, 3)
	questions := []QuestionItem{{
		ID:       "q1",
		Question: "Pick one",
		Options:  []QuestionOption{{Label: "A", Value: "a"}, {Label: "B", Value: "b"}},
	}}

	done := make(chan []QuestionAnswerResult, 1)
	go func() {
		answers, _, err := mgr.AskQuestions(ctx, "u1", "s1", questions)
		if err != nil {
			t.Errorf("AskQuestions error: %v", err)
			done <- nil
			return
		}
		done <- answers
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if pending := mgr.GetPending("u1"); pending != nil {
			if !mgr.ResolveAnswer(pending.ID, []QuestionAnswerResult{{QuestionID: "q1", Selected: []string{"b"}}}) {
				t.Fatal("ResolveAnswer returned false")
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	select {
	case answers := <-done:
		if len(answers) != 1 || len(answers[0].Selected) != 1 || answers[0].Selected[0] != "b" {
			t.Fatalf("unexpected answers: %+v", answers)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for AskQuestions")
	}

	if len(observer.questionRequested) != 1 {
		t.Fatalf("questionRequested len = %d, want 1", len(observer.questionRequested))
	}
	if len(observer.questionResolved) != 1 {
		t.Fatalf("questionResolved len = %d, want 1", len(observer.questionResolved))
	}
	if observer.questionRequested[0].RunID != "run-q1" || observer.questionResolved[0].RunID != "run-q1" {
		t.Fatalf("unexpected run ids: req=%q res=%q", observer.questionRequested[0].RunID, observer.questionResolved[0].RunID)
	}
	if observer.questionResolved[0].TimedOut {
		t.Fatal("question should not be marked timed out")
	}
}

func TestQuestionManager_TimeoutActionError(t *testing.T) {
	broker := sse.NewBroker()
	ch := broker.Subscribe("u1")
	defer broker.Unsubscribe("u1", ch)
	mgr := NewQuestionManager(broker, func() bool { return false }, 20*time.Millisecond)
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
	broker := sse.NewBroker()
	ch := broker.Subscribe("u1")
	defer broker.Unsubscribe("u1", ch)
	mgr := NewQuestionManager(broker, func() bool { return false }, 2*time.Minute)
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

func TestQuestionManager_NoActiveClient_DefaultReturnsImmediately(t *testing.T) {
	mgr := NewQuestionManager(sse.NewBroker(), func() bool { return false }, 2*time.Minute)
	mgr.SetTimeoutActionFunc(func() string { return "default" })

	questions := []QuestionItem{{
		ID:       "q1",
		Question: "Pick one",
		Options:  []QuestionOption{{Label: "A", Value: "a"}, {Label: "B", Value: "b"}},
	}}

	start := time.Now()
	ans, silent, err := mgr.AskQuestions(context.Background(), "u1", "s1", questions)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("AskQuestions error: %v", err)
	}
	if !silent {
		t.Fatal("expected silent=true when no active SSE client")
	}
	if len(ans) != 1 || len(ans[0].Selected) != 1 || ans[0].Selected[0] != "a" {
		t.Fatalf("unexpected default answers: %+v", ans)
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("expected immediate fallback without timeout wait, elapsed=%s", elapsed)
	}
}

func TestQuestionManager_NoActiveClient_ErrorReturnsImmediately(t *testing.T) {
	mgr := NewQuestionManager(sse.NewBroker(), func() bool { return false }, 2*time.Minute)
	mgr.SetTimeoutActionFunc(func() string { return "error" })

	questions := []QuestionItem{{
		ID:       "q1",
		Question: "Pick one",
		Options:  []QuestionOption{{Label: "A", Value: "a"}, {Label: "B", Value: "b"}},
	}}

	start := time.Now()
	ans, silent, err := mgr.AskQuestions(context.Background(), "u1", "s1", questions)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected immediate delivery error when no active SSE client")
	}
	if silent {
		t.Fatal("silent should be false on delivery error")
	}
	if ans != nil {
		t.Fatalf("answers should be nil on delivery error, got %+v", ans)
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("expected immediate error without timeout wait, elapsed=%s", elapsed)
	}
}

func TestQuestionManager_GetPending_StrictUserMatchAndNewest(t *testing.T) {
	mgr := NewQuestionManager(sse.NewBroker(), func() bool { return false }, 2*time.Minute)
	now := time.Now()
	nowMs := timeutil.NowMilli()

	mgr.pending["old"] = &pendingQuestion{
		request: QuestionRequest{ID: "old", UserID: "u1", SessionID: "s1", ExpiresAt: nowMs + 60_000},
		created: now.Add(-2 * time.Second),
	}
	mgr.pending["new"] = &pendingQuestion{
		request: QuestionRequest{ID: "new", UserID: "u1", SessionID: "s2", ExpiresAt: nowMs + 60_000},
		created: now,
	}
	mgr.pending["other"] = &pendingQuestion{
		request: QuestionRequest{ID: "other", UserID: "u2", SessionID: "s3", ExpiresAt: nowMs + 60_000},
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
	nowMs := timeutil.NowMilli()

	mgr.pending["s-old"] = &pendingQuestion{
		request: QuestionRequest{ID: "s-old", UserID: "u1", SessionID: "conv-1", ExpiresAt: nowMs + 60_000},
		created: now.Add(-3 * time.Second),
	}
	mgr.pending["s-new"] = &pendingQuestion{
		request: QuestionRequest{ID: "s-new", UserID: "u2", SessionID: "conv-1", ExpiresAt: nowMs + 60_000},
		created: now,
	}
	mgr.pending["other"] = &pendingQuestion{
		request: QuestionRequest{ID: "other", UserID: "u3", SessionID: "conv-2", ExpiresAt: nowMs + 60_000},
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

func TestQuestionManager_CleanupExpired(t *testing.T) {
	mgr := NewQuestionManager(sse.NewBroker(), func() bool { return false }, 2*time.Minute)
	nowMs := timeutil.NowMilli()

	mgr.pending["expired"] = &pendingQuestion{
		request: QuestionRequest{ID: "expired", UserID: "u1", SessionID: "s1", ExpiresAt: nowMs - 1},
		created: time.Now().Add(-2 * time.Second),
	}
	mgr.pending["active"] = &pendingQuestion{
		request: QuestionRequest{ID: "active", UserID: "u1", SessionID: "s1", ExpiresAt: nowMs + 60_000},
		created: time.Now(),
	}

	removed := mgr.CleanupExpired()
	if len(removed) != 1 || removed[0] != "expired" {
		t.Fatalf("CleanupExpired() removed = %+v, want [expired]", removed)
	}
	if got := mgr.GetPending("u1"); got == nil || got.ID != "active" {
		t.Fatalf("GetPending(u1) after cleanup = %+v, want active", got)
	}
}

func TestQuestionManager_GetPending_SkipsExpiredEntries(t *testing.T) {
	mgr := NewQuestionManager(sse.NewBroker(), func() bool { return false }, 2*time.Minute)
	nowMs := timeutil.NowMilli()
	now := time.Now()

	mgr.pending["expired-new"] = &pendingQuestion{
		request: QuestionRequest{ID: "expired-new", UserID: "u1", SessionID: "s1", ExpiresAt: nowMs - 1},
		created: now.Add(2 * time.Second),
	}
	mgr.pending["active-old"] = &pendingQuestion{
		request: QuestionRequest{ID: "active-old", UserID: "u1", SessionID: "s1", ExpiresAt: nowMs + 60_000},
		created: now,
	}

	got := mgr.GetPending("u1")
	if got == nil || got.ID != "active-old" {
		t.Fatalf("GetPending(u1) = %+v, want active-old", got)
	}
	if _, exists := mgr.pending["expired-new"]; exists {
		t.Fatal("expired entry should be removed during GetPending")
	}
}

func TestQuestionManager_GetPendingBySession_SkipsExpiredEntries(t *testing.T) {
	mgr := NewQuestionManager(sse.NewBroker(), func() bool { return false }, 2*time.Minute)
	nowMs := timeutil.NowMilli()
	now := time.Now()

	mgr.pending["expired-new"] = &pendingQuestion{
		request: QuestionRequest{ID: "expired-new", UserID: "u1", SessionID: "conv-1", ExpiresAt: nowMs - 1},
		created: now.Add(2 * time.Second),
	}
	mgr.pending["active-old"] = &pendingQuestion{
		request: QuestionRequest{ID: "active-old", UserID: "u2", SessionID: "conv-1", ExpiresAt: nowMs + 60_000},
		created: now,
	}

	got := mgr.GetPendingBySession("conv-1")
	if got == nil || got.ID != "active-old" {
		t.Fatalf("GetPendingBySession(conv-1) = %+v, want active-old", got)
	}
	if _, exists := mgr.pending["expired-new"]; exists {
		t.Fatal("expired entry should be removed during GetPendingBySession")
	}
}

func TestQuestionManager_AskQuestionsWithContext_PersistsContext(t *testing.T) {
	broker := sse.NewBroker()
	ch := broker.Subscribe("u1")
	defer broker.Unsubscribe("u1", ch)
	mgr := NewQuestionManager(broker, func() bool { return false }, 2*time.Minute)

	ctx := context.Background()
	questions := []QuestionItem{{
		ID:       "q1",
		Question: "Continue?",
		Options:  []QuestionOption{{Label: "Continue", Value: "continue"}, {Label: "Cancel", Value: "cancel"}},
	}}
	contextPayload := map[string]interface{}{
		"kind":       "browser_checkpoint",
		"required":   true,
		"risk_level": "high",
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _, _ = mgr.AskQuestionsWithContext(ctx, "u1", "s1", questions, contextPayload)
	}()

	// Wait briefly for pending request to be created.
	time.Sleep(20 * time.Millisecond)
	req := mgr.GetPending("u1")
	if req == nil {
		t.Fatal("expected pending question request")
	}
	if req.Context == nil {
		t.Fatal("expected context payload in question request")
	}
	if got, _ := req.Context["kind"].(string); got != "browser_checkpoint" {
		t.Fatalf("unexpected context kind: %q", got)
	}
	if !mgr.ResolveAnswer(req.ID, []QuestionAnswerResult{{QuestionID: "q1", Selected: []string{"continue"}}}) {
		t.Fatal("expected resolve answer to succeed")
	}
	<-done
}
