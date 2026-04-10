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
	contract := harness.DecodeHarnessContract(run.Metadata)
	metadata := cloneMap(run.Metadata)
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	metadata["harness_silent_mode"] = true
	metadata["non_interactive"] = true
	metadata["skip_hil"] = true
	if approvalMode := strings.TrimSpace(string(run.ApprovalMode)); approvalMode != "" {
		metadata["approval_mode"] = approvalMode
	}
	if model := strings.TrimSpace(run.Model); model != "" {
		if _, ok := metadata["model"]; !ok {
			metadata["model"] = model
		}
	}
	if providerID := strings.TrimSpace(run.ProviderID); providerID != "" {
		if _, ok := metadata["provider_id"]; !ok {
			metadata["provider_id"] = providerID
		}
	}
	task := &agentpkg.Task{
		ID:              run.ID,
		UserID:          run.UserID,
		ConversationID:  runConversationID(run),
		Goal:            run.Goal,
		Status:          agentpkg.TaskStatusPending,
		WorkspaceRoot:   run.WorkspaceRoot,
		SuccessCriteria: harness.HarnessContractSuccessCriteria(contract),
		FallbackPlan:    harness.HarnessContractFallbackPlan(contract),
		Metadata:        metadata,
	}
	conversationCtx := composeConversationContext(run.Metadata)
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

func (d *AgentDriver) ListRuntimeEvidence(ctx context.Context, run *harness.Run) ([]harness.RuntimeEvidenceEntry, error) {
	if d == nil || d.store == nil || run == nil {
		return nil, nil
	}
	events, err := d.store.ListRuntimeEvents(ctx, run.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	out := make([]harness.RuntimeEvidenceEntry, 0, len(events))
	for _, event := range events {
		out = append(out, harness.RuntimeEvidenceEntry{
			ID:           event.ID,
			RunID:        run.ID,
			StepIndex:    event.StepIndex,
			PlannerRound: event.PlannerRound,
			EventType:    event.EventType,
			Summary:      runtimeEvidenceSummary(event),
			PayloadJSON:  event.PayloadJSON,
			CreatedAt:    event.CreatedAt,
		})
	}
	return out, nil
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
	run.ConversationID = mergedRunConversationID(existing, task)
	run.SessionID = mergedRunSessionID(existing, task)
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
	raw, ok := meta[key]
	if !ok {
		return ""
	}
	if value, ok := raw.(string); ok {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(fmt.Sprint(raw))
}

func composeConversationContext(meta map[string]interface{}) string {
	parts := make([]string, 0, 4)
	if base := metadataString(meta, "context"); base != "" {
		parts = append(parts, base)
	}
	if contractContext := harness.BuildHarnessContractContext(harness.DecodeHarnessContract(meta)); contractContext != "" {
		parts = append(parts, contractContext)
	}
	if checkpointContext := harness.BuildHarnessCheckpointContext(metadataMap(meta["resume_checkpoint"])); checkpointContext != "" {
		parts = append(parts, checkpointContext)
	}
	if retry := metadataString(meta, "retry_context"); retry != "" {
		parts = append(parts, retry)
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func runConversationID(run *harness.Run) string {
	if run == nil {
		return ""
	}
	return firstNonEmpty(run.ConversationID, run.SessionID)
}

func mergedRunConversationID(existing *harness.Run, task *agentpkg.Task) string {
	if task == nil {
		if existing == nil {
			return ""
		}
		return firstNonEmpty(existing.ConversationID, existing.SessionID)
	}
	if existing == nil {
		return strings.TrimSpace(task.ConversationID)
	}
	return firstNonEmpty(task.ConversationID, existing.ConversationID, existing.SessionID)
}

func mergedRunSessionID(existing *harness.Run, task *agentpkg.Task) string {
	if task == nil {
		if existing == nil {
			return ""
		}
		return firstNonEmpty(existing.SessionID, existing.ConversationID)
	}
	if existing == nil {
		return strings.TrimSpace(task.ConversationID)
	}
	return firstNonEmpty(task.ConversationID, existing.SessionID, existing.ConversationID)
}

func runtimeEvidenceSummary(event agentpkg.RuntimeEvent) string {
	payload := decodePayloadJSON(event.PayloadJSON)
	switch strings.TrimSpace(event.EventType) {
	case "tool_call":
		return firstNonEmpty(metadataString(payload, "tool"), metadataString(payload, "tool_name"), "tool call")
	case "tool_result":
		parts := []string{firstNonEmpty(metadataString(payload, "tool"), metadataString(payload, "tool_name"), "tool result")}
		if exitCode := metadataString(payload, "exit_code"); exitCode != "" {
			parts = append(parts, "exit "+exitCode)
		}
		return strings.TrimSpace(strings.Join(parts, " "))
	case "state_update":
		return firstNonEmpty(metadataString(payload, "kind"), "state update")
	default:
		return strings.TrimSpace(event.EventType)
	}
}

func decodePayloadJSON(raw string) map[string]interface{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
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
