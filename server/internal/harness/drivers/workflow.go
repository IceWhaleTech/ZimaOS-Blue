package drivers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
)

const (
	workflowIDMetadataKey                 = "workflow_id"
	workflowExecutionIDMetadataKey        = "workflow_execution_id"
	workflowNameMetadataKey               = "workflow_name"
	workflowTriggerTypeMetadataKey        = "trigger_type"
	workflowStatusReasonMetadataKey       = "workflow_status_reason"
	workflowCheckpointKindMetadataKey     = "workflow_checkpoint_kind"
	workflowCheckpointReasonMetadataKey   = "workflow_checkpoint_reason"
	workflowCheckpointNodeNameMetadataKey = "workflow_checkpoint_node_name"
	workflowTenantIDMetadataKey           = "tenant_id"
)

type workflowRuntime interface {
	ExecuteWorkflow(ctx context.Context, id string, triggerData map[string]interface{}) (*workflow.Execution, error)
	GetExecution(ctx context.Context, id string) (*workflow.Execution, error)
	CancelExecution(ctx context.Context, id string) error
	GetExecutionLogs(ctx context.Context, executionID string, opts *workflow.ListOptions) ([]*workflow.ExecutionLog, int, error)
}

type workflowTriggerRuntime interface {
	ExecuteWorkflowWithTrigger(ctx context.Context, id string, triggerType workflow.TriggerType, triggerData map[string]interface{}) (*workflow.Execution, error)
}

type workflowDirectCancelRuntime interface {
	CancelExecutionDirect(ctx context.Context, id string) error
}

type workflowResumeRuntime interface {
	ResumeExecutionDirect(ctx context.Context, id string, resume workflow.ExecutionResumeInput) (*workflow.Execution, error)
}

type WorkflowDriver struct {
	service workflowRuntime
	manager *harness.Controller
}

func NewWorkflowDriver(service workflowRuntime, manager *harness.Controller) *WorkflowDriver {
	return &WorkflowDriver{service: service, manager: manager}
}

func (d *WorkflowDriver) Kind() harness.RunKind { return harness.RunKindWorkflow }

func (d *WorkflowDriver) Validate(spec harness.RunSpec) error {
	if d == nil || d.service == nil {
		return fmt.Errorf("workflow runtime is not available")
	}
	if strings.TrimSpace(spec.Goal) == "" {
		return fmt.Errorf("goal is required")
	}
	if workflowID := metadataString(spec.Metadata, workflowIDMetadataKey); workflowID == "" {
		return fmt.Errorf("workflow_id is required")
	}
	return nil
}

func (d *WorkflowDriver) Start(ctx context.Context, run *harness.Run, env harness.RunEnv) error {
	if d == nil || d.service == nil || run == nil {
		return fmt.Errorf("workflow runtime is not available")
	}
	workflowID := metadataString(run.Metadata, workflowIDMetadataKey)
	if workflowID == "" {
		return fmt.Errorf("workflow_id is required")
	}

	ctx = workflowContextForRun(ctx, run)
	triggerType := workflowTriggerTypeForRun(run)
	var execution *workflow.Execution
	var err error
	if runtime, ok := d.service.(workflowTriggerRuntime); ok {
		execution, err = runtime.ExecuteWorkflowWithTrigger(ctx, workflowID, triggerType, metadataMap(run.Metadata["trigger_data"]))
	} else {
		execution, err = d.service.ExecuteWorkflow(ctx, workflowID, metadataMap(run.Metadata["trigger_data"]))
	}
	if err != nil {
		return err
	}
	if execution == nil {
		return fmt.Errorf("workflow execution was not created")
	}
	snapshot := workflowExecutionToRun(run, execution)
	return d.persistWorkflowSnapshot(ctx, run, snapshot, env.Manager)
}

func (d *WorkflowDriver) Cancel(ctx context.Context, run *harness.Run) error {
	if d == nil || d.service == nil || run == nil {
		return fmt.Errorf("workflow runtime is not available")
	}
	executionID := metadataString(run.Metadata, workflowExecutionIDMetadataKey)
	if executionID == "" {
		return fmt.Errorf("workflow_execution_id is required")
	}
	if runtime, ok := d.service.(workflowDirectCancelRuntime); ok {
		return runtime.CancelExecutionDirect(workflowContextForRun(ctx, run), executionID)
	}
	return d.service.CancelExecution(workflowContextForRun(ctx, run), executionID)
}

func (d *WorkflowDriver) PerformAction(ctx context.Context, run *harness.Run, action string, input map[string]interface{}) (*harness.Run, error) {
	if d == nil || d.service == nil || run == nil {
		return nil, fmt.Errorf("workflow runtime is not available")
	}
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "resume":
		runtime, ok := d.service.(workflowResumeRuntime)
		if !ok {
			return nil, fmt.Errorf("workflow runtime does not support resume control")
		}
		executionID := metadataString(run.Metadata, workflowExecutionIDMetadataKey)
		if executionID == "" {
			return nil, fmt.Errorf("workflow_execution_id is required")
		}
		execution, err := runtime.ResumeExecutionDirect(workflowContextForRun(ctx, run), executionID, workflow.ExecutionResumeInput{
			Decision: metadataString(input, "decision"),
			Payload:  metadataMap(input["payload"]),
		})
		if err != nil {
			return nil, err
		}
		if execution == nil {
			return nil, fmt.Errorf("workflow execution was not updated")
		}
		return workflowExecutionToRun(run, execution), nil
	default:
		return nil, fmt.Errorf("unsupported run action %q", strings.TrimSpace(action))
	}
}

func (d *WorkflowDriver) Sync(ctx context.Context, run *harness.Run) (*harness.Run, error) {
	if d == nil || d.service == nil || run == nil {
		return run, nil
	}
	executionID := metadataString(run.Metadata, workflowExecutionIDMetadataKey)
	if executionID == "" {
		return run, nil
	}
	execution, err := d.service.GetExecution(workflowContextForRun(ctx, run), executionID)
	if err != nil {
		if err == workflow.ErrExecutionNotFound {
			return run, nil
		}
		return nil, err
	}
	if execution == nil {
		return run, nil
	}
	snapshot := workflowExecutionToRun(run, execution)
	controller := workflowDriverManager(d.manager, nil)
	if controller == nil {
		return snapshot, nil
	}
	if err := controller.SyncSnapshot(ctx, snapshot); err != nil {
		return nil, err
	}
	return controller.GetStored(ctx, run.ID)
}

func (d *WorkflowDriver) ListRuntimeEvidence(ctx context.Context, run *harness.Run) ([]harness.RuntimeEvidenceEntry, error) {
	if d == nil || d.service == nil || run == nil {
		return nil, nil
	}
	executionID := metadataString(run.Metadata, workflowExecutionIDMetadataKey)
	if executionID == "" {
		return nil, nil
	}
	logs, err := listWorkflowExecutionLogs(workflowContextForRun(ctx, run), d.service, executionID)
	if err != nil {
		if err == workflow.ErrExecutionNotFound {
			return nil, nil
		}
		return nil, err
	}
	out := make([]harness.RuntimeEvidenceEntry, 0, len(logs))
	for _, entry := range logs {
		payloadJSON := ""
		if raw, err := json.Marshal(entry); err == nil {
			payloadJSON = string(raw)
		}
		out = append(out, harness.RuntimeEvidenceEntry{
			ID:          entry.ID,
			RunID:       run.ID,
			EventType:   workflowLogEventType(entry.Level),
			Summary:     workflowLogSummary(entry),
			PayloadJSON: payloadJSON,
			CreatedAt:   entry.Timestamp,
		})
	}
	return out, nil
}

func (d *WorkflowDriver) persistWorkflowSnapshot(
	ctx context.Context,
	run *harness.Run,
	snapshot *harness.Run,
	manager *harness.Controller,
) error {
	if snapshot == nil {
		return nil
	}
	controller := workflowDriverManager(d.manager, manager)
	if controller != nil {
		return controller.SyncSnapshot(ctx, snapshot)
	}
	*run = *snapshot
	return nil
}

func workflowDriverManager(primary *harness.Controller, secondary *harness.Controller) *harness.Controller {
	if secondary != nil {
		return secondary
	}
	return primary
}

func workflowContextForRun(ctx context.Context, run *harness.Run) context.Context {
	if run == nil {
		return ctx
	}
	return workflow.WithTenantContext(ctx, metadataString(run.Metadata, workflowTenantIDMetadataKey))
}

func workflowExecutionToRun(existing *harness.Run, execution *workflow.Execution) *harness.Run {
	if execution == nil {
		return existing
	}
	run := &harness.Run{}
	if existing != nil {
		*run = *existing
		run.Metadata = cloneMap(existing.Metadata)
	}
	run.Kind = harness.RunKindWorkflow
	run.Status = workflowExecutionStatusToRunStatus(execution.Status)
	run.Result = ""
	run.Error = strings.TrimSpace(execution.Error)
	run.UpdatedAt = workflowExecutionUpdatedAt(run, execution)
	if !execution.StartedAt.IsZero() {
		started := execution.StartedAt
		run.StartedAt = &started
	}
	if execution.CompletedAt != nil && !execution.CompletedAt.IsZero() {
		finished := *execution.CompletedAt
		run.FinishedAt = &finished
	} else {
		run.FinishedAt = nil
	}
	if run.Metadata == nil {
		run.Metadata = map[string]interface{}{}
	}
	setOrDeleteMetadata(run.Metadata, workflowIDMetadataKey, strings.TrimSpace(execution.WorkflowID))
	setOrDeleteMetadata(run.Metadata, workflowExecutionIDMetadataKey, strings.TrimSpace(execution.ID))
	setOrDeleteMetadata(run.Metadata, workflowNameMetadataKey, strings.TrimSpace(execution.WorkflowName))
	setOrDeleteMetadata(run.Metadata, workflowTriggerTypeMetadataKey, firstNonEmptyDriverString(
		strings.TrimSpace(string(execution.TriggerType)),
		metadataString(run.Metadata, workflowTriggerTypeMetadataKey),
		string(workflowTriggerTypeForRun(existing)),
	))
	setOrDeleteMetadata(run.Metadata, workflowStatusReasonMetadataKey, strings.TrimSpace(execution.StatusReason))
	setOrDeleteMetadata(run.Metadata, workflowCheckpointKindMetadataKey, workflowCheckpointKind(execution))
	setOrDeleteMetadata(run.Metadata, workflowCheckpointReasonMetadataKey, workflowCheckpointReason(execution))
	setOrDeleteMetadata(run.Metadata, workflowCheckpointNodeNameMetadataKey, workflowCheckpointNodeName(execution))
	setOrDeleteMetadata(run.Metadata, workflowTenantIDMetadataKey, strings.TrimSpace(execution.TenantID))
	return run
}

func workflowTriggerTypeForRun(run *harness.Run) workflow.TriggerType {
	if run == nil {
		return workflow.TriggerTypeManual
	}
	triggerType := workflow.TriggerType(strings.TrimSpace(metadataString(run.Metadata, workflowTriggerTypeMetadataKey)))
	if triggerType == "" {
		return workflow.TriggerTypeManual
	}
	return triggerType
}

func firstNonEmptyDriverString(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func workflowExecutionStatusToRunStatus(status workflow.ExecutionStatus) harness.RunStatus {
	switch status {
	case workflow.ExecutionStatusRunning:
		return harness.RunStatusExecuting
	case workflow.ExecutionStatusCompleted:
		return harness.RunStatusCompleted
	case workflow.ExecutionStatusFailed:
		return harness.RunStatusFailed
	case workflow.ExecutionStatusCancelled:
		return harness.RunStatusCancelled
	case workflow.ExecutionStatusPaused:
		return harness.RunStatusWaitingInput
	default:
		return harness.RunStatusPending
	}
}

func workflowExecutionUpdatedAt(run *harness.Run, execution *workflow.Execution) time.Time {
	if execution == nil {
		if run != nil {
			return run.UpdatedAt
		}
		return timeutil.NowTime()
	}
	if execution.CompletedAt != nil && !execution.CompletedAt.IsZero() {
		return *execution.CompletedAt
	}
	return timeutil.NowTime()
}

func workflowCheckpointKind(execution *workflow.Execution) string {
	if execution == nil || execution.Checkpoint == nil {
		return ""
	}
	return strings.TrimSpace(string(execution.Checkpoint.Kind))
}

func workflowCheckpointReason(execution *workflow.Execution) string {
	if execution == nil || execution.Checkpoint == nil {
		return ""
	}
	return strings.TrimSpace(execution.Checkpoint.Reason)
}

func workflowCheckpointNodeName(execution *workflow.Execution) string {
	if execution == nil || execution.Checkpoint == nil {
		return ""
	}
	return strings.TrimSpace(execution.Checkpoint.NodeName)
}

func setOrDeleteMetadata(meta map[string]interface{}, key string, value string) {
	if meta == nil {
		return
	}
	if value == "" {
		delete(meta, key)
		return
	}
	meta[key] = value
}

func listWorkflowExecutionLogs(ctx context.Context, service workflowRuntime, executionID string) ([]*workflow.ExecutionLog, error) {
	const pageSize = 200

	out := make([]*workflow.ExecutionLog, 0, pageSize)
	total := pageSize
	for offset := 0; offset < total; offset += pageSize {
		logs, count, err := service.GetExecutionLogs(ctx, executionID, &workflow.ListOptions{
			Limit:  pageSize,
			Offset: offset,
		})
		if err != nil {
			return nil, err
		}
		if count > total {
			total = count
		}
		out = append(out, logs...)
		if len(logs) == 0 || len(out) >= count {
			break
		}
	}
	return out, nil
}

func workflowLogEventType(level string) string {
	level = strings.ToLower(strings.TrimSpace(level))
	if level == "" {
		return "workflow_log"
	}
	return "workflow_log_" + level
}

func workflowLogSummary(entry *workflow.ExecutionLog) string {
	if entry == nil {
		return ""
	}
	message := strings.TrimSpace(entry.Message)
	nodeID := strings.TrimSpace(entry.NodeID)
	switch {
	case nodeID != "" && message != "":
		return nodeID + ": " + message
	case message != "":
		return message
	case nodeID != "":
		return nodeID
	default:
		return "workflow log"
	}
}
