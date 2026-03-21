package harness

import (
	"context"
	"testing"
	"time"
)

func TestSQLiteStore_EvalObjectsRoundTrip(t *testing.T) {
	store := newTestHarnessStore(t)
	ctx := context.Background()

	dataset := &Dataset{
		ID:             "dataset-1",
		Name:           "Smoke Dataset",
		OwnerUserID:    "user-1",
		Subject:        "agent_task",
		DefaultRunKind: RunKindAgentTask,
		DefaultProfile: "smoke",
	}
	if err := store.CreateDataset(ctx, dataset); err != nil {
		t.Fatalf("CreateDataset failed: %v", err)
	}

	version := &DatasetVersion{
		ID:             "dataset-version-1",
		DatasetID:      dataset.ID,
		Version:        "v1",
		ManifestSHA256: manifestSHA256(map[string]interface{}{"items": []interface{}{map[string]interface{}{"id": "case-1"}}}),
		ItemCount:      1,
		Manifest: map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{
					"id": "case-1",
					"input": map[string]interface{}{
						"goal": "finish the task",
					},
				},
			},
		},
		CreatedBy: "user-1",
		CreatedAt: time.Now().UTC(),
	}
	if err := store.CreateDatasetVersion(ctx, version); err != nil {
		t.Fatalf("CreateDatasetVersion failed: %v", err)
	}

	dataset.ActiveVersionID = version.ID
	if err := store.UpdateDataset(ctx, dataset); err != nil {
		t.Fatalf("UpdateDataset failed: %v", err)
	}

	evalSpec := &EvalSpec{
		ID:               "eval-spec-1",
		Name:             "Smoke Eval",
		OwnerUserID:      "user-1",
		RunKind:          RunKindAgentTask,
		Profile:          "smoke",
		DatasetID:        dataset.ID,
		DatasetVersionID: version.ID,
		ScoringConfig: GroupScoringConfig{
			Mode: ScoringModeHybrid,
		},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := store.CreateEvalSpec(ctx, evalSpec); err != nil {
		t.Fatalf("CreateEvalSpec failed: %v", err)
	}

	evalRun := &EvalRun{
		ID:               "eval-run-1",
		EvalSpecID:       evalSpec.ID,
		GroupID:          "group-1",
		DatasetVersionID: version.ID,
		OwnerUserID:      "user-1",
		Status:           RunGroupStatusQueued,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	if err := store.CreateEvalRun(ctx, evalRun); err != nil {
		t.Fatalf("CreateEvalRun failed: %v", err)
	}

	gotDataset, err := store.GetDataset(ctx, dataset.ID)
	if err != nil {
		t.Fatalf("GetDataset failed: %v", err)
	}
	if gotDataset.ActiveVersionID != version.ID {
		t.Fatalf("dataset ActiveVersionID = %q, want %q", gotDataset.ActiveVersionID, version.ID)
	}

	versions, err := store.ListDatasetVersions(ctx, dataset.ID, 10)
	if err != nil {
		t.Fatalf("ListDatasetVersions failed: %v", err)
	}
	if len(versions) != 1 || versions[0].ID != version.ID {
		t.Fatalf("unexpected versions: %#v", versions)
	}

	specs, err := store.ListEvalSpecs(ctx, EvalSpecFilter{OwnerUserID: "user-1", Limit: 10})
	if err != nil {
		t.Fatalf("ListEvalSpecs failed: %v", err)
	}
	if len(specs) != 1 || specs[0].ID != evalSpec.ID {
		t.Fatalf("unexpected eval specs: %#v", specs)
	}

	evalRuns, err := store.ListEvalRuns(ctx, EvalRunFilter{OwnerUserID: "user-1", Limit: 10})
	if err != nil {
		t.Fatalf("ListEvalRuns failed: %v", err)
	}
	if len(evalRuns) != 1 || evalRuns[0].ID != evalRun.ID {
		t.Fatalf("unexpected eval runs: %#v", evalRuns)
	}
}

func TestController_SubmitEvalRunMaterializesGroupAndReport(t *testing.T) {
	controller := newTestController(t)
	controller.RegisterDriver(&autoCompleteGroupDriver{
		kind:   RunKindAgentTask,
		status: RunStatusCompleted,
		result: "verified result",
		delay:  10 * time.Millisecond,
	})

	dataset, err := controller.CreateDataset(context.Background(), DatasetSpec{
		Name:           "Agent Task Smoke",
		OwnerUserID:    "user-1",
		Subject:        "agent_task",
		DefaultRunKind: RunKindAgentTask,
		DefaultProfile: "agent_task",
	})
	if err != nil {
		t.Fatalf("CreateDataset failed: %v", err)
	}

	version, err := controller.CreateDatasetVersion(context.Background(), dataset.ID, DatasetVersionSpec{
		Version: "v1",
		Manifest: map[string]interface{}{
			"defaults": map[string]interface{}{
				"run_kind": "agent_task",
				"profile":  "agent_task",
			},
			"items": []interface{}{
				map[string]interface{}{
					"id": "case-1",
					"input": map[string]interface{}{
						"goal": "finish and verify",
					},
					"expected": map[string]interface{}{
						"contains": "verified",
					},
				},
			},
		},
		CreatedBy: "user-1",
	})
	if err != nil {
		t.Fatalf("CreateDatasetVersion failed: %v", err)
	}

	evalSpec, err := controller.CreateEvalSpec(context.Background(), EvalSpecSpec{
		Name:             "Smoke Eval",
		OwnerUserID:      "user-1",
		DatasetID:        dataset.ID,
		DatasetVersionID: version.ID,
		RunKind:          RunKindAgentTask,
		Profile:          "agent_task",
		ScoringConfig: GroupScoringConfig{
			Mode:          ScoringModeRule,
			PassThreshold: 0.5,
		},
	})
	if err != nil {
		t.Fatalf("CreateEvalSpec failed: %v", err)
	}

	evalRun, err := controller.SubmitEvalRun(context.Background(), EvalRunSpec{
		EvalSpecID:  evalSpec.ID,
		OwnerUserID: "user-1",
		Title:       "smoke-run-1",
	})
	if err != nil {
		t.Fatalf("SubmitEvalRun failed: %v", err)
	}
	if evalRun.GroupID == "" {
		t.Fatalf("expected eval run group id, got %#v", evalRun)
	}

	items, err := controller.ListGroupItems(context.Background(), evalRun.GroupID)
	if err != nil {
		t.Fatalf("ListGroupItems failed: %v", err)
	}
	if len(items) != 1 || items[0].Metadata["dataset_case_id"] != "case-1" {
		t.Fatalf("unexpected materialized items: %#v", items)
	}

	dispatcher := NewGroupDispatcher(controller)
	dispatcher.SetRunPollInterval(10 * time.Millisecond)
	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatalf("DispatchOnce failed: %v", err)
	}

	waitForCondition(t, "eval run completion", func() bool {
		got, err := controller.GetEvalRun(context.Background(), evalRun.ID)
		if err != nil || got == nil {
			return false
		}
		return got.Status == RunGroupStatusCompleted
	})

	report, err := controller.GetEvalRunReport(context.Background(), evalRun.ID)
	if err != nil {
		t.Fatalf("GetEvalRunReport failed: %v", err)
	}
	if report.EvalRun.Status != RunGroupStatusCompleted {
		t.Fatalf("eval run status = %s, want completed", report.EvalRun.Status)
	}
	if report.GroupReport == nil || len(report.GroupReport.Scorecards) != 1 {
		t.Fatalf("unexpected group report: %#v", report.GroupReport)
	}
	if len(report.GroupReport.LinkedRuns) != 1 || report.GroupReport.LinkedRuns[0].GroupID != evalRun.GroupID {
		t.Fatalf("unexpected linked runs: %#v", report.GroupReport.LinkedRuns)
	}
}
