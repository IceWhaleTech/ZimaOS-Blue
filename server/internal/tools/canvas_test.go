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
