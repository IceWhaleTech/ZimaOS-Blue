package workflow

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	tempDir := filepath.Join(os.TempDir(), "workflow-test-"+time.Now().Format("20060102150405"))
	os.MkdirAll(tempDir, 0755)

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(tempDir)
	}

	return db, cleanup
}

func TestRepository_CreateWorkflow(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := NewRepository(db)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	ctx := context.Background()

	workflow := &Workflow{
		TenantID:    "tenant-1",
		Name:        "Test Workflow",
		Description: "A test workflow",
		Status:      WorkflowStatusDraft,
		Nodes: []Node{
			{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"},
		},
		Connections: []Connection{},
		Variables:   map[string]string{"key": "value"},
		Tags:        []string{"test"},
	}

	err = repo.CreateWorkflow(ctx, workflow)
	if err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	if workflow.ID == "" {
		t.Error("expected workflow ID to be set")
	}

	// Verify workflow was created
	retrieved, err := repo.GetWorkflow(ctx, workflow.ID)
	if err != nil {
		t.Fatalf("failed to get workflow: %v", err)
	}

	if retrieved.Name != "Test Workflow" {
		t.Errorf("expected name 'Test Workflow', got '%s'", retrieved.Name)
	}

	if len(retrieved.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(retrieved.Nodes))
	}
}

func TestRepository_UpdateWorkflow(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewRepository(db)
	ctx := context.Background()

	workflow := &Workflow{
		TenantID: "tenant-1",
		Name:     "Original Name",
		Status:   WorkflowStatusDraft,
		Nodes:    []Node{{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"}},
	}

	repo.CreateWorkflow(ctx, workflow)

	// Update workflow
	workflow.Name = "Updated Name"
	workflow.Status = WorkflowStatusActive

	err := repo.UpdateWorkflow(ctx, workflow)
	if err != nil {
		t.Fatalf("failed to update workflow: %v", err)
	}

	// Verify update
	retrieved, _ := repo.GetWorkflow(ctx, workflow.ID)
	if retrieved.Name != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got '%s'", retrieved.Name)
	}
	if retrieved.Status != WorkflowStatusActive {
		t.Errorf("expected status active, got %s", retrieved.Status)
	}
	if retrieved.Version != 2 {
		t.Errorf("expected version 2, got %d", retrieved.Version)
	}
}

func TestRepository_DeleteWorkflow(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewRepository(db)
	ctx := context.Background()

	workflow := &Workflow{
		TenantID: "tenant-1",
		Name:     "To Delete",
		Status:   WorkflowStatusDraft,
		Nodes:    []Node{{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"}},
	}

	repo.CreateWorkflow(ctx, workflow)

	err := repo.DeleteWorkflow(ctx, workflow.ID)
	if err != nil {
		t.Fatalf("failed to delete workflow: %v", err)
	}

	// Verify deletion
	_, err = repo.GetWorkflow(ctx, workflow.ID)
	if err != ErrWorkflowNotFound {
		t.Errorf("expected ErrWorkflowNotFound, got %v", err)
	}
}

func TestRepository_ListWorkflows(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewRepository(db)
	ctx := context.Background()

	// Create multiple workflows
	for i := 0; i < 5; i++ {
		workflow := &Workflow{
			TenantID: "tenant-1",
			Name:     "Workflow " + string(rune('A'+i)),
			Status:   WorkflowStatusDraft,
			Nodes:    []Node{{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"}},
		}
		repo.CreateWorkflow(ctx, workflow)
	}

	// List all
	workflows, total, err := repo.ListWorkflows(ctx, "tenant-1", &ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("failed to list workflows: %v", err)
	}

	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}

	if len(workflows) != 5 {
		t.Errorf("expected 5 workflows, got %d", len(workflows))
	}

	// Test pagination
	workflows, _, _ = repo.ListWorkflows(ctx, "tenant-1", &ListOptions{Limit: 2, Offset: 0})
	if len(workflows) != 2 {
		t.Errorf("expected 2 workflows with limit, got %d", len(workflows))
	}

	// Test filtering by status
	workflows[0].Status = WorkflowStatusActive
	repo.UpdateWorkflow(ctx, workflows[0])

	workflows, total, _ = repo.ListWorkflows(ctx, "tenant-1", &ListOptions{
		Limit:   10,
		Filters: map[string]string{"status": "active"},
	})
	if total != 1 {
		t.Errorf("expected 1 active workflow, got %d", total)
	}
}

func TestRepository_SaveExecution(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewRepository(db)
	ctx := context.Background()

	// Create workflow first
	workflow := &Workflow{
		TenantID: "tenant-1",
		Name:     "Test Workflow",
		Status:   WorkflowStatusActive,
		Nodes:    []Node{{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"}},
	}
	repo.CreateWorkflow(ctx, workflow)

	// Create execution
	execution := &Execution{
		ID:           "exec-1",
		WorkflowID:   workflow.ID,
		WorkflowName: workflow.Name,
		TenantID:     "tenant-1",
		Status:       ExecutionStatusCompleted,
		TriggerType:  TriggerTypeManual,
		TriggerData:  map[string]interface{}{"key": "value"},
		Variables:    map[string]interface{}{"var": "val"},
		NodeResults: map[string]*NodeResult{
			"trigger-1": {
				NodeID:   "trigger-1",
				NodeName: "Trigger",
				Status:   NodeStatusCompleted,
			},
		},
		StartedAt: time.Now(),
	}

	err := repo.SaveExecution(ctx, execution)
	if err != nil {
		t.Fatalf("failed to save execution: %v", err)
	}

	// Retrieve execution
	retrieved, err := repo.GetExecution(ctx, "exec-1")
	if err != nil {
		t.Fatalf("failed to get execution: %v", err)
	}

	if retrieved.Status != ExecutionStatusCompleted {
		t.Errorf("expected status completed, got %s", retrieved.Status)
	}

	if retrieved.TriggerData["key"] != "value" {
		t.Error("expected trigger data to be preserved")
	}
}

func TestRepository_ListExecutions(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewRepository(db)
	ctx := context.Background()

	// Create workflow
	workflow := &Workflow{
		TenantID: "tenant-1",
		Name:     "Test Workflow",
		Status:   WorkflowStatusActive,
		Nodes:    []Node{{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"}},
	}
	repo.CreateWorkflow(ctx, workflow)

	// Create multiple executions
	for i := 0; i < 3; i++ {
		execution := &Execution{
			ID:           "exec-" + string(rune('1'+i)),
			WorkflowID:   workflow.ID,
			WorkflowName: workflow.Name,
			TenantID:     "tenant-1",
			Status:       ExecutionStatusCompleted,
			TriggerType:  TriggerTypeManual,
			StartedAt:    time.Now(),
		}
		repo.SaveExecution(ctx, execution)
	}

	executions, total, err := repo.ListExecutions(ctx, workflow.ID, &ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("failed to list executions: %v", err)
	}

	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}

	if len(executions) != 3 {
		t.Errorf("expected 3 executions, got %d", len(executions))
	}
}

func TestRepository_ExecutionLogs(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewRepository(db)
	ctx := context.Background()

	// Create workflow and execution
	workflow := &Workflow{
		TenantID: "tenant-1",
		Name:     "Test Workflow",
		Status:   WorkflowStatusActive,
		Nodes:    []Node{{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"}},
	}
	repo.CreateWorkflow(ctx, workflow)

	execution := &Execution{
		ID:           "exec-1",
		WorkflowID:   workflow.ID,
		WorkflowName: workflow.Name,
		TenantID:     "tenant-1",
		Status:       ExecutionStatusCompleted,
		TriggerType:  TriggerTypeManual,
		StartedAt:    time.Now(),
	}
	repo.SaveExecution(ctx, execution)

	// Save logs
	for i := 0; i < 5; i++ {
		log := &ExecutionLog{
			ExecutionID: "exec-1",
			NodeID:      "trigger-1",
			Level:       "info",
			Message:     "Log message " + string(rune('1'+i)),
			Timestamp:   time.Now(),
		}
		err := repo.SaveExecutionLog(ctx, log)
		if err != nil {
			t.Fatalf("failed to save log: %v", err)
		}
	}

	// Retrieve logs
	logs, total, err := repo.GetExecutionLogs(ctx, "exec-1", &ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("failed to get logs: %v", err)
	}

	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}

	if len(logs) != 5 {
		t.Errorf("expected 5 logs, got %d", len(logs))
	}
}

func TestRepository_Webhooks(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewRepository(db)
	ctx := context.Background()

	// Create workflow
	workflow := &Workflow{
		TenantID: "tenant-1",
		Name:     "Test Workflow",
		Status:   WorkflowStatusActive,
		Nodes:    []Node{{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"}},
	}
	repo.CreateWorkflow(ctx, workflow)

	// Save webhook
	err := repo.SaveWebhook(ctx, workflow.ID, "/my-webhook", "POST", "none", nil)
	if err != nil {
		t.Fatalf("failed to save webhook: %v", err)
	}

	// Get webhook by path
	workflowID, err := repo.GetWebhookByPath(ctx, "/my-webhook")
	if err != nil {
		t.Fatalf("failed to get webhook: %v", err)
	}

	if workflowID != workflow.ID {
		t.Errorf("expected workflow ID %s, got %s", workflow.ID, workflowID)
	}

	// Delete webhook
	err = repo.DeleteWebhook(ctx, workflow.ID)
	if err != nil {
		t.Fatalf("failed to delete webhook: %v", err)
	}

	// Verify deletion
	_, err = repo.GetWebhookByPath(ctx, "/my-webhook")
	if err != ErrWorkflowNotFound {
		t.Errorf("expected ErrWorkflowNotFound, got %v", err)
	}
}

func TestRepository_GetStats(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewRepository(db)
	ctx := context.Background()

	// Create workflows with different statuses
	for i := 0; i < 3; i++ {
		status := WorkflowStatusDraft
		if i == 0 {
			status = WorkflowStatusActive
		}
		workflow := &Workflow{
			TenantID: "tenant-1",
			Name:     "Workflow " + string(rune('A'+i)),
			Status:   status,
			Nodes:    []Node{{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"}},
		}
		repo.CreateWorkflow(ctx, workflow)
	}

	stats, err := repo.GetStats(ctx, "tenant-1")
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}

	if stats.TotalWorkflows != 3 {
		t.Errorf("expected 3 total workflows, got %d", stats.TotalWorkflows)
	}

	if stats.ActiveWorkflows != 1 {
		t.Errorf("expected 1 active workflow, got %d", stats.ActiveWorkflows)
	}
}

func TestRepository_CleanupOldExecutions(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewRepository(db)
	ctx := context.Background()

	// Create workflow
	workflow := &Workflow{
		TenantID: "tenant-1",
		Name:     "Test Workflow",
		Status:   WorkflowStatusActive,
		Nodes:    []Node{{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"}},
	}
	repo.CreateWorkflow(ctx, workflow)

	// Create old execution
	oldTime := time.Now().AddDate(0, 0, -60) // 60 days ago
	execution := &Execution{
		ID:           "old-exec",
		WorkflowID:   workflow.ID,
		WorkflowName: workflow.Name,
		TenantID:     "tenant-1",
		Status:       ExecutionStatusCompleted,
		TriggerType:  TriggerTypeManual,
		StartedAt:    oldTime,
		CompletedAt:  &oldTime,
	}
	repo.SaveExecution(ctx, execution)

	// Create recent execution
	recentTime := time.Now()
	execution2 := &Execution{
		ID:           "recent-exec",
		WorkflowID:   workflow.ID,
		WorkflowName: workflow.Name,
		TenantID:     "tenant-1",
		Status:       ExecutionStatusCompleted,
		TriggerType:  TriggerTypeManual,
		StartedAt:    recentTime,
		CompletedAt:  &recentTime,
	}
	repo.SaveExecution(ctx, execution2)

	// Cleanup old executions (30 days retention)
	deleted, err := repo.CleanupOldExecutions(ctx, 30)
	if err != nil {
		t.Fatalf("failed to cleanup: %v", err)
	}

	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	// Verify old execution is gone
	_, err = repo.GetExecution(ctx, "old-exec")
	if err != ErrExecutionNotFound {
		t.Error("expected old execution to be deleted")
	}

	// Verify recent execution still exists
	_, err = repo.GetExecution(ctx, "recent-exec")
	if err != nil {
		t.Error("expected recent execution to still exist")
	}
}
