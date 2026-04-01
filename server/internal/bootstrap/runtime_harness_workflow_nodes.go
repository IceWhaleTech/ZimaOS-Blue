package bootstrap

import (
	"context"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
)

func (s harnessWorkflowNodesService) service() (*workflow.WorkflowService, error) {
	if s.resolve == nil {
		return nil, fmt.Errorf("workflow service unavailable")
	}
	runtime := s.resolve()
	if runtime == nil {
		return nil, fmt.Errorf("workflow service unavailable")
	}
	return runtime, nil
}

func (s harnessWorkflowNodesService) ListWorkflows(ctx context.Context, tenantID string, opts *workflow.ListOptions) ([]*workflow.Workflow, int, error) {
	runtime, err := s.service()
	if err != nil {
		return nil, 0, err
	}
	return runtime.ListWorkflows(workflow.WithTenantContext(ctx, tenantID), tenantID, opts)
}

func (s harnessWorkflowNodesService) GetWorkflow(ctx context.Context, id string) (*workflow.Workflow, error) {
	runtime, err := s.service()
	if err != nil {
		return nil, err
	}
	return runtime.GetWorkflow(ctx, id)
}

func (s harnessWorkflowNodesService) CreateWorkflow(ctx context.Context, item *workflow.Workflow) (*workflow.Workflow, error) {
	runtime, err := s.service()
	if err != nil {
		return nil, err
	}
	return runtime.CreateWorkflow(ctx, item)
}

func (s harnessWorkflowNodesService) UpdateWorkflow(ctx context.Context, item *workflow.Workflow) (*workflow.Workflow, error) {
	runtime, err := s.service()
	if err != nil {
		return nil, err
	}
	return runtime.UpdateWorkflow(ctx, item)
}

func (s harnessWorkflowNodesService) DeleteWorkflow(ctx context.Context, id string) error {
	runtime, err := s.service()
	if err != nil {
		return err
	}
	return runtime.DeleteWorkflow(ctx, id)
}

func (s harnessWorkflowNodesService) ExecuteWorkflow(ctx context.Context, id string, triggerData map[string]interface{}) (*workflow.Execution, error) {
	if s.launcher != nil {
		return s.launcher.LaunchExecution(ctx, workflow.ExecutionLaunchRequest{
			WorkflowID:     strings.TrimSpace(id),
			TriggerType:    workflow.TriggerTypeManual,
			TriggerData:    cloneWorkflowMetadataMap(triggerData),
			UserID:         strings.TrimSpace(tools.GetUserID(ctx)),
			ConversationID: workflowConversationID(tools.GetSessionID(ctx), triggerData),
			TenantID:       firstNonEmptyWorkflowString(workflow.TenantFromContext(ctx), workflowTriggerString(triggerData, "tenant_id", "tenantId")),
		})
	}
	runtime, err := s.service()
	if err != nil {
		return nil, err
	}
	return runtime.ExecuteWorkflow(ctx, id, triggerData)
}

func (s harnessWorkflowNodesService) EnableWorkflow(ctx context.Context, id string) error {
	runtime, err := s.service()
	if err != nil {
		return err
	}
	return runtime.EnableWorkflow(ctx, id)
}

func (s harnessWorkflowNodesService) DisableWorkflow(ctx context.Context, id string) error {
	runtime, err := s.service()
	if err != nil {
		return err
	}
	return runtime.DisableWorkflow(ctx, id)
}

func (s harnessWorkflowNodesService) Templates(ctx context.Context) []workflow.WorkflowTemplateResponse {
	_ = ctx
	return workflow.DefaultTemplates()
}
