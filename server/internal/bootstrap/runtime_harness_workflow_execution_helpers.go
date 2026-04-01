package bootstrap

import (
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
)

func workflowConversationID(fallback string, triggerData map[string]interface{}) string {
	return firstNonEmptyWorkflowString(
		strings.TrimSpace(fallback),
		workflowTriggerString(triggerData, "conversation_id", "conversationId", "session_id", "sessionId", "session"),
	)
}

func workflowTriggerString(triggerData map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value := workflowMetadataString(triggerData, key); value != "" {
			return value
		}
	}
	return ""
}

func workflowMetadataString(meta map[string]interface{}, key string) string {
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

func cloneWorkflowMetadataMap(in map[string]interface{}) map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func workflowExecutionFromRun(
	run *harness.Run,
	item *workflow.Workflow,
	triggerData map[string]interface{},
	tenantID string,
) *workflow.Execution {
	if run == nil {
		return nil
	}
	execution := &workflow.Execution{
		ID:           firstNonEmptyWorkflowString(workflowMetadataString(run.Metadata, "workflow_execution_id"), run.ID),
		WorkflowID:   firstNonEmptyWorkflowString(workflowMetadataString(run.Metadata, "workflow_id"), itemID(item)),
		WorkflowName: firstNonEmptyWorkflowString(workflowMetadataString(run.Metadata, "workflow_name"), itemName(item)),
		TenantID:     firstNonEmptyWorkflowString(tenantID, workflowMetadataString(run.Metadata, "tenant_id")),
		Status:       workflowExecutionStatusFromRun(run.Status),
		TriggerType:  workflowTriggerTypeFromMetadata(run.Metadata),
		TriggerData:  triggerData,
		Error:        strings.TrimSpace(run.Error),
		StartedAt:    run.CreatedAt,
	}
	if run.StartedAt != nil && !run.StartedAt.IsZero() {
		execution.StartedAt = *run.StartedAt
	}
	if run.FinishedAt != nil && !run.FinishedAt.IsZero() {
		finished := *run.FinishedAt
		execution.CompletedAt = &finished
	}
	return execution
}

func workflowTriggerTypeFromMetadata(meta map[string]interface{}) workflow.TriggerType {
	triggerType := workflow.TriggerType(strings.TrimSpace(workflowMetadataString(meta, "trigger_type")))
	if triggerType == "" {
		return workflow.TriggerTypeManual
	}
	return triggerType
}

func workflowExecutionStatusFromRun(status harness.RunStatus) workflow.ExecutionStatus {
	switch status {
	case harness.RunStatusExecuting, harness.RunStatusPlanning, harness.RunStatusVerifying:
		return workflow.ExecutionStatusRunning
	case harness.RunStatusCompleted:
		return workflow.ExecutionStatusCompleted
	case harness.RunStatusFailed, harness.RunStatusAborted:
		return workflow.ExecutionStatusFailed
	case harness.RunStatusCancelled:
		return workflow.ExecutionStatusCancelled
	case harness.RunStatusWaitingInput:
		return workflow.ExecutionStatusPaused
	default:
		return workflow.ExecutionStatusPending
	}
}

func itemID(item *workflow.Workflow) string {
	if item == nil {
		return ""
	}
	return strings.TrimSpace(item.ID)
}

func itemName(item *workflow.Workflow) string {
	if item == nil {
		return ""
	}
	return strings.TrimSpace(item.Name)
}

func firstNonEmptyWorkflowString(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
