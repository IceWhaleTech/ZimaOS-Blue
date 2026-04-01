package harness

import (
	"context"
	"testing"
)

func TestController_EnsureSelectorCuratedAssets_ReusesBuiltins(t *testing.T) {
	controller := newTestController(t)

	first, err := controller.EnsureSelectorCuratedAssets(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("EnsureSelectorCuratedAssets(first) failed: %v", err)
	}
	second, err := controller.EnsureSelectorCuratedAssets(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("EnsureSelectorCuratedAssets(second) failed: %v", err)
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

	versions, err := controller.ListDatasetVersions(context.Background(), first.Dataset.ID, 20)
	if err != nil {
		t.Fatalf("ListDatasetVersions failed: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("dataset versions len = %d, want 1", len(versions))
	}
	specs, err := controller.ListEvalSpecs(context.Background(), EvalSpecFilter{
		OwnerUserID: "user-1",
		DatasetID:   first.Dataset.ID,
		Limit:       20,
	})
	if err != nil {
		t.Fatalf("ListEvalSpecs failed: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("eval specs len = %d, want 1", len(specs))
	}
}

func TestController_EvaluateSelectorGate_PassesWithOptionalComparisonThresholds(t *testing.T) {
	controller := newTestController(t)
	items := selectorGateSmokeItems(t)
	evalSpec := createSelectorEvalSpecForItems(t, controller, "Selector gate pass", items)

	baselineReport := runSelectorEvalReport(t, controller, evalSpec, "selector-gate-baseline", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_search", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("exec", true, "clarify"),
		},
	}, len(items))
	baseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "selector-gate-pass-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   baselineReport.EvalRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline failed: %v", err)
	}

	candidateReport := runSelectorEvalReport(t, controller, evalSpec, "selector-gate-candidate", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_search", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("exec", true, "clarify"),
		},
	}, len(items))

	minRouteAgreement := 1.0
	maxClarifyDelta := 0.01
	maxCriticalRegression := 0
	report, err := controller.EvaluateSelectorGate(context.Background(), candidateReport.EvalRun.ID, SelectorGateRequest{
		BaselineID: baseline.ID,
		Thresholds: SelectorGateThresholds{
			MinRouteAgreementRate:      &minRouteAgreement,
			MaxClarifyRateDelta:        &maxClarifyDelta,
			MaxCriticalRegressionCount: &maxCriticalRegression,
		},
	})
	if err != nil {
		t.Fatalf("EvaluateSelectorGate failed: %v", err)
	}
	if !report.Passed {
		t.Fatalf("selector gate report = %#v, want pass", report)
	}
	if report.Metrics.PassRate != 1 {
		t.Fatalf("metrics.pass_rate = %#v, want 1", report.Metrics.PassRate)
	}
	if report.Metrics.RouteAgreementRate != 1 {
		t.Fatalf("metrics.route_agreement_rate = %#v, want 1", report.Metrics.RouteAgreementRate)
	}
	if report.Metrics.CriticalRegressionCount != 0 {
		t.Fatalf("metrics.critical_regression_count = %#v, want 0", report.Metrics.CriticalRegressionCount)
	}
	if got := report.Metrics.LocaleBreakdown["zh-CN"].RouteCompatibleRate; got != 1 {
		t.Fatalf("metrics.locale_breakdown[zh-CN].route_compatible_rate = %#v, want 1", got)
	}
	if got := report.Metrics.PrimaryRouteBreakdown["exec"].RouteCompatibleRate; got != 1 {
		t.Fatalf("metrics.primary_route_breakdown[exec].route_compatible_rate = %#v, want 1", got)
	}
	if got := report.Metrics.SelectedCanonicalSkillBreakdown[harnessCanonicalWebQuerySkill]; got != 1 {
		t.Fatalf("metrics.selected_canonical_skill_breakdown[web_query] = %#v, want 1", got)
	}
	if got := report.Metrics.SelectedCanonicalSkillBreakdown["exec"]; got != 1 {
		t.Fatalf("metrics.selected_canonical_skill_breakdown[exec] = %#v, want 1", got)
	}
	if got := report.Metrics.NativeSurfaceModeBreakdown["legacy"]; got != 1 {
		t.Fatalf("metrics.native_surface_mode_breakdown[legacy] = %#v, want 1", got)
	}
	if got := report.Metrics.NativeSurfaceModeBreakdown["clarify_none"]; got != 1 {
		t.Fatalf("metrics.native_surface_mode_breakdown[clarify_none] = %#v, want 1", got)
	}
	if got := report.Metrics.NativeSurfaceReasonBreakdown["legacy_native_surface"]; got != 1 {
		t.Fatalf("metrics.native_surface_reason_breakdown[legacy_native_surface] = %#v, want 1", got)
	}
	if got := report.Metrics.NativeSurfaceReasonBreakdown["clarify_required"]; got != 1 {
		t.Fatalf("metrics.native_surface_reason_breakdown[clarify_required] = %#v, want 1", got)
	}
	if got := report.Metrics.ExecutionProfileBreakdown["prefer_fork"]; got != 1 {
		t.Fatalf("metrics.execution_profile_breakdown[prefer_fork] = %#v, want 1", got)
	}
	if got := report.Metrics.ExecutionProfileBreakdown["inline"]; got != 1 {
		t.Fatalf("metrics.execution_profile_breakdown[inline] = %#v, want 1", got)
	}
	if check := selectorGateCheckByName(t, report.Checks, "route_agreement_rate"); !check.Passed {
		t.Fatalf("route_agreement_rate check = %#v, want pass", check)
	}
	if check := selectorGateCheckByName(t, report.Checks, "clarify_rate_delta"); !check.Passed {
		t.Fatalf("clarify_rate_delta check = %#v, want pass", check)
	}
}

func TestController_EvaluateSelectorGate_FailsCriticalRegressionWhenEnabled(t *testing.T) {
	controller := newTestController(t)
	items := selectorGateSmokeItems(t)
	evalSpec := createSelectorEvalSpecForItems(t, controller, "Selector gate critical regression", items)

	baselineReport := runSelectorEvalReport(t, controller, evalSpec, "selector-gate-baseline", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_search", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("exec", true, "clarify"),
		},
	}, len(items))
	baseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "selector-gate-critical-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   baselineReport.EvalRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline failed: %v", err)
	}

	candidateReport := runSelectorEvalReport(t, controller, evalSpec, "selector-gate-candidate", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_search", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("web_search", false, "selected"),
		},
	}, len(items))

	maxCriticalRegression := 0
	report, err := controller.EvaluateSelectorGate(context.Background(), candidateReport.EvalRun.ID, SelectorGateRequest{
		BaselineID: baseline.ID,
		Thresholds: SelectorGateThresholds{
			MaxCriticalRegressionCount: &maxCriticalRegression,
		},
	})
	if err != nil {
		t.Fatalf("EvaluateSelectorGate failed: %v", err)
	}
	if report.Passed {
		t.Fatalf("selector gate report = %#v, want failure", report)
	}
	if report.Metrics.CriticalRegressionCount != 1 {
		t.Fatalf("metrics.critical_regression_count = %#v, want 1", report.Metrics.CriticalRegressionCount)
	}
	if got := report.Metrics.LocaleBreakdown["zh-CN"].CriticalRegressionCount; got != 1 {
		t.Fatalf("metrics.locale_breakdown[zh-CN].critical_regression_count = %#v, want 1", got)
	}
	if got := report.Metrics.PrimaryRouteBreakdown["exec"].CriticalRegressionCount; got != 1 {
		t.Fatalf("metrics.primary_route_breakdown[exec].critical_regression_count = %#v, want 1", got)
	}
	if check := selectorGateCheckByName(t, report.Checks, "critical_regression_count"); check.Passed {
		t.Fatalf("critical_regression_count check = %#v, want fail", check)
	}
	if check := selectorGateCheckByName(t, report.Checks, "critical_pass_rate"); check.Passed {
		t.Fatalf("critical_pass_rate check = %#v, want fail", check)
	}
}

func TestController_EvaluateSelectorGate_FailsClarifyDeltaWhenEnabled(t *testing.T) {
	controller := newTestController(t)
	items := selectorGateSmokeItems(t)
	evalSpec := createSelectorEvalSpecForItems(t, controller, "Selector gate clarify delta", items)

	baselineReport := runSelectorEvalReport(t, controller, evalSpec, "selector-gate-baseline", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_search", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("exec", true, "clarify"),
		},
	}, len(items))
	baseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "selector-gate-clarify-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   baselineReport.EvalRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline failed: %v", err)
	}

	candidateReport := runSelectorEvalReport(t, controller, evalSpec, "selector-gate-candidate", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_search", true, "clarify"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("exec", true, "clarify"),
		},
	}, len(items))

	maxClarifyDelta := 0.01
	report, err := controller.EvaluateSelectorGate(context.Background(), candidateReport.EvalRun.ID, SelectorGateRequest{
		BaselineID: baseline.ID,
		Thresholds: SelectorGateThresholds{
			MaxClarifyRateDelta: &maxClarifyDelta,
		},
	})
	if err != nil {
		t.Fatalf("EvaluateSelectorGate failed: %v", err)
	}
	if report.Passed {
		t.Fatalf("selector gate report = %#v, want failure", report)
	}
	if report.Metrics.ClarifyRateDelta != 0.5 {
		t.Fatalf("metrics.clarify_rate_delta = %#v, want 0.5", report.Metrics.ClarifyRateDelta)
	}
	if check := selectorGateCheckByName(t, report.Checks, "clarify_rate_delta"); check.Passed {
		t.Fatalf("clarify_rate_delta check = %#v, want fail", check)
	}
}

func TestController_EvaluateSelectorGate_DefaultsFavorTruthOverBaselineAgreement(t *testing.T) {
	controller := newTestController(t)
	items := selectorGateSmokeItems(t)
	evalSpec := createSelectorEvalSpecForItems(t, controller, "Selector gate defaults", items)

	baselineReport := runSelectorEvalReport(t, controller, evalSpec, "selector-gate-baseline", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_search", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("web_search", false, "selected"),
		},
	}, len(items))
	baseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "selector-gate-defaults-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   baselineReport.EvalRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline failed: %v", err)
	}

	candidateReport := runSelectorEvalReport(t, controller, evalSpec, "selector-gate-candidate", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_search", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("exec", true, "clarify"),
		},
	}, len(items))

	report, err := controller.EvaluateSelectorGate(context.Background(), candidateReport.EvalRun.ID, SelectorGateRequest{
		BaselineID: baseline.ID,
	})
	if err != nil {
		t.Fatalf("EvaluateSelectorGate failed: %v", err)
	}
	if !report.Passed {
		t.Fatalf("selector gate report = %#v, want pass", report)
	}
	if report.Metrics.RouteAgreementRate != 0.5 {
		t.Fatalf("metrics.route_agreement_rate = %#v, want 0.5", report.Metrics.RouteAgreementRate)
	}
	if report.Metrics.RouteCompatibleRate != 1 {
		t.Fatalf("metrics.route_compatible_rate = %#v, want 1", report.Metrics.RouteCompatibleRate)
	}
	if report.Metrics.ClarifyRateDelta != 0.5 {
		t.Fatalf("metrics.clarify_rate_delta = %#v, want 0.5", report.Metrics.ClarifyRateDelta)
	}
	if selectorGateHasCheck(report.Checks, "route_agreement_rate") {
		t.Fatalf("checks = %#v, want route_agreement_rate omitted by default", report.Checks)
	}
	if selectorGateHasCheck(report.Checks, "clarify_rate_delta") {
		t.Fatalf("checks = %#v, want clarify_rate_delta omitted by default", report.Checks)
	}
}

func TestController_EvaluateSelectorGate_PassesRouteCompatibleThresholdForTruthImprovement(t *testing.T) {
	controller := newTestController(t)
	items := selectorGateSmokeItems(t)
	evalSpec := createSelectorEvalSpecForItems(t, controller, "Selector gate route compatibility", items)

	baselineReport := runSelectorEvalReport(t, controller, evalSpec, "selector-gate-baseline", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_search", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("web_search", false, "selected"),
		},
	}, len(items))
	baseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "selector-gate-route-compatible-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   baselineReport.EvalRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline failed: %v", err)
	}

	candidateReport := runSelectorEvalReport(t, controller, evalSpec, "selector-gate-candidate", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_search", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("exec", true, "clarify"),
		},
	}, len(items))

	minRouteCompatible := 1.0
	report, err := controller.EvaluateSelectorGate(context.Background(), candidateReport.EvalRun.ID, SelectorGateRequest{
		BaselineID: baseline.ID,
		Thresholds: SelectorGateThresholds{
			MinRouteCompatibleRate: &minRouteCompatible,
		},
	})
	if err != nil {
		t.Fatalf("EvaluateSelectorGate failed: %v", err)
	}
	if !report.Passed {
		t.Fatalf("selector gate report = %#v, want pass", report)
	}
	if report.Metrics.RouteAgreementRate != 0.5 {
		t.Fatalf("metrics.route_agreement_rate = %#v, want 0.5", report.Metrics.RouteAgreementRate)
	}
	if report.Metrics.RouteCompatibleRate != 1 {
		t.Fatalf("metrics.route_compatible_rate = %#v, want 1", report.Metrics.RouteCompatibleRate)
	}
	if report.Metrics.RouteImprovementCount != 1 {
		t.Fatalf("metrics.route_improvement_count = %#v, want 1", report.Metrics.RouteImprovementCount)
	}
	if got := report.Metrics.LocaleBreakdown["zh-CN"].RouteImprovementCount; got != 1 {
		t.Fatalf("metrics.locale_breakdown[zh-CN].route_improvement_count = %#v, want 1", got)
	}
	if got := report.Metrics.PrimaryRouteBreakdown["exec"].RouteCompatibleRate; got != 1 {
		t.Fatalf("metrics.primary_route_breakdown[exec].route_compatible_rate = %#v, want 1", got)
	}
	if check := selectorGateCheckByName(t, report.Checks, "route_compatible_rate"); !check.Passed {
		t.Fatalf("route_compatible_rate check = %#v, want pass", check)
	}
}

func selectorGateSmokeItems(t *testing.T) []DatasetManifestItem {
	t.Helper()
	fullManifest := SelectorCuratedDatasetManifest()
	return []DatasetManifestItem{
		findSelectorManifestItem(t, fullManifest.Items, "selected-web_search-en-us"),
		findSelectorManifestItem(t, fullManifest.Items, "clarify-mixed-local-web-zh-cn"),
	}
}

func selectorGateCheckByName(t *testing.T, checks []SelectorGateCheck, name string) SelectorGateCheck {
	t.Helper()
	for _, check := range checks {
		if check.Name == name {
			return check
		}
	}
	t.Fatalf("selector gate check %q not found in %#v", name, checks)
	return SelectorGateCheck{}
}

func selectorGateHasCheck(checks []SelectorGateCheck, name string) bool {
	for _, check := range checks {
		if check.Name == name {
			return true
		}
	}
	return false
}
