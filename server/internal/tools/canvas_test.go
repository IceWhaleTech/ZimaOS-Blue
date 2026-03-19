package tools

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a2ui"
	"go.uber.org/zap"
)

func TestCanvasToolCreateFromContent(t *testing.T) {
	tool := NewCanvasTool(a2ui.NewManager(zap.NewNop()))
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"title":   "Release Plan",
		"content": "# Hello\nworld",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	canvas := payload["canvas"].(*a2ui.Canvas)
	if canvas.ID == "" || len(canvas.Components) != 1 {
		t.Fatalf("unexpected canvas: %#v", canvas)
	}
	if canvas.Components[0].Type != a2ui.ComponentTypeMarkdown {
		t.Fatalf("component type = %q, want markdown", canvas.Components[0].Type)
	}
}

func TestCanvasToolCreateFromSingleComponentObject(t *testing.T) {
	tool := NewCanvasTool(a2ui.NewManager(zap.NewNop()))
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"title": "Greeting",
		"components": map[string]interface{}{
			"id":   "text1",
			"type": "text",
			"props": map[string]interface{}{
				"text": "Hello world",
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	canvas := payload["canvas"].(*a2ui.Canvas)
	if len(canvas.Components) != 1 {
		t.Fatalf("components len = %d, want 1", len(canvas.Components))
	}
	if canvas.Components[0].ID != "text1" || canvas.Components[0].Type != a2ui.ComponentTypeText {
		t.Fatalf("unexpected component: %#v", canvas.Components[0])
	}
}

func TestCanvasToolListAndGet(t *testing.T) {
	mgr := a2ui.NewManager(zap.NewNop())
	if err := mgr.CreateCanvas(&a2ui.Canvas{ID: "canvas_1", Title: "One"}); err != nil {
		t.Fatalf("CreateCanvas failed: %v", err)
	}
	tool := NewCanvasTool(mgr)
	listed, err := tool.Execute(context.Background(), map[string]interface{}{"action": "list", "include_canvas": true})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	listPayload := listed.(map[string]interface{})
	if listPayload["count"].(int) != 1 {
		t.Fatalf("count = %v, want 1", listPayload["count"])
	}
	got, err := tool.Execute(context.Background(), map[string]interface{}{"id": "canvas_1"})
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	getPayload := got.(map[string]interface{})
	canvas := getPayload["canvas"].(*a2ui.Canvas)
	if canvas.Title != "One" {
		t.Fatalf("title = %q, want One", canvas.Title)
	}
}

func TestCanvasToolSupportsNestedCamelCaseArgs(t *testing.T) {
	mgr := a2ui.NewManager(zap.NewNop())
	mgr.RegisterHandler("submit", func(ctx context.Context, action a2ui.Action, formData map[string]interface{}) (*a2ui.ActionResult, error) {
		return &a2ui.ActionResult{Success: true, Data: formData["name"]}, nil
	})
	if err := mgr.CreateCanvas(&a2ui.Canvas{
		ID:    "canvas_1",
		Title: "One",
		Components: []a2ui.Component{{
			ID:   "btn1",
			Type: a2ui.ComponentTypeButton,
			Actions: []a2ui.Action{{
				ID:      "act_1",
				Type:    "click",
				Handler: "submit",
			}},
		}},
	}); err != nil {
		t.Fatalf("CreateCanvas failed: %v", err)
	}
	tool := NewCanvasTool(mgr)

	listed, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"action":        "list",
			"includeCanvas": true,
		},
	})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	listPayload := listed.(map[string]interface{})
	items := listPayload["items"].([]*a2ui.Canvas)
	if len(items) != 1 || items[0].ID != "canvas_1" {
		t.Fatalf("unexpected list items: %#v", items)
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"action":   "execute",
			"canvasId": "canvas_1",
			"actionId": "act_1",
			"formData": map[string]interface{}{"name": "orca"},
		},
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	payload := result.(map[string]interface{})
	actionResult := payload["result"].(*a2ui.ActionResult)
	if !actionResult.Success || actionResult.Data != "orca" {
		t.Fatalf("unexpected action result: %#v", actionResult)
	}
}

func TestCanvasToolExecuteAction(t *testing.T) {
	mgr := a2ui.NewManager(zap.NewNop())
	mgr.RegisterHandler("submit", func(ctx context.Context, action a2ui.Action, formData map[string]interface{}) (*a2ui.ActionResult, error) {
		return &a2ui.ActionResult{Success: true, Data: formData["name"]}, nil
	})
	if err := mgr.CreateCanvas(&a2ui.Canvas{
		ID: "canvas_1",
		Components: []a2ui.Component{{
			ID:   "btn1",
			Type: a2ui.ComponentTypeButton,
			Actions: []a2ui.Action{{
				ID:      "act_1",
				Type:    "click",
				Handler: "submit",
			}},
		}},
	}); err != nil {
		t.Fatalf("CreateCanvas failed: %v", err)
	}
	tool := NewCanvasTool(mgr)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "execute",
		"id":        "canvas_1",
		"action_id": "act_1",
		"form_data": map[string]interface{}{"name": "orca"},
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	payload := result.(map[string]interface{})
	actionResult := payload["result"].(*a2ui.ActionResult)
	if !actionResult.Success || actionResult.Data != "orca" {
		t.Fatalf("unexpected action result: %#v", actionResult)
	}
}

func TestCanvasToolDefinitionSupportsSingleOrArrayComponents(t *testing.T) {
	tool := NewCanvasTool(a2ui.NewManager(zap.NewNop()))
	props := tool.Definition().Parameters["properties"].(map[string]interface{})
	components := props["components"].(map[string]interface{})
	arraySchema, objectSchema := canvasComponentSchemaBranches(t, components)
	items, ok := arraySchema["items"].(map[string]interface{})
	if !ok {
		t.Fatalf("array branch items missing from schema: %#v", arraySchema)
	}
	if items["type"] != "object" {
		t.Fatalf("array branch items.type = %#v, want object", items["type"])
	}
	if objectSchema["additionalProperties"] != true {
		t.Fatalf("object branch additionalProperties = %#v, want true", objectSchema["additionalProperties"])
	}
}

func TestCanvasToolSchemaCompressionPreservesFlexibleComponentsSchema(t *testing.T) {
	router := DefaultToolRouter()
	router.DynamicExposure = false

	def := NewCanvasTool(a2ui.NewManager(zap.NewNop())).Definition()
	routed := router.Route("create canvas", "auto", []ToolDefinition{def})
	if len(routed) != 1 {
		t.Fatalf("expected 1 routed tool, got %d", len(routed))
	}

	props := routed[0].Parameters["properties"].(map[string]interface{})
	components := props["components"].(map[string]interface{})
	arraySchema, objectSchema := canvasComponentSchemaBranches(t, components)
	items, ok := arraySchema["items"].(map[string]interface{})
	if !ok {
		t.Fatalf("compressed array branch items missing from schema: %#v", arraySchema)
	}
	if items["type"] != "object" {
		t.Fatalf("compressed array branch items.type = %#v, want object", items["type"])
	}
	if objectSchema["additionalProperties"] != true {
		t.Fatalf("compressed object branch additionalProperties = %#v, want true", objectSchema["additionalProperties"])
	}
}

func TestCanvasToolSchemaValidationAllowsSingleComponentObject(t *testing.T) {
	def := NewCanvasTool(a2ui.NewManager(zap.NewNop())).Definition()
	err := ValidateToolArguments(def.Parameters, map[string]interface{}{
		"components": map[string]interface{}{
			"id":   "text1",
			"type": "text",
		},
	})
	if err != nil {
		t.Fatalf("ValidateToolArguments returned error: %v", err)
	}
}

func canvasComponentSchemaBranches(t *testing.T, components map[string]interface{}) (map[string]interface{}, map[string]interface{}) {
	t.Helper()

	rawBranches, ok := components["anyOf"].([]interface{})
	if !ok {
		t.Fatalf("components.anyOf missing from schema: %#v", components)
	}

	var arraySchema map[string]interface{}
	var objectSchema map[string]interface{}
	for _, raw := range rawBranches {
		branch, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		typeName, _ := branch["type"].(string)
		switch typeName {
		case "array":
			arraySchema = branch
		case "object":
			objectSchema = branch
		}
	}

	if arraySchema == nil || objectSchema == nil {
		t.Fatalf("components.anyOf missing expected branches: %#v", rawBranches)
	}

	return arraySchema, objectSchema
}
