package tools

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
)

type stubNodesService struct {
	workflows map[string]*workflow.Workflow
	runID     string
}

func (s *stubNodesService) ListWorkflows(ctx context.Context, tenantID string, opts *workflow.ListOptions) ([]*workflow.Workflow, int, error) {
	_ = ctx
	_ = tenantID
	_ = opts
	items := make([]*workflow.Workflow, 0, len(s.workflows))
	for _, item := range s.workflows {
		copy := *item
		items = append(items, &copy)
	}
	return items, len(items), nil
}
func (s *stubNodesService) GetWorkflow(ctx context.Context, id string) (*workflow.Workflow, error) {
	_ = ctx
	item, ok := s.workflows[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	copy := *item
	return &copy, nil
}
func (s *stubNodesService) CreateWorkflow(ctx context.Context, item *workflow.Workflow) (*workflow.Workflow, error) {
	_ = ctx
	if s.workflows == nil {
		s.workflows = map[string]*workflow.Workflow{}
	}
	copy := *item
	if copy.ID == "" {
		copy.ID = "wf_1"
	}
	copy.CreatedAt = time.Now()
	copy.UpdatedAt = copy.CreatedAt
	copy.Version = 1
	item = &copy
	s.workflows[item.ID] = item
	return item, nil
}
func (s *stubNodesService) UpdateWorkflow(ctx context.Context, item *workflow.Workflow) (*workflow.Workflow, error) {
	_ = ctx
	copy := *item
	copy.UpdatedAt = time.Now()
	s.workflows[item.ID] = &copy
	return &copy, nil
}
func (s *stubNodesService) DeleteWorkflow(ctx context.Context, id string) error {
	_ = ctx
	delete(s.workflows, id)
	return nil
}
func (s *stubNodesService) ExecuteWorkflow(ctx context.Context, id string, triggerData map[string]interface{}) (*workflow.Execution, error) {
	_ = ctx
	_ = triggerData
	s.runID = id
	return &workflow.Execution{ID: "exec_1", WorkflowID: id}, nil
}
func (s *stubNodesService) EnableWorkflow(ctx context.Context, id string) error {
	_ = ctx
	if item, ok := s.workflows[id]; ok {
		item.Status = workflow.WorkflowStatusActive
	}
	return nil
}
func (s *stubNodesService) DisableWorkflow(ctx context.Context, id string) error {
	_ = ctx
	if item, ok := s.workflows[id]; ok {
		item.Status = workflow.WorkflowStatusInactive
	}
	return nil
}
func (s *stubNodesService) Templates(ctx context.Context) []workflow.WorkflowTemplateResponse {
	_ = ctx
	return workflow.DefaultTemplates()
}

func TestNodesToolTemplatesExecute(t *testing.T) {
	tool := NewNodesTool(&stubNodesService{})
	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "templates"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	templates := payload["templates"].([]workflow.WorkflowTemplateResponse)
	if len(templates) == 0 {
		t.Fatal("expected templates")
	}
}

func TestNodesToolCreateFromTemplateExecute(t *testing.T) {
	svc := &stubNodesService{}
	tool := NewNodesTool(svc)
	ctx := WithUserID(context.Background(), "user-1")
	result, err := tool.Execute(ctx, map[string]interface{}{
		"action":      "create",
		"template_id": "scheduled-task",
		"name":        "Nightly Check",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	item := payload["workflow"].(*workflow.Workflow)
	if item.Name != "Nightly Check" || len(item.Nodes) == 0 {
		t.Fatalf("unexpected workflow: %#v", item)
	}
}

func TestNodesToolRunExecute(t *testing.T) {
	svc := &stubNodesService{workflows: map[string]*workflow.Workflow{"wf_1": {ID: "wf_1", Name: "Test"}}}
	tool := NewNodesTool(svc)
	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "run", "id": "wf_1", "trigger_data": map[string]interface{}{"x": 1}})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	execution := payload["execution"].(*workflow.Execution)
	if execution.WorkflowID != "wf_1" || svc.runID != "wf_1" {
		t.Fatalf("unexpected execution: %#v serviceRun=%q", execution, svc.runID)
	}
}
