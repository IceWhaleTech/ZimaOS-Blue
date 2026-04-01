package harness

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestController_ResolveComparisonBaseSelectionPrefersExplicitBaseEvalRunID(t *testing.T) {
	controller := newTestController(t)

	selection, err := controller.resolveComparisonBaseSelection(context.Background(), &EvalRun{
		ID:                "target-run",
		EvalSpecID:        "eval-spec-1",
		OwnerUserID:       "user-1",
		BaselineEvalRunID: "embedded-base-run",
	}, CompareEvalRunRequest{
		BaseEvalRunID: "explicit-base-run",
	})
	if err != nil {
		t.Fatalf("resolveComparisonBaseSelection failed: %v", err)
	}
	if selection == nil {
		t.Fatal("expected selection")
	}
	if selection.evalRunID != "explicit-base-run" {
		t.Fatalf("selection.evalRunID = %q, want %q", selection.evalRunID, "explicit-base-run")
	}
	if selection.baseline != nil {
		t.Fatalf("selection.baseline = %#v, want nil", selection.baseline)
	}
}

func TestController_ResolveComparisonBaseSelectionFallsBackToDefaultBaseline(t *testing.T) {
	controller := newTestController(t)
	now := time.Now().UTC()

	if err := controller.store.CreateBaseline(context.Background(), &Baseline{
		ID:          "baseline-non-default",
		Name:        "non-default",
		OwnerUserID: "user-1",
		EvalSpecID:  "eval-spec-1",
		EvalRunID:   "run-non-default",
		IsDefault:   false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		t.Fatalf("CreateBaseline(non-default) failed: %v", err)
	}
	if err := controller.store.CreateBaseline(context.Background(), &Baseline{
		ID:          "baseline-default",
		Name:        "default",
		OwnerUserID: "user-1",
		EvalSpecID:  "eval-spec-1",
		EvalRunID:   "run-default",
		IsDefault:   true,
		CreatedAt:   now.Add(time.Second),
		UpdatedAt:   now.Add(time.Second),
	}); err != nil {
		t.Fatalf("CreateBaseline(default) failed: %v", err)
	}
	if err := controller.store.CreateBaseline(context.Background(), &Baseline{
		ID:          "baseline-other-spec",
		Name:        "other-spec",
		OwnerUserID: "user-1",
		EvalSpecID:  "eval-spec-2",
		EvalRunID:   "run-other-spec",
		IsDefault:   true,
		CreatedAt:   now.Add(2 * time.Second),
		UpdatedAt:   now.Add(2 * time.Second),
	}); err != nil {
		t.Fatalf("CreateBaseline(other spec) failed: %v", err)
	}

	selection, err := controller.resolveComparisonBaseSelection(context.Background(), &EvalRun{
		ID:          "target-run",
		EvalSpecID:  "eval-spec-1",
		OwnerUserID: "user-1",
	}, CompareEvalRunRequest{})
	if err != nil {
		t.Fatalf("resolveComparisonBaseSelection failed: %v", err)
	}
	if selection == nil || selection.baseline == nil {
		t.Fatalf("expected default baseline selection, got %#v", selection)
	}
	if selection.evalRunID != "run-default" {
		t.Fatalf("selection.evalRunID = %q, want %q", selection.evalRunID, "run-default")
	}
	if selection.baseline.ID != "baseline-default" {
		t.Fatalf("selection.baseline.ID = %q, want %q", selection.baseline.ID, "baseline-default")
	}
}

func TestController_ResolveComparisonBaseSelectionFallsBackToTargetBaselineEvalRunID(t *testing.T) {
	controller := newTestController(t)

	selection, err := controller.resolveComparisonBaseSelection(context.Background(), &EvalRun{
		ID:                "target-run",
		EvalSpecID:        "eval-spec-1",
		OwnerUserID:       "user-1",
		BaselineEvalRunID: "embedded-base-run",
	}, CompareEvalRunRequest{})
	if err != nil {
		t.Fatalf("resolveComparisonBaseSelection failed: %v", err)
	}
	if selection == nil {
		t.Fatal("expected selection")
	}
	if selection.evalRunID != "embedded-base-run" {
		t.Fatalf("selection.evalRunID = %q, want %q", selection.evalRunID, "embedded-base-run")
	}
	if selection.baseline != nil {
		t.Fatalf("selection.baseline = %#v, want nil", selection.baseline)
	}
}

func TestController_ResolveComparisonBaseSelectionRejectsNilTargetEvalRun(t *testing.T) {
	controller := newTestController(t)

	selection, err := controller.resolveComparisonBaseSelection(context.Background(), nil, CompareEvalRunRequest{})
	if err == nil {
		t.Fatalf("resolveComparisonBaseSelection returned selection %#v, want error", selection)
	}
	if err.Error() != "target eval run is required" {
		t.Fatalf("error = %v, want target eval run is required", err)
	}
}

func TestController_ResolveComparisonBaseSelectionRejectsMissingBaseSelection(t *testing.T) {
	controller := newTestController(t)

	selection, err := controller.resolveComparisonBaseSelection(context.Background(), &EvalRun{
		ID:          "target-run",
		EvalSpecID:  "eval-spec-1",
		OwnerUserID: "user-1",
	}, CompareEvalRunRequest{})
	if err == nil {
		t.Fatalf("resolveComparisonBaseSelection returned selection %#v, want error", selection)
	}
	if err.Error() != "no baseline or base eval run selected" {
		t.Fatalf("error = %v, want no baseline or base eval run selected", err)
	}
}

func TestController_LoadComparisonExecutionContextRejectsSelfComparison(t *testing.T) {
	controller := newTestController(t)
	run := &EvalRun{
		ID:         "eval-run-1",
		EvalSpecID: "eval-spec-1",
		GroupID:    "group-self",
		Status:     RunGroupStatusCompleted,
	}
	if err := controller.store.CreateEvalRun(context.Background(), run); err != nil {
		t.Fatalf("CreateEvalRun failed: %v", err)
	}

	execCtx, err := controller.loadComparisonExecutionContext(context.Background(), run.ID, CompareEvalRunRequest{
		BaseEvalRunID: run.ID,
	})
	if err == nil {
		t.Fatalf("loadComparisonExecutionContext returned %#v, want error", execCtx)
	}
	if err.Error() != "comparison base must differ from target eval run" {
		t.Fatalf("error = %v, want comparison base must differ from target eval run", err)
	}
}

func TestController_LoadComparisonExecutionContextRejectsCrossSpecComparison(t *testing.T) {
	controller := newTestController(t)
	targetRun := &EvalRun{
		ID:         "eval-run-target",
		EvalSpecID: "eval-spec-target",
		GroupID:    "group-target",
		Status:     RunGroupStatusCompleted,
	}
	baseRun := &EvalRun{
		ID:         "eval-run-base",
		EvalSpecID: "eval-spec-base",
		GroupID:    "group-base",
		Status:     RunGroupStatusCompleted,
	}
	if err := controller.store.CreateEvalRun(context.Background(), targetRun); err != nil {
		t.Fatalf("CreateEvalRun(target) failed: %v", err)
	}
	if err := controller.store.CreateEvalRun(context.Background(), baseRun); err != nil {
		t.Fatalf("CreateEvalRun(base) failed: %v", err)
	}

	execCtx, err := controller.loadComparisonExecutionContext(context.Background(), targetRun.ID, CompareEvalRunRequest{
		BaseEvalRunID: baseRun.ID,
	})
	if err == nil {
		t.Fatalf("loadComparisonExecutionContext returned %#v, want error", execCtx)
	}
	if err.Error() != "eval runs must belong to the same eval spec" {
		t.Fatalf("error = %v, want eval runs must belong to the same eval spec", err)
	}
}

func TestController_LoadComparisonReportContextPairReturnsSyncedSnapshots(t *testing.T) {
	controller := newTestController(t)

	dataset, err := controller.CreateDataset(context.Background(), DatasetSpec{
		Name:           "Comparison Dataset",
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
			"items": []interface{}{},
		},
		CreatedBy: "user-1",
	})
	if err != nil {
		t.Fatalf("CreateDatasetVersion failed: %v", err)
	}

	evalSpec, err := controller.CreateEvalSpec(context.Background(), EvalSpecSpec{
		Name:             "Comparison Eval",
		OwnerUserID:      "user-1",
		DatasetID:        dataset.ID,
		DatasetVersionID: version.ID,
		RunKind:          RunKindAgentTask,
		Profile:          "agent_task",
	})
	if err != nil {
		t.Fatalf("CreateEvalSpec failed: %v", err)
	}

	now := time.Now().UTC()
	targetGroup := &RunGroup{
		ID:          "comparison-target-group",
		Kind:        RunGroupKindEval,
		Title:       "comparison-target-group",
		Status:      RunGroupStatusCompleted,
		OwnerUserID: "user-1",
		Subject:     "agent_task",
		Summary: map[string]interface{}{
			"item_count": 0,
		},
		CreatedAt:  now,
		UpdatedAt:  now,
		StartedAt:  &now,
		FinishedAt: &now,
	}
	baseGroup := &RunGroup{
		ID:          "comparison-base-group",
		Kind:        RunGroupKindEval,
		Title:       "comparison-base-group",
		Status:      RunGroupStatusCompleted,
		OwnerUserID: "user-1",
		Subject:     "agent_task",
		Summary: map[string]interface{}{
			"item_count": 0,
		},
		CreatedAt:  now,
		UpdatedAt:  now,
		StartedAt:  &now,
		FinishedAt: &now,
	}
	if err := controller.store.CreateGroup(context.Background(), targetGroup); err != nil {
		t.Fatalf("CreateGroup(target) failed: %v", err)
	}
	if err := controller.store.CreateGroup(context.Background(), baseGroup); err != nil {
		t.Fatalf("CreateGroup(base) failed: %v", err)
	}

	targetRun := &EvalRun{
		ID:               "comparison-target-run",
		EvalSpecID:       evalSpec.ID,
		GroupID:          targetGroup.ID,
		DatasetVersionID: version.ID,
		Title:            "comparison-target-run",
		OwnerUserID:      "user-1",
		Status:           RunGroupStatusQueued,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	baseRun := &EvalRun{
		ID:               "comparison-base-run",
		EvalSpecID:       evalSpec.ID,
		GroupID:          baseGroup.ID,
		DatasetVersionID: version.ID,
		Title:            "comparison-base-run",
		OwnerUserID:      "user-1",
		Status:           RunGroupStatusQueued,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := controller.store.CreateEvalRun(context.Background(), targetRun); err != nil {
		t.Fatalf("CreateEvalRun(target) failed: %v", err)
	}
	if err := controller.store.CreateEvalRun(context.Background(), baseRun); err != nil {
		t.Fatalf("CreateEvalRun(base) failed: %v", err)
	}

	baseline := &Baseline{
		ID:          "comparison-baseline",
		Name:        "comparison-default-baseline",
		OwnerUserID: "user-1",
		EvalSpecID:  evalSpec.ID,
		EvalRunID:   baseRun.ID,
		IsDefault:   true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := controller.store.CreateBaseline(context.Background(), baseline); err != nil {
		t.Fatalf("CreateBaseline failed: %v", err)
	}

	pair, err := controller.loadComparisonReportContextPair(context.Background(), targetRun.ID, CompareEvalRunRequest{
		BaselineID: baseline.ID,
	})
	if err != nil {
		t.Fatalf("loadComparisonReportContextPair failed: %v", err)
	}
	if pair == nil {
		t.Fatal("expected report context pair")
	}
	if pair.baseline == nil || pair.baseline.ID != baseline.ID {
		t.Fatalf("pair.baseline = %#v, want %q", pair.baseline, baseline.ID)
	}
	if pair.target == nil || pair.target.evalRun == nil || pair.target.evalRun.ID != targetRun.ID {
		t.Fatalf("pair.target = %#v, want eval run %q", pair.target, targetRun.ID)
	}
	if pair.base == nil || pair.base.evalRun == nil || pair.base.evalRun.ID != baseRun.ID {
		t.Fatalf("pair.base = %#v, want eval run %q", pair.base, baseRun.ID)
	}
	if pair.target.evalRun.Status != RunGroupStatusCompleted {
		t.Fatalf("target status = %q, want %q", pair.target.evalRun.Status, RunGroupStatusCompleted)
	}
	if pair.base.evalRun.Status != RunGroupStatusCompleted {
		t.Fatalf("base status = %q, want %q", pair.base.evalRun.Status, RunGroupStatusCompleted)
	}
	if pair.target.group == nil || pair.target.group.ID != targetGroup.ID {
		t.Fatalf("target group = %#v, want %q", pair.target.group, targetGroup.ID)
	}
	if pair.base.group == nil || pair.base.group.ID != baseGroup.ID {
		t.Fatalf("base group = %#v, want %q", pair.base.group, baseGroup.ID)
	}
	if pair.target.datasetVersion == nil || pair.target.datasetVersion.ID != version.ID {
		t.Fatalf("target dataset version = %#v, want %q", pair.target.datasetVersion, version.ID)
	}
	if pair.base.datasetVersion == nil || pair.base.datasetVersion.ID != version.ID {
		t.Fatalf("base dataset version = %#v, want %q", pair.base.datasetVersion, version.ID)
	}
}

func TestBuildStoredComparisonReportDecoratesRuntimeMetadata(t *testing.T) {
	comparison := buildStoredComparisonReport(&comparisonExecutionContext{
		targetEvalRun: &EvalRun{
			ID:         "target-run",
			EvalSpecID: "eval-spec-1",
			Title:      "candidate",
		},
		baseEvalRun: &EvalRun{
			ID:          "base-run",
			EvalSpecID:  "eval-spec-1",
			OwnerUserID: "base-owner",
			Title:       "baseline",
		},
		baseline: &Baseline{
			ID:   "baseline-1",
			Name: "release-baseline",
		},
		targetReport: &EvalRunReport{
			EvalRun: &EvalRun{
				ID:    "target-run",
				Title: "candidate",
			},
		},
		baseReport: &EvalRunReport{
			EvalRun: &EvalRun{
				ID:    "base-run",
				Title: "baseline",
			},
		},
	})
	if comparison == nil {
		t.Fatal("expected comparison report")
	}
	if comparison.ID == "" {
		t.Fatal("expected comparison id")
	}
	if comparison.CreatedAt.IsZero() {
		t.Fatal("expected created_at to be populated")
	}
	if comparison.OwnerUserID != "base-owner" {
		t.Fatalf("owner_user_id = %q, want base-owner", comparison.OwnerUserID)
	}
	if comparison.EvalSpecID != "eval-spec-1" {
		t.Fatalf("eval_spec_id = %q, want eval-spec-1", comparison.EvalSpecID)
	}
	if comparison.BaseEvalRunID != "base-run" || comparison.TargetEvalRunID != "target-run" {
		t.Fatalf("comparison run binding = (%q, %q), want (base-run, target-run)", comparison.BaseEvalRunID, comparison.TargetEvalRunID)
	}
	if comparison.BaselineID != "baseline-1" {
		t.Fatalf("baseline_id = %q, want baseline-1", comparison.BaselineID)
	}
	if got := comparison.Summary["baseline_name"]; got != "release-baseline" {
		t.Fatalf("summary.baseline_name = %#v, want release-baseline", got)
	}
}

func TestController_CreateComparisonReportPersistsStoredRecord(t *testing.T) {
	controller := newTestController(t)

	execCtx := &comparisonExecutionContext{
		targetEvalRun: &EvalRun{
			ID:          "target-run",
			EvalSpecID:  "eval-spec-1",
			OwnerUserID: "target-owner",
			Title:       "candidate",
		},
		baseEvalRun: &EvalRun{
			ID:          "base-run",
			EvalSpecID:  "eval-spec-1",
			OwnerUserID: "base-owner",
			Title:       "baseline",
		},
		baseline: &Baseline{
			ID:   "baseline-1",
			Name: "release-baseline",
		},
		targetReport: &EvalRunReport{
			EvalRun: &EvalRun{
				ID:    "target-run",
				Title: "candidate",
			},
			GroupReport: &RunGroupReport{
				Group: &RunGroup{
					ID:      "target-group",
					Summary: map[string]interface{}{"changed_case_count": 1},
				},
			},
		},
		baseReport: &EvalRunReport{
			EvalRun: &EvalRun{
				ID:    "base-run",
				Title: "baseline",
			},
			GroupReport: &RunGroupReport{
				Group: &RunGroup{
					ID:      "base-group",
					Summary: map[string]interface{}{"changed_case_count": 0},
				},
			},
		},
	}

	comparison, err := controller.createComparisonReport(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("createComparisonReport failed: %v", err)
	}
	if comparison == nil {
		t.Fatal("expected comparison report")
	}
	if comparison.ID == "" {
		t.Fatal("expected persisted comparison id")
	}
	if comparison.OwnerUserID != "target-owner" {
		t.Fatalf("owner_user_id = %q, want target-owner", comparison.OwnerUserID)
	}
	if comparison.BaseEvalRunID != "base-run" || comparison.TargetEvalRunID != "target-run" {
		t.Fatalf("comparison run binding = (%q, %q), want (base-run, target-run)", comparison.BaseEvalRunID, comparison.TargetEvalRunID)
	}
	if comparison.BaselineID != "baseline-1" {
		t.Fatalf("baseline_id = %q, want baseline-1", comparison.BaselineID)
	}

	stored, err := controller.store.GetComparisonReport(context.Background(), comparison.ID)
	if err != nil {
		t.Fatalf("GetComparisonReport failed: %v", err)
	}
	if stored.ID != comparison.ID {
		t.Fatalf("stored id = %q, want %q", stored.ID, comparison.ID)
	}
	if stored.OwnerUserID != comparison.OwnerUserID {
		t.Fatalf("stored owner_user_id = %q, want %q", stored.OwnerUserID, comparison.OwnerUserID)
	}
	if stored.BaseEvalRunID != comparison.BaseEvalRunID || stored.TargetEvalRunID != comparison.TargetEvalRunID {
		t.Fatalf("stored run binding = (%q, %q), want (%q, %q)", stored.BaseEvalRunID, stored.TargetEvalRunID, comparison.BaseEvalRunID, comparison.TargetEvalRunID)
	}
	if stored.BaselineID != comparison.BaselineID {
		t.Fatalf("stored baseline_id = %q, want %q", stored.BaselineID, comparison.BaselineID)
	}
	if got := stored.Summary["baseline_name"]; got != "release-baseline" {
		t.Fatalf("stored summary.baseline_name = %#v, want release-baseline", got)
	}
	if !stored.CreatedAt.Equal(comparison.CreatedAt) {
		t.Fatalf("stored created_at = %s, want %s", stored.CreatedAt, comparison.CreatedAt)
	}
}

func TestBuildComparisonReportKeepsSummaryAndScorerDeltaAligned(t *testing.T) {
	now := time.Now().UTC()
	baseReport := &EvalRunReport{
		EvalRun: &EvalRun{
			ID:    "base-run",
			Title: "baseline",
		},
		GroupReport: &RunGroupReport{
			Group: &RunGroup{
				ID: "base-group",
			},
			Items: []RunGroupItem{
				{
					ID:          "item-1",
					Index:       0,
					Status:      RunGroupItemStatusPassed,
					LatestRunID: "run-base",
					Profile:     "agent_task",
					Input:       map[string]interface{}{"goal": "ship the baseline"},
					Metadata:    map[string]interface{}{"dataset_case_id": "case-1"},
				},
			},
			VerdictCounts: map[string]int{string(ScoreVerdictPass): 1},
			OverallScore:  1,
			PassRate:      1,
			Scorecards: []Scorecard{
				{
					ID:            "score-base",
					GroupItemID:   "item-1",
					RunID:         "run-base",
					Verdict:       ScoreVerdictPass,
					Score:         1,
					BreakdownJSON: `{"verification_passed":true,"evidence_score":0.9}`,
					CreatedAt:     now,
				},
			},
			LinkedRuns: []Run{{ID: "run-base"}},
		},
	}
	targetReport := &EvalRunReport{
		EvalRun: &EvalRun{
			ID:    "target-run",
			Title: "candidate",
		},
		GroupReport: &RunGroupReport{
			Group: &RunGroup{
				ID: "target-group",
			},
			Items: []RunGroupItem{
				{
					ID:          "item-1",
					Index:       0,
					Status:      RunGroupItemStatusFailed,
					LatestRunID: "run-target",
					Profile:     "agent_task",
					Input:       map[string]interface{}{"goal": "ship the candidate"},
					Metadata:    map[string]interface{}{"dataset_case_id": "case-1"},
				},
			},
			VerdictCounts: map[string]int{string(ScoreVerdictFail): 1},
			OverallScore:  0,
			PassRate:      0,
			Scorecards: []Scorecard{
				{
					ID:            "score-target",
					GroupItemID:   "item-1",
					RunID:         "run-target",
					Verdict:       ScoreVerdictFail,
					Score:         0,
					BreakdownJSON: `{"verification_passed":false,"failure_label":"missing_artifact"}`,
					CreatedAt:     now.Add(time.Second),
				},
			},
			LinkedRuns: []Run{{ID: "run-target"}},
			Artifacts:  []ArtifactRef{{ID: "artifact-target", RunID: "run-target", Kind: "file"}},
		},
	}

	report := buildComparisonReport(baseReport, targetReport, &Baseline{
		ID:   "baseline-1",
		Name: "release-baseline",
	})
	if report == nil {
		t.Fatal("expected comparison report")
	}
	if got, want := report.Summary["overall_score_delta"], report.ScorerDelta["overall_score_delta"]; got != want {
		t.Fatalf("summary overall_score_delta = %#v, want %#v", got, want)
	}
	if got, want := report.Summary["pass_rate_delta"], report.ScorerDelta["pass_rate_delta"]; got != want {
		t.Fatalf("summary pass_rate_delta = %#v, want %#v", got, want)
	}
	if got, want := report.Summary["verification_pass_rate_delta"], report.ScorerDelta["verification_pass_rate_delta"]; got != want {
		t.Fatalf("summary verification_pass_rate_delta = %#v, want %#v", got, want)
	}
	if got, want := report.Summary["evidence_backed_pass_rate_delta"], report.ScorerDelta["evidence_backed_pass_rate_delta"]; got != want {
		t.Fatalf("summary evidence_backed_pass_rate_delta = %#v, want %#v", got, want)
	}
	if got, want := report.Summary["retry_recovered_delta"], report.ScorerDelta["retry_recovered_delta"]; got != want {
		t.Fatalf("summary retry_recovered_delta = %#v, want %#v", got, want)
	}
	if got, want := report.Summary["regression_count"], len(report.Regressions); got != want {
		t.Fatalf("summary regression_count = %#v, want %d", got, want)
	}
	if got, want := report.Summary["improvement_count"], len(report.Improvements); got != want {
		t.Fatalf("summary improvement_count = %#v, want %d", got, want)
	}
	if got, want := report.Summary["new_failure_count"], 1; got != want {
		t.Fatalf("summary new_failure_count = %#v, want %d", got, want)
	}
	if got, want := report.Summary["failure_label_delta"], report.ScorerDelta["failure_label_delta"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("summary failure_label_delta = %#v, want %#v", got, want)
	}
	if got, want := report.ScorerDelta["linked_run_count_delta"], 0; got != want {
		t.Fatalf("scorer_delta linked_run_count_delta = %#v, want %d", got, want)
	}
	if got, want := report.ScorerDelta["artifact_count_delta"], 1; got != want {
		t.Fatalf("scorer_delta artifact_count_delta = %#v, want %d", got, want)
	}
	if verdictDelta, ok := report.ScorerDelta["verdict_count_delta"].(map[string]interface{}); !ok {
		t.Fatalf("scorer_delta verdict_count_delta = %#v, want map", report.ScorerDelta["verdict_count_delta"])
	} else {
		if got := verdictDelta[string(ScoreVerdictPass)]; got != -1 {
			t.Fatalf("verdict_count_delta[pass] = %#v, want -1", got)
		}
		if got := verdictDelta[string(ScoreVerdictFail)]; got != 1 {
			t.Fatalf("verdict_count_delta[fail] = %#v, want 1", got)
		}
	}
}

func TestComparisonReportViewCaseSnapshotsPreserveFallbacks(t *testing.T) {
	longResult := strings.Repeat("runtime output ", 14)
	now := time.Now().UTC()

	view := newComparisonReportView(&EvalRunReport{
		GroupReport: &RunGroupReport{
			Items: []RunGroupItem{
				{
					ID:          "item-fallback",
					Index:       2,
					Status:      RunGroupItemStatusPassed,
					LatestRunID: "run-error",
					Profile:     "agent_task",
				},
				{
					ID:          "item-card",
					Index:       4,
					Status:      RunGroupItemStatusFailed,
					LatestRunID: "run-latest",
					Profile:     "agent_task",
					Input:       map[string]interface{}{"goal": "collect runtime evidence"},
					Metadata:    map[string]interface{}{"dataset_case_id": "case-card"},
				},
				{
					ID:          "item-trace",
					Index:       6,
					Status:      RunGroupItemStatusFailed,
					LatestRunID: "run-trace",
					Profile:     "agent_task",
					Input:       map[string]interface{}{"goal": "judge fallback"},
					Metadata:    map[string]interface{}{"dataset_case_id": "case-trace"},
				},
				{
					ID:          "item-result",
					Index:       7,
					Status:      RunGroupItemStatusFailed,
					LatestRunID: "run-result",
					Profile:     "agent_task",
					Input:       map[string]interface{}{"goal": "result fallback"},
					Metadata:    map[string]interface{}{"dataset_case_id": "case-result"},
				},
			},
			Scorecards: []Scorecard{
				{
					ID:             "score-card",
					GroupItemID:    "item-card",
					RunID:          "run-card",
					Verdict:        ScoreVerdictFail,
					Score:          0.25,
					BreakdownJSON:  `{"reason":"missing artifact","failure_label":"missing_artifact","verification_passed":false,"evidence_score":0.4}`,
					JudgeTraceJSON: `{"reason":"judge trace reason"}`,
					CreatedAt:      now,
				},
				{
					ID:             "score-trace",
					GroupItemID:    "item-trace",
					Verdict:        ScoreVerdictPartial,
					Score:          0.5,
					BreakdownJSON:  `{"verification_passed":true}`,
					JudgeTraceJSON: `{"reason":"judge trace reason"}`,
					CreatedAt:      now.Add(time.Second),
				},
				{
					ID:            "score-result",
					GroupItemID:   "item-result",
					Score:         0.1,
					BreakdownJSON: `{"evidence_score":0.2}`,
					CreatedAt:     now.Add(2 * time.Second),
				},
			},
			LinkedRuns: []Run{
				{ID: "run-error", Error: "approval blocked"},
				{ID: "run-card", Error: "runner error"},
				{ID: "run-trace", Error: "trace runner error"},
				{ID: "run-result", Result: longResult},
			},
		},
	})

	snapshots := view.caseSnapshots()

	fallbackCase, ok := snapshots["item-2"]
	if !ok {
		t.Fatalf("snapshots = %#v, want fallback key item-2", snapshots)
	}
	if fallbackCase.Label != "item-2" {
		t.Fatalf("fallback label = %q, want item-2", fallbackCase.Label)
	}
	if fallbackCase.Verdict != "pass" {
		t.Fatalf("fallback verdict = %q, want pass", fallbackCase.Verdict)
	}
	if fallbackCase.RunID != "run-error" {
		t.Fatalf("fallback run_id = %q, want run-error", fallbackCase.RunID)
	}
	if fallbackCase.Reason != "approval blocked" {
		t.Fatalf("fallback reason = %q, want approval blocked", fallbackCase.Reason)
	}

	cardCase, ok := snapshots["case-card"]
	if !ok {
		t.Fatalf("snapshots = %#v, want case-card", snapshots)
	}
	if cardCase.Label != "collect runtime evidence" {
		t.Fatalf("case-card label = %q, want collect runtime evidence", cardCase.Label)
	}
	if cardCase.RunID != "run-card" {
		t.Fatalf("case-card run_id = %q, want run-card", cardCase.RunID)
	}
	if cardCase.Verdict != "fail" {
		t.Fatalf("case-card verdict = %q, want fail", cardCase.Verdict)
	}
	if cardCase.Reason != "missing artifact" {
		t.Fatalf("case-card reason = %q, want missing artifact", cardCase.Reason)
	}
	if cardCase.FailureLabel != "missing_artifact" {
		t.Fatalf("case-card failure_label = %q, want missing_artifact", cardCase.FailureLabel)
	}
	if cardCase.Verification != "failed" {
		t.Fatalf("case-card verification = %q, want failed", cardCase.Verification)
	}
	if cardCase.EvidenceScore != 0.4 {
		t.Fatalf("case-card evidence_score = %v, want 0.4", cardCase.EvidenceScore)
	}

	traceCase, ok := snapshots["case-trace"]
	if !ok {
		t.Fatalf("snapshots = %#v, want case-trace", snapshots)
	}
	if traceCase.RunID != "run-trace" {
		t.Fatalf("case-trace run_id = %q, want run-trace", traceCase.RunID)
	}
	if traceCase.Verdict != "partial" {
		t.Fatalf("case-trace verdict = %q, want partial", traceCase.Verdict)
	}
	if traceCase.Reason != "judge trace reason" {
		t.Fatalf("case-trace reason = %q, want judge trace reason", traceCase.Reason)
	}
	if traceCase.Verification != "passed" {
		t.Fatalf("case-trace verification = %q, want passed", traceCase.Verification)
	}

	resultCase, ok := snapshots["case-result"]
	if !ok {
		t.Fatalf("snapshots = %#v, want case-result", snapshots)
	}
	if resultCase.Verdict != "fail" {
		t.Fatalf("case-result verdict = %q, want fail", resultCase.Verdict)
	}
	if resultCase.Reason != comparisonReason(nil, Run{Result: longResult}) {
		t.Fatalf("case-result reason = %q, want truncated run result", resultCase.Reason)
	}
}

func TestClassifyComparisonCasePreservesDecisionBoundaries(t *testing.T) {
	tests := []struct {
		name                 string
		base                 evalCaseSnapshot
		hasBase              bool
		target               evalCaseSnapshot
		hasTarget            bool
		wantClassification   string
		wantChanged          bool
		wantBaseVerdict      string
		wantTargetVerdict    string
		wantBaseStatus       string
		wantTargetStatus     string
		wantBaseVerification string
		wantTargetVerify     string
	}{
		{
			name: "unchanged normalized pass case",
			base: evalCaseSnapshot{
				Key:           "case-1",
				Status:        "completed",
				Verification:  "passed",
				FailureLabel:  "",
				Score:         1,
				EvidenceScore: 0.8,
			},
			hasBase: true,
			target: evalCaseSnapshot{
				Key:           "case-1",
				Verdict:       "pass",
				Status:        "completed",
				Verification:  "passed",
				FailureLabel:  "",
				Score:         1,
				EvidenceScore: 0.8,
			},
			hasTarget:            true,
			wantClassification:   "",
			wantChanged:          false,
			wantBaseVerdict:      "pass",
			wantTargetVerdict:    "pass",
			wantBaseStatus:       "completed",
			wantTargetStatus:     "completed",
			wantBaseVerification: "passed",
			wantTargetVerify:     "passed",
		},
		{
			name: "same rank but evidence delta is unstable",
			base: evalCaseSnapshot{
				Key:           "case-2",
				Verdict:       "pass",
				Status:        "completed",
				Verification:  "passed",
				FailureLabel:  "",
				Score:         1,
				EvidenceScore: 0.2,
			},
			hasBase: true,
			target: evalCaseSnapshot{
				Key:           "case-2",
				Verdict:       "pass",
				Status:        "completed",
				Verification:  "passed",
				FailureLabel:  "",
				Score:         1,
				EvidenceScore: 0.9,
			},
			hasTarget:            true,
			wantClassification:   "unstable",
			wantChanged:          true,
			wantBaseVerdict:      "pass",
			wantTargetVerdict:    "pass",
			wantBaseStatus:       "completed",
			wantTargetStatus:     "completed",
			wantBaseVerification: "passed",
			wantTargetVerify:     "passed",
		},
		{
			name: "fail to pass is improvement",
			base: evalCaseSnapshot{
				Key:          "case-3",
				Status:       "failed",
				Verification: "failed",
			},
			hasBase: true,
			target: evalCaseSnapshot{
				Key:          "case-3",
				Status:       "passed",
				Verification: "passed",
			},
			hasTarget:            true,
			wantClassification:   "improvement",
			wantChanged:          true,
			wantBaseVerdict:      "fail",
			wantTargetVerdict:    "pass",
			wantBaseStatus:       "failed",
			wantTargetStatus:     "passed",
			wantBaseVerification: "failed",
			wantTargetVerify:     "passed",
		},
		{
			name: "missing base with failing target is regression",
			target: evalCaseSnapshot{
				Key:    "case-4",
				Status: "error",
			},
			hasTarget:            true,
			wantClassification:   "regression",
			wantChanged:          true,
			wantBaseVerdict:      "missing",
			wantTargetVerdict:    "error",
			wantBaseStatus:       "missing",
			wantTargetStatus:     "error",
			wantBaseVerification: "missing",
			wantTargetVerify:     "unknown",
		},
		{
			name: "missing target for passing base is unstable",
			base: evalCaseSnapshot{
				Key:    "case-5",
				Status: "passed",
			},
			hasBase:              true,
			wantClassification:   "unstable",
			wantChanged:          true,
			wantBaseVerdict:      "pass",
			wantTargetVerdict:    "missing",
			wantBaseStatus:       "passed",
			wantTargetStatus:     "missing",
			wantBaseVerification: "unknown",
			wantTargetVerify:     "missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delta, classification, changed := classifyComparisonCase(tt.base, tt.hasBase, tt.target, tt.hasTarget)
			if classification != tt.wantClassification {
				t.Fatalf("classification = %q, want %q", classification, tt.wantClassification)
			}
			if changed != tt.wantChanged {
				t.Fatalf("changed = %t, want %t", changed, tt.wantChanged)
			}
			if delta.BaseVerdict != tt.wantBaseVerdict {
				t.Fatalf("base verdict = %q, want %q", delta.BaseVerdict, tt.wantBaseVerdict)
			}
			if delta.TargetVerdict != tt.wantTargetVerdict {
				t.Fatalf("target verdict = %q, want %q", delta.TargetVerdict, tt.wantTargetVerdict)
			}
			if delta.BaseStatus != tt.wantBaseStatus {
				t.Fatalf("base status = %q, want %q", delta.BaseStatus, tt.wantBaseStatus)
			}
			if delta.TargetStatus != tt.wantTargetStatus {
				t.Fatalf("target status = %q, want %q", delta.TargetStatus, tt.wantTargetStatus)
			}
			if delta.BaseVerification != tt.wantBaseVerification {
				t.Fatalf("base verification = %q, want %q", delta.BaseVerification, tt.wantBaseVerification)
			}
			if delta.TargetVerification != tt.wantTargetVerify {
				t.Fatalf("target verification = %q, want %q", delta.TargetVerification, tt.wantTargetVerify)
			}
		})
	}
}

func TestBuildComparisonCaseRollupTracksClassificationCounts(t *testing.T) {
	baseView := newComparisonReportView(&EvalRunReport{
		GroupReport: &RunGroupReport{
			Items: []RunGroupItem{
				{
					ID:       "base-only",
					Index:    0,
					Status:   RunGroupItemStatusPassed,
					Metadata: map[string]interface{}{"dataset_case_id": "case-base-only"},
				},
				{
					ID:       "improvement",
					Index:    1,
					Status:   RunGroupItemStatusFailed,
					Metadata: map[string]interface{}{"dataset_case_id": "case-improvement"},
				},
				{
					ID:       "regression",
					Index:    2,
					Status:   RunGroupItemStatusPassed,
					Metadata: map[string]interface{}{"dataset_case_id": "case-regression"},
				},
				{
					ID:       "unstable",
					Index:    3,
					Status:   RunGroupItemStatusPassed,
					Metadata: map[string]interface{}{"dataset_case_id": "case-unstable"},
				},
			},
			Scorecards: []Scorecard{
				{
					ID:            "score-unstable-base",
					GroupItemID:   "unstable",
					Verdict:       ScoreVerdictPass,
					Score:         1,
					BreakdownJSON: `{"verification_passed":true,"evidence_score":0.2}`,
				},
			},
		},
	})

	targetView := newComparisonReportView(&EvalRunReport{
		GroupReport: &RunGroupReport{
			Items: []RunGroupItem{
				{
					ID:       "improvement",
					Index:    1,
					Status:   RunGroupItemStatusPassed,
					Metadata: map[string]interface{}{"dataset_case_id": "case-improvement"},
				},
				{
					ID:       "regression",
					Index:    2,
					Status:   RunGroupItemStatusFailed,
					Metadata: map[string]interface{}{"dataset_case_id": "case-regression"},
				},
				{
					ID:       "target-only",
					Index:    4,
					Status:   RunGroupItemStatusError,
					Metadata: map[string]interface{}{"dataset_case_id": "case-target-only"},
				},
				{
					ID:       "unstable",
					Index:    3,
					Status:   RunGroupItemStatusPassed,
					Metadata: map[string]interface{}{"dataset_case_id": "case-unstable"},
				},
			},
			Scorecards: []Scorecard{
				{
					ID:            "score-unstable-target",
					GroupItemID:   "unstable",
					Verdict:       ScoreVerdictPass,
					Score:         1,
					BreakdownJSON: `{"verification_passed":true,"evidence_score":0.9}`,
				},
			},
		},
	})

	rollup := buildComparisonCaseRollup(baseView, targetView)

	if rollup.changedCases != 5 {
		t.Fatalf("changed_cases = %d, want 5", rollup.changedCases)
	}
	if len(rollup.regressions) != 2 {
		t.Fatalf("regressions len = %d, want 2", len(rollup.regressions))
	}
	if len(rollup.improvements) != 1 {
		t.Fatalf("improvements len = %d, want 1", len(rollup.improvements))
	}
	if rollup.unstableCases != 2 {
		t.Fatalf("unstable_cases = %d, want 2", rollup.unstableCases)
	}
	if rollup.newFailures != 2 {
		t.Fatalf("new_failures = %d, want 2", rollup.newFailures)
	}
	if rollup.resolvedFailures != 1 {
		t.Fatalf("resolved_failures = %d, want 1", rollup.resolvedFailures)
	}
	if rollup.regressions[0].Key != "case-regression" || rollup.regressions[1].Key != "case-target-only" {
		t.Fatalf("regression keys = %#v, want [case-regression case-target-only]", []string{rollup.regressions[0].Key, rollup.regressions[1].Key})
	}
	if rollup.improvements[0].Key != "case-improvement" {
		t.Fatalf("improvement key = %q, want case-improvement", rollup.improvements[0].Key)
	}
}

func TestBuildComparisonSummaryOmitsEmptyFailureLabelDelta(t *testing.T) {
	summary := buildComparisonSummary(nil, &comparisonReportBundle{
		baseView:   newComparisonReportView(nil),
		targetView: newComparisonReportView(nil),
	})

	if _, ok := summary["failure_label_delta"]; ok {
		t.Fatalf("summary failure_label_delta = %#v, want omitted", summary["failure_label_delta"])
	}
	if got := summary["changed_case_count"]; got != 0 {
		t.Fatalf("summary changed_case_count = %#v, want 0", got)
	}
}

func TestBuildComparisonScorerDeltaKeepsEmptyFailureLabelDelta(t *testing.T) {
	scorerDelta := buildComparisonScorerDelta(&comparisonReportBundle{
		baseView:   newComparisonReportView(nil),
		targetView: newComparisonReportView(nil),
	})

	failureDelta, ok := scorerDelta["failure_label_delta"].(map[string]interface{})
	if !ok {
		t.Fatalf("scorer failure_label_delta = %#v, want map", scorerDelta["failure_label_delta"])
	}
	if len(failureDelta) != 0 {
		t.Fatalf("scorer failure_label_delta len = %d, want 0", len(failureDelta))
	}
	if got := scorerDelta["overall_score_delta"]; got != float64(0) {
		t.Fatalf("scorer overall_score_delta = %#v, want 0", got)
	}
}

func TestBuildComparisonSummaryIdentityUsesViewFallbacks(t *testing.T) {
	identity := buildComparisonSummaryIdentity(nil, &comparisonReportBundle{
		baseView: newComparisonReportView(&EvalRunReport{
			EvalRun: &EvalRun{
				ID:    "base-run",
				Title: "baseline title",
			},
			GroupReport: &RunGroupReport{
				Group: &RunGroup{ID: "base-group"},
			},
		}),
		targetView: newComparisonReportView(&EvalRunReport{
			EvalRun: &EvalRun{
				ID:    "target-run",
				Title: "candidate title",
			},
			GroupReport: &RunGroupReport{
				Group: &RunGroup{ID: "target-group"},
			},
		}),
	})

	if identity.comparisonKind != "eval_run" {
		t.Fatalf("comparison_kind = %q, want eval_run", identity.comparisonKind)
	}
	if identity.baselineName != "baseline title" {
		t.Fatalf("baseline_name = %q, want baseline title", identity.baselineName)
	}
	if identity.baseTitle != "baseline title" {
		t.Fatalf("base_title = %q, want baseline title", identity.baseTitle)
	}
	if identity.targetTitle != "candidate title" {
		t.Fatalf("target_title = %q, want candidate title", identity.targetTitle)
	}
	if identity.baseGroupID != "base-group" {
		t.Fatalf("base_group_id = %q, want base-group", identity.baseGroupID)
	}
	if identity.targetGroupID != "target-group" {
		t.Fatalf("target_group_id = %q, want target-group", identity.targetGroupID)
	}
}

func TestBuildComparisonSummaryRollupPreservesCounts(t *testing.T) {
	rollup := buildComparisonSummaryRollup(comparisonCaseRollup{
		regressions:      []ComparisonCaseDelta{{Key: "regression-1"}, {Key: "regression-2"}},
		improvements:     []ComparisonCaseDelta{{Key: "improvement-1"}},
		changedCases:     5,
		unstableCases:    2,
		newFailures:      2,
		resolvedFailures: 1,
	})

	if rollup.changedCaseCount != 5 {
		t.Fatalf("changed_case_count = %d, want 5", rollup.changedCaseCount)
	}
	if rollup.regressionCount != 2 {
		t.Fatalf("regression_count = %d, want 2", rollup.regressionCount)
	}
	if rollup.improvementCount != 1 {
		t.Fatalf("improvement_count = %d, want 1", rollup.improvementCount)
	}
	if rollup.unstableCaseCount != 2 {
		t.Fatalf("unstable_case_count = %d, want 2", rollup.unstableCaseCount)
	}
	if rollup.newFailureCount != 2 {
		t.Fatalf("new_failure_count = %d, want 2", rollup.newFailureCount)
	}
	if rollup.resolvedFailureCount != 1 {
		t.Fatalf("resolved_failure_count = %d, want 1", rollup.resolvedFailureCount)
	}
}

func TestBuildComparisonScorerDeltaContextPreservesMetricAndStructuralFields(t *testing.T) {
	context := buildComparisonScorerDeltaContext(comparisonMetricBundle{
		base: comparisonViewMetricSnapshot{
			overallScore:           0.9,
			passRate:               1,
			verificationPassRate:   1,
			evidenceBackedPassRate: 0.75,
			retryRecoveredCount:    3,
		},
		target: comparisonViewMetricSnapshot{
			overallScore:           0.4,
			passRate:               0.5,
			verificationPassRate:   0,
			evidenceBackedPassRate: 0.25,
			retryRecoveredCount:    1,
		},
		structural: comparisonStructuralDelta{
			failureLabelDelta:   map[string]interface{}{"missing_artifact": 1},
			verdictCountDelta:   map[string]interface{}{"pass": -1, "fail": 1},
			linkedRunCountDelta: 2,
			artifactCountDelta:  -1,
		},
	})

	if context.overallScoreDelta != -0.5 {
		t.Fatalf("overall_score_delta = %v, want -0.5", context.overallScoreDelta)
	}
	if context.passRateDelta != -0.5 {
		t.Fatalf("pass_rate_delta = %v, want -0.5", context.passRateDelta)
	}
	if context.verificationRateDelta != -1 {
		t.Fatalf("verification_rate_delta = %v, want -1", context.verificationRateDelta)
	}
	if context.evidenceBackedRateDelta != -0.5 {
		t.Fatalf("evidence_backed_rate_delta = %v, want -0.5", context.evidenceBackedRateDelta)
	}
	if context.retryRecoveredDelta != -2 {
		t.Fatalf("retry_recovered_delta = %d, want -2", context.retryRecoveredDelta)
	}
	if got := context.failureLabelDelta["missing_artifact"]; got != 1 {
		t.Fatalf("failure_label_delta[missing_artifact] = %#v, want 1", got)
	}
	if got := context.verdictCountDelta["pass"]; got != -1 {
		t.Fatalf("verdict_count_delta[pass] = %#v, want -1", got)
	}
	if context.linkedRunCountDelta != 2 {
		t.Fatalf("linked_run_count_delta = %d, want 2", context.linkedRunCountDelta)
	}
	if context.artifactCountDelta != -1 {
		t.Fatalf("artifact_count_delta = %d, want -1", context.artifactCountDelta)
	}
}

func TestBuildComparisonMetricBundleSeparatesViewMetricsAndStructuralDeltas(t *testing.T) {
	baseView := newComparisonReportView(&EvalRunReport{
		GroupReport: &RunGroupReport{
			Group: &RunGroup{
				Summary: map[string]interface{}{
					"verification_pass_rate":    1.0,
					"evidence_backed_pass_rate": 0.75,
					"retry_recovered_count":     2,
				},
			},
			OverallScore: 0.9,
			PassRate:     1,
			Items: []RunGroupItem{
				{ID: "item-1"},
			},
			VerdictCounts: map[string]int{
				"pass": 1,
			},
			Scorecards: []Scorecard{
				{
					GroupItemID:   "item-1",
					Verdict:       ScoreVerdictPass,
					BreakdownJSON: `{"failure_label":"baseline_only"}`,
				},
			},
			LinkedRuns: []Run{{ID: "run-base"}},
			Artifacts:  []ArtifactRef{{ID: "artifact-base"}},
		},
	})
	targetView := newComparisonReportView(&EvalRunReport{
		GroupReport: &RunGroupReport{
			Group: &RunGroup{
				Summary: map[string]interface{}{
					"verification_pass_rate":    0.5,
					"evidence_backed_pass_rate": 0.25,
					"retry_recovered_count":     1,
				},
			},
			OverallScore: 0.4,
			PassRate:     0.5,
			Items: []RunGroupItem{
				{ID: "item-1"},
			},
			VerdictCounts: map[string]int{
				"fail": 1,
			},
			Scorecards: []Scorecard{
				{
					GroupItemID:   "item-1",
					Verdict:       ScoreVerdictFail,
					BreakdownJSON: `{"failure_label":"missing_artifact"}`,
				},
			},
			LinkedRuns: []Run{{ID: "run-target"}, {ID: "run-target-2"}},
			Artifacts:  []ArtifactRef{},
		},
	})

	metrics := buildComparisonMetricBundle(baseView, targetView)

	if metrics.base.overallScore != 0.9 || metrics.target.overallScore != 0.4 {
		t.Fatalf("overall scores = (%v, %v), want (0.9, 0.4)", metrics.base.overallScore, metrics.target.overallScore)
	}
	if metrics.base.linkedRunCount != 1 || metrics.target.linkedRunCount != 2 {
		t.Fatalf("linked run counts = (%d, %d), want (1, 2)", metrics.base.linkedRunCount, metrics.target.linkedRunCount)
	}
	if metrics.base.artifactCount != 1 || metrics.target.artifactCount != 0 {
		t.Fatalf("artifact counts = (%d, %d), want (1, 0)", metrics.base.artifactCount, metrics.target.artifactCount)
	}
	if got := metrics.structural.failureLabelDelta["baseline_only"]; got != -1 {
		t.Fatalf("failure_label_delta[baseline_only] = %#v, want -1", got)
	}
	if got := metrics.structural.failureLabelDelta["missing_artifact"]; got != 1 {
		t.Fatalf("failure_label_delta[missing_artifact] = %#v, want 1", got)
	}
	if got := metrics.structural.verdictCountDelta["pass"]; got != -1 {
		t.Fatalf("verdict_count_delta[pass] = %#v, want -1", got)
	}
	if got := metrics.structural.verdictCountDelta["fail"]; got != 1 {
		t.Fatalf("verdict_count_delta[fail] = %#v, want 1", got)
	}
	if metrics.structural.linkedRunCountDelta != 1 {
		t.Fatalf("linked_run_count_delta = %d, want 1", metrics.structural.linkedRunCountDelta)
	}
	if metrics.structural.artifactCountDelta != -1 {
		t.Fatalf("artifact_count_delta = %d, want -1", metrics.structural.artifactCountDelta)
	}
}

func TestBuildComparisonReportSectionsKeepSummaryBodyAndScorerAligned(t *testing.T) {
	sections := buildComparisonReportSections(nil, &comparisonReportBundle{
		rollup: comparisonCaseRollup{
			regressions:      []ComparisonCaseDelta{{Key: "regression-1"}},
			improvements:     []ComparisonCaseDelta{{Key: "improvement-1"}, {Key: "improvement-2"}},
			changedCases:     4,
			unstableCases:    1,
			newFailures:      1,
			resolvedFailures: 2,
		},
		metrics: comparisonMetricBundle{
			base: comparisonViewMetricSnapshot{
				overallScore:           1,
				passRate:               1,
				verificationPassRate:   1,
				evidenceBackedPassRate: 1,
				retryRecoveredCount:    1,
			},
			target: comparisonViewMetricSnapshot{
				overallScore:           0.5,
				passRate:               0.5,
				verificationPassRate:   0.5,
				evidenceBackedPassRate: 0.25,
				retryRecoveredCount:    0,
			},
			structural: comparisonStructuralDelta{
				failureLabelDelta:   map[string]interface{}{"missing_artifact": 1},
				verdictCountDelta:   map[string]interface{}{"pass": -1, "fail": 1},
				linkedRunCountDelta: 1,
				artifactCountDelta:  -2,
			},
		},
	})

	if got, want := sections.summary["regression_count"], len(sections.body.regressions); got != want {
		t.Fatalf("summary regression_count = %#v, want %d", got, want)
	}
	if got, want := sections.summary["improvement_count"], len(sections.body.improvements); got != want {
		t.Fatalf("summary improvement_count = %#v, want %d", got, want)
	}
	if got, want := sections.summary["changed_case_count"], 4; got != want {
		t.Fatalf("summary changed_case_count = %#v, want %d", got, want)
	}
	if got, want := sections.summary["unstable_case_count"], 1; got != want {
		t.Fatalf("summary unstable_case_count = %#v, want %d", got, want)
	}
	if got, want := sections.summary["new_failure_count"], 1; got != want {
		t.Fatalf("summary new_failure_count = %#v, want %d", got, want)
	}
	if got, want := sections.summary["resolved_failure_count"], 2; got != want {
		t.Fatalf("summary resolved_failure_count = %#v, want %d", got, want)
	}
	if got, want := sections.summary["overall_score_delta"], sections.scorerDelta["overall_score_delta"]; got != want {
		t.Fatalf("summary overall_score_delta = %#v, want %#v", got, want)
	}
	if got, want := sections.summary["failure_label_delta"], sections.scorerDelta["failure_label_delta"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("summary failure_label_delta = %#v, want %#v", got, want)
	}
}

func TestBuildComparisonPresentationContextKeepsSectionInputsAligned(t *testing.T) {
	context := buildComparisonPresentationContext(nil, &comparisonReportBundle{
		baseView: newComparisonReportView(&EvalRunReport{
			EvalRun: &EvalRun{Title: "base-title"},
			GroupReport: &RunGroupReport{
				Group: &RunGroup{ID: "base-group"},
			},
		}),
		targetView: newComparisonReportView(&EvalRunReport{
			EvalRun: &EvalRun{Title: "target-title"},
			GroupReport: &RunGroupReport{
				Group: &RunGroup{ID: "target-group"},
			},
		}),
		rollup: comparisonCaseRollup{
			regressions:      []ComparisonCaseDelta{{Key: "regression-1"}},
			improvements:     []ComparisonCaseDelta{{Key: "improvement-1"}},
			changedCases:     3,
			unstableCases:    1,
			newFailures:      1,
			resolvedFailures: 1,
		},
		metrics: comparisonMetricBundle{
			base: comparisonViewMetricSnapshot{
				overallScore:           1,
				passRate:               1,
				verificationPassRate:   1,
				evidenceBackedPassRate: 0.8,
				retryRecoveredCount:    1,
			},
			target: comparisonViewMetricSnapshot{
				overallScore:           0.5,
				passRate:               0.5,
				verificationPassRate:   0.5,
				evidenceBackedPassRate: 0.3,
				retryRecoveredCount:    0,
			},
			structural: comparisonStructuralDelta{
				failureLabelDelta:   map[string]interface{}{"missing_artifact": 1},
				verdictCountDelta:   map[string]interface{}{"pass": -1, "fail": 1},
				linkedRunCountDelta: 2,
				artifactCountDelta:  -1,
			},
		},
	})

	if context.identity.baseTitle != "base-title" || context.identity.targetTitle != "target-title" {
		t.Fatalf("identity titles = (%q, %q), want (base-title, target-title)", context.identity.baseTitle, context.identity.targetTitle)
	}
	if context.rollup.changedCaseCount != 3 || context.rollup.regressionCount != 1 || context.rollup.improvementCount != 1 {
		t.Fatalf("rollup = %#v, want changed=3 regression=1 improvement=1", context.rollup)
	}
	if context.metricDelta.overallScoreDelta != -0.5 || context.metricDelta.retryRecoveredDelta != -1 {
		t.Fatalf("metric_delta = %#v, want overall=-0.5 retry=-1", context.metricDelta)
	}
	if context.scorer.linkedRunCountDelta != 2 || context.scorer.artifactCountDelta != -1 {
		t.Fatalf("scorer structural deltas = (%d, %d), want (2, -1)", context.scorer.linkedRunCountDelta, context.scorer.artifactCountDelta)
	}
	if len(context.body.regressions) != 1 || len(context.body.improvements) != 1 {
		t.Fatalf("body = %#v, want 1 regression and 1 improvement", context.body)
	}
}

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

	baseline := &Baseline{
		ID:          "baseline-1",
		Name:        "Default Baseline",
		OwnerUserID: "user-1",
		EvalSpecID:  evalSpec.ID,
		EvalRunID:   evalRun.ID,
		IsDefault:   true,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := store.CreateBaseline(ctx, baseline); err != nil {
		t.Fatalf("CreateBaseline failed: %v", err)
	}

	baselines, err := store.ListBaselines(ctx, BaselineFilter{OwnerUserID: "user-1", Limit: 10})
	if err != nil {
		t.Fatalf("ListBaselines failed: %v", err)
	}
	if len(baselines) != 1 || baselines[0].ID != baseline.ID {
		t.Fatalf("unexpected baselines: %#v", baselines)
	}

	comparison := &ComparisonReport{
		ID:              "comparison-1",
		OwnerUserID:     "user-1",
		EvalSpecID:      evalSpec.ID,
		BaseEvalRunID:   evalRun.ID,
		TargetEvalRunID: evalRun.ID,
		Summary: map[string]interface{}{
			"changed_case_count": 0,
		},
		CreatedAt: time.Now().UTC(),
	}
	if err := store.CreateComparisonReport(ctx, comparison); err != nil {
		t.Fatalf("CreateComparisonReport failed: %v", err)
	}

	storedComparison, err := store.GetComparisonReport(ctx, comparison.ID)
	if err != nil {
		t.Fatalf("GetComparisonReport failed: %v", err)
	}
	if storedComparison.ID != comparison.ID {
		t.Fatalf("stored comparison id = %q, want %q", storedComparison.ID, comparison.ID)
	}
	if storedComparison.OwnerUserID != comparison.OwnerUserID {
		t.Fatalf("stored comparison owner = %q, want %q", storedComparison.OwnerUserID, comparison.OwnerUserID)
	}
	if got := storedComparison.Summary["changed_case_count"]; got != float64(0) {
		t.Fatalf("stored comparison summary.changed_case_count = %#v, want 0", got)
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

func TestController_EvalRunSyncRefreshesDirtyGroupSummary(t *testing.T) {
	controller := newTestController(t)

	dataset, err := controller.CreateDataset(context.Background(), DatasetSpec{
		Name:           "Queued Eval Dataset",
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
						"goal": "wait for dispatch",
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
		Name:             "Queued Eval",
		OwnerUserID:      "user-1",
		DatasetID:        dataset.ID,
		DatasetVersionID: version.ID,
		RunKind:          RunKindAgentTask,
		Profile:          "agent_task",
	})
	if err != nil {
		t.Fatalf("CreateEvalSpec failed: %v", err)
	}

	evalRun, err := controller.SubmitEvalRun(context.Background(), EvalRunSpec{
		EvalSpecID:  evalSpec.ID,
		OwnerUserID: "user-1",
		Title:       "queued-eval-run",
	})
	if err != nil {
		t.Fatalf("SubmitEvalRun failed: %v", err)
	}
	if evalRun.Status != RunGroupStatusQueued {
		t.Fatalf("initial status = %s, want queued", evalRun.Status)
	}

	items, err := controller.ListGroupItems(context.Background(), evalRun.GroupID)
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

	listed, err := controller.ListEvalRuns(context.Background(), EvalRunFilter{
		OwnerUserID: "user-1",
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("ListEvalRuns failed: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("listed len = %d, want 1", len(listed))
	}
	if listed[0].Status != RunGroupStatusRunning {
		t.Fatalf("listed status = %s, want running", listed[0].Status)
	}
	if listed[0].StartedAt == nil {
		t.Fatal("expected listed eval run to have StartedAt set")
	}
	counts := nestedMetadataMap(listed[0].Summary, "counts")
	if gotCount := intMetadata(counts[string(RunGroupItemStatusRunning)]); gotCount != 1 {
		t.Fatalf("listed summary counts.running = %#v, want 1", counts[string(RunGroupItemStatusRunning)])
	}

	got, err := controller.GetEvalRun(context.Background(), evalRun.ID)
	if err != nil {
		t.Fatalf("GetEvalRun failed: %v", err)
	}
	if got.Status != RunGroupStatusRunning {
		t.Fatalf("status = %s, want running", got.Status)
	}
	if got.StartedAt == nil {
		t.Fatal("expected eval run to have StartedAt set")
	}

	stored, err := controller.store.GetEvalRun(context.Background(), evalRun.ID)
	if err != nil {
		t.Fatalf("GetEvalRun(stored) failed: %v", err)
	}
	if stored.Status != RunGroupStatusRunning {
		t.Fatalf("stored status = %s, want running", stored.Status)
	}
	storedCounts := nestedMetadataMap(stored.Summary, "counts")
	if gotCount := intMetadata(storedCounts[string(RunGroupItemStatusRunning)]); gotCount != 1 {
		t.Fatalf("stored summary counts.running = %#v, want 1", storedCounts[string(RunGroupItemStatusRunning)])
	}
}

func TestController_GetEvalRunReportUsesConsistentGroupSnapshot(t *testing.T) {
	controller := newTestController(t)
	driver := &stagedSnapshotDriver{
		kind:         RunKindAgentTask,
		controller:   controller,
		syncStatuses: []RunStatus{RunStatusPending, RunStatusExecuting},
	}
	controller.RegisterDriver(driver)

	dataset, err := controller.CreateDataset(context.Background(), DatasetSpec{
		Name:           "Eval Report Dataset",
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
						"goal": "assemble eval report",
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
		Name:             "Eval Report",
		OwnerUserID:      "user-1",
		DatasetID:        dataset.ID,
		DatasetVersionID: version.ID,
		RunKind:          RunKindAgentTask,
		Profile:          "agent_task",
	})
	if err != nil {
		t.Fatalf("CreateEvalSpec failed: %v", err)
	}

	evalRun, err := controller.SubmitEvalRun(context.Background(), EvalRunSpec{
		EvalSpecID:  evalSpec.ID,
		OwnerUserID: "user-1",
		Title:       "eval-report-run",
	})
	if err != nil {
		t.Fatalf("SubmitEvalRun failed: %v", err)
	}

	items, err := controller.ListGroupItems(context.Background(), evalRun.GroupID)
	if err != nil {
		t.Fatalf("ListGroupItems failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}

	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:        RunKindAgentTask,
		Goal:        "assemble eval report",
		UserID:      "user-1",
		GroupID:     evalRun.GroupID,
		GroupItemID: items[0].ID,
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	items[0].Status = RunGroupItemStatusRunning
	items[0].AttemptCount = 1
	items[0].LatestRunID = run.ID
	if err := controller.store.UpdateGroupItem(context.Background(), &items[0]); err != nil {
		t.Fatalf("UpdateGroupItem failed: %v", err)
	}

	report, err := controller.GetEvalRunReport(context.Background(), evalRun.ID)
	if err != nil {
		t.Fatalf("GetEvalRunReport failed: %v", err)
	}
	if report.EvalRun == nil || report.EvalRun.Status != RunGroupStatusRunning {
		t.Fatalf("eval run status = %#v, want running", report.EvalRun)
	}
	if report.GroupReport == nil || report.GroupReport.Group == nil {
		t.Fatalf("expected group report, got %#v", report.GroupReport)
	}
	if report.GroupReport.Group.Status != RunGroupStatusRunning {
		t.Fatalf("group report status = %s, want running", report.GroupReport.Group.Status)
	}
	if report.GroupReport.Group.ID != evalRun.GroupID {
		t.Fatalf("group report id = %q, want %q", report.GroupReport.Group.ID, evalRun.GroupID)
	}
	if len(report.GroupReport.LinkedRuns) != 1 {
		t.Fatalf("linked runs len = %d, want 1", len(report.GroupReport.LinkedRuns))
	}
	if report.GroupReport.LinkedRuns[0].Status != RunStatusExecuting {
		t.Fatalf("linked run status = %s, want executing", report.GroupReport.LinkedRuns[0].Status)
	}
	if report.GroupReport.LinkedRuns[0].StartedAt == nil {
		t.Fatal("expected linked run StartedAt to be set")
	}

	stored, err := controller.store.GetEvalRun(context.Background(), evalRun.ID)
	if err != nil {
		t.Fatalf("GetEvalRun(stored) failed: %v", err)
	}
	if stored.Status != RunGroupStatusRunning {
		t.Fatalf("stored eval run status = %s, want running", stored.Status)
	}
}

func TestController_CancelEvalRunSyncsCancelledGroupSnapshot(t *testing.T) {
	controller := newTestController(t)

	dataset, err := controller.CreateDataset(context.Background(), DatasetSpec{
		Name:           "Cancelled Eval Dataset",
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
						"goal": "cancel this eval",
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
		Name:             "Cancelled Eval",
		OwnerUserID:      "user-1",
		DatasetID:        dataset.ID,
		DatasetVersionID: version.ID,
		RunKind:          RunKindAgentTask,
		Profile:          "agent_task",
	})
	if err != nil {
		t.Fatalf("CreateEvalSpec failed: %v", err)
	}

	evalRun, err := controller.SubmitEvalRun(context.Background(), EvalRunSpec{
		EvalSpecID:  evalSpec.ID,
		OwnerUserID: "user-1",
		Title:       "cancelled-eval-run",
	})
	if err != nil {
		t.Fatalf("SubmitEvalRun failed: %v", err)
	}

	if err := controller.CancelEvalRun(context.Background(), evalRun.ID, "user aborted eval"); err != nil {
		t.Fatalf("CancelEvalRun failed: %v", err)
	}

	got, err := controller.GetEvalRun(context.Background(), evalRun.ID)
	if err != nil {
		t.Fatalf("GetEvalRun failed: %v", err)
	}
	if got.Status != RunGroupStatusCancelled {
		t.Fatalf("status = %q, want %q", got.Status, RunGroupStatusCancelled)
	}
	if got.StartedAt == nil {
		t.Fatal("expected cancelled eval run to keep StartedAt")
	}
	if got.FinishedAt == nil {
		t.Fatal("expected cancelled eval run to keep FinishedAt")
	}
	if got.Summary["cancel_reason"] != "user aborted eval" {
		t.Fatalf("summary.cancel_reason = %#v, want %q", got.Summary["cancel_reason"], "user aborted eval")
	}
	counts := nestedMetadataMap(got.Summary, "counts")
	if gotCount := intMetadata(counts[string(RunGroupItemStatusCancelled)]); gotCount != 1 {
		t.Fatalf("summary counts.cancelled = %#v, want 1", counts[string(RunGroupItemStatusCancelled)])
	}

	stored, err := controller.store.GetEvalRun(context.Background(), evalRun.ID)
	if err != nil {
		t.Fatalf("GetEvalRun(stored) failed: %v", err)
	}
	if stored.Status != RunGroupStatusCancelled {
		t.Fatalf("stored status = %q, want %q", stored.Status, RunGroupStatusCancelled)
	}
	if stored.Summary["cancel_reason"] != "user aborted eval" {
		t.Fatalf("stored summary.cancel_reason = %#v, want %q", stored.Summary["cancel_reason"], "user aborted eval")
	}
}

func TestController_LoadEvalRunGroupSnapshotRejectsNilEvalRun(t *testing.T) {
	controller := newTestController(t)

	snapshot, err := controller.loadEvalRunGroupSnapshot(context.Background(), nil)
	if err == nil {
		t.Fatalf("loadEvalRunGroupSnapshot returned %#v, want error", snapshot)
	}
	if err.Error() != "eval run is required" {
		t.Fatalf("error = %v, want eval run is required", err)
	}
}

func TestController_LoadEvalRunDatasetContextRejectsNilEvalRun(t *testing.T) {
	controller := newTestController(t)

	datasetContext, err := controller.loadEvalRunDatasetContext(context.Background(), nil)
	if err == nil {
		t.Fatalf("loadEvalRunDatasetContext returned %#v, want error", datasetContext)
	}
	if err.Error() != "eval run is required" {
		t.Fatalf("error = %v, want eval run is required", err)
	}
}

func TestController_LoadEvalRunDatasetContextFallsBackToEvalSpecDatasetVersionID(t *testing.T) {
	controller := newTestController(t)

	dataset, err := controller.CreateDataset(context.Background(), DatasetSpec{
		Name:           "Legacy Eval Dataset Context",
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
			"items": []interface{}{},
		},
		CreatedBy: "user-1",
	})
	if err != nil {
		t.Fatalf("CreateDatasetVersion failed: %v", err)
	}

	evalSpec, err := controller.CreateEvalSpec(context.Background(), EvalSpecSpec{
		Name:             "Legacy Eval Dataset Context",
		OwnerUserID:      "user-1",
		DatasetID:        dataset.ID,
		DatasetVersionID: version.ID,
		RunKind:          RunKindAgentTask,
		Profile:          "agent_task",
	})
	if err != nil {
		t.Fatalf("CreateEvalSpec failed: %v", err)
	}

	group := &RunGroup{
		ID:          "legacy-eval-dataset-context-group",
		Kind:        RunGroupKindEval,
		Title:       "legacy-eval-dataset-context-group",
		Status:      RunGroupStatusCompleted,
		OwnerUserID: "user-1",
		Subject:     "agent_task",
		Summary: map[string]interface{}{
			"item_count": 0,
		},
	}
	if err := controller.store.CreateGroup(context.Background(), group); err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}

	legacyEvalRun := &EvalRun{
		ID:          "legacy-eval-dataset-context-run",
		EvalSpecID:  evalSpec.ID,
		GroupID:     group.ID,
		Title:       "legacy-eval-dataset-context-run",
		OwnerUserID: "user-1",
		Status:      RunGroupStatusCompleted,
	}
	if err := controller.store.CreateEvalRun(context.Background(), legacyEvalRun); err != nil {
		t.Fatalf("CreateEvalRun failed: %v", err)
	}

	datasetContext, err := controller.loadEvalRunDatasetContext(context.Background(), legacyEvalRun)
	if err != nil {
		t.Fatalf("loadEvalRunDatasetContext failed: %v", err)
	}
	if datasetContext.specContext == nil || datasetContext.specContext.evalSpec == nil || datasetContext.specContext.evalSpec.ID != evalSpec.ID {
		t.Fatalf("spec context = %#v, want eval spec %q", datasetContext.specContext, evalSpec.ID)
	}
	if datasetContext.dataset == nil || datasetContext.dataset.ID != dataset.ID {
		t.Fatalf("dataset = %#v, want %q", datasetContext.dataset, dataset.ID)
	}
	if datasetContext.datasetVersion == nil || datasetContext.datasetVersion.ID != version.ID {
		t.Fatalf("dataset version = %#v, want %q", datasetContext.datasetVersion, version.ID)
	}
	if datasetContext.specContext.snapshot == nil || datasetContext.specContext.snapshot.group == nil || datasetContext.specContext.snapshot.group.ID != group.ID {
		t.Fatalf("snapshot group = %#v, want %q", datasetContext.specContext.snapshot, group.ID)
	}
}

func TestController_GetEvalRunReportFallsBackToEvalSpecDatasetVersionID(t *testing.T) {
	controller := newTestController(t)

	dataset, err := controller.CreateDataset(context.Background(), DatasetSpec{
		Name:           "Legacy Eval Dataset",
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
			"items": []interface{}{},
		},
		CreatedBy: "user-1",
	})
	if err != nil {
		t.Fatalf("CreateDatasetVersion failed: %v", err)
	}

	evalSpec, err := controller.CreateEvalSpec(context.Background(), EvalSpecSpec{
		Name:             "Legacy Eval Report",
		OwnerUserID:      "user-1",
		DatasetID:        dataset.ID,
		DatasetVersionID: version.ID,
		RunKind:          RunKindAgentTask,
		Profile:          "agent_task",
	})
	if err != nil {
		t.Fatalf("CreateEvalSpec failed: %v", err)
	}

	group := &RunGroup{
		ID:          "legacy-eval-group",
		Kind:        RunGroupKindEval,
		Title:       "legacy-eval-group",
		Status:      RunGroupStatusCompleted,
		OwnerUserID: "user-1",
		Subject:     "agent_task",
		Summary: map[string]interface{}{
			"item_count": 0,
		},
	}
	if err := controller.store.CreateGroup(context.Background(), group); err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}

	legacyEvalRun := &EvalRun{
		ID:          "legacy-eval-run",
		EvalSpecID:  evalSpec.ID,
		GroupID:     group.ID,
		Title:       "legacy-eval-run",
		OwnerUserID: "user-1",
		Status:      RunGroupStatusCompleted,
	}
	if err := controller.store.CreateEvalRun(context.Background(), legacyEvalRun); err != nil {
		t.Fatalf("CreateEvalRun failed: %v", err)
	}

	report, err := controller.GetEvalRunReport(context.Background(), legacyEvalRun.ID)
	if err != nil {
		t.Fatalf("GetEvalRunReport failed: %v", err)
	}
	if report.EvalSpec == nil || report.EvalSpec.ID != evalSpec.ID {
		t.Fatalf("report eval spec = %#v, want %q", report.EvalSpec, evalSpec.ID)
	}
	if report.Dataset == nil || report.Dataset.ID != dataset.ID {
		t.Fatalf("report dataset = %#v, want %q", report.Dataset, dataset.ID)
	}
	if report.DatasetVersion == nil || report.DatasetVersion.ID != version.ID {
		t.Fatalf("report dataset version = %#v, want %q", report.DatasetVersion, version.ID)
	}
	if report.GroupReport == nil || report.GroupReport.Group == nil || report.GroupReport.Group.ID != group.ID {
		t.Fatalf("report group = %#v, want %q", report.GroupReport, group.ID)
	}

	stored, err := controller.store.GetEvalRun(context.Background(), legacyEvalRun.ID)
	if err != nil {
		t.Fatalf("GetEvalRun(stored) failed: %v", err)
	}
	if stored.DatasetVersionID != "" {
		t.Fatalf("stored dataset_version_id = %q, want empty legacy value", stored.DatasetVersionID)
	}
}

func TestController_GetEvalRunReportIncludesCheckpointArtifacts(t *testing.T) {
	controller := newTestController(t)
	controller.RegisterDriver(&autoCompleteGroupDriver{
		kind:   RunKindAgentTask,
		status: RunStatusCompleted,
		result: "checkpoint-ready result",
		delay:  10 * time.Millisecond,
		onStart: func(run *Run, env RunEnv) error {
			return env.Manager.AttachArtifact(context.Background(), ArtifactRef{
				ID:           "checkpoint-" + run.ID,
				RunID:        run.ID,
				Kind:         "checkpoint",
				Label:        "checkpoint-1",
				PathOrURL:    "checkpoint-1.json",
				MetadataJSON: marshalMetadata(map[string]interface{}{"summary": "resume from checkpoint"}),
			})
		},
	})

	dataset, err := controller.CreateDataset(context.Background(), DatasetSpec{
		Name:           "Checkpoint Eval Dataset",
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
						"goal": "produce a checkpoint",
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
		Name:             "Checkpoint Eval",
		OwnerUserID:      "user-1",
		DatasetID:        dataset.ID,
		DatasetVersionID: version.ID,
		RunKind:          RunKindAgentTask,
		Profile:          "agent_task",
	})
	if err != nil {
		t.Fatalf("CreateEvalSpec failed: %v", err)
	}

	dispatcher := NewGroupDispatcher(controller)
	dispatcher.SetRunPollInterval(10 * time.Millisecond)
	evalRun, err := controller.SubmitEvalRun(context.Background(), EvalRunSpec{
		EvalSpecID:  evalSpec.ID,
		OwnerUserID: "user-1",
		Title:       "checkpoint-eval-run",
	})
	if err != nil {
		t.Fatalf("SubmitEvalRun failed: %v", err)
	}
	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatalf("DispatchOnce failed: %v", err)
	}

	waitForCondition(t, "checkpoint eval completion", func() bool {
		got, err := controller.GetEvalRun(context.Background(), evalRun.ID)
		return err == nil && got != nil && got.Status == RunGroupStatusCompleted
	})

	report, err := controller.GetEvalRunReport(context.Background(), evalRun.ID)
	if err != nil {
		t.Fatalf("GetEvalRunReport failed: %v", err)
	}
	if report.GroupReport == nil {
		t.Fatalf("expected group report, got %#v", report.GroupReport)
	}
	if len(report.GroupReport.LinkedRuns) != 1 {
		t.Fatalf("linked runs len = %d, want 1", len(report.GroupReport.LinkedRuns))
	}
	if len(report.GroupReport.Artifacts) == 0 {
		t.Fatalf("artifacts len = %d, want at least 1", len(report.GroupReport.Artifacts))
	}
	if len(report.GroupReport.Checkpoints) == 0 {
		t.Fatalf("checkpoints len = %d, want at least 1", len(report.GroupReport.Checkpoints))
	}
	foundCheckpointArtifact := false
	for _, artifact := range report.GroupReport.Artifacts {
		if artifact.Kind == "checkpoint" && artifact.ID == "checkpoint-"+report.GroupReport.LinkedRuns[0].ID {
			foundCheckpointArtifact = true
			break
		}
	}
	if !foundCheckpointArtifact {
		t.Fatalf("artifacts = %#v, want checkpoint artifact for linked run", report.GroupReport.Artifacts)
	}
	foundCheckpoint := false
	for _, checkpoint := range report.GroupReport.Checkpoints {
		if checkpoint.RunID != report.GroupReport.LinkedRuns[0].ID {
			continue
		}
		if checkpoint.Artifact.ID != "checkpoint-"+report.GroupReport.LinkedRuns[0].ID {
			continue
		}
		if metadataString(checkpoint.Payload, "summary") != "resume from checkpoint" {
			continue
		}
		foundCheckpoint = true
		break
	}
	if !foundCheckpoint {
		t.Fatalf("checkpoints = %#v, want attached checkpoint payload for linked run", report.GroupReport.Checkpoints)
	}
}

func TestMaterializeEvalGroupSpec_NormalizesHarnessContract(t *testing.T) {
	evalSpec := &EvalSpec{
		ID:               "eval-spec-contract",
		Name:             "Contract Eval",
		RunKind:          RunKindAgentTask,
		Profile:          "agent_task",
		DatasetID:        "dataset-1",
		DatasetVersionID: "version-1",
		RuntimePolicy: map[string]interface{}{
			"deliverables":   []interface{}{"ship the parser fix"},
			"fallback_order": []interface{}{"inspect parser diff", "retry with narrower change"},
			"browser_checks": []interface{}{
				map[string]interface{}{
					"name":                 "open preview",
					"required_observation": "browser_used",
					"require_screenshot":   true,
				},
			},
		},
	}
	dataset := &Dataset{
		ID:             "dataset-1",
		Name:           "Dataset",
		DefaultRunKind: RunKindAgentTask,
		DefaultProfile: "agent_task",
	}
	version := &DatasetVersion{
		ID:        "version-1",
		DatasetID: dataset.ID,
		Version:   "v1",
		Manifest: map[string]interface{}{
			"defaults": map[string]interface{}{
				"run_kind": "agent_task",
				"profile":  "agent_task",
			},
			"items": []interface{}{
				map[string]interface{}{
					"id": "case-1",
					"input": map[string]interface{}{
						"goal": "fix the parser",
					},
					"expected": map[string]interface{}{
						"required_tool_calls": []interface{}{"write"},
					},
				},
			},
		},
	}

	groupSpec, err := materializeEvalGroupSpec(evalSpec, dataset, version, EvalRunSpec{})
	if err != nil {
		t.Fatalf("materializeEvalGroupSpec failed: %v", err)
	}
	if len(groupSpec.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(groupSpec.Items))
	}
	groupContract := nestedMetadataMap(groupSpec.Metadata, "harness_contract")
	if len(groupContract) == 0 {
		t.Fatalf("expected group metadata to include normalized harness_contract, got %#v", groupSpec.Metadata)
	}
	itemContract := nestedMetadataMap(groupSpec.Items[0].Metadata, "harness_contract")
	if len(itemContract) == 0 {
		t.Fatalf("expected item metadata to include normalized harness_contract, got %#v", groupSpec.Items[0].Metadata)
	}
	if got := decodeStringSlice(groupSpec.Items[0].Metadata["task_success_criteria"]); len(got) == 0 || got[0] != "ship the parser fix" {
		t.Fatalf("task_success_criteria = %#v, want deliverables propagated", groupSpec.Items[0].Metadata["task_success_criteria"])
	}
	if got := decodeStringSlice(groupSpec.Items[0].Expected["required_tool_calls"]); len(got) != 1 || got[0] != "write" {
		t.Fatalf("required_tool_calls = %#v, want [write]", groupSpec.Items[0].Expected["required_tool_calls"])
	}
	if got := groupSpec.Items[0].Expected["browser_checks"]; got == nil {
		t.Fatalf("expected browser_checks to be projected into expected contract, got %#v", groupSpec.Items[0].Expected)
	}
}

func TestController_PromoteGroupCreatesReusableEvalAssets(t *testing.T) {
	controller := newTestController(t)
	controller.RegisterDriver(&autoCompleteGroupDriver{
		kind:   RunKindAgentTask,
		status: RunStatusCompleted,
		result: "verified result",
		delay:  10 * time.Millisecond,
	})

	group, err := controller.SubmitGroup(context.Background(), RunGroupSpec{
		Kind:        RunGroupKindEval,
		Title:       "Quick smoke group",
		OwnerUserID: "user-1",
		Subject:     "agent_task",
		SchedulerConfig: GroupSchedulerConfig{
			MaxConcurrency: 2,
			MaxAttempts:    1,
		},
		ScoringConfig: GroupScoringConfig{
			Mode:          ScoringModeRule,
			PassThreshold: 0.5,
		},
		Metadata: map[string]interface{}{
			"quick_eval": true,
			"ephemeral":  true,
		},
		Items: []RunGroupItemSpec{
			{
				RunKind: RunKindAgentTask,
				Profile: "smoke",
				Input: map[string]interface{}{
					"goal": "finish and verify the task",
				},
				Expected: map[string]interface{}{
					"contains": "verified",
				},
				Metadata: map[string]interface{}{
					"source_mode": "manifest",
				},
			},
			{
				RunKind: RunKindAgentTask,
				Profile: "smoke",
				Input: map[string]interface{}{
					"goal": "finish the follow-up task",
				},
				Expected: map[string]interface{}{
					"contains": "verified",
				},
				Metadata: map[string]interface{}{
					"dataset_case_id": "existing-case-2",
					"source_mode":     "manifest",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("SubmitGroup failed: %v", err)
	}

	result, err := controller.PromoteGroup(context.Background(), group.ID, GroupPromotionSpec{
		DatasetName: "Promoted quick smoke",
		Description: "Promoted from an ephemeral quick eval.",
		Subject:     "agent_task",
		EvalName:    "Promoted quick smoke eval",
	})
	if err != nil {
		t.Fatalf("PromoteGroup failed: %v", err)
	}
	if result == nil || result.Dataset == nil || result.DatasetVersion == nil || result.EvalSpec == nil {
		t.Fatalf("unexpected promotion result: %#v", result)
	}
	if result.Dataset.Metadata["promoted_from_group_id"] != group.ID {
		t.Fatalf("dataset metadata = %#v, want promoted_from_group_id=%s", result.Dataset.Metadata, group.ID)
	}
	if result.DatasetVersion.SourceType != "group_promotion" || result.DatasetVersion.SourceRef != group.ID {
		t.Fatalf("dataset version source = (%q, %q), want (group_promotion, %q)", result.DatasetVersion.SourceType, result.DatasetVersion.SourceRef, group.ID)
	}
	defaults, _ := result.DatasetVersion.Manifest["defaults"].(map[string]interface{})
	if defaults["run_kind"] != "agent_task" {
		t.Fatalf("defaults.run_kind = %#v, want agent_task", defaults["run_kind"])
	}
	if defaults["profile"] != "smoke" {
		t.Fatalf("defaults.profile = %#v, want smoke", defaults["profile"])
	}
	itemsRaw, _ := result.DatasetVersion.Manifest["items"].([]interface{})
	if len(itemsRaw) != 2 {
		t.Fatalf("manifest items = %#v, want 2 items", result.DatasetVersion.Manifest["items"])
	}
	firstItem, _ := itemsRaw[0].(map[string]interface{})
	secondItem, _ := itemsRaw[1].(map[string]interface{})
	if firstItem["id"] != "group-"+group.ID+"-item-0" {
		t.Fatalf("first item id = %#v, want generated group item id", firstItem["id"])
	}
	if secondItem["id"] != "existing-case-2" {
		t.Fatalf("second item id = %#v, want existing-case-2", secondItem["id"])
	}
	if result.EvalSpec.DatasetID != result.Dataset.ID || result.EvalSpec.DatasetVersionID != result.DatasetVersion.ID {
		t.Fatalf("eval spec dataset binding = (%q, %q), want (%q, %q)", result.EvalSpec.DatasetID, result.EvalSpec.DatasetVersionID, result.Dataset.ID, result.DatasetVersion.ID)
	}

	dispatcher := NewGroupDispatcher(controller)
	dispatcher.SetRunPollInterval(10 * time.Millisecond)
	evalRun, err := controller.SubmitEvalRun(context.Background(), EvalRunSpec{
		EvalSpecID:  result.EvalSpec.ID,
		OwnerUserID: "user-1",
		Title:       "promoted-rerun",
	})
	if err != nil {
		t.Fatalf("SubmitEvalRun failed: %v", err)
	}
	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatalf("DispatchOnce failed: %v", err)
	}

	waitForCondition(t, "promoted eval run completion", func() bool {
		got, err := controller.GetEvalRun(context.Background(), evalRun.ID)
		return err == nil && got != nil && got.Status == RunGroupStatusCompleted
	})
}

func TestController_CompareEvalRunCreatesRegressionReport(t *testing.T) {
	controller := newTestController(t)
	controller.RegisterDriver(&autoCompleteGroupDriver{
		kind:   RunKindAgentTask,
		status: RunStatusCompleted,
		result: "verified baseline",
		delay:  10 * time.Millisecond,
	})

	dataset, err := controller.CreateDataset(context.Background(), DatasetSpec{
		Name:           "Agent Task Compare",
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
		Name:             "Compare Eval",
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

	dispatcher := NewGroupDispatcher(controller)
	dispatcher.SetRunPollInterval(10 * time.Millisecond)

	baselineRun, err := controller.SubmitEvalRun(context.Background(), EvalRunSpec{
		EvalSpecID:  evalSpec.ID,
		OwnerUserID: "user-1",
		Title:       "baseline-run",
	})
	if err != nil {
		t.Fatalf("SubmitEvalRun baseline failed: %v", err)
	}
	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatalf("DispatchOnce baseline failed: %v", err)
	}

	waitForCondition(t, "baseline completion", func() bool {
		got, err := controller.GetEvalRun(context.Background(), baselineRun.ID)
		return err == nil && got != nil && got.Status == RunGroupStatusCompleted
	})

	baseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "release-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   baselineRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline failed: %v", err)
	}

	controller.RegisterDriver(&autoCompleteGroupDriver{
		kind:   RunKindAgentTask,
		status: RunStatusCompleted,
		result: "broken candidate",
		delay:  10 * time.Millisecond,
	})

	candidateRun, err := controller.SubmitEvalRun(context.Background(), EvalRunSpec{
		EvalSpecID:  evalSpec.ID,
		OwnerUserID: "user-1",
		Title:       "candidate-run",
	})
	if err != nil {
		t.Fatalf("SubmitEvalRun candidate failed: %v", err)
	}
	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatalf("DispatchOnce candidate failed: %v", err)
	}

	waitForCondition(t, "candidate completion", func() bool {
		got, err := controller.GetEvalRun(context.Background(), candidateRun.ID)
		return err == nil && got != nil && got.Status == RunGroupStatusFailed
	})

	comparison, err := controller.CompareEvalRun(context.Background(), candidateRun.ID, CompareEvalRunRequest{
		BaselineID: baseline.ID,
	})
	if err != nil {
		t.Fatalf("CompareEvalRun failed: %v", err)
	}
	if comparison.BaselineID != baseline.ID {
		t.Fatalf("comparison baseline id = %q, want %q", comparison.BaselineID, baseline.ID)
	}
	if len(comparison.Regressions) != 1 {
		t.Fatalf("comparison regressions = %#v, want 1 regression", comparison.Regressions)
	}
	if comparison.Regressions[0].BaseVerdict != "pass" || comparison.Regressions[0].TargetVerdict != "fail" {
		t.Fatalf("unexpected regression delta: %#v", comparison.Regressions[0])
	}
	if got := comparison.Summary["new_failure_count"]; got != 1 {
		t.Fatalf("new_failure_count = %#v, want 1", got)
	}
	if got := comparison.Summary["regression_count"]; got != 1 {
		t.Fatalf("regression_count = %#v, want 1", got)
	}
}

func TestController_CompareEvalRunIncludesVerificationDeltas(t *testing.T) {
	controller := newTestController(t)
	controller.RegisterDriver(&autoCompleteGroupDriver{
		kind:   RunKindAgentTask,
		status: RunStatusCompleted,
		result: "verified baseline",
		delay:  10 * time.Millisecond,
		onStart: func(run *Run, env RunEnv) error {
			return env.Manager.AttachArtifact(context.Background(), ArtifactRef{
				RunID:     run.ID,
				Kind:      "file",
				Label:     "result.txt",
				PathOrURL: "result.txt",
			})
		},
	})

	dataset, err := controller.CreateDataset(context.Background(), DatasetSpec{
		Name:           "Verification Compare",
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
						"goal": "finish and produce an artifact",
					},
					"expected": map[string]interface{}{
						"contains":           "verified",
						"expected_artifacts": []interface{}{"result.txt"},
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
		Name:             "Verification Delta Eval",
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

	dispatcher := NewGroupDispatcher(controller)
	dispatcher.SetRunPollInterval(10 * time.Millisecond)

	baselineRun, err := controller.SubmitEvalRun(context.Background(), EvalRunSpec{
		EvalSpecID:  evalSpec.ID,
		OwnerUserID: "user-1",
		Title:       "baseline-run",
	})
	if err != nil {
		t.Fatalf("SubmitEvalRun baseline failed: %v", err)
	}
	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatalf("DispatchOnce baseline failed: %v", err)
	}
	waitForCondition(t, "verification baseline completion", func() bool {
		got, err := controller.GetEvalRun(context.Background(), baselineRun.ID)
		return err == nil && got != nil && got.Status == RunGroupStatusCompleted
	})

	baseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "verification-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   baselineRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline failed: %v", err)
	}

	controller.RegisterDriver(&autoCompleteGroupDriver{
		kind:   RunKindAgentTask,
		status: RunStatusCompleted,
		result: "verified candidate",
		delay:  10 * time.Millisecond,
	})

	candidateRun, err := controller.SubmitEvalRun(context.Background(), EvalRunSpec{
		EvalSpecID:  evalSpec.ID,
		OwnerUserID: "user-1",
		Title:       "candidate-run",
	})
	if err != nil {
		t.Fatalf("SubmitEvalRun candidate failed: %v", err)
	}
	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatalf("DispatchOnce candidate failed: %v", err)
	}
	waitForCondition(t, "verification candidate completion", func() bool {
		got, err := controller.GetEvalRun(context.Background(), candidateRun.ID)
		return err == nil && got != nil && got.Status == RunGroupStatusFailed
	})

	comparison, err := controller.CompareEvalRun(context.Background(), candidateRun.ID, CompareEvalRunRequest{
		BaselineID: baseline.ID,
	})
	if err != nil {
		t.Fatalf("CompareEvalRun failed: %v", err)
	}
	if got := comparison.Summary["verification_pass_rate_delta"]; got != float64(-1) {
		t.Fatalf("verification_pass_rate_delta = %#v, want -1", got)
	}
	if got := comparison.Summary["evidence_backed_pass_rate_delta"]; got != float64(-1) {
		t.Fatalf("evidence_backed_pass_rate_delta = %#v, want -1", got)
	}
	if got := comparison.Summary["retry_recovered_delta"]; got != 0 {
		t.Fatalf("retry_recovered_delta = %#v, want 0", got)
	}
	failureDelta, ok := comparison.Summary["failure_label_delta"].(map[string]interface{})
	if !ok {
		t.Fatalf("failure_label_delta = %#v, want map", comparison.Summary["failure_label_delta"])
	}
	if got := failureDelta["missing_artifact"]; got != 1 {
		t.Fatalf("failure_label_delta[missing_artifact] = %#v, want 1", got)
	}
	if len(comparison.Regressions) != 1 {
		t.Fatalf("comparison regressions = %#v, want 1 regression", comparison.Regressions)
	}
	if comparison.Regressions[0].TargetFailureLabel != "missing_artifact" {
		t.Fatalf("target_failure_label = %q, want missing_artifact", comparison.Regressions[0].TargetFailureLabel)
	}
	if comparison.Regressions[0].BaseVerification != "passed" || comparison.Regressions[0].TargetVerification != "failed" {
		t.Fatalf("unexpected verification statuses: %#v", comparison.Regressions[0])
	}
}
