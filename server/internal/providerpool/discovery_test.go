package providerpool

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestModelDiscovery(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "discovery-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)

	discovery := NewModelDiscovery(registry, storage, time.Hour)

	t.Run("GetBuiltinModels", func(t *testing.T) {
		// Register a built-in provider
		provider := GetBuiltinProvider("openai")
		provider.Enabled = true
		registry.Register(provider)

		models, err := discovery.GetModels("openai")
		if err != nil {
			t.Fatalf("GetModels failed: %v", err)
		}

		if len(models) == 0 {
			t.Error("Expected built-in models for OpenAI")
		}
	})

	t.Run("GetAllModels", func(t *testing.T) {
		// Register another provider
		anthropic := GetBuiltinProvider("anthropic")
		anthropic.Enabled = true
		registry.Register(anthropic)

		models := discovery.GetAllModels()
		if len(models) == 0 {
			t.Error("Expected models from enabled providers")
		}
	})

	t.Run("FindModel", func(t *testing.T) {
		model, provider, err := discovery.FindModel("gpt-4o")
		if err != nil {
			t.Fatalf("FindModel failed: %v", err)
		}

		if model == nil {
			t.Error("Expected to find gpt-4o model")
		}
		if provider == nil {
			t.Error("Expected provider to be returned")
		}
	})

	t.Run("GetModel", func(t *testing.T) {
		model, err := discovery.GetModel("openai", "gpt-4o")
		if err != nil {
			t.Fatalf("GetModel failed: %v", err)
		}

		if model.ID != "gpt-4o" {
			t.Errorf("Expected gpt-4o, got %s", model.ID)
		}
	})

	t.Run("FindModelsByCapability", func(t *testing.T) {
		// Find models with vision capability
		models := discovery.FindModelsByCapability(ModelCapabilities{
			Vision: true,
		})

		if len(models) == 0 {
			t.Error("Expected to find models with vision capability")
		}

		for _, model := range models {
			if !model.Capabilities.Vision {
				t.Errorf("Model %s doesn't have vision capability", model.ID)
			}
		}
	})

	t.Run("FindModelsByThinking", func(t *testing.T) {
		// Find models with thinking capability
		models := discovery.FindModelsByCapability(ModelCapabilities{
			Thinking: true,
		})

		if len(models) == 0 {
			t.Error("Expected to find models with thinking capability")
		}
	})
}

func TestFetchOpenAIModels(t *testing.T) {
	// Create mock server
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"data": [
				{"id": "gpt-4", "object": "model", "created": 1687882411, "owned_by": "openai"},
				{"id": "gpt-4-turbo", "object": "model", "created": 1687882411, "owned_by": "openai"},
				{"id": "gpt-3.5-turbo", "object": "model", "created": 1687882411, "owned_by": "openai"}
			]
		}`))
	}))
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "fetch-test-*")
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)

	// Register provider with mock server URL
	provider := &Provider{
		ID:      "test-openai",
		Name:    "Test OpenAI",
		Type:    ProviderTypeCustom,
		Enabled: true,
		BaseURL: server.URL,
		APIKeys: []APIKey{{ID: "key1", Key: "test-key", Enabled: true}},
	}
	registry.Register(provider)

	discovery := NewModelDiscovery(registry, storage, time.Hour)

	models, err := discovery.FetchModels(context.Background(), "test-openai")
	if err != nil {
		t.Fatalf("FetchModels failed: %v", err)
	}

	if len(models) != 3 {
		t.Errorf("Expected 3 models, got %d", len(models))
	}

	// Verify model IDs
	expectedIDs := map[string]bool{"gpt-4": true, "gpt-4-turbo": true, "gpt-3.5-turbo": true}
	for _, model := range models {
		if !expectedIDs[model.ID] {
			t.Errorf("Unexpected model ID: %s", model.ID)
		}
	}
}

func TestFetchOpenAIModels_ResponsesBaseURLUsesRootModelsEndpoint(t *testing.T) {
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"data": [
				{"id": "gpt-5.3-codex-spark", "object": "model", "created": 1687882411, "owned_by": "relay"}
			]
		}`))
	}))
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "fetch-responses-models-test-*")
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)

	provider := &Provider{
		ID:        "test-responses",
		Name:      "Test Responses",
		Type:      ProviderTypeCustom,
		Enabled:   true,
		BaseURL:   server.URL + "/v1/responses",
		APIFormat: APIFormatResponses,
		APIKeys:   []APIKey{{ID: "key1", Key: "test-key", Enabled: true}},
	}
	registry.Register(provider)

	discovery := NewModelDiscovery(registry, storage, time.Hour)

	models, err := discovery.FetchModels(context.Background(), "test-responses")
	if err != nil {
		t.Fatalf("FetchModels failed: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
	if models[0].ID != "gpt-5.3-codex-spark" {
		t.Fatalf("expected gpt-5.3-codex-spark, got %s", models[0].ID)
	}
}

func TestFetchOpenRouterFreeModels(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "openrouter-free-test-*")
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)

	discovery := NewModelDiscovery(registry, storage, time.Hour)
	provider := &Provider{ID: "openrouter-free"}
	body := []byte(`{
		"data": [
			{"id": "openai/gpt-oss-20b:free", "object": "model", "created": 1687882411, "owned_by": "openrouter", "pricing": {"prompt": "0", "completion": "0"}},
			{"id": "qwen/qwen3-14b", "object": "model", "created": 1687882411, "owned_by": "openrouter", "pricing": {"prompt": "0", "completion": "0", "request": "0"}},
			{"id": "anthropic/claude-3.5-sonnet", "object": "model", "created": 1687882411, "owned_by": "openrouter", "pricing": {"prompt": "0.000003", "completion": "0.000015"}},
			{"id": "meta/llama-3.3-70b-instruct", "object": "model", "created": 1687882411, "owned_by": "openrouter"}
		]
	}`)

	models, err := discovery.parseOpenAIModelsResponse(body, provider)
	if err != nil {
		t.Fatalf("parseOpenAIModelsResponse failed: %v", err)
	}

	if len(models) != 2 {
		t.Fatalf("Expected 2 free models, got %d", len(models))
	}

	expected := map[string]bool{
		"openai/gpt-oss-20b:free": true,
		"qwen/qwen3-14b":          true,
	}
	for _, model := range models {
		if !expected[model.ID] {
			t.Errorf("Unexpected filtered model: %s", model.ID)
		}
	}
}

func TestFetchOllamaModels(t *testing.T) {
	// Create mock server
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"models": [
				{"name": "llama3:latest", "modified_at": "2024-01-01T00:00:00Z", "size": 4000000000},
				{"name": "codellama:7b", "modified_at": "2024-01-01T00:00:00Z", "size": 3000000000},
				{"name": "llava:latest", "modified_at": "2024-01-01T00:00:00Z", "size": 5000000000}
			]
		}`))
	}))
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "ollama-test-*")
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)

	// Register Ollama provider with mock server URL
	provider := &Provider{
		ID:      "ollama",
		Name:    "Ollama",
		Type:    ProviderTypeBuiltin,
		Enabled: true,
		BaseURL: server.URL,
	}
	registry.Register(provider)

	discovery := NewModelDiscovery(registry, storage, time.Hour)

	models, err := discovery.FetchModels(context.Background(), "ollama")
	if err != nil {
		t.Fatalf("FetchModels failed: %v", err)
	}

	if len(models) != 3 {
		t.Errorf("Expected 3 models, got %d", len(models))
	}

	// Check llava has vision capability
	for _, model := range models {
		if model.Name == "llava:latest" {
			if !model.Capabilities.Vision {
				t.Error("llava should have vision capability")
			}
		}
		if model.Name == "codellama:7b" {
			if !model.Capabilities.Completion {
				t.Error("codellama should have completion capability")
			}
		}
	}
}

func TestModelCaching(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "cache-test-*")
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)

	// Register provider
	provider := GetBuiltinProvider("openai")
	provider.Enabled = true
	registry.Register(provider)

	// Use short cache TTL for testing
	discovery := NewModelDiscovery(registry, storage, 100*time.Millisecond)

	// First call should use built-in models
	models1, _ := discovery.GetModels("openai")

	// Second call should use cache
	models2, _ := discovery.GetModels("openai")

	if len(models1) != len(models2) {
		t.Error("Cached models should match original")
	}

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Should still work (falls back to storage or built-in)
	models3, err := discovery.GetModels("openai")
	if err != nil {
		t.Fatalf("GetModels after cache expiry failed: %v", err)
	}

	if len(models3) == 0 {
		t.Error("Should still return models after cache expiry")
	}
}

func TestGetFilteredModels_AllowlistConfiguredEmptyPersists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "allowlist-persist-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)

	provider := &Provider{
		ID:                  "custom-provider",
		Name:                "Custom Provider",
		Type:                ProviderTypeCustom,
		Enabled:             true,
		Status:              ProviderStatusActive,
		AllowlistConfigured: true,
		AllowedModels:       []string{},
	}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := storage.SaveModels("custom-provider", []*Model{{
		ID:          "gpt-4o",
		ProviderID:  "custom-provider",
		Name:        "gpt-4o",
		DisplayName: "GPT-4o",
		Enabled:     true,
		Capabilities: ModelCapabilities{
			Chat: true,
		},
	}}); err != nil {
		t.Fatalf("SaveModels failed: %v", err)
	}

	// Simulate restart: reload registry from persisted storage.
	reloadedRegistry, _ := NewRegistry(storage)
	discovery := NewModelDiscovery(reloadedRegistry, storage, time.Hour)

	filtered, err := discovery.GetFilteredModels("custom-provider")
	if err != nil {
		t.Fatalf("GetFilteredModels failed: %v", err)
	}
	if len(filtered) != 0 {
		t.Fatalf("Expected 0 models when allowlist configured empty, got %d", len(filtered))
	}
}

func TestGetFilteredModels_DefaultSortsByPreference(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "model-pref-sort-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)

	provider := &Provider{
		ID:      "custom-provider",
		Name:    "Custom Provider",
		Type:    ProviderTypeCustom,
		Enabled: true,
		Status:  ProviderStatusActive,
	}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := storage.SaveModels(provider.ID, []*Model{
		{ID: "gpt-4o-mini", ProviderID: provider.ID, Name: "gpt-4o-mini", Enabled: true},
		{ID: "gpt-5.3-codex-spark", ProviderID: provider.ID, Name: "gpt-5.3-codex-spark", Enabled: true},
		{ID: "gpt-5.3-codex", ProviderID: provider.ID, Name: "gpt-5.3-codex", Enabled: true},
	}); err != nil {
		t.Fatalf("SaveModels failed: %v", err)
	}

	discovery := NewModelDiscovery(registry, storage, time.Hour)
	filtered, err := discovery.GetFilteredModels(provider.ID)
	if err != nil {
		t.Fatalf("GetFilteredModels failed: %v", err)
	}
	if len(filtered) != 3 {
		t.Fatalf("Expected 3 models, got %d", len(filtered))
	}
	if filtered[0].ID != "gpt-5.3-codex" || filtered[1].ID != "gpt-5.3-codex-spark" || filtered[2].ID != "gpt-4o-mini" {
		t.Fatalf("unexpected model order: [%s %s %s]", filtered[0].ID, filtered[1].ID, filtered[2].ID)
	}
}

func TestGetFilteredModels_PreservesAllowlistOrder(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "allowlist-order-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)

	provider := &Provider{
		ID:                  "custom-provider",
		Name:                "Custom Provider",
		Type:                ProviderTypeCustom,
		Enabled:             true,
		Status:              ProviderStatusActive,
		AllowlistConfigured: true,
		AllowedModels:       []string{"gpt-4o-mini", "gpt-5.3-codex"},
	}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := storage.SaveModels(provider.ID, []*Model{
		{ID: "gpt-5.3-codex", ProviderID: provider.ID, Name: "gpt-5.3-codex", Enabled: true},
		{ID: "gpt-4o-mini", ProviderID: provider.ID, Name: "gpt-4o-mini", Enabled: true},
	}); err != nil {
		t.Fatalf("SaveModels failed: %v", err)
	}

	discovery := NewModelDiscovery(registry, storage, time.Hour)
	filtered, err := discovery.GetFilteredModels(provider.ID)
	if err != nil {
		t.Fatalf("GetFilteredModels failed: %v", err)
	}
	if len(filtered) != 2 {
		t.Fatalf("Expected 2 models, got %d", len(filtered))
	}
	if filtered[0].ID != "gpt-4o-mini" || filtered[1].ID != "gpt-5.3-codex" {
		t.Fatalf("unexpected allowlist order: [%s %s]", filtered[0].ID, filtered[1].ID)
	}
}

func TestFormatModelName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"gpt-4", "4"},
		{"gpt-4-turbo", "4 Turbo"},
		{"claude-3-opus", "3 Opus"},
		{"llama3:latest", "Llama3:latest"},
		{"models/gemini-pro", "Gemini Pro"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := formatModelName(tt.input)
			if result != tt.expected {
				t.Errorf("formatModelName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestInferCapabilities(t *testing.T) {
	tests := []struct {
		modelName string
		vision    bool
		thinking  bool
		funcCall  bool
	}{
		{"gpt-4o", true, false, true},
		{"gpt-4-vision-preview", true, false, true},
		{"o1-preview", false, true, false},
		{"claude-3-opus", true, false, true},
		{"deepseek-reasoner", false, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.modelName, func(t *testing.T) {
			model := &Model{
				Name: tt.modelName,
				Capabilities: ModelCapabilities{
					Chat:      true,
					Streaming: true,
				},
			}
			inferCapabilities(model)

			if model.Capabilities.Vision != tt.vision {
				t.Errorf("Vision: got %v, want %v", model.Capabilities.Vision, tt.vision)
			}
			if model.Capabilities.Thinking != tt.thinking {
				t.Errorf("Thinking: got %v, want %v", model.Capabilities.Thinking, tt.thinking)
			}
			if model.Capabilities.FunctionCall != tt.funcCall {
				t.Errorf("FunctionCall: got %v, want %v", model.Capabilities.FunctionCall, tt.funcCall)
			}
		})
	}
}

func TestMatchesCapabilities(t *testing.T) {
	model := ModelCapabilities{
		Chat:         true,
		Vision:       true,
		FunctionCall: true,
		Streaming:    true,
	}

	tests := []struct {
		name     string
		required ModelCapabilities
		matches  bool
	}{
		{"empty", ModelCapabilities{}, true},
		{"chat only", ModelCapabilities{Chat: true}, true},
		{"vision", ModelCapabilities{Vision: true}, true},
		{"thinking", ModelCapabilities{Thinking: true}, false},
		{"multiple", ModelCapabilities{Chat: true, Vision: true}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesCapabilities(model, tt.required)
			if result != tt.matches {
				t.Errorf("matchesCapabilities() = %v, want %v", result, tt.matches)
			}
		})
	}
}
