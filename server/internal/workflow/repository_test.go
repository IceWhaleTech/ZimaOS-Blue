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

func TestRepository_GetWorkflow_NotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewRepository(db)
	ctx := context.Background()

	_, err := repo.GetWorkflow(ctx, "non-existent-id")
	if err != ErrWorkflowNotFound {
		t.Errorf("expected ErrWorkflowNotFound, got %v", err)
	}
}

func TestRepository_UpdateWorkflow_NotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewRepository(db)
	ctx := context.Background()

	workflow := &Workflow{
		ID:       "non-existent-id",
		TenantID: "tenant-1",
		Name:     "Test",
		Status:   WorkflowStatusDraft,
		Nodes:    []Node{{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"}},
	}

	err := repo.UpdateWorkflow(ctx, workflow)
	if err != ErrWorkflowNotFound {
		t.Errorf("expected ErrWorkflowNotFound, got %v", err)
	}
}

func TestRepository_DeleteWorkflow_NotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewRepository(db)
	ctx := context.Background()

	err := repo.DeleteWorkflow(ctx, "non-existent-id")
	if err != ErrWorkflowNotFound {
		t.Errorf("expected ErrWorkflowNotFound, got %v", err)
	}
}

func TestRepository_CreateWorkflow_WithAllFields(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewRepository(db)
	ctx := context.Background()

	workflow := &Workflow{
		TenantID:    "tenant-1",
		Name:        "Full Workflow",
		Description: "A workflow with all fields",
		Status:      WorkflowStatusActive,
		Nodes:       []Node{{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"}},
		Connections: []Connection{{ID: "conn-1", SourceNode: "trigger-1", TargetNode: "action-1"}},
		Variables:   map[string]string{"var1": "value1", "var2": "value2"},
		Settings: &WorkflowSettings{
			Timeout:            300,
			MaxRetries:         3,
			ContinueOnError:    true,
			SaveExecutionData:  true,
			NotifyOnCompletion: true,
		},
		Tags:      []string{"tag1", "tag2", "tag3"},
		CreatedBy: "user-123",
	}

	err := repo.CreateWorkflow(ctx, workflow)
	if err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	retrieved, err := repo.GetWorkflow(ctx, workflow.ID)
	if err != nil {
		t.Fatalf("failed to get workflow: %v", err)
	}

	if retrieved.Description != "A workflow with all fields" {
		t.Errorf("expected description to be preserved, got '%s'", retrieved.Description)
	}

	if retrieved.CreatedBy != "user-123" {
		t.Errorf("expected created_by 'user-123', got '%s'", retrieved.CreatedBy)
	}

	if len(retrieved.Tags) != 3 {
		t.Errorf("expected 3 tags, got %d", len(retrieved.Tags))
	}

	if retrieved.Variables["var1"] != "value1" {
		t.Error("expected variables to be preserved")
	}

	if retrieved.Settings == nil || retrieved.Settings.Timeout != 300 {
		t.Error("expected settings to be preserved")
	}

	if len(retrieved.Connections) != 1 {
		t.Errorf("expected 1 connection, got %d", len(retrieved.Connections))
	}
}

func TestRepository_CreateWorkflow_PresetID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewRepository(db)
	ctx := context.Background()

	presetID := "custom-workflow-id-123"
	workflow := &Workflow{
		ID:       presetID,
		TenantID: "tenant-1",
		Name:     "Preset ID Workflow",
		Status:   WorkflowStatusDraft,
		Nodes:    []Node{{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"}},
	}

	err := repo.CreateWorkflow(ctx, workflow)
	if err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	if workflow.ID != presetID {
		t.Errorf("expected ID '%s', got '%s'", presetID, workflow.ID)
	}

	retrieved, err := repo.GetWorkflow(ctx, presetID)
	if err != nil {
		t.Fatalf("failed to get workflow: %v", err)
	}

	if retrieved.ID != presetID {
		t.Errorf("expected retrieved ID '%s', got '%s'", presetID, retrieved.ID)
	}
}

func TestRepository_UpdateWorkflow_VersionIncrement(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewRepository(db)
	ctx := context.Background()

	workflow := &Workflow{
		TenantID: "tenant-1",
		Name:     "Version Test",
		Status:   WorkflowStatusDraft,
		Nodes:    []Node{{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"}},
	}

	repo.CreateWorkflow(ctx, workflow)

	if workflow.Version != 1 {
		t.Errorf("expected initial version 1, got %d", workflow.Version)
	}

	// First update
	workflow.Name = "Version Test v2"
	err := repo.UpdateWorkflow(ctx, workflow)
	if err != nil {
		t.Fatalf("failed to update workflow: %v", err)
	}

	retrieved, _ := repo.GetWorkflow(ctx, workflow.ID)
	if retrieved.Version != 2 {
		t.Errorf("expected version 2 after first update, got %d", retrieved.Version)
	}

	// Second update
	workflow.Name = "Version Test v3"
	workflow.Version = retrieved.Version
	err = repo.UpdateWorkflow(ctx, workflow)
	if err != nil {
		t.Fatalf("failed to update workflow second time: %v", err)
	}

	retrieved, _ = repo.GetWorkflow(ctx, workflow.ID)
	if retrieved.Version != 3 {
		t.Errorf("expected version 3 after second update, got %d", retrieved.Version)
	}
}

func TestRepository_ListWorkflows_Pagination(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewRepository(db)
	ctx := context.Background()

	// Create 5 workflows
	for i := 0; i < 5; i++ {
		workflow := &Workflow{
			TenantID: "tenant-1",
			Name:     "Workflow " + string(rune('A'+i)),
			Status:   WorkflowStatusDraft,
			Nodes:    []Node{{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"}},
		}
		repo.CreateWorkflow(ctx, workflow)
	}

	// Test offset beyond total
	workflows, total, err := repo.ListWorkflows(ctx, "tenant-1", &ListOptions{Limit: 10, Offset: 100})
	if err != nil {
		t.Fatalf("failed to list workflows: %v", err)
	}
	if len(workflows) != 0 {
		t.Errorf("expected 0 workflows with offset beyond total, got %d", len(workflows))
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}

	// Test limit 0
	workflows, total, err = repo.ListWorkflows(ctx, "tenant-1", &ListOptions{Limit: 0})
	if err != nil {
		t.Fatalf("failed to list workflows: %v", err)
	}
	if len(workflows) != 0 {
		t.Errorf("expected 0 workflows with limit 0, got %d", len(workflows))
	}

	// Test different tenant isolation
	workflow := &Workflow{
		TenantID: "tenant-2",
		Name:     "Tenant 2 Workflow",
		Status:   WorkflowStatusDraft,
		Nodes:    []Node{{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"}},
	}
	repo.CreateWorkflow(ctx, workflow)

	workflows, total, err = repo.ListWorkflows(ctx, "tenant-2", &ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("failed to list workflows for tenant-2: %v", err)
	}
	if total != 1 {
		t.Errorf("expected 1 workflow for tenant-2, got %d", total)
	}
	if len(workflows) != 1 {
		t.Errorf("expected 1 workflow in list for tenant-2, got %d", len(workflows))
	}
}

func TestRepository_ListWorkflows_MultipleStatuses(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewRepository(db)
	ctx := context.Background()

	// Create workflows with all 3 statuses
	statuses := []WorkflowStatus{WorkflowStatusActive, WorkflowStatusInactive, WorkflowStatusDraft}
	for i, status := range statuses {
		workflow := &Workflow{
			TenantID: "tenant-1",
			Name:     "Workflow " + string(rune('A'+i)),
			Status:   status,
			Nodes:    []Node{{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"}},
		}
		repo.CreateWorkflow(ctx, workflow)
	}

	// Filter by active
	_, total, err := repo.ListWorkflows(ctx, "tenant-1", &ListOptions{
		Limit:   10,
		Filters: map[string]string{"status": "active"},
	})
	if err != nil {
		t.Fatalf("failed to list active workflows: %v", err)
	}
	if total != 1 {
		t.Errorf("expected 1 active workflow, got %d", total)
	}

	// Filter by inactive
	_, total, err = repo.ListWorkflows(ctx, "tenant-1", &ListOptions{
		Limit:   10,
		Filters: map[string]string{"status": "inactive"},
	})
	if err != nil {
		t.Fatalf("failed to list inactive workflows: %v", err)
	}
	if total != 1 {
		t.Errorf("expected 1 inactive workflow, got %d", total)
	}

	// Filter by draft
	_, total, err = repo.ListWorkflows(ctx, "tenant-1", &ListOptions{
		Limit:   10,
		Filters: map[string]string{"status": "draft"},
	})
	if err != nil {
		t.Fatalf("failed to list draft workflows: %v", err)
	}
	if total != 1 {
		t.Errorf("expected 1 draft workflow, got %d", total)
	}
}

func TestRepository_SaveExecution_AllStatuses(t *testing.T) {
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

	// Test different execution statuses
	statuses := []ExecutionStatus{
		ExecutionStatusCompleted,
		ExecutionStatusFailed,
		ExecutionStatusCancelled,
	}

	for i, status := range statuses {
		execution := &Execution{
			ID:           "exec-" + string(rune('1'+i)),
			WorkflowID:   workflow.ID,
			WorkflowName: workflow.Name,
			TenantID:     "tenant-1",
			Status:       status,
			TriggerType:  TriggerTypeManual,
			StartedAt:    time.Now(),
		}

		err := repo.SaveExecution(ctx, execution)
		if err != nil {
			t.Fatalf("failed to save execution with status %s: %v", status, err)
		}

		retrieved, err := repo.GetExecution(ctx, execution.ID)
		if err != nil {
			t.Fatalf("failed to get execution: %v", err)
		}

		if retrieved.Status != status {
			t.Errorf("expected status %s, got %s", status, retrieved.Status)
		}
	}
}

func TestRepository_GetExecution_NotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewRepository(db)
	ctx := context.Background()

	_, err := repo.GetExecution(ctx, "non-existent-exec")
	if err != ErrExecutionNotFound {
		t.Errorf("expected ErrExecutionNotFound, got %v", err)
	}
}

func TestRepository_ListExecutions_Pagination(t *testing.T) {
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

	// Create 5 executions
	for i := 0; i < 5; i++ {
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

	// Test with limit and offset
	executions, total, err := repo.ListExecutions(ctx, workflow.ID, &ListOptions{Limit: 2, Offset: 1})
	if err != nil {
		t.Fatalf("failed to list executions: %v", err)
	}
	if len(executions) != 2 {
		t.Errorf("expected 2 executions with limit, got %d", len(executions))
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}

	// Test for workflow with no executions
	workflow2 := &Workflow{
		TenantID: "tenant-1",
		Name:     "Empty Workflow",
		Status:   WorkflowStatusActive,
		Nodes:    []Node{{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"}},
	}
	repo.CreateWorkflow(ctx, workflow2)

	executions, total, err = repo.ListExecutions(ctx, workflow2.ID, &ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("failed to list executions for empty workflow: %v", err)
	}
	if len(executions) != 0 {
		t.Errorf("expected 0 executions for new workflow, got %d", len(executions))
	}
	if total != 0 {
		t.Errorf("expected total 0 for new workflow, got %d", total)
	}
}

func TestRepository_ExecutionLogs_Pagination(t *testing.T) {
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

	// Save 10 logs
	for i := 0; i < 10; i++ {
		log := &ExecutionLog{
			ExecutionID: "exec-1",
			NodeID:      "trigger-1",
			Level:       "info",
			Message:     "Log message " + string(rune('0'+i)),
			Timestamp:   time.Now(),
		}
		repo.SaveExecutionLog(ctx, log)
	}

	// Test with limit and offset
	logs, total, err := repo.GetExecutionLogs(ctx, "exec-1", &ListOptions{Limit: 3, Offset: 2})
	if err != nil {
		t.Fatalf("failed to get logs: %v", err)
	}
	if len(logs) != 3 {
		t.Errorf("expected 3 logs with limit, got %d", len(logs))
	}
	if total != 10 {
		t.Errorf("expected total 10, got %d", total)
	}

	// Test for non-existent execution
	logs, total, err = repo.GetExecutionLogs(ctx, "non-existent-exec", &ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("failed to get logs for non-existent execution: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("expected 0 logs for non-existent execution, got %d", len(logs))
	}
	if total != 0 {
		t.Errorf("expected total 0 for non-existent execution, got %d", total)
	}
}

func TestRepository_Webhooks_DuplicatePath(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewRepository(db)
	ctx := context.Background()

	// Create two workflows
	workflow1 := &Workflow{
		TenantID: "tenant-1",
		Name:     "Workflow 1",
		Status:   WorkflowStatusActive,
		Nodes:    []Node{{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"}},
	}
	repo.CreateWorkflow(ctx, workflow1)

	workflow2 := &Workflow{
		TenantID: "tenant-1",
		Name:     "Workflow 2",
		Status:   WorkflowStatusActive,
		Nodes:    []Node{{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"}},
	}
	repo.CreateWorkflow(ctx, workflow2)

	// Save webhook for first workflow
	err := repo.SaveWebhook(ctx, workflow1.ID, "/duplicate-path", "POST", "none", nil)
	if err != nil {
		t.Fatalf("failed to save first webhook: %v", err)
	}

	// Try to save webhook with same path for second workflow (ReplaceInto replaces the old one)
	err = repo.SaveWebhook(ctx, workflow2.ID, "/duplicate-path", "POST", "none", nil)
	if err != nil {
		t.Fatalf("failed to save second webhook: %v", err)
	}

	// Path should now point to second workflow
	workflowID, err := repo.GetWebhookByPath(ctx, "/duplicate-path")
	if err != nil {
		t.Fatalf("failed to get webhook: %v", err)
	}
	if workflowID != workflow2.ID {
		t.Errorf("expected webhook to point to workflow2 %s, got %s", workflow2.ID, workflowID)
	}
}

func TestRepository_Webhooks_DifferentMethods(t *testing.T) {
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

	// Save webhook with GET method
	err := repo.SaveWebhook(ctx, workflow.ID, "/get-webhook", "GET", "none", nil)
	if err != nil {
		t.Fatalf("failed to save GET webhook: %v", err)
	}

	workflowID, err := repo.GetWebhookByPath(ctx, "/get-webhook")
	if err != nil {
		t.Fatalf("failed to get GET webhook: %v", err)
	}
	if workflowID != workflow.ID {
		t.Errorf("expected workflow ID %s, got %s", workflow.ID, workflowID)
	}

	// Delete and save webhook with auth config
	repo.DeleteWebhook(ctx, workflow.ID)

	authConfig := map[string]string{"token": "secret-token"}
	err = repo.SaveWebhook(ctx, workflow.ID, "/auth-webhook", "POST", "bearer", authConfig)
	if err != nil {
		t.Fatalf("failed to save webhook with auth: %v", err)
	}

	workflowID, err = repo.GetWebhookByPath(ctx, "/auth-webhook")
	if err != nil {
		t.Fatalf("failed to get webhook with auth: %v", err)
	}
	if workflowID != workflow.ID {
		t.Errorf("expected workflow ID %s, got %s", workflow.ID, workflowID)
	}
}

func TestRepository_GetStats_Empty(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewRepository(db)
	ctx := context.Background()

	stats, err := repo.GetStats(ctx, "empty-tenant")
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}

	if stats.TotalWorkflows != 0 {
		t.Errorf("expected 0 total workflows, got %d", stats.TotalWorkflows)
	}

	if stats.ActiveWorkflows != 0 {
		t.Errorf("expected 0 active workflows, got %d", stats.ActiveWorkflows)
	}

	if stats.TotalExecutions != 0 {
		t.Errorf("expected 0 total executions, got %d", stats.TotalExecutions)
	}
}

func TestRepository_GetStats_MultiTenant(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, _ := NewRepository(db)
	ctx := context.Background()

	// Create workflows for tenant-1
	for i := 0; i < 3; i++ {
		workflow := &Workflow{
			TenantID: "tenant-1",
			Name:     "Tenant 1 Workflow " + string(rune('A'+i)),
			Status:   WorkflowStatusActive,
			Nodes:    []Node{{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"}},
		}
		repo.CreateWorkflow(ctx, workflow)
	}

	// Create workflows for tenant-2
	for i := 0; i < 2; i++ {
		workflow := &Workflow{
			TenantID: "tenant-2",
			Name:     "Tenant 2 Workflow " + string(rune('A'+i)),
			Status:   WorkflowStatusDraft,
			Nodes:    []Node{{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"}},
		}
		repo.CreateWorkflow(ctx, workflow)
	}

	// Get stats for tenant-1
	stats1, err := repo.GetStats(ctx, "tenant-1")
	if err != nil {
		t.Fatalf("failed to get stats for tenant-1: %v", err)
	}
	if stats1.TotalWorkflows != 3 {
		t.Errorf("expected 3 workflows for tenant-1, got %d", stats1.TotalWorkflows)
	}
	if stats1.ActiveWorkflows != 3 {
		t.Errorf("expected 3 active workflows for tenant-1, got %d", stats1.ActiveWorkflows)
	}

	// Get stats for tenant-2
	stats2, err := repo.GetStats(ctx, "tenant-2")
	if err != nil {
		t.Fatalf("failed to get stats for tenant-2: %v", err)
	}
	if stats2.TotalWorkflows != 2 {
		t.Errorf("expected 2 workflows for tenant-2, got %d", stats2.TotalWorkflows)
	}
	if stats2.ActiveWorkflows != 0 {
		t.Errorf("expected 0 active workflows for tenant-2, got %d", stats2.ActiveWorkflows)
	}
}

func TestRepository_CleanupOldExecutions_NoneToClean(t *testing.T) {
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

	// Create recent execution
	recentTime := time.Now()
	execution := &Execution{
		ID:           "recent-exec",
		WorkflowID:   workflow.ID,
		WorkflowName: workflow.Name,
		TenantID:     "tenant-1",
		Status:       ExecutionStatusCompleted,
		TriggerType:  TriggerTypeManual,
		StartedAt:    recentTime,
		CompletedAt:  &recentTime,
	}
	repo.SaveExecution(ctx, execution)

	// Cleanup with 30 days retention (should delete nothing)
	deleted, err := repo.CleanupOldExecutions(ctx, 30)
	if err != nil {
		t.Fatalf("failed to cleanup: %v", err)
	}

	if deleted != 0 {
		t.Errorf("expected 0 deleted, got %d", deleted)
	}

	// Verify execution still exists
	_, err = repo.GetExecution(ctx, "recent-exec")
	if err != nil {
		t.Error("expected execution to still exist")
	}
}

func TestRepository_SaveExecution_WithError(t *testing.T) {
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

	// Create failed execution with error message
	errorMsg := "Node execution failed: connection timeout"
	execution := &Execution{
		ID:           "failed-exec",
		WorkflowID:   workflow.ID,
		WorkflowName: workflow.Name,
		TenantID:     "tenant-1",
		Status:       ExecutionStatusFailed,
		TriggerType:  TriggerTypeManual,
		Error:        errorMsg,
		StartedAt:    time.Now(),
	}

	err := repo.SaveExecution(ctx, execution)
	if err != nil {
		t.Fatalf("failed to save execution: %v", err)
	}

	// Retrieve and verify error is preserved
	retrieved, err := repo.GetExecution(ctx, "failed-exec")
	if err != nil {
		t.Fatalf("failed to get execution: %v", err)
	}

	if retrieved.Status != ExecutionStatusFailed {
		t.Errorf("expected status failed, got %s", retrieved.Status)
	}

	if retrieved.Error != errorMsg {
		t.Errorf("expected error '%s', got '%s'", errorMsg, retrieved.Error)
	}
}
