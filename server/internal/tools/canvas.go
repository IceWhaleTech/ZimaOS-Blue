package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a2ui"
	"github.com/google/uuid"
)

// CanvasTool manages lightweight A2UI canvases.
type CanvasTool struct {
	manager *a2ui.Manager
}

// NewCanvasTool creates a native canvas tool.
func NewCanvasTool(manager *a2ui.Manager) *CanvasTool {
	return &CanvasTool{manager: manager}
}

// Definition returns the tool schema.
func (t *CanvasTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "canvas",
		Description: "Create, inspect, update, delete, and execute lightweight Agent-to-UI canvases.",
		Icon:        "canvas",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"list", "get", "create", "update", "delete", "execute"},
					"description": "Canvas action. Defaults based on provided fields.",
				},
				"id":          map[string]interface{}{"type": "string", "description": "Canvas ID."},
				"title":       map[string]interface{}{"type": "string", "description": "Canvas title."},
				"description": map[string]interface{}{"type": "string", "description": "Canvas description."},
				"components": map[string]interface{}{
					"type":        "array",
					"description": "Canvas component tree.",
					"items": map[string]interface{}{
						"type":                 "object",
						"additionalProperties": true,
					},
				},
				"layout":         map[string]interface{}{"type": "string", "description": "Layout type: vertical, horizontal, or grid."},
				"metadata":       map[string]interface{}{"type": "object", "description": "Additional canvas metadata."},
				"ttl_seconds":    map[string]interface{}{"type": "integer", "description": "Optional canvas TTL in seconds."},
				"content":        map[string]interface{}{"type": "string", "description": "Convenience markdown/text content for quick canvas creation."},
				"text":           map[string]interface{}{"type": "string", "description": "Alias for content."},
				"input":          map[string]interface{}{"type": "string", "description": "Alias for content."},
				"action_id":      map[string]interface{}{"type": "string", "description": "Canvas action ID for execute."},
				"form_data":      map[string]interface{}{"type": "object", "description": "Form payload for execute."},
				"include_canvas": map[string]interface{}{"type": "boolean", "description": "Include full canvases in list output."},
			},
		},
	}
}

// Execute dispatches the requested canvas action.
func (t *CanvasTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.manager == nil {
		return nil, errors.New("canvas manager not available")
	}
	switch canvasAction(args) {
	case "list":
		return t.executeList(args)
	case "get":
		return t.executeGet(args)
	case "create":
		return t.executeCreate(args)
	case "update":
		return t.executeUpdate(args)
	case "delete":
		return t.executeDelete(args)
	case "execute":
		return t.executeAction(ctx, args)
	default:
		return nil, errors.New("unsupported canvas action")
	}
}

func (t *CanvasTool) executeList(args map[string]interface{}) (interface{}, error) {
	ids := t.manager.ListCanvases()
	result := map[string]interface{}{
		"canvases": ids,
		"count":    len(ids),
	}
	includeCanvas, _ := compatBoolArg(args, "include_canvas", "includeCanvas")
	if includeCanvas {
		items := make([]*a2ui.Canvas, 0, len(ids))
		for _, id := range ids {
			canvas, err := t.manager.GetCanvas(id)
			if err == nil && canvas != nil {
				items = append(items, canvas)
			}
		}
		result["items"] = items
	}
	return result, nil
}

func (t *CanvasTool) executeGet(args map[string]interface{}) (interface{}, error) {
	id := firstCompatString(args, "id", "canvas_id", "canvasId")
	if id == "" {
		return nil, errors.New("id is required")
	}
	canvas, err := t.manager.GetCanvas(id)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"canvas": canvas}, nil
}

func (t *CanvasTool) executeCreate(args map[string]interface{}) (interface{}, error) {
	canvas, err := buildCanvasFromArgs(args, nil)
	if err != nil {
		return nil, err
	}
	if canvas.ID == "" {
		canvas.ID = "canvas_" + uuid.New().String()
	}
	if err := t.manager.CreateCanvas(canvas); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"canvas":  canvas,
		"created": true,
		"id":      canvas.ID,
	}, nil
}

func (t *CanvasTool) executeUpdate(args map[string]interface{}) (interface{}, error) {
	id := firstCompatString(args, "id", "canvas_id", "canvasId")
	if id == "" {
		return nil, errors.New("id is required")
	}
	existing, err := t.manager.GetCanvas(id)
	if err != nil {
		return nil, err
	}
	updated, err := buildCanvasFromArgs(args, existing)
	if err != nil {
		return nil, err
	}
	updated.ID = id
	if err := t.manager.UpdateCanvas(updated); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"canvas":  updated,
		"updated": true,
		"id":      id,
	}, nil
}

func (t *CanvasTool) executeDelete(args map[string]interface{}) (interface{}, error) {
	id := firstCompatString(args, "id", "canvas_id", "canvasId")
	if id == "" {
		return nil, errors.New("id is required")
	}
	if _, err := t.manager.GetCanvas(id); err != nil {
		return nil, err
	}
	t.manager.DeleteCanvas(id)
	return map[string]interface{}{"deleted": true, "id": id}, nil
}

func (t *CanvasTool) executeAction(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id := firstCompatString(args, "id", "canvas_id", "canvasId")
	if id == "" {
		return nil, errors.New("id is required")
	}
	actionID := firstCompatString(args, "action_id", "actionId", "id_action")
	if actionID == "" {
		return nil, errors.New("action_id is required")
	}
	formDataRaw, _ := compatArgValue(args, "form_data", "formData")
	formData, err := decodeMap(formDataRaw)
	if err != nil {
		return nil, err
	}
	result, err := t.manager.ExecuteAction(ctx, id, actionID, formData)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"result": result}, nil
}

func canvasAction(args map[string]interface{}) string {
	action := strings.ToLower(strings.TrimSpace(firstCompatString(args, "action", "op", "operation", "command")))
	switch action {
	case "", "list", "ls", "status":
		if firstCompatString(args, "action_id", "actionId") != "" {
			return "execute"
		}
		if firstCompatString(args, "id", "canvas_id", "canvasId") != "" {
			if hasCanvasMutableArgs(args) {
				return "update"
			}
			return "get"
		}
		if hasCanvasMutableArgs(args) {
			return "create"
		}
		return "list"
	case "get", "show", "read":
		return "get"
	case "create", "add":
		return "create"
	case "update", "edit", "patch":
		return "update"
	case "delete", "remove", "rm":
		return "delete"
	case "execute", "run", "trigger":
		return "execute"
	default:
		return action
	}
}

func hasCanvasMutableArgs(args map[string]interface{}) bool {
	for _, keys := range [][]string{{"title"}, {"description"}, {"components"}, {"layout"}, {"metadata"}, {"ttl_seconds", "ttlSeconds"}, {"content"}, {"text"}, {"input"}} {
		if value, ok := compatArgValue(args, keys...); ok && value != nil && !(asString(value) == "") {
			return true
		}
	}
	return false
}

func buildCanvasFromArgs(args map[string]interface{}, base *a2ui.Canvas) (*a2ui.Canvas, error) {
	canvas := &a2ui.Canvas{}
	if base != nil {
		copy := *base
		canvas = &copy
	}
	if canvas.CreatedAt.IsZero() {
		canvas.CreatedAt = time.Now()
	}
	if id := firstCompatString(args, "id", "canvas_id", "canvasId"); id != "" {
		canvas.ID = id
	}
	if title := firstCompatString(args, "title", "name"); title != "" {
		canvas.Title = title
	}
	if desc := firstCompatString(args, "description", "summary"); desc != "" {
		canvas.Description = desc
	}
	if layout := firstCompatString(args, "layout"); layout != "" {
		canvas.Layout = layout
	}
	metadataRaw, _ := compatArgValue(args, "metadata")
	if metadata, err := decodeMap(metadataRaw); err != nil {
		return nil, err
	} else if metadata != nil {
		canvas.Metadata = metadata
	}
	componentsRaw, hasComponents := compatArgValue(args, "components")
	if hasComponents && componentsRaw != nil {
		components, err := decodeComponents(componentsRaw)
		if err != nil {
			return nil, err
		}
		canvas.Components = components
	} else if content := firstCompatString(args, "content", "text", "input"); content != "" {
		componentID := "content"
		if canvas.ID != "" {
			componentID = canvas.ID + "_content"
		}
		canvas.Components = []a2ui.Component{a2ui.Markdown(componentID, content)}
		if canvas.Title == "" {
			canvas.Title = trimCompatText(content, 72)
		}
	}
	if ttlRaw, ok := compatArgValue(args, "ttl_seconds", "ttlSeconds"); ok {
		if ttl, ok := asCompatInt(ttlRaw); ok && ttl > 0 {
			expiry := time.Now().Add(time.Duration(ttl) * time.Second)
			canvas.ExpiresAt = &expiry
		}
	}
	return canvas, nil
}

func decodeComponents(value interface{}) ([]a2ui.Component, error) {
	if value == nil {
		return nil, nil
	}
	if typed, ok := value.([]a2ui.Component); ok {
		return typed, nil
	}
	var components []a2ui.Component
	blob, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal components: %w", err)
	}
	if err := json.Unmarshal(blob, &components); err != nil {
		return nil, fmt.Errorf("decode components: %w", err)
	}
	return components, nil
}

func decodeMap(value interface{}) (map[string]interface{}, error) {
	if value == nil {
		return nil, nil
	}
	if typed, ok := value.(map[string]interface{}); ok {
		return typed, nil
	}
	var out map[string]interface{}
	blob, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal object: %w", err)
	}
	if err := json.Unmarshal(blob, &out); err != nil {
		return nil, fmt.Errorf("decode object: %w", err)
	}
	return out, nil
}

func asCompatInt(value interface{}) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	default:
		return 0, false
	}
}

// RegisterCanvasTools registers the native canvas tool.
func RegisterCanvasTools(registry *Registry, manager *a2ui.Manager) {
	if registry == nil || manager == nil {
		return
	}
	registry.Register(NewCanvasTool(manager))
}
