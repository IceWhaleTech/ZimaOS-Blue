package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

type selectorEvalSource struct {
	responses map[string]map[string]interface{}
}

const harnessCanonicalWebQuerySkill = "web_query"

func canonicalHarnessRouteName(route string) string {
	route = strings.TrimSpace(route)
	if route == "web_search" {
		return harnessCanonicalWebQuerySkill
	}
	return route
}

func canonicalHarnessRouteNames(routes []string) []string {
	out := make([]string, 0, len(routes))
	for _, route := range routes {
		if route = canonicalHarnessRouteName(route); route != "" {
			out = append(out, route)
		}
	}
	return out
}

func canonicalHarnessDatasetCaseID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	return strings.ReplaceAll(id, "web_search", harnessCanonicalWebQuerySkill)
}

func (s selectorEvalSource) PreviewSelectorDryRun(_ context.Context, query string, model string) (map[string]interface{}, error) {
	response := cloneMetadataMap(s.responses[query])
	if response == nil {
		response = map[string]interface{}{}
	}
	response["query"] = query
	response["model"] = model
	return response, nil
}

type selectorEvalDriver struct {
	source selectorEvalSource
}

func (d selectorEvalDriver) Kind() RunKind { return RunKindAgentTask }

func (d selectorEvalDriver) Validate(spec RunSpec) error {
	if query := selectorEvalQuery(spec.Goal, spec.Metadata); query == "" {
		return fmt.Errorf("selector eval query is required")
	}
	return nil
}

func (d selectorEvalDriver) Start(ctx context.Context, run *Run, env RunEnv) error {
	if run == nil {
		return fmt.Errorf("run is required")
	}
	response, err := d.source.PreviewSelectorDryRun(ctx, selectorEvalQuery(run.Goal, run.Metadata), strings.TrimSpace(run.Model))
	if err != nil {
		return err
	}
	raw, err := json.Marshal(response)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	snapshot := *run
	snapshot.Status = RunStatusCompleted
	snapshot.Result = string(raw)
	snapshot.UpdatedAt = now
	snapshot.FinishedAt = &now
	return env.Manager.SyncSnapshot(ctx, &snapshot)
}

func (d selectorEvalDriver) Cancel(context.Context, *Run) error { return nil }

func TestController_SubmitEvalRunSelectorCuratedDataset_PassesStructuredRoutingChecks(t *testing.T) {
	controller := newTestController(t)
	controller.RegisterDriver(selectorEvalDriver{source: selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_search", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("exec", true, "clarify"),
		},
	}})

	fullManifest := SelectorCuratedDatasetManifest()
	manifest := DatasetManifest{
		Dataset:  fullManifest.Dataset,
		Defaults: fullManifest.Defaults,
		Items: []DatasetManifestItem{
			findSelectorManifestItem(t, fullManifest.Items, "selected-web_search-en-us"),
			findSelectorManifestItem(t, fullManifest.Items, "clarify-mixed-local-web-zh-cn"),
		},
	}
	manifestRaw, err := datasetManifestMap(manifest)
	if err != nil {
		t.Fatalf("datasetManifestMap failed: %v", err)
	}

	dataset, err := controller.CreateDataset(context.Background(), DatasetSpec{
		Name:           "Selector smoke",
		OwnerUserID:    "user-1",
		Subject:        SelectorCuratedDatasetSubject,
		DefaultRunKind: RunKindAgentTask,
		DefaultProfile: selectorCuratedProfile,
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
		Name:             "Selector smoke eval",
		OwnerUserID:      "user-1",
		DatasetID:        dataset.ID,
		DatasetVersionID: version.ID,
		RunKind:          RunKindAgentTask,
		Profile:          selectorCuratedProfile,
		RuntimePolicy:    selectorDryRunRuntimePolicy(),
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
		Title:       "selector-smoke-run",
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
		if err == nil &&
			report != nil &&
			report.EvalRun != nil &&
			report.GroupReport != nil &&
			report.GroupReport.Group != nil &&
			report.EvalRun.Status == RunGroupStatusCompleted &&
			report.GroupReport.Group.Status == RunGroupStatusCompleted &&
			len(report.GroupReport.Scorecards) == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	report, err := controller.GetEvalRunReport(context.Background(), evalRun.ID)
	if err != nil {
		t.Fatalf("GetEvalRunReport failed: %v", err)
	}
	if report.EvalRun == nil || report.EvalRun.Status != RunGroupStatusCompleted {
		t.Fatalf("eval run = %#v, want completed", report.EvalRun)
	}
	if report.GroupReport == nil || len(report.GroupReport.Scorecards) != 2 {
		t.Fatalf("group report = %#v, want 2 scorecards", report.GroupReport)
	}
	for _, card := range report.GroupReport.Scorecards {
		if card.Verdict != ScoreVerdictPass {
			t.Fatalf("scorecard verdict = %s, want pass breakdown=%s", card.Verdict, card.BreakdownJSON)
		}
	}
}

func TestController_CompareEvalRunSelectorCuratedDatasetTracksRouteAgreementAndCriticalRegressions(t *testing.T) {
	controller := newTestController(t)
	fullManifest := SelectorCuratedDatasetManifest()
	items := []DatasetManifestItem{
		findSelectorManifestItem(t, fullManifest.Items, "selected-web_search-en-us"),
		findSelectorManifestItem(t, fullManifest.Items, "clarify-mixed-local-web-zh-cn"),
	}
	evalSpec := createSelectorEvalSpecForItems(t, controller, "Selector compare", items)

	baselineReport := runSelectorEvalReport(t, controller, evalSpec, "selector-baseline", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_search", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("exec", true, "clarify"),
		},
	}, len(items))

	baseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "selector-routing-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   baselineReport.EvalRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline failed: %v", err)
	}

	candidateReport := runSelectorEvalReport(t, controller, evalSpec, "selector-candidate", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_search", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("web_search", false, "selected"),
		},
	}, len(items))

	comparison, err := controller.CompareEvalRun(context.Background(), candidateReport.EvalRun.ID, CompareEvalRunRequest{
		BaselineID: baseline.ID,
	})
	if err != nil {
		t.Fatalf("CompareEvalRun failed: %v", err)
	}
	if got := comparison.Summary["route_case_count"]; got != 2 {
		t.Fatalf("summary route_case_count = %#v, want 2", got)
	}
	if got := comparison.Summary["route_agreement_count"]; got != 1 {
		t.Fatalf("summary route_agreement_count = %#v, want 1", got)
	}
	if got := comparison.Summary["route_agreement_rate"]; got != 0.5 {
		t.Fatalf("summary route_agreement_rate = %#v, want 0.5", got)
	}
	if got := comparison.Summary["route_compatible_count"]; got != 1 {
		t.Fatalf("summary route_compatible_count = %#v, want 1", got)
	}
	if got := comparison.Summary["route_compatible_rate"]; got != 0.5 {
		t.Fatalf("summary route_compatible_rate = %#v, want 0.5", got)
	}
	if got := comparison.Summary["route_improvement_count"]; got != 0 {
		t.Fatalf("summary route_improvement_count = %#v, want 0", got)
	}
	if got := comparison.Summary["critical_route_case_count"]; got != 1 {
		t.Fatalf("summary critical_route_case_count = %#v, want 1", got)
	}
	if got := comparison.Summary["critical_regression_count"]; got != 1 {
		t.Fatalf("summary critical_regression_count = %#v, want 1", got)
	}
	localeBreakdown, ok := comparison.Summary["locale_breakdown"].(map[string]SelectorGateSegmentMetrics)
	if !ok {
		t.Fatalf("summary locale_breakdown = %#v, want typed breakdown map", comparison.Summary["locale_breakdown"])
	}
	if got := localeBreakdown["en-US"].RouteCompatibleRate; got != 1.0 {
		t.Fatalf("locale_breakdown[en-US].route_compatible_rate = %#v, want 1", got)
	}
	if got := localeBreakdown["zh-CN"].CriticalRegressionCount; got != 1 {
		t.Fatalf("locale_breakdown[zh-CN].critical_regression_count = %#v, want 1", got)
	}
	routeBreakdown, ok := comparison.Summary["primary_route_breakdown"].(map[string]SelectorGateSegmentMetrics)
	if !ok {
		t.Fatalf("summary primary_route_breakdown = %#v, want typed breakdown map", comparison.Summary["primary_route_breakdown"])
	}
	if got := routeBreakdown["exec"].RouteCompatibleRate; got != 0.0 {
		t.Fatalf("primary_route_breakdown[exec].route_compatible_rate = %#v, want 0", got)
	}
	if got := routeBreakdown["exec"].CriticalRegressionCount; got != 1 {
		t.Fatalf("primary_route_breakdown[exec].critical_regression_count = %#v, want 1", got)
	}
	if got := comparison.Summary["base_clarify_rate"]; got != 0.5 {
		t.Fatalf("summary base_clarify_rate = %#v, want 0.5", got)
	}
	if got := comparison.Summary["target_clarify_rate"]; got != 0.0 {
		t.Fatalf("summary target_clarify_rate = %#v, want 0", got)
	}
	if got := comparison.Summary["clarify_rate_delta"]; got != -0.5 {
		t.Fatalf("summary clarify_rate_delta = %#v, want -0.5", got)
	}
	if got := comparison.Summary["regression_count"]; got != 1 {
		t.Fatalf("summary regression_count = %#v, want 1", got)
	}
	if len(comparison.Regressions) != 1 || comparison.Regressions[0].Key != "clarify-mixed-local-web-zh-cn" {
		t.Fatalf("comparison regressions = %#v, want clarify-mixed-local-web-zh-cn", comparison.Regressions)
	}
	if got := comparison.ScorerDelta["route_agreement_rate"]; got != 0.5 {
		t.Fatalf("scorer_delta route_agreement_rate = %#v, want 0.5", got)
	}
	if got := comparison.ScorerDelta["route_compatible_rate"]; got != 0.5 {
		t.Fatalf("scorer_delta route_compatible_rate = %#v, want 0.5", got)
	}
}

func TestController_CompareEvalRunSelectorCuratedDatasetTracksRouteCompatibilityForTruthImprovements(t *testing.T) {
	controller := newTestController(t)
	fullManifest := SelectorCuratedDatasetManifest()
	items := []DatasetManifestItem{
		findSelectorManifestItem(t, fullManifest.Items, "selected-web_search-en-us"),
		findSelectorManifestItem(t, fullManifest.Items, "clarify-mixed-local-web-zh-cn"),
	}
	evalSpec := createSelectorEvalSpecForItems(t, controller, "Selector compare compatibility", items)

	baselineReport := runSelectorEvalReport(t, controller, evalSpec, "selector-baseline", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_search", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("web_search", false, "selected"),
		},
	}, len(items))
	baseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "selector-routing-compatibility-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   baselineReport.EvalRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline failed: %v", err)
	}

	candidateReport := runSelectorEvalReport(t, controller, evalSpec, "selector-candidate", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_search", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("exec", true, "clarify"),
		},
	}, len(items))

	comparison, err := controller.CompareEvalRun(context.Background(), candidateReport.EvalRun.ID, CompareEvalRunRequest{
		BaselineID: baseline.ID,
	})
	if err != nil {
		t.Fatalf("CompareEvalRun failed: %v", err)
	}
	if got := comparison.Summary["route_agreement_rate"]; got != 0.5 {
		t.Fatalf("summary route_agreement_rate = %#v, want 0.5", got)
	}
	if got := comparison.Summary["route_compatible_count"]; got != 2 {
		t.Fatalf("summary route_compatible_count = %#v, want 2", got)
	}
	if got := comparison.Summary["route_compatible_rate"]; got != 1.0 {
		t.Fatalf("summary route_compatible_rate = %#v, want 1", got)
	}
	if got := comparison.Summary["route_improvement_count"]; got != 1 {
		t.Fatalf("summary route_improvement_count = %#v, want 1", got)
	}
	if got := comparison.Summary["critical_regression_count"]; got != 0 {
		t.Fatalf("summary critical_regression_count = %#v, want 0", got)
	}
	localeBreakdown, ok := comparison.Summary["locale_breakdown"].(map[string]SelectorGateSegmentMetrics)
	if !ok {
		t.Fatalf("summary locale_breakdown = %#v, want typed breakdown map", comparison.Summary["locale_breakdown"])
	}
	if got := localeBreakdown["zh-CN"].RouteImprovementCount; got != 1 {
		t.Fatalf("locale_breakdown[zh-CN].route_improvement_count = %#v, want 1", got)
	}
	if got := localeBreakdown["zh-CN"].RouteCompatibleRate; got != 1.0 {
		t.Fatalf("locale_breakdown[zh-CN].route_compatible_rate = %#v, want 1", got)
	}
	routeBreakdown, ok := comparison.Summary["primary_route_breakdown"].(map[string]SelectorGateSegmentMetrics)
	if !ok {
		t.Fatalf("summary primary_route_breakdown = %#v, want typed breakdown map", comparison.Summary["primary_route_breakdown"])
	}
	if got := routeBreakdown["exec"].RouteImprovementCount; got != 1 {
		t.Fatalf("primary_route_breakdown[exec].route_improvement_count = %#v, want 1", got)
	}
}

func selectorEvalResponse(skill string, clarify bool, outcome string) map[string]interface{} {
	return selectorEvalResponseWithTools(skill, []string{skill}, clarify, outcome)
}

func selectorEvalResponseWithTools(skill string, selectedTools []string, clarify bool, outcome string) map[string]interface{} {
	selectedAlias := strings.TrimSpace(skill)
	skill = canonicalHarnessRouteName(skill)
	selectedTools = canonicalHarnessRouteNames(selectedTools)
	selectedNativeTools := append([]string(nil), selectedTools...)
	nativeSurfaceMode := "legacy"
	nativeSurfaceReason := "legacy_native_surface"
	if clarify {
		selectedNativeTools = nil
		nativeSurfaceMode = "clarify_none"
		nativeSurfaceReason = "clarify_required"
	} else if len(selectedNativeTools) == 1 && selectedNativeTools[0] == "exec" {
		nativeSurfaceMode = "skill_exec"
		if skill == "exec" {
			nativeSurfaceReason = "legacy_exec_collapse_compat"
		} else {
			nativeSurfaceReason = "discover_first_cutover"
		}
	}
	executionProfile := selectorEvalExecutionProfile(skill)
	toolPayload := make([]interface{}, 0, len(selectedTools))
	for _, tool := range selectedTools {
		if tool = strings.TrimSpace(tool); tool != "" {
			toolPayload = append(toolPayload, tool)
		}
	}
	nativeToolPayload := make([]interface{}, 0, len(selectedNativeTools))
	for _, tool := range selectedNativeTools {
		if tool = strings.TrimSpace(tool); tool != "" {
			nativeToolPayload = append(nativeToolPayload, tool)
		}
	}
	response := map[string]interface{}{
		"selected_tools":                 toolPayload,
		"selected_tool_surface":          selectorEvalToolSurface(selectedTools),
		"selected_native_tools":          nativeToolPayload,
		"selected_native_tool_surface":   selectorEvalToolSurface(selectedNativeTools),
		"selected_native_surface_mode":   nativeSurfaceMode,
		"selected_native_surface_reason": nativeSurfaceReason,
		"selected_canonical_skill":       skill,
		"selected_alias":                 firstNonEmpty(selectedAlias, skill),
		"execution_profile":              executionProfile,
		"skill_exec_cutover":             nativeSurfaceMode == "skill_exec",
		"forked_skill_execution":         executionProfile == "prefer_fork" || executionProfile == "require_fork",
		"skill_decision":                 map[string]interface{}{"selected_skill": skill, "need_clarify": clarify},
		"discovery_decision": map[string]interface{}{
			"canonical_target":    skill,
			"alias_resolved":      firstNonEmpty(selectedAlias, skill),
			"need_clarify":        clarify,
			"execution_profile":   executionProfile,
			"native_surface_mode": nativeSurfaceMode,
		},
		"discovery_runtime": map[string]interface{}{
			"canonical_target":       skill,
			"selected_alias":         firstNonEmpty(selectedAlias, skill),
			"selected_native_mode":   nativeSurfaceMode,
			"native_surface_mode":    nativeSurfaceMode,
			"surface_reason":         nativeSurfaceReason,
			"execution_profile":      executionProfile,
			"skill_exec_cutover":     nativeSurfaceMode == "skill_exec",
			"forked_skill_execution": executionProfile == "prefer_fork" || executionProfile == "require_fork",
		},
		"skill_prompt_hint":     "Use the curated selector route.",
		"canonical_skill_id":    skill,
		"skill_need_clarify":    clarify,
		"skill_route_outcome":   outcome,
		"decision_reason":       "curated_test",
		"decision_stage":        "rerank",
		"smart_skill_selection": true,
	}
	if clarify {
		response["clarify_reason"] = "The request mixes local-workspace and live-web intents."
	}
	raw, _ := json.Marshal(response)
	var cloned map[string]interface{}
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func selectorEvalExecutionProfile(skill string) string {
	switch canonicalHarnessRouteName(skill) {
	case harnessCanonicalWebQuerySkill, "analyze":
		return "prefer_fork"
	case "deep_research":
		return "require_fork"
	default:
		return "inline"
	}
}

func selectorEvalToolSurface(selectedTools []string) map[string]interface{} {
	toolCount := 0
	schemaBytes := 0
	for _, tool := range selectedTools {
		tool = strings.TrimSpace(tool)
		if tool == "" {
			continue
		}
		toolCount++
		switch strings.ToLower(tool) {
		case "exec", "bash":
			schemaBytes += 80
		default:
			schemaBytes += 400
		}
	}
	return map[string]interface{}{
		"tool_count":   toolCount,
		"schema_bytes": schemaBytes,
	}
}

func createSelectorEvalSpecForItems(t *testing.T, controller *Controller, datasetName string, items []DatasetManifestItem) *EvalSpec {
	t.Helper()
	manifest := DatasetManifest{
		Dataset: DatasetManifestMeta{
			Name:    datasetName,
			Subject: SelectorCuratedDatasetSubject,
		},
		Defaults: DatasetManifestDefaults{
			RunKind:       RunKindAgentTask,
			Profile:       selectorCuratedProfile,
			RuntimePolicy: selectorDryRunRuntimePolicy(),
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
		Subject:        SelectorCuratedDatasetSubject,
		DefaultRunKind: RunKindAgentTask,
		DefaultProfile: selectorCuratedProfile,
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
		Profile:          selectorCuratedProfile,
		RuntimePolicy:    selectorDryRunRuntimePolicy(),
		ScoringConfig: GroupScoringConfig{
			Mode:          ScoringModeRule,
			PassThreshold: 0.5,
		},
	})
	if err != nil {
		t.Fatalf("CreateEvalSpec failed: %v", err)
	}
	return evalSpec
}

func runSelectorEvalReport(t *testing.T, controller *Controller, evalSpec *EvalSpec, title string, source selectorEvalSource, wantScorecards int) *EvalRunReport {
	t.Helper()
	controller.RegisterDriver(selectorEvalDriver{source: source})

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
	t.Fatalf("selector eval report = %#v, want completed with %d scorecards", report, wantScorecards)
	return nil
}

func selectorEvalReportTerminal(report *EvalRunReport, wantScorecards int) bool {
	return report != nil &&
		report.EvalRun != nil &&
		report.GroupReport != nil &&
		report.GroupReport.Group != nil &&
		isTerminalGroupStatus(report.EvalRun.Status) &&
		isTerminalGroupStatus(report.GroupReport.Group.Status) &&
		len(report.GroupReport.Scorecards) == wantScorecards
}

func selectorEvalQuery(goal string, meta map[string]interface{}) string {
	if groupInput, _ := meta["group_input"].(map[string]interface{}); len(groupInput) > 0 {
		if query := strings.TrimSpace(fmt.Sprint(groupInput["query"])); query != "" {
			return query
		}
	}
	if query := strings.TrimSpace(fmt.Sprint(meta["query"])); query != "" {
		return query
	}
	return strings.TrimSpace(goal)
}
