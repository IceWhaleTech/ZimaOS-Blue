package harness

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/labstack/echo/v4"
)

func TestHandler_ListRunsSupportsTreeFilters(t *testing.T) {
	controller := newTestController(t)
	parentDriver := &stubDriver{kind: RunKindAgentTask}
	childDriver := &stubDriver{kind: RunKindSubagent}
	controller.RegisterDriver(parentDriver)
	controller.RegisterDriver(childDriver)

	parent, err := controller.Submit(context.Background(), RunSpec{
		Kind:         RunKindAgentTask,
		Goal:         "parent",
		UserID:       "user-1",
		ApprovalMode: ApprovalModeAsk,
		MaxDepth:     2,
		MaxSubagents: 2,
	})
	if err != nil {
		t.Fatalf("Submit parent failed: %v", err)
	}
	child, err := controller.SpawnChild(context.Background(), parent.ID, RunSpec{
		Kind: RunKindSubagent,
		Goal: "child",
	})
	if err != nil {
		t.Fatalf("SpawnChild failed: %v", err)
	}
	child.Status = RunStatusCompleted
	if err := controller.store.UpdateRun(context.Background(), child); err != nil {
		t.Fatalf("UpdateRun failed: %v", err)
	}

	if _, err := controller.Submit(context.Background(), RunSpec{
		Kind:   RunKindAgentTask,
		Goal:   "other",
		UserID: "user-1",
	}); err != nil {
		t.Fatalf("Submit sibling failed: %v", err)
	}

	handler := NewHandler(controller)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/runs?root_run_id="+parent.RootRunID+"&parent_run_id="+parent.ID+"&statuses=completed&kinds=subagent", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.ListRuns(c); err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var runs []Run
	if err := json.Unmarshal(rec.Body.Bytes(), &runs); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs len = %d, want 1; body=%s", len(runs), rec.Body.String())
	}
	if runs[0].ID != child.ID {
		t.Fatalf("run id = %q, want %q", runs[0].ID, child.ID)
	}
}

func TestHandler_GetRunDetailIncludesAvailableActions(t *testing.T) {
	controller := newTestController(t)
	driver := &stubDriver{kind: RunKindWorkflow}
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:   RunKindWorkflow,
		Goal:   "Resume workflow review",
		UserID: "user-1",
		Metadata: map[string]interface{}{
			"workflow_execution_id":    "exec-1",
			"workflow_checkpoint_kind": "pause_for_approval",
			"workflow_name":            "Review Workflow",
		},
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	run.Status = RunStatusWaitingInput
	if err := controller.store.UpdateRun(context.Background(), run); err != nil {
		t.Fatalf("UpdateRun failed: %v", err)
	}

	handler := NewHandler(controller)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/runs/"+run.ID+"/detail", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-1"}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(run.ID)

	if err := handler.GetRunDetail(c); err != nil {
		t.Fatalf("GetRunDetail returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var detail RunDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(detail.Actions.Items) != 2 {
		t.Fatalf("action descriptors = %#v, want two descriptors", detail.Actions.Items)
	}
	if detail.Actions.Items[0].ID != "cancel" || detail.Actions.Items[0].Path != "/harness/runs/"+run.ID+"/actions/cancel" {
		t.Fatalf("cancel descriptor = %#v, want harness cancel path", detail.Actions.Items[0])
	}
	if detail.Actions.Items[0].Variant != "danger" {
		t.Fatalf("cancel descriptor variant = %q, want danger", detail.Actions.Items[0].Variant)
	}
	if detail.Actions.Items[1].ID != "resume" || detail.Actions.Items[1].Path != "/harness/runs/"+run.ID+"/actions/resume" {
		t.Fatalf("resume descriptor = %#v, want harness resume path", detail.Actions.Items[1])
	}
	if detail.Actions.Items[1].Variant != "primary" {
		t.Fatalf("resume descriptor variant = %q, want primary", detail.Actions.Items[1].Variant)
	}
	if !detail.Actions.Items[1].RequiresInput {
		t.Fatalf("resume descriptor requires_input = %v, want true", detail.Actions.Items[1].RequiresInput)
	}
	if detail.Actions.Items[1].Input.Title == "" || detail.Actions.Items[1].Input.SubmitLabel != "Submit decision" {
		t.Fatalf("resume descriptor dialog copy = %#v, want title and submit label", detail.Actions.Items[1].Input)
	}
	if len(detail.Actions.Items[1].Input.Fields) != 2 {
		t.Fatalf("resume descriptor fields = %#v, want decision/comment schema", detail.Actions.Items[1].Input.Fields)
	}
	if detail.Actions.Items[1].Input.Fields[0].Kind != "choice" || len(detail.Actions.Items[1].Input.Fields[0].Options) != 2 || detail.Actions.Items[1].Input.Fields[0].Options[1] != "reject" {
		t.Fatalf("resume descriptor input = %#v, want approve/reject options", detail.Actions.Items[1].Input)
	}
	if detail.Actions.Items[1].Input.Fields[1].Kind != "textarea" || detail.Actions.Items[1].Input.Fields[1].PayloadKey != "comment" {
		t.Fatalf("resume descriptor payload schema = %#v, want text comment payload", detail.Actions.Items[1].Input.Fields)
	}
}

func TestHandler_GetRunDetailIncludesRunTrace(t *testing.T) {
	controller := newTestController(t)
	driver := &stubDriver{kind: RunKindAgentTask}
	controller.RegisterDriver(driver)
	controller.SetRunTraceProvider(NewRunTraceCollector(controller))

	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:   RunKindAgentTask,
		Goal:   "Trace this run",
		UserID: "user-1",
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	startedAt := time.Now().UTC().Add(-1500 * time.Millisecond)
	finishedAt := startedAt.Add(1500 * time.Millisecond)
	run.Status = RunStatusCompleted
	run.StartedAt = &startedAt
	run.FinishedAt = &finishedAt
	run.UpdatedAt = finishedAt
	if err := controller.store.UpdateRun(context.Background(), run); err != nil {
		t.Fatalf("UpdateRun failed: %v", err)
	}
	if err := controller.AppendEvent(context.Background(), RunEvent{
		RunID:     run.ID,
		RootRunID: run.RootRunID,
		Type:      "trace_started",
		Message:   "driver start completed",
		CreatedAt: startedAt,
	}); err != nil {
		t.Fatalf("AppendEvent(trace_started) failed: %v", err)
	}
	if err := controller.AppendEvent(context.Background(), RunEvent{
		RunID:       run.ID,
		RootRunID:   run.RootRunID,
		Type:        "stage_changed",
		Message:     "run finalized",
		CreatedAt:   finishedAt,
		PayloadJSON: marshalInterface(map[string]interface{}{"stage": RuntimeStageFinalize, "status": RunStatusCompleted}),
	}); err != nil {
		t.Fatalf("AppendEvent(stage_changed) failed: %v", err)
	}
	if err := controller.store.AttachArtifact(context.Background(), ArtifactRef{
		ID:        "artifact-trace",
		RunID:     run.ID,
		Kind:      "trace",
		Label:     "trace log",
		PathOrURL: "/tmp/trace.log",
	}); err != nil {
		t.Fatalf("AttachArtifact failed: %v", err)
	}

	handler := NewHandler(controller)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/runs/"+run.ID+"/detail", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-1"}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(run.ID)

	if err := handler.GetRunDetail(c); err != nil {
		t.Fatalf("GetRunDetail returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var detail RunDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if detail.RunTrace == nil {
		t.Fatalf("expected run_trace in detail, got %#v", detail)
	}
	if detail.RunTrace.RunID != run.ID || detail.RunTrace.Status != RunStatusCompleted {
		t.Fatalf("unexpected trace header: %#v", detail.RunTrace)
	}
	if !hasTraceStage(detail.RunTrace.Stages, RuntimeStageFinalize) {
		t.Fatalf("unexpected trace stages: %#v", detail.RunTrace.Stages)
	}
	if !hasTraceEvent(detail.RunTrace.Events, "trace_started") {
		t.Fatalf("unexpected trace events: %#v", detail.RunTrace.Events)
	}
	if len(detail.RunTrace.Artifacts) != 1 || detail.RunTrace.Artifacts[0].Label != "trace log" {
		t.Fatalf("unexpected trace artifacts: %#v", detail.RunTrace.Artifacts)
	}
}

func TestHandler_PerformRunActionResumesScopedRun(t *testing.T) {
	controller := newTestController(t)
	driver := &stubDriver{
		kind: RunKindWorkflow,
		actionResults: map[string]RunStatus{
			"resume": RunStatusExecuting,
		},
	}
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:   RunKindWorkflow,
		Goal:   "Resume workflow review",
		UserID: "user-1",
		Metadata: map[string]interface{}{
			"workflow_execution_id":    "exec-1",
			"workflow_checkpoint_kind": "pause_for_approval",
		},
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	run.Status = RunStatusWaitingInput
	if err := controller.store.UpdateRun(context.Background(), run); err != nil {
		t.Fatalf("UpdateRun failed: %v", err)
	}

	handler := NewHandler(controller)
	e := echo.New()
	body := strings.NewReader(`{"decision":"approve","payload":{"ticket":"A-1"}}`)
	req := httptest.NewRequest(http.MethodPost, "/runs/"+run.ID+"/actions/resume", body)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-1"}))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id", "action")
	c.SetParamValues(run.ID, "resume")

	if err := handler.PerformRunAction(c); err != nil {
		t.Fatalf("PerformRunAction returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var updated Run
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if updated.ID != run.ID || updated.Status != RunStatusExecuting {
		t.Fatalf("updated run = %#v, want executing snapshot", updated)
	}
	if len(driver.actions) != 1 || driver.actions[0] != "resume" {
		t.Fatalf("driver actions = %#v, want [resume]", driver.actions)
	}
	if len(driver.actionInputs) != 1 {
		t.Fatalf("action inputs = %#v, want one payload", driver.actionInputs)
	}
	if got := runActionMetadataString(driver.actionInputs[0], "decision"); got != "approve" {
		t.Fatalf("decision = %q, want approve", got)
	}
	payload, _ := driver.actionInputs[0]["payload"].(map[string]interface{})
	if got := runActionMetadataString(payload, "ticket"); got != "A-1" {
		t.Fatalf("payload.ticket = %q, want A-1", got)
	}
}

func TestHandler_PerformRunActionCancelsScopedRun(t *testing.T) {
	controller := newTestController(t)
	driver := &stubDriver{kind: RunKindAgentTask}
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:   RunKindAgentTask,
		Goal:   "Cancel active run",
		UserID: "user-1",
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	run.Status = RunStatusExecuting
	if err := controller.store.UpdateRun(context.Background(), run); err != nil {
		t.Fatalf("UpdateRun failed: %v", err)
	}

	handler := NewHandler(controller)
	e := echo.New()
	body := strings.NewReader(`{"reason":"cancelled by route"}`)
	req := httptest.NewRequest(http.MethodPost, "/runs/"+run.ID+"/actions/cancel", body)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-1"}))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id", "action")
	c.SetParamValues(run.ID, "cancel")

	if err := handler.PerformRunAction(c); err != nil {
		t.Fatalf("PerformRunAction returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var updated Run
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if updated.ID != run.ID || updated.Status != RunStatusCancelled {
		t.Fatalf("updated run = %#v, want cancelled snapshot", updated)
	}
	if strings.TrimSpace(updated.Error) != "cancelled by route" {
		t.Fatalf("updated error = %q, want cancelled by route", updated.Error)
	}
	if len(driver.cancelled) != 1 || driver.cancelled[0] != run.ID {
		t.Fatalf("driver cancel calls = %#v, want [%q]", driver.cancelled, run.ID)
	}
}

func TestHandler_PerformGroupActionCancelsScopedGroup(t *testing.T) {
	controller := newTestController(t)
	group, err := controller.SubmitGroup(context.Background(), RunGroupSpec{
		Kind:        RunGroupKindEval,
		Title:       "Cancelable Group",
		Subject:     "research",
		OwnerUserID: "user-1",
		Items: []RunGroupItemSpec{{
			RunKind: RunKindResearch,
			Profile: "research",
			Input: map[string]interface{}{
				"goal": "Investigate",
			},
		}},
	})
	if err != nil {
		t.Fatalf("SubmitGroup failed: %v", err)
	}
	group.Status = RunGroupStatusRunning
	if err := controller.store.UpdateGroup(context.Background(), group); err != nil {
		t.Fatalf("UpdateGroup failed: %v", err)
	}

	handler := NewHandler(controller)
	e := echo.New()
	body := strings.NewReader(`{"reason":"cancelled by group route"}`)
	req := httptest.NewRequest(http.MethodPost, "/groups/"+group.ID+"/actions/cancel", body)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-1"}))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id", "action")
	c.SetParamValues(group.ID, "cancel")

	if err := handler.PerformGroupAction(c); err != nil {
		t.Fatalf("PerformGroupAction returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var updated RunGroup
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if updated.ID != group.ID || updated.Status != RunGroupStatusCancelled {
		t.Fatalf("updated group = %#v, want cancelled snapshot", updated)
	}
	if got := runActionMetadataString(updated.Summary, "cancel_reason"); got != "cancelled by group route" {
		t.Fatalf("cancel_reason = %q, want cancelled by group route", got)
	}
}

func TestHandler_EnsureSelectorCuratedAssetsUsesScopedUser(t *testing.T) {
	controller := newTestController(t)
	handler := NewHandler(controller)
	e := echo.New()

	req := httptest.NewRequest(http.MethodPost, "/selector-curated/ensure", strings.NewReader(`{}`))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-1"}))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.EnsureSelectorCuratedAssets(c); err != nil {
		t.Fatalf("EnsureSelectorCuratedAssets returned error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var assets SelectorCuratedAssets
	if err := json.Unmarshal(rec.Body.Bytes(), &assets); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if assets.Dataset == nil || assets.Dataset.OwnerUserID != "user-1" {
		t.Fatalf("dataset = %#v, want owner user-1", assets.Dataset)
	}
	if assets.EvalSpec == nil || assets.EvalSpec.OwnerUserID != "user-1" {
		t.Fatalf("eval spec = %#v, want owner user-1", assets.EvalSpec)
	}
}

func TestHandler_EnsureBatch1ExecutionAssetsUsesScopedUser(t *testing.T) {
	controller := newTestController(t)
	handler := NewHandler(controller)
	e := echo.New()

	req := httptest.NewRequest(http.MethodPost, "/execution-batch1/ensure", strings.NewReader(`{}`))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-1"}))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.EnsureBatch1ExecutionAssets(c); err != nil {
		t.Fatalf("EnsureBatch1ExecutionAssets returned error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var assets Batch1ExecutionAssets
	if err := json.Unmarshal(rec.Body.Bytes(), &assets); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if assets.Dataset == nil || assets.Dataset.OwnerUserID != "user-1" {
		t.Fatalf("dataset = %#v, want owner user-1", assets.Dataset)
	}
	if assets.EvalSpec == nil || assets.EvalSpec.OwnerUserID != "user-1" {
		t.Fatalf("eval spec = %#v, want owner user-1", assets.EvalSpec)
	}
}

func TestHandler_EvaluateSelectorGateReturnsStructuredGateReport(t *testing.T) {
	controller := newTestController(t)
	handler := NewHandler(controller)
	items := selectorGateSmokeItems(t)
	evalSpec := createSelectorEvalSpecForItems(t, controller, "Selector gate handler", items)

	baselineReport := runSelectorEvalReport(t, controller, evalSpec, "selector-gate-baseline", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_search", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("exec", true, "clarify"),
		},
	}, len(items))
	baseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "selector-gate-handler-baseline",
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

	e := echo.New()
	body := `{"baseline_id":"` + baseline.ID + `"}`
	req := httptest.NewRequest(http.MethodPost, "/eval-runs/"+candidateReport.EvalRun.ID+"/selector-gate", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-1"}))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(candidateReport.EvalRun.ID)

	if err := handler.EvaluateSelectorGate(c); err != nil {
		t.Fatalf("EvaluateSelectorGate returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var report SelectorGateReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !report.Passed {
		t.Fatalf("report = %#v, want pass", report)
	}
	if report.BaselineID != baseline.ID {
		t.Fatalf("baseline_id = %q, want %q", report.BaselineID, baseline.ID)
	}
	if report.Metrics.PassRate != 1 {
		t.Fatalf("metrics.pass_rate = %#v, want 1", report.Metrics.PassRate)
	}
}

func TestHandler_EvaluateExecutionEquivalenceReturnsStructuredGateReport(t *testing.T) {
	controller := newTestController(t)
	handler := NewHandler(controller)
	items := executionGateSmokeItems(t)
	evalSpec := createExecutionEvalSpecForItems(t, controller, "Execution gate handler", items)

	baselineReport := runExecutionEvalReport(t, controller, evalSpec, "execution-gate-baseline", executionEvalDriver{
		defaultOutcome: executionEvalOutcome{Status: RunStatusCompleted, Result: "completed"},
		delay:          10 * time.Millisecond,
	}, len(items))
	baseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "execution-gate-handler-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   baselineReport.EvalRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline failed: %v", err)
	}
	candidateReport := runExecutionEvalReport(t, controller, evalSpec, "execution-gate-candidate", executionEvalDriver{
		defaultOutcome: executionEvalOutcome{Status: RunStatusCompleted, Result: "completed"},
		delay:          10 * time.Millisecond,
	}, len(items))

	e := echo.New()
	body := `{"baseline_id":"` + baseline.ID + `"}`
	req := httptest.NewRequest(http.MethodPost, "/eval-runs/"+candidateReport.EvalRun.ID+"/execution-gate", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-1"}))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(candidateReport.EvalRun.ID)

	if err := handler.EvaluateExecutionEquivalence(c); err != nil {
		t.Fatalf("EvaluateExecutionEquivalence returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var report ExecutionEquivalenceReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !report.Passed {
		t.Fatalf("report = %#v, want pass", report)
	}
	if report.BaselineID != baseline.ID {
		t.Fatalf("baseline_id = %q, want %q", report.BaselineID, baseline.ID)
	}
	if report.Metrics.PassRate != 1 {
		t.Fatalf("metrics.pass_rate = %#v, want 1", report.Metrics.PassRate)
	}
}

func TestHandler_EvaluateSkillCutoverBudgetGateReturnsStructuredGateReport(t *testing.T) {
	controller := newTestController(t)
	handler := NewHandler(controller)
	items := selectorGateSmokeItems(t)
	evalSpec := createSelectorEvalSpecForItems(t, controller, "Budget gate handler", items)

	baselineReport := runSelectorEvalReport(t, controller, evalSpec, "budget-gate-baseline", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_search", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("analyze", true, "clarify"),
		},
	}, len(items))
	baseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "budget-gate-handler-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   baselineReport.EvalRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline failed: %v", err)
	}
	candidateReport := runSelectorEvalReport(t, controller, evalSpec, "budget-gate-candidate", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponseWithTools("web_search", []string{"exec"}, false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponseWithTools("exec", []string{"exec"}, true, "clarify"),
		},
	}, len(items))

	e := echo.New()
	body := `{"baseline_id":"` + baseline.ID + `"}`
	req := httptest.NewRequest(http.MethodPost, "/eval-runs/"+candidateReport.EvalRun.ID+"/budget-gate", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-1"}))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(candidateReport.EvalRun.ID)

	if err := handler.EvaluateSkillCutoverBudgetGate(c); err != nil {
		t.Fatalf("EvaluateSkillCutoverBudgetGate returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var report SkillCutoverBudgetReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !report.Passed {
		t.Fatalf("report = %#v, want pass", report)
	}
	if report.BaselineID != baseline.ID {
		t.Fatalf("baseline_id = %q, want %q", report.BaselineID, baseline.ID)
	}
	if report.Metrics.NonAllowedNativeToolCaseCount != 0 {
		t.Fatalf("metrics.non_allowed_native_tool_case_count = %#v, want 0", report.Metrics.NonAllowedNativeToolCaseCount)
	}
}

func TestHandler_EvaluateSkillCutoverReadinessUsesScopedUser(t *testing.T) {
	controller := newTestController(t)
	handler := NewHandler(controller)

	selectorItems := selectorGateSmokeItems(t)
	selectorEvalSpec := createSelectorEvalSpecForItems(t, controller, "Cutover readiness handler selector", selectorItems)
	executionItems := executionGateSmokeItems(t)
	executionEvalSpec := createExecutionEvalSpecForItems(t, controller, "Cutover readiness handler execution", executionItems)

	selectorBaselineReport := runSelectorEvalReportForCandidate(t, controller, selectorEvalSpec, "selector-baseline", "", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_search", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("exec", true, "clarify"),
		},
	}, len(selectorItems))
	selectorBaseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "cutover-handler-selector-baseline",
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
		Name:        "cutover-handler-execution-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   executionBaselineReport.EvalRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline(execution) failed: %v", err)
	}

	candidateID := "rc-handler"
	runSelectorEvalReportForCandidate(t, controller, selectorEvalSpec, "selector-candidate", candidateID, selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_search", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("exec", true, "clarify"),
		},
	}, len(selectorItems))
	runExecutionEvalReportForCandidate(t, controller, executionEvalSpec, "execution-candidate", candidateID, executionEvalDriver{
		defaultOutcome: executionEvalOutcome{Status: RunStatusCompleted, Result: "completed"},
		delay:          10 * time.Millisecond,
	}, len(executionItems))

	e := echo.New()
	body := `{"owner_user_id":"ignored-user","candidate_id":"` + candidateID + `","selector_eval_spec_id":"` + selectorEvalSpec.ID + `","execution_eval_spec_id":"` + executionEvalSpec.ID + `","required_consecutive_runs":1,"selector":{"baseline_id":"` + selectorBaseline.ID + `"},"execution":{"baseline_id":"` + executionBaseline.ID + `"}}`
	req := httptest.NewRequest(http.MethodPost, "/cutover-readiness", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-1"}))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.EvaluateSkillCutoverReadiness(c); err != nil {
		t.Fatalf("EvaluateSkillCutoverReadiness returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var report SkillCutoverReadinessReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if report.CandidateID != candidateID {
		t.Fatalf("candidate_id = %q, want %q", report.CandidateID, candidateID)
	}
	if report.EvaluatedGatesReady {
		t.Fatalf("evaluated_gates_ready = %#v, want false while budget gate is red", report.EvaluatedGatesReady)
	}
	if report.Ready {
		t.Fatalf("ready = %#v, want false while budget gate is red", report.Ready)
	}
	if report.Budget.Ready {
		t.Fatalf("budget readiness = %#v, want false", report.Budget)
	}
}

func TestHandler_GetComparisonReportRespectsScopedUser(t *testing.T) {
	controller := newTestController(t)
	handler := NewHandler(controller)
	items := selectorGateSmokeItems(t)
	evalSpec := createSelectorEvalSpecForItems(t, controller, "Comparison report handler", items)

	baselineReport := runSelectorEvalReport(t, controller, evalSpec, "comparison-handler-baseline", selectorEvalSource{
		responses: map[string]map[string]interface{}{
			"Search the latest OpenAI Responses API documentation.":            selectorEvalResponse("web_search", false, "selected"),
			"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponse("exec", true, "clarify"),
		},
	}, len(items))
	baseline, err := controller.CreateBaseline(context.Background(), BaselineSpec{
		Name:        "comparison-handler-baseline",
		OwnerUserID: "user-1",
		EvalRunID:   baselineReport.EvalRun.ID,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateBaseline failed: %v", err)
	}
	candidateReport := runSelectorEvalReport(t, controller, evalSpec, "comparison-handler-candidate", selectorEvalSource{
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

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/comparison-reports/"+comparison.ID, nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-1"}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(comparison.ID)

	if err := handler.GetComparisonReport(c); err != nil {
		t.Fatalf("GetComparisonReport returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var stored ComparisonReport
	if err := json.Unmarshal(rec.Body.Bytes(), &stored); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if stored.ID != comparison.ID {
		t.Fatalf("comparison id = %q, want %q", stored.ID, comparison.ID)
	}
	if stored.OwnerUserID != "user-1" {
		t.Fatalf("owner_user_id = %q, want %q", stored.OwnerUserID, "user-1")
	}
	if stored.BaselineID != baseline.ID {
		t.Fatalf("baseline_id = %q, want %q", stored.BaselineID, baseline.ID)
	}
}
