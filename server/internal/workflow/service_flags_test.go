package workflow

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

func TestWorkflowServiceDisablesCheckpointRuntimeWhenFlagIsOff(t *testing.T) {
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

	service.SetFlagEvaluator(config.NewFlagEvaluator(&config.GrayscaleConfig{
		Enabled: true,
		Flags: []config.FeatureFlag{{
			Name:    workflowCheckpointResumeFlagName,
			Enabled: false,
		}},
	}))

	ctx := withWorkflowTenant(context.Background(), "tenant-1")
	workflow := &Workflow{
		TenantID: "tenant-1",
		Name:     "Flagged Workflow",
		Status:   WorkflowStatusActive,
		Nodes: []Node{
			{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"},
			{
				ID:   "action-1",
				Type: NodeTypeAction,
				Name: "Checkpointed Action",
				Config: map[string]interface{}{
					"type":            string(ActionTypeSetVariable),
					"name":            "result",
					"value":           "ready",
					"checkpoint_kind": string(ExecutionCheckpointPauseForApproval),
				},
			},
		},
		Connections: []Connection{
			{ID: "conn-1", SourceNode: "trigger-1", TargetNode: "action-1"},
		},
	}
	if err := repo.CreateWorkflow(ctx, workflow); err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}

	execution, err := service.ExecuteWorkflow(ctx, workflow.ID, nil)
	if err != nil {
		t.Fatalf("ExecuteWorkflow() error = %v", err)
	}

	waitForWorkflowExecutionStatus(t, service.engine, execution.ID, ExecutionStatusCompleted)
	completed, err := service.engine.GetExecution(execution.ID)
	if err != nil {
		t.Fatalf("GetExecution() error = %v", err)
	}
	if completed.Checkpoint != nil {
		t.Fatalf("checkpoint = %+v, want nil when feature flag is disabled", completed.Checkpoint)
	}
}
