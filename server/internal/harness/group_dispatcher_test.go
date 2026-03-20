package harness

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type autoCompleteGroupDriver struct {
	kind   RunKind
	status RunStatus
	result string
	delay  time.Duration
}

func (d *autoCompleteGroupDriver) Kind() RunKind { return d.kind }

func (d *autoCompleteGroupDriver) Validate(spec RunSpec) error {
	if spec.Goal == "" {
		return fmt.Errorf("goal is required")
	}
	return nil
}

func (d *autoCompleteGroupDriver) Start(_ context.Context, run *Run, env RunEnv) error {
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

func TestGroupDispatcher_CompletesAndScoresQueuedItem(t *testing.T) {
	controller := newTestController(t)
	controller.RegisterDriver(&autoCompleteGroupDriver{
		kind:   RunKindAgentTask,
		status: RunStatusCompleted,
		result: "done and verified",
		delay:  10 * time.Millisecond,
	})
	dispatcher := NewGroupDispatcher(controller)
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
			Mode: ScoringModeJudge,
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

func TestController_SyncExperimentGroupProjection(t *testing.T) {
	controller := newTestController(t)
	run := &Run{
		ID:            "research-1",
		RootRunID:     "research-1",
		Kind:          RunKindResearch,
		Status:        RunStatusCompleted,
		UserID:        "user-1",
		Goal:          "experiment route",
		Result:        "report ready",
		Metadata:      map[string]interface{}{"route_mode": "experiment"},
		ArtifactRoot:  "/tmp/artifacts/research-1",
		WorkspaceRoot: "/tmp/workspace",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := controller.store.CreateRun(context.Background(), run); err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	if err := controller.SyncExperimentGroupProjection(context.Background(), run.ID); err != nil {
		t.Fatalf("SyncExperimentGroupProjection failed: %v", err)
	}

	groupID := experimentProjectionGroupID(run.ID)
	report, err := controller.GetGroupReport(context.Background(), groupID)
	if err != nil {
		t.Fatalf("GetGroupReport failed: %v", err)
	}
	if report.Group.Kind != RunGroupKindExperiment || report.Group.Status != RunGroupStatusCompleted {
		t.Fatalf("unexpected projection group: %#v", report.Group)
	}
	if len(report.Items) != 1 || report.Items[0].LatestRunID != run.ID {
		t.Fatalf("unexpected projection items: %#v", report.Items)
	}
	if len(report.Scorecards) != 1 || report.Scorecards[0].Verdict != ScoreVerdictPass {
		t.Fatalf("unexpected projection scorecards: %#v", report.Scorecards)
	}
}
