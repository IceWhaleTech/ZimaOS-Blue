package harness

import (
	"context"
	"testing"
)

func TestSkillCutoverIncreaseRate_IgnoresNearZeroLatencyJitter(t *testing.T) {
	if got := skillCutoverIncreaseRate(0, 27.5); got != 0 {
		t.Fatalf("skillCutoverIncreaseRate(0, 27.5) = %#v, want 0 within noise floor", got)
	}
	if got := skillCutoverIncreaseRate(20, 45); got != 0 {
		t.Fatalf("skillCutoverIncreaseRate(20, 45) = %#v, want 0 within noise floor", got)
	}
	if got := skillCutoverIncreaseRate(50, 100); got != 0 {
		t.Fatalf("skillCutoverIncreaseRate(50, 100) = %#v, want 0 within doubled low-latency jitter band", got)
	}
	if got := skillCutoverIncreaseRate(100, 200); got != 0 {
		t.Fatalf("skillCutoverIncreaseRate(100, 200) = %#v, want 0 within low-latency warmup band", got)
	}
	if got := skillCutoverIncreaseRate(0, 350); got != 0 {
		t.Fatalf("skillCutoverIncreaseRate(0, 350) = %#v, want 0 within selector warmup band", got)
	}
	if got := skillCutoverIncreaseRate(350, 450); got != 0 {
		t.Fatalf("skillCutoverIncreaseRate(350, 450) = %#v, want 0 within low-latency selector jitter band", got)
	}
	if got := skillCutoverIncreaseRate(-38.1598125, 100); got != 0 {
		t.Fatalf("skillCutoverIncreaseRate(-38.1598125, 100) = %#v, want 0 when baseline latency is negative jitter", got)
	}
	if got := skillCutoverIncreaseRate(0, 450); got <= defaultSkillCutoverMaxMedianLatencyIncreaseRate {
		t.Fatalf("skillCutoverIncreaseRate(0, 450) = %#v, want failure-sized increase above low-latency warmup band", got)
	}
}

func TestController_EvaluateSkillCutoverBudgetGate_PassesForExecOnlySurface(t *testing.T) {
	controller := newTestController(t)
	items := selectorGateSmokeItems(t)
	evalSpec := createSelectorEvalSpecForItems(t, controller, "Budget gate pass", items)

	baselineReport := runSelectorEvalReport(t, controller, evalSpec, "budget-baseline", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_query", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("analyze", true, "clarify"),
		},
	}, len(items))
	baseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "budget-pass-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   baselineReport.EvalRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline failed: %v", err)
	}

	candidateReport := runSelectorEvalReport(t, controller, evalSpec, "budget-candidate", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponseWithTools("web_query", []string{"exec"}, false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponseWithTools("exec", []string{"exec"}, true, "clarify"),
		},
	}, len(items))

	report, err := controller.EvaluateSkillCutoverBudgetGate(context.Background(), candidateReport.EvalRun.ID, SkillCutoverBudgetRequest{
		BaselineID: baseline.ID,
	})
	if err != nil {
		t.Fatalf("EvaluateSkillCutoverBudgetGate failed: %v", err)
	}

	if !report.Passed {
		t.Fatalf("report = %#v, want pass", report)
	}
	if report.Metrics.MedianSchemaByteReductionRate < 0.8 {
		t.Fatalf("median_schema_byte_reduction_rate = %#v, want >= 0.8", report.Metrics.MedianSchemaByteReductionRate)
	}
	if report.Metrics.NonAllowedNativeToolCaseCount != 0 {
		t.Fatalf("non_allowed_native_tool_case_count = %#v, want 0", report.Metrics.NonAllowedNativeToolCaseCount)
	}
	if got := report.Metrics.SelectedCanonicalSkillBreakdown[harnessCanonicalWebQuerySkill]; got != 1 {
		t.Fatalf("metrics.selected_canonical_skill_breakdown[web_query] = %#v, want 1", got)
	}
	if got := report.Metrics.SelectedCanonicalSkillBreakdown["exec"]; got != 1 {
		t.Fatalf("metrics.selected_canonical_skill_breakdown[exec] = %#v, want 1", got)
	}
	if got := report.Metrics.NativeSurfaceModeBreakdown["skill_exec"]; got != 1 {
		t.Fatalf("metrics.native_surface_mode_breakdown[skill_exec] = %#v, want 1", got)
	}
	if got := report.Metrics.NativeSurfaceModeBreakdown["clarify_none"]; got != 1 {
		t.Fatalf("metrics.native_surface_mode_breakdown[clarify_none] = %#v, want 1", got)
	}
	if got := report.Metrics.NativeSurfaceReasonBreakdown["discover_first_cutover"]; got != 1 {
		t.Fatalf("metrics.native_surface_reason_breakdown[discover_first_cutover] = %#v, want 1", got)
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
}

func TestController_EvaluateSkillCutoverBudgetGate_FailsForNonExecSurface(t *testing.T) {
	controller := newTestController(t)
	items := selectorGateSmokeItems(t)
	evalSpec := createSelectorEvalSpecForItems(t, controller, "Budget gate fail", items)

	baselineReport := runSelectorEvalReport(t, controller, evalSpec, "budget-baseline", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_query", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("analyze", true, "clarify"),
		},
	}, len(items))
	baseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "budget-fail-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   baselineReport.EvalRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline failed: %v", err)
	}

	candidateReport := runSelectorEvalReport(t, controller, evalSpec, "budget-candidate", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_query", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("analyze", true, "clarify"),
		},
	}, len(items))

	report, err := controller.EvaluateSkillCutoverBudgetGate(context.Background(), candidateReport.EvalRun.ID, SkillCutoverBudgetRequest{
		BaselineID: baseline.ID,
	})
	if err != nil {
		t.Fatalf("EvaluateSkillCutoverBudgetGate failed: %v", err)
	}

	if report.Passed {
		t.Fatalf("report = %#v, want fail", report)
	}
	if report.Metrics.NonAllowedNativeToolCaseCount == 0 {
		t.Fatalf("non_allowed_native_tool_case_count = %#v, want > 0", report.Metrics.NonAllowedNativeToolCaseCount)
	}
	if report.Metrics.MedianSchemaByteReductionRate <= 0 {
		t.Fatalf("median_schema_byte_reduction_rate = %#v, want > 0 because clarify cases now hide native tools", report.Metrics.MedianSchemaByteReductionRate)
	}
}

func TestController_EvaluateSkillCutoverBudgetGate_PrefersSelectedNativeSurface(t *testing.T) {
	controller := newTestController(t)
	items := selectorGateSmokeItems(t)
	evalSpec := createSelectorEvalSpecForItems(t, controller, "Budget gate native surface preference", items)

	baselineReport := runSelectorEvalReport(t, controller, evalSpec, "budget-baseline", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_query", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("exec", true, "clarify"),
		},
	}, len(items))
	baseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "budget-native-preference-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   baselineReport.EvalRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline failed: %v", err)
	}

	candidateReport := runSelectorEvalReport(t, controller, evalSpec, "budget-candidate", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.": {
				"selected_tools":               []interface{}{harnessCanonicalWebQuerySkill},
				"selected_tool_surface":        selectorEvalToolSurface([]string{harnessCanonicalWebQuerySkill}),
				"selected_native_tools":        []interface{}{"exec"},
				"selected_native_tool_surface": selectorEvalToolSurface([]string{"exec"}),
				"skill_decision":               map[string]interface{}{"selected_skill": harnessCanonicalWebQuerySkill, "need_clarify": false},
				"skill_prompt_hint":            "Use the curated selector route.",
				"canonical_skill_id":           harnessCanonicalWebQuerySkill,
				"skill_need_clarify":           false,
				"skill_route_outcome":          "selected",
				"decision_reason":              "curated_test",
				"decision_stage":               "rerank",
			},
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": {
				"selected_tools":               []interface{}{"exec"},
				"selected_tool_surface":        selectorEvalToolSurface([]string{"exec"}),
				"selected_native_tools":        []interface{}{},
				"selected_native_tool_surface": selectorEvalToolSurface(nil),
				"skill_decision":               map[string]interface{}{"selected_skill": "exec", "need_clarify": true},
				"skill_prompt_hint":            "Use the curated selector route.",
				"canonical_skill_id":           "exec",
				"skill_need_clarify":           true,
				"skill_route_outcome":          "clarify",
				"decision_reason":              "curated_test",
				"decision_stage":               "rerank",
				"clarify_reason":               "The request mixes local-workspace and live-web intents.",
			},
		},
	}, len(items))

	report, err := controller.EvaluateSkillCutoverBudgetGate(context.Background(), candidateReport.EvalRun.ID, SkillCutoverBudgetRequest{
		BaselineID: baseline.ID,
	})
	if err != nil {
		t.Fatalf("EvaluateSkillCutoverBudgetGate failed: %v", err)
	}

	if !report.Passed {
		t.Fatalf("report = %#v, want pass when selected_native_tools are exec-only/empty", report)
	}
	if report.Metrics.NonAllowedNativeToolCaseCount != 0 {
		t.Fatalf("non_allowed_native_tool_case_count = %#v, want 0", report.Metrics.NonAllowedNativeToolCaseCount)
	}
}

func TestController_EvaluateSkillCutoverBudgetGate_UsesLegacySurfaceForBaselineWhenAvailable(t *testing.T) {
	controller := newTestController(t)
	items := selectorGateSmokeItems(t)
	evalSpec := createSelectorEvalSpecForItems(t, controller, "Budget gate baseline surface preference", items)

	baselineReport := runSelectorEvalReport(t, controller, evalSpec, "budget-baseline", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.": {
				"selected_tools":               []interface{}{harnessCanonicalWebQuerySkill, "browser", "ask"},
				"selected_tool_surface":        selectorEvalToolSurface([]string{harnessCanonicalWebQuerySkill, "browser", "ask"}),
				"selected_native_tools":        []interface{}{"exec"},
				"selected_native_tool_surface": selectorEvalToolSurface([]string{"exec"}),
				"skill_decision":               map[string]interface{}{"selected_skill": harnessCanonicalWebQuerySkill, "need_clarify": false},
				"skill_prompt_hint":            "Use the curated selector route.",
				"canonical_skill_id":           harnessCanonicalWebQuerySkill,
				"skill_need_clarify":           false,
				"skill_route_outcome":          "selected",
				"decision_reason":              "curated_test",
				"decision_stage":               "rerank",
			},
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": {
				"selected_tools":               []interface{}{"exec", "browser"},
				"selected_tool_surface":        selectorEvalToolSurface([]string{"exec", "browser"}),
				"selected_native_tools":        []interface{}{},
				"selected_native_tool_surface": selectorEvalToolSurface(nil),
				"skill_decision":               map[string]interface{}{"selected_skill": "exec", "need_clarify": true},
				"skill_prompt_hint":            "Use the curated selector route.",
				"canonical_skill_id":           "exec",
				"skill_need_clarify":           true,
				"skill_route_outcome":          "clarify",
				"decision_reason":              "curated_test",
				"decision_stage":               "rerank",
				"clarify_reason":               "The request mixes local-workspace and live-web intents.",
			},
		},
	}, len(items))
	baseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "budget-baseline-prefers-legacy-surface",
		OwnerUserID: "user-1",
		EvalRunID:   baselineReport.EvalRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline failed: %v", err)
	}

	candidateReport := runSelectorEvalReport(t, controller, evalSpec, "budget-candidate", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.": {
				"selected_tools":               []interface{}{harnessCanonicalWebQuerySkill, "browser", "ask"},
				"selected_tool_surface":        selectorEvalToolSurface([]string{harnessCanonicalWebQuerySkill, "browser", "ask"}),
				"selected_native_tools":        []interface{}{"exec"},
				"selected_native_tool_surface": selectorEvalToolSurface([]string{"exec"}),
				"skill_decision":               map[string]interface{}{"selected_skill": harnessCanonicalWebQuerySkill, "need_clarify": false},
				"skill_prompt_hint":            "Use the curated selector route.",
				"canonical_skill_id":           harnessCanonicalWebQuerySkill,
				"skill_need_clarify":           false,
				"skill_route_outcome":          "selected",
				"decision_reason":              "curated_test",
				"decision_stage":               "rerank",
			},
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": {
				"selected_tools":               []interface{}{"exec", "browser"},
				"selected_tool_surface":        selectorEvalToolSurface([]string{"exec", "browser"}),
				"selected_native_tools":        []interface{}{},
				"selected_native_tool_surface": selectorEvalToolSurface(nil),
				"skill_decision":               map[string]interface{}{"selected_skill": "exec", "need_clarify": true},
				"skill_prompt_hint":            "Use the curated selector route.",
				"canonical_skill_id":           "exec",
				"skill_need_clarify":           true,
				"skill_route_outcome":          "clarify",
				"decision_reason":              "curated_test",
				"decision_stage":               "rerank",
				"clarify_reason":               "The request mixes local-workspace and live-web intents.",
			},
		},
	}, len(items))

	report, err := controller.EvaluateSkillCutoverBudgetGate(context.Background(), candidateReport.EvalRun.ID, SkillCutoverBudgetRequest{
		BaselineID: baseline.ID,
	})
	if err != nil {
		t.Fatalf("EvaluateSkillCutoverBudgetGate failed: %v", err)
	}

	if !report.Passed {
		t.Fatalf("report = %#v, want pass when baseline keeps legacy surface and candidate keeps exec-only final surface", report)
	}
	if report.Metrics.MedianSchemaByteReductionRate <= 0 {
		t.Fatalf("median_schema_byte_reduction_rate = %#v, want > 0", report.Metrics.MedianSchemaByteReductionRate)
	}
}
