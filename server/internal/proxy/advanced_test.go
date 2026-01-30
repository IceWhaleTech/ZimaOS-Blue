package proxy

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestModelCompatLayer tests model compatibility layer
func TestModelCompatLayer(t *testing.T) {
	config := &ModelCompatConfig{
		AutoDetect:       true,
		DetectionCache:   1 * time.Hour,
		ToolCallFallback: "prompt",
		ModelOverrides:   make(map[string]*ModelFeatures),
	}
	mcl := NewModelCompatLayer(config)

	t.Run("GetKnownModelFeatures", func(t *testing.T) {
		features := mcl.GetFeatures("claude-opus-4-5")
		if features == nil {
			t.Fatal("expected features for claude-opus-4-5")
		}
		if !features.ToolCalling {
			t.Error("expected tool calling to be supported")
		}
		if !features.Vision {
			t.Error("expected vision to be supported")
		}
		if features.MaxContextTokens != 200000 {
			t.Errorf("expected max context 200000, got %d", features.MaxContextTokens)
		}
	})

	t.Run("GetUnknownModelFeatures", func(t *testing.T) {
		features := mcl.GetFeatures("unknown-model")
		if features == nil {
			t.Fatal("expected default features for unknown model")
		}
		if features.ToolCalling {
			t.Error("expected tool calling to be disabled for unknown model")
		}
		if features.MaxContextTokens != 8192 {
			t.Errorf("expected default max context 8192, got %d", features.MaxContextTokens)
		}
	})

	t.Run("SetFeatures", func(t *testing.T) {
		customFeatures := &ModelFeatures{
			Model:            "custom-model",
			ToolCalling:      true,
			Vision:           true,
			MaxContextTokens: 50000,
		}
		mcl.SetFeatures("custom-model", customFeatures)

		features := mcl.GetFeatures("custom-model")
		if features.MaxContextTokens != 50000 {
			t.Errorf("expected max context 50000, got %d", features.MaxContextTokens)
		}
	})

	t.Run("ListModels", func(t *testing.T) {
		models := mcl.ListModels()
		if len(models) == 0 {
			t.Error("expected at least one model")
		}
	})

	t.Run("AdaptRequestToolCallError", func(t *testing.T) {
		errorConfig := &ModelCompatConfig{
			ToolCallFallback: "error",
		}
		errorMcl := NewModelCompatLayer(errorConfig)

		req := &ChatRequest{
			Model: "llama3",
			Tools: []Tool{{Name: "test", Description: "test tool"}},
		}

		_, err := errorMcl.AdaptRequest("llama3", req)
		if err != ErrToolCallingNotSupported {
			t.Errorf("expected ErrToolCallingNotSupported, got %v", err)
		}
	})

	t.Run("AdaptRequestToolCallSkip", func(t *testing.T) {
		skipConfig := &ModelCompatConfig{
			ToolCallFallback: "skip",
		}
		skipMcl := NewModelCompatLayer(skipConfig)

		req := &ChatRequest{
			Model: "llama3",
			Tools: []Tool{{Name: "test", Description: "test tool"}},
		}

		adapted, err := skipMcl.AdaptRequest("llama3", req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(adapted.Tools) != 0 {
			t.Error("expected tools to be removed")
		}
	})

	t.Run("AdaptRequestToolCallPrompt", func(t *testing.T) {
		req := &ChatRequest{
			Model: "llama3",
			Tools: []Tool{{Name: "test", Description: "test tool"}},
		}

		adapted, err := mcl.AdaptRequest("llama3", req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(adapted.Tools) != 0 {
			t.Error("expected tools to be converted to prompt")
		}
		if adapted.System == "" {
			t.Error("expected system prompt to contain tool info")
		}
	})

	t.Run("Stats", func(t *testing.T) {
		stats := mcl.Stats()
		if stats["known_models"].(int) == 0 {
			t.Error("expected known models count")
		}
	})
}

// TestChatRequest tests ChatRequest methods
func TestChatRequest(t *testing.T) {
	t.Run("HasImages", func(t *testing.T) {
		req := &ChatRequest{
			Messages: []ChatMessage{
				{Role: "user", Content: "Hello"},
			},
		}
		if req.HasImages() {
			t.Error("expected no images")
		}

		reqWithImage := &ChatRequest{
			Messages: []ChatMessage{
				{
					Role: "user",
					Content: []interface{}{
						map[string]interface{}{"type": "text", "text": "Hello"},
						map[string]interface{}{"type": "image", "url": "http://example.com/img.png"},
					},
				},
			},
		}
		if !reqWithImage.HasImages() {
			t.Error("expected images to be detected")
		}
	})

	t.Run("TokenCount", func(t *testing.T) {
		req := &ChatRequest{
			Messages: []ChatMessage{
				{Role: "user", Content: "Hello world this is a test message"},
			},
			System: "You are a helpful assistant",
		}
		count := req.TokenCount()
		if count == 0 {
			t.Error("expected non-zero token count")
		}
	})
}

// TestMockHandler tests mock endpoint handling
func TestMockHandler(t *testing.T) {
	config := &MockConfig{
		Enabled:   true,
		Endpoints: make([]*MockEndpoint, 0),
	}
	mh := NewMockHandler(config)

	t.Run("HandleDefaultEndpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		rr := httptest.NewRecorder()

		handled := mh.Handle(rr, req)
		if !handled {
			t.Error("expected /health to be handled")
		}
		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}
	})

	t.Run("HandleModelsEndpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/models", nil)
		rr := httptest.NewRecorder()

		handled := mh.Handle(rr, req)
		if !handled {
			t.Error("expected /v1/models to be handled")
		}
		if rr.Header().Get("X-Mock-Response") != "true" {
			t.Error("expected X-Mock-Response header")
		}
	})

	t.Run("HandleUnknownEndpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/unknown", nil)
		rr := httptest.NewRecorder()

		handled := mh.Handle(rr, req)
		if handled {
			t.Error("expected /unknown to not be handled")
		}
	})

	t.Run("AddEndpoint", func(t *testing.T) {
		mh.AddEndpoint(&MockEndpoint{
			Path:       "/custom",
			Method:     "GET",
			StatusCode: 201,
			Response:   map[string]string{"custom": "response"},
			Enabled:    true,
		})

		req := httptest.NewRequest("GET", "/custom", nil)
		rr := httptest.NewRecorder()

		handled := mh.Handle(rr, req)
		if !handled {
			t.Error("expected /custom to be handled")
		}
		if rr.Code != 201 {
			t.Errorf("expected status 201, got %d", rr.Code)
		}
	})

	t.Run("RemoveEndpoint", func(t *testing.T) {
		removed := mh.RemoveEndpoint("/custom", "GET")
		if !removed {
			t.Error("expected endpoint to be removed")
		}

		req := httptest.NewRequest("GET", "/custom", nil)
		rr := httptest.NewRecorder()

		handled := mh.Handle(rr, req)
		if handled {
			t.Error("expected /custom to not be handled after removal")
		}
	})

	t.Run("SetEnabled", func(t *testing.T) {
		mh.SetEnabled("/health", "GET", false)

		req := httptest.NewRequest("GET", "/health", nil)
		rr := httptest.NewRecorder()

		handled := mh.Handle(rr, req)
		if handled {
			t.Error("expected /health to not be handled when disabled")
		}

		mh.SetEnabled("/health", "GET", true)
	})

	t.Run("DisableGlobally", func(t *testing.T) {
		mh.SetGlobalEnabled(false)

		req := httptest.NewRequest("GET", "/v1/models", nil)
		rr := httptest.NewRecorder()

		handled := mh.Handle(rr, req)
		if handled {
			t.Error("expected no handling when globally disabled")
		}

		mh.SetGlobalEnabled(true)
	})

	t.Run("ListEndpoints", func(t *testing.T) {
		endpoints := mh.ListEndpoints()
		if len(endpoints) == 0 {
			t.Error("expected at least one endpoint")
		}
	})

	t.Run("Stats", func(t *testing.T) {
		stats := mh.Stats()
		if stats["enabled"] != true {
			t.Error("expected enabled to be true")
		}
		if stats["endpoint_count"].(int) == 0 {
			t.Error("expected endpoint count")
		}
	})
}

// TestConfigWatcher tests configuration watcher
func TestConfigWatcher(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	initialConfig := `
enabled: true
port:
  value: 9000
`
	err := os.WriteFile(configPath, []byte(initialConfig), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	t.Run("CreateWatcher", func(t *testing.T) {
		cw := NewConfigWatcher(configPath, 100*time.Millisecond)
		if cw == nil {
			t.Fatal("expected watcher to be created")
		}
		if cw.GetConfigPath() != configPath {
			t.Errorf("expected config path %s, got %s", configPath, cw.GetConfigPath())
		}
	})

	t.Run("DetectChanges", func(t *testing.T) {
		changed := make(chan bool, 1)

		cw := NewConfigWatcher(configPath, 50*time.Millisecond)
		cw.OnChange = func(old, new *ProxyConfig) {
			changed <- true
		}

		cw.Start()
		defer cw.Stop()

		// Wait for initial state
		time.Sleep(100 * time.Millisecond)

		// Modify config
		newConfig := `
enabled: false
port:
  value: 9001
`
		err := os.WriteFile(configPath, []byte(newConfig), 0644)
		if err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		// Wait for change detection
		select {
		case <-changed:
			// Success
		case <-time.After(500 * time.Millisecond):
			t.Error("expected change to be detected")
		}
	})

	t.Run("ForceReload", func(t *testing.T) {
		reloaded := false

		cw := NewConfigWatcher(configPath, 1*time.Hour) // Long interval
		cw.OnChange = func(old, new *ProxyConfig) {
			reloaded = true
		}

		err := cw.ForceReload()
		if err != nil {
			t.Fatalf("force reload failed: %v", err)
		}

		if !reloaded {
			t.Error("expected OnChange to be called")
		}
	})

	t.Run("SetConfigPath", func(t *testing.T) {
		cw := NewConfigWatcher(configPath, 100*time.Millisecond)
		newPath := filepath.Join(tmpDir, "new-config.yaml")
		cw.SetConfigPath(newPath)

		if cw.GetConfigPath() != newPath {
			t.Errorf("expected path %s, got %s", newPath, cw.GetConfigPath())
		}
	})

	t.Run("Stats", func(t *testing.T) {
		cw := NewConfigWatcher(configPath, 100*time.Millisecond)
		stats := cw.Stats()

		if stats["config_path"] != configPath {
			t.Error("expected config path in stats")
		}
	})
}

// TestDataMasker tests data masking (stub)
func TestDataMasker(t *testing.T) {
	config := &MaskingConfig{
		Enabled: true,
		Rules:   make([]*MaskingRule, 0),
	}
	dm := NewDataMasker(config)

	t.Run("MaskStub", func(t *testing.T) {
		input := "This contains sensitive@email.com data"
		output := dm.Mask(input, MaskingBoth)

		// Stub implementation returns input unchanged
		if output != input {
			t.Error("stub should return input unchanged")
		}
	})

	t.Run("AddRule", func(t *testing.T) {
		rule := &MaskingRule{
			ID:          "test-rule",
			Name:        "Test Rule",
			Category:    MaskingCustom,
			Pattern:     `test\d+`,
			Replacement: "[TEST]",
			Direction:   MaskingBoth,
			Enabled:     true,
		}

		err := dm.AddRule(rule)
		if err != nil {
			t.Fatalf("failed to add rule: %v", err)
		}

		retrieved, ok := dm.GetRule("test-rule")
		if !ok {
			t.Error("expected to find rule")
		}
		if retrieved.Name != "Test Rule" {
			t.Errorf("expected name 'Test Rule', got '%s'", retrieved.Name)
		}
	})

	t.Run("RemoveRule", func(t *testing.T) {
		removed := dm.RemoveRule("test-rule")
		if !removed {
			t.Error("expected rule to be removed")
		}

		_, ok := dm.GetRule("test-rule")
		if ok {
			t.Error("expected rule to not be found after removal")
		}
	})

	t.Run("SetRuleEnabled", func(t *testing.T) {
		dm.AddRule(&MaskingRule{
			ID:      "toggle-rule",
			Enabled: true,
		})

		dm.SetRuleEnabled("toggle-rule", false)
		rule, _ := dm.GetRule("toggle-rule")
		if rule.Enabled {
			t.Error("expected rule to be disabled")
		}
	})

	t.Run("ListRules", func(t *testing.T) {
		rules := dm.ListRules()
		if len(rules) == 0 {
			t.Error("expected at least one rule")
		}
	})

	t.Run("SetEnabled", func(t *testing.T) {
		dm.SetEnabled(false)
		if dm.IsEnabled() {
			t.Error("expected masking to be disabled")
		}

		dm.SetEnabled(true)
		if !dm.IsEnabled() {
			t.Error("expected masking to be enabled")
		}
	})

	t.Run("Stats", func(t *testing.T) {
		stats := dm.Stats()
		if stats["status"] != "stub" {
			t.Error("expected status to be 'stub'")
		}
	})

	t.Run("GetDefaultRules", func(t *testing.T) {
		defaults := GetDefaultRules()
		if len(defaults) == 0 {
			t.Error("expected default rules")
		}

		// Check for expected categories
		hasEmail := false
		hasCreditCard := false
		for _, rule := range defaults {
			if rule.ID == "email" {
				hasEmail = true
			}
			if rule.ID == "credit_card" {
				hasCreditCard = true
			}
		}

		if !hasEmail {
			t.Error("expected email rule in defaults")
		}
		if !hasCreditCard {
			t.Error("expected credit_card rule in defaults")
		}
	})
}

// TestConfigReloader tests configuration reloader
func TestConfigReloader(t *testing.T) {
	router := NewRouter(&RouteConfig{
		LoadBalancing: "priority",
		Providers: []*ProviderConfig{
			{Name: "test", Endpoint: "http://test.com", Enabled: true},
		},
	})
	mock := NewMockHandler(nil)
	compat := NewModelCompatLayer(nil)

	cr := NewConfigReloader(router, nil, nil, nil, mock, compat)

	t.Run("ApplyConfig", func(t *testing.T) {
		config := &ProxyConfig{
			Route: &RouteConfig{
				Providers: []*ProviderConfig{
					{Name: "new-provider", Endpoint: "http://new.com", Enabled: true},
				},
			},
			Mock: &MockConfig{
				Enabled: true,
			},
			ModelCompat: &ModelCompatConfig{
				ModelOverrides: map[string]*ModelFeatures{
					"custom": {Model: "custom", ToolCalling: true},
				},
			},
		}

		err := cr.ApplyConfig(config)
		if err != nil {
			t.Fatalf("failed to apply config: %v", err)
		}

		// Check mock was enabled
		if !mock.IsEnabled() {
			t.Error("expected mock to be enabled")
		}

		// Check model override was applied
		features := compat.GetFeatures("custom")
		if !features.ToolCalling {
			t.Error("expected custom model to have tool calling")
		}
	})
}
