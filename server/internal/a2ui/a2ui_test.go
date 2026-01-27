package a2ui

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNewManager(t *testing.T) {
	logger := zap.NewNop()
	m := NewManager(logger)

	if m == nil {
		t.Fatal("Expected non-nil manager")
	}
}

func TestManager_RegisterHandler(t *testing.T) {
	logger := zap.NewNop()
	m := NewManager(logger)

	called := false
	m.RegisterHandler("test", func(ctx context.Context, action Action, formData map[string]interface{}) (*ActionResult, error) {
		called = true
		return &ActionResult{Success: true}, nil
	})

	// Verify handler is registered
	m.mu.RLock()
	_, ok := m.handlers["test"]
	m.mu.RUnlock()

	if !ok {
		t.Error("Handler not registered")
	}

	// Handler should not be called until an action is triggered
	// The test was incorrectly expecting the handler to be called on registration
	_ = called // Handler will be called when action is triggered, not on registration
}

func TestManager_CreateCanvas(t *testing.T) {
	logger := zap.NewNop()
	m := NewManager(logger)

	canvas := &Canvas{
		ID:    "test-canvas",
		Title: "Test Canvas",
		Components: []Component{
			Text("text1", "Hello World"),
		},
	}

	err := m.CreateCanvas(canvas)
	if err != nil {
		t.Fatalf("CreateCanvas() error = %v", err)
	}

	// Verify canvas is stored
	retrieved, err := m.GetCanvas("test-canvas")
	if err != nil {
		t.Fatalf("GetCanvas() error = %v", err)
	}

	if retrieved.Title != "Test Canvas" {
		t.Errorf("Canvas title = %s, want Test Canvas", retrieved.Title)
	}
}

func TestManager_CreateCanvas_EmptyID(t *testing.T) {
	logger := zap.NewNop()
	m := NewManager(logger)

	canvas := &Canvas{
		Title: "Test Canvas",
	}

	err := m.CreateCanvas(canvas)
	if err == nil {
		t.Error("CreateCanvas() with empty ID should error")
	}
}

func TestManager_GetCanvas_NotFound(t *testing.T) {
	logger := zap.NewNop()
	m := NewManager(logger)

	_, err := m.GetCanvas("nonexistent")
	if err == nil {
		t.Error("GetCanvas() for nonexistent canvas should error")
	}
}

func TestManager_GetCanvas_Expired(t *testing.T) {
	logger := zap.NewNop()
	m := NewManager(logger)

	expiredTime := time.Now().Add(-time.Hour)
	canvas := &Canvas{
		ID:        "expired-canvas",
		ExpiresAt: &expiredTime,
	}

	m.mu.Lock()
	m.canvases["expired-canvas"] = canvas
	m.mu.Unlock()

	_, err := m.GetCanvas("expired-canvas")
	if err == nil {
		t.Error("GetCanvas() for expired canvas should error")
	}
}

func TestManager_UpdateCanvas(t *testing.T) {
	logger := zap.NewNop()
	m := NewManager(logger)

	canvas := &Canvas{
		ID:    "test-canvas",
		Title: "Original Title",
	}
	m.CreateCanvas(canvas)

	canvas.Title = "Updated Title"
	err := m.UpdateCanvas(canvas)
	if err != nil {
		t.Fatalf("UpdateCanvas() error = %v", err)
	}

	retrieved, _ := m.GetCanvas("test-canvas")
	if retrieved.Title != "Updated Title" {
		t.Errorf("Canvas title = %s, want Updated Title", retrieved.Title)
	}
}

func TestManager_DeleteCanvas(t *testing.T) {
	logger := zap.NewNop()
	m := NewManager(logger)

	canvas := &Canvas{ID: "test-canvas"}
	m.CreateCanvas(canvas)

	m.DeleteCanvas("test-canvas")

	_, err := m.GetCanvas("test-canvas")
	if err == nil {
		t.Error("GetCanvas() after delete should error")
	}
}

func TestManager_ExecuteAction(t *testing.T) {
	logger := zap.NewNop()
	m := NewManager(logger)

	// Register handler
	m.RegisterHandler("test_handler", func(ctx context.Context, action Action, formData map[string]interface{}) (*ActionResult, error) {
		return &ActionResult{
			Success: true,
			Data:    "handler executed",
		}, nil
	})

	// Create canvas with action
	canvas := &Canvas{
		ID: "test-canvas",
		Components: []Component{
			{
				ID:   "button1",
				Type: ComponentTypeButton,
				Actions: []Action{
					{
						ID:      "action1",
						Type:    "click",
						Handler: "test_handler",
					},
				},
			},
		},
	}
	m.CreateCanvas(canvas)

	// Execute action
	result, err := m.ExecuteAction(context.Background(), "test-canvas", "action1", nil)
	if err != nil {
		t.Fatalf("ExecuteAction() error = %v", err)
	}

	if !result.Success {
		t.Error("ExecuteAction() result.Success = false, want true")
	}

	if result.Data != "handler executed" {
		t.Errorf("ExecuteAction() result.Data = %v, want 'handler executed'", result.Data)
	}
}

func TestManager_ExecuteAction_HandlerNotFound(t *testing.T) {
	logger := zap.NewNop()
	m := NewManager(logger)

	canvas := &Canvas{
		ID: "test-canvas",
		Components: []Component{
			{
				ID:   "button1",
				Type: ComponentTypeButton,
				Actions: []Action{
					{
						ID:      "action1",
						Type:    "click",
						Handler: "nonexistent_handler",
					},
				},
			},
		},
	}
	m.CreateCanvas(canvas)

	_, err := m.ExecuteAction(context.Background(), "test-canvas", "action1", nil)
	if err == nil {
		t.Error("ExecuteAction() with nonexistent handler should error")
	}
}

func TestManager_CleanupExpired(t *testing.T) {
	logger := zap.NewNop()
	m := NewManager(logger)

	expiredTime := time.Now().Add(-time.Hour)
	futureTime := time.Now().Add(time.Hour)

	m.mu.Lock()
	m.canvases["expired1"] = &Canvas{ID: "expired1", ExpiresAt: &expiredTime}
	m.canvases["expired2"] = &Canvas{ID: "expired2", ExpiresAt: &expiredTime}
	m.canvases["valid"] = &Canvas{ID: "valid", ExpiresAt: &futureTime}
	m.canvases["no_expiry"] = &Canvas{ID: "no_expiry"}
	m.mu.Unlock()

	count := m.CleanupExpired()
	if count != 2 {
		t.Errorf("CleanupExpired() = %d, want 2", count)
	}

	// Verify remaining canvases
	ids := m.ListCanvases()
	if len(ids) != 2 {
		t.Errorf("ListCanvases() len = %d, want 2", len(ids))
	}
}

func TestBuilder(t *testing.T) {
	component := NewBuilder(ComponentTypeButton).
		ID("btn1").
		Prop("label", "Click Me").
		Style("color", "blue").
		Action(Action{
			ID:      "btn1_click",
			Type:    "click",
			Handler: "handle_click",
		}).
		Build()

	if component.ID != "btn1" {
		t.Errorf("Component ID = %s, want btn1", component.ID)
	}

	if component.Type != ComponentTypeButton {
		t.Errorf("Component Type = %s, want button", component.Type)
	}

	if component.Props["label"] != "Click Me" {
		t.Errorf("Component Props[label] = %v, want Click Me", component.Props["label"])
	}

	if component.Style["color"] != "blue" {
		t.Errorf("Component Style[color] = %v, want blue", component.Style["color"])
	}

	if len(component.Actions) != 1 {
		t.Errorf("Component Actions len = %d, want 1", len(component.Actions))
	}
}

func TestHelperFunctions(t *testing.T) {
	// Test Text
	text := Text("text1", "Hello")
	if text.Type != ComponentTypeText {
		t.Errorf("Text() type = %s, want text", text.Type)
	}
	if text.Props["content"] != "Hello" {
		t.Errorf("Text() content = %v, want Hello", text.Props["content"])
	}

	// Test Button
	button := Button("btn1", "Click", "handler")
	if button.Type != ComponentTypeButton {
		t.Errorf("Button() type = %s, want button", button.Type)
	}
	if len(button.Actions) != 1 {
		t.Errorf("Button() actions len = %d, want 1", len(button.Actions))
	}

	// Test Input
	input := Input("input1", "Name", "Enter name")
	if input.Type != ComponentTypeInput {
		t.Errorf("Input() type = %s, want input", input.Type)
	}

	// Test Card
	card := Card("card1", "Title", text, button)
	if card.Type != ComponentTypeCard {
		t.Errorf("Card() type = %s, want card", card.Type)
	}
	if len(card.Children) != 2 {
		t.Errorf("Card() children len = %d, want 2", len(card.Children))
	}

	// Test Alert
	alert := Alert("alert1", "warning", "Warning message")
	if alert.Type != ComponentTypeAlert {
		t.Errorf("Alert() type = %s, want alert", alert.Type)
	}

	// Test Progress
	progress := Progress("progress1", 50, 100)
	if progress.Type != ComponentTypeProgress {
		t.Errorf("Progress() type = %s, want progress", progress.Type)
	}

	// Test Code
	code := Code("code1", "go", "fmt.Println()")
	if code.Type != ComponentTypeCode {
		t.Errorf("Code() type = %s, want code", code.Type)
	}

	// Test Markdown
	md := Markdown("md1", "# Hello")
	if md.Type != ComponentTypeMarkdown {
		t.Errorf("Markdown() type = %s, want markdown", md.Type)
	}

	// Test Image
	img := Image("img1", "http://example.com/img.png", "Example")
	if img.Type != ComponentTypeImage {
		t.Errorf("Image() type = %s, want image", img.Type)
	}

	// Test Table
	table := Table("table1", []string{"A", "B"}, [][]interface{}{{"1", "2"}})
	if table.Type != ComponentTypeTable {
		t.Errorf("Table() type = %s, want table", table.Type)
	}

	// Test List
	list := List("list1", []string{"item1", "item2"})
	if list.Type != ComponentTypeList {
		t.Errorf("List() type = %s, want list", list.Type)
	}

	// Test Form
	form := Form("form1", "submit_handler", input)
	if form.Type != ComponentTypeForm {
		t.Errorf("Form() type = %s, want form", form.Type)
	}

	// Test Grid
	grid := Grid("grid1", 2, text, button)
	if grid.Type != ComponentTypeGrid {
		t.Errorf("Grid() type = %s, want grid", grid.Type)
	}
}

func TestCanvas_ToJSON(t *testing.T) {
	canvas := &Canvas{
		ID:    "test-canvas",
		Title: "Test",
		Components: []Component{
			Text("text1", "Hello"),
		},
	}

	data, err := canvas.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("ToJSON() produced invalid JSON: %v", err)
	}

	if parsed["id"] != "test-canvas" {
		t.Errorf("ToJSON() id = %v, want test-canvas", parsed["id"])
	}
}

func TestFromJSON(t *testing.T) {
	jsonData := `{
		"id": "test-canvas",
		"title": "Test Canvas",
		"components": [
			{
				"id": "text1",
				"type": "text",
				"props": {"content": "Hello"}
			}
		]
	}`

	canvas, err := FromJSON([]byte(jsonData))
	if err != nil {
		t.Fatalf("FromJSON() error = %v", err)
	}

	if canvas.ID != "test-canvas" {
		t.Errorf("FromJSON() ID = %s, want test-canvas", canvas.ID)
	}

	if canvas.Title != "Test Canvas" {
		t.Errorf("FromJSON() Title = %s, want Test Canvas", canvas.Title)
	}

	if len(canvas.Components) != 1 {
		t.Errorf("FromJSON() components len = %d, want 1", len(canvas.Components))
	}
}

func TestFromJSON_Invalid(t *testing.T) {
	_, err := FromJSON([]byte("invalid json"))
	if err == nil {
		t.Error("FromJSON() with invalid JSON should error")
	}
}

func TestValidation(t *testing.T) {
	min := 0.0
	max := 100.0

	input := NewBuilder(ComponentTypeInput).
		ID("age").
		Prop("label", "Age").
		Validation(&Validation{
			Required: true,
			Min:      &min,
			Max:      &max,
			Message:  "Age must be between 0 and 100",
		}).
		Build()

	if input.Validation == nil {
		t.Fatal("Expected validation to be set")
	}

	if !input.Validation.Required {
		t.Error("Validation.Required = false, want true")
	}

	if *input.Validation.Min != 0 {
		t.Errorf("Validation.Min = %v, want 0", *input.Validation.Min)
	}

	if *input.Validation.Max != 100 {
		t.Errorf("Validation.Max = %v, want 100", *input.Validation.Max)
	}
}

func TestConfirmDialog(t *testing.T) {
	button := NewBuilder(ComponentTypeButton).
		ID("delete").
		Prop("label", "Delete").
		Action(Action{
			ID:      "delete_action",
			Type:    "click",
			Handler: "delete_handler",
			Confirm: &ConfirmDialog{
				Title:       "Confirm Delete",
				Message:     "Are you sure?",
				ConfirmText: "Yes, delete",
				CancelText:  "Cancel",
			},
		}).
		Build()

	if len(button.Actions) != 1 {
		t.Fatal("Expected 1 action")
	}

	if button.Actions[0].Confirm == nil {
		t.Fatal("Expected confirm dialog")
	}

	if button.Actions[0].Confirm.Title != "Confirm Delete" {
		t.Errorf("Confirm.Title = %s, want Confirm Delete", button.Actions[0].Confirm.Title)
	}
}

func TestNestedComponents(t *testing.T) {
	// Build a complex nested structure
	form := Form("contact_form", "submit_contact",
		Grid("form_grid", 2,
			Input("first_name", "First Name", "Enter first name"),
			Input("last_name", "Last Name", "Enter last name"),
		),
		Input("email", "Email", "Enter email"),
		Button("submit", "Submit", "submit_contact"),
	)

	if form.Type != ComponentTypeForm {
		t.Errorf("Form type = %s, want form", form.Type)
	}

	if len(form.Children) != 3 {
		t.Errorf("Form children len = %d, want 3", len(form.Children))
	}

	// Check grid
	grid := form.Children[0]
	if grid.Type != ComponentTypeGrid {
		t.Errorf("Grid type = %s, want grid", grid.Type)
	}

	if len(grid.Children) != 2 {
		t.Errorf("Grid children len = %d, want 2", len(grid.Children))
	}
}
