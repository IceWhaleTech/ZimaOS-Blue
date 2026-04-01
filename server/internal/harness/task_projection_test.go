package harness

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/labstack/echo/v4"
)

type stubRunDetailProvider struct {
	approvals map[string][]map[string]interface{}
	questions map[string][]map[string]interface{}
}

func (p *stubRunDetailProvider) PendingApprovals(runID string) []map[string]interface{} {
	if p == nil {
		return nil
	}
	return p.approvals[runID]
}

func (p *stubRunDetailProvider) PendingQuestions(runID string) []map[string]interface{} {
	if p == nil {
		return nil
	}
	return p.questions[runID]
}

func TestUserTaskProjectionService_ListScopesAndBlockers(t *testing.T) {
	controller := newTestController(t)
	agentDriver := &stubDriver{kind: RunKindAgentTask}
	researchDriver := &stubDriver{kind: RunKindResearch}
	workflowDriver := &stubDriver{kind: RunKindWorkflow}
	subagentDriver := &stubDriver{kind: RunKindSubagent}
	controller.RegisterDriver(agentDriver)
	controller.RegisterDriver(researchDriver)
	controller.RegisterDriver(workflowDriver)
	controller.RegisterDriver(subagentDriver)

	ctx := context.Background()
	currentRun, err := controller.Submit(ctx, RunSpec{
		Kind:           RunKindAgentTask,
		Goal:           "Ship the release checklist",
		UserID:         "user-1",
		ConversationID: "conv-current",
	})
	if err != nil {
		t.Fatalf("Submit current run failed: %v", err)
	}
	currentRun.Status = RunStatusWaitingInput
	currentRun.Progress = 42
	currentRun.Result = "draft release notes"
	if err := controller.store.UpdateRun(ctx, currentRun); err != nil {
		t.Fatalf("UpdateRun current failed: %v", err)
	}
	if err := controller.AttachArtifact(ctx, ArtifactRef{
		ID:        "artifact-visible",
		RunID:     currentRun.ID,
		Kind:      "report",
		Label:     "Release report",
		PathOrURL: "https://example.com/reports/release",
	}); err != nil {
		t.Fatalf("AttachArtifact visible failed: %v", err)
	}
	if err := controller.AttachArtifact(ctx, ArtifactRef{
		ID:        "artifact-hidden",
		RunID:     currentRun.ID,
		Kind:      "log",
		Label:     "trace log",
		PathOrURL: "/tmp/run.log",
	}); err != nil {
		t.Fatalf("AttachArtifact hidden failed: %v", err)
	}

	backgroundRun, err := controller.Submit(ctx, RunSpec{
		Kind:           RunKindResearch,
		Goal:           "Research storage pricing",
		UserID:         "user-1",
		ConversationID: "conv-background",
		Metadata: map[string]interface{}{
			"stage":         "verify",
			"latest_action": "cross-checking vendors",
			"latest_gap":    "missing EU pricing",
			"source_inventory": []map[string]interface{}{
				{
					"title":             "Primary vendor pricing page",
					"url":               "https://example.com/pricing",
					"domain":            "example.com",
					"source_type":       "web",
					"relevance_score":   0.98,
					"credibility_score": 0.94,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Submit background run failed: %v", err)
	}
	backgroundRun.Status = RunStatusExecuting
	backgroundRun.Progress = 67
	if err := controller.store.UpdateRun(ctx, backgroundRun); err != nil {
		t.Fatalf("UpdateRun background failed: %v", err)
	}

	groupRun, err := controller.Submit(ctx, RunSpec{
		Kind:           RunKindAgentTask,
		Goal:           "Internal eval case",
		UserID:         "user-1",
		ConversationID: "conv-current",
		GroupID:        "group-1",
		GroupItemID:    "item-1",
	})
	if err != nil {
		t.Fatalf("Submit group run failed: %v", err)
	}
	groupRun.Status = RunStatusExecuting
	if err := controller.store.UpdateRun(ctx, groupRun); err != nil {
		t.Fatalf("UpdateRun group run failed: %v", err)
	}

	childRun, err := controller.SpawnChild(ctx, currentRun.ID, RunSpec{
		Kind: RunKindSubagent,
		Goal: "internal subagent",
	})
	if err != nil {
		t.Fatalf("SpawnChild failed: %v", err)
	}
	childRun.Status = RunStatusExecuting
	if err := controller.store.UpdateRun(ctx, childRun); err != nil {
		t.Fatalf("UpdateRun child failed: %v", err)
	}

	service := NewUserTaskProjectionService(controller, &stubRunDetailProvider{
		approvals: map[string][]map[string]interface{}{
			currentRun.ID: {{
				"id": "approval-1",
			}},
		},
	})

	current, err := service.List(ctx, UserTaskProjectionFilter{
		UserID:         "user-1",
		ConversationID: "conv-current",
		Scope:          "current",
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("List current failed: %v", err)
	}
	if len(current) != 1 {
		t.Fatalf("current len = %d, want 1", len(current))
	}
	if current[0].ID != currentRun.ID {
		t.Fatalf("current[0].ID = %q, want %q", current[0].ID, currentRun.ID)
	}
	if current[0].Blocker == nil || current[0].Blocker.Kind != "approval" {
		t.Fatalf("expected approval blocker, got %#v", current[0].Blocker)
	}
	if current[0].Status != "waiting_user" || current[0].Stage != "waiting_user" {
		t.Fatalf("unexpected waiting mapping: %#v", current[0])
	}
	if len(current[0].Artifacts) != 1 || current[0].Artifacts[0].Kind != "report" {
		t.Fatalf("artifacts = %#v, want only visible report", current[0].Artifacts)
	}
	if len(current[0].Actions.Items) != 2 || current[0].Actions.Items[0].Path != "/tasks/"+currentRun.ID+"/actions/cancel" {
		t.Fatalf("expected current task action descriptors, got %#v", current[0].Actions.Items)
	}
	if current[0].Actions.Items[0].Variant != "danger" {
		t.Fatalf("expected current task cancel variant danger, got %#v", current[0].Actions.Items[0])
	}
	if current[0].Actions.Items[1].ID != "send_update" || current[0].Actions.Items[1].Path != "/agent/tasks/"+currentRun.ID+"/message" {
		t.Fatalf("expected current task send_update descriptor, got %#v", current[0].Actions.Items[1])
	}
	if !current[0].Actions.Items[1].RequiresInput {
		t.Fatalf("expected current task send_update descriptor to require input, got %#v", current[0].Actions.Items[1])
	}
	if current[0].Actions.Items[1].Input == nil || len(current[0].Actions.Items[1].Input.Fields) != 1 {
		t.Fatalf("expected current task send_update input schema, got %#v", current[0].Actions.Items[1].Input)
	}

	background, err := service.List(ctx, UserTaskProjectionFilter{
		UserID:         "user-1",
		ConversationID: "conv-current",
		Scope:          "background",
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("List background failed: %v", err)
	}
	if len(background) != 1 {
		t.Fatalf("background len = %d, want 1", len(background))
	}
	if background[0].ID != backgroundRun.ID {
		t.Fatalf("background[0].ID = %q, want %q", background[0].ID, backgroundRun.ID)
	}
	if background[0].Subtitle == "" {
		t.Fatalf("expected research subtitle from metadata")
	}
	if len(background[0].ResearchSources) != 1 || background[0].ResearchSources[0].Title != "Primary vendor pricing page" {
		t.Fatalf("unexpected research sources: %#v", background[0].ResearchSources)
	}
	if len(background[0].Actions.Items) != 1 || background[0].Actions.Items[0].Path != "/tasks/"+backgroundRun.ID+"/actions/cancel" {
		t.Fatalf("expected background task action descriptor, got %#v", background[0].Actions.Items)
	}
	if background[0].Actions.Items[0].Variant != "danger" {
		t.Fatalf("expected background task cancel variant danger, got %#v", background[0].Actions.Items[0])
	}
}

func TestUserTaskProjectionService_ListsWorkflowRuns(t *testing.T) {
	controller := newTestController(t)
	controller.RegisterDriver(&stubDriver{kind: RunKindWorkflow})

	ctx := context.Background()
	run, err := controller.Submit(ctx, RunSpec{
		Kind:           RunKindWorkflow,
		Goal:           "Execute nightly sync",
		UserID:         "user-1",
		ConversationID: "conv-workflow",
		Metadata: map[string]interface{}{
			"workflow_execution_id":    "exec-workflow-1",
			"workflow_name":            "Nightly Sync",
			"workflow_status_reason":   "approval_needed",
			"workflow_checkpoint_kind": "pause_for_approval",
		},
	})
	if err != nil {
		t.Fatalf("Submit workflow run failed: %v", err)
	}
	run.Status = RunStatusWaitingInput
	if err := controller.store.UpdateRun(ctx, run); err != nil {
		t.Fatalf("UpdateRun workflow failed: %v", err)
	}

	service := NewUserTaskProjectionService(controller, nil)
	projections, err := service.List(ctx, UserTaskProjectionFilter{
		UserID:         "user-1",
		ConversationID: "conv-workflow",
		Scope:          "current",
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(projections) != 1 {
		t.Fatalf("len(projections) = %d, want 1", len(projections))
	}
	if projections[0].Kind != RunKindWorkflow {
		t.Fatalf("projection kind = %q, want %q", projections[0].Kind, RunKindWorkflow)
	}
	if projections[0].Subtitle != "Nightly Sync • pause_for_approval • approval_needed" {
		t.Fatalf("subtitle = %q, want workflow subtitle", projections[0].Subtitle)
	}
	if len(projections[0].Actions.Items) != 2 {
		t.Fatalf("expected workflow task descriptors, got %#v", projections[0].Actions.Items)
	}
	if projections[0].Actions.Items[1].ID != "resume" || projections[0].Actions.Items[1].Path != "/tasks/"+run.ID+"/actions/resume" {
		t.Fatalf("expected workflow resume descriptor, got %#v", projections[0].Actions.Items[1])
	}
	if projections[0].Actions.Items[0].Variant != "danger" || projections[0].Actions.Items[1].Variant != "primary" {
		t.Fatalf("expected workflow action variants danger/primary, got %#v", projections[0].Actions.Items)
	}
	if !projections[0].Actions.Items[1].RequiresInput {
		t.Fatalf("expected workflow resume descriptor to require input, got %#v", projections[0].Actions.Items[1])
	}
	if projections[0].Actions.Items[1].Input.Title == "" || projections[0].Actions.Items[1].Input.SubmitLabel != "Submit decision" {
		t.Fatalf("expected workflow resume dialog copy, got %#v", projections[0].Actions.Items[1].Input)
	}
	if len(projections[0].Actions.Items[1].Input.Fields) != 2 {
		t.Fatalf("expected workflow resume input fields, got %#v", projections[0].Actions.Items[1].Input.Fields)
	}
	if projections[0].Actions.Items[1].Input.Fields[0].Kind != "choice" || len(projections[0].Actions.Items[1].Input.Fields[0].Options) != 2 || projections[0].Actions.Items[1].Input.Fields[0].Options[0] != "approve" {
		t.Fatalf("expected workflow resume decision schema, got %#v", projections[0].Actions.Items[1].Input.Fields)
	}
	if projections[0].Actions.Items[1].Input.Fields[1].Kind != "textarea" || projections[0].Actions.Items[1].Input.Fields[1].PayloadKey != "comment" {
		t.Fatalf("expected workflow resume payload schema, got %#v", projections[0].Actions.Items[1].Input.Fields)
	}
}

func TestUserTaskProjectionHandler_ListAndCancel(t *testing.T) {
	controller := newTestController(t)
	driver := &stubDriver{kind: RunKindAgentTask}
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:           RunKindAgentTask,
		Goal:           "Finish migration",
		UserID:         "user-1",
		ConversationID: "conv-1",
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	run.Status = RunStatusExecuting
	run.Progress = 50
	if err := controller.store.UpdateRun(context.Background(), run); err != nil {
		t.Fatalf("UpdateRun failed: %v", err)
	}
	if _, err := controller.Submit(context.Background(), RunSpec{
		Kind:           RunKindAgentTask,
		Goal:           "Someone else's task",
		UserID:         "user-2",
		ConversationID: "conv-2",
	}); err != nil {
		t.Fatalf("Submit other task failed: %v", err)
	}

	handler := NewUserTaskProjectionHandler(controller, nil)
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/tasks?scope=current&conversation_id=conv-1", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-1"}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := handler.ListTasks(c); err != nil {
		t.Fatalf("ListTasks returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("ListTasks status = %d, want %d", rec.Code, http.StatusOK)
	}
	var projections []UserTaskProjection
	if err := json.Unmarshal(rec.Body.Bytes(), &projections); err != nil {
		t.Fatalf("unmarshal ListTasks response: %v", err)
	}
	if len(projections) != 1 || projections[0].ID != run.ID {
		t.Fatalf("unexpected projections: %#v", projections)
	}

	cancelReq := httptest.NewRequest(http.MethodPost, "/tasks/"+run.ID+"/cancel", nil)
	cancelReq = cancelReq.WithContext(context.WithValue(cancelReq.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-1"}))
	cancelRec := httptest.NewRecorder()
	cancelCtx := e.NewContext(cancelReq, cancelRec)
	cancelCtx.SetParamNames("id")
	cancelCtx.SetParamValues(run.ID)
	if err := handler.CancelTask(cancelCtx); err != nil {
		t.Fatalf("CancelTask returned error: %v", err)
	}
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("CancelTask status = %d, want %d", cancelRec.Code, http.StatusOK)
	}
	var cancelled UserTaskProjection
	if err := json.Unmarshal(cancelRec.Body.Bytes(), &cancelled); err != nil {
		t.Fatalf("unmarshal CancelTask response: %v", err)
	}
	if cancelled.Status != "cancelled" || cancelled.Stage != "cancelled" {
		t.Fatalf("unexpected cancelled projection: %#v", cancelled)
	}
	if len(driver.cancelled) != 1 || driver.cancelled[0] != run.ID {
		t.Fatalf("driver cancel calls = %#v, want [%q]", driver.cancelled, run.ID)
	}
}

func TestUserTaskProjectionService_ProjectsAutoHarnessGroups(t *testing.T) {
	controller := newTestController(t)
	ctx := context.Background()

	group, err := controller.SubmitGroup(ctx, RunGroupSpec{
		Kind:        RunGroupKindEval,
		Title:       "Release candidate fix Smoke Auto Harness",
		Subject:     "agent_task",
		OwnerUserID: "user-1",
		Metadata: map[string]interface{}{
			"auto_harness":       true,
			"conversation_id":    "conv-current",
			"quick_eval_preset":  "smoke",
			"conversation_title": "Release candidate fix",
		},
		Items: []RunGroupItemSpec{{
			RunKind: RunKindAgentTask,
			Profile: "smoke",
			Input: map[string]interface{}{
				"goal": "Finish and verify the fix",
			},
			Expected: map[string]interface{}{
				"status": "completed",
			},
		}},
	})
	if err != nil {
		t.Fatalf("SubmitGroup failed: %v", err)
	}
	items, err := controller.store.ListGroupItems(ctx, group.ID)
	if err != nil {
		t.Fatalf("ListGroupItems failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("group items len = %d, want 1", len(items))
	}
	items[0].Status = RunGroupItemStatusRunning
	items[0].AttemptCount = 1
	if err := controller.store.UpdateGroupItem(ctx, &items[0]); err != nil {
		t.Fatalf("UpdateGroupItem running failed: %v", err)
	}
	group.Status = RunGroupStatusRunning
	group.Summary = map[string]interface{}{
		"item_count": 1,
		"counts": map[string]interface{}{
			"running": 1,
		},
	}
	if err := controller.store.UpdateGroup(ctx, group); err != nil {
		t.Fatalf("UpdateGroup running failed: %v", err)
	}

	service := NewUserTaskProjectionService(controller, nil)
	current, err := service.List(ctx, UserTaskProjectionFilter{
		UserID:         "user-1",
		ConversationID: "conv-current",
		Scope:          "current",
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("List current failed: %v", err)
	}
	if len(current) != 1 {
		t.Fatalf("current len = %d, want 1", len(current))
	}
	if current[0].ID != group.ID {
		t.Fatalf("current[0].ID = %q, want %q", current[0].ID, group.ID)
	}
	if current[0].Stage != "verifying" || current[0].Status != "running" {
		t.Fatalf("unexpected current projection: %#v", current[0])
	}
	if len(current[0].Actions.Items) != 1 || current[0].Actions.Items[0].ID != "cancel" {
		t.Fatalf("expected auto harness group cancel descriptor, got %#v", current[0].Actions.Items)
	}
	if len(current[0].Artifacts) != 1 || current[0].Artifacts[0].URL != "/harness/"+group.ID {
		t.Fatalf("artifacts = %#v, want harness report link", current[0].Artifacts)
	}

	items[0].Status = RunGroupItemStatusFailed
	items[0].AttemptCount = 1
	if err := controller.store.UpdateGroupItem(ctx, &items[0]); err != nil {
		t.Fatalf("UpdateGroupItem failed: %v", err)
	}

	current, err = service.List(ctx, UserTaskProjectionFilter{
		UserID:         "user-1",
		ConversationID: "conv-current",
		Scope:          "current",
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("List current terminal failed: %v", err)
	}
	if len(current) != 1 {
		t.Fatalf("terminal current len = %d, want 1", len(current))
	}
	if current[0].Stage != "failed" || current[0].Status != "failed" {
		t.Fatalf("unexpected failed projection: %#v", current[0])
	}
	if current[0].ErrorPreview == "" {
		t.Fatalf("expected failed auto harness preview")
	}
}

func TestUserTaskProjectionHandler_GetTaskRejectsOtherUsersAutoHarnessGroup(t *testing.T) {
	controller := newTestController(t)
	ctx := context.Background()

	group, err := controller.SubmitGroup(ctx, RunGroupSpec{
		Kind:        RunGroupKindEval,
		Title:       "Private Auto Harness",
		Subject:     "agent_task",
		OwnerUserID: "user-1",
		Metadata: map[string]interface{}{
			"auto_harness":    true,
			"conversation_id": "conv-private",
		},
		Items: []RunGroupItemSpec{{
			RunKind: RunKindAgentTask,
			Profile: "smoke",
			Input: map[string]interface{}{
				"goal": "Verify private task",
			},
		}},
	})
	if err != nil {
		t.Fatalf("SubmitGroup failed: %v", err)
	}

	handler := NewUserTaskProjectionHandler(controller, nil)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/tasks/"+group.ID+"?conversation_id=conv-private", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-2"}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(group.ID)

	if err := handler.GetTask(c); err != nil {
		t.Fatalf("GetTask returned error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GetTask status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestUserTaskProjectionHandler_CancelsAutoHarnessGroup(t *testing.T) {
	controller := newTestController(t)
	ctx := context.Background()

	group, err := controller.SubmitGroup(ctx, RunGroupSpec{
		Kind:        RunGroupKindEval,
		Title:       "Research Auto Harness",
		Subject:     "research",
		OwnerUserID: "user-1",
		Metadata: map[string]interface{}{
			"auto_harness":      true,
			"conversation_id":   "conv-1",
			"quick_eval_preset": "research",
		},
		Items: []RunGroupItemSpec{{
			RunKind: RunKindResearch,
			Profile: "research",
			Input: map[string]interface{}{
				"goal": "Investigate the regression",
			},
			Expected: map[string]interface{}{
				"status": "completed",
			},
		}},
	})
	if err != nil {
		t.Fatalf("SubmitGroup failed: %v", err)
	}
	group.Status = RunGroupStatusRunning
	group.Summary = map[string]interface{}{
		"item_count": 1,
		"counts": map[string]interface{}{
			"running": 1,
		},
	}
	if err := controller.store.UpdateGroup(ctx, group); err != nil {
		t.Fatalf("UpdateGroup failed: %v", err)
	}

	handler := NewUserTaskProjectionHandler(controller, nil)
	e := echo.New()

	cancelReq := httptest.NewRequest(http.MethodPost, "/tasks/"+group.ID+"/cancel?conversation_id=conv-1", nil)
	cancelReq = cancelReq.WithContext(context.WithValue(cancelReq.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-1"}))
	cancelRec := httptest.NewRecorder()
	cancelCtx := e.NewContext(cancelReq, cancelRec)
	cancelCtx.SetParamNames("id")
	cancelCtx.SetParamValues(group.ID)
	if err := handler.CancelTask(cancelCtx); err != nil {
		t.Fatalf("CancelTask returned error: %v", err)
	}
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("CancelTask status = %d, want %d", cancelRec.Code, http.StatusOK)
	}
	var cancelled UserTaskProjection
	if err := json.Unmarshal(cancelRec.Body.Bytes(), &cancelled); err != nil {
		t.Fatalf("unmarshal CancelTask response: %v", err)
	}
	if cancelled.ID != group.ID || cancelled.Status != "cancelled" || cancelled.Stage != "cancelled" {
		t.Fatalf("unexpected cancelled projection: %#v", cancelled)
	}
}

func TestUserTaskProjectionHandler_ResumesWorkflowTask(t *testing.T) {
	controller := newTestController(t)
	driver := &stubDriver{
		kind: RunKindWorkflow,
		actionResults: map[string]RunStatus{
			"resume": RunStatusExecuting,
		},
	}
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:           RunKindWorkflow,
		Goal:           "Resume workflow approval",
		UserID:         "user-1",
		ConversationID: "conv-1",
		Metadata: map[string]interface{}{
			"workflow_name":            "Approval Workflow",
			"workflow_execution_id":    "exec-1",
			"workflow_status_reason":   "approval_needed",
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

	handler := NewUserTaskProjectionHandler(controller, nil)
	e := echo.New()

	body := strings.NewReader(`{"decision":"approve","payload":{"ticket":"A-1"}}`)
	req := httptest.NewRequest(http.MethodPost, "/tasks/"+run.ID+"/resume?conversation_id=conv-1", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-1"}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(run.ID)

	if err := handler.ResumeTask(c); err != nil {
		t.Fatalf("ResumeTask returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("ResumeTask status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resumed UserTaskProjection
	if err := json.Unmarshal(rec.Body.Bytes(), &resumed); err != nil {
		t.Fatalf("unmarshal ResumeTask response: %v", err)
	}
	if resumed.ID != run.ID || resumed.Status != "running" || resumed.Stage != "working" {
		t.Fatalf("unexpected resumed projection: %#v", resumed)
	}
	if len(driver.actions) != 1 || driver.actions[0] != "resume" {
		t.Fatalf("driver actions = %#v, want [resume]", driver.actions)
	}
	if len(driver.actionInputs) != 1 {
		t.Fatalf("action inputs = %#v, want one payload", driver.actionInputs)
	}
	if got, _ := driver.actionInputs[0]["decision"].(string); got != "approve" {
		t.Fatalf("decision = %q, want approve", got)
	}
	payload, _ := driver.actionInputs[0]["payload"].(map[string]interface{})
	if got, _ := payload["ticket"].(string); got != "A-1" {
		t.Fatalf("payload.ticket = %q, want A-1", got)
	}
}

func TestUserTaskProjectionHandler_PerformTaskActionResumesWorkflowTask(t *testing.T) {
	controller := newTestController(t)
	driver := &stubDriver{
		kind: RunKindWorkflow,
		actionResults: map[string]RunStatus{
			"resume": RunStatusExecuting,
		},
	}
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), RunSpec{
		Kind:           RunKindWorkflow,
		Goal:           "Resume workflow approval",
		UserID:         "user-1",
		ConversationID: "conv-1",
		Metadata: map[string]interface{}{
			"workflow_name":            "Approval Workflow",
			"workflow_execution_id":    "exec-1",
			"workflow_status_reason":   "approval_needed",
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

	handler := NewUserTaskProjectionHandler(controller, nil)
	e := echo.New()

	body := strings.NewReader(`{"decision":"approve","payload":{"ticket":"A-2"}}`)
	req := httptest.NewRequest(http.MethodPost, "/tasks/"+run.ID+"/actions/resume?conversation_id=conv-1", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-1"}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id", "action")
	c.SetParamValues(run.ID, "resume")

	if err := handler.PerformTaskAction(c); err != nil {
		t.Fatalf("PerformTaskAction returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("PerformTaskAction status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resumed UserTaskProjection
	if err := json.Unmarshal(rec.Body.Bytes(), &resumed); err != nil {
		t.Fatalf("unmarshal PerformTaskAction response: %v", err)
	}
	if resumed.ID != run.ID || resumed.Status != "running" || resumed.Stage != "working" {
		t.Fatalf("unexpected resumed projection: %#v", resumed)
	}
	if len(driver.actions) != 1 || driver.actions[0] != "resume" {
		t.Fatalf("driver actions = %#v, want [resume]", driver.actions)
	}
	payload, _ := driver.actionInputs[0]["payload"].(map[string]interface{})
	if got, _ := payload["ticket"].(string); got != "A-2" {
		t.Fatalf("payload.ticket = %q, want A-2", got)
	}
}

func TestUserTaskProjectionHandler_PerformTaskActionCancelsAutoHarnessGroup(t *testing.T) {
	controller := newTestController(t)
	ctx := context.Background()

	group, err := controller.SubmitGroup(ctx, RunGroupSpec{
		Kind:        RunGroupKindEval,
		Title:       "Research Auto Harness",
		Subject:     "research",
		OwnerUserID: "user-1",
		Metadata: map[string]interface{}{
			"auto_harness":      true,
			"conversation_id":   "conv-1",
			"quick_eval_preset": "research",
		},
		Items: []RunGroupItemSpec{{
			RunKind: RunKindResearch,
			Profile: "research",
			Input: map[string]interface{}{
				"goal": "Investigate the regression",
			},
			Expected: map[string]interface{}{
				"status": "completed",
			},
		}},
	})
	if err != nil {
		t.Fatalf("SubmitGroup failed: %v", err)
	}
	group.Status = RunGroupStatusRunning
	group.Summary = map[string]interface{}{
		"item_count": 1,
		"counts": map[string]interface{}{
			"running": 1,
		},
	}
	if err := controller.store.UpdateGroup(ctx, group); err != nil {
		t.Fatalf("UpdateGroup failed: %v", err)
	}

	handler := NewUserTaskProjectionHandler(controller, nil)
	e := echo.New()

	req := httptest.NewRequest(http.MethodPost, "/tasks/"+group.ID+"/actions/cancel?conversation_id=conv-1", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-1"}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id", "action")
	c.SetParamValues(group.ID, "cancel")
	if err := handler.PerformTaskAction(c); err != nil {
		t.Fatalf("PerformTaskAction returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("PerformTaskAction status = %d, want %d", rec.Code, http.StatusOK)
	}
	var cancelled UserTaskProjection
	if err := json.Unmarshal(rec.Body.Bytes(), &cancelled); err != nil {
		t.Fatalf("unmarshal PerformTaskAction response: %v", err)
	}
	if cancelled.ID != group.ID || cancelled.Status != "cancelled" || cancelled.Stage != "cancelled" {
		t.Fatalf("unexpected cancelled projection: %#v", cancelled)
	}
}
