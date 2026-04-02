package harness

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type completingSubagentDriver struct {
	controller *Controller
	kind       RunKind
	result     string
}

func (d *completingSubagentDriver) Kind() RunKind { return d.kind }

func (d *completingSubagentDriver) Validate(spec RunSpec) error {
	if spec.Goal == "" {
		return fmt.Errorf("goal is required")
	}
	return nil
}

func (d *completingSubagentDriver) Start(_ context.Context, run *Run, _ RunEnv) error {
	go func(runID string) {
		time.Sleep(20 * time.Millisecond)
		snapshot, err := d.controller.GetStored(context.Background(), runID)
		if err != nil {
			return
		}
		now := timeutil.NowTime()
		snapshot.Status = RunStatusCompleted
		snapshot.Result = d.result
		snapshot.UpdatedAt = now
		snapshot.StartedAt = &now
		snapshot.FinishedAt = &now
		_ = d.controller.SyncSnapshot(context.Background(), snapshot)
	}(run.ID)
	return nil
}

func (d *completingSubagentDriver) Cancel(_ context.Context, _ *Run) error { return nil }

type blockingSubagentDriver struct {
	kind      RunKind
	started   []string
	cancelled []string
}

func (d *blockingSubagentDriver) Kind() RunKind { return d.kind }

func (d *blockingSubagentDriver) Validate(spec RunSpec) error {
	if spec.Goal == "" {
		return fmt.Errorf("goal is required")
	}
	return nil
}

func (d *blockingSubagentDriver) Start(_ context.Context, run *Run, _ RunEnv) error {
	d.started = append(d.started, run.ID)
	return nil
}

func (d *blockingSubagentDriver) Cancel(_ context.Context, run *Run) error {
	d.cancelled = append(d.cancelled, run.ID)
	return nil
}

func TestHarnessSubagentExecutorSpawnsAndWaits(t *testing.T) {
	controller := newTestController(t)
	parentDriver := &stubDriver{kind: RunKindAgentTask}
	childDriver := &completingSubagentDriver{
		controller: controller,
		kind:       RunKindSubagent,
		result:     "child complete",
	}
	controller.RegisterDriver(parentDriver)
	controller.RegisterDriver(childDriver)

	parent, err := controller.Submit(context.Background(), RunSpec{
		Kind:          RunKindAgentTask,
		Goal:          "root goal",
		UserID:        "user-1",
		AgentID:       "main",
		Model:         "gpt-parent",
		ApprovalMode:  ApprovalModeAsk,
		SandboxMode:   "inherit",
		MaxDuration:   8 * time.Minute,
		MaxSteps:      20,
		MaxToolRounds: 40,
		MaxSubagents:  2,
		MaxDepth:      2,
	})
	if err != nil {
		t.Fatalf("Submit parent failed: %v", err)
	}

	executor := NewSubagentExecutor(controller, config.DefaultAgentsConfig())
	ctx := tools.WithRunID(context.Background(), parent.ID)
	result, err := executor.ExecuteSubagent(ctx, tools.SubagentRequest{
		Goal:    "inspect the failing query",
		Context: "Focus on SQL logs first.",
		Wait:    true,
	})
	if err != nil {
		t.Fatalf("ExecuteSubagent failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if !result.Waited || !result.Terminal || !result.Completed {
		t.Fatalf("unexpected terminal flags: %#v", result)
	}
	if result.ParentRunID != parent.ID || result.Status != string(RunStatusCompleted) {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Model != "gpt-parent" {
		t.Fatalf("model = %q, want gpt-parent", result.Model)
	}

	child, err := controller.GetStored(context.Background(), result.RunID)
	if err != nil {
		t.Fatalf("GetStored child failed: %v", err)
	}
	if child.MaxSteps != 10 {
		t.Fatalf("MaxSteps = %d, want 10", child.MaxSteps)
	}
	if child.MaxToolRounds != 20 {
		t.Fatalf("MaxToolRounds = %d, want 20", child.MaxToolRounds)
	}
	if child.MaxDuration != 4*time.Minute {
		t.Fatalf("MaxDuration = %v, want 4m", child.MaxDuration)
	}
	if contextText, _ := child.Metadata["context"].(string); contextText != "Focus on SQL logs first." {
		t.Fatalf("unexpected metadata: %#v", child.Metadata)
	}
	if isolated, _ := child.Metadata["subagent_isolated"].(bool); !isolated {
		t.Fatalf("expected child metadata to mark isolated subagent, got %#v", child.Metadata)
	}
	if !result.ContextIsolated {
		t.Fatalf("expected context_isolated=true, got %#v", result)
	}
}

func TestHarnessSubagentExecutor_DoesNotLeakParentMetadataIntoChild(t *testing.T) {
	controller := newTestController(t)
	parentDriver := &stubDriver{kind: RunKindAgentTask}
	childDriver := &completingSubagentDriver{
		controller: controller,
		kind:       RunKindSubagent,
		result:     "child complete",
	}
	controller.RegisterDriver(parentDriver)
	controller.RegisterDriver(childDriver)

	parent, err := controller.Submit(context.Background(), RunSpec{
		Kind:          RunKindAgentTask,
		Goal:          "root goal",
		UserID:        "user-1",
		AgentID:       "main",
		ApprovalMode:  ApprovalModeAsk,
		MaxSubagents:  2,
		MaxDepth:      2,
		Metadata:      map[string]interface{}{"secret": "keep-out"},
		MaxSteps:      10,
		MaxToolRounds: 10,
	})
	if err != nil {
		t.Fatalf("Submit parent failed: %v", err)
	}

	executor := NewSubagentExecutor(controller, config.DefaultAgentsConfig())
	ctx := tools.WithRunID(context.Background(), parent.ID)
	_, err = executor.ExecuteSubagent(ctx, tools.SubagentRequest{
		Goal:     "inspect the failing query",
		Wait:     true,
		Metadata: map[string]interface{}{"worker_role": "researcher"},
	})
	if err != nil {
		t.Fatalf("ExecuteSubagent failed: %v", err)
	}

	children, err := controller.List(context.Background(), RunFilter{ParentRunID: parent.ID, Limit: 10})
	if err != nil || len(children) != 1 {
		t.Fatalf("List child runs failed: %v children=%#v", err, children)
	}
	child := children[0]
	if _, leaked := child.Metadata["secret"]; leaked {
		t.Fatalf("expected parent metadata secret to stay isolated, got %#v", child.Metadata)
	}
	if role, _ := child.Metadata["worker_role"].(string); role != "researcher" {
		t.Fatalf("expected request metadata to survive, got %#v", child.Metadata)
	}
	if parentRunID, _ := child.Metadata["subagent_parent_run_id"].(string); parentRunID != parent.ID {
		t.Fatalf("expected child metadata to keep parent run reference, got %#v", child.Metadata)
	}
}

func TestHarnessSubagentExecutorWaitUsesIsolatedResultChannel(t *testing.T) {
	controller := newTestController(t)
	parentDriver := &stubDriver{kind: RunKindAgentTask}
	childDriver := &completingSubagentDriver{
		controller: controller,
		kind:       RunKindSubagent,
		result:     "child complete",
	}
	controller.RegisterDriver(parentDriver)
	controller.RegisterDriver(childDriver)

	parent, err := controller.Submit(context.Background(), RunSpec{
		Kind:          RunKindAgentTask,
		Goal:          "root goal",
		UserID:        "user-1",
		AgentID:       "main",
		ApprovalMode:  ApprovalModeAsk,
		MaxSubagents:  2,
		MaxDepth:      2,
		MaxSteps:      10,
		MaxToolRounds: 10,
	})
	if err != nil {
		t.Fatalf("Submit parent failed: %v", err)
	}

	executor := NewSubagentExecutor(controller, config.DefaultAgentsConfig())
	execution, immediate, err := executor.startIsolatedExecution(tools.WithRunID(context.Background(), parent.ID), tools.SubagentRequest{
		Goal: "inspect the failing query",
		Wait: true,
	})
	if err != nil {
		t.Fatalf("startIsolatedExecution failed: %v", err)
	}
	if execution == nil || execution.resultCh == nil {
		t.Fatalf("expected isolated execution with result channel, got %#v", execution)
	}
	if immediate == nil || !immediate.ContextIsolated {
		t.Fatalf("expected immediate isolated result, got %#v", immediate)
	}

	select {
	case outcome := <-execution.resultCh:
		if outcome.err != nil {
			t.Fatalf("unexpected wait error: %v", outcome.err)
		}
		if outcome.result == nil || !outcome.result.ContextIsolated || !outcome.result.Completed {
			t.Fatalf("unexpected isolated outcome: %#v", outcome.result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for isolated result")
	}
}

func TestHarnessSubagentExecutorAppendsLifecycleEvents(t *testing.T) {
	controller := newTestController(t)
	parentDriver := &stubDriver{kind: RunKindAgentTask}
	childDriver := &completingSubagentDriver{
		controller: controller,
		kind:       RunKindSubagent,
		result:     "child complete",
	}
	controller.RegisterDriver(parentDriver)
	controller.RegisterDriver(childDriver)

	parent, err := controller.Submit(context.Background(), RunSpec{
		Kind:          RunKindAgentTask,
		Goal:          "root goal",
		UserID:        "user-1",
		AgentID:       "main",
		ApprovalMode:  ApprovalModeAsk,
		MaxSubagents:  2,
		MaxDepth:      2,
		MaxSteps:      10,
		MaxToolRounds: 10,
	})
	if err != nil {
		t.Fatalf("Submit parent failed: %v", err)
	}

	executor := NewSubagentExecutor(controller, config.DefaultAgentsConfig())
	result, err := executor.ExecuteSubagent(tools.WithRunID(context.Background(), parent.ID), tools.SubagentRequest{
		Goal: "inspect the failing query",
		Wait: true,
	})
	if err != nil {
		t.Fatalf("ExecuteSubagent failed: %v", err)
	}

	events, err := controller.ListEvents(context.Background(), result.RunID, 20)
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	seenStart := false
	seenComplete := false
	for _, event := range events {
		if event.Type == "subagent_start" {
			seenStart = true
		}
		if event.Type == "subagent_complete" {
			seenComplete = true
		}
	}
	if !seenStart || !seenComplete {
		t.Fatalf("expected subagent lifecycle events, got %#v", events)
	}
}

func TestHarnessSubagentExecutorRejectsMissingParentRunWithGuardError(t *testing.T) {
	controller := newTestController(t)
	executor := NewSubagentExecutor(controller, config.DefaultAgentsConfig())

	_, err := executor.ExecuteSubagent(context.Background(), tools.SubagentRequest{
		Goal: "inspect the failing query",
		Wait: false,
	})
	if err == nil {
		t.Fatal("expected missing parent run to fail")
	}
	var guardErr *GuardPipelineError
	if !errors.As(err, &guardErr) {
		t.Fatalf("expected GuardPipelineError, got %T: %v", err, err)
	}
	if guardErr.Code != "parent_required" || guardErr.Stage != RuntimeStagePolicy {
		t.Fatalf("unexpected guard error: %#v", guardErr)
	}
}

func TestHarnessSubagentExecutorRejectsUnknownParentRunWithGuardError(t *testing.T) {
	controller := newTestController(t)
	executor := NewSubagentExecutor(controller, config.DefaultAgentsConfig())

	ctx := tools.WithRunID(context.Background(), "run-missing-parent")
	_, err := executor.ExecuteSubagent(ctx, tools.SubagentRequest{
		Goal: "inspect the failing query",
		Wait: false,
	})
	if err == nil {
		t.Fatal("expected unknown parent run to fail")
	}
	var guardErr *GuardPipelineError
	if !errors.As(err, &guardErr) {
		t.Fatalf("expected GuardPipelineError, got %T: %v", err, err)
	}
	if guardErr.Code != "parent_not_found" || guardErr.Stage != RuntimeStagePolicy {
		t.Fatalf("unexpected guard error: %#v", guardErr)
	}
}

func TestHarnessSubagentExecutorRejectsDisabledSubagentsWithGuardError(t *testing.T) {
	controller := newTestController(t)
	parentDriver := &stubDriver{kind: RunKindAgentTask}
	controller.RegisterDriver(parentDriver)

	parent, err := controller.Submit(context.Background(), RunSpec{
		Kind:         RunKindAgentTask,
		Goal:         "root goal",
		UserID:       "user-1",
		AgentID:      "main",
		ApprovalMode: ApprovalModeAsk,
		MaxDepth:     2,
		MaxSubagents: 2,
	})
	if err != nil {
		t.Fatalf("Submit parent failed: %v", err)
	}

	cfg := config.DefaultAgentsConfig()
	cfg.Defaults.Subagents.Enabled = false
	executor := NewSubagentExecutor(controller, &config.AgentsConfig{Defaults: cfg.Defaults})
	ctx := tools.WithRunID(context.Background(), parent.ID)

	_, err = executor.ExecuteSubagent(ctx, tools.SubagentRequest{
		Goal: "inspect the failing query",
		Wait: false,
	})
	if err == nil {
		t.Fatal("expected disabled subagents to fail")
	}
	var guardErr *GuardPipelineError
	if !errors.As(err, &guardErr) {
		t.Fatalf("expected GuardPipelineError, got %T: %v", err, err)
	}
	if guardErr.Code != "subagent_disabled" || guardErr.Stage != RuntimeStagePolicy {
		t.Fatalf("unexpected guard error: %#v", guardErr)
	}
}

func TestHarnessSubagentExecutorWaitCancellationReturnsStructuredGuardError(t *testing.T) {
	controller := newTestController(t)
	parentDriver := &stubDriver{kind: RunKindAgentTask}
	childDriver := &blockingSubagentDriver{kind: RunKindSubagent}
	controller.RegisterDriver(parentDriver)
	controller.RegisterDriver(childDriver)

	parent, err := controller.Submit(context.Background(), RunSpec{
		Kind:          RunKindAgentTask,
		Goal:          "root goal",
		UserID:        "user-1",
		AgentID:       "main",
		ApprovalMode:  ApprovalModeAsk,
		MaxDuration:   8 * time.Minute,
		MaxSteps:      20,
		MaxToolRounds: 40,
		MaxSubagents:  2,
		MaxDepth:      2,
	})
	if err != nil {
		t.Fatalf("Submit parent failed: %v", err)
	}

	executor := NewSubagentExecutor(controller, config.DefaultAgentsConfig())
	executor.pollInterval = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(tools.WithRunID(context.Background(), parent.ID))
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		_, runErr := executor.ExecuteSubagent(ctx, tools.SubagentRequest{
			Goal: "inspect the failing query",
			Wait: true,
		})
		errCh <- runErr
	}()

	var child Run
	waitForCondition(t, "subagent child created", func() bool {
		children, listErr := controller.List(context.Background(), RunFilter{ParentRunID: parent.ID, Limit: 10})
		if listErr != nil || len(children) == 0 {
			return false
		}
		child = children[0]
		return true
	})

	cancel()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected cancellation error")
		}
		var guardErr *GuardPipelineError
		if !errors.As(err, &guardErr) {
			t.Fatalf("expected GuardPipelineError, got %T: %v", err, err)
		}
		if guardErr.Code != "subagent_cancelled" || guardErr.Stage != RuntimeStageExecute {
			t.Fatalf("unexpected guard error: %#v", guardErr)
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected cancellation to match context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ExecuteSubagent cancellation")
	}

	if len(childDriver.cancelled) != 1 || childDriver.cancelled[0] != child.ID {
		t.Fatalf("child cancel calls = %#v, want [%q]", childDriver.cancelled, child.ID)
	}

	stored, err := controller.GetStored(context.Background(), child.ID)
	if err != nil {
		t.Fatalf("GetStored child failed: %v", err)
	}
	if stored.Status != RunStatusCancelled {
		t.Fatalf("child status = %q, want %q", stored.Status, RunStatusCancelled)
	}
}

func TestHarnessSubagentExecutorWaitDeadlineReturnsStructuredGuardError(t *testing.T) {
	controller := newTestController(t)
	parentDriver := &stubDriver{kind: RunKindAgentTask}
	childDriver := &blockingSubagentDriver{kind: RunKindSubagent}
	controller.RegisterDriver(parentDriver)
	controller.RegisterDriver(childDriver)

	parent, err := controller.Submit(context.Background(), RunSpec{
		Kind:          RunKindAgentTask,
		Goal:          "root goal",
		UserID:        "user-1",
		AgentID:       "main",
		ApprovalMode:  ApprovalModeAsk,
		MaxDuration:   8 * time.Minute,
		MaxSteps:      20,
		MaxToolRounds: 40,
		MaxSubagents:  2,
		MaxDepth:      2,
	})
	if err != nil {
		t.Fatalf("Submit parent failed: %v", err)
	}

	executor := NewSubagentExecutor(controller, config.DefaultAgentsConfig())
	executor.pollInterval = 5 * time.Millisecond

	ctx, cancel := context.WithTimeout(tools.WithRunID(context.Background(), parent.ID), 30*time.Millisecond)
	defer cancel()

	_, err = executor.ExecuteSubagent(ctx, tools.SubagentRequest{
		Goal: "inspect the failing query",
		Wait: true,
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	var guardErr *GuardPipelineError
	if !errors.As(err, &guardErr) {
		t.Fatalf("expected GuardPipelineError, got %T: %v", err, err)
	}
	if guardErr.Code != "subagent_timeout" || guardErr.Stage != RuntimeStageExecute {
		t.Fatalf("unexpected guard error: %#v", guardErr)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected timeout to match context.DeadlineExceeded, got %v", err)
	}
	if len(childDriver.cancelled) != 1 {
		t.Fatalf("expected one child cancellation, got %#v", childDriver.cancelled)
	}
	children, listErr := controller.List(context.Background(), RunFilter{ParentRunID: parent.ID, Limit: 10})
	if listErr != nil || len(children) != 1 {
		t.Fatalf("List child runs failed: %v children=%#v", listErr, children)
	}
	events, err := controller.ListEvents(context.Background(), children[0].ID, 20)
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	seenTimeout := false
	for _, event := range events {
		if event.Type == "subagent_timeout" {
			seenTimeout = true
			break
		}
	}
	if !seenTimeout {
		t.Fatalf("expected subagent_timeout event, got %#v", events)
	}
}
