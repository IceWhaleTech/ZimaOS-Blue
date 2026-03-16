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

func TestCanvasToolDefinitionIncludesArrayItems(t *testing.T) {
	tool := NewCanvasTool(a2ui.NewManager(zap.NewNop()))
	props := tool.Definition().Parameters["properties"].(map[string]interface{})
	components := props["components"].(map[string]interface{})
	items, ok := components["items"].(map[string]interface{})
	if !ok {
		t.Fatalf("components.items missing from schema: %#v", components)
	}
	if items["type"] != "object" {
		t.Fatalf("components.items.type = %#v, want object", items["type"])
	}
}

func TestCanvasToolSchemaCompressionPreservesArrayItems(t *testing.T) {
	router := DefaultToolRouter()
	router.DynamicExposure = false

	def := NewCanvasTool(a2ui.NewManager(zap.NewNop())).Definition()
	routed := router.Route("create canvas", "auto", []ToolDefinition{def})
	if len(routed) != 1 {
		t.Fatalf("expected 1 routed tool, got %d", len(routed))
	}

	props := routed[0].Parameters["properties"].(map[string]interface{})
	components := props["components"].(map[string]interface{})
	items, ok := components["items"].(map[string]interface{})
	if !ok {
		t.Fatalf("compressed components.items missing from schema: %#v", components)
	}
	if items["type"] != "object" {
		t.Fatalf("compressed components.items.type = %#v, want object", items["type"])
	}
}
