package harness

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type runtimeEvidenceDriver struct {
	kind       RunKind
	evidenceBy map[string][]RuntimeEvidenceEntry
}

func (d *runtimeEvidenceDriver) Kind() RunKind { return d.kind }

func (d *runtimeEvidenceDriver) Validate(RunSpec) error { return nil }

func (d *runtimeEvidenceDriver) Start(_ context.Context, _ *Run, _ RunEnv) error { return nil }

func (d *runtimeEvidenceDriver) Cancel(_ context.Context, _ *Run) error { return nil }

func (d *runtimeEvidenceDriver) ListRuntimeEvidence(_ context.Context, run *Run) ([]RuntimeEvidenceEntry, error) {
	if d == nil || run == nil || len(d.evidenceBy) == 0 {
		return nil, nil
	}
	evidence := d.evidenceBy[run.ID]
	if len(evidence) == 0 {
		return nil, nil
	}
	out := make([]RuntimeEvidenceEntry, len(evidence))
	copy(out, evidence)
	return out, nil
}

func TestController_GetGroupReportIncludesRuntimeTraces(t *testing.T) {
	controller := newTestController(t)
	controller.SetRunTraceProvider(NewRunTraceCollector(controller))
	ctx := context.Background()

	group, err := controller.SubmitGroup(ctx, RunGroupSpec{
		Kind:        RunGroupKindEval,
		Title:       "runtime traces",
		OwnerUserID: "user-1",
		Items: []RunGroupItemSpec{
			{
				RunKind: RunKindAgentTask,
				Input:   map[string]interface{}{"goal": "trace linked run"},
			},
		},
	})
	if err != nil {
		t.Fatalf("SubmitGroup failed: %v", err)
	}

	items, err := controller.ListGroupItems(ctx, group.ID)
	if err != nil {
		t.Fatalf("ListGroupItems failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("ListGroupItems len = %d, want 1", len(items))
	}

	startedAt := time.Now().UTC().Add(-2 * time.Second)
	finishedAt := startedAt.Add(1200 * time.Millisecond)
	run := &Run{
		ID:           uuid.NewString(),
		RootRunID:    "",
		GroupID:      group.ID,
		GroupItemID:  items[0].ID,
		AttemptIndex: 1,
		Kind:         RunKindAgentTask,
		Status:       RunStatusCompleted,
		UserID:       "user-1",
		Goal:         "trace linked run",
		CreatedAt:    startedAt,
		UpdatedAt:    finishedAt,
		StartedAt:    &startedAt,
		FinishedAt:   &finishedAt,
	}
	run.RootRunID = run.ID
	if err := controller.store.CreateRun(ctx, run); err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	item := items[0]
	item.LatestRunID = run.ID
	item.AttemptCount = 1
	item.Status = RunGroupItemStatusPassed
	if err := controller.store.UpdateGroupItem(ctx, &item); err != nil {
		t.Fatalf("UpdateGroupItem failed: %v", err)
	}

	if err := controller.AppendEvent(ctx, RunEvent{
		RunID:       run.ID,
		RootRunID:   run.RootRunID,
		Type:        "stage_changed",
		Message:     "driver dispatch started",
		CreatedAt:   startedAt,
		PayloadJSON: marshalInterface(map[string]interface{}{"stage": RuntimeStageExecute, "status": RunStatusExecuting, "driver_type": "test_driver"}),
	}); err != nil {
		t.Fatalf("AppendEvent(stage_changed) failed: %v", err)
	}
	if err := controller.AppendEvent(ctx, RunEvent{
		RunID:     run.ID,
		RootRunID: run.RootRunID,
		Type:      "trace_started",
		Message:   "driver start completed",
		CreatedAt: startedAt,
	}); err != nil {
		t.Fatalf("AppendEvent(trace_started) failed: %v", err)
	}
	if err := controller.store.AttachArtifact(ctx, ArtifactRef{
		ID:        uuid.NewString(),
		RunID:     run.ID,
		Kind:      "trace",
		Label:     "trace log",
		PathOrURL: "/tmp/trace.log",
	}); err != nil {
		t.Fatalf("AttachArtifact failed: %v", err)
	}

	report, err := controller.GetGroupReport(ctx, group.ID)
	if err != nil {
		t.Fatalf("GetGroupReport failed: %v", err)
	}
	if report == nil || len(report.RuntimeTraces) != 1 {
		t.Fatalf("runtime traces = %#v, want 1 entry", report)
	}
	trace, ok := report.RuntimeTraces[run.ID]
	if !ok {
		t.Fatalf("runtime traces missing run %q: %#v", run.ID, report.RuntimeTraces)
	}
	if trace.RunID != run.ID || trace.Status != RunStatusCompleted {
		t.Fatalf("unexpected runtime trace header: %#v", trace)
	}
	if !hasTraceStage(trace.Stages, RuntimeStageExecute) {
		t.Fatalf("unexpected runtime trace stages: %#v", trace.Stages)
	}
	if !hasTraceEvent(trace.Events, "trace_started") {
		t.Fatalf("unexpected runtime trace events: %#v", trace.Events)
	}
	if len(trace.Artifacts) != 1 || trace.Artifacts[0].Label != "trace log" {
		t.Fatalf("unexpected runtime trace artifacts: %#v", trace.Artifacts)
	}
}

func TestController_RefreshGroupSummaryNoOpPreservesUpdatedAt(t *testing.T) {
	controller := newTestController(t)

	group, err := controller.SubmitGroup(context.Background(), RunGroupSpec{
		Kind:        RunGroupKindEval,
		Title:       "no-op summary refresh",
		OwnerUserID: "user-1",
		Items: []RunGroupItemSpec{
			{
				RunKind: RunKindAgentTask,
				Input:   map[string]interface{}{"goal": "stay queued"},
			},
		},
	})
	if err != nil {
		t.Fatalf("SubmitGroup failed: %v", err)
	}

	before, err := controller.store.GetGroup(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("GetGroup(before) failed: %v", err)
	}

	time.Sleep(20 * time.Millisecond)

	refreshed, err := controller.refreshGroupSummary(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("refreshGroupSummary failed: %v", err)
	}
	if refreshed.Status != RunGroupStatusQueued {
		t.Fatalf("status = %q, want %q", refreshed.Status, RunGroupStatusQueued)
	}

	after, err := controller.store.GetGroup(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("GetGroup(after) failed: %v", err)
	}
	if !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Fatalf("UpdatedAt changed on no-op refresh: before=%s after=%s", before.UpdatedAt, after.UpdatedAt)
	}
}

func TestController_GetGroupPreservesCompletedEmptyGroup(t *testing.T) {
	controller := newTestController(t)

	group, err := controller.SubmitGroup(context.Background(), RunGroupSpec{
		Kind:        RunGroupKindEval,
		Title:       "empty group",
		OwnerUserID: "user-1",
	})
	if err != nil {
		t.Fatalf("SubmitGroup failed: %v", err)
	}
	if group.Status != RunGroupStatusCompleted {
		t.Fatalf("initial status = %q, want %q", group.Status, RunGroupStatusCompleted)
	}
	if group.FinishedAt == nil {
		t.Fatal("expected empty group to have FinishedAt set")
	}

	finishedAt := *group.FinishedAt
	time.Sleep(20 * time.Millisecond)

	got, err := controller.GetGroup(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("GetGroup failed: %v", err)
	}
	if got.Status != RunGroupStatusCompleted {
		t.Fatalf("status = %q, want %q", got.Status, RunGroupStatusCompleted)
	}
	if got.FinishedAt == nil {
		t.Fatal("expected completed empty group to retain FinishedAt")
	}
	if !got.FinishedAt.Equal(finishedAt) {
		t.Fatalf("FinishedAt changed across refresh: before=%s after=%s", finishedAt, *got.FinishedAt)
	}
}

func TestController_GetGroupReportReconcilesCompletedRunAgainstStaleScorecard(t *testing.T) {
	controller := newTestController(t)
	ctx := context.Background()

	group, err := controller.SubmitGroup(ctx, RunGroupSpec{
		Kind:        RunGroupKindEval,
		Title:       "reconcile stale scorecard",
		OwnerUserID: "user-1",
		Items: []RunGroupItemSpec{
			{
				RunKind:  RunKindAgentTask,
				Input:    map[string]interface{}{"goal": "summarize"},
				Expected: map[string]interface{}{"status": "completed"},
				Metadata: map[string]interface{}{"dataset_case_id": "stale-scorecard-case"},
			},
		},
	})
	if err != nil {
		t.Fatalf("SubmitGroup failed: %v", err)
	}

	items, err := controller.ListGroupItems(ctx, group.ID)
	if err != nil {
		t.Fatalf("ListGroupItems failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("ListGroupItems len = %d, want 1", len(items))
	}
	item := items[0]

	now := time.Now().UTC()
	run := &Run{
		ID:           uuid.NewString(),
		RootRunID:    "",
		GroupID:      group.ID,
		GroupItemID:  item.ID,
		AttemptIndex: 1,
		Kind:         RunKindAgentTask,
		Status:       RunStatusFailed,
		UserID:       "user-1",
		Goal:         "summarize",
		Error:        "proxy returned 502",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	run.RootRunID = run.ID
	if err := controller.store.CreateRun(ctx, run); err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	item.LatestRunID = run.ID
	item.AttemptCount = 1
	item.Status = RunGroupItemStatusError
	if err := controller.store.UpdateGroupItem(ctx, &item); err != nil {
		t.Fatalf("UpdateGroupItem failed: %v", err)
	}

	staleCard := Scorecard{
		ID:          uuid.NewString(),
		GroupID:     group.ID,
		GroupItemID: item.ID,
		RunID:       run.ID,
		Mode:        ScoringModeRule,
		Verdict:     ScoreVerdictError,
		Score:       0.49,
		BreakdownJSON: marshalInterface(map[string]interface{}{
			"failure_label":       "infra_provider_auth",
			"retryable":           false,
			"verification_passed": false,
		}),
		CreatedAt: now.Add(10 * time.Millisecond),
	}
	if err := controller.store.AttachScorecard(ctx, staleCard); err != nil {
		t.Fatalf("AttachScorecard failed: %v", err)
	}

	completedAt := now.Add(20 * time.Millisecond)
	run.Status = RunStatusCompleted
	run.Error = ""
	run.Result = "verification passed"
	run.UpdatedAt = completedAt
	run.FinishedAt = &completedAt
	if err := controller.store.UpdateRun(ctx, run); err != nil {
		t.Fatalf("UpdateRun failed: %v", err)
	}
	if err := controller.store.AppendEvent(ctx, RunEvent{
		RunID:       run.ID,
		Type:        "run_completed",
		Message:     "verification passed",
		CreatedAt:   completedAt,
		PayloadJSON: "{}",
	}); err != nil {
		t.Fatalf("AppendEvent failed: %v", err)
	}

	report, err := controller.GetGroupReport(ctx, group.ID)
	if err != nil {
		t.Fatalf("GetGroupReport failed: %v", err)
	}

	latest, err := controller.store.LatestScorecardForItem(ctx, item.ID)
	if err != nil {
		t.Fatalf("LatestScorecardForItem failed: %v", err)
	}
	if latest == nil {
		t.Fatal("LatestScorecardForItem returned nil")
	}
	if latest.Verdict != ScoreVerdictPass {
		t.Fatalf("latest verdict = %q, want %q", latest.Verdict, ScoreVerdictPass)
	}

	reloadedItems, err := controller.ListGroupItems(ctx, group.ID)
	if err != nil {
		t.Fatalf("ListGroupItems(reload) failed: %v", err)
	}
	if len(reloadedItems) != 1 {
		t.Fatalf("ListGroupItems(reload) len = %d, want 1", len(reloadedItems))
	}
	if reloadedItems[0].Status != RunGroupItemStatusPassed {
		t.Fatalf("reloaded item status = %q, want %q", reloadedItems[0].Status, RunGroupItemStatusPassed)
	}
	if report == nil || len(report.Scorecards) == 0 {
		t.Fatalf("GetGroupReport scorecards = %#v, want non-empty", report)
	}
}

func TestController_GetGroupPreservesCancelReasonAcrossRefresh(t *testing.T) {
	controller := newTestController(t)

	group, err := controller.SubmitGroup(context.Background(), RunGroupSpec{
		Kind:        RunGroupKindEval,
		Title:       "cancel reason",
		OwnerUserID: "user-1",
		Items: []RunGroupItemSpec{
			{
				RunKind: RunKindAgentTask,
				Input:   map[string]interface{}{"goal": "cancel me"},
			},
		},
	})
	if err != nil {
		t.Fatalf("SubmitGroup failed: %v", err)
	}

	if err := controller.CancelGroup(context.Background(), group.ID, "user stopped the run"); err != nil {
		t.Fatalf("CancelGroup failed: %v", err)
	}

	stored, err := controller.store.GetGroup(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("GetGroup(stored) failed: %v", err)
	}
	if stored.Summary["cancel_reason"] != "user stopped the run" {
		t.Fatalf("stored cancel_reason = %#v, want %q", stored.Summary["cancel_reason"], "user stopped the run")
	}
	storedCounts := nestedMetadataMap(stored.Summary, "counts")
	if gotCount := intMetadata(storedCounts[string(RunGroupItemStatusCancelled)]); gotCount != 1 {
		t.Fatalf("stored summary counts.cancelled = %#v, want 1", storedCounts[string(RunGroupItemStatusCancelled)])
	}

	time.Sleep(20 * time.Millisecond)

	got, err := controller.GetGroup(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("GetGroup failed: %v", err)
	}
	if got.Status != RunGroupStatusCancelled {
		t.Fatalf("status = %q, want %q", got.Status, RunGroupStatusCancelled)
	}
	if got.FinishedAt == nil {
		t.Fatal("expected cancelled group to keep FinishedAt")
	}
	if got.Summary["cancel_reason"] != "user stopped the run" {
		t.Fatalf("cancel_reason = %#v, want %q", got.Summary["cancel_reason"], "user stopped the run")
	}
	counts := nestedMetadataMap(got.Summary, "counts")
	if gotCount := intMetadata(counts[string(RunGroupItemStatusCancelled)]); gotCount != 1 {
		t.Fatalf("summary counts.cancelled = %#v, want 1", counts[string(RunGroupItemStatusCancelled)])
	}
}

func TestController_RetryFailedGroupClearsFinishedAtAndRequeuesItems(t *testing.T) {
	controller := newTestController(t)

	group, err := controller.SubmitGroup(context.Background(), RunGroupSpec{
		Kind:        RunGroupKindEval,
		Title:       "retry failed group",
		OwnerUserID: "user-1",
		SchedulerConfig: GroupSchedulerConfig{
			MaxAttempts: 2,
		},
		Items: []RunGroupItemSpec{
			{
				RunKind: RunKindAgentTask,
				Input:   map[string]interface{}{"goal": "retry me"},
			},
		},
	})
	if err != nil {
		t.Fatalf("SubmitGroup failed: %v", err)
	}

	items, err := controller.ListGroupItems(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("ListGroupItems failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	items[0].Status = RunGroupItemStatusFailed
	items[0].AttemptCount = 1
	if err := controller.store.UpdateGroupItem(context.Background(), &items[0]); err != nil {
		t.Fatalf("UpdateGroupItem failed: %v", err)
	}

	now := time.Now().UTC()
	group.Status = RunGroupStatusFailed
	group.FinishedAt = &now
	group.Summary = map[string]interface{}{
		"item_count": 1,
		"counts": map[string]interface{}{
			"failed": 1,
		},
	}
	if err := controller.store.UpdateGroup(context.Background(), group); err != nil {
		t.Fatalf("UpdateGroup failed: %v", err)
	}

	retried, err := controller.RetryFailedGroup(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("RetryFailedGroup failed: %v", err)
	}
	if retried != 1 {
		t.Fatalf("retried = %d, want 1", retried)
	}

	stored, err := controller.store.GetGroup(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("GetGroup(stored) failed: %v", err)
	}
	storedCounts := nestedMetadataMap(stored.Summary, "counts")
	if gotCount := intMetadata(storedCounts[string(RunGroupItemStatusQueued)]); gotCount != 1 {
		t.Fatalf("stored summary counts.queued = %#v, want 1", storedCounts[string(RunGroupItemStatusQueued)])
	}
	if stored.FinishedAt != nil {
		t.Fatalf("stored FinishedAt = %v, want nil", *stored.FinishedAt)
	}

	got, err := controller.GetGroup(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("GetGroup failed: %v", err)
	}
	if got.Status != RunGroupStatusQueued {
		t.Fatalf("status = %q, want %q", got.Status, RunGroupStatusQueued)
	}
	if got.FinishedAt != nil {
		t.Fatalf("FinishedAt = %v, want nil", *got.FinishedAt)
	}
	counts := nestedMetadataMap(got.Summary, "counts")
	if gotCount := intMetadata(counts[string(RunGroupItemStatusQueued)]); gotCount != 1 {
		t.Fatalf("summary counts.queued = %#v, want 1", counts[string(RunGroupItemStatusQueued)])
	}

	items, err = controller.ListGroupItems(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("ListGroupItems(after retry) failed: %v", err)
	}
	if len(items) != 1 || items[0].Status != RunGroupItemStatusQueued {
		t.Fatalf("unexpected retried items: %#v", items)
	}
	if items[0].LeaseOwner != "" {
		t.Fatalf("LeaseOwner = %q, want empty", items[0].LeaseOwner)
	}
	if items[0].LeaseExpiresAt == nil {
		t.Fatal("expected retried item to have backoff lease")
	}
}

func TestController_ListGroupsPreservesStableTerminalSummaryAnnotations(t *testing.T) {
	controller := newTestController(t)

	group, err := controller.SubmitGroup(context.Background(), RunGroupSpec{
		Kind:        RunGroupKindEval,
		Title:       "stable terminal group",
		OwnerUserID: "user-1",
	})
	if err != nil {
		t.Fatalf("SubmitGroup failed: %v", err)
	}

	group.Summary = map[string]interface{}{
		"item_count": 0,
		"sticky":     "keep-me",
	}
	if err := controller.store.UpdateGroup(context.Background(), group); err != nil {
		t.Fatalf("UpdateGroup failed: %v", err)
	}

	groups, err := controller.ListGroups(context.Background(), RunGroupFilter{
		OwnerUserID: "user-1",
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("ListGroups failed: %v", err)
	}

	var listed *RunGroup
	for i := range groups {
		if groups[i].ID == group.ID {
			listed = &groups[i]
			break
		}
	}
	if listed == nil {
		t.Fatalf("group %q not returned", group.ID)
	}
	if listed.Summary["sticky"] != "keep-me" {
		t.Fatalf("sticky summary annotation = %#v, want %q", listed.Summary["sticky"], "keep-me")
	}
}

func TestController_ListGroupsRefreshesActiveGroupSummaries(t *testing.T) {
	controller := newTestController(t)

	group, err := controller.SubmitGroup(context.Background(), RunGroupSpec{
		Kind:        RunGroupKindEval,
		Title:       "active group refresh",
		OwnerUserID: "user-1",
		Items: []RunGroupItemSpec{
			{
				RunKind: RunKindAgentTask,
				Input:   map[string]interface{}{"goal": "run"},
			},
		},
	})
	if err != nil {
		t.Fatalf("SubmitGroup failed: %v", err)
	}

	items, err := controller.ListGroupItems(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("ListGroupItems failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	items[0].Status = RunGroupItemStatusRunning
	items[0].AttemptCount = 1
	if err := controller.store.UpdateGroupItem(context.Background(), &items[0]); err != nil {
		t.Fatalf("UpdateGroupItem failed: %v", err)
	}

	groups, err := controller.ListGroups(context.Background(), RunGroupFilter{
		OwnerUserID: "user-1",
		Statuses:    []RunGroupStatus{RunGroupStatusQueued, RunGroupStatusRunning},
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("ListGroups failed: %v", err)
	}

	var listed *RunGroup
	for i := range groups {
		if groups[i].ID == group.ID {
			listed = &groups[i]
			break
		}
	}
	if listed == nil {
		t.Fatalf("group %q not returned", group.ID)
	}
	if listed.Status != RunGroupStatusRunning {
		t.Fatalf("status = %q, want %q", listed.Status, RunGroupStatusRunning)
	}
	if listed.StartedAt == nil {
		t.Fatal("expected running group to have StartedAt set")
	}
	counts := nestedMetadataMap(listed.Summary, "counts")
	if gotCount := intMetadata(counts[string(RunGroupItemStatusRunning)]); gotCount != 1 {
		t.Fatalf("summary counts.running = %#v, want 1", counts[string(RunGroupItemStatusRunning)])
	}
}

func TestController_GetGroupReportSyncsLinkedRuns(t *testing.T) {
	controller := newTestController(t)
	driver := &stagedSnapshotDriver{
		kind:         RunKindAgentTask,
		controller:   controller,
		syncStatuses: []RunStatus{RunStatusPending, RunStatusExecuting},
	}
	controller.RegisterDriver(driver)

	group, err := controller.SubmitGroup(context.Background(), RunGroupSpec{
		Kind:        RunGroupKindEval,
		Title:       "report linked runs",
		OwnerUserID: "user-1",
		Items: []RunGroupItemSpec{
			{
				RunKind: RunKindAgentTask,
				Input:   map[string]interface{}{"goal": "sync linked run"},
			},
		},
	})
	if err != nil {
		t.Fatalf("SubmitGroup failed: %v", err)
	}

	items, err := controller.ListGroupItems(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("ListGroupItems failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}

	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:        RunKindAgentTask,
		Goal:        "sync linked run",
		UserID:      "user-1",
		GroupID:     group.ID,
		GroupItemID: items[0].ID,
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if run.Status != RunStatusPending {
		t.Fatalf("initial run status = %s, want pending", run.Status)
	}

	items[0].Status = RunGroupItemStatusRunning
	items[0].AttemptCount = 1
	items[0].LatestRunID = run.ID
	if err := controller.store.UpdateGroupItem(context.Background(), &items[0]); err != nil {
		t.Fatalf("UpdateGroupItem failed: %v", err)
	}

	report, err := controller.GetGroupReport(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("GetGroupReport failed: %v", err)
	}
	if report.Group == nil || report.Group.Status != RunGroupStatusRunning {
		t.Fatalf("group status = %#v, want running", report.Group)
	}
	if len(report.LinkedRuns) != 1 {
		t.Fatalf("linked runs len = %d, want 1", len(report.LinkedRuns))
	}
	if report.LinkedRuns[0].Status != RunStatusExecuting {
		t.Fatalf("linked run status = %s, want executing", report.LinkedRuns[0].Status)
	}
	if report.LinkedRuns[0].StartedAt == nil {
		t.Fatal("expected synced linked run to have StartedAt set")
	}

	stored, err := controller.store.GetRun(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("GetRun(stored) failed: %v", err)
	}
	if stored.Status != RunStatusExecuting {
		t.Fatalf("stored run status = %s, want executing", stored.Status)
	}
}

func TestController_GetGroupReportAggregatesMetricsAndFailedItems(t *testing.T) {
	controller := newTestController(t)
	controller.RegisterDriver(&blockingGroupDriver{kind: RunKindAgentTask})

	group, err := controller.SubmitGroup(context.Background(), RunGroupSpec{
		Kind:        RunGroupKindEval,
		Title:       "report aggregates",
		OwnerUserID: "user-1",
		Items: []RunGroupItemSpec{
			{
				RunKind: RunKindAgentTask,
				Input:   map[string]interface{}{"goal": "ship passing task"},
			},
			{
				RunKind: RunKindAgentTask,
				Input:   map[string]interface{}{"goal": "ship failing task"},
			},
		},
	})
	if err != nil {
		t.Fatalf("SubmitGroup failed: %v", err)
	}

	items, err := controller.ListGroupItems(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("ListGroupItems failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items len = %d, want 2", len(items))
	}

	passingRun, err := controller.Submit(context.Background(), RunSpec{
		Kind:        RunKindAgentTask,
		Goal:        "ship passing task",
		UserID:      "user-1",
		GroupID:     group.ID,
		GroupItemID: items[0].ID,
	})
	if err != nil {
		t.Fatalf("Submit(passing) failed: %v", err)
	}
	failingRun, err := controller.Submit(context.Background(), RunSpec{
		Kind:        RunKindAgentTask,
		Goal:        "ship failing task",
		UserID:      "user-1",
		GroupID:     group.ID,
		GroupItemID: items[1].ID,
	})
	if err != nil {
		t.Fatalf("Submit(failing) failed: %v", err)
	}

	passSnapshot := *passingRun
	passSnapshot.Status = RunStatusCompleted
	passFinished := time.Now().UTC()
	passSnapshot.FinishedAt = &passFinished
	if err := controller.SyncSnapshot(context.Background(), &passSnapshot); err != nil {
		t.Fatalf("SyncSnapshot(passing) failed: %v", err)
	}
	failSnapshot := *failingRun
	failSnapshot.Status = RunStatusFailed
	failSnapshot.Error = "missing artifact"
	failFinished := time.Now().UTC()
	failSnapshot.FinishedAt = &failFinished
	if err := controller.SyncSnapshot(context.Background(), &failSnapshot); err != nil {
		t.Fatalf("SyncSnapshot(failing) failed: %v", err)
	}

	items[0].Status = RunGroupItemStatusPassed
	items[0].AttemptCount = 1
	items[0].LatestRunID = passingRun.ID
	if err := controller.store.UpdateGroupItem(context.Background(), &items[0]); err != nil {
		t.Fatalf("UpdateGroupItem(passing) failed: %v", err)
	}
	items[1].Status = RunGroupItemStatusFailed
	items[1].AttemptCount = 2
	items[1].LatestRunID = failingRun.ID
	if err := controller.store.UpdateGroupItem(context.Background(), &items[1]); err != nil {
		t.Fatalf("UpdateGroupItem(failing) failed: %v", err)
	}

	now := time.Now().UTC()
	if err := controller.store.AttachScorecard(context.Background(), Scorecard{
		ID:            "score-pass",
		GroupID:       group.ID,
		GroupItemID:   items[0].ID,
		RunID:         passingRun.ID,
		Mode:          ScoringModeRule,
		Verdict:       ScoreVerdictPass,
		Score:         1,
		BreakdownJSON: `{"verification_passed":true,"evidence_score":0.95}`,
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("AttachScorecard(pass) failed: %v", err)
	}
	if err := controller.store.AttachScorecard(context.Background(), Scorecard{
		ID:            "score-fail",
		GroupID:       group.ID,
		GroupItemID:   items[1].ID,
		RunID:         failingRun.ID,
		Mode:          ScoringModeRule,
		Verdict:       ScoreVerdictFail,
		Score:         0,
		BreakdownJSON: `{"verification_passed":false,"failure_label":"missing_artifact"}`,
		CreatedAt:     now.Add(time.Millisecond),
	}); err != nil {
		t.Fatalf("AttachScorecard(fail) failed: %v", err)
	}

	report, err := controller.GetGroupReport(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("GetGroupReport failed: %v", err)
	}
	if report.OverallScore != 0.5 {
		t.Fatalf("overall_score = %#v, want 0.5", report.OverallScore)
	}
	if report.PassRate != 0.5 {
		t.Fatalf("pass_rate = %#v, want 0.5", report.PassRate)
	}
	if report.VerdictCounts[string(ScoreVerdictPass)] != 1 || report.VerdictCounts[string(ScoreVerdictFail)] != 1 {
		t.Fatalf("verdict counts = %#v, want pass=1 fail=1", report.VerdictCounts)
	}
	if len(report.FailedItems) != 1 {
		t.Fatalf("failed_items = %#v, want 1 entry", report.FailedItems)
	}
	failedRun, ok := report.FailedItems[0]["run"].(*Run)
	if !ok || failedRun == nil {
		t.Fatalf("failed item run = %#v, want *Run", report.FailedItems[0]["run"])
	}
	if failedRun.ID != failingRun.ID {
		t.Fatalf("failed item run id = %q, want %q", failedRun.ID, failingRun.ID)
	}
	if got := report.Group.Summary["overall_score"]; got != float64(0.5) {
		t.Fatalf("summary.overall_score = %#v, want 0.5", got)
	}
	if got := report.Group.Summary["pass_rate"]; got != float64(0.5) {
		t.Fatalf("summary.pass_rate = %#v, want 0.5", got)
	}
}

func TestController_GetGroupReportIncludesRuntimeEvidenceAndCheckpointArtifacts(t *testing.T) {
	controller := newTestController(t)
	driver := &runtimeEvidenceDriver{
		kind:       RunKindAgentTask,
		evidenceBy: make(map[string][]RuntimeEvidenceEntry),
	}
	controller.RegisterDriver(driver)

	group, err := controller.SubmitGroup(context.Background(), RunGroupSpec{
		Kind:        RunGroupKindEval,
		Title:       "report runtime bundle",
		OwnerUserID: "user-1",
		Items: []RunGroupItemSpec{
			{
				RunKind: RunKindAgentTask,
				Input:   map[string]interface{}{"goal": "collect runtime bundle"},
			},
		},
	})
	if err != nil {
		t.Fatalf("SubmitGroup failed: %v", err)
	}

	items, err := controller.ListGroupItems(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("ListGroupItems failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}

	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:        RunKindAgentTask,
		Goal:        "collect runtime bundle",
		UserID:      "user-1",
		GroupID:     group.ID,
		GroupItemID: items[0].ID,
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	driver.evidenceBy[run.ID] = []RuntimeEvidenceEntry{
		{
			ID:           "evidence-" + run.ID,
			RunID:        run.ID,
			StepIndex:    1,
			PlannerRound: 1,
			EventType:    "tool_call",
			Summary:      "opened repository docs",
			PayloadJSON:  `{"tool":"web.open"}`,
			CreatedAt:    time.Now().UTC(),
		},
	}

	snapshot := *run
	snapshot.Status = RunStatusCompleted
	finishedAt := time.Now().UTC()
	snapshot.FinishedAt = &finishedAt
	if err := controller.SyncSnapshot(context.Background(), &snapshot); err != nil {
		t.Fatalf("SyncSnapshot failed: %v", err)
	}

	items[0].Status = RunGroupItemStatusPassed
	items[0].AttemptCount = 1
	items[0].LatestRunID = run.ID
	if err := controller.store.UpdateGroupItem(context.Background(), &items[0]); err != nil {
		t.Fatalf("UpdateGroupItem failed: %v", err)
	}

	if err := controller.AttachArtifact(context.Background(), ArtifactRef{
		ID:           "checkpoint-" + run.ID,
		RunID:        run.ID,
		Kind:         "checkpoint",
		Label:        "checkpoint-1",
		PathOrURL:    "checkpoint-1.json",
		MetadataJSON: marshalMetadata(map[string]interface{}{"summary": "resume from group report"}),
	}); err != nil {
		t.Fatalf("AttachArtifact failed: %v", err)
	}

	report, err := controller.GetGroupReport(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("GetGroupReport failed: %v", err)
	}
	if len(report.LinkedRuns) != 1 {
		t.Fatalf("linked runs len = %d, want 1", len(report.LinkedRuns))
	}
	evidence := report.RuntimeEvidence[run.ID]
	if len(evidence) != 1 {
		t.Fatalf("runtime evidence = %#v, want 1 entry for run %q", report.RuntimeEvidence, run.ID)
	}
	if evidence[0].EventType != "tool_call" {
		t.Fatalf("runtime evidence event_type = %q, want tool_call", evidence[0].EventType)
	}
	if evidence[0].Summary != "opened repository docs" {
		t.Fatalf("runtime evidence summary = %q, want opened repository docs", evidence[0].Summary)
	}
	if len(report.Artifacts) != 1 {
		t.Fatalf("artifacts len = %d, want 1", len(report.Artifacts))
	}
	if report.Artifacts[0].ID != "checkpoint-"+run.ID {
		t.Fatalf("artifact id = %q, want %q", report.Artifacts[0].ID, "checkpoint-"+run.ID)
	}
	if len(report.Checkpoints) != 1 {
		t.Fatalf("checkpoints len = %d, want 1", len(report.Checkpoints))
	}
	if report.Checkpoints[0].RunID != run.ID {
		t.Fatalf("checkpoint run_id = %q, want %q", report.Checkpoints[0].RunID, run.ID)
	}
	if report.Checkpoints[0].GroupItemID != items[0].ID {
		t.Fatalf("checkpoint group_item_id = %q, want %q", report.Checkpoints[0].GroupItemID, items[0].ID)
	}
	if metadataString(report.Checkpoints[0].Payload, "summary") != "resume from group report" {
		t.Fatalf("checkpoint payload = %#v, want summary", report.Checkpoints[0].Payload)
	}
}

func TestController_LoadGroupReportContextRejectsNilGroup(t *testing.T) {
	controller := newTestController(t)

	reportCtx, err := controller.loadGroupReportContext(context.Background(), nil)
	if err == nil {
		t.Fatalf("loadGroupReportContext returned %#v, want error", reportCtx)
	}
	if err.Error() != "group is required" {
		t.Fatalf("error = %v, want group is required", err)
	}
}
