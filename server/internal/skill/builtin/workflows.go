package builtin

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

// WorkflowServiceInterface defines the interface for workflow service used by the skill.
type WorkflowServiceInterface interface {
	CreateWorkflow(ctx context.Context, name, description string, status string) (WorkflowInfo, error)
	ListWorkflows(ctx context.Context) ([]WorkflowInfo, error)
	GetWorkflow(ctx context.Context, id string) (WorkflowInfo, error)
	DeleteWorkflow(ctx context.Context, id string) error
	EnableWorkflow(ctx context.Context, id string) error
	DisableWorkflow(ctx context.Context, id string) error
	ExecuteWorkflow(ctx context.Context, id string, triggerData map[string]interface{}) (WorkflowExecutionInfo, error)
}

// WorkflowInfo represents a workflow returned by the interface.
type WorkflowInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"`
	NodeCount   int    `json:"node_count"`
	Version     int    `json:"version"`
}

// WorkflowExecutionInfo represents a workflow execution.
type WorkflowExecutionInfo struct {
	ID           string `json:"id"`
	WorkflowID   string `json:"workflow_id"`
	WorkflowName string `json:"workflow_name"`
	Status       string `json:"status"`
	Error        string `json:"error,omitempty"`
}

// Workflows is a built-in skill for managing workflow automations.
type Workflows struct {
	manifest *skill.Manifest
	mu       sync.RWMutex
	svc      WorkflowServiceInterface
}

// NewWorkflows creates a new workflows skill.
func NewWorkflows() *Workflows {
	return &Workflows{
		manifest: &skill.Manifest{
			ID:          "workflows",
			Name:        "Workflows",
			Version:     "1.0.0",
			Description: "Create, manage, and execute n8n-style workflow automations. Workflows chain triggers, conditions, and actions (HTTP, LLM, shell, notifications) into automated pipelines.",
			Category:    "system",
			Icon:        "workflow",
			Tags:        []string{"workflow", "automation", "pipeline", "n8n", "trigger"},
			Inputs: []skill.Parameter{
				{
					Name:        "action",
					Type:        "string",
					Description: "Action: create, list, get, delete, enable, disable, execute",
					Required:    true,
				},
				{
					Name:        "name",
					Type:        "string",
					Description: "Workflow name (required for create)",
					Required:    false,
				},
				{
					Name:        "description",
					Type:        "string",
					Description: "Workflow description (optional for create)",
					Required:    false,
				},
				{
					Name:        "id",
					Type:        "string",
					Description: "Workflow ID (required for get, delete, enable, disable, execute)",
					Required:    false,
				},
				{
					Name:        "locale",
					Type:        "string",
					Description: "Language/locale code for localized responses (e.g., en-US, zh-CN)",
					Required:    false,
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "workflow",
					Type:        "object",
					Description: "Workflow details",
				},
				{
					Name:        "workflows",
					Type:        "array",
					Description: "List of workflows",
				},
			},
		},
	}
}

// SetWorkflowService injects the workflow service.
func (w *Workflows) SetWorkflowService(svc WorkflowServiceInterface) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.svc = svc
}

func (w *Workflows) Manifest() *skill.Manifest { return w.manifest }

func (w *Workflows) Validate(input map[string]any) error {
	action, ok := input["action"]
	if !ok {
		return fmt.Errorf("action is required")
	}
	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("action must be a string")
	}
	valid := map[string]bool{
		"create": true, "list": true, "get": true,
		"delete": true, "enable": true, "disable": true, "execute": true,
	}
	if !valid[actionStr] {
		return fmt.Errorf("invalid action: %s", actionStr)
	}
	switch actionStr {
	case "create":
		if _, ok := input["name"]; !ok {
			return fmt.Errorf("name is required for create")
		}
	case "get", "delete", "enable", "disable", "execute":
		if _, ok := input["id"]; !ok {
			return fmt.Errorf("id is required for %s", actionStr)
		}
	}
	return nil
}

func (w *Workflows) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	w.mu.RLock()
	svc := w.svc
	w.mu.RUnlock()

	if svc == nil {
		return skill.NewErrorResult(fmt.Errorf("workflow service not available")), nil
	}

	action := input["action"].(string)

	switch action {
	case "create":
		name := input["name"].(string)
		desc, _ := input["description"].(string)
		wf, err := svc.CreateWorkflow(ctx, name, desc, "draft")
		if err != nil {
			return skill.NewErrorResult(err), nil
		}
		return skill.NewResult(map[string]any{
			"workflow": wf,
			"message":  fmt.Sprintf("Workflow '%s' created (ID: %s, status: draft)", wf.Name, wf.ID),
		}), nil

	case "list":
		wfs, err := svc.ListWorkflows(ctx)
		if err != nil {
			return skill.NewErrorResult(err), nil
		}
		if len(wfs) == 0 {
			return skill.NewResult(map[string]any{"workflows": []WorkflowInfo{}, "count": 0, "message": "No workflows"}), nil
		}
		var lines []string
		for _, wf := range wfs {
			lines = append(lines, fmt.Sprintf("- %s (%s): %s [%d nodes]", wf.Name, wf.ID, wf.Status, wf.NodeCount))
		}
		return skill.NewResult(map[string]any{
			"workflows": wfs, "count": len(wfs),
			"message": fmt.Sprintf("%d workflows:\n%s", len(wfs), strings.Join(lines, "\n")),
		}), nil

	case "get":
		wf, err := svc.GetWorkflow(ctx, input["id"].(string))
		if err != nil {
			return skill.NewErrorResult(err), nil
		}
		return skill.NewResult(map[string]any{"workflow": wf}), nil

	case "delete":
		id := input["id"].(string)
		if err := svc.DeleteWorkflow(ctx, id); err != nil {
			return skill.NewErrorResult(err), nil
		}
		return skill.NewResult(map[string]any{"id": id, "deleted": true, "message": fmt.Sprintf("Workflow %s deleted", id)}), nil

	case "enable":
		id := input["id"].(string)
		if err := svc.EnableWorkflow(ctx, id); err != nil {
			return skill.NewErrorResult(err), nil
		}
		return skill.NewResult(map[string]any{"id": id, "enabled": true, "message": fmt.Sprintf("Workflow %s enabled", id)}), nil

	case "disable":
		id := input["id"].(string)
		if err := svc.DisableWorkflow(ctx, id); err != nil {
			return skill.NewErrorResult(err), nil
		}
		return skill.NewResult(map[string]any{"id": id, "disabled": true, "message": fmt.Sprintf("Workflow %s disabled", id)}), nil

	case "execute":
		id := input["id"].(string)
		exec, err := svc.ExecuteWorkflow(ctx, id, nil)
		if err != nil {
			return skill.NewErrorResult(err), nil
		}
		return skill.NewResult(map[string]any{
			"execution": exec,
			"message":   fmt.Sprintf("Workflow %s executed (execution: %s, status: %s)", id, exec.ID, exec.Status),
		}), nil
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}
