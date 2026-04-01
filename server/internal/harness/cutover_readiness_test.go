package harness

import (
	"context"
	"testing"
	"time"
)

func TestResolveSkillCutoverCandidateIDPrefersLatestSharedCandidate(t *testing.T) {
	now := time.Now()
	selectorRuns := []EvalRun{
		{ID: "selector-1", CreatedAt: now.Add(-2 * time.Minute), Metadata: map[string]interface{}{"candidate_id": "rc-old"}},
		{ID: "selector-2", CreatedAt: now.Add(-1 * time.Minute), Metadata: map[string]interface{}{"candidate_id": "rc-new"}},
	}
	executionRuns := []EvalRun{
		{ID: "execution-1", CreatedAt: now.Add(-90 * time.Second), Metadata: map[string]interface{}{"candidate_id": "rc-old"}},
		{ID: "execution-2", CreatedAt: now.Add(-30 * time.Second), Metadata: map[string]interface{}{"candidate_id": "rc-new"}},
		{ID: "execution-3", CreatedAt: now, Metadata: map[string]interface{}{"candidate_id": "rc-exec-only"}},
	}

	if got := resolveSkillCutoverCandidateID(selectorRuns, executionRuns); got != "rc-new" {
		t.Fatalf("resolveSkillCutoverCandidateID() = %q, want rc-new", got)
	}
}

func TestController_EvaluateSkillCutoverReadiness_ComputesCandidateStreaks(t *testing.T) {
	controller := newTestController(t)

	selectorItems := selectorGateSmokeItems(t)
	selectorEvalSpec := createSelectorEvalSpecForItems(t, controller, "Cutover readiness selector", selectorItems)
	executionItems := executionGateSmokeItems(t)
	executionEvalSpec := createExecutionEvalSpecForItems(t, controller, "Cutover readiness execution", executionItems)

	selectorBaselineReport := runSelectorEvalReportForCandidate(t, controller, selectorEvalSpec, "selector-baseline", "", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_search", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("exec", true, "clarify"),
		},
	}, len(selectorItems))
	selectorBaseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "cutover-selector-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   selectorBaselineReport.EvalRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline(selector) failed: %v", err)
	}

	executionBaselineReport := runExecutionEvalReportForCandidate(t, controller, executionEvalSpec, "execution-baseline", "", executionEvalDriver{
		defaultOutcome: executionEvalOutcome{Status: RunStatusCompleted, Result: "completed"},
		delay:          10 * time.Millisecond,
	}, len(executionItems))
	executionBaseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "cutover-execution-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   executionBaselineReport.EvalRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline(execution) failed: %v", err)
	}

	candidateID := "rc-2026-03-28"
	runSelectorEvalReportForCandidate(t, controller, selectorEvalSpec, "selector-candidate-1", candidateID, selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_search", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("exec", true, "clarify"),
		},
	}, len(selectorItems))
	runExecutionEvalReportForCandidate(t, controller, executionEvalSpec, "execution-candidate-1", candidateID, executionEvalDriver{
		defaultOutcome: executionEvalOutcome{Status: RunStatusCompleted, Result: "completed"},
		delay:          10 * time.Millisecond,
	}, len(executionItems))
	runSelectorEvalReportForCandidate(t, controller, selectorEvalSpec, "selector-candidate-2", candidateID, selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_search", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("exec", true, "clarify"),
		},
	}, len(selectorItems))
	runExecutionEvalReportForCandidate(t, controller, executionEvalSpec, "execution-candidate-2", candidateID, executionEvalDriver{
		defaultOutcome: executionEvalOutcome{Status: RunStatusCompleted, Result: "completed"},
		delay:          10 * time.Millisecond,
	}, len(executionItems))

	report, err := controller.EvaluateSkillCutoverReadiness(context.Background(), SkillCutoverReadinessRequest{
		OwnerUserID:             "user-1",
		CandidateID:             candidateID,
		SelectorEvalSpecID:      selectorEvalSpec.ID,
		ExecutionEvalSpecID:     executionEvalSpec.ID,
		RequiredConsecutiveRuns: 2,
		MaxAssessments:          3,
		Selector: SelectorGateRequest{
			BaselineID: selectorBaseline.ID,
		},
		Execution: ExecutionEquivalenceRequest{
			BaselineID: executionBaseline.ID,
		},
	})
	if err != nil {
		t.Fatalf("EvaluateSkillCutoverReadiness failed: %v", err)
	}

	if report.CandidateID != candidateID {
		t.Fatalf("candidate_id = %q, want %q", report.CandidateID, candidateID)
	}
	if !report.Selector.Ready {
		t.Fatalf("selector readiness = %#v, want ready", report.Selector)
	}
	if !report.Execution.Ready {
		t.Fatalf("execution readiness = %#v, want ready", report.Execution)
	}
	if report.Budget.Ready {
		t.Fatalf("budget readiness = %#v, want false while native tool surface is not exec-only", report.Budget)
	}
	if report.Selector.ConsecutivePassCount != 2 {
		t.Fatalf("selector consecutive_pass_count = %d, want 2", report.Selector.ConsecutivePassCount)
	}
	if report.Execution.ConsecutivePassCount != 2 {
		t.Fatalf("execution consecutive_pass_count = %d, want 2", report.Execution.ConsecutivePassCount)
	}
	if report.Budget.ConsecutivePassCount != 0 {
		t.Fatalf("budget consecutive_pass_count = %d, want 0", report.Budget.ConsecutivePassCount)
	}
	if report.EvaluatedGatesReady {
		t.Fatalf("evaluated_gates_ready = %#v, want false until budget gate passes", report.EvaluatedGatesReady)
	}
	if report.Ready {
		t.Fatalf("ready = %#v, want false while budget gate is red", report.Ready)
	}
	if len(report.UnverifiedRequirements) != 0 {
		t.Fatalf("unverified_requirements = %#v, want empty after budget automation", report.UnverifiedRequirements)
	}
	if len(report.BlockingReasons) == 0 {
		t.Fatalf("blocking_reasons = %#v, want budget blocking reason", report.BlockingReasons)
	}
}

func TestController_EvaluateSkillCutoverReadiness_FailsWhenLatestLaneRunRegresses(t *testing.T) {
	controller := newTestController(t)

	selectorItems := selectorGateSmokeItems(t)
	selectorEvalSpec := createSelectorEvalSpecForItems(t, controller, "Cutover readiness selector regression", selectorItems)
	executionItems := executionGateSmokeItems(t)
	executionEvalSpec := createExecutionEvalSpecForItems(t, controller, "Cutover readiness execution regression", executionItems)

	selectorBaselineReport := runSelectorEvalReportForCandidate(t, controller, selectorEvalSpec, "selector-baseline", "", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_search", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("exec", true, "clarify"),
		},
	}, len(selectorItems))
	selectorBaseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "cutover-selector-regression-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   selectorBaselineReport.EvalRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline(selector) failed: %v", err)
	}

	executionBaselineReport := runExecutionEvalReportForCandidate(t, controller, executionEvalSpec, "execution-baseline", "", executionEvalDriver{
		defaultOutcome: executionEvalOutcome{Status: RunStatusCompleted, Result: "completed"},
		delay:          10 * time.Millisecond,
	}, len(executionItems))
	executionBaseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "cutover-execution-regression-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   executionBaselineReport.EvalRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline(execution) failed: %v", err)
	}

	candidateID := "rc-regression"
	runSelectorEvalReportForCandidate(t, controller, selectorEvalSpec, "selector-candidate-pass", candidateID, selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_search", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("exec", true, "clarify"),
		},
	}, len(selectorItems))
	runExecutionEvalReportForCandidate(t, controller, executionEvalSpec, "execution-candidate-pass", candidateID, executionEvalDriver{
		defaultOutcome: executionEvalOutcome{Status: RunStatusCompleted, Result: "completed"},
		delay:          10 * time.Millisecond,
	}, len(executionItems))
	// timeutil.NowTime() is cached in 100ms buckets, so separate the latest
	// regression run from the prior pass to keep readiness ordering stable.
	time.Sleep(150 * time.Millisecond)
	runExecutionEvalReportForCandidate(t, controller, executionEvalSpec, "execution-candidate-regression", candidateID, executionEvalDriver{
		defaultOutcome: executionEvalOutcome{
			Status: RunStatusCompleted,
			Result: "completed",
			Events: []executionEventSpec{
				{Type: "tool_call", ToolName: "reminder"},
			},
		},
		delay: 10 * time.Millisecond,
	}, len(executionItems))

	report, err := controller.EvaluateSkillCutoverReadiness(context.Background(), SkillCutoverReadinessRequest{
		OwnerUserID:             "user-1",
		CandidateID:             candidateID,
		SelectorEvalSpecID:      selectorEvalSpec.ID,
		ExecutionEvalSpecID:     executionEvalSpec.ID,
		RequiredConsecutiveRuns: 2,
		Selector: SelectorGateRequest{
			BaselineID: selectorBaseline.ID,
		},
		Execution: ExecutionEquivalenceRequest{
			BaselineID: executionBaseline.ID,
		},
	})
	if err != nil {
		t.Fatalf("EvaluateSkillCutoverReadiness failed: %v", err)
	}

	if report.EvaluatedGatesReady {
		t.Fatalf("evaluated_gates_ready = %#v, want false", report.EvaluatedGatesReady)
	}
	if report.Execution.ConsecutivePassCount != 0 {
		t.Fatalf("execution consecutive_pass_count = %d, want 0 after latest regression; assessments=%#v", report.Execution.ConsecutivePassCount, report.Execution.Assessments)
	}
	if report.Execution.Ready {
		t.Fatalf("execution readiness = %#v, want false", report.Execution)
	}
	if len(report.BlockingReasons) == 0 {
		t.Fatalf("blocking_reasons = %#v, want at least one blocking reason", report.BlockingReasons)
	}
}

func TestController_EvaluateSkillCutoverReadiness_PassesWhenBudgetLaneIsExecOnly(t *testing.T) {
	controller := newTestController(t)

	selectorItems := selectorGateSmokeItems(t)
	selectorEvalSpec := createSelectorEvalSpecForItems(t, controller, "Cutover readiness selector exec-only", selectorItems)
	executionItems := executionGateSmokeItems(t)
	executionEvalSpec := createExecutionEvalSpecForItems(t, controller, "Cutover readiness execution exec-only", executionItems)

	selectorBaselineReport := runSelectorEvalReportForCandidate(t, controller, selectorEvalSpec, "selector-baseline", "", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_search", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("exec", true, "clarify"),
		},
	}, len(selectorItems))
	selectorBaseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "cutover-selector-exec-only-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   selectorBaselineReport.EvalRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline(selector) failed: %v", err)
	}

	executionBaselineReport := runExecutionEvalReportForCandidate(t, controller, executionEvalSpec, "execution-baseline", "", executionEvalDriver{
		defaultOutcome: executionEvalOutcome{Status: RunStatusCompleted, Result: "completed"},
		delay:          10 * time.Millisecond,
	}, len(executionItems))
	executionBaseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "cutover-execution-exec-only-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   executionBaselineReport.EvalRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline(execution) failed: %v", err)
	}

	budgetBaselineReport := runSelectorEvalReportForCandidate(t, controller, selectorEvalSpec, "budget-baseline", "", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_search", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("analyze", true, "clarify"),
		},
	}, len(selectorItems))
	budgetBaseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "cutover-budget-exec-only-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   budgetBaselineReport.EvalRun.ID,
		IsDefault:   false,
	})
	if err != nil {
		t.Fatalf("CreateBaseline(budget) failed: %v", err)
	}

	candidateID := "rc-exec-only"
	execOnlySource := selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponseWithTools("web_search", []string{"exec"}, false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponseWithTools("exec", []string{"exec"}, true, "clarify"),
		},
	}
	runSelectorEvalReportForCandidate(t, controller, selectorEvalSpec, "selector-candidate-1", candidateID, execOnlySource, len(selectorItems))
	runExecutionEvalReportForCandidate(t, controller, executionEvalSpec, "execution-candidate-1", candidateID, executionEvalDriver{
		defaultOutcome: executionEvalOutcome{Status: RunStatusCompleted, Result: "completed"},
		delay:          10 * time.Millisecond,
	}, len(executionItems))
	runSelectorEvalReportForCandidate(t, controller, selectorEvalSpec, "selector-candidate-2", candidateID, execOnlySource, len(selectorItems))
	runExecutionEvalReportForCandidate(t, controller, executionEvalSpec, "execution-candidate-2", candidateID, executionEvalDriver{
		defaultOutcome: executionEvalOutcome{Status: RunStatusCompleted, Result: "completed"},
		delay:          10 * time.Millisecond,
	}, len(executionItems))

	report, err := controller.EvaluateSkillCutoverReadiness(context.Background(), SkillCutoverReadinessRequest{
		OwnerUserID:             "user-1",
		CandidateID:             candidateID,
		SelectorEvalSpecID:      selectorEvalSpec.ID,
		ExecutionEvalSpecID:     executionEvalSpec.ID,
		RequiredConsecutiveRuns: 2,
		Selector: SelectorGateRequest{
			BaselineID: selectorBaseline.ID,
		},
		Execution: ExecutionEquivalenceRequest{
			BaselineID: executionBaseline.ID,
		},
		Budget: SkillCutoverBudgetRequest{
			BaselineID: budgetBaseline.ID,
		},
	})
	if err != nil {
		t.Fatalf("EvaluateSkillCutoverReadiness failed: %v", err)
	}

	if !report.Selector.Ready || !report.Execution.Ready || !report.Budget.Ready {
		t.Fatalf("expected all lanes ready, got selector=%#v execution=%#v budget=%#v", report.Selector, report.Execution, report.Budget)
	}
	if !report.EvaluatedGatesReady {
		t.Fatalf("evaluated_gates_ready = %#v, want true", report.EvaluatedGatesReady)
	}
	if !report.Ready {
		t.Fatalf("ready = %#v, want true", report.Ready)
	}
	if len(report.BlockingReasons) != 0 {
		t.Fatalf("blocking_reasons = %#v, want none", report.BlockingReasons)
	}
}

func runSelectorEvalReportForCandidate(t *testing.T, controller *Controller, evalSpec *EvalSpec, title string, candidateID string, source selectorEvalSource, wantScorecards int) *EvalRunReport {
	t.Helper()
	controller.RegisterDriver(selectorEvalDriver{source: source})

	evalRun, err := controller.SubmitEvalRun(context.Background(), EvalRunSpec{
		EvalSpecID:  evalSpec.ID,
		OwnerUserID: "user-1",
		Title:       title,
		Metadata: map[string]interface{}{
			"candidate_id": candidateID,
		},
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
	t.Fatalf("selector eval report = %#v, want completed with %d scorecards", report, wantScorecards)
	return nil
}

func runExecutionEvalReportForCandidate(t *testing.T, controller *Controller, evalSpec *EvalSpec, title string, candidateID string, driver executionEvalDriver, wantScorecards int) *EvalRunReport {
	t.Helper()
	controller.RegisterDriver(driver)

	evalRun, err := controller.SubmitEvalRun(context.Background(), EvalRunSpec{
		EvalSpecID:  evalSpec.ID,
		OwnerUserID: "user-1",
		Title:       title,
		Metadata: map[string]interface{}{
			"candidate_id": candidateID,
		},
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
	t.Fatalf("execution eval report = %#v, want terminal with %d scorecards", report, wantScorecards)
	return nil
}
