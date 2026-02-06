package formfiller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/labstack/echo/v4"
)

func setupTestHandler(t *testing.T) (*Handler, *echo.Echo, func()) {
	tmpDir, err := os.MkdirTemp("", "formfiller-handler-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	store, err := NewStore(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create store: %v", err)
	}

	handler := NewHandler(store)
	e := echo.New()

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return handler, e, cleanup
}

func TestListTemplatesHandler(t *testing.T) {
	handler, e, cleanup := setupTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/templates", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.ListTemplates(c); err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var templates []FillTemplate
	if err := json.Unmarshal(rec.Body.Bytes(), &templates); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Should have default template
	if len(templates) != 1 {
		t.Errorf("Expected 1 template, got %d", len(templates))
	}
}

func TestCreateTemplateHandler(t *testing.T) {
	handler, e, cleanup := setupTestHandler(t)
	defer cleanup()

	body := `{"name": "Test Template", "is_default": false, "fields": {"email": "test@example.com"}}`
	req := httptest.NewRequest(http.MethodPost, "/templates", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.CreateTemplate(c); err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if rec.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var template FillTemplate
	if err := json.Unmarshal(rec.Body.Bytes(), &template); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if template.Name != "Test Template" {
		t.Errorf("Expected name 'Test Template', got '%s'", template.Name)
	}

	if template.Fields["email"] != "test@example.com" {
		t.Errorf("Expected email 'test@example.com', got '%s'", template.Fields["email"])
	}
}

func TestCreateTemplateHandler_MissingName(t *testing.T) {
	handler, e, cleanup := setupTestHandler(t)
	defer cleanup()

	body := `{"is_default": false}`
	req := httptest.NewRequest(http.MethodPost, "/templates", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.CreateTemplate(c); err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestGetTemplateHandler(t *testing.T) {
	handler, e, cleanup := setupTestHandler(t)
	defer cleanup()

	// First create a template
	createBody := `{"name": "Get Test", "fields": {}}`
	createReq := httptest.NewRequest(http.MethodPost, "/templates", bytes.NewBufferString(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	createCtx := e.NewContext(createReq, createRec)
	handler.CreateTemplate(createCtx)

	var created FillTemplate
	json.Unmarshal(createRec.Body.Bytes(), &created)

	// Now get the template
	req := httptest.NewRequest(http.MethodGet, "/templates/"+created.ID, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(created.ID)

	if err := handler.GetTemplate(c); err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var template FillTemplate
	if err := json.Unmarshal(rec.Body.Bytes(), &template); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if template.ID != created.ID {
		t.Errorf("Expected ID '%s', got '%s'", created.ID, template.ID)
	}
}

func TestGetTemplateHandler_NotFound(t *testing.T) {
	handler, e, cleanup := setupTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/templates/nonexistent", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("nonexistent")

	if err := handler.GetTemplate(c); err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestUpdateTemplateHandler(t *testing.T) {
	handler, e, cleanup := setupTestHandler(t)
	defer cleanup()

	// First create a template
	createBody := `{"name": "Update Test", "fields": {}}`
	createReq := httptest.NewRequest(http.MethodPost, "/templates", bytes.NewBufferString(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	createCtx := e.NewContext(createReq, createRec)
	handler.CreateTemplate(createCtx)

	var created FillTemplate
	json.Unmarshal(createRec.Body.Bytes(), &created)

	// Now update the template
	updateBody := `{"name": "Updated Name", "fields": {"phone": "123-456-7890"}}`
	req := httptest.NewRequest(http.MethodPut, "/templates/"+created.ID, bytes.NewBufferString(updateBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(created.ID)

	if err := handler.UpdateTemplate(c); err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var updated FillTemplate
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if updated.Name != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got '%s'", updated.Name)
	}

	if updated.Fields["phone"] != "123-456-7890" {
		t.Errorf("Expected phone '123-456-7890', got '%s'", updated.Fields["phone"])
	}
}

func TestDeleteTemplateHandler(t *testing.T) {
	handler, e, cleanup := setupTestHandler(t)
	defer cleanup()

	// First create a template
	createBody := `{"name": "Delete Test", "fields": {}}`
	createReq := httptest.NewRequest(http.MethodPost, "/templates", bytes.NewBufferString(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	createCtx := e.NewContext(createReq, createRec)
	handler.CreateTemplate(createCtx)

	var created FillTemplate
	json.Unmarshal(createRec.Body.Bytes(), &created)

	// Now delete the template
	req := httptest.NewRequest(http.MethodDelete, "/templates/"+created.ID, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(created.ID)

	if err := handler.DeleteTemplate(c); err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if rec.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, rec.Code)
	}

	// Verify it's deleted
	getReq := httptest.NewRequest(http.MethodGet, "/templates/"+created.ID, nil)
	getRec := httptest.NewRecorder()
	getCtx := e.NewContext(getReq, getRec)
	getCtx.SetParamNames("id")
	getCtx.SetParamValues(created.ID)
	handler.GetTemplate(getCtx)

	if getRec.Code != http.StatusNotFound {
		t.Errorf("Expected status %d after delete, got %d", http.StatusNotFound, getRec.Code)
	}
}

func TestGetPatternsHandler(t *testing.T) {
	handler, e, cleanup := setupTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/patterns", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.GetPatterns(c); err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var patterns FieldPatterns
	if err := json.Unmarshal(rec.Body.Bytes(), &patterns); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(patterns.Patterns) == 0 {
		t.Error("Expected patterns to be populated")
	}
}

func TestUpdatePatternsHandler(t *testing.T) {
	handler, e, cleanup := setupTestHandler(t)
	defer cleanup()

	body := `{"email": ["email", "correo", "邮箱"], "phone": ["phone", "tel"]}`
	req := httptest.NewRequest(http.MethodPut, "/patterns", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.UpdatePatterns(c); err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var patterns FieldPatterns
	if err := json.Unmarshal(rec.Body.Bytes(), &patterns); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(patterns.Patterns[FieldEmail]) != 3 {
		t.Errorf("Expected 3 email patterns, got %d", len(patterns.Patterns[FieldEmail]))
	}
}

func TestSiteMappingHandler(t *testing.T) {
	handler, e, cleanup := setupTestHandler(t)
	defer cleanup()

	// Save site mapping
	saveBody := `{"domain": "example.com", "field_mappings": {"#email": "email"}, "user_corrections": 1}`
	saveReq := httptest.NewRequest(http.MethodPut, "/sites/example.com", bytes.NewBufferString(saveBody))
	saveReq.Header.Set("Content-Type", "application/json")
	saveRec := httptest.NewRecorder()
	saveCtx := e.NewContext(saveReq, saveRec)
	saveCtx.SetParamNames("domain")
	saveCtx.SetParamValues("example.com")

	if err := handler.SaveSiteMapping(saveCtx); err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if saveRec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, saveRec.Code)
	}

	// Get site mapping
	getReq := httptest.NewRequest(http.MethodGet, "/sites/example.com", nil)
	getRec := httptest.NewRecorder()
	getCtx := e.NewContext(getReq, getRec)
	getCtx.SetParamNames("domain")
	getCtx.SetParamValues("example.com")

	if err := handler.GetSiteMapping(getCtx); err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if getRec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, getRec.Code)
	}

	var mapping SiteMapping
	if err := json.Unmarshal(getRec.Body.Bytes(), &mapping); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if mapping.FieldMappings["#email"] != "email" {
		t.Errorf("Expected email mapping, got '%s'", mapping.FieldMappings["#email"])
	}
}

func TestGetSiteMappingHandler_NotFound(t *testing.T) {
	handler, e, cleanup := setupTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/sites/nonexistent.com", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("domain")
	c.SetParamValues("nonexistent.com")

	if err := handler.GetSiteMapping(c); err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestDetectFieldsHandler(t *testing.T) {
	handler, e, cleanup := setupTestHandler(t)
	defer cleanup()

	body := `{
		"fields": [
			{"id": "email", "name": "email", "type": "email", "placeholder": "Enter email"},
			{"id": "phone", "name": "phone", "type": "tel", "placeholder": "Phone number"},
			{"id": "unknown", "name": "xyz123", "type": "text"}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/detect", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.DetectFields(c); err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response DetectResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Count < 2 {
		t.Errorf("Expected at least 2 detected fields, got %d", response.Count)
	}

	// Check that email was detected
	var emailFound bool
	for _, field := range response.Fields {
		if field.FieldType == FieldEmail {
			emailFound = true
			if field.Confidence < 0.8 {
				t.Errorf("Expected high confidence for email field, got %f", field.Confidence)
			}
		}
	}

	if !emailFound {
		t.Error("Email field was not detected")
	}
}

func TestGetConfigHandler(t *testing.T) {
	handler, e, cleanup := setupTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/config", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.GetConfig(c); err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var config Config
	if err := json.Unmarshal(rec.Body.Bytes(), &config); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !config.Enabled {
		t.Error("Expected config to be enabled by default")
	}

	if config.Widget.KeyboardShortcut != "Ctrl+Shift+F" {
		t.Errorf("Expected keyboard shortcut 'Ctrl+Shift+F', got '%s'", config.Widget.KeyboardShortcut)
	}
}
