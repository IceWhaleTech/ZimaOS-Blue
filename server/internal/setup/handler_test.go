package setup

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestNewHandler(t *testing.T) {
	tempDir := t.TempDir()
	h := NewHandler(tempDir)

	if h == nil {
		t.Fatal("Expected handler, got nil")
	}

	if h.status.Completed {
		t.Error("Expected setup not completed initially")
	}

	if h.status.TotalSteps != 5 {
		t.Errorf("Expected 5 total steps, got %d", h.status.TotalSteps)
	}
}

func TestHandler_GetStatus(t *testing.T) {
	tempDir := t.TempDir()
	h := NewHandler(tempDir)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.GetStatus(c); err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var status SetupStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if status.Completed {
		t.Error("Expected setup not completed")
	}

	if status.TotalSteps != 5 {
		t.Errorf("Expected 5 total steps, got %d", status.TotalSteps)
	}
}

func TestHandler_GetDefaults(t *testing.T) {
	tempDir := t.TempDir()
	h := NewHandler(tempDir)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/setup/defaults", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.GetDefaults(c); err != nil {
		t.Fatalf("GetDefaults failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var defaults map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &defaults); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if defaults["language"] != "en" {
		t.Errorf("Expected default language 'en', got %v", defaults["language"])
	}

	providers, ok := defaults["providers"].([]interface{})
	if !ok || len(providers) == 0 {
		t.Error("Expected providers list")
	}
}

func TestHandler_ValidateStep_BasicSettings(t *testing.T) {
	tempDir := t.TempDir()
	h := NewHandler(tempDir)

	tests := []struct {
		name      string
		step      int
		config    SetupConfig
		expectValid bool
	}{
		{
			name: "valid_basic_settings",
			step: 1,
			config: SetupConfig{
				Language: "en",
				Timezone: "UTC",
			},
			expectValid: true,
		},
		{
			name: "missing_language",
			step: 1,
			config: SetupConfig{
				Timezone: "UTC",
			},
			expectValid: false,
		},
		{
			name: "missing_timezone",
			step: 1,
			config: SetupConfig{
				Language: "en",
			},
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]interface{}{
				"step":   tt.step,
				"config": tt.config,
			})

			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/api/setup/validate", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			if err := h.ValidateStep(c); err != nil {
				t.Fatalf("ValidateStep failed: %v", err)
			}

			var result ValidationResult
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			if result.Valid != tt.expectValid {
				t.Errorf("Expected valid=%v, got %v", tt.expectValid, result.Valid)
			}
		})
	}
}

func TestHandler_ValidateStep_LLMConfig(t *testing.T) {
	tempDir := t.TempDir()
	h := NewHandler(tempDir)

	tests := []struct {
		name        string
		config      SetupConfig
		expectValid bool
	}{
		{
			name: "valid_openai",
			config: SetupConfig{
				LLMProvider: "openai",
				LLMAPIKey:   "sk-test-key",
				LLMModel:    "gpt-4o-mini",
			},
			expectValid: true,
		},
		{
			name: "valid_ollama_no_key",
			config: SetupConfig{
				LLMProvider: "ollama",
				LLMModel:    "llama3.2",
			},
			expectValid: true,
		},
		{
			name: "missing_api_key_for_openai",
			config: SetupConfig{
				LLMProvider: "openai",
				LLMModel:    "gpt-4o-mini",
			},
			expectValid: false,
		},
		{
			name: "custom_missing_base_url",
			config: SetupConfig{
				LLMProvider: "custom",
				LLMAPIKey:   "test-key",
				LLMModel:    "custom-model",
			},
			expectValid: false,
		},
		{
			name: "valid_custom",
			config: SetupConfig{
				LLMProvider: "custom",
				LLMAPIKey:   "test-key",
				LLMModel:    "custom-model",
				LLMBaseURL:  "https://custom.api.com",
			},
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]interface{}{
				"step":   2,
				"config": tt.config,
			})

			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/api/setup/validate", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			if err := h.ValidateStep(c); err != nil {
				t.Fatalf("ValidateStep failed: %v", err)
			}

			var result ValidationResult
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			if result.Valid != tt.expectValid {
				t.Errorf("Expected valid=%v, got %v (errors: %v)", tt.expectValid, result.Valid, result.Errors)
			}
		})
	}
}

func TestHandler_ValidateStep_Security(t *testing.T) {
	tempDir := t.TempDir()
	h := NewHandler(tempDir)

	tests := []struct {
		name        string
		config      SetupConfig
		expectValid bool
	}{
		{
			name: "valid_security",
			config: SetupConfig{
				AdminUsername: "admin",
				AdminPassword: "securepassword123",
			},
			expectValid: true,
		},
		{
			name: "short_username",
			config: SetupConfig{
				AdminUsername: "ab",
				AdminPassword: "securepassword123",
			},
			expectValid: false,
		},
		{
			name: "short_password",
			config: SetupConfig{
				AdminUsername: "admin",
				AdminPassword: "short",
			},
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]interface{}{
				"step":   3,
				"config": tt.config,
			})

			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/api/setup/validate", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			if err := h.ValidateStep(c); err != nil {
				t.Fatalf("ValidateStep failed: %v", err)
			}

			var result ValidationResult
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			if result.Valid != tt.expectValid {
				t.Errorf("Expected valid=%v, got %v", tt.expectValid, result.Valid)
			}
		})
	}
}

func TestHandler_CompleteSetup(t *testing.T) {
	tempDir := t.TempDir()
	h := NewHandler(tempDir)

	config := SetupConfig{
		Language:      "en",
		Timezone:      "UTC",
		LLMProvider:   "ollama",
		LLMModel:      "llama3.2",
		AdminUsername: "admin",
		AdminPassword: "securepassword123",
	}

	body, _ := json.Marshal(config)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/setup/complete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.CompleteSetup(c); err != nil {
		t.Fatalf("CompleteSetup failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["success"] != true {
		t.Errorf("Expected success=true, got %v", result["success"])
	}

	// Verify setup is marked complete
	if !h.IsSetupComplete() {
		t.Error("Expected setup to be marked complete")
	}

	// Verify config file was created
	configPath := filepath.Join(tempDir, "wizard_config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Expected config file to be created")
	}
}

func TestHandler_ResetSetup(t *testing.T) {
	tempDir := t.TempDir()
	h := NewHandler(tempDir)

	// First complete setup
	config := SetupConfig{
		Language:      "en",
		Timezone:      "UTC",
		LLMProvider:   "ollama",
		LLMModel:      "llama3.2",
		AdminUsername: "admin",
		AdminPassword: "securepassword123",
	}

	body, _ := json.Marshal(config)
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/setup/complete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	h.CompleteSetup(c)

	// Now reset
	req = httptest.NewRequest(http.MethodPost, "/api/setup/reset", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)

	if err := h.ResetSetup(c); err != nil {
		t.Fatalf("ResetSetup failed: %v", err)
	}

	if h.IsSetupComplete() {
		t.Error("Expected setup to be reset")
	}
}

func TestHandler_TestConnection(t *testing.T) {
	tempDir := t.TempDir()
	h := NewHandler(tempDir)

	tests := []struct {
		name          string
		connType      string
		config        map[string]string
		expectSuccess bool
	}{
		{
			name:     "llm_valid",
			connType: "llm",
			config: map[string]string{
				"provider": "openai",
				"api_key":  "sk-test",
			},
			expectSuccess: true,
		},
		{
			name:     "llm_missing_key",
			connType: "llm",
			config: map[string]string{
				"provider": "openai",
			},
			expectSuccess: false,
		},
		{
			name:     "homeassistant_valid",
			connType: "homeassistant",
			config: map[string]string{
				"url":   "http://localhost:8123",
				"token": "test-token",
			},
			expectSuccess: true,
		},
		{
			name:     "telegram_valid",
			connType: "telegram",
			config: map[string]string{
				"token": "123456:ABC-DEF",
			},
			expectSuccess: true,
		},
		{
			name:          "unknown_type",
			connType:      "unknown",
			config:        map[string]string{},
			expectSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]interface{}{
				"type":   tt.connType,
				"config": tt.config,
			})

			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/api/setup/test-connection", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			if err := h.TestConnection(c); err != nil {
				t.Fatalf("TestConnection failed: %v", err)
			}

			var result map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			success, _ := result["success"].(bool)
			if success != tt.expectSuccess {
				t.Errorf("Expected success=%v, got %v", tt.expectSuccess, success)
			}
		})
	}
}

func TestHandler_StatusPersistence(t *testing.T) {
	tempDir := t.TempDir()

	// Create handler and complete setup
	h1 := NewHandler(tempDir)
	config := SetupConfig{
		Language:      "en",
		Timezone:      "UTC",
		LLMProvider:   "ollama",
		LLMModel:      "llama3.2",
		AdminUsername: "admin",
		AdminPassword: "securepassword123",
	}

	body, _ := json.Marshal(config)
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/setup/complete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	h1.CompleteSetup(c)

	// Create new handler (simulating restart)
	h2 := NewHandler(tempDir)

	if !h2.IsSetupComplete() {
		t.Error("Expected setup status to persist after restart")
	}
}
