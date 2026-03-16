package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
)

// NodesService provides access to workflow/node management.
type NodesService interface {
	ListWorkflows(ctx context.Context, tenantID string, opts *workflow.ListOptions) ([]*workflow.Workflow, int, error)
	GetWorkflow(ctx context.Context, id string) (*workflow.Workflow, error)
	CreateWorkflow(ctx context.Context, workflow *workflow.Workflow) (*workflow.Workflow, error)
	UpdateWorkflow(ctx context.Context, workflow *workflow.Workflow) (*workflow.Workflow, error)
	DeleteWorkflow(ctx context.Context, id string) error
	ExecuteWorkflow(ctx context.Context, id string, triggerData map[string]interface{}) (*workflow.Execution, error)
	EnableWorkflow(ctx context.Context, id string) error
	DisableWorkflow(ctx context.Context, id string) error
	Templates(ctx context.Context) []workflow.WorkflowTemplateResponse
}

// NodesTool manages workflow graphs/nodes.
type NodesTool struct {
	service NodesService
}

// NewNodesTool creates a native nodes tool.
func NewNodesTool(service NodesService) *NodesTool {
	return &NodesTool{service: service}
}

// Definition returns the tool schema.
func (t *NodesTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "nodes",
		Description: "Manage workflow graphs and node-based automations.",
		Icon:        "nodes",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"list", "templates", "get", "create", "update", "delete", "run", "enable", "disable"},
					"description": "Nodes action. Defaults based on provided fields.",
				},
				"id":          map[string]interface{}{"type": "string", "description": "Workflow ID."},
				"template_id": map[string]interface{}{"type": "string", "description": "Optional built-in template ID for create."},
				"tenant_id":   map[string]interface{}{"type": "string", "description": "Optional tenant ID override (default: default)."},
				"name":        map[string]interface{}{"type": "string", "description": "Workflow name."},
				"description": map[string]interface{}{"type": "string", "description": "Workflow description."},
				"status":      map[string]interface{}{"type": "string", "description": "Workflow status: draft, active, inactive."},
				"nodes": map[string]interface{}{
					"type":        "array",
					"description": "Workflow nodes.",
					"items": map[string]interface{}{
						"type":                 "object",
						"additionalProperties": true,
					},
				},
				"connections": map[string]interface{}{
					"type":        "array",
					"description": "Workflow connections.",
					"items": map[string]interface{}{
						"type":                 "object",
						"additionalProperties": true,
					},
				},
				"variables": map[string]interface{}{"type": "object", "description": "Workflow variables."},
				"settings":  map[string]interface{}{"type": "object", "description": "Workflow settings."},
				"tags": map[string]interface{}{
					"type":        "array",
					"description": "Workflow tags.",
					"items":       map[string]interface{}{"type": "string"},
				},
				"offset":       map[string]interface{}{"type": "integer", "description": "List offset (default 0)."},
				"limit":        map[string]interface{}{"type": "integer", "description": "List limit (default 20, max 100)."},
				"trigger_data": map[string]interface{}{"type": "object", "description": "Execution payload for run."},
			},
		},
	}
}

// Execute dispatches the requested nodes action.
func (t *NodesTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.service == nil {
		return nil, errors.New("nodes service not available")
	}
	switch nodesAction(args) {
	case "list":
		return t.executeList(ctx, args)
	case "templates":
		return t.executeTemplates(ctx)
	case "get":
		return t.executeGet(ctx, args)
	case "create":
		return t.executeCreate(ctx, args)
	case "update":
		return t.executeUpdate(ctx, args)
	case "delete":
		return t.executeDelete(ctx, args)
	case "run":
		return t.executeRun(ctx, args)
	case "enable":
		return t.executeEnable(ctx, args)
	case "disable":
		return t.executeDisable(ctx, args)
	default:
		return nil, errors.New("unsupported nodes action")
	}
}

func (t *NodesTool) executeList(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	limit, err := fsAsInt(args, "limit", 20)
	if err != nil {
		return nil, err
	}
	offset, err := fsAsInt(args, "offset", 0)
	if err != nil {
		return nil, err
	}
	limit = fsClamp(limit, 1, 100)
	if offset < 0 {
		offset = 0
	}
	items, total, err := t.service.ListWorkflows(ctx, nodesTenantID(ctx, args), &workflow.ListOptions{Offset: offset, Limit: limit})
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"workflows": items, "count": len(items), "total": total}, nil
}

func (t *NodesTool) executeTemplates(ctx context.Context) (interface{}, error) {
	templates := t.service.Templates(ctx)
	return map[string]interface{}{"templates": templates, "count": len(templates)}, nil
}

func (t *NodesTool) executeGet(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id := firstCompatString(args, "id", "workflow_id", "workflowId")
	if id == "" {
		return nil, errors.New("id is required")
	}
	item, err := t.service.GetWorkflow(ctx, id)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"workflow": item}, nil
}

func (t *NodesTool) executeCreate(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	item, err := buildWorkflowFromArgs(ctx, t.service, args, nil)
	if err != nil {
		return nil, err
	}
	created, err := t.service.CreateWorkflow(ctx, item)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"workflow": created, "created": true, "id": created.ID}, nil
}

func (t *NodesTool) executeUpdate(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id := firstCompatString(args, "id", "workflow_id", "workflowId")
	if id == "" {
		return nil, errors.New("id is required")
	}
	existing, err := t.service.GetWorkflow(ctx, id)
	if err != nil {
		return nil, err
	}
	updated, err := buildWorkflowFromArgs(ctx, t.service, args, existing)
	if err != nil {
		return nil, err
	}
	updated.ID = id
	result, err := t.service.UpdateWorkflow(ctx, updated)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"workflow": result, "updated": true, "id": id}, nil
}

func (t *NodesTool) executeDelete(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id := firstCompatString(args, "id", "workflow_id", "workflowId")
	if id == "" {
		return nil, errors.New("id is required")
	}
	if err := t.service.DeleteWorkflow(ctx, id); err != nil {
		return nil, err
	}
	return map[string]interface{}{"deleted": true, "id": id}, nil
}

func (t *NodesTool) executeRun(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id := firstCompatString(args, "id", "workflow_id", "workflowId")
	if id == "" {
		return nil, errors.New("id is required")
	}
	triggerData, err := decodeMap(firstCompatRawValue(args, "trigger_data", "triggerData"))
	if err != nil {
		return nil, err
	}
	execution, err := t.service.ExecuteWorkflow(ctx, id, triggerData)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"execution": execution, "triggered": true, "id": execution.ID}, nil
}

func (t *NodesTool) executeEnable(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id := firstCompatString(args, "id", "workflow_id", "workflowId")
	if id == "" {
		return nil, errors.New("id is required")
	}
	if err := t.service.EnableWorkflow(ctx, id); err != nil {
		return nil, err
	}
	item, err := t.service.GetWorkflow(ctx, id)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"workflow": item, "enabled": true, "id": id}, nil
}

func (t *NodesTool) executeDisable(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id := firstCompatString(args, "id", "workflow_id", "workflowId")
	if id == "" {
		return nil, errors.New("id is required")
	}
	if err := t.service.DisableWorkflow(ctx, id); err != nil {
		return nil, err
	}
	item, err := t.service.GetWorkflow(ctx, id)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"workflow": item, "disabled": true, "id": id}, nil
}

func nodesAction(args map[string]interface{}) string {
	action := strings.ToLower(strings.TrimSpace(firstCompatString(args, "action", "op", "operation", "command")))
	switch action {
	case "", "list", "ls", "status":
		if firstCompatString(args, "template_id", "templateId") != "" && firstCompatString(args, "id", "workflow_id", "workflowId") == "" {
			return "create"
		}
		if firstCompatString(args, "id", "workflow_id", "workflowId") != "" {
			if hasNodesMutableArgs(args) {
				return "update"
			}
			return "get"
		}
		if hasNodesMutableArgs(args) {
			return "create"
		}
		return "list"
	case "templates":
		return "templates"
	case "get", "show", "read":
		return "get"
	case "create", "add":
		return "create"
	case "update", "edit", "patch":
		return "update"
	case "delete", "remove", "rm":
		return "delete"
	case "run", "execute", "trigger":
		return "run"
	case "enable":
		return "enable"
	case "disable", "pause":
		return "disable"
	default:
		return action
	}
}

func hasNodesMutableArgs(args map[string]interface{}) bool {
	for _, keys := range [][]string{{"name"}, {"title"}, {"description"}, {"nodes"}, {"connections"}, {"variables"}, {"settings"}, {"tags"}, {"status"}, {"template_id", "templateId"}} {
		if value, ok := compatArgValue(args, keys...); ok && value != nil && !(asString(value) == "") {
			return true
		}
	}
	return false
}

func nodesTenantID(ctx context.Context, args map[string]interface{}) string {
	if tenantID := firstCompatString(args, "tenant_id", "tenantId", "tenant"); tenantID != "" {
		return tenantID
	}
	return "default"
}

func firstCompatRawValue(args map[string]interface{}, keys ...string) interface{} {
	value, ok := compatArgValue(args, keys...)
	if !ok {
		return nil
	}
	return value
}

func buildWorkflowFromArgs(ctx context.Context, service NodesService, args map[string]interface{}, base *workflow.Workflow) (*workflow.Workflow, error) {
	item := &workflow.Workflow{}
	if base != nil {
		copy := *base
		item = &copy
	}
	if templateID := firstCompatString(args, "template_id", "templateId", "template"); templateID != "" && base == nil {
		tpl := findWorkflowTemplate(service.Templates(ctx), templateID)
		if tpl == nil {
			return nil, fmt.Errorf("template %q not found", templateID)
		}
		copy := tpl.Workflow
		item = &copy
	}
	if item.TenantID == "" {
		item.TenantID = nodesTenantID(ctx, args)
	}
	if name := firstCompatString(args, "name", "title"); name != "" {
		item.Name = name
	}
	if item.Name == "" {
		item.Name = "Untitled Workflow"
	}
	if desc := firstCompatString(args, "description"); desc != "" {
		item.Description = desc
	}
	if status := firstCompatString(args, "status"); status != "" {
		item.Status = workflow.WorkflowStatus(status)
	}
	if item.Status == "" {
		item.Status = workflow.WorkflowStatusDraft
	}
	if userID := GetUserID(ctx); userID != "" {
		if item.CreatedBy == "" {
			item.CreatedBy = userID
		}
		item.UpdatedBy = userID
	}
	if raw, ok := compatArgValue(args, "nodes"); ok && raw != nil {
		blob, _ := json.Marshal(raw)
		var nodes []workflow.Node
		if err := json.Unmarshal(blob, &nodes); err != nil {
			return nil, fmt.Errorf("decode nodes: %w", err)
		}
		item.Nodes = nodes
	}
	if raw, ok := compatArgValue(args, "connections"); ok && raw != nil {
		blob, _ := json.Marshal(raw)
		var connections []workflow.Connection
		if err := json.Unmarshal(blob, &connections); err != nil {
			return nil, fmt.Errorf("decode connections: %w", err)
		}
		item.Connections = connections
	}
	if raw, ok := compatArgValue(args, "variables"); ok && raw != nil {
		blob, _ := json.Marshal(raw)
		var variables map[string]string
		if err := json.Unmarshal(blob, &variables); err != nil {
			return nil, fmt.Errorf("decode variables: %w", err)
		}
		item.Variables = variables
	}
	if raw, ok := compatArgValue(args, "settings"); ok && raw != nil {
		blob, _ := json.Marshal(raw)
		var settings workflow.WorkflowSettings
		if err := json.Unmarshal(blob, &settings); err != nil {
			return nil, fmt.Errorf("decode settings: %w", err)
		}
		item.Settings = &settings
	}
	if raw, ok := compatArgValue(args, "tags"); ok && raw != nil {
		blob, _ := json.Marshal(raw)
		var tags []string
		if err := json.Unmarshal(blob, &tags); err != nil {
			return nil, fmt.Errorf("decode tags: %w", err)
		}
		item.Tags = tags
	}
	return item, nil
}

func findWorkflowTemplate(templates []workflow.WorkflowTemplateResponse, id string) *workflow.WorkflowTemplateResponse {
	for i := range templates {
		if templates[i].ID == id {
			return &templates[i]
		}
	}
	return nil
}

type nodesServiceAdapter struct {
	runtime *workflow.WorkflowService
}

func (a nodesServiceAdapter) ListWorkflows(ctx context.Context, tenantID string, opts *workflow.ListOptions) ([]*workflow.Workflow, int, error) {
	return a.runtime.ListWorkflows(ctx, tenantID, opts)
}
func (a nodesServiceAdapter) GetWorkflow(ctx context.Context, id string) (*workflow.Workflow, error) {
	return a.runtime.GetWorkflow(ctx, id)
}
func (a nodesServiceAdapter) CreateWorkflow(ctx context.Context, item *workflow.Workflow) (*workflow.Workflow, error) {
	return a.runtime.CreateWorkflow(ctx, item)
}
func (a nodesServiceAdapter) UpdateWorkflow(ctx context.Context, item *workflow.Workflow) (*workflow.Workflow, error) {
	return a.runtime.UpdateWorkflow(ctx, item)
}
func (a nodesServiceAdapter) DeleteWorkflow(ctx context.Context, id string) error {
	return a.runtime.DeleteWorkflow(ctx, id)
}
func (a nodesServiceAdapter) ExecuteWorkflow(ctx context.Context, id string, triggerData map[string]interface{}) (*workflow.Execution, error) {
	return a.runtime.ExecuteWorkflow(ctx, id, triggerData)
}
func (a nodesServiceAdapter) EnableWorkflow(ctx context.Context, id string) error {
	return a.runtime.EnableWorkflow(ctx, id)
}
func (a nodesServiceAdapter) DisableWorkflow(ctx context.Context, id string) error {
	return a.runtime.DisableWorkflow(ctx, id)
}
func (a nodesServiceAdapter) Templates(_ context.Context) []workflow.WorkflowTemplateResponse {
	return workflow.DefaultTemplates()
}

// RegisterNodesTool registers the native nodes tool.
func RegisterNodesTool(registry *Registry, runtime *workflow.WorkflowService) {
	if registry == nil || runtime == nil {
		return
	}
	registry.Register(NewNodesTool(nodesServiceAdapter{runtime: runtime}))
}
