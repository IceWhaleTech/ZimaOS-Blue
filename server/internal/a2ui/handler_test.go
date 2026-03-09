package a2ui

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func setupTestHandler() (*Handler, *echo.Echo) {
	logger := zap.NewNop()
	manager := NewManager(logger)
	handler := NewHandler(manager)

	e := echo.New()
	g := e.Group("/api/v1/a2ui")
	handler.RegisterRoutes(g)

	return handler, e
}

func TestListCanvases(t *testing.T) {
	handler, e := setupTestHandler()

	// Create some canvases first
	canvas1 := &Canvas{ID: "canvas1", Title: "Test 1"}
	canvas2 := &Canvas{ID: "canvas2", Title: "Test 2"}
	handler.manager.CreateCanvas(canvas1)
	handler.manager.CreateCanvas(canvas2)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/a2ui/canvases", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	count := int(resp["count"].(float64))
	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}
}

func TestCreateCanvas(t *testing.T) {
	_, e := setupTestHandler()

	body := `{"id": "test-canvas", "title": "Test Canvas", "components": []}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/a2ui/canvases", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var canvas Canvas
	if err := json.Unmarshal(rec.Body.Bytes(), &canvas); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if canvas.ID != "test-canvas" {
		t.Errorf("expected ID 'test-canvas', got '%s'", canvas.ID)
	}
	if canvas.Title != "Test Canvas" {
		t.Errorf("expected title 'Test Canvas', got '%s'", canvas.Title)
	}
}

func TestCreateCanvasWithoutID(t *testing.T) {
	_, e := setupTestHandler()

	body := `{"title": "Test Canvas"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/a2ui/canvases", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var canvas Canvas
	if err := json.Unmarshal(rec.Body.Bytes(), &canvas); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if canvas.ID == "" {
		t.Fatal("expected generated canvas ID")
	}
}

func TestGetCanvas(t *testing.T) {
	handler, e := setupTestHandler()

	// Create a canvas first
	canvas := &Canvas{
		ID:    "get-test",
		Title: "Get Test Canvas",
		Components: []Component{
			Text("text1", "Hello World"),
		},
	}
	handler.manager.CreateCanvas(canvas)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/a2ui/canvases/get-test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var result Canvas
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if result.ID != "get-test" {
		t.Errorf("expected ID 'get-test', got '%s'", result.ID)
	}
}

func TestGetCanvasNotFound(t *testing.T) {
	_, e := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/a2ui/canvases/nonexistent", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestUpdateCanvas(t *testing.T) {
	handler, e := setupTestHandler()

	// Create a canvas first
	canvas := &Canvas{ID: "update-test", Title: "Original Title"}
	handler.manager.CreateCanvas(canvas)

	body := `{"title": "Updated Title"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/a2ui/canvases/update-test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var result Canvas
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if result.Title != "Updated Title" {
		t.Errorf("expected title 'Updated Title', got '%s'", result.Title)
	}
}

func TestDeleteCanvas(t *testing.T) {
	handler, e := setupTestHandler()

	// Create a canvas first
	canvas := &Canvas{ID: "delete-test", Title: "To Delete"}
	handler.manager.CreateCanvas(canvas)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/a2ui/canvases/delete-test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Verify canvas is deleted
	_, err := handler.manager.GetCanvas("delete-test")
	if err == nil {
		t.Error("expected canvas to be deleted")
	}
}

func TestDeleteCanvasNotFound(t *testing.T) {
	_, e := setupTestHandler()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/a2ui/canvases/nonexistent", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestExecuteAction(t *testing.T) {
	handler, e := setupTestHandler()

	// Register a test handler
	handler.manager.RegisterHandler("test-handler", func(ctx context.Context, action Action, formData map[string]interface{}) (*ActionResult, error) {
		return &ActionResult{
			Success: true,
			Data:    map[string]interface{}{"received": formData},
		}, nil
	})

	// Create a canvas with an action
	canvas := &Canvas{
		ID:    "action-test",
		Title: "Action Test",
		Components: []Component{
			{
				ID:   "btn1",
				Type: ComponentTypeButton,
				Actions: []Action{
					{
						ID:      "action1",
						Type:    "click",
						Handler: "test-handler",
					},
				},
			},
		},
	}
	handler.manager.CreateCanvas(canvas)

	body := `{"form_data": {"key": "value"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/a2ui/canvases/action-test/actions/action1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var result ActionResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !result.Success {
		t.Error("expected action to succeed")
	}
}

func TestExecuteActionNotFound(t *testing.T) {
	handler, e := setupTestHandler()

	// Create a canvas without actions
	canvas := &Canvas{ID: "no-action", Title: "No Action"}
	handler.manager.CreateCanvas(canvas)

	body := `{"form_data": {}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/a2ui/canvases/no-action/actions/nonexistent", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}
