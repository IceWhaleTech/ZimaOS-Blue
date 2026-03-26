package harness

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	subagentDriver := &stubDriver{kind: RunKindSubagent}
	controller.RegisterDriver(agentDriver)
	controller.RegisterDriver(researchDriver)
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
	if !current[0].Actions.CanSendUpdate {
		t.Fatalf("expected CanSendUpdate for current agent task")
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
	if !background[0].Actions.CanOpenChat {
		t.Fatalf("expected background task to allow opening chat")
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
	if !current[0].Actions.CanCancel {
		t.Fatalf("expected auto harness group to be cancellable while active")
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
