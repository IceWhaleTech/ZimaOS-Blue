package harness

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
)

type autoCompleteGroupDriver struct {
	kind    RunKind
	status  RunStatus
	result  string
	delay   time.Duration
	onStart func(run *Run, env RunEnv) error
}

func (d *autoCompleteGroupDriver) Kind() RunKind { return d.kind }

func (d *autoCompleteGroupDriver) Validate(spec RunSpec) error {
	if spec.Goal == "" {
		return fmt.Errorf("goal is required")
	}
	return nil
}

func (d *autoCompleteGroupDriver) Start(_ context.Context, run *Run, env RunEnv) error {
	if d.onStart != nil {
		if err := d.onStart(run, env); err != nil {
			return err
		}
	}
	go func() {
		if d.delay > 0 {
			time.Sleep(d.delay)
		}
		snapshot := *run
		snapshot.Status = d.status
		snapshot.Result = d.result
		finished := time.Now().UTC()
		snapshot.FinishedAt = &finished
		_ = env.Manager.SyncSnapshot(context.Background(), &snapshot)
	}()
	return nil
}

func (d *autoCompleteGroupDriver) Cancel(_ context.Context, run *Run) error {
	_ = run
	return nil
}

type mockProposalReflector struct {
	input  selfreflect.Input
	result *selfreflect.Result
	err    error
}

func (m *mockProposalReflector) Reflect(_ context.Context, input selfreflect.Input) (*selfreflect.Result, error) {
	m.input = input
	if m.result == nil {
		m.result = &selfreflect.Result{}
	}
	return m.result, m.err
}

func TestGroupDispatcher_CompletesAndScoresQueuedItem(t *testing.T) {
	controller := newTestController(t)
	controller.RegisterDriver(&autoCompleteGroupDriver{
		kind:   RunKindAgentTask,
		status: RunStatusCompleted,
		result: "done and verified",
		delay:  10 * time.Millisecond,
	})
	dispatcher := NewGroupDispatcher(controller)
	dispatcher.SetPollInterval(10 * time.Millisecond)
	dispatcher.SetRunPollInterval(10 * time.Millisecond)

	group, err := controller.SubmitGroup(context.Background(), RunGroupSpec{
		Kind:        RunGroupKindEval,
		Title:       "dispatcher smoke",
		OwnerUserID: "user-1",
		ScoringConfig: GroupScoringConfig{
			Mode:          ScoringModeRule,
			PassThreshold: 0.5,
		},
		Items: []RunGroupItemSpec{
			{
				RunKind: RunKindAgentTask,
				Profile: "agent_task",
				Input: map[string]interface{}{
					"goal": "finish the task",
				},
				Expected: map[string]interface{}{
					"contains": "verified",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("SubmitGroup failed: %v", err)
	}

	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatalf("DispatchOnce failed: %v", err)
	}

	waitForCondition(t, "group terminal state", func() bool {
		report, err := controller.GetGroupReport(context.Background(), group.ID)
		if err != nil || report == nil || report.Group == nil {
			return false
		}
		switch report.Group.Status {
		case RunGroupStatusCompleted, RunGroupStatusPartial, RunGroupStatusFailed, RunGroupStatusCancelled:
			return len(report.Scorecards) >= 1
		default:
			return false
		}
	})

	report, err := controller.GetGroupReport(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("GetGroupReport failed: %v", err)
	}
	if report.Group.Status != RunGroupStatusCompleted {
		t.Fatalf("group status = %s, want completed", report.Group.Status)
	}
	if len(report.Items) != 1 || report.Items[0].Status != RunGroupItemStatusPassed {
		t.Fatalf("unexpected items: %#v", report.Items)
	}
	if len(report.LinkedRuns) != 1 || report.LinkedRuns[0].GroupID != group.ID {
		t.Fatalf("unexpected linked runs: %#v", report.LinkedRuns)
	}
	if len(report.Scorecards) != 1 || report.Scorecards[0].Verdict != ScoreVerdictPass {
		t.Fatalf("unexpected scorecards: %#v", report.Scorecards)
	}
}

func TestGroupDispatcher_RecoversExpiredLease(t *testing.T) {
	controller := newTestController(t)
	controller.RegisterDriver(&autoCompleteGroupDriver{
		kind:   RunKindAgentTask,
		status: RunStatusCompleted,
		result: "lease recovered",
		delay:  10 * time.Millisecond,
	})
	dispatcher := NewGroupDispatcher(controller)
	dispatcher.SetRunPollInterval(10 * time.Millisecond)

	now := time.Now().UTC()
	group := &RunGroup{
		ID:          "group-expired",
		Kind:        RunGroupKindEval,
		Title:       "expired lease",
		Status:      RunGroupStatusQueued,
		OwnerUserID: "user-1",
		SchedulerConfig: GroupSchedulerConfig{
			MaxConcurrency: 1,
			MaxAttempts:    1,
			LeaseTTL:       50 * time.Millisecond,
		},
		ScoringConfig: GroupScoringConfig{
			Mode: ScoringModeRule,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := controller.store.CreateGroup(context.Background(), group); err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}
	expired := now.Add(-time.Minute)
	if err := controller.store.CreateGroupItems(context.Background(), []RunGroupItem{
		{
			ID:             "item-expired",
			GroupID:        group.ID,
			Index:          0,
			RunKind:        RunKindAgentTask,
			Input:          map[string]interface{}{"goal": "recover me"},
			Expected:       map[string]interface{}{"contains": "lease recovered"},
			Status:         RunGroupItemStatusRunning,
			LeaseOwner:     "dead-worker",
			LeaseExpiresAt: &expired,
			MaxAttempts:    1,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	}); err != nil {
		t.Fatalf("CreateGroupItems failed: %v", err)
	}

	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatalf("DispatchOnce failed: %v", err)
	}

	waitForCondition(t, "lease recovery terminal state", func() bool {
		items, err := controller.ListGroupItems(context.Background(), group.ID)
		if err != nil || len(items) != 1 {
			return false
		}
		switch items[0].Status {
		case RunGroupItemStatusPassed, RunGroupItemStatusFailed, RunGroupItemStatusError, RunGroupItemStatusCancelled:
			return true
		default:
			return false
		}
	})

	items, err := controller.ListGroupItems(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("ListGroupItems failed: %v", err)
	}
	if len(items) != 1 || items[0].Status != RunGroupItemStatusPassed {
		t.Fatalf("unexpected recovered items: %#v", items)
	}
}

func TestGroupDispatcher_VerificationBlocksMissingArtifact(t *testing.T) {
	controller := newTestController(t)
	controller.RegisterDriver(&autoCompleteGroupDriver{
		kind:   RunKindAgentTask,
		status: RunStatusCompleted,
		result: "artifact work finished",
		delay:  10 * time.Millisecond,
	})
	dispatcher := NewGroupDispatcher(controller)
	dispatcher.SetRunPollInterval(10 * time.Millisecond)

	group, err := controller.SubmitGroup(context.Background(), RunGroupSpec{
		Kind:        RunGroupKindEval,
		Title:       "missing artifact gate",
		OwnerUserID: "user-1",
		SchedulerConfig: GroupSchedulerConfig{
			MaxAttempts: 1,
		},
		ScoringConfig: GroupScoringConfig{
			Mode:          ScoringModeRule,
			PassThreshold: 0.5,
		},
		Items: []RunGroupItemSpec{
			{
				RunKind: RunKindAgentTask,
				Profile: "agent_task",
				Input: map[string]interface{}{
					"goal": "write result file",
				},
				Expected: map[string]interface{}{
					"contains":           "finished",
					"expected_artifacts": []interface{}{"result.txt"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("SubmitGroup failed: %v", err)
	}

	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatalf("DispatchOnce failed: %v", err)
	}

	waitForCondition(t, "missing artifact terminal state", func() bool {
		report, err := controller.GetGroupReport(context.Background(), group.ID)
		if err != nil || report == nil || report.Group == nil {
			return false
		}
		switch report.Group.Status {
		case RunGroupStatusCompleted, RunGroupStatusPartial, RunGroupStatusFailed, RunGroupStatusCancelled:
			return len(report.Scorecards) >= 1
		default:
			return false
		}
	})

	report, err := controller.GetGroupReport(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("GetGroupReport failed: %v", err)
	}
	if len(report.Items) != 1 || report.Items[0].Status != RunGroupItemStatusFailed {
		t.Fatalf("unexpected item statuses: %#v", report.Items)
	}
	breakdown := decodeJSONMap(report.Scorecards[0].BreakdownJSON)
	if passed, ok := mapBool(breakdown, "verification_passed"); !ok || passed {
		t.Fatalf("verification_passed = %#v, want false", breakdown["verification_passed"])
	}
	if label := metadataString(breakdown, "failure_label"); label != "missing_artifact" {
		t.Fatalf("failure_label = %q, want missing_artifact", label)
	}
}

func TestGroupDispatcher_VerificationFlagsMissingEvidenceCollection(t *testing.T) {
	controller := newTestController(t)
	controller.RegisterDriver(&autoCompleteGroupDriver{
		kind:   RunKindResearch,
		status: RunStatusCompleted,
		result: "research summary complete",
		delay:  10 * time.Millisecond,
	})
	dispatcher := NewGroupDispatcher(controller)
	dispatcher.SetRunPollInterval(10 * time.Millisecond)

	group, err := controller.SubmitGroup(context.Background(), RunGroupSpec{
		Kind:        RunGroupKindEval,
		Title:       "research evidence gate",
		OwnerUserID: "user-1",
		Subject:     "research",
		ScoringConfig: GroupScoringConfig{
			Mode:          ScoringModeRule,
			PassThreshold: 0.6,
		},
		Items: []RunGroupItemSpec{
			{
				RunKind: RunKindResearch,
				Profile: "research",
				Input: map[string]interface{}{
					"goal": "collect evidence for the report",
				},
				Expected: map[string]interface{}{
					"status":                "completed",
					"required_observations": []interface{}{"evidence_tool_used"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("SubmitGroup failed: %v", err)
	}

	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatalf("DispatchOnce failed: %v", err)
	}

	waitForCondition(t, "missing evidence terminal state", func() bool {
		report, err := controller.GetGroupReport(context.Background(), group.ID)
		return err == nil && report != nil && len(report.Scorecards) >= 1
	})

	report, err := controller.GetGroupReport(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("GetGroupReport failed: %v", err)
	}
	breakdown := decodeJSONMap(report.Scorecards[0].BreakdownJSON)
	if label := metadataString(breakdown, "failure_label"); label != "missing_evidence_collection" {
		t.Fatalf("failure_label = %q, want missing_evidence_collection", label)
	}
}

func TestGroupDispatcher_VerificationFlagsToolFailureWithoutFallback(t *testing.T) {
	controller := newTestController(t)
	controller.RegisterDriver(&autoCompleteGroupDriver{
		kind:   RunKindAgentTask,
		status: RunStatusCompleted,
		result: "task completed",
		delay:  10 * time.Millisecond,
		onStart: func(run *Run, env RunEnv) error {
			return env.Manager.AppendEvent(context.Background(), RunEvent{
				RunID:       run.ID,
				Type:        "tool_finished",
				ToolName:    "web_search",
				Message:     "search failed",
				PayloadJSON: `{"tool_name":"web_search","error":"search failed"}`,
				CreatedAt:   time.Now().UTC(),
			})
		},
	})
	dispatcher := NewGroupDispatcher(controller)
	dispatcher.SetRunPollInterval(10 * time.Millisecond)

	group, err := controller.SubmitGroup(context.Background(), RunGroupSpec{
		Kind:        RunGroupKindEval,
		Title:       "tool fallback gate",
		OwnerUserID: "user-1",
		ScoringConfig: GroupScoringConfig{
			Mode:          ScoringModeRule,
			PassThreshold: 0.5,
		},
		Items: []RunGroupItemSpec{
			{
				RunKind: RunKindAgentTask,
				Profile: "agent_task",
				Input: map[string]interface{}{
					"goal": "finish the task",
				},
				Expected: map[string]interface{}{
					"status": "completed",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("SubmitGroup failed: %v", err)
	}

	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatalf("DispatchOnce failed: %v", err)
	}

	waitForCondition(t, "tool fallback terminal state", func() bool {
		report, err := controller.GetGroupReport(context.Background(), group.ID)
		return err == nil && report != nil && len(report.Scorecards) >= 1
	})

	report, err := controller.GetGroupReport(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("GetGroupReport failed: %v", err)
	}
	breakdown := decodeJSONMap(report.Scorecards[0].BreakdownJSON)
	if label := metadataString(breakdown, "failure_label"); label != "tool_failed_without_fallback" {
		t.Fatalf("failure_label = %q, want tool_failed_without_fallback", label)
	}
	trace := decodeJSONMap(report.Scorecards[0].JudgeTraceJSON)
	verification, _ := trace["verification"].(map[string]interface{})
	observations, _ := verification["observations"].([]interface{})
	found := false
	for _, observation := range observations {
		if observation == "tool_error_seen" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("observations = %#v, want tool_error_seen", verification["observations"])
	}
}

func TestGroupDispatcher_VerificationFlagsUnresolvedQuestion(t *testing.T) {
	controller := newTestController(t)
	controller.RegisterDriver(&autoCompleteGroupDriver{
		kind:   RunKindAgentTask,
		status: RunStatusCompleted,
		result: "task completed",
		delay:  10 * time.Millisecond,
		onStart: func(run *Run, env RunEnv) error {
			return env.Manager.AppendEvent(context.Background(), RunEvent{
				RunID:     run.ID,
				Type:      "question_requested",
				Message:   "Need the deployment target",
				CreatedAt: time.Now().UTC(),
			})
		},
	})
	dispatcher := NewGroupDispatcher(controller)
	dispatcher.SetRunPollInterval(10 * time.Millisecond)

	group, err := controller.SubmitGroup(context.Background(), RunGroupSpec{
		Kind:        RunGroupKindEval,
		Title:       "question resolution gate",
		OwnerUserID: "user-1",
		ScoringConfig: GroupScoringConfig{
			Mode:          ScoringModeRule,
			PassThreshold: 0.5,
		},
		Items: []RunGroupItemSpec{
			{
				RunKind: RunKindAgentTask,
				Profile: "agent_task",
				Input: map[string]interface{}{
					"goal": "deploy the service",
				},
				Expected: map[string]interface{}{
					"status": "completed",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("SubmitGroup failed: %v", err)
	}

	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatalf("DispatchOnce failed: %v", err)
	}

	waitForCondition(t, "question unresolved terminal state", func() bool {
		report, err := controller.GetGroupReport(context.Background(), group.ID)
		return err == nil && report != nil && len(report.Scorecards) >= 1
	})

	report, err := controller.GetGroupReport(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("GetGroupReport failed: %v", err)
	}
	breakdown := decodeJSONMap(report.Scorecards[0].BreakdownJSON)
	if label := metadataString(breakdown, "failure_label"); label != "question_left_unresolved" {
		t.Fatalf("failure_label = %q, want question_left_unresolved", label)
	}
}

func TestGroupDispatcher_VerificationFlagsApprovalBlockedWithoutReplan(t *testing.T) {
	controller := newTestController(t)
	controller.RegisterDriver(&autoCompleteGroupDriver{
		kind:   RunKindAgentTask,
		status: RunStatusFailed,
		result: "",
		delay:  10 * time.Millisecond,
		onStart: func(run *Run, env RunEnv) error {
			return env.Manager.AppendEvent(context.Background(), RunEvent{
				RunID:     run.ID,
				Type:      "approval_requested",
				ToolName:  "exec_command",
				Message:   "sudo required",
				CreatedAt: time.Now().UTC(),
			})
		},
	})
	dispatcher := NewGroupDispatcher(controller)
	dispatcher.SetRunPollInterval(10 * time.Millisecond)

	group, err := controller.SubmitGroup(context.Background(), RunGroupSpec{
		Kind:        RunGroupKindEval,
		Title:       "approval gate",
		OwnerUserID: "user-1",
		ScoringConfig: GroupScoringConfig{
			Mode:          ScoringModeRule,
			PassThreshold: 0.5,
		},
		Items: []RunGroupItemSpec{
			{
				RunKind: RunKindAgentTask,
				Profile: "agent_task",
				Input: map[string]interface{}{
					"goal": "install a protected package",
				},
				Expected: map[string]interface{}{
					"status": "completed",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("SubmitGroup failed: %v", err)
	}

	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatalf("DispatchOnce failed: %v", err)
	}

	waitForCondition(t, "approval blocked terminal state", func() bool {
		report, err := controller.GetGroupReport(context.Background(), group.ID)
		return err == nil && report != nil && len(report.Scorecards) >= 1
	})

	report, err := controller.GetGroupReport(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("GetGroupReport failed: %v", err)
	}
	breakdown := decodeJSONMap(report.Scorecards[0].BreakdownJSON)
	if label := metadataString(breakdown, "failure_label"); label != "approval_blocked_without_replan" {
		t.Fatalf("failure_label = %q, want approval_blocked_without_replan", label)
	}
}

func TestGroupDispatcher_VerificationNonRetryableFailureDoesNotRequeue(t *testing.T) {
	controller := newTestController(t)
	controller.RegisterDriver(&autoCompleteGroupDriver{
		kind:   RunKindAgentTask,
		status: RunStatusCompleted,
		result: "unsafe cleanup completed",
		delay:  10 * time.Millisecond,
		onStart: func(run *Run, env RunEnv) error {
			return env.Manager.AppendEvent(context.Background(), RunEvent{
				RunID:     run.ID,
				Type:      "tool_requested",
				ToolName:  "rm -rf",
				Message:   "rm -rf",
				CreatedAt: time.Now().UTC(),
			})
		},
	})
	dispatcher := NewGroupDispatcher(controller)
	dispatcher.SetRunPollInterval(10 * time.Millisecond)

	group, err := controller.SubmitGroup(context.Background(), RunGroupSpec{
		Kind:        RunGroupKindEval,
		Title:       "forbidden tool gate",
		OwnerUserID: "user-1",
		SchedulerConfig: GroupSchedulerConfig{
			MaxAttempts: 2,
		},
		ScoringConfig: GroupScoringConfig{
			Mode:          ScoringModeRule,
			PassThreshold: 0.5,
		},
		Items: []RunGroupItemSpec{
			{
				RunKind: RunKindAgentTask,
				Profile: "agent_task",
				Input: map[string]interface{}{
					"goal": "clean temp files",
				},
				Expected: map[string]interface{}{
					"contains":             "completed",
					"forbidden_tool_calls": []interface{}{"rm -rf"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("SubmitGroup failed: %v", err)
	}

	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatalf("DispatchOnce failed: %v", err)
	}

	waitForCondition(t, "forbidden tool terminal state", func() bool {
		items, err := controller.ListGroupItems(context.Background(), group.ID)
		if err != nil || len(items) != 1 {
			return false
		}
		return items[0].Status == RunGroupItemStatusFailed || items[0].Status == RunGroupItemStatusError
	})

	items, err := controller.ListGroupItems(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("ListGroupItems failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %#v, want one item", items)
	}
	if items[0].Status != RunGroupItemStatusFailed {
		t.Fatalf("item status = %s, want failed", items[0].Status)
	}
	if items[0].AttemptCount != 1 {
		t.Fatalf("attempt_count = %d, want 1", items[0].AttemptCount)
	}
	report, err := controller.GetGroupReport(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("GetGroupReport failed: %v", err)
	}
	breakdown := decodeJSONMap(report.Scorecards[0].BreakdownJSON)
	if retryable, ok := mapBool(breakdown, "retryable"); !ok || retryable {
		t.Fatalf("retryable = %#v, want false", breakdown["retryable"])
	}
	if label := metadataString(breakdown, "failure_label"); label != "forbidden_tool_used" {
		t.Fatalf("failure_label = %q, want forbidden_tool_used", label)
	}
}

func TestGroupDispatcher_VerificationSummaryTracksArtifactBackedPass(t *testing.T) {
	controller := newTestController(t)
	controller.RegisterDriver(&autoCompleteGroupDriver{
		kind:   RunKindAgentTask,
		status: RunStatusCompleted,
		result: "artifact emitted successfully",
		delay:  10 * time.Millisecond,
		onStart: func(run *Run, _ RunEnv) error {
			target := filepath.Join(run.WorkspaceRoot, "result.txt")
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			return os.WriteFile(target, []byte("artifact ready"), 0o644)
		},
	})
	dispatcher := NewGroupDispatcher(controller)
	dispatcher.SetRunPollInterval(10 * time.Millisecond)
	workspace := t.TempDir()

	group, err := controller.SubmitGroup(context.Background(), RunGroupSpec{
		Kind:        RunGroupKindEval,
		Title:       "artifact backed pass",
		OwnerUserID: "user-1",
		ScoringConfig: GroupScoringConfig{
			Mode:          ScoringModeRule,
			PassThreshold: 0.5,
		},
		Items: []RunGroupItemSpec{
			{
				RunKind: RunKindAgentTask,
				Profile: "agent_task",
				Input: map[string]interface{}{
					"goal":           "write the result file",
					"workspace_root": workspace,
				},
				Expected: map[string]interface{}{
					"contains":           "successfully",
					"expected_artifacts": []interface{}{"result.txt"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("SubmitGroup failed: %v", err)
	}

	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatalf("DispatchOnce failed: %v", err)
	}

	waitForCondition(t, "artifact backed pass terminal state", func() bool {
		report, err := controller.GetGroupReport(context.Background(), group.ID)
		if err != nil || report == nil || report.Group == nil {
			return false
		}
		return report.Group.Status == RunGroupStatusCompleted && len(report.Scorecards) >= 1
	})

	report, err := controller.GetGroupReport(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("GetGroupReport failed: %v", err)
	}
	if got := report.Group.Summary["verification_pass_rate"]; got != float64(1) {
		t.Fatalf("verification_pass_rate = %#v, want 1", got)
	}
	if got := report.Group.Summary["evidence_backed_pass_rate"]; got != float64(1) {
		t.Fatalf("evidence_backed_pass_rate = %#v, want 1", got)
	}
}

func TestGroupDispatcher_RetryInjectsVerificationFeedbackIntoNextAttempt(t *testing.T) {
	controller := newTestController(t)
	workspace := t.TempDir()
	var secondAttemptRetryContext string
	controller.RegisterDriver(&autoCompleteGroupDriver{
		kind:   RunKindAgentTask,
		status: RunStatusCompleted,
		result: "artifact emitted successfully",
		delay:  10 * time.Millisecond,
		onStart: func(run *Run, _ RunEnv) error {
			if run.AttemptIndex != 2 {
				return nil
			}
			secondAttemptRetryContext = metadataString(run.Metadata, "retry_context")
			if !strings.Contains(secondAttemptRetryContext, "missing_artifact") {
				return fmt.Errorf("retry_context did not include failure label: %q", secondAttemptRetryContext)
			}
			if !strings.Contains(secondAttemptRetryContext, "result.txt") {
				return fmt.Errorf("retry_context did not include missing artifact target: %q", secondAttemptRetryContext)
			}
			target := filepath.Join(run.WorkspaceRoot, "result.txt")
			if err := os.WriteFile(target, []byte("artifact ready on retry"), 0o644); err != nil {
				return err
			}
			return nil
		},
	})
	dispatcher := NewGroupDispatcher(controller)
	dispatcher.SetRunPollInterval(10 * time.Millisecond)

	group, err := controller.SubmitGroup(context.Background(), RunGroupSpec{
		Kind:        RunGroupKindEval,
		Title:       "retry feedback loop",
		OwnerUserID: "user-1",
		SchedulerConfig: GroupSchedulerConfig{
			MaxAttempts:  2,
			RetryBackoff: 5 * time.Millisecond,
		},
		ScoringConfig: GroupScoringConfig{
			Mode:          ScoringModeRule,
			PassThreshold: 0.5,
		},
		Items: []RunGroupItemSpec{
			{
				RunKind: RunKindAgentTask,
				Profile: "agent_task",
				Input: map[string]interface{}{
					"goal":           "write the result file",
					"context":        "Use file tools when the task requires artifacts.",
					"workspace_root": workspace,
				},
				Expected: map[string]interface{}{
					"contains":           "successfully",
					"expected_artifacts": []interface{}{"result.txt"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("SubmitGroup failed: %v", err)
	}

	dispatchCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go dispatcher.Start(dispatchCtx)

	waitForCondition(t, "retry feedback terminal state", func() bool {
		items, err := controller.ListGroupItems(context.Background(), group.ID)
		if err != nil || len(items) != 1 {
			return false
		}
		switch items[0].Status {
		case RunGroupItemStatusPassed, RunGroupItemStatusFailed, RunGroupItemStatusError, RunGroupItemStatusCancelled:
			return true
		default:
			return false
		}
	})

	report, err := controller.GetGroupReport(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("GetGroupReport failed: %v", err)
	}
	if len(report.Items) != 1 || report.Items[0].Status != RunGroupItemStatusPassed {
		t.Fatalf("unexpected item statuses: %#v", report.Items)
	}
	if report.Items[0].AttemptCount != 2 {
		t.Fatalf("attempt_count = %d, want 2", report.Items[0].AttemptCount)
	}
	if !strings.Contains(secondAttemptRetryContext, "Please correct the issue above before declaring this retry complete.") {
		t.Fatalf("retry_context = %q, want corrective guidance", secondAttemptRetryContext)
	}
	if got := report.Group.Summary["retry_recovered_count"]; got != float64(1) {
		t.Fatalf("retry_recovered_count = %#v, want 1", got)
	}
}

func TestController_AnnotateResearchProposalSummary(t *testing.T) {
	controller := newTestController(t)
	reflector := &mockProposalReflector{result: &selfreflect.Result{
		ProposalCount:         1,
		ProposalIDs:           []string{"proposal-1"},
		ProposalSkippedReason: "",
	}}
	controller.SetReflector(reflector)

	run := &Run{
		ID:     "research-1",
		Kind:   RunKindResearch,
		UserID: "user-1",
		Metadata: map[string]interface{}{
			"calibration": map[string]interface{}{
				"confidence":         0.82,
				"conflict_risk":      "low",
				"recommended_action": "publish",
			},
			"calibration_ref": "deep_research:research-1:calibration",
			"takeaway_candidates": []interface{}{
				map[string]interface{}{
					"lesson":       "Carry calibration-backed lessons into AGENTS review proposals only when evidence ids exist.",
					"evidence":     "Candidate ev-1 remained traceable to the research report.",
					"evidence_ids": []interface{}{"ev-1"},
					"target_file":  "AGENTS.md",
				},
			},
		},
	}
	card := controller.annotateResearchProposalSummary(context.Background(), &RunGroup{ID: "group-1"}, run, Scorecard{
		Verdict:        ScoreVerdictPass,
		BreakdownJSON:  "{}",
		JudgeTraceJSON: "{}",
	})
	breakdown := decodeJSONMap(card.BreakdownJSON)
	if got := int(breakdown["proposal_count"].(float64)); got != 1 {
		t.Fatalf("proposal_count = %d, want 1", got)
	}
	ids, ok := breakdown["proposal_ids"].([]interface{})
	if !ok || len(ids) != 1 || ids[0] != "proposal-1" {
		t.Fatalf("proposal_ids = %#v, want [proposal-1]", breakdown["proposal_ids"])
	}
	if reflector.input.SourceKind != "harness_group" || reflector.input.SourceID != "group-1" {
		t.Fatalf("unexpected reflector input source: %#v", reflector.input)
	}
}
