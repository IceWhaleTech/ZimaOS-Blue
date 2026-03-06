package providerpool

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestRegistry(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "registry-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := NewFileStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStorage failed: %v", err)
	}

	registry, err := NewRegistry(storage)
	if err != nil {
		t.Fatalf("NewRegistry failed: %v", err)
	}

	t.Run("Register", func(t *testing.T) {
		provider := &Provider{
			ID:       "test-provider",
			Name:     "Test Provider",
			Type:     ProviderTypeCustom,
			Enabled:  true,
			BaseURL:  "https://api.test.com/v1",
			Priority: 10,
		}

		if err := registry.Register(provider); err != nil {
			t.Fatalf("Register failed: %v", err)
		}

		// Try to register again - should fail
		if err := registry.Register(provider); err != ErrProviderExists {
			t.Errorf("Expected ErrProviderExists, got %v", err)
		}
	})

	t.Run("Get", func(t *testing.T) {
		provider, err := registry.Get("test-provider")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if provider.Name != "Test Provider" {
			t.Errorf("Name mismatch: got %s, want Test Provider", provider.Name)
		}

		// Get non-existent
		_, err = registry.Get("non-existent")
		if err != ErrProviderNotFound {
			t.Errorf("Expected ErrProviderNotFound, got %v", err)
		}
	})

	t.Run("List", func(t *testing.T) {
		providers := registry.List()
		if len(providers) != 1 {
			t.Errorf("Expected 1 provider, got %d", len(providers))
		}
	})

	t.Run("Update", func(t *testing.T) {
		provider, _ := registry.Get("test-provider")
		provider.Name = "Updated Provider"

		if err := registry.Update(provider); err != nil {
			t.Fatalf("Update failed: %v", err)
		}

		updated, _ := registry.Get("test-provider")
		if updated.Name != "Updated Provider" {
			t.Errorf("Name not updated: got %s", updated.Name)
		}
	})

	t.Run("EnableDisable", func(t *testing.T) {
		if err := registry.Disable("test-provider"); err != nil {
			t.Fatalf("Disable failed: %v", err)
		}

		provider, _ := registry.Get("test-provider")
		if provider.Enabled {
			t.Error("Provider should be disabled")
		}

		if err := registry.Enable("test-provider"); err != nil {
			t.Fatalf("Enable failed: %v", err)
		}

		provider, _ = registry.Get("test-provider")
		if !provider.Enabled {
			t.Error("Provider should be enabled")
		}
	})

	t.Run("ListEnabled", func(t *testing.T) {
		// Add a disabled provider
		disabled := &Provider{
			ID:      "disabled-provider",
			Name:    "Disabled Provider",
			Type:    ProviderTypeCustom,
			Enabled: false,
		}
		registry.Register(disabled)

		enabled := registry.ListEnabled()
		if len(enabled) != 1 {
			t.Errorf("Expected 1 enabled provider, got %d", len(enabled))
		}
	})

	t.Run("APIKeyManagement", func(t *testing.T) {
		key := &APIKey{
			Key:     "sk-test-key-12345",
			Label:   "Test Key",
			Enabled: true,
		}

		if err := registry.AddAPIKey("test-provider", key); err != nil {
			t.Fatalf("AddAPIKey failed: %v", err)
		}

		provider, _ := registry.Get("test-provider")
		if len(provider.APIKeys) != 1 {
			t.Fatalf("Expected 1 API key, got %d", len(provider.APIKeys))
		}

		// Get API key
		apiKey, err := registry.GetAPIKey("test-provider")
		if err != nil {
			t.Fatalf("GetAPIKey failed: %v", err)
		}
		if apiKey.Key != "sk-test-key-12345" {
			t.Errorf("API key mismatch: got %s", apiKey.Key)
		}

		// Remove API key
		if err := registry.RemoveAPIKey("test-provider", provider.APIKeys[0].ID); err != nil {
			t.Fatalf("RemoveAPIKey failed: %v", err)
		}

		provider, _ = registry.Get("test-provider")
		if len(provider.APIKeys) != 0 {
			t.Errorf("Expected 0 API keys, got %d", len(provider.APIKeys))
		}
	})

	t.Run("Unregister", func(t *testing.T) {
		if err := registry.Unregister("test-provider"); err != nil {
			t.Fatalf("Unregister failed: %v", err)
		}

		_, err := registry.Get("test-provider")
		if err != ErrProviderNotFound {
			t.Errorf("Expected ErrProviderNotFound, got %v", err)
		}
	})
}

func TestRegistryWithCallback(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "registry-callback-test-*")
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)

	var actions []string
	callback := func(provider *Provider, action string) {
		actions = append(actions, action)
	}

	registry, _ := NewRegistry(storage, WithProviderChangeCallback(callback))

	provider := &Provider{
		ID:   "test",
		Name: "Test",
		Type: ProviderTypeCustom,
	}

	registry.Register(provider)
	registry.Enable("test")
	registry.Disable("test")
	registry.AddAPIKey("test", &APIKey{ID: "k1", Key: "sk-test", Enabled: true})
	registry.RemoveAPIKey("test", "k1")
	registry.Update(provider)
	registry.Unregister("test")

	expected := []string{"register", "enable", "disable", "add_api_key", "remove_api_key", "update", "unregister"}
	if len(actions) != len(expected) {
		t.Fatalf("Expected %d actions, got %d", len(expected), len(actions))
	}

	for i, action := range expected {
		if actions[i] != action {
			t.Errorf("Action %d: expected %s, got %s", i, action, actions[i])
		}
	}
}

func TestDefaultHealthChecker(t *testing.T) {
	checker := &DefaultHealthChecker{}

	t.Run("NoCredentials", func(t *testing.T) {
		provider := &Provider{
			ID:      "test",
			BaseURL: "https://api.test.com",
		}

		result := checker.Check(context.Background(), provider)
		if result.Healthy {
			t.Error("Expected unhealthy for provider without credentials")
		}
	})

	t.Run("WithCredentials", func(t *testing.T) {
		provider := &Provider{
			ID:      "test",
			BaseURL: "https://api.test.com",
			APIKeys: []APIKey{
				{Key: "sk-test"},
			},
		}

		result := checker.Check(context.Background(), provider)
		if !result.Healthy {
			t.Error("Expected healthy for provider with credentials")
		}
	})
}

func TestBuiltinProviders(t *testing.T) {
	providers := BuiltinProviders()

	if len(providers) == 0 {
		t.Error("Expected at least one built-in provider")
	}

	// Check required providers exist
	required := []string{"openai", "anthropic", "qwen", "minimax", "openrouter"}
	for _, id := range required {
		found := false
		for _, p := range providers {
			if p.ID == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Required provider %s not found", id)
		}
	}
}

func TestBuiltinModels(t *testing.T) {
	models := BuiltinModels()

	// Check OpenAI models
	openaiModels := models["openai"]
	if len(openaiModels) == 0 {
		t.Error("Expected OpenAI models")
	}

	// Check Anthropic models
	anthropicModels := models["anthropic"]
	if len(anthropicModels) == 0 {
		t.Error("Expected Anthropic models")
	}

	// Verify model capabilities
	for providerID, providerModels := range models {
		for _, model := range providerModels {
			if model.ProviderID != providerID {
				t.Errorf("Model %s has wrong provider ID: %s", model.ID, model.ProviderID)
			}
			if model.ContextWindow <= 0 {
				t.Errorf("Model %s has invalid context window: %d", model.ID, model.ContextWindow)
			}
		}
	}
}

func TestGetHealthCheckURLs(t *testing.T) {
	tests := []struct {
		name     string
		provider *Provider
		method   string
		expected string
	}{
		{
			name:     "openai",
			provider: &Provider{ID: "openai", BaseURL: "https://api.openai.com/v1"},
			method:   "GET",
			expected: "https://api.openai.com/v1/models",
		},
		{
			name:     "anthropic",
			provider: &Provider{ID: "anthropic", BaseURL: "https://api.anthropic.com", APIFormat: APIFormatAnthropic},
			method:   "POST",
			expected: "https://api.anthropic.com/v1/messages",
		},
		{
			name:     "ollama",
			provider: &Provider{ID: "ollama", BaseURL: "http://localhost:11434", APIFormat: APIFormatOllama},
			method:   "GET",
			expected: "http://localhost:11434/api/tags",
		},
		{
			name:     "custom-openai-v1",
			provider: &Provider{ID: "custom", BaseURL: "https://custom.api.com/v1", APIFormat: APIFormatOpenAI},
			method:   "GET",
			expected: "https://custom.api.com/v1/models",
		},
		{
			name:     "custom-no-v1",
			provider: &Provider{ID: "custom", BaseURL: "https://custom.api.com/", APIFormat: APIFormatOpenAI},
			method:   "GET",
			expected: "https://custom.api.com/v1/models",
		},
		{
			name:     "trial-anthropic",
			provider: &Provider{ID: "zimaos-blue-trial", BaseURL: "https://paid.tribiosapi.top", APIFormat: APIFormatAnthropic},
			method:   "POST",
			expected: "https://paid.tribiosapi.top/v1/messages",
		},
		{
			name:     "glm-v4",
			provider: &Provider{ID: "glm", BaseURL: "https://open.bigmodel.cn/api/paas/v4", APIFormat: APIFormatOpenAI},
			method:   "GET",
			expected: "https://open.bigmodel.cn/api/paas/v4/models",
		},
		{
			name:     "nvidia-v1",
			provider: &Provider{ID: "nvidia", BaseURL: "https://integrate.api.nvidia.com/v1", APIFormat: APIFormatOpenAI},
			method:   "POST",
			expected: "https://integrate.api.nvidia.com/v1/chat/completions",
		},
		{
			name:     "nvidia-root",
			provider: &Provider{ID: "nvidia", BaseURL: "https://integrate.api.nvidia.com", APIFormat: APIFormatOpenAI},
			method:   "POST",
			expected: "https://integrate.api.nvidia.com/v1/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method, urls := getHealthCheckMethod(tt.provider)
			if len(urls) == 0 {
				t.Fatal("Expected at least one URL")
			}
			if method != tt.method {
				t.Errorf("Expected method %s, got %s", tt.method, method)
			}
			if urls[0] != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, urls[0])
			}
		})
	}
}

func TestRegistryHealthCheck(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "registry-health-test-*")
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage, WithHealthCheck(100*time.Millisecond, 5*time.Second))

	// Register a provider
	provider := &Provider{
		ID:      "test",
		Name:    "Test",
		Type:    ProviderTypeCustom,
		Enabled: true,
		APIKeys: []APIKey{{Key: "test-key"}},
	}
	registry.Register(provider)

	// Start health check
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	checker := &DefaultHealthChecker{}
	registry.StartHealthCheck(ctx, checker)

	// Wait for health check to run
	time.Sleep(200 * time.Millisecond)

	// Check health status
	result, exists := registry.GetHealth("test")
	if !exists {
		t.Error("Expected health result to exist")
	}
	if result == nil {
		t.Fatal("Health result is nil")
	}
	if !result.Healthy {
		t.Error("Expected provider to be healthy")
	}

	registry.StopHealthCheck()
}

type mockKeyHealthChecker struct {
	results map[string]*HealthCheckResult
	calls   []string
}

func (m *mockKeyHealthChecker) Check(_ context.Context, provider *Provider) *HealthCheckResult {
	keyID := ""
	if len(provider.APIKeys) > 0 {
		keyID = provider.APIKeys[0].ID
	}
	m.calls = append(m.calls, keyID)

	if res, ok := m.results[keyID]; ok {
		copy := *res
		return &copy
	}

	return &HealthCheckResult{
		ProviderID: provider.ID,
		Healthy:    false,
		Error:      "unknown_key",
		CheckedAt:  time.Now(),
	}
}

func TestRegistryRunHealthCheck_TriesAPIKeysUntilHealthy(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "registry-health-keys-test-*")
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)

	provider := &Provider{
		ID:      "test",
		Name:    "Test",
		Type:    ProviderTypeCustom,
		Enabled: true,
		APIKeys: []APIKey{
			{ID: "k1", Key: "sk-bad", Enabled: true},
			{ID: "k2", Key: "sk-good", Enabled: true},
		},
	}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("register provider failed: %v", err)
	}

	checker := &mockKeyHealthChecker{
		results: map[string]*HealthCheckResult{
			"k1": {Healthy: false, Error: "auth_error:401", CheckedAt: time.Now()},
			"k2": {Healthy: true, CheckedAt: time.Now()},
		},
	}

	registry.runHealthCheck(context.Background(), checker)

	if len(checker.calls) != 2 {
		t.Fatalf("expected 2 health checks, got %d (%v)", len(checker.calls), checker.calls)
	}
	if checker.calls[0] != "k1" || checker.calls[1] != "k2" {
		t.Fatalf("expected check order [k1 k2], got %v", checker.calls)
	}

	result, ok := registry.GetHealth("test")
	if !ok || result == nil {
		t.Fatal("missing health result")
	}
	if !result.Healthy {
		t.Fatalf("expected healthy result, got unhealthy: %+v", result)
	}
	if result.KeyID != "k2" {
		t.Fatalf("expected healthy key k2, got %q", result.KeyID)
	}

	updatedProvider, err := registry.Get("test")
	if err != nil {
		t.Fatalf("get provider failed: %v", err)
	}
	if updatedProvider.Status != ProviderStatusActive {
		t.Fatalf("expected provider status active, got %s", updatedProvider.Status)
	}
}
