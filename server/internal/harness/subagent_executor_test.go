package harness

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type completingSubagentDriver struct {
	controller *Controller
	kind       RunKind
	result     string
}

func (d *completingSubagentDriver) Kind() RunKind { return d.kind }

func (d *completingSubagentDriver) Validate(spec RunSpec) error {
	if spec.Goal == "" {
		return fmt.Errorf("goal is required")
	}
	return nil
}

func (d *completingSubagentDriver) Start(_ context.Context, run *Run, _ RunEnv) error {
	go func(runID string) {
		time.Sleep(20 * time.Millisecond)
		snapshot, err := d.controller.GetStored(context.Background(), runID)
		if err != nil {
			return
		}
		now := timeutil.NowTime()
		snapshot.Status = RunStatusCompleted
		snapshot.Result = d.result
		snapshot.UpdatedAt = now
		snapshot.StartedAt = &now
		snapshot.FinishedAt = &now
		_ = d.controller.SyncSnapshot(context.Background(), snapshot)
	}(run.ID)
	return nil
}

func (d *completingSubagentDriver) Cancel(_ context.Context, _ *Run) error { return nil }

func TestHarnessSubagentExecutorSpawnsAndWaits(t *testing.T) {
	controller := newTestController(t)
	parentDriver := &stubDriver{kind: RunKindAgentTask}
	childDriver := &completingSubagentDriver{
		controller: controller,
		kind:       RunKindSubagent,
		result:     "child complete",
	}
	controller.RegisterDriver(parentDriver)
	controller.RegisterDriver(childDriver)

	parent, err := controller.Submit(context.Background(), RunSpec{
		Kind:          RunKindAgentTask,
		Goal:          "root goal",
		UserID:        "user-1",
		AgentID:       "main",
		Model:         "gpt-parent",
		ApprovalMode:  ApprovalModeAsk,
		SandboxMode:   "inherit",
		MaxDuration:   8 * time.Minute,
		MaxSteps:      20,
		MaxToolRounds: 40,
		MaxSubagents:  2,
		MaxDepth:      2,
	})
	if err != nil {
		t.Fatalf("Submit parent failed: %v", err)
	}

	executor := NewSubagentExecutor(controller, config.DefaultAgentsConfig())
	ctx := tools.WithRunID(context.Background(), parent.ID)
	result, err := executor.ExecuteSubagent(ctx, tools.SubagentRequest{
		Goal:    "inspect the failing query",
		Context: "Focus on SQL logs first.",
		Wait:    true,
	})
	if err != nil {
		t.Fatalf("ExecuteSubagent failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if !result.Waited || !result.Terminal || !result.Completed {
		t.Fatalf("unexpected terminal flags: %#v", result)
	}
	if result.ParentRunID != parent.ID || result.Status != string(RunStatusCompleted) {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Model != "gpt-parent" {
		t.Fatalf("model = %q, want gpt-parent", result.Model)
	}

	child, err := controller.GetStored(context.Background(), result.RunID)
	if err != nil {
		t.Fatalf("GetStored child failed: %v", err)
	}
	if child.MaxSteps != 10 {
		t.Fatalf("MaxSteps = %d, want 10", child.MaxSteps)
	}
	if child.MaxToolRounds != 20 {
		t.Fatalf("MaxToolRounds = %d, want 20", child.MaxToolRounds)
	}
	if child.MaxDuration != 4*time.Minute {
		t.Fatalf("MaxDuration = %v, want 4m", child.MaxDuration)
	}
	if contextText, _ := child.Metadata["context"].(string); contextText != "Focus on SQL logs first." {
		t.Fatalf("unexpected metadata: %#v", child.Metadata)
	}
}
