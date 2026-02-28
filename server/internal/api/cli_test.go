package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/features"
)

func TestCLIHandler_GetConfig(t *testing.T) {
	cfg := config.DefaultClaudeCodeCLIConfig()
	gate := features.NewFeatureGate()
	handler := NewCLIHandler(cfg, gate)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cli/config", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.GetConfig(c); err != nil {
		t.Fatalf("GetConfig failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp CLIConfigResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Enabled {
		t.Error("expected Enabled to be true")
	}
}

func TestCLIHandler_UpdateConfig(t *testing.T) {
	cfg := config.DefaultClaudeCodeCLIConfig()
	gate := features.NewFeatureGate()
	handler := NewCLIHandler(cfg, gate)

	e := echo.New()
	body := `{"enabled": false}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/cli/config", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.UpdateConfig(c); err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	// Verify config was updated
	if cfg.Enabled {
		t.Error("expected Enabled to be false after update")
	}
}

func TestCLIHandler_EnableCLI(t *testing.T) {
	cfg := config.DefaultClaudeCodeCLIConfig()
	cfg.Enabled = false
	gate := features.NewFeatureGate()
	handler := NewCLIHandler(cfg, gate)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cli/enable", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.EnableCLI(c); err != nil {
		t.Fatalf("EnableCLI failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp EnableCLIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Error("expected Success to be true")
	}

	// Verify config was updated
	if !cfg.Enabled {
		t.Error("expected Enabled to be true after enable")
	}
}

func TestCLIHandler_EnableCLI_AlreadyEnabled(t *testing.T) {
	cfg := config.DefaultClaudeCodeCLIConfig()
	cfg.Enabled = true
	gate := features.NewFeatureGate()
	handler := NewCLIHandler(cfg, gate)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cli/enable", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.EnableCLI(c); err != nil {
		t.Fatalf("EnableCLI failed: %v", err)
	}

	var resp EnableCLIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Error("expected Success to be true")
	}

	if resp.Message != "CLI is already enabled" {
		t.Errorf("expected 'CLI is already enabled', got '%s'", resp.Message)
	}
}

func TestCLIHandler_DisableCLI_RequiresConfirmation(t *testing.T) {
	cfg := config.DefaultClaudeCodeCLIConfig()
	cfg.Enabled = true
	gate := features.NewFeatureGate()
	handler := NewCLIHandler(cfg, gate)

	e := echo.New()
	body := `{"confirm": false}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cli/disable", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.DisableCLI(c); err != nil {
		t.Fatalf("DisableCLI failed: %v", err)
	}

	var resp DisableCLIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Success {
		t.Error("expected Success to be false without confirmation")
	}

	if resp.Warning == "" {
		t.Error("expected Warning to be non-empty")
	}

	// Config should not be changed
	if !cfg.Enabled {
		t.Error("expected Enabled to still be true")
	}
}

func TestCLIHandler_DisableCLI_WithConfirmation(t *testing.T) {
	cfg := config.DefaultClaudeCodeCLIConfig()
	cfg.Enabled = true
	gate := features.NewFeatureGate()
	handler := NewCLIHandler(cfg, gate)

	e := echo.New()
	body := `{"confirm": true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cli/disable", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.DisableCLI(c); err != nil {
		t.Fatalf("DisableCLI failed: %v", err)
	}

	var resp DisableCLIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Error("expected Success to be true with confirmation")
	}

	// Config should be changed
	if cfg.Enabled {
		t.Error("expected Enabled to be false after disable")
	}

	// Should have disabled features listed
	if len(resp.FeaturesDisabled) == 0 {
		t.Error("expected FeaturesDisabled to be non-empty")
	}
}

func TestCLIHandler_DisableCLI_AlreadyDisabled(t *testing.T) {
	cfg := config.DefaultClaudeCodeCLIConfig()
	cfg.Enabled = false
	gate := features.NewFeatureGate()
	handler := NewCLIHandler(cfg, gate)

	e := echo.New()
	body := `{"confirm": true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cli/disable", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.DisableCLI(c); err != nil {
		t.Fatalf("DisableCLI failed: %v", err)
	}

	var resp DisableCLIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Error("expected Success to be true")
	}

	if resp.Message != "CLI is already disabled" {
		t.Errorf("expected 'CLI is already disabled', got '%s'", resp.Message)
	}
}

func TestCLIHandler_OnConfigChange(t *testing.T) {
	cfg := config.DefaultClaudeCodeCLIConfig()
	gate := features.NewFeatureGate()
	handler := NewCLIHandler(cfg, gate)

	callbackCalled := false
	handler.SetOnConfigChange(func(c *config.ClaudeCodeCLIConfig) error {
		callbackCalled = true
		return nil
	})

	e := echo.New()
	body := `{"enabled": false}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/cli/config", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.UpdateConfig(c); err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	if !callbackCalled {
		t.Error("expected callback to be called")
	}
}

func TestCLIHandler_GetCLIFeatureMatrix(t *testing.T) {
	cfg := config.DefaultClaudeCodeCLIConfig()
	gate := features.NewFeatureGate()
	handler := NewCLIHandler(cfg, gate)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cli/feature-matrix", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.GetCLIFeatureMatrix(c); err != nil {
		t.Fatalf("GetCLIFeatureMatrix failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	matrix, ok := resp["matrix"].([]interface{})
	if !ok {
		t.Fatal("expected 'matrix' to be an array")
	}

	if len(matrix) == 0 {
		t.Error("expected matrix to have entries")
	}

	agentFound := false
	mcpFound := false
	for _, item := range matrix {
		row, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		feature, _ := row["feature"].(string)
		switch feature {
		case "Agent Mode":
			agentFound = true
			if got, _ := row["without_cli"].(string); got != "Full" {
				t.Fatalf("Agent Mode without_cli = %q, want Full", got)
			}
			if req, _ := row["requires_cli"].(bool); req {
				t.Fatalf("Agent Mode requires_cli = true, want false")
			}
		case "MCP Tools":
			mcpFound = true
			if got, _ := row["without_cli"].(string); got != "Full" {
				t.Fatalf("MCP Tools without_cli = %q, want Full", got)
			}
		}
	}
	if !agentFound {
		t.Fatal("feature matrix missing Agent Mode row")
	}
	if !mcpFound {
		t.Fatal("feature matrix missing MCP Tools row")
	}
}
