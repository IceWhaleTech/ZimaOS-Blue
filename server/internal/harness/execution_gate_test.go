package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

type executionEvalOutcome struct {
	Status RunStatus
	Result string
	Error  string
	Events []executionEventSpec
}

type executionEventSpec struct {
	Type     string
	ToolName string
	Message  string
	Payload  map[string]interface{}
}

type executionEvalDriver struct {
	outcomes       map[string]executionEvalOutcome
	defaultOutcome executionEvalOutcome
	delay          time.Duration
}

func (d executionEvalDriver) Kind() RunKind { return RunKindAgentTask }

func (d executionEvalDriver) Validate(spec RunSpec) error {
	if strings.TrimSpace(spec.Goal) == "" {
		return fmt.Errorf("goal is required")
	}
	return nil
}

func (d executionEvalDriver) Start(_ context.Context, run *Run, env RunEnv) error {
	if run == nil {
		return fmt.Errorf("run is required")
	}
	outcome := d.defaultOutcome
	if configured, ok := d.outcomes[run.Goal]; ok {
		outcome = configured
	}
	if outcome.Status == "" {
		outcome.Status = RunStatusCompleted
	}
	go func() {
		if d.delay > 0 {
			time.Sleep(d.delay)
		}
		events := outcome.Events
		if len(events) == 0 {
			events = defaultExecutionRouteEvents(run)
		}
		for _, event := range events {
			payloadJSON := ""
			if len(event.Payload) > 0 {
				raw, _ := json.Marshal(event.Payload)
				payloadJSON = string(raw)
			}
			_ = env.Manager.AppendEvent(context.Background(), RunEvent{
				RunID:       run.ID,
				Type:        strings.TrimSpace(event.Type),
				ToolName:    strings.TrimSpace(event.ToolName),
				Message:     strings.TrimSpace(event.Message),
				PayloadJSON: payloadJSON,
				CreatedAt:   time.Now().UTC(),
			})
		}
		snapshot := *run
		snapshot.Status = outcome.Status
		snapshot.Result = outcome.Result
		snapshot.Error = outcome.Error
		finished := time.Now().UTC()
		snapshot.FinishedAt = &finished
		_ = env.Manager.SyncSnapshot(context.Background(), &snapshot)
	}()
	return nil
}

func (d executionEvalDriver) Cancel(context.Context, *Run) error { return nil }

func defaultExecutionRouteEvents(run *Run) []executionEventSpec {
	if run == nil {
		return nil
	}
	primaryRoute := metadataString(run.Metadata, "primary_route")
	if primaryRoute == "" {
		return nil
	}
	if expectedCLIAction := metadataString(run.Metadata, "expected_cli_action"); expectedCLIAction != "" {
		requestPayload := map[string]interface{}{
			"tool_name": "exec",
			"arguments": map[string]interface{}{
				"command": expectedCLIAction,
			},
		}
		finishedPayload := map[string]interface{}{
			"tool_name": "exec",
			"result": map[string]interface{}{
				"status": "completed",
			},
		}
		if sessionID := strings.TrimSpace(run.SessionID); sessionID != "" {
			requestPayload["session_id"] = sessionID
			finishedPayload["session_id"] = sessionID
		}
		return []executionEventSpec{
			{
				Type:     "tool_requested",
				ToolName: "exec",
				Message:  "exec",
				Payload:  requestPayload,
			},
			{
				Type:     "tool_finished",
				ToolName: "exec",
				Message:  "exec completed",
				Payload:  finishedPayload,
			},
		}
	}
	payload := map[string]interface{}{
		"tool_name": primaryRoute,
	}
	if sessionID := strings.TrimSpace(run.SessionID); sessionID != "" {
		payload["session_id"] = sessionID
	}
	return []executionEventSpec{
		{
			Type:     "tool_requested",
			ToolName: primaryRoute,
			Message:  primaryRoute,
			Payload:  payload,
		},
	}
}

func TestController_EnsureBatch1ExecutionAssets_ReusesBuiltins(t *testing.T) {
	controller := newTestController(t)

	first, err := controller.EnsureBatch1ExecutionAssets(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("EnsureBatch1ExecutionAssets(first) failed: %v", err)
	}
	second, err := controller.EnsureBatch1ExecutionAssets(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("EnsureBatch1ExecutionAssets(second) failed: %v", err)
	}

	if first.Dataset == nil || second.Dataset == nil || first.Dataset.ID != second.Dataset.ID {
		t.Fatalf("dataset ids = %#v %#v, want same dataset", first.Dataset, second.Dataset)
	}
	if first.DatasetVersion == nil || second.DatasetVersion == nil || first.DatasetVersion.ID != second.DatasetVersion.ID {
		t.Fatalf("dataset version ids = %#v %#v, want same version", first.DatasetVersion, second.DatasetVersion)
	}
	if first.EvalSpec == nil || second.EvalSpec == nil || first.EvalSpec.ID != second.EvalSpec.ID {
		t.Fatalf("eval spec ids = %#v %#v, want same eval spec", first.EvalSpec, second.EvalSpec)
	}
}

func TestController_EnsureBatch1ExecutionAssets_ReusesEvalSpecLineageAcrossVersionUpgrade(t *testing.T) {
	controller := newTestController(t)

	dataset, err := controller.CreateDataset(context.Background(), Batch1ExecutionDatasetSpec("user-1"))
	if err != nil {
		t.Fatalf("CreateDataset failed: %v", err)
	}

	oldVersionSpec, err := Batch1ExecutionDatasetVersionSpec("tester")
	if err != nil {
		t.Fatalf("Batch1ExecutionDatasetVersionSpec failed: %v", err)
	}
	oldVersionSpec.Version = "skill-exec-batch1-v5"
	oldVersion, err := controller.CreateDatasetVersion(context.Background(), dataset.ID, oldVersionSpec)
	if err != nil {
		t.Fatalf("CreateDatasetVersion(old) failed: %v", err)
	}
	dataset.ActiveVersionID = oldVersion.ID
	if err := controller.store.UpdateDataset(context.Background(), dataset); err != nil {
		t.Fatalf("UpdateDataset failed: %v", err)
	}

	oldEvalSpecSpec := Batch1ExecutionEvalSpecSpec(dataset.ID, oldVersion.ID, "user-1")
	oldEvalSpecSpec.RuntimePolicy["policy_model_hint"] = "claude-haiku-4-5-20251001"
	oldEvalSpec, err := controller.CreateEvalSpec(context.Background(), oldEvalSpecSpec)
	if err != nil {
		t.Fatalf("CreateEvalSpec(old) failed: %v", err)
	}

	assets, err := controller.EnsureBatch1ExecutionAssets(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("EnsureBatch1ExecutionAssets failed: %v", err)
	}
	if assets.EvalSpec == nil {
		t.Fatal("EvalSpec = nil, want reused spec")
	}
	if assets.DatasetVersion == nil {
		t.Fatal("DatasetVersion = nil, want upgraded version")
	}
	if assets.EvalSpec.ID != oldEvalSpec.ID {
		t.Fatalf("eval spec id = %q, want reused %q", assets.EvalSpec.ID, oldEvalSpec.ID)
	}
	if assets.EvalSpec.DatasetVersionID != assets.DatasetVersion.ID {
		t.Fatalf("eval spec dataset_version_id = %q, want %q", assets.EvalSpec.DatasetVersionID, assets.DatasetVersion.ID)
	}
	if got := metadataString(assets.EvalSpec.RuntimePolicy, "policy_model_hint"); got != batch1ExecutionPolicyModelHint {
		t.Fatalf("runtime_policy.policy_model_hint = %q, want %q", got, batch1ExecutionPolicyModelHint)
	}

	specs, err := controller.ListEvalSpecs(context.Background(), EvalSpecFilter{
		OwnerUserID: "user-1",
		DatasetID:   dataset.ID,
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("ListEvalSpecs failed: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("eval specs len = %d, want 1", len(specs))
	}
}

func TestController_EnsureBatch1ExecutionAssets_PrefersHistoryBearingLineageWhenFreshVersionSpecAlreadyExists(t *testing.T) {
	controller := newTestController(t)

	dataset, err := controller.CreateDataset(context.Background(), Batch1ExecutionDatasetSpec("user-1"))
	if err != nil {
		t.Fatalf("CreateDataset failed: %v", err)
	}

	oldVersionSpec, err := Batch1ExecutionDatasetVersionSpec("tester")
	if err != nil {
		t.Fatalf("Batch1ExecutionDatasetVersionSpec failed: %v", err)
	}
	oldVersionSpec.Version = "skill-exec-batch1-v5"
	oldVersion, err := controller.CreateDatasetVersion(context.Background(), dataset.ID, oldVersionSpec)
	if err != nil {
		t.Fatalf("CreateDatasetVersion(old) failed: %v", err)
	}

	oldEvalSpec, err := controller.CreateEvalSpec(context.Background(), Batch1ExecutionEvalSpecSpec(dataset.ID, oldVersion.ID, "user-1"))
	if err != nil {
		t.Fatalf("CreateEvalSpec(old) failed: %v", err)
	}
	if err := controller.store.CreateEvalRun(context.Background(), &EvalRun{
		ID:               "old-history-run",
		EvalSpecID:       oldEvalSpec.ID,
		GroupID:          "old-history-group",
		DatasetVersionID: oldVersion.ID,
		OwnerUserID:      "user-1",
		Status:           RunGroupStatusCompleted,
		Title:            "old-history",
	}); err != nil {
		t.Fatalf("CreateEvalRun(history) failed: %v", err)
	}

	currentVersionSpec, err := Batch1ExecutionDatasetVersionSpec("tester")
	if err != nil {
		t.Fatalf("Batch1ExecutionDatasetVersionSpec(current) failed: %v", err)
	}
	currentVersion, err := controller.CreateDatasetVersion(context.Background(), dataset.ID, currentVersionSpec)
	if err != nil {
		t.Fatalf("CreateDatasetVersion(current) failed: %v", err)
	}
	if _, err := controller.CreateEvalSpec(context.Background(), Batch1ExecutionEvalSpecSpec(dataset.ID, currentVersion.ID, "user-1")); err != nil {
		t.Fatalf("CreateEvalSpec(current) failed: %v", err)
	}
	dataset.ActiveVersionID = currentVersion.ID
	if err := controller.store.UpdateDataset(context.Background(), dataset); err != nil {
		t.Fatalf("UpdateDataset failed: %v", err)
	}

	assets, err := controller.EnsureBatch1ExecutionAssets(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("EnsureBatch1ExecutionAssets failed: %v", err)
	}
	if assets.EvalSpec == nil {
		t.Fatal("EvalSpec = nil, want reused lineage spec")
	}
	if assets.EvalSpec.ID != oldEvalSpec.ID {
		t.Fatalf("eval spec id = %q, want history-bearing %q", assets.EvalSpec.ID, oldEvalSpec.ID)
	}
	if assets.EvalSpec.DatasetVersionID != currentVersion.ID {
		t.Fatalf("eval spec dataset_version_id = %q, want %q", assets.EvalSpec.DatasetVersionID, currentVersion.ID)
	}
}

func TestController_EnsureBatch1ExecutionAssets_FailsClearlyWhenBuiltinVersionManifestDiffers(t *testing.T) {
	controller := newTestController(t)

	dataset, err := controller.CreateDataset(context.Background(), Batch1ExecutionDatasetSpec("user-1"))
	if err != nil {
		t.Fatalf("CreateDataset failed: %v", err)
	}

	versionSpec, err := Batch1ExecutionDatasetVersionSpec("tester")
	if err != nil {
		t.Fatalf("Batch1ExecutionDatasetVersionSpec failed: %v", err)
	}

	raw, err := json.Marshal(versionSpec.Manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	var driftedManifest map[string]interface{}
	if err := json.Unmarshal(raw, &driftedManifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	driftedManifest["x_builtin_manifest_drift"] = true

	if _, err := controller.CreateDatasetVersion(context.Background(), dataset.ID, DatasetVersionSpec{
		Version:    Batch1ExecutionDatasetVersion,
		SourceType: versionSpec.SourceType,
		SourceRef:  versionSpec.SourceRef,
		Manifest:   driftedManifest,
		Metadata:   versionSpec.Metadata,
		CreatedBy:  "tester",
	}); err != nil {
		t.Fatalf("CreateDatasetVersion failed: %v", err)
	}

	_, err = controller.EnsureBatch1ExecutionAssets(context.Background(), "user-1")
	if err == nil {
		t.Fatal("EnsureBatch1ExecutionAssets error = nil, want builtin version conflict")
	}
	if !strings.Contains(err.Error(), "different manifest hash") {
		t.Fatalf("EnsureBatch1ExecutionAssets error = %v, want different manifest hash", err)
	}
	if !strings.Contains(err.Error(), Batch1ExecutionDatasetVersion) {
		t.Fatalf("EnsureBatch1ExecutionAssets error = %v, want version %q", err, Batch1ExecutionDatasetVersion)
	}
}

func TestController_EvaluateExecutionEquivalence_PassesWithinDefaultThresholds(t *testing.T) {
	controller := newTestController(t)
	items := executionGateSmokeItems(t)
	evalSpec := createExecutionEvalSpecForItems(t, controller, "Execution gate pass", items)

	baselineReport := runExecutionEvalReport(t, controller, evalSpec, "execution-baseline", executionEvalDriver{
		defaultOutcome: executionEvalOutcome{Status: RunStatusCompleted, Result: "completed"},
		delay:          10 * time.Millisecond,
	}, len(items))
	baseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "execution-gate-pass-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   baselineReport.EvalRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline failed: %v", err)
	}

	candidateReport := runExecutionEvalReport(t, controller, evalSpec, "execution-candidate", executionEvalDriver{
		defaultOutcome: executionEvalOutcome{Status: RunStatusCompleted, Result: "completed"},
		delay:          10 * time.Millisecond,
	}, len(items))

	report, err := controller.EvaluateExecutionEquivalence(context.Background(), candidateReport.EvalRun.ID, ExecutionEquivalenceRequest{
		BaselineID: baseline.ID,
	})
	if err != nil {
		t.Fatalf("EvaluateExecutionEquivalence failed: %v", err)
	}
	if !report.Passed {
		t.Fatalf("execution equivalence report = %#v, want pass", report)
	}
	if report.Metrics.PassRate != 1 {
		t.Fatalf("metrics.pass_rate = %#v, want 1", report.Metrics.PassRate)
	}
	if report.Metrics.PassRateDelta != 0 {
		t.Fatalf("metrics.pass_rate_delta = %#v, want 0", report.Metrics.PassRateDelta)
	}
	if report.Metrics.CriticalRegressionCount != 0 {
		t.Fatalf("metrics.critical_regression_count = %#v, want 0", report.Metrics.CriticalRegressionCount)
	}
	if got := report.Metrics.LocaleBreakdown["zh-CN"].PassRate; got != 1 {
		t.Fatalf("metrics.locale_breakdown[zh-CN].pass_rate = %#v, want 1", got)
	}
	if got := report.Metrics.PrimaryRouteBreakdown["analyze"].CriticalPassRate; got != 1 {
		t.Fatalf("metrics.primary_route_breakdown[analyze].critical_pass_rate = %#v, want 1", got)
	}
}

func TestController_EvaluateExecutionEquivalence_FailsPassRateDrop(t *testing.T) {
	controller := newTestController(t)
	items := executionGateSmokeItems(t)
	evalSpec := createExecutionEvalSpecForItems(t, controller, "Execution gate pass drop", items)

	baselineReport := runExecutionEvalReport(t, controller, evalSpec, "execution-baseline", executionEvalDriver{
		defaultOutcome: executionEvalOutcome{Status: RunStatusCompleted, Result: "completed"},
		delay:          10 * time.Millisecond,
	}, len(items))
	baseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "execution-gate-drop-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   baselineReport.EvalRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline failed: %v", err)
	}

	nonCriticalQuery := findExecutionManifestItem(t, items, "exec-web_search-en-us").Input["goal"].(string)
	candidateReport := runExecutionEvalReport(t, controller, evalSpec, "execution-candidate", executionEvalDriver{
		defaultOutcome: executionEvalOutcome{Status: RunStatusCompleted, Result: "completed"},
		outcomes: map[string]executionEvalOutcome{
			nonCriticalQuery: {Status: RunStatusFailed, Error: "web search execution failed"},
		},
		delay: 10 * time.Millisecond,
	}, len(items))

	report, err := controller.EvaluateExecutionEquivalence(context.Background(), candidateReport.EvalRun.ID, ExecutionEquivalenceRequest{
		BaselineID: baseline.ID,
	})
	if err != nil {
		t.Fatalf("EvaluateExecutionEquivalence failed: %v", err)
	}
	if report.Passed {
		t.Fatalf("execution equivalence report = %#v, want failure", report)
	}
	if report.Metrics.CriticalRegressionCount != 0 {
		t.Fatalf("metrics.critical_regression_count = %#v, want 0", report.Metrics.CriticalRegressionCount)
	}
	if report.Metrics.NewFailureCount != 1 {
		t.Fatalf("metrics.new_failure_count = %#v, want 1", report.Metrics.NewFailureCount)
	}
	if check := executionGateCheckByName(t, report.Checks, "pass_rate_drop"); check.Passed {
		t.Fatalf("pass_rate_drop check = %#v, want fail", check)
	}
}

func TestController_EvaluateExecutionEquivalence_FailsCriticalRegression(t *testing.T) {
	controller := newTestController(t)
	items := executionGateSmokeItems(t)
	evalSpec := createExecutionEvalSpecForItems(t, controller, "Execution gate critical regression", items)

	baselineReport := runExecutionEvalReport(t, controller, evalSpec, "execution-baseline", executionEvalDriver{
		defaultOutcome: executionEvalOutcome{Status: RunStatusCompleted, Result: "completed"},
		delay:          10 * time.Millisecond,
	}, len(items))
	baseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "execution-gate-critical-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   baselineReport.EvalRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline failed: %v", err)
	}

	criticalQuery := findExecutionManifestItem(t, items, "critical-analyze-url-summary-zh-cn").Input["goal"].(string)
	candidateReport := runExecutionEvalReport(t, controller, evalSpec, "execution-candidate", executionEvalDriver{
		defaultOutcome: executionEvalOutcome{Status: RunStatusCompleted, Result: "completed"},
		outcomes: map[string]executionEvalOutcome{
			criticalQuery: {Status: RunStatusFailed, Error: "url analyze execution failed"},
		},
		delay: 10 * time.Millisecond,
	}, len(items))

	report, err := controller.EvaluateExecutionEquivalence(context.Background(), candidateReport.EvalRun.ID, ExecutionEquivalenceRequest{
		BaselineID: baseline.ID,
	})
	if err != nil {
		t.Fatalf("EvaluateExecutionEquivalence failed: %v", err)
	}
	if report.Passed {
		t.Fatalf("execution equivalence report = %#v, want failure", report)
	}
	if report.Metrics.CriticalRegressionCount != 1 {
		t.Fatalf("metrics.critical_regression_count = %#v, want 1", report.Metrics.CriticalRegressionCount)
	}
	if got := report.Metrics.LocaleBreakdown["zh-CN"].CriticalRegressionCount; got != 1 {
		t.Fatalf("metrics.locale_breakdown[zh-CN].critical_regression_count = %#v, want 1", got)
	}
	if got := report.Metrics.PrimaryRouteBreakdown["analyze"].CriticalRegressionCount; got != 1 {
		t.Fatalf("metrics.primary_route_breakdown[analyze].critical_regression_count = %#v, want 1", got)
	}
	if check := executionGateCheckByName(t, report.Checks, "critical_regression_count"); check.Passed {
		t.Fatalf("critical_regression_count check = %#v, want fail", check)
	}
}

func TestController_EvaluateExecutionEquivalence_FailsVerificationRouteRegression(t *testing.T) {
	controller := newTestController(t)
	items := []DatasetManifestItem{
		findExecutionManifestItem(t, Batch1ExecutionDatasetManifest().Items, "critical-web-search-latest-docs-zh-cn"),
	}
	evalSpec := createExecutionEvalSpecForItems(t, controller, "Execution gate verification regression", items)

	baselineReport := runExecutionEvalReport(t, controller, evalSpec, "execution-baseline", executionEvalDriver{
		defaultOutcome: executionEvalOutcome{Status: RunStatusCompleted, Result: "completed"},
		delay:          10 * time.Millisecond,
	}, len(items))
	baseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "execution-gate-verification-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   baselineReport.EvalRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline failed: %v", err)
	}

	query := items[0].Input["goal"].(string)
	candidateReport := runExecutionEvalReport(t, controller, evalSpec, "execution-candidate", executionEvalDriver{
		defaultOutcome: executionEvalOutcome{Status: RunStatusCompleted, Result: "completed"},
		outcomes: map[string]executionEvalOutcome{
			query: {
				Status: RunStatusCompleted,
				Result: "completed",
				Events: []executionEventSpec{
					{
						Type:     "tool_requested",
						ToolName: "exec",
						Message:  "exec",
						Payload: map[string]interface{}{
							"tool_name": "exec",
							"arguments": map[string]interface{}{
								"command": "blue analyze topic=\"latest docs\"",
							},
						},
					},
				},
			},
		},
		delay: 10 * time.Millisecond,
	}, len(items))

	maxPassRateDrop := 1.0
	maxCriticalRegressions := 1
	maxVerificationPassRateDrop := 0.0
	report, err := controller.EvaluateExecutionEquivalence(context.Background(), candidateReport.EvalRun.ID, ExecutionEquivalenceRequest{
		BaselineID: baseline.ID,
		Thresholds: ExecutionEquivalenceThresholds{
			MaxPassRateDrop:             &maxPassRateDrop,
			MaxCriticalRegressionCount:  &maxCriticalRegressions,
			MaxVerificationPassRateDrop: &maxVerificationPassRateDrop,
		},
	})
	if err != nil {
		t.Fatalf("EvaluateExecutionEquivalence failed: %v", err)
	}
	if report.Passed {
		t.Fatalf("execution equivalence report = %#v, want verification failure", report)
	}
	if report.Metrics.VerificationPassRateDelta >= 0 {
		t.Fatalf("metrics.verification_pass_rate_delta = %#v, want negative drop", report.Metrics.VerificationPassRateDelta)
	}
	if check := executionGateCheckByName(t, report.Checks, "verification_pass_rate_drop"); check.Passed {
		t.Fatalf("verification_pass_rate_drop check = %#v, want fail", check)
	}
}

func TestController_EvaluateExecutionEquivalence_FailsReminderSessionRegression(t *testing.T) {
	controller := newTestController(t)
	items := executionGateSmokeItems(t)
	evalSpec := createExecutionEvalSpecForItems(t, controller, "Execution gate session regression", items)

	baselineReport := runExecutionEvalReport(t, controller, evalSpec, "execution-baseline", executionEvalDriver{
		defaultOutcome: executionEvalOutcome{Status: RunStatusCompleted, Result: "completed"},
		delay:          10 * time.Millisecond,
	}, len(items))
	baseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "execution-gate-session-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   baselineReport.EvalRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline failed: %v", err)
	}

	reminderGoal := findExecutionManifestItem(t, Batch1ExecutionDatasetManifest().Items, "critical-reminder-tomorrow-9-zh-cn").Input["goal"].(string)
	candidateReport := runExecutionEvalReport(t, controller, evalSpec, "execution-candidate", executionEvalDriver{
		defaultOutcome: executionEvalOutcome{Status: RunStatusCompleted, Result: "completed"},
		outcomes: map[string]executionEvalOutcome{
			reminderGoal: {
				Status: RunStatusCompleted,
				Result: "completed",
				Events: []executionEventSpec{
					{
						Type:     "tool_requested",
						ToolName: "exec",
						Message:  "exec",
						Payload: map[string]interface{}{
							"tool_name": "exec",
							"arguments": map[string]interface{}{
								"command": "blue reminder add",
							},
						},
					},
					{
						Type:     "tool_finished",
						ToolName: "exec",
						Message:  "exec completed",
						Payload: map[string]interface{}{
							"tool_name": "exec",
							"result": map[string]interface{}{
								"status": "completed",
							},
						},
					},
				},
			},
		},
		delay: 10 * time.Millisecond,
	}, len(items))

	maxPassRateDrop := 1.0
	maxCriticalRegressions := 1
	maxVerificationPassRateDrop := 0.0
	report, err := controller.EvaluateExecutionEquivalence(context.Background(), candidateReport.EvalRun.ID, ExecutionEquivalenceRequest{
		BaselineID: baseline.ID,
		Thresholds: ExecutionEquivalenceThresholds{
			MaxPassRateDrop:             &maxPassRateDrop,
			MaxCriticalRegressionCount:  &maxCriticalRegressions,
			MaxVerificationPassRateDrop: &maxVerificationPassRateDrop,
		},
	})
	if err != nil {
		t.Fatalf("EvaluateExecutionEquivalence failed: %v", err)
	}
	if report.Passed {
		t.Fatalf("execution equivalence report = %#v, want session verification failure", report)
	}
	if report.Metrics.VerificationPassRateDelta >= 0 {
		t.Fatalf("metrics.verification_pass_rate_delta = %#v, want negative drop", report.Metrics.VerificationPassRateDelta)
	}
	if check := executionGateCheckByName(t, report.Checks, "verification_pass_rate_drop"); check.Passed {
		t.Fatalf("verification_pass_rate_drop check = %#v, want fail", check)
	}
}

func TestController_EvaluateExecutionEquivalence_BlocksProviderInfraFailures(t *testing.T) {
	controller := newTestController(t)
	items := []DatasetManifestItem{
		findExecutionManifestItem(t, Batch1ExecutionDatasetManifest().Items, "exec-web_search-en-us"),
	}
	evalSpec := createExecutionEvalSpecForItems(t, controller, "Execution gate provider blocked", items)

	baselineReport := runExecutionEvalReport(t, controller, evalSpec, "execution-baseline", executionEvalDriver{
		defaultOutcome: executionEvalOutcome{Status: RunStatusCompleted, Result: "completed"},
		delay:          10 * time.Millisecond,
	}, len(items))
	baseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "execution-gate-provider-blocked-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   baselineReport.EvalRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline failed: %v", err)
	}

	query := items[0].Input["goal"].(string)
	candidateReport := runExecutionEvalReport(t, controller, evalSpec, "execution-candidate", executionEvalDriver{
		defaultOutcome: executionEvalOutcome{Status: RunStatusCompleted, Result: "completed"},
		outcomes: map[string]executionEvalOutcome{
			query: {
				Status: RunStatusFailed,
				Error:  "planning failed: parse response: unexpected end of JSON input, raw response:",
				Result: "Summary: The task failed before all grounded checks passed.",
				Events: []executionEventSpec{
					{
						Type:    "task_reflection_completed",
						Message: "Reflection did not produce reusable lessons.",
						Payload: map[string]interface{}{
							"output": "Reflection error: proxy returned 401: provider prov_x auth error (401): {\"error\":{\"type\":\"invalid_api_key\",\"message\":\"invalid access token or token expired\"}}",
						},
					},
					{
						Type:    "run_failed",
						Message: "Task failed during planning: parse response: unexpected end of JSON input, raw response:",
					},
				},
			},
		},
		delay: 10 * time.Millisecond,
	}, len(items))

	latestCards := latestScorecardsByItem(candidateReport.GroupReport.Scorecards)
	card, ok := latestCards[candidateReport.GroupReport.Items[0].ID]
	if !ok {
		t.Fatalf("latest scorecard missing for provider-blocked case")
	}
	if label := metadataString(decodeJSONMap(card.BreakdownJSON), "failure_label"); label != "infra_provider_auth" {
		t.Fatalf("failure_label = %q, want infra_provider_auth", label)
	}

	report, err := controller.EvaluateExecutionEquivalence(context.Background(), candidateReport.EvalRun.ID, ExecutionEquivalenceRequest{
		BaselineID: baseline.ID,
	})
	if err != nil {
		t.Fatalf("EvaluateExecutionEquivalence failed: %v", err)
	}
	if report.Passed {
		t.Fatalf("execution equivalence report = %#v, want blocked failure", report)
	}
	if report.Metrics.TargetInfraBlockedCount != 1 {
		t.Fatalf("metrics.target_infra_blocked_count = %#v, want 1", report.Metrics.TargetInfraBlockedCount)
	}
	if report.Metrics.InfraBlockedCount != 1 {
		t.Fatalf("metrics.infra_blocked_count = %#v, want 1", report.Metrics.InfraBlockedCount)
	}
	if report.Metrics.PassRate != 1 {
		t.Fatalf("metrics.pass_rate = %#v, want 1 with base-rate fallback for fully blocked candidates", report.Metrics.PassRate)
	}
	if report.Metrics.RegressionCount != 0 {
		t.Fatalf("metrics.regression_count = %#v, want 0", report.Metrics.RegressionCount)
	}
	if report.Metrics.NewFailureCount != 0 {
		t.Fatalf("metrics.new_failure_count = %#v, want 0", report.Metrics.NewFailureCount)
	}
	if check := executionGateCheckByName(t, report.Checks, "pass_rate_drop"); !check.Passed {
		t.Fatalf("pass_rate_drop check = %#v, want pass", check)
	}
	if check := executionGateCheckByName(t, report.Checks, "critical_regression_count"); !check.Passed {
		t.Fatalf("critical_regression_count check = %#v, want pass", check)
	}
	if check := executionGateCheckByName(t, report.Checks, "infra_blocked_count"); check.Passed {
		t.Fatalf("infra_blocked_count check = %#v, want fail", check)
	}
	if got := report.Metrics.PrimaryRouteBreakdown[harnessCanonicalWebQuerySkill].InfraBlockedCount; got != 1 {
		t.Fatalf("metrics.primary_route_breakdown[%s].infra_blocked_count = %#v, want 1", harnessCanonicalWebQuerySkill, got)
	}
}

func createExecutionEvalSpecForItems(t *testing.T, controller *Controller, datasetName string, items []DatasetManifestItem) *EvalSpec {
	t.Helper()
	manifest := DatasetManifest{
		Dataset: DatasetManifestMeta{
			Name:    datasetName,
			Subject: Batch1ExecutionDatasetSubject,
		},
		Defaults: DatasetManifestDefaults{
			RunKind:       RunKindAgentTask,
			Profile:       batch1ExecutionProfile,
			RuntimePolicy: batch1ExecutionRuntimePolicy(),
			Scoring: GroupScoringConfig{
				Mode:          ScoringModeRule,
				PassThreshold: 1,
			},
		},
		Items: items,
	}
	manifestRaw, err := datasetManifestMap(manifest)
	if err != nil {
		t.Fatalf("datasetManifestMap failed: %v", err)
	}

	dataset, err := controller.CreateDataset(context.Background(), DatasetSpec{
		Name:           datasetName,
		OwnerUserID:    "user-1",
		Subject:        Batch1ExecutionDatasetSubject,
		DefaultRunKind: RunKindAgentTask,
		DefaultProfile: batch1ExecutionProfile,
	})
	if err != nil {
		t.Fatalf("CreateDataset failed: %v", err)
	}
	version, err := controller.CreateDatasetVersion(context.Background(), dataset.ID, DatasetVersionSpec{
		Version:   "v1",
		Manifest:  manifestRaw,
		CreatedBy: "user-1",
	})
	if err != nil {
		t.Fatalf("CreateDatasetVersion failed: %v", err)
	}
	evalSpec, err := controller.CreateEvalSpec(context.Background(), EvalSpecSpec{
		Name:             datasetName + " eval",
		OwnerUserID:      "user-1",
		DatasetID:        dataset.ID,
		DatasetVersionID: version.ID,
		RunKind:          RunKindAgentTask,
		Profile:          batch1ExecutionProfile,
		RuntimePolicy:    batch1ExecutionRuntimePolicy(),
		ScoringConfig: GroupScoringConfig{
			Mode:          ScoringModeRule,
			PassThreshold: 1,
		},
	})
	if err != nil {
		t.Fatalf("CreateEvalSpec failed: %v", err)
	}
	return evalSpec
}

func runExecutionEvalReport(t *testing.T, controller *Controller, evalSpec *EvalSpec, title string, driver executionEvalDriver, wantScorecards int) *EvalRunReport {
	t.Helper()
	controller.RegisterDriver(driver)

	evalRun, err := controller.SubmitEvalRun(context.Background(), EvalRunSpec{
		EvalSpecID:  evalSpec.ID,
		OwnerUserID: "user-1",
		Title:       title,
	})
	if err != nil {
		t.Fatalf("SubmitEvalRun failed: %v", err)
	}

	dispatcher := NewGroupDispatcher(controller)
	dispatcher.SetPollInterval(10 * time.Millisecond)
	dispatcher.SetRunPollInterval(10 * time.Millisecond)
	dispatchCtx, cancelDispatch := context.WithCancel(context.Background())
	defer cancelDispatch()
	go dispatcher.Start(dispatchCtx)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		report, err := controller.GetEvalRunReport(context.Background(), evalRun.ID)
		if err == nil && selectorEvalReportTerminal(report, wantScorecards) {
			return report
		}
		time.Sleep(10 * time.Millisecond)
	}

	report, err := controller.GetEvalRunReport(context.Background(), evalRun.ID)
	if err != nil {
		t.Fatalf("GetEvalRunReport failed: %v", err)
	}
	raw, _ := json.Marshal(report)
	t.Fatalf("execution eval report = %s, want terminal with %d scorecards", string(raw), wantScorecards)
	return nil
}

func executionGateSmokeItems(t *testing.T) []DatasetManifestItem {
	t.Helper()
	fullManifest := Batch1ExecutionDatasetManifest()
	return []DatasetManifestItem{
		findExecutionManifestItem(t, fullManifest.Items, "exec-web_search-en-us"),
		findExecutionManifestItem(t, fullManifest.Items, "critical-analyze-url-summary-zh-cn"),
		findExecutionManifestItem(t, fullManifest.Items, "critical-reminder-tomorrow-9-zh-cn"),
	}
}

func executionGateCheckByName(t *testing.T, checks []ExecutionEquivalenceCheck, name string) ExecutionEquivalenceCheck {
	t.Helper()
	for _, check := range checks {
		if check.Name == name {
			return check
		}
	}
	t.Fatalf("execution gate check %q not found in %#v", name, checks)
	return ExecutionEquivalenceCheck{}
}
