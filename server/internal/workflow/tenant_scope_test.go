package workflow

import (
	"context"
	"testing"
	"time"
)

func TestRepository_GetExecutionLogsHonorsTenantContext(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := NewRepository(db)
	if err != nil {
		t.Fatalf("NewRepository failed: %v", err)
	}

	ctx := context.Background()
	execution := &Execution{
		ID:           "exec-tenant-2",
		WorkflowID:   "wf-tenant-2",
		WorkflowName: "Tenant Two Workflow",
		TenantID:     "tenant-2",
		Status:       ExecutionStatusCompleted,
		TriggerType:  TriggerTypeManual,
		StartedAt:    time.Now(),
	}
	if err := repo.SaveExecution(ctx, execution); err != nil {
		t.Fatalf("SaveExecution failed: %v", err)
	}
	if err := repo.SaveExecutionLog(ctx, &ExecutionLog{
		ID:          "log-1",
		ExecutionID: execution.ID,
		Level:       "info",
		Message:     "completed",
		Timestamp:   time.Now(),
	}); err != nil {
		t.Fatalf("SaveExecutionLog failed: %v", err)
	}

	_, _, err = repo.GetExecutionLogs(withWorkflowTenant(ctx, "tenant-1"), execution.ID, &ListOptions{Limit: 10})
	if err != ErrExecutionNotFound {
		t.Fatalf("GetExecutionLogs cross-tenant error = %v, want ErrExecutionNotFound", err)
	}
}

func TestWorkflowServiceGetExecutionHonorsTenantForRunningExecution(t *testing.T) {
	service := &WorkflowService{
		config: DefaultConfig(),
		engine: NewEngine(DefaultConfig()),
	}

	execution := &Execution{
		ID:           "running-exec",
		WorkflowID:   "wf-tenant-2",
		WorkflowName: "Tenant Two Workflow",
		TenantID:     "tenant-2",
		Status:       ExecutionStatusRunning,
		TriggerType:  TriggerTypeManual,
		StartedAt:    time.Now(),
	}
	service.engine.executions[execution.ID] = &executionState{execution: execution}

	_, err := service.GetExecution(withWorkflowTenant(context.Background(), "tenant-1"), execution.ID)
	if err != ErrExecutionNotFound {
		t.Fatalf("GetExecution cross-tenant error = %v, want ErrExecutionNotFound", err)
	}
}
