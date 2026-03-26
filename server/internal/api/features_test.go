package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/features"
)

func TestFeaturesHandler_GetAllFeatures(t *testing.T) {
	gate := features.NewFeatureGate()
	handler := NewFeaturesHandler(gate)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/features", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.GetAllFeatures(c); err != nil {
		t.Fatalf("GetAllFeatures failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if _, ok := resp["features"]; !ok {
		t.Error("expected 'features' in response")
	}
}

func TestFeaturesHandler_GetStatus(t *testing.T) {
	gate := features.NewFeatureGate()
	handler := NewFeaturesHandler(gate)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/features/status", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.GetStatus(c); err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if _, ok := resp["total_features"]; !ok {
		t.Error("expected 'total_features' in response")
	}
	if _, ok := resp["enabled_count"]; !ok {
		t.Error("expected 'enabled_count' in response")
	}
}

func TestFeaturesHandler_GetFeature(t *testing.T) {
	gate := features.NewFeatureGate()
	handler := NewFeaturesHandler(gate)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/features/basic_chat", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("name")
	c.SetParamValues("basic_chat")

	if err := handler.GetFeature(c); err != nil {
		t.Fatalf("GetFeature failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp features.FeatureInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Name != "Basic Chat" {
		t.Errorf("expected feature name 'Basic Chat', got '%s'", resp.Name)
	}
}

func TestFeaturesHandler_GetFeature_NotFound(t *testing.T) {
	gate := features.NewFeatureGate()
	handler := NewFeaturesHandler(gate)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/features/nonexistent", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("name")
	c.SetParamValues("nonexistent")

	if err := handler.GetFeature(c); err != nil {
		t.Fatalf("GetFeature failed: %v", err)
	}

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestFeaturesHandler_GetCLIDependentFeatures(t *testing.T) {
	gate := features.NewFeatureGate()
	handler := NewFeaturesHandler(gate)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/features/cli-dependent", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.GetCLIDependentFeatures(c); err != nil {
		t.Fatalf("GetCLIDependentFeatures failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	featuresArr, ok := resp["features"].([]interface{})
	if !ok {
		t.Fatal("expected 'features' to be an array")
	}

	if len(featuresArr) != 0 {
		t.Errorf("expected no CLI-dependent features, got %d", len(featuresArr))
	}
}

func TestFeaturesHandler_GetFeaturesByCategory(t *testing.T) {
	gate := features.NewFeatureGate()
	handler := NewFeaturesHandler(gate)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/features/by-category", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.GetFeaturesByCategory(c); err != nil {
		t.Fatalf("GetFeaturesByCategory failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	categories, ok := resp["categories"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'categories' to be a map")
	}

	// Should have core category
	if _, ok := categories["core"]; !ok {
		t.Error("expected 'core' category")
	}
}

func TestFeatureMiddleware_Enabled(t *testing.T) {
	gate := features.NewFeatureGate()
	// basic_chat is always enabled
	middleware := FeatureMiddleware(gate, features.FeatureBasicChat)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handlerCalled := false
	handler := func(c echo.Context) error {
		handlerCalled = true
		return c.String(http.StatusOK, "OK")
	}

	if err := middleware(handler)(c); err != nil {
		t.Fatalf("middleware failed: %v", err)
	}

	if !handlerCalled {
		t.Error("expected handler to be called")
	}
}

func TestFeatureMiddleware_Disabled(t *testing.T) {
	gate := features.NewFeatureGate()
	middleware := FeatureMiddleware(gate, features.FeatureToolCalling)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handlerCalled := false
	handler := func(c echo.Context) error {
		handlerCalled = true
		return c.String(http.StatusOK, "OK")
	}

	if err := middleware(handler)(c); err != nil {
		t.Fatalf("middleware failed: %v", err)
	}

	if !handlerCalled {
		t.Error("expected handler to be called")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestFeatureMiddleware_EnabledWithCLI(t *testing.T) {
	gate := features.NewFeatureGate()
	gate.SetCLIInstalled(false)
	middleware := FeatureMiddleware(gate, features.FeatureToolCalling)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handlerCalled := false
	handler := func(c echo.Context) error {
		handlerCalled = true
		return c.String(http.StatusOK, "OK")
	}

	if err := middleware(handler)(c); err != nil {
		t.Fatalf("middleware failed: %v", err)
	}

	if !handlerCalled {
		t.Error("expected handler to be called even when legacy CLI flag is false")
	}
}
