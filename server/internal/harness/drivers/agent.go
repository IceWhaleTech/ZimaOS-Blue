package drivers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	agentpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

type AgentDriver struct {
	runner  *agentpkg.Runner
	store   *agentpkg.Store
	manager *harness.Controller
	kind    harness.RunKind
}

func NewAgentDriver(kind harness.RunKind, runner *agentpkg.Runner, store *agentpkg.Store, manager *harness.Controller) *AgentDriver {
	return &AgentDriver{
		runner:  runner,
		store:   store,
		manager: manager,
		kind:    kind,
	}
}

func (d *AgentDriver) Kind() harness.RunKind {
	if d == nil || d.kind == "" {
		return harness.RunKindAgentTask
	}
	return d.kind
}

func (d *AgentDriver) Validate(spec harness.RunSpec) error {
	if d == nil || d.runner == nil || d.store == nil {
		return fmt.Errorf("agent runtime is not available")
	}
	if strings.TrimSpace(spec.Goal) == "" {
		return fmt.Errorf("goal is required")
	}
	return nil
}

func (d *AgentDriver) Start(ctx context.Context, run *harness.Run, _ harness.RunEnv) error {
	if d == nil || d.runner == nil {
		return fmt.Errorf("agent runtime is not available")
	}
	task := &agentpkg.Task{
		ID:             run.ID,
		UserID:         run.UserID,
		ConversationID: run.ConversationID,
		Goal:           run.Goal,
		Status:         agentpkg.TaskStatusPending,
		WorkspaceRoot:  run.WorkspaceRoot,
	}
	conversationCtx := metadataString(run.Metadata, "context")
	_, err := d.runner.SubmitTask(ctx, task, conversationCtx)
	return err
}

func (d *AgentDriver) Cancel(_ context.Context, run *harness.Run) error {
	if d == nil || d.runner == nil || run == nil {
		return fmt.Errorf("agent runtime is not available")
	}
	if !d.runner.Cancel(run.ID) {
		return fmt.Errorf("task not found or not running")
	}
	return nil
}

func (d *AgentDriver) Sync(ctx context.Context, run *harness.Run) (*harness.Run, error) {
	if d == nil || d.store == nil || run == nil {
		return run, nil
	}
	task, err := d.store.Get(ctx, run.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			return run, nil
		}
		return nil, err
	}
	snapshot := taskToRun(run, task, d.Kind())
	if err := d.manager.SyncSnapshot(ctx, snapshot); err != nil {
		return nil, err
	}
	return d.manager.GetStored(ctx, snapshot.ID)
}

func (d *AgentDriver) HandleTaskEvent(event agentpkg.TaskEvent) {
	if d == nil || d.manager == nil || strings.TrimSpace(event.TaskID) == "" {
		return
	}
	run, err := d.manager.GetStored(context.Background(), event.TaskID)
	if err == nil {
		if _, syncErr := d.Sync(context.Background(), run); syncErr == nil {
			run, _ = d.manager.GetStored(context.Background(), event.TaskID)
		}
	}
	eventType := mapAgentEventType(event.EventType)
	if eventType == "" {
		return
	}
	payloadJSON := ""
	if raw, err := json.Marshal(event); err == nil {
		payloadJSON = string(raw)
	}
	_ = d.manager.AppendEvent(context.Background(), harness.RunEvent{
		RunID:       event.TaskID,
		Type:        eventType,
		StepIndex:   event.StepIndex,
		Message:     strings.TrimSpace(event.Message),
		PayloadJSON: payloadJSON,
		CreatedAt:   timeutil.NowTime(),
	})
}

func mapAgentEventType(name string) string {
	switch strings.TrimSpace(name) {
	case "task_created":
		return ""
	case "task_state_transition":
		return "state_changed"
	case "task_planning":
		return "plan_generated"
	case "task_progress":
		return "step_started"
	case "task_step_completed":
		return "step_finished"
	case "task_question":
		return "question_requested"
	case "task_question_answered", "task_question_timeout":
		return "question_resolved"
	case "task_completed":
		return "run_completed"
	case "task_failed":
		return "run_failed"
	case "task_cancelled":
		return "run_cancelled"
	default:
		return name
	}
}

func taskToRun(existing *harness.Run, task *agentpkg.Task, kind harness.RunKind) *harness.Run {
	run := &harness.Run{}
	if existing != nil {
		*run = *existing
		run.Metadata = cloneMap(existing.Metadata)
	}
	run.ID = task.ID
	run.Kind = kind
	run.UserID = task.UserID
	run.ConversationID = task.ConversationID
	run.SessionID = task.ConversationID
	run.Goal = task.Goal
	run.Status = taskStatusToRunStatus(task.Status)
	run.RuntimeState = task.RuntimeState
	run.CurrentStep = task.CurrentStep
	run.Progress = task.Progress
	run.Result = task.Result
	run.Error = task.Error
	run.UpdatedAt = task.UpdatedAt
	run.CreatedAt = task.CreatedAt
	started := firstTaskStart(task)
	if started != nil {
		run.StartedAt = started
	}
	if isTaskTerminal(task.Status) {
		finished := task.UpdatedAt
		run.FinishedAt = &finished
	}
	return run
}

func taskStatusToRunStatus(status agentpkg.TaskStatus) harness.RunStatus {
	switch status {
	case agentpkg.TaskStatusPlanning:
		return harness.RunStatusPlanning
	case agentpkg.TaskStatusWaitingInput:
		return harness.RunStatusWaitingInput
	case agentpkg.TaskStatusExecuting:
		if status == agentpkg.TaskStatusExecuting {
			return harness.RunStatusExecuting
		}
	case agentpkg.TaskStatusCompleted:
		return harness.RunStatusCompleted
	case agentpkg.TaskStatusFailed:
		return harness.RunStatusFailed
	case agentpkg.TaskStatusCancelled:
		return harness.RunStatusCancelled
	case agentpkg.TaskStatusAborted:
		return harness.RunStatusAborted
	}
	if status == agentpkg.TaskStatusExecuting {
		return harness.RunStatusExecuting
	}
	return harness.RunStatusPending
}

func isTaskTerminal(status agentpkg.TaskStatus) bool {
	switch status {
	case agentpkg.TaskStatusCompleted, agentpkg.TaskStatusFailed, agentpkg.TaskStatusCancelled, agentpkg.TaskStatusAborted:
		return true
	default:
		return false
	}
}

func firstTaskStart(task *agentpkg.Task) *time.Time {
	if task == nil {
		return nil
	}
	for _, step := range task.Plan {
		if step.StartedAt != nil && !step.StartedAt.IsZero() {
			ts := *step.StartedAt
			return &ts
		}
	}
	if !task.CreatedAt.IsZero() && task.Status != agentpkg.TaskStatusPending {
		ts := task.CreatedAt
		return &ts
	}
	return nil
}

func metadataString(meta map[string]interface{}, key string) string {
	if len(meta) == 0 {
		return ""
	}
	raw, _ := meta[key]
	if value, ok := raw.(string); ok {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(fmt.Sprint(raw))
}

func cloneMap(in map[string]interface{}) map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
