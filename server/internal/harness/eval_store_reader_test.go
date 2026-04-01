package harness

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestSQLiteStore_EvalReadsUseReaderDB(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "harness-eval-reader.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open write sqlite: %v", err)
	}
	readDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("open read sqlite: %v", err)
	}
	t.Cleanup(func() { _ = readDB.Close() })

	store, err := NewSQLiteStoreWithReadDB(writeDB, readDB)
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("NewSQLiteStoreWithReadDB failed: %v", err)
	}

	dataset := &Dataset{
		ID:             "dataset-1",
		Name:           "Dataset 1",
		OwnerUserID:    "user-1",
		Subject:        "routing",
		DefaultRunKind: RunKindResearch,
	}
	if err := store.CreateDataset(ctx, dataset); err != nil {
		_ = writeDB.Close()
		t.Fatalf("CreateDataset failed: %v", err)
	}

	version := &DatasetVersion{
		ID:         "version-1",
		DatasetID:  dataset.ID,
		Version:    "v1",
		ItemCount:  1,
		Manifest:   map[string]interface{}{"items": []interface{}{}},
		CreatedBy:  "user-1",
		SourceType: "seed",
	}
	if err := store.CreateDatasetVersion(ctx, version); err != nil {
		_ = writeDB.Close()
		t.Fatalf("CreateDatasetVersion failed: %v", err)
	}

	spec := &EvalSpec{
		ID:               "spec-1",
		Name:             "Spec 1",
		OwnerUserID:      "user-1",
		Subject:          "routing",
		RunKind:          RunKindResearch,
		Profile:          "default",
		DatasetID:        dataset.ID,
		DatasetVersionID: version.ID,
	}
	if err := store.CreateEvalSpec(ctx, spec); err != nil {
		_ = writeDB.Close()
		t.Fatalf("CreateEvalSpec failed: %v", err)
	}

	evalRun := &EvalRun{
		ID:               "eval-run-1",
		EvalSpecID:       spec.ID,
		GroupID:          "group-1",
		DatasetVersionID: version.ID,
		Title:            "Eval Run 1",
		OwnerUserID:      "user-1",
		Status:           RunGroupStatusCompleted,
		TriggerKind:      "manual",
	}
	if err := store.CreateEvalRun(ctx, evalRun); err != nil {
		_ = writeDB.Close()
		t.Fatalf("CreateEvalRun failed: %v", err)
	}

	baseline := &Baseline{
		ID:          "baseline-1",
		Name:        "Baseline 1",
		OwnerUserID: "user-1",
		Subject:     "routing",
		EvalSpecID:  spec.ID,
		EvalRunID:   evalRun.ID,
		IsDefault:   true,
	}
	if err := store.CreateBaseline(ctx, baseline); err != nil {
		_ = writeDB.Close()
		t.Fatalf("CreateBaseline failed: %v", err)
	}

	report := &ComparisonReport{
		ID:              "comparison-1",
		OwnerUserID:     "user-1",
		BaselineID:      baseline.ID,
		EvalSpecID:      spec.ID,
		BaseEvalRunID:   evalRun.ID,
		TargetEvalRunID: evalRun.ID,
		Summary:         map[string]interface{}{"result": "ok"},
	}
	if err := store.CreateComparisonReport(ctx, report); err != nil {
		_ = writeDB.Close()
		t.Fatalf("CreateComparisonReport failed: %v", err)
	}

	if err := writeDB.Close(); err != nil {
		t.Fatalf("close write sqlite: %v", err)
	}

	gotDataset, err := store.GetDataset(ctx, dataset.ID)
	if err != nil {
		t.Fatalf("GetDataset failed after closing write db: %v", err)
	}
	if gotDataset.ID != dataset.ID {
		t.Fatalf("GetDataset returned %q, want %q", gotDataset.ID, dataset.ID)
	}

	datasets, err := store.ListDatasets(ctx, DatasetFilter{OwnerUserID: "user-1", Limit: 10})
	if err != nil {
		t.Fatalf("ListDatasets failed after closing write db: %v", err)
	}
	if len(datasets) != 1 || datasets[0].ID != dataset.ID {
		t.Fatalf("ListDatasets returned %#v", datasets)
	}

	gotVersion, err := store.GetDatasetVersion(ctx, version.ID)
	if err != nil {
		t.Fatalf("GetDatasetVersion failed after closing write db: %v", err)
	}
	if gotVersion.ID != version.ID {
		t.Fatalf("GetDatasetVersion returned %q, want %q", gotVersion.ID, version.ID)
	}

	versions, err := store.ListDatasetVersions(ctx, dataset.ID, 10)
	if err != nil {
		t.Fatalf("ListDatasetVersions failed after closing write db: %v", err)
	}
	if len(versions) != 1 || versions[0].ID != version.ID {
		t.Fatalf("ListDatasetVersions returned %#v", versions)
	}

	gotSpec, err := store.GetEvalSpec(ctx, spec.ID)
	if err != nil {
		t.Fatalf("GetEvalSpec failed after closing write db: %v", err)
	}
	if gotSpec.ID != spec.ID {
		t.Fatalf("GetEvalSpec returned %q, want %q", gotSpec.ID, spec.ID)
	}

	specs, err := store.ListEvalSpecs(ctx, EvalSpecFilter{OwnerUserID: "user-1", DatasetID: dataset.ID, Limit: 10})
	if err != nil {
		t.Fatalf("ListEvalSpecs failed after closing write db: %v", err)
	}
	if len(specs) != 1 || specs[0].ID != spec.ID {
		t.Fatalf("ListEvalSpecs returned %#v", specs)
	}

	gotEvalRun, err := store.GetEvalRun(ctx, evalRun.ID)
	if err != nil {
		t.Fatalf("GetEvalRun failed after closing write db: %v", err)
	}
	if gotEvalRun.ID != evalRun.ID {
		t.Fatalf("GetEvalRun returned %q, want %q", gotEvalRun.ID, evalRun.ID)
	}

	evalRuns, err := store.ListEvalRuns(ctx, EvalRunFilter{
		OwnerUserID: "user-1",
		EvalSpecID:  spec.ID,
		Statuses:    []RunGroupStatus{RunGroupStatusCompleted},
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("ListEvalRuns failed after closing write db: %v", err)
	}
	if len(evalRuns) != 1 || evalRuns[0].ID != evalRun.ID {
		t.Fatalf("ListEvalRuns returned %#v", evalRuns)
	}

	gotBaseline, err := store.GetBaseline(ctx, baseline.ID)
	if err != nil {
		t.Fatalf("GetBaseline failed after closing write db: %v", err)
	}
	if gotBaseline.ID != baseline.ID {
		t.Fatalf("GetBaseline returned %q, want %q", gotBaseline.ID, baseline.ID)
	}

	baselines, err := store.ListBaselines(ctx, BaselineFilter{OwnerUserID: "user-1", EvalSpecID: spec.ID, Limit: 10})
	if err != nil {
		t.Fatalf("ListBaselines failed after closing write db: %v", err)
	}
	if len(baselines) != 1 || baselines[0].ID != baseline.ID {
		t.Fatalf("ListBaselines returned %#v", baselines)
	}

	gotReport, err := store.GetComparisonReport(ctx, report.ID)
	if err != nil {
		t.Fatalf("GetComparisonReport failed after closing write db: %v", err)
	}
	if gotReport.ID != report.ID {
		t.Fatalf("GetComparisonReport returned %q, want %q", gotReport.ID, report.ID)
	}
}

func TestController_GetEvalRunReportFallsBackWhenReaderMissesGroupEvalContext(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	staleReader, err := sql.Open("sqlite3", filepath.Join(tmpDir, "stale-reader.db"))
	if err != nil {
		t.Fatalf("open stale reader sqlite: %v", err)
	}
	t.Cleanup(func() { _ = staleReader.Close() })
	if _, err := NewSQLiteStoreWithReadDB(staleReader, staleReader); err != nil {
		t.Fatalf("initialize stale reader schema: %v", err)
	}

	controller := newTestControllerWithReadDB(t, staleReader)

	dataset := &Dataset{
		ID:             "dataset-fallback",
		Name:           "Dataset fallback",
		OwnerUserID:    "user-1",
		Subject:        "routing",
		DefaultRunKind: RunKindResearch,
	}
	if err := controller.store.CreateDataset(ctx, dataset); err != nil {
		t.Fatalf("CreateDataset failed: %v", err)
	}

	version := &DatasetVersion{
		ID:         "version-fallback",
		DatasetID:  dataset.ID,
		Version:    "v1",
		ItemCount:  0,
		Manifest:   map[string]interface{}{"items": []interface{}{}},
		CreatedBy:  "user-1",
		SourceType: "seed",
	}
	if err := controller.store.CreateDatasetVersion(ctx, version); err != nil {
		t.Fatalf("CreateDatasetVersion failed: %v", err)
	}

	spec := &EvalSpec{
		ID:               "spec-fallback",
		Name:             "Spec fallback",
		OwnerUserID:      "user-1",
		Subject:          "routing",
		RunKind:          RunKindResearch,
		Profile:          "default",
		DatasetID:        dataset.ID,
		DatasetVersionID: version.ID,
	}
	if err := controller.store.CreateEvalSpec(ctx, spec); err != nil {
		t.Fatalf("CreateEvalSpec failed: %v", err)
	}

	now := time.Now().UTC()
	group := &RunGroup{
		ID:          "group-fallback",
		Kind:        RunGroupKindEval,
		Title:       "group fallback",
		Status:      RunGroupStatusCompleted,
		OwnerUserID: "user-1",
		Summary:     map[string]interface{}{"item_count": 0},
		CreatedAt:   now,
		UpdatedAt:   now,
		StartedAt:   &now,
		FinishedAt:  &now,
	}
	if err := controller.store.CreateGroup(ctx, group); err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}

	evalRun := &EvalRun{
		ID:               "eval-run-fallback",
		EvalSpecID:       spec.ID,
		GroupID:          group.ID,
		DatasetVersionID: version.ID,
		Title:            "Eval Run fallback",
		OwnerUserID:      "user-1",
		Status:           RunGroupStatusCompleted,
		TriggerKind:      "manual",
		CreatedAt:        now,
		UpdatedAt:        now,
		StartedAt:        &now,
		FinishedAt:       &now,
	}
	if err := controller.store.CreateEvalRun(ctx, evalRun); err != nil {
		t.Fatalf("CreateEvalRun failed: %v", err)
	}

	report, err := controller.GetEvalRunReport(ctx, evalRun.ID)
	if err != nil {
		t.Fatalf("GetEvalRunReport fallback failed: %v", err)
	}
	if report == nil || report.EvalRun == nil || report.GroupReport == nil {
		t.Fatalf("GetEvalRunReport returned %#v", report)
	}
	if report.EvalRun.ID != evalRun.ID {
		t.Fatalf("report eval run = %q, want %q", report.EvalRun.ID, evalRun.ID)
	}
	if report.GroupReport.Group == nil || report.GroupReport.Group.ID != group.ID {
		t.Fatalf("report group = %#v, want id %q", report.GroupReport.Group, group.ID)
	}
}
