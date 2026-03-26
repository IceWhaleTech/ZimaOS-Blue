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
