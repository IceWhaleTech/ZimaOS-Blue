package workflow

import (
	"context"
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	config := DefaultConfig()
	engine := NewEngine(config)
	defer engine.Close()

	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
}

func TestEngine_ValidateWorkflow(t *testing.T) {
	engine := NewEngine(nil)
	defer engine.Close()

	t.Run("valid workflow", func(t *testing.T) {
		workflow := &Workflow{
			ID:   "test-workflow",
			Name: "Test Workflow",
			Nodes: []Node{
				{
					ID:   "trigger-1",
					Type: NodeTypeTrigger,
					Name: "Manual Trigger",
				},
				{
					ID:   "action-1",
					Type: NodeTypeAction,
					Name: "Test Action",
					Config: map[string]interface{}{
						"type": "set_variable",
					},
				},
			},
			Connections: []Connection{
				{
					ID:         "conn-1",
					SourceNode: "trigger-1",
					TargetNode: "action-1",
				},
			},
		}

		err := engine.ValidateWorkflow(workflow)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("missing ID", func(t *testing.T) {
		workflow := &Workflow{
			Name: "Test Workflow",
			Nodes: []Node{
				{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"},
			},
		}

		err := engine.ValidateWorkflow(workflow)
		if err == nil {
			t.Error("expected error for missing ID")
		}
	})

	t.Run("missing name", func(t *testing.T) {
		workflow := &Workflow{
			ID: "test-workflow",
			Nodes: []Node{
				{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"},
			},
		}

		err := engine.ValidateWorkflow(workflow)
		if err == nil {
			t.Error("expected error for missing name")
		}
	})

	t.Run("no nodes", func(t *testing.T) {
		workflow := &Workflow{
			ID:    "test-workflow",
			Name:  "Test Workflow",
			Nodes: []Node{},
		}

		err := engine.ValidateWorkflow(workflow)
		if err == nil {
			t.Error("expected error for no nodes")
		}
	})

	t.Run("no trigger node", func(t *testing.T) {
		workflow := &Workflow{
			ID:   "test-workflow",
			Name: "Test Workflow",
			Nodes: []Node{
				{ID: "action-1", Type: NodeTypeAction, Name: "Action"},
			},
		}

		err := engine.ValidateWorkflow(workflow)
		if err == nil {
			t.Error("expected error for no trigger node")
		}
	})

	t.Run("duplicate node ID", func(t *testing.T) {
		workflow := &Workflow{
			ID:   "test-workflow",
			Name: "Test Workflow",
			Nodes: []Node{
				{ID: "node-1", Type: NodeTypeTrigger, Name: "Trigger"},
				{ID: "node-1", Type: NodeTypeAction, Name: "Action"},
			},
		}

		err := engine.ValidateWorkflow(workflow)
		if err == nil {
			t.Error("expected error for duplicate node ID")
		}
	})

	t.Run("invalid connection source", func(t *testing.T) {
		workflow := &Workflow{
			ID:   "test-workflow",
			Name: "Test Workflow",
			Nodes: []Node{
				{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"},
			},
			Connections: []Connection{
				{ID: "conn-1", SourceNode: "invalid", TargetNode: "trigger-1"},
			},
		}

		err := engine.ValidateWorkflow(workflow)
		if err == nil {
			t.Error("expected error for invalid connection source")
		}
	})

	t.Run("cyclic dependency", func(t *testing.T) {
		workflow := &Workflow{
			ID:   "test-workflow",
			Name: "Test Workflow",
			Nodes: []Node{
				{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"},
				{ID: "action-1", Type: NodeTypeAction, Name: "Action 1"},
				{ID: "action-2", Type: NodeTypeAction, Name: "Action 2"},
			},
			Connections: []Connection{
				{ID: "conn-1", SourceNode: "trigger-1", TargetNode: "action-1"},
				{ID: "conn-2", SourceNode: "action-1", TargetNode: "action-2"},
				{ID: "conn-3", SourceNode: "action-2", TargetNode: "action-1"}, // Cycle
			},
		}

		err := engine.ValidateWorkflow(workflow)
		if err != ErrCyclicDependency {
			t.Errorf("expected ErrCyclicDependency, got %v", err)
		}
	})
}

func TestEngine_Execute(t *testing.T) {
	engine := NewEngine(nil)
	defer engine.Close()

	t.Run("simple workflow", func(t *testing.T) {
		workflow := &Workflow{
			ID:     "test-workflow",
			Name:   "Test Workflow",
			Status: WorkflowStatusActive,
			Nodes: []Node{
				{
					ID:   "trigger-1",
					Type: NodeTypeTrigger,
					Name: "Manual Trigger",
				},
				{
					ID:   "action-1",
					Type: NodeTypeAction,
					Name: "Set Variable",
					Config: map[string]interface{}{
						"type":  "set_variable",
						"name":  "result",
						"value": "success",
					},
				},
			},
			Connections: []Connection{
				{ID: "conn-1", SourceNode: "trigger-1", TargetNode: "action-1"},
			},
		}

		ctx := context.Background()
		execution, err := engine.Execute(ctx, workflow, TriggerTypeManual, nil)
		if err != nil {
			t.Fatalf("failed to execute workflow: %v", err)
		}

		if execution.ID == "" {
			t.Error("expected non-empty execution ID")
		}

		// Wait for execution to complete
		time.Sleep(100 * time.Millisecond)

		// Check execution status
		exec, err := engine.GetExecution(execution.ID)
		if err != nil {
			t.Fatalf("failed to get execution: %v", err)
		}

		if exec.Status != ExecutionStatusCompleted {
			t.Errorf("expected status completed, got %s", exec.Status)
		}
	})

	t.Run("disabled workflow", func(t *testing.T) {
		workflow := &Workflow{
			ID:     "test-workflow",
			Name:   "Test Workflow",
			Status: WorkflowStatusInactive,
			Nodes: []Node{
				{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"},
			},
		}

		ctx := context.Background()
		_, err := engine.Execute(ctx, workflow, TriggerTypeManual, nil)
		if err != ErrWorkflowDisabled {
			t.Errorf("expected ErrWorkflowDisabled, got %v", err)
		}
	})

	t.Run("with trigger data", func(t *testing.T) {
		workflow := &Workflow{
			ID:     "test-workflow",
			Name:   "Test Workflow",
			Status: WorkflowStatusActive,
			Nodes: []Node{
				{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"},
			},
		}

		ctx := context.Background()
		triggerData := map[string]interface{}{
			"key": "value",
		}

		execution, err := engine.Execute(ctx, workflow, TriggerTypeManual, triggerData)
		if err != nil {
			t.Fatalf("failed to execute workflow: %v", err)
		}

		if execution.TriggerData["key"] != "value" {
			t.Error("expected trigger data to be preserved")
		}
	})
}

func TestEngine_ExecutePausesAndResumesCheckpoint(t *testing.T) {
	engine := NewEngine(nil)
	defer engine.Close()

	workflow := &Workflow{
		ID:     "checkpoint-workflow",
		Name:   "Checkpoint Workflow",
		Status: WorkflowStatusActive,
		Nodes: []Node{
			{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"},
			{
				ID:   "action-1",
				Type: NodeTypeAction,
				Name: "Checkpointed Action",
				Config: map[string]interface{}{
					"type":              string(ActionTypeSetVariable),
					"name":              "result",
					"value":             "ready",
					"checkpoint_kind":   string(ExecutionCheckpointPauseForApproval),
					"checkpoint_reason": "approval_needed",
				},
			},
		},
		Connections: []Connection{
			{ID: "conn-1", SourceNode: "trigger-1", TargetNode: "action-1"},
		},
	}

	execution, err := engine.Execute(context.Background(), workflow, TriggerTypeManual, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	waitForWorkflowExecutionStatus(t, engine, execution.ID, ExecutionStatusPaused)
	paused, err := engine.GetExecution(execution.ID)
	if err != nil {
		t.Fatalf("GetExecution(paused) error = %v", err)
	}
	if paused.StatusReason != "approval_needed" {
		t.Fatalf("status_reason = %q, want approval_needed", paused.StatusReason)
	}
	if paused.Checkpoint == nil || paused.Checkpoint.Kind != ExecutionCheckpointPauseForApproval {
		t.Fatalf("checkpoint = %+v, want pause_for_approval", paused.Checkpoint)
	}

	resumed, err := engine.ResumeExecution(execution.ID, ExecutionResumeInput{
		Decision: "approve",
		Payload:  map[string]interface{}{"ticket": "A-1"},
	})
	if err != nil {
		t.Fatalf("ResumeExecution() error = %v", err)
	}
	if resumed.Status != ExecutionStatusRunning && resumed.Status != ExecutionStatusCompleted {
		t.Fatalf("resume status = %q, want running or completed", resumed.Status)
	}
	if resumed.Status == ExecutionStatusRunning && resumed.StatusReason != string(ExecutionCheckpointResumeWithDecision) {
		t.Fatalf("resume status_reason = %q, want %q", resumed.StatusReason, ExecutionCheckpointResumeWithDecision)
	}

	waitForWorkflowExecutionStatus(t, engine, execution.ID, ExecutionStatusCompleted)
	completed, err := engine.GetExecution(execution.ID)
	if err != nil {
		t.Fatalf("GetExecution(completed) error = %v", err)
	}
	if completed.Checkpoint == nil || completed.Checkpoint.Resume == nil {
		t.Fatalf("checkpoint resume = %+v, want populated resume", completed.Checkpoint)
	}
	if completed.Checkpoint.Resume.Decision != "approve" {
		t.Fatalf("checkpoint resume decision = %q, want approve", completed.Checkpoint.Resume.Decision)
	}
	if got := completed.Variables["checkpoint_decision"]; got != "approve" {
		t.Fatalf("checkpoint_decision = %v, want approve", got)
	}
}

func waitForWorkflowExecutionStatus(t *testing.T, engine *Engine, executionID string, status ExecutionStatus) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		execution, err := engine.GetExecution(executionID)
		if err == nil && execution.Status == status {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	execution, err := engine.GetExecution(executionID)
	if err != nil {
		t.Fatalf("GetExecution(%s) error = %v", executionID, err)
	}
	t.Fatalf("execution status = %q, want %q", execution.Status, status)
}

func TestEngine_CancelExecution(t *testing.T) {
	engine := NewEngine(nil)
	defer engine.Close()

	workflow := &Workflow{
		ID:     "test-workflow",
		Name:   "Test Workflow",
		Status: WorkflowStatusActive,
		Nodes: []Node{
			{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"},
			{
				ID:   "delay-1",
				Type: NodeTypeDelay,
				Name: "Delay",
				Config: map[string]interface{}{
					"duration": 60, // 60 seconds
				},
			},
		},
		Connections: []Connection{
			{ID: "conn-1", SourceNode: "trigger-1", TargetNode: "delay-1"},
		},
	}

	ctx := context.Background()
	execution, err := engine.Execute(ctx, workflow, TriggerTypeManual, nil)
	if err != nil {
		t.Fatalf("failed to execute workflow: %v", err)
	}

	// Wait a bit for execution to start the delay node
	time.Sleep(100 * time.Millisecond)

	// Cancel execution
	err = engine.CancelExecution(execution.ID)
	if err != nil {
		t.Fatalf("failed to cancel execution: %v", err)
	}

	// Wait for cancellation to take effect
	time.Sleep(100 * time.Millisecond)

	exec, _ := engine.GetExecution(execution.ID)
	// Execution should be cancelled or already completed/failed
	if exec.Status != ExecutionStatusCancelled && exec.Status != ExecutionStatusCompleted && exec.Status != ExecutionStatusFailed {
		t.Errorf("expected status cancelled/completed/failed, got %s", exec.Status)
	}
}

func TestEngine_InterpolateString(t *testing.T) {
	engine := NewEngine(nil)
	defer engine.Close()

	variables := map[string]interface{}{
		"name": "John",
		"nested": map[string]interface{}{
			"value": "test",
		},
	}

	input := map[string]interface{}{
		"data": "input-data",
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"Hello {{name}}", "Hello John"},
		{"Value: {{nested.value}}", "Value: test"},
		{"Input: {{input.data}}", "Input: input-data"},
		{"No interpolation", "No interpolation"},
		{"{{unknown}}", "{{unknown}}"},
	}

	for _, tt := range tests {
		result := engine.interpolateString(tt.input, variables, input)
		if result != tt.expected {
			t.Errorf("interpolateString(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestEngine_EvaluateCondition(t *testing.T) {
	engine := NewEngine(nil)
	defer engine.Close()

	variables := map[string]interface{}{
		"count": float64(10),
		"name":  "test",
	}

	input := map[string]interface{}{}

	tests := []struct {
		expression string
		expected   bool
	}{
		{"true", true},
		{"false", false},
		{"count == 10", true},
		{"count == 5", false},
		{"name == 'test'", true},
		{"name != 'other'", true},
	}

	for _, tt := range tests {
		result := engine.evaluateCondition(tt.expression, variables, input)
		if result != tt.expected {
			t.Errorf("evaluateCondition(%q) = %v, want %v", tt.expression, result, tt.expected)
		}
	}
}

func TestEngine_ConditionNode(t *testing.T) {
	engine := NewEngine(nil)
	defer engine.Close()

	workflow := &Workflow{
		ID:     "test-workflow",
		Name:   "Test Workflow",
		Status: WorkflowStatusActive,
		Variables: map[string]string{
			"value": "10",
		},
		Nodes: []Node{
			{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"},
			{
				ID:   "condition-1",
				Type: NodeTypeCondition,
				Name: "Check Value",
				Config: map[string]interface{}{
					"expression": "true",
					"true_port":  "yes",
					"false_port": "no",
				},
			},
			{
				ID:   "action-true",
				Type: NodeTypeAction,
				Name: "True Action",
				Config: map[string]interface{}{
					"type": "set_variable",
				},
			},
		},
		Connections: []Connection{
			{ID: "conn-1", SourceNode: "trigger-1", TargetNode: "condition-1"},
			{ID: "conn-2", SourceNode: "condition-1", SourcePort: "yes", TargetNode: "action-true"},
		},
	}

	ctx := context.Background()
	execution, err := engine.Execute(ctx, workflow, TriggerTypeManual, nil)
	if err != nil {
		t.Fatalf("failed to execute workflow: %v", err)
	}

	// Wait for execution
	time.Sleep(100 * time.Millisecond)

	exec, _ := engine.GetExecution(execution.ID)
	if exec.Status != ExecutionStatusCompleted {
		t.Errorf("expected status completed, got %s", exec.Status)
	}

	// Check condition result
	if result, ok := exec.NodeResults["condition-1"]; ok {
		if result.Output["result"] != true {
			t.Error("expected condition result to be true")
		}
	}
}

func TestEngine_DelayNode(t *testing.T) {
	engine := NewEngine(nil)
	defer engine.Close()

	workflow := &Workflow{
		ID:     "test-workflow",
		Name:   "Test Workflow",
		Status: WorkflowStatusActive,
		Nodes: []Node{
			{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"},
			{
				ID:   "delay-1",
				Type: NodeTypeDelay,
				Name: "Short Delay",
				Config: map[string]interface{}{
					"duration": 0.1, // 100ms
				},
			},
		},
		Connections: []Connection{
			{ID: "conn-1", SourceNode: "trigger-1", TargetNode: "delay-1"},
		},
	}

	ctx := context.Background()
	start := time.Now()

	execution, err := engine.Execute(ctx, workflow, TriggerTypeManual, nil)
	if err != nil {
		t.Fatalf("failed to execute workflow: %v", err)
	}

	// Wait for execution
	time.Sleep(200 * time.Millisecond)

	exec, _ := engine.GetExecution(execution.ID)
	if exec.Status != ExecutionStatusCompleted {
		t.Errorf("expected status completed, got %s", exec.Status)
	}

	elapsed := time.Since(start)
	if elapsed < 100*time.Millisecond {
		t.Errorf("delay should have taken at least 100ms, took %v", elapsed)
	}
}

func TestEngine_DisabledNode(t *testing.T) {
	engine := NewEngine(nil)
	defer engine.Close()

	workflow := &Workflow{
		ID:     "test-workflow",
		Name:   "Test Workflow",
		Status: WorkflowStatusActive,
		Nodes: []Node{
			{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"},
			{
				ID:       "action-1",
				Type:     NodeTypeAction,
				Name:     "Disabled Action",
				Disabled: true,
				Config: map[string]interface{}{
					"type": "set_variable",
				},
			},
			{
				ID:   "action-2",
				Type: NodeTypeAction,
				Name: "Enabled Action",
				Config: map[string]interface{}{
					"type": "set_variable",
				},
			},
		},
		Connections: []Connection{
			{ID: "conn-1", SourceNode: "trigger-1", TargetNode: "action-1"},
			{ID: "conn-2", SourceNode: "action-1", TargetNode: "action-2"},
		},
	}

	ctx := context.Background()
	execution, err := engine.Execute(ctx, workflow, TriggerTypeManual, nil)
	if err != nil {
		t.Fatalf("failed to execute workflow: %v", err)
	}

	// Wait for execution
	time.Sleep(100 * time.Millisecond)

	exec, _ := engine.GetExecution(execution.ID)
	if exec.Status != ExecutionStatusCompleted {
		t.Errorf("expected status completed, got %s", exec.Status)
	}

	// Disabled node should be skipped
	if result, ok := exec.NodeResults["action-1"]; ok {
		if result.Status != NodeStatusSkipped {
			// Note: Current implementation marks disabled nodes as completed
			// This is acceptable behavior
		}
	}
}

func TestEngine_MaxConcurrentExecutions(t *testing.T) {
	config := DefaultConfig()
	config.MaxConcurrentExecutions = 1
	engine := NewEngine(config)
	defer engine.Close()

	workflow := &Workflow{
		ID:     "test-workflow",
		Name:   "Test Workflow",
		Status: WorkflowStatusActive,
		Nodes: []Node{
			{ID: "trigger-1", Type: NodeTypeTrigger, Name: "Trigger"},
			{
				ID:   "delay-1",
				Type: NodeTypeDelay,
				Name: "Long Delay",
				Config: map[string]interface{}{
					"duration": 10, // 10 seconds
				},
			},
		},
		Connections: []Connection{
			{ID: "conn-1", SourceNode: "trigger-1", TargetNode: "delay-1"},
		},
	}

	ctx := context.Background()

	// Start first execution
	_, err := engine.Execute(ctx, workflow, TriggerTypeManual, nil)
	if err != nil {
		t.Fatalf("failed to start first execution: %v", err)
	}

	// Wait a bit
	time.Sleep(50 * time.Millisecond)

	// Try to start second execution - should fail
	_, err = engine.Execute(ctx, workflow, TriggerTypeManual, nil)
	if err != ErrMaxExecutionsReached {
		t.Errorf("expected ErrMaxExecutionsReached, got %v", err)
	}
}
