package workflow

import (
	"context"
	"testing"
)

func TestWorkflowServiceHandleWebhookUsesExecutionLauncher(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := NewRepository(db)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	service, err := NewService(nil, repo)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	defer service.Close()

	ctx := withWorkflowTenant(context.Background(), "tenant-1")
	item := &Workflow{
		ID:       "wf-webhook",
		TenantID: "tenant-1",
		Name:     "Webhook Workflow",
		Status:   WorkflowStatusActive,
		Nodes: []Node{
			{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"},
		},
	}
	if err := repo.CreateWorkflow(ctx, item); err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	if err := repo.SaveWebhook(ctx, item.ID, "/hook", "POST", "none", nil); err != nil {
		t.Fatalf("SaveWebhook() error = %v", err)
	}

	launcher := &stubExecutionLauncher{
		result: &Execution{
			ID:          "exec-hook",
			WorkflowID:  item.ID,
			TenantID:    "tenant-1",
			Status:      ExecutionStatusRunning,
			TriggerType: TriggerTypeWebhook,
		},
	}
	service.SetExecutionLauncher(launcher)

	execution, err := service.HandleWebhook(context.Background(), "/hook", "POST", map[string]string{"X-Test": "1"}, []byte("payload"))
	if err != nil {
		t.Fatalf("HandleWebhook() error = %v", err)
	}
	if launcher.calls != 1 {
		t.Fatalf("LaunchExecution calls = %d, want 1", launcher.calls)
	}
	if launcher.request.WorkflowID != item.ID {
		t.Fatalf("workflow_id = %q, want %q", launcher.request.WorkflowID, item.ID)
	}
	if launcher.request.TriggerType != TriggerTypeWebhook {
		t.Fatalf("trigger_type = %q, want %q", launcher.request.TriggerType, TriggerTypeWebhook)
	}
	if launcher.request.TenantID != "tenant-1" {
		t.Fatalf("tenant_id = %q, want tenant-1", launcher.request.TenantID)
	}
	if got, _ := launcher.request.TriggerData["path"].(string); got != "/hook" {
		t.Fatalf("trigger_data.path = %q, want /hook", got)
	}
	if execution == nil || execution.ID != "exec-hook" {
		t.Fatalf("unexpected execution: %#v", execution)
	}
}

func TestWorkflowServiceRetryExecutionUsesExecutionLauncher(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := NewRepository(db)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	service, err := NewService(nil, repo)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	defer service.Close()

	ctx := withWorkflowTenant(context.Background(), "tenant-1")
	item := &Workflow{
		ID:       "wf-retry",
		TenantID: "tenant-1",
		Name:     "Retry Workflow",
		Status:   WorkflowStatusActive,
		Nodes: []Node{
			{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"},
		},
	}
	if err := repo.CreateWorkflow(ctx, item); err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	failed := &Execution{
		ID:           "exec-failed",
		WorkflowID:   item.ID,
		WorkflowName: item.Name,
		TenantID:     "tenant-1",
		Status:       ExecutionStatusFailed,
		TriggerType:  TriggerTypeWebhook,
		TriggerData: map[string]interface{}{
			"source": "original",
		},
	}
	if err := repo.SaveExecution(ctx, failed); err != nil {
		t.Fatalf("SaveExecution() error = %v", err)
	}

	launcher := &stubExecutionLauncher{
		result: &Execution{
			ID:          "exec-retried",
			WorkflowID:  item.ID,
			TenantID:    "tenant-1",
			Status:      ExecutionStatusRunning,
			TriggerType: TriggerTypeManual,
		},
	}
	service.SetExecutionLauncher(launcher)

	retryCtx := withWorkflowConversation(withWorkflowUser(ctx, "user-1"), "conv-1")
	execution, err := service.RetryExecution(retryCtx, failed.ID)
	if err != nil {
		t.Fatalf("RetryExecution() error = %v", err)
	}
	if launcher.calls != 1 {
		t.Fatalf("LaunchExecution calls = %d, want 1", launcher.calls)
	}
	if launcher.request.WorkflowID != item.ID {
		t.Fatalf("workflow_id = %q, want %q", launcher.request.WorkflowID, item.ID)
	}
	if launcher.request.TriggerType != TriggerTypeManual {
		t.Fatalf("trigger_type = %q, want %q", launcher.request.TriggerType, TriggerTypeManual)
	}
	if launcher.request.UserID != "user-1" || launcher.request.ConversationID != "conv-1" {
		t.Fatalf("launcher request = %+v, want user/session propagated", launcher.request)
	}
	if got, _ := launcher.request.TriggerData["source"].(string); got != "original" {
		t.Fatalf("trigger_data.source = %q, want original", got)
	}
	if execution == nil || execution.ID != "exec-retried" {
		t.Fatalf("unexpected execution: %#v", execution)
	}
}

func TestWorkflowServiceCancelExecutionUsesExecutionLauncher(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := NewRepository(db)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	service, err := NewService(nil, repo)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	defer service.Close()

	ctx := withWorkflowTenant(context.Background(), "tenant-1")
	item := &Workflow{
		ID:       "wf-cancel",
		TenantID: "tenant-1",
		Name:     "Cancel Workflow",
		Status:   WorkflowStatusActive,
		Nodes: []Node{
			{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"},
		},
	}
	if err := repo.CreateWorkflow(ctx, item); err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	running := &Execution{
		ID:           "exec-running",
		WorkflowID:   item.ID,
		WorkflowName: item.Name,
		TenantID:     "tenant-1",
		Status:       ExecutionStatusRunning,
		TriggerType:  TriggerTypeManual,
	}
	if err := repo.SaveExecution(ctx, running); err != nil {
		t.Fatalf("SaveExecution() error = %v", err)
	}

	launcher := &stubExecutionLauncher{cancelHandled: true}
	service.SetExecutionLauncher(launcher)

	if err := service.CancelExecution(ctx, running.ID); err != nil {
		t.Fatalf("CancelExecution() error = %v", err)
	}
	if launcher.cancelCalls != 1 {
		t.Fatalf("CancelExecution launcher calls = %d, want 1", launcher.cancelCalls)
	}
	if launcher.cancelExecutionID != running.ID {
		t.Fatalf("cancel execution id = %q, want %q", launcher.cancelExecutionID, running.ID)
	}
}

func TestWorkflowServiceResumeExecutionUsesExecutionLauncher(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := NewRepository(db)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	service, err := NewService(nil, repo)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	defer service.Close()

	ctx := withWorkflowTenant(context.Background(), "tenant-1")
	item := &Workflow{
		ID:       "wf-resume",
		TenantID: "tenant-1",
		Name:     "Resume Workflow",
		Status:   WorkflowStatusActive,
		Nodes: []Node{
			{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"},
		},
	}
	if err := repo.CreateWorkflow(ctx, item); err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	paused := &Execution{
		ID:           "exec-paused",
		WorkflowID:   item.ID,
		WorkflowName: item.Name,
		TenantID:     "tenant-1",
		Status:       ExecutionStatusPaused,
		TriggerType:  TriggerTypeManual,
		Checkpoint: &ExecutionCheckpoint{
			Kind: ExecutionCheckpointPauseForApproval,
		},
	}
	if err := repo.SaveExecution(ctx, paused); err != nil {
		t.Fatalf("SaveExecution() error = %v", err)
	}

	launcher := &stubExecutionLauncher{
		resumeHandled: true,
		resumeResult: &Execution{
			ID:           paused.ID,
			WorkflowID:   item.ID,
			WorkflowName: item.Name,
			TenantID:     "tenant-1",
			Status:       ExecutionStatusRunning,
			StatusReason: string(ExecutionCheckpointResumeWithDecision),
		},
	}
	service.SetExecutionLauncher(launcher)

	resumed, err := service.ResumeExecution(ctx, paused.ID, ExecutionResumeInput{
		Decision: "approve",
		Payload: map[string]interface{}{
			"ticket": "A-1",
		},
	})
	if err != nil {
		t.Fatalf("ResumeExecution() error = %v", err)
	}
	if launcher.resumeCalls != 1 {
		t.Fatalf("ResumeExecution launcher calls = %d, want 1", launcher.resumeCalls)
	}
	if launcher.resumeExecutionID != paused.ID {
		t.Fatalf("resume execution id = %q, want %q", launcher.resumeExecutionID, paused.ID)
	}
	if launcher.resumeInput.Decision != "approve" {
		t.Fatalf("resume decision = %q, want approve", launcher.resumeInput.Decision)
	}
	if got, _ := launcher.resumeInput.Payload["ticket"].(string); got != "A-1" {
		t.Fatalf("resume payload ticket = %q, want A-1", got)
	}
	if resumed == nil || resumed.Status != ExecutionStatusRunning {
		t.Fatalf("unexpected resumed execution: %#v", resumed)
	}
}
