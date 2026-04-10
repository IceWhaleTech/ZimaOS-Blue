package agent

import (
	"context"
	"strings"
	"testing"
	"time"
)

func waitForTerminalTask(t *testing.T, store *Store, taskID string, timeout time.Duration) *Task {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		task, err := store.Get(context.Background(), taskID)
		if err == nil && task != nil {
			switch task.Status {
			case TaskStatusCompleted, TaskStatusFailed, TaskStatusAborted, TaskStatusCancelled:
				return task
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("task %s did not reach terminal state within %s", taskID, timeout)
	return nil
}

func hasRuntimeTransition(events []RuntimeAuditEvent, from, to RuntimeState) bool {
	for _, ev := range events {
		if ev.From == from && ev.To == to && ev.Error == "" {
			return true
		}
	}
	return false
}

func TestRunnerSmoke_ClarifyTimeoutDefault_Completes(t *testing.T) {
	store := testStore(t)
	llmMock := &mockLLM{planJSON: `[{"description":"implement primary task"}]`}
	runner := NewRunner(store, llmMock, nil, nil, nil, RunnerConfig{
		TaskTimeout:      5 * time.Second,
		AskTimeout:       30 * time.Millisecond,
		AskTimeoutAction: "default",
		MaxConcurrent:    2,
	})
	runner.minAskTimeout = time.Millisecond
	t.Cleanup(func() { runner.Shutdown() })

	task, err := runner.Submit(context.Background(), "u1", "TBD: build feature", "", "")
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	got := waitForTerminalTask(t, store, task.ID, 5*time.Second)
	if got.Status != TaskStatusCompleted {
		t.Fatalf("status=%q, want %q; error=%s", got.Status, TaskStatusCompleted, got.Error)
	}
	if got.RuntimeState != RuntimeStateDone {
		t.Fatalf("runtime_state=%q, want %q", got.RuntimeState, RuntimeStateDone)
	}
	if !hasRuntimeTransition(got.RuntimeAudit, RuntimeStateIntake, RuntimeStateClarify) {
		t.Fatal("expected runtime transition INTAKE -> CLARIFY")
	}
	if !hasRuntimeTransition(got.RuntimeAudit, RuntimeStateClarify, RuntimeStatePlan) {
		t.Fatal("expected runtime transition CLARIFY -> PLAN")
	}
	if !hasRuntimeTransition(got.RuntimeAudit, RuntimeStateVerify, RuntimeStateReport) {
		t.Fatal("expected runtime transition VERIFY -> REPORT")
	}
}

func TestRunnerSmoke_ClarifyTimeoutError_Fails(t *testing.T) {
	store := testStore(t)
	llmMock := &mockLLM{planJSON: `[{"description":"implement primary task"}]`}
	runner := NewRunner(store, llmMock, nil, nil, nil, RunnerConfig{
		TaskTimeout:      5 * time.Second,
		AskTimeout:       25 * time.Millisecond,
		AskTimeoutAction: "error",
		MaxConcurrent:    2,
	})
	runner.minAskTimeout = time.Millisecond
	t.Cleanup(func() { runner.Shutdown() })

	task, err := runner.Submit(context.Background(), "u1", "TBD: choose strategy", "", "")
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	got := waitForTerminalTask(t, store, task.ID, 5*time.Second)
	if got.Status != TaskStatusFailed {
		t.Fatalf("status=%q, want %q", got.Status, TaskStatusFailed)
	}
	if got.RuntimeState != RuntimeStateDone {
		t.Fatalf("runtime_state=%q, want %q", got.RuntimeState, RuntimeStateDone)
	}
	if !strings.Contains(got.Error, "clarify failed") || !strings.Contains(got.Error, "timed out") {
		t.Fatalf("unexpected error message: %q", got.Error)
	}
	if !hasRuntimeTransition(got.RuntimeAudit, RuntimeStateIntake, RuntimeStateClarify) {
		t.Fatal("expected runtime transition INTAKE -> CLARIFY")
	}
	if !hasRuntimeTransition(got.RuntimeAudit, RuntimeStateClarify, RuntimeStateReport) {
		t.Fatal("expected runtime transition CLARIFY -> REPORT")
	}
}

func TestRunnerSmoke_VerifyRecoverFailure_EndsDone(t *testing.T) {
	store := testStore(t)
	llmMock := &scriptedLLM{
		calls: []scriptedLLMCall{
			{content: `{"goal":"build","subtasks":[{"description":"primary step"}],"success_criteria":["verify passes"],"fallback_plan":["recover once"]}`},
			{content: "not-json"},
			{content: "not-json"},
		},
	}
	runner := NewRunner(store, llmMock, nil, nil, nil, RunnerConfig{
		TaskTimeout:   5 * time.Second,
		MaxConcurrent: 2,
	})
	t.Cleanup(func() { runner.Shutdown() })

	task, err := runner.Submit(context.Background(), "u1", "execute and verify", "", "")
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	got := waitForTerminalTask(t, store, task.ID, 5*time.Second)
	if got.Status != TaskStatusFailed {
		t.Fatalf("status=%q, want %q", got.Status, TaskStatusFailed)
	}
	if got.RuntimeState != RuntimeStateDone {
		t.Fatalf("runtime_state=%q, want %q", got.RuntimeState, RuntimeStateDone)
	}

	var verifyFailed, recoverFailed bool
	for _, step := range got.Plan {
		if strings.HasPrefix(step.Description, "Verify the completed work") && step.Status == StepStatusFailed {
			verifyFailed = true
		}
	}
	if !verifyFailed {
		t.Fatal("expected failed verify step in plan")
	}
	if recoverFailed {
		t.Fatal("did not expect recovery step in grounded runtime flow")
	}
	if !hasRuntimeTransition(got.RuntimeAudit, RuntimeStateVerify, RuntimeStateReport) {
		t.Fatal("expected runtime transition VERIFY -> REPORT")
	}
}
