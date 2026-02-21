package workflow

import (
	"context"
	"fmt"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
)

// SkillAdapter adapts WorkflowService to builtin.WorkflowServiceInterface.
// Supports lazy initialization via a resolver function.
type SkillAdapter struct {
	resolve func() *WorkflowService
}

// NewSkillAdapter creates a new lazy skill adapter for the workflow service.
func NewSkillAdapter(resolve func() *WorkflowService) *SkillAdapter {
	return &SkillAdapter{resolve: resolve}
}

func (a *SkillAdapter) svc() (*WorkflowService, error) {
	s := a.resolve()
	if s == nil {
		return nil, fmt.Errorf("workflow service not available")
	}
	return s, nil
}

func (a *SkillAdapter) CreateWorkflow(ctx context.Context, name, description string, status string) (builtin.WorkflowInfo, error) {
	s, err := a.svc()
	if err != nil {
		return builtin.WorkflowInfo{}, err
	}
	wf := &Workflow{
		Name:        name,
		Description: description,
		Status:      WorkflowStatus(status),
	}
	created, err := s.CreateWorkflow(ctx, wf)
	if err != nil {
		return builtin.WorkflowInfo{}, err
	}
	return toWorkflowInfo(created), nil
}

func (a *SkillAdapter) ListWorkflows(ctx context.Context) ([]builtin.WorkflowInfo, error) {
	s, err := a.svc()
	if err != nil {
		return nil, err
	}
	wfs, _, err := s.ListWorkflows(ctx, "", nil)
	if err != nil {
		return nil, err
	}
	results := make([]builtin.WorkflowInfo, len(wfs))
	for i, wf := range wfs {
		results[i] = toWorkflowInfo(wf)
	}
	return results, nil
}

func (a *SkillAdapter) GetWorkflow(ctx context.Context, id string) (builtin.WorkflowInfo, error) {
	s, err := a.svc()
	if err != nil {
		return builtin.WorkflowInfo{}, err
	}
	wf, err := s.GetWorkflow(ctx, id)
	if err != nil {
		return builtin.WorkflowInfo{}, err
	}
	return toWorkflowInfo(wf), nil
}

func (a *SkillAdapter) DeleteWorkflow(ctx context.Context, id string) error {
	s, err := a.svc()
	if err != nil {
		return err
	}
	return s.DeleteWorkflow(ctx, id)
}

func (a *SkillAdapter) EnableWorkflow(ctx context.Context, id string) error {
	s, err := a.svc()
	if err != nil {
		return err
	}
	return s.EnableWorkflow(ctx, id)
}

func (a *SkillAdapter) DisableWorkflow(ctx context.Context, id string) error {
	s, err := a.svc()
	if err != nil {
		return err
	}
	return s.DisableWorkflow(ctx, id)
}

func (a *SkillAdapter) ExecuteWorkflow(ctx context.Context, id string, triggerData map[string]interface{}) (builtin.WorkflowExecutionInfo, error) {
	s, err := a.svc()
	if err != nil {
		return builtin.WorkflowExecutionInfo{}, err
	}
	exec, err := s.ExecuteWorkflow(ctx, id, triggerData)
	if err != nil {
		return builtin.WorkflowExecutionInfo{}, err
	}
	return builtin.WorkflowExecutionInfo{
		ID:           exec.ID,
		WorkflowID:   exec.WorkflowID,
		WorkflowName: exec.WorkflowName,
		Status:       string(exec.Status),
		Error:        exec.Error,
	}, nil
}

func toWorkflowInfo(wf *Workflow) builtin.WorkflowInfo {
	return builtin.WorkflowInfo{
		ID:          wf.ID,
		Name:        wf.Name,
		Description: wf.Description,
		Status:      string(wf.Status),
		NodeCount:   len(wf.Nodes),
		Version:     wf.Version,
	}
}
