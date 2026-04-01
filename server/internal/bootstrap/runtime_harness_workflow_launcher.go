package bootstrap

import (
	"context"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	harnessdrivers "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness/drivers"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
)

func (l *harnessWorkflowExecutionLauncher) LaunchExecution(ctx context.Context, req workflow.ExecutionLaunchRequest) (*workflow.Execution, error) {
	if l == nil || l.manager == nil || l.resolve == nil {
		return nil, fmt.Errorf("workflow launcher is not configured")
	}
	service := l.resolve()
	if service == nil {
		return nil, fmt.Errorf("workflow service unavailable")
	}
	l.ensureWorkflowDriver(service)
	ctx = workflow.WithTenantContext(ctx, strings.TrimSpace(req.TenantID))
	item, err := service.GetWorkflow(ctx, strings.TrimSpace(req.WorkflowID))
	if err != nil {
		return nil, err
	}
	tenantID := firstNonEmptyWorkflowString(strings.TrimSpace(req.TenantID), item.TenantID)
	ctx = workflow.WithTenantContext(ctx, tenantID)
	run, err := l.manager.Submit(ctx, newWorkflowHarnessRunSpec(workflowHarnessRunInput{
		WorkflowID:     item.ID,
		WorkflowName:   item.Name,
		TriggerType:    req.TriggerType,
		UserID:         strings.TrimSpace(req.UserID),
		ConversationID: workflowConversationID(req.ConversationID, req.TriggerData),
		TenantID:       tenantID,
		WorkspaceRoot:  l.defaultWorkspaceRoot,
		TriggerData:    cloneWorkflowMetadataMap(req.TriggerData),
	}))
	if err != nil {
		return nil, err
	}
	executionID := workflowMetadataString(run.Metadata, "workflow_execution_id")
	if executionID != "" {
		execution, err := service.GetExecution(ctx, executionID)
		if err == nil && execution != nil {
			return execution, nil
		}
	}
	return workflowExecutionFromRun(run, item, cloneWorkflowMetadataMap(req.TriggerData), tenantID), nil
}

func (l *harnessWorkflowExecutionLauncher) CancelExecution(ctx context.Context, executionID string, reason string) (bool, error) {
	if l == nil || l.manager == nil {
		return false, nil
	}
	run, err := l.manager.FindRunByMetadata(ctx, harness.RunKindWorkflow, "workflow_execution_id", executionID)
	if err != nil {
		return false, err
	}
	if run == nil {
		return false, nil
	}
	return true, l.manager.Cancel(ctx, run.ID, strings.TrimSpace(reason))
}

func (l *harnessWorkflowExecutionLauncher) ResumeExecution(
	ctx context.Context,
	executionID string,
	resume workflow.ExecutionResumeInput,
) (bool, *workflow.Execution, error) {
	if l == nil || l.manager == nil || l.resolve == nil {
		return false, nil, nil
	}
	run, err := l.manager.FindRunByMetadata(ctx, harness.RunKindWorkflow, "workflow_execution_id", executionID)
	if err != nil {
		return false, nil, err
	}
	if run == nil {
		return false, nil, nil
	}
	service := l.resolve()
	if service == nil {
		return true, nil, fmt.Errorf("workflow service unavailable")
	}
	l.ensureWorkflowDriver(service)
	ctx = workflow.WithTenantContext(ctx, workflowMetadataString(run.Metadata, "tenant_id"))
	updatedRun, err := l.manager.PerformAction(ctx, run.ID, "resume", map[string]interface{}{
		"decision": strings.TrimSpace(resume.Decision),
		"payload":  cloneWorkflowMetadataMap(resume.Payload),
	})
	if err != nil {
		return true, nil, err
	}
	if updatedRun == nil {
		updatedRun = run
	}
	if updatedExecutionID := workflowMetadataString(updatedRun.Metadata, "workflow_execution_id"); updatedExecutionID != "" {
		executionID = updatedExecutionID
	}
	execution, err := service.GetExecution(ctx, executionID)
	if err == nil && execution != nil {
		return true, execution, nil
	}
	return true, workflowExecutionFromRun(updatedRun, nil, nil, workflowMetadataString(updatedRun.Metadata, "tenant_id")), nil
}

func (l *harnessWorkflowExecutionLauncher) ensureWorkflowDriver(service *workflow.WorkflowService) {
	if l == nil || l.manager == nil || service == nil {
		return
	}
	if l.manager.GetRegisteredDriver(harness.RunKindWorkflow) != nil {
		return
	}
	l.manager.RegisterDriver(harnessdrivers.NewWorkflowDriver(service, l.manager))
}
