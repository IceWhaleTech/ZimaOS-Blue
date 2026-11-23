package agent

import (
	"context"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
)

type budgetCheckingReflector struct {
	called      bool
	deadline    time.Time
	hasDeadline bool
}

func (r *budgetCheckingReflector) Reflect(ctx context.Context, _ selfreflect.Input) (*selfreflect.Result, error) {
	r.called = true
	r.deadline, r.hasDeadline = ctx.Deadline()
	return &selfreflect.Result{Summary: "optional lessons"}, nil
}

func TestReflectionUsesIndependentShortBudget(t *testing.T) {
	store := testStore(t)
	reflector := &budgetCheckingReflector{}
	runner := NewRunner(store, &scriptedLLM{calls: []scriptedLLMCall{{content: `{"goal":"finish","subtasks":[{"description":"finish"}],"success_criteria":["verification passes"]}`}}}, nil, nil, nil, RunnerConfig{TaskTimeout: time.Minute, AutoReflect: true})
	withCompletedGroundedWork(t, runner)
	runner.SetReflector(reflector)
	t.Cleanup(runner.Shutdown)
	task, err := runner.Submit(context.Background(), "u1", "finish", "", "")
	if err != nil {
		t.Fatal(err)
	}
	got := waitForTerminalTask(t, store, task.ID, 5*time.Second)
	if got.Status != TaskStatusCompleted {
		t.Fatalf("status=%s", got.Status)
	}
	if !reflector.called || !reflector.hasDeadline || time.Until(reflector.deadline) > 10*time.Second {
		t.Fatal("optional reflection inherited the whole task budget")
	}
}

func TestReflectionReservesTimeForVerifiedResultFinalization(t *testing.T) {
	store := testStore(t)
	reflector := &budgetCheckingReflector{}
	runner := NewRunner(store, nil, nil, nil, nil, RunnerConfig{AutoReflect: true})
	runner.SetReflector(reflector)
	t.Cleanup(runner.Shutdown)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	task := &Task{ID: "verified", UserID: "u1", Result: "Actual source-backed answer", Status: TaskStatusExecuting}
	runner.runReflection(ctx, task, TaskStatusCompleted, "")
	if reflector.called {
		t.Fatal("optional reflection consumed the finalization reserve")
	}
	if ctx.Err() != nil {
		t.Fatal("reflection exhausted parent deadline")
	}
	if len(task.Plan) != 0 {
		t.Fatal("skipped optional reflection created unfinished work")
	}
}

type waitingBudgetReflector struct{ started chan struct{} }

func (r *waitingBudgetReflector) Reflect(ctx context.Context, _ selfreflect.Input) (*selfreflect.Result, error) {
	if r.started != nil {
		close(r.started)
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestSlowOptionalReflectionCannotCancelVerifiedCompletion(t *testing.T) {
	store := testStore(t)
	runner := NewRunner(store, &scriptedLLM{calls: []scriptedLLMCall{{content: `{"goal":"finish","subtasks":[{"description":"finish"}],"success_criteria":["verification passes"]}`}}}, nil, nil, nil, RunnerConfig{TaskTimeout: 1500 * time.Millisecond, AutoReflect: true})
	withCompletedGroundedWork(t, runner)
	runner.SetReflector(&waitingBudgetReflector{})
	t.Cleanup(runner.Shutdown)
	task, err := runner.Submit(context.Background(), "u1", "finish", "", "")
	if err != nil {
		t.Fatal(err)
	}
	got := waitForTerminalTask(t, store, task.ID, 3*time.Second)
	if got.Status != TaskStatusCompleted {
		t.Fatalf("verified result became %s while capturing optional lessons", got.Status)
	}
}

func TestExplicitCancellationDuringReflectionRemainsCancellation(t *testing.T) {
	store := testStore(t)
	runner := NewRunner(store, &scriptedLLM{calls: []scriptedLLMCall{{content: `{"goal":"finish","subtasks":[{"description":"finish"}],"success_criteria":["verification passes"]}`}}}, nil, nil, nil, RunnerConfig{TaskTimeout: time.Minute, AutoReflect: true})
	started := make(chan struct{})
	withCompletedGroundedWork(t, runner)
	runner.SetReflector(&waitingBudgetReflector{started: started})
	t.Cleanup(runner.Shutdown)
	task, err := runner.Submit(context.Background(), "u1", "finish", "", "")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("reflection never started")
	}
	if !runner.Cancel(task.ID) {
		t.Fatal("explicit cancellation was rejected")
	}
	got := waitForTerminalTask(t, store, task.ID, time.Second)
	if got.Status != TaskStatusCancelled {
		t.Fatalf("explicit cancellation became %s", got.Status)
	}
}
