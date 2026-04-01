package drivers

import (
	"context"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
)

type stubWorkflowRuntime struct {
	executeExecution *workflow.Execution
	executeErr       error
	getExecution     *workflow.Execution
	getErr           error
	cancelErr        error
	resumeExecution  *workflow.Execution
	resumeErr        error
	logs             []*workflow.ExecutionLog
	logsErr          error

	executeCalls []stubWorkflowExecuteCall
	cancelCalls  []stubWorkflowCancelCall
	resumeCalls  []stubWorkflowResumeCall
	logCalls     []stubWorkflowLogCall
}

type stubWorkflowExecuteCall struct {
	ctx         context.Context
	workflowID  string
	triggerType workflow.TriggerType
	triggerData map[string]interface{}
}

type stubWorkflowCancelCall struct {
	ctx         context.Context
	executionID string
}

type stubWorkflowResumeCall struct {
	ctx         context.Context
	executionID string
	resume      workflow.ExecutionResumeInput
}

type stubWorkflowLogCall struct {
	ctx         context.Context
	executionID string
	offset      int
	limit       int
}

func (s *stubWorkflowRuntime) ExecuteWorkflow(ctx context.Context, id string, triggerData map[string]interface{}) (*workflow.Execution, error) {
	s.executeCalls = append(s.executeCalls, stubWorkflowExecuteCall{
		ctx:         ctx,
		workflowID:  id,
		triggerType: workflow.TriggerTypeManual,
		triggerData: triggerData,
	})
	if s.executeErr != nil {
		return nil, s.executeErr
	}
	return s.executeExecution, nil
}

func (s *stubWorkflowRuntime) ExecuteWorkflowWithTrigger(ctx context.Context, id string, triggerType workflow.TriggerType, triggerData map[string]interface{}) (*workflow.Execution, error) {
	s.executeCalls = append(s.executeCalls, stubWorkflowExecuteCall{
		ctx:         ctx,
		workflowID:  id,
		triggerType: triggerType,
		triggerData: triggerData,
	})
	if s.executeErr != nil {
		return nil, s.executeErr
	}
	return s.executeExecution, nil
}

func (s *stubWorkflowRuntime) GetExecution(_ context.Context, _ string) (*workflow.Execution, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.getExecution != nil {
		return s.getExecution, nil
	}
	return s.executeExecution, nil
}

func (s *stubWorkflowRuntime) CancelExecution(ctx context.Context, id string) error {
	s.cancelCalls = append(s.cancelCalls, stubWorkflowCancelCall{
		ctx:         ctx,
		executionID: id,
	})
	return s.cancelErr
}

func (s *stubWorkflowRuntime) ResumeExecutionDirect(ctx context.Context, id string, resume workflow.ExecutionResumeInput) (*workflow.Execution, error) {
	s.resumeCalls = append(s.resumeCalls, stubWorkflowResumeCall{
		ctx:         ctx,
		executionID: id,
		resume:      resume,
	})
	if s.resumeErr != nil {
		return nil, s.resumeErr
	}
	if s.resumeExecution != nil {
		s.getExecution = s.resumeExecution
	}
	return s.resumeExecution, nil
}

func (s *stubWorkflowRuntime) GetExecutionLogs(ctx context.Context, executionID string, opts *workflow.ListOptions) ([]*workflow.ExecutionLog, int, error) {
	if opts == nil {
		opts = &workflow.ListOptions{}
	}
	s.logCalls = append(s.logCalls, stubWorkflowLogCall{
		ctx:         ctx,
		executionID: executionID,
		offset:      opts.Offset,
		limit:       opts.Limit,
	})
	if s.logsErr != nil {
		return nil, 0, s.logsErr
	}
	if opts.Limit <= 0 {
		opts.Limit = len(s.logs)
	}
	if opts.Offset >= len(s.logs) {
		return nil, len(s.logs), nil
	}
	end := opts.Offset + opts.Limit
	if end > len(s.logs) {
		end = len(s.logs)
	}
	return s.logs[opts.Offset:end], len(s.logs), nil
}

func TestWorkflowDriverStartPersistsWorkflowExecutionMapping(t *testing.T) {
	controller := newDriverTestController(t)
	now := time.Now()
	runtime := &stubWorkflowRuntime{
		executeExecution: &workflow.Execution{
			ID:           "exec-1",
			WorkflowID:   "wf-1",
			WorkflowName: "Nightly Sync",
			TenantID:     "tenant-1",
			Status:       workflow.ExecutionStatusRunning,
			StatusReason: "node_running",
			StartedAt:    now,
		},
	}
	driver := NewWorkflowDriver(runtime, controller)
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), harness.RunSpec{
		Kind:           harness.RunKindWorkflow,
		Goal:           "Run nightly sync",
		UserID:         "user-1",
		ConversationID: "conv-1",
		Metadata: map[string]interface{}{
			"workflow_id": "wf-1",
			"tenant_id":   "tenant-1",
			"trigger_data": map[string]interface{}{
				"source": "manual",
			},
		},
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if len(runtime.executeCalls) != 1 {
		t.Fatalf("ExecuteWorkflow calls = %d, want 1", len(runtime.executeCalls))
	}
	if runtime.executeCalls[0].workflowID != "wf-1" {
		t.Fatalf("workflowID = %q, want wf-1", runtime.executeCalls[0].workflowID)
	}
	if runtime.executeCalls[0].triggerType != workflow.TriggerTypeManual {
		t.Fatalf("triggerType = %q, want %q", runtime.executeCalls[0].triggerType, workflow.TriggerTypeManual)
	}
	if got := workflow.TenantFromContext(runtime.executeCalls[0].ctx); got != "tenant-1" {
		t.Fatalf("tenant context = %q, want tenant-1", got)
	}
	if got, _ := runtime.executeCalls[0].triggerData["source"].(string); got != "manual" {
		t.Fatalf("trigger_data.source = %q, want manual", got)
	}
	if run.Status != harness.RunStatusExecuting {
		t.Fatalf("run.Status = %q, want %q", run.Status, harness.RunStatusExecuting)
	}
	if got := metadataString(run.Metadata, workflowExecutionIDMetadataKey); got != "exec-1" {
		t.Fatalf("workflow_execution_id = %q, want exec-1", got)
	}
	if got := metadataString(run.Metadata, workflowNameMetadataKey); got != "Nightly Sync" {
		t.Fatalf("workflow_name = %q, want Nightly Sync", got)
	}
	if got := metadataString(run.Metadata, workflowTriggerTypeMetadataKey); got != string(workflow.TriggerTypeManual) {
		t.Fatalf("trigger_type = %q, want %q", got, workflow.TriggerTypeManual)
	}
}

func TestWorkflowDriverStartUsesTriggerTypeMetadataWhenPresent(t *testing.T) {
	controller := newDriverTestController(t)
	runtime := &stubWorkflowRuntime{
		executeExecution: &workflow.Execution{
			ID:           "exec-hook-1",
			WorkflowID:   "wf-hook-1",
			WorkflowName: "Webhook Workflow",
			TenantID:     "tenant-webhook",
			Status:       workflow.ExecutionStatusRunning,
			TriggerType:  workflow.TriggerTypeWebhook,
			StartedAt:    time.Now(),
		},
	}
	driver := NewWorkflowDriver(runtime, controller)
	controller.RegisterDriver(driver)

	_, err := controller.Submit(context.Background(), harness.RunSpec{
		Kind: harness.RunKindWorkflow,
		Goal: "Webhook workflow",
		Metadata: map[string]interface{}{
			"workflow_id":  "wf-hook-1",
			"tenant_id":    "tenant-webhook",
			"trigger_type": string(workflow.TriggerTypeWebhook),
		},
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if len(runtime.executeCalls) != 1 {
		t.Fatalf("ExecuteWorkflow calls = %d, want 1", len(runtime.executeCalls))
	}
	if runtime.executeCalls[0].triggerType != workflow.TriggerTypeWebhook {
		t.Fatalf("triggerType = %q, want %q", runtime.executeCalls[0].triggerType, workflow.TriggerTypeWebhook)
	}
}

func TestWorkflowDriverSyncMapsPausedExecutionToWaitingInput(t *testing.T) {
	controller := newDriverTestController(t)
	now := time.Now()
	runtime := &stubWorkflowRuntime{
		executeExecution: &workflow.Execution{
			ID:         "exec-2",
			WorkflowID: "wf-2",
			TenantID:   "tenant-2",
			Status:     workflow.ExecutionStatusRunning,
			StartedAt:  now,
		},
	}
	driver := NewWorkflowDriver(runtime, controller)
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), harness.RunSpec{
		Kind:           harness.RunKindWorkflow,
		Goal:           "Run approval workflow",
		UserID:         "user-2",
		ConversationID: "conv-2",
		Metadata: map[string]interface{}{
			"workflow_id": "wf-2",
			"tenant_id":   "tenant-2",
		},
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	runtime.getExecution = &workflow.Execution{
		ID:           "exec-2",
		WorkflowID:   "wf-2",
		WorkflowName: "Approval Workflow",
		TenantID:     "tenant-2",
		Status:       workflow.ExecutionStatusPaused,
		StatusReason: "approval_needed",
		Checkpoint: &workflow.ExecutionCheckpoint{
			Kind:     workflow.ExecutionCheckpointPauseForApproval,
			Reason:   "approval_needed",
			NodeName: "Manager Review",
		},
		StartedAt: now,
	}

	synced, err := controller.Get(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if synced.Status != harness.RunStatusWaitingInput {
		t.Fatalf("synced.Status = %q, want %q", synced.Status, harness.RunStatusWaitingInput)
	}
	if got := metadataString(synced.Metadata, workflowCheckpointKindMetadataKey); got != string(workflow.ExecutionCheckpointPauseForApproval) {
		t.Fatalf("workflow_checkpoint_kind = %q, want %q", got, workflow.ExecutionCheckpointPauseForApproval)
	}
	if got := metadataString(synced.Metadata, workflowStatusReasonMetadataKey); got != "approval_needed" {
		t.Fatalf("workflow_status_reason = %q, want approval_needed", got)
	}
	if got := metadataString(synced.Metadata, workflowCheckpointReasonMetadataKey); got != "approval_needed" {
		t.Fatalf("workflow_checkpoint_reason = %q, want approval_needed", got)
	}
	if got := metadataString(synced.Metadata, workflowCheckpointNodeNameMetadataKey); got != "Manager Review" {
		t.Fatalf("workflow_checkpoint_node_name = %q, want Manager Review", got)
	}
}

func TestWorkflowDriverPerformActionResume(t *testing.T) {
	controller := newDriverTestController(t)
	runtime := &stubWorkflowRuntime{
		executeExecution: &workflow.Execution{
			ID:           "exec-3",
			WorkflowID:   "wf-3",
			WorkflowName: "Approval Workflow",
			TenantID:     "tenant-3",
			Status:       workflow.ExecutionStatusPaused,
			StatusReason: "approval_needed",
			Checkpoint: &workflow.ExecutionCheckpoint{
				Kind: workflow.ExecutionCheckpointPauseForApproval,
			},
			StartedAt: time.Now(),
		},
		resumeExecution: &workflow.Execution{
			ID:           "exec-3",
			WorkflowID:   "wf-3",
			WorkflowName: "Approval Workflow",
			TenantID:     "tenant-3",
			Status:       workflow.ExecutionStatusRunning,
			StatusReason: string(workflow.ExecutionCheckpointResumeWithDecision),
			Checkpoint: &workflow.ExecutionCheckpoint{
				Kind: workflow.ExecutionCheckpointPauseForApproval,
				Resume: &workflow.ExecutionResumeInput{
					Decision: "approve",
				},
			},
			StartedAt: time.Now(),
		},
	}
	driver := NewWorkflowDriver(runtime, controller)
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), harness.RunSpec{
		Kind: harness.RunKindWorkflow,
		Goal: "Resume approval workflow",
		Metadata: map[string]interface{}{
			"workflow_id": "wf-3",
			"tenant_id":   "tenant-3",
		},
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if run.Status != harness.RunStatusWaitingInput {
		t.Fatalf("run.Status = %q, want %q", run.Status, harness.RunStatusWaitingInput)
	}

	updated, err := controller.PerformAction(context.Background(), run.ID, "resume", map[string]interface{}{
		"decision": "approve",
		"payload": map[string]interface{}{
			"ticket": "A-3",
		},
	})
	if err != nil {
		t.Fatalf("PerformAction(resume) failed: %v", err)
	}
	if len(runtime.resumeCalls) != 1 {
		t.Fatalf("ResumeExecutionDirect calls = %d, want 1", len(runtime.resumeCalls))
	}
	if runtime.resumeCalls[0].executionID != "exec-3" {
		t.Fatalf("executionID = %q, want exec-3", runtime.resumeCalls[0].executionID)
	}
	if got := workflow.TenantFromContext(runtime.resumeCalls[0].ctx); got != "tenant-3" {
		t.Fatalf("tenant context = %q, want tenant-3", got)
	}
	if runtime.resumeCalls[0].resume.Decision != "approve" {
		t.Fatalf("decision = %q, want approve", runtime.resumeCalls[0].resume.Decision)
	}
	if got, _ := runtime.resumeCalls[0].resume.Payload["ticket"].(string); got != "A-3" {
		t.Fatalf("payload.ticket = %q, want A-3", got)
	}
	if updated == nil || updated.Status != harness.RunStatusExecuting {
		t.Fatalf("updated run = %#v, want executing workflow snapshot", updated)
	}
	if got := metadataString(updated.Metadata, workflowStatusReasonMetadataKey); got != string(workflow.ExecutionCheckpointResumeWithDecision) {
		t.Fatalf("workflow_status_reason = %q, want %q", got, workflow.ExecutionCheckpointResumeWithDecision)
	}
}

func TestWorkflowDriverCancelUsesMappedExecutionID(t *testing.T) {
	controller := newDriverTestController(t)
	runtime := &stubWorkflowRuntime{
		executeExecution: &workflow.Execution{
			ID:         "exec-3",
			WorkflowID: "wf-3",
			TenantID:   "tenant-3",
			Status:     workflow.ExecutionStatusRunning,
			StartedAt:  time.Now(),
		},
	}
	driver := NewWorkflowDriver(runtime, controller)
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), harness.RunSpec{
		Kind:           harness.RunKindWorkflow,
		Goal:           "Run cancel workflow",
		UserID:         "user-3",
		ConversationID: "conv-3",
		Metadata: map[string]interface{}{
			"workflow_id": "wf-3",
			"tenant_id":   "tenant-3",
		},
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	if err := controller.Cancel(context.Background(), run.ID, "user cancelled"); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}
	if len(runtime.cancelCalls) != 1 {
		t.Fatalf("CancelExecution calls = %d, want 1", len(runtime.cancelCalls))
	}
	if runtime.cancelCalls[0].executionID != "exec-3" {
		t.Fatalf("executionID = %q, want exec-3", runtime.cancelCalls[0].executionID)
	}
	if got := workflow.TenantFromContext(runtime.cancelCalls[0].ctx); got != "tenant-3" {
		t.Fatalf("tenant context = %q, want tenant-3", got)
	}
}

func TestWorkflowDriverListRuntimeEvidenceMapsExecutionLogs(t *testing.T) {
	controller := newDriverTestController(t)
	runtime := &stubWorkflowRuntime{
		logs: []*workflow.ExecutionLog{
			{
				ID:          "log-1",
				ExecutionID: "exec-4",
				NodeID:      "node-a",
				Level:       "info",
				Message:     "started",
				Timestamp:   time.Now(),
			},
			{
				ID:          "log-2",
				ExecutionID: "exec-4",
				Level:       "error",
				Message:     "failed",
				Timestamp:   time.Now().Add(time.Second),
			},
		},
	}
	driver := NewWorkflowDriver(runtime, controller)

	run := &harness.Run{
		ID: "run-4",
		Metadata: map[string]interface{}{
			"workflow_execution_id": "exec-4",
			"tenant_id":             "tenant-4",
		},
	}
	evidence, err := driver.ListRuntimeEvidence(context.Background(), run)
	if err != nil {
		t.Fatalf("ListRuntimeEvidence failed: %v", err)
	}
	if len(evidence) != 2 {
		t.Fatalf("len(evidence) = %d, want 2", len(evidence))
	}
	if evidence[0].EventType != "workflow_log_info" || evidence[0].Summary != "node-a: started" {
		t.Fatalf("unexpected first evidence entry: %#v", evidence[0])
	}
	if evidence[1].EventType != "workflow_log_error" || evidence[1].Summary != "failed" {
		t.Fatalf("unexpected second evidence entry: %#v", evidence[1])
	}
	if len(runtime.logCalls) != 1 {
		t.Fatalf("GetExecutionLogs calls = %d, want 1", len(runtime.logCalls))
	}
	if got := workflow.TenantFromContext(runtime.logCalls[0].ctx); got != "tenant-4" {
		t.Fatalf("tenant context = %q, want tenant-4", got)
	}
}
