package bootstrap

import (
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
)

type workflowHarnessRunInput struct {
	WorkflowID     string
	WorkflowName   string
	TriggerType    workflow.TriggerType
	UserID         string
	ConversationID string
	TenantID       string
	WorkspaceRoot  string
	TriggerData    map[string]interface{}
}

func newWorkflowHarnessRunSpec(input workflowHarnessRunInput) harness.RunSpec {
	workflowID := strings.TrimSpace(input.WorkflowID)
	workflowName := strings.TrimSpace(input.WorkflowName)
	triggerType := strings.TrimSpace(string(input.TriggerType))
	if triggerType == "" {
		triggerType = string(workflow.TriggerTypeManual)
	}
	goal := workflowName
	if goal == "" {
		if workflowID != "" {
			goal = fmt.Sprintf("Workflow %s", workflowID)
		} else {
			goal = "Workflow run"
		}
	}
	return harness.RunSpec{
		Kind:           harness.RunKindWorkflow,
		Goal:           goal,
		UserID:         strings.TrimSpace(input.UserID),
		ConversationID: strings.TrimSpace(input.ConversationID),
		SessionID:      strings.TrimSpace(input.ConversationID),
		WorkspaceRoot:  strings.TrimSpace(input.WorkspaceRoot),
		Metadata: map[string]interface{}{
			"workflow_id":   workflowID,
			"workflow_name": workflowName,
			"trigger_type":  triggerType,
			"tenant_id":     strings.TrimSpace(input.TenantID),
			"trigger_data":  cloneWorkflowMetadataMap(input.TriggerData),
		},
	}
}
