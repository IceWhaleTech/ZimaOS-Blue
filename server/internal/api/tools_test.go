package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/adapters"
)

func TestToolsHandler_GetCompatibility(t *testing.T) {
	router := adapters.NewCapabilityRouter(nil)
	handler := NewToolsHandler(router)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tools/compatibility", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.GetCompatibility(c); err != nil {
		t.Fatalf("GetCompatibility failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp CompatibilityResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Should have some providers
	if len(resp.Providers) == 0 {
		t.Error("expected at least one provider")
	}
}

func TestToolsHandler_GetProviderCapabilities(t *testing.T) {
	router := adapters.NewCapabilityRouter(nil)
	handler := NewToolsHandler(router)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/anthropic/capabilities", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("name")
	c.SetParamValues("anthropic")

	if err := handler.GetProviderCapabilities(c); err != nil {
		t.Fatalf("GetProviderCapabilities failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["name"] != "anthropic" {
		t.Errorf("expected name 'anthropic', got '%s'", resp["name"])
	}

	if resp["tool_calling"] != "native" {
		t.Errorf("expected tool_calling 'native', got '%s'", resp["tool_calling"])
	}
}

func TestToolsHandler_GetProviderCapabilities_NotFound(t *testing.T) {
	router := adapters.NewCapabilityRouter(nil)
	handler := NewToolsHandler(router)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/nonexistent/capabilities", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("name")
	c.SetParamValues("nonexistent")

	if err := handler.GetProviderCapabilities(c); err != nil {
		t.Fatalf("GetProviderCapabilities failed: %v", err)
	}

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestToolsHandler_GetAllCapabilities(t *testing.T) {
	router := adapters.NewCapabilityRouter(nil)
	handler := NewToolsHandler(router)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/capabilities", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.GetAllCapabilities(c); err != nil {
		t.Fatalf("GetAllCapabilities failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	providers, ok := resp["providers"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'providers' to be a map")
	}

	if len(providers) == 0 {
		t.Error("expected at least one provider")
	}
}

func TestToolsHandler_TestToolCalling(t *testing.T) {
	router := adapters.NewCapabilityRouter(nil)
	handler := NewToolsHandler(router)

	e := echo.New()
	body := `{"provider": "anthropic"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tools/test", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.TestToolCalling(c); err != nil {
		t.Fatalf("TestToolCalling failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp TestToolCallingResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Error("expected Success to be true")
	}

	if resp.Provider != "anthropic" {
		t.Errorf("expected Provider 'anthropic', got '%s'", resp.Provider)
	}

	if resp.ToolCalling != "native" {
		t.Errorf("expected ToolCalling 'native', got '%s'", resp.ToolCalling)
	}
}

func TestToolsHandler_TestToolCalling_MissingProvider(t *testing.T) {
	router := adapters.NewCapabilityRouter(nil)
	handler := NewToolsHandler(router)

	e := echo.New()
	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tools/test", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.TestToolCalling(c); err != nil {
		t.Fatalf("TestToolCalling failed: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestToolsHandler_TestToolCalling_UnknownProvider(t *testing.T) {
	router := adapters.NewCapabilityRouter(nil)
	handler := NewToolsHandler(router)

	e := echo.New()
	body := `{"provider": "nonexistent"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tools/test", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.TestToolCalling(c); err != nil {
		t.Fatalf("TestToolCalling failed: %v", err)
	}

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestToolsHandler_TestToolCalling_Ollama(t *testing.T) {
	router := adapters.NewCapabilityRouter(nil)
	handler := NewToolsHandler(router)

	e := echo.New()
	body := `{"provider": "ollama"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tools/test", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.TestToolCalling(c); err != nil {
		t.Fatalf("TestToolCalling failed: %v", err)
	}

	var resp TestToolCallingResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Ollama has partial tool calling support
	if resp.ToolCalling != "partial" {
		t.Errorf("expected ToolCalling 'partial', got '%s'", resp.ToolCalling)
	}

	if resp.AdapterUsed != "ccnexus" {
		t.Errorf("expected AdapterUsed 'ccnexus', got '%s'", resp.AdapterUsed)
	}
}
