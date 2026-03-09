package providerpool

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

func setupRouterTest(t *testing.T) (*Router, func()) {
	tmpDir, err := os.MkdirTemp("", "router-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)
	discovery := NewModelDiscovery(registry, storage, time.Hour)
	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	// Register test providers
	providers := []*Provider{
		{
			ID:       "provider-high",
			Name:     "High Priority",
			Type:     ProviderTypeCustom,
			Enabled:  true,
			Status:   ProviderStatusActive,
			Priority: 100,
			APIKeys:  []APIKey{{ID: "k1", Key: "key1", Enabled: true}},
		},
		{
			ID:       "provider-medium",
			Name:     "Medium Priority",
			Type:     ProviderTypeCustom,
			Enabled:  true,
			Status:   ProviderStatusActive,
			Priority: 50,
			APIKeys:  []APIKey{{ID: "k2", Key: "key2", Enabled: true}},
		},
		{
			ID:       "provider-low",
			Name:     "Low Priority",
			Type:     ProviderTypeCustom,
			Enabled:  true,
			Status:   ProviderStatusActive,
			Priority: 10,
			APIKeys:  []APIKey{{ID: "k3", Key: "key3", Enabled: true}},
		},
	}

	for _, p := range providers {
		registry.Register(p)
	}

	// Save models for each provider
	models := []*Model{
		{
			ID:          "test-model",
			ProviderID:  "provider-high",
			Name:        "test-model",
			DisplayName: "Test Model",
			Enabled:     true,
			Capabilities: ModelCapabilities{
				Chat:      true,
				Streaming: true,
			},
			InputPrice:  10.0,
			OutputPrice: 20.0,
		},
	}
	storage.SaveModels("provider-high", models)

	models[0].ProviderID = "provider-medium"
	models[0].InputPrice = 5.0
	models[0].OutputPrice = 10.0
	storage.SaveModels("provider-medium", models)

	models[0].ProviderID = "provider-low"
	models[0].InputPrice = 1.0
	models[0].OutputPrice = 2.0
	storage.SaveModels("provider-low", models)

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return router, cleanup
}

func TestRouterPriorityStrategy(t *testing.T) {
	router, cleanup := setupRouterTest(t)
	defer cleanup()

	result, err := router.Route(&RouteRequest{
		ModelID:  "test-model",
		Strategy: RoutingStrategyPriority,
	})

	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	if result.Provider.ID != "provider-high" {
		t.Errorf("Expected provider-high, got %s", result.Provider.ID)
	}

	if len(result.Fallbacks) != 2 {
		t.Errorf("Expected 2 fallbacks, got %d", len(result.Fallbacks))
	}
}

func TestRouterCostStrategy(t *testing.T) {
	router, cleanup := setupRouterTest(t)
	defer cleanup()

	result, err := router.Route(&RouteRequest{
		ModelID:  "test-model",
		Strategy: RoutingStrategyCost,
	})

	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	// provider-low has the lowest cost
	if result.Provider.ID != "provider-low" {
		t.Errorf("Expected provider-low (cheapest), got %s", result.Provider.ID)
	}
}

func TestRouterLatencyStrategy(t *testing.T) {
	router, cleanup := setupRouterTest(t)
	defer cleanup()

	// Set latencies
	router.UpdateLatency("provider-high", 500*time.Millisecond)
	router.UpdateLatency("provider-medium", 100*time.Millisecond)
	router.UpdateLatency("provider-low", 300*time.Millisecond)

	result, err := router.Route(&RouteRequest{
		ModelID:  "test-model",
		Strategy: RoutingStrategyLatency,
	})

	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	// provider-medium has the lowest latency
	if result.Provider.ID != "provider-medium" {
		t.Errorf("Expected provider-medium (fastest), got %s", result.Provider.ID)
	}
}

func TestRouterRoundRobinStrategy(t *testing.T) {
	router, cleanup := setupRouterTest(t)
	defer cleanup()

	// Make multiple requests and track which providers are selected
	selected := make(map[string]int)

	for i := 0; i < 9; i++ {
		result, err := router.Route(&RouteRequest{
			ModelID:  "test-model",
			Strategy: RoutingStrategyRoundRobin,
		})

		if err != nil {
			t.Fatalf("Route failed: %v", err)
		}

		selected[result.Provider.ID]++
	}

	// Each provider should be selected 3 times
	for _, count := range selected {
		if count != 3 {
			t.Errorf("Expected each provider to be selected 3 times, got distribution: %v", selected)
			break
		}
	}
}

func TestRouterExcludeProviders(t *testing.T) {
	router, cleanup := setupRouterTest(t)
	defer cleanup()

	result, err := router.Route(&RouteRequest{
		ModelID:  "test-model",
		Strategy: RoutingStrategyPriority,
		Exclude:  []string{"provider-high"},
	})

	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	if result.Provider.ID == "provider-high" {
		t.Error("Excluded provider should not be selected")
	}

	if result.Provider.ID != "provider-medium" {
		t.Errorf("Expected provider-medium, got %s", result.Provider.ID)
	}
}

func TestRouterNoAvailableProvider(t *testing.T) {
	router, cleanup := setupRouterTest(t)
	defer cleanup()

	_, err := router.Route(&RouteRequest{
		ModelID:  "non-existent-model",
		Strategy: RoutingStrategyPriority,
	})

	if err != ErrNoAvailableProvider {
		t.Errorf("Expected ErrNoAvailableProvider, got %v", err)
	}
}

func TestRouterWithFallback(t *testing.T) {
	router, cleanup := setupRouterTest(t)
	defer cleanup()

	callCount := 0
	failFirst := true

	err := router.RouteWithFallback(context.Background(), &RouteRequest{
		ModelID:  "test-model",
		Strategy: RoutingStrategyPriority,
	}, func(result *RouteResult) error {
		callCount++
		if failFirst && result.Provider.ID == "provider-high" {
			failFirst = false
			return errors.New("simulated failure")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("RouteWithFallback failed: %v", err)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 calls (1 failure + 1 success), got %d", callCount)
	}
}

func TestRouterWithFallback_RetriesSameProviderOnceOnTransientError(t *testing.T) {
	router, cleanup := setupRouterTest(t)
	defer cleanup()

	var triedProviders []string
	providerHighAttempts := 0

	err := router.RouteWithFallback(context.Background(), &RouteRequest{
		ModelID:  "test-model",
		Strategy: RoutingStrategyPriority,
	}, func(result *RouteResult) error {
		triedProviders = append(triedProviders, result.Provider.ID)
		if result.Provider.ID == "provider-high" {
			providerHighAttempts++
			if providerHighAttempts == 1 {
				return errors.New("upstream 503")
			}
			return nil
		}
		return errors.New("should not switch providers")
	})

	if err != nil {
		t.Fatalf("RouteWithFallback failed: %v", err)
	}
	if providerHighAttempts != 2 {
		t.Fatalf("Expected 2 attempts on provider-high, got %d", providerHighAttempts)
	}
	if len(triedProviders) != 2 || triedProviders[0] != "provider-high" || triedProviders[1] != "provider-high" {
		t.Fatalf("Expected transient retry to stay on provider-high, got %v", triedProviders)
	}
}

func TestRouterWithFallback_RetriesNextAPIKeyOnAuthError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "router-multikey-auth-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)
	discovery := NewModelDiscovery(registry, storage, time.Hour)
	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	provider := &Provider{
		ID:       "provider-multi-key",
		Name:     "Provider Multi Key",
		Type:     ProviderTypeCustom,
		Enabled:  true,
		Status:   ProviderStatusActive,
		Priority: 100,
		APIKeys: []APIKey{
			{ID: "k-bad", Key: "sk-bad", Enabled: true},
			{ID: "k-good", Key: "sk-good", Enabled: true},
		},
	}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("Failed to register provider: %v", err)
	}
	if err := storage.SaveModels(provider.ID, []*Model{{
		ID:          "test-model",
		ProviderID:  provider.ID,
		Name:        "test-model",
		DisplayName: "Test Model",
		Enabled:     true,
		Capabilities: ModelCapabilities{
			Chat: true,
		},
	}}); err != nil {
		t.Fatalf("Failed to save model: %v", err)
	}
	router.RebuildCandidates()

	var triedKeys []string
	err = router.RouteWithFallback(context.Background(), &RouteRequest{
		ModelID:  "test-model",
		Strategy: RoutingStrategyPriority,
	}, func(result *RouteResult) error {
		if result.APIKey == nil {
			return errors.New("missing api key")
		}
		triedKeys = append(triedKeys, result.APIKey.ID)
		if result.APIKey.ID == "k-bad" {
			return errors.New("provider provider-multi-key auth error (401): invalid api key")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("RouteWithFallback failed: %v", err)
	}
	if len(triedKeys) != 2 {
		t.Fatalf("Expected 2 key attempts, got %d: %v", len(triedKeys), triedKeys)
	}
	if triedKeys[0] != "k-bad" || triedKeys[1] != "k-good" {
		t.Errorf("Expected key order [k-bad k-good], got %v", triedKeys)
	}
}

func TestRouterWithFallback_RetriesNextAPIKeyOnRateLimit(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "router-multikey-ratelimit-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)
	discovery := NewModelDiscovery(registry, storage, time.Hour)
	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	provider := &Provider{
		ID:       "provider-multi-key",
		Name:     "Provider Multi Key",
		Type:     ProviderTypeCustom,
		Enabled:  true,
		Status:   ProviderStatusActive,
		Priority: 100,
		APIKeys: []APIKey{
			{ID: "k-rl", Key: "sk-ratelimited", Enabled: true},
			{ID: "k-ok", Key: "sk-ok", Enabled: true},
		},
	}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("Failed to register provider: %v", err)
	}
	if err := storage.SaveModels(provider.ID, []*Model{{
		ID:          "test-model",
		ProviderID:  provider.ID,
		Name:        "test-model",
		DisplayName: "Test Model",
		Enabled:     true,
		Capabilities: ModelCapabilities{
			Chat: true,
		},
	}}); err != nil {
		t.Fatalf("Failed to save model: %v", err)
	}
	router.RebuildCandidates()

	var triedKeys []string
	err = router.RouteWithFallback(context.Background(), &RouteRequest{
		ModelID:  "test-model",
		Strategy: RoutingStrategyPriority,
	}, func(result *RouteResult) error {
		if result.APIKey == nil {
			return errors.New("missing api key")
		}
		triedKeys = append(triedKeys, result.APIKey.ID)
		if result.APIKey.ID == "k-rl" {
			return errors.New("provider provider-multi-key throttled (429)")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("RouteWithFallback failed: %v", err)
	}
	if len(triedKeys) != 2 {
		t.Fatalf("Expected 2 key attempts, got %d: %v", len(triedKeys), triedKeys)
	}
	if triedKeys[0] != "k-rl" || triedKeys[1] != "k-ok" {
		t.Errorf("Expected key order [k-rl k-ok], got %v", triedKeys)
	}
}

func TestRouterWithFallback_DoesNotRetryNextAPIKeyOnNonAuthError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "router-multikey-nonauth-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)
	discovery := NewModelDiscovery(registry, storage, time.Hour)
	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	provider := &Provider{
		ID:       "provider-multi-key",
		Name:     "Provider Multi Key",
		Type:     ProviderTypeCustom,
		Enabled:  true,
		Status:   ProviderStatusActive,
		Priority: 100,
		APIKeys: []APIKey{
			{ID: "k-first", Key: "sk-first", Enabled: true},
			{ID: "k-second", Key: "sk-second", Enabled: true},
		},
	}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("Failed to register provider: %v", err)
	}
	if err := storage.SaveModels(provider.ID, []*Model{{
		ID:          "test-model",
		ProviderID:  provider.ID,
		Name:        "test-model",
		DisplayName: "Test Model",
		Enabled:     true,
		Capabilities: ModelCapabilities{
			Chat: true,
		},
	}}); err != nil {
		t.Fatalf("Failed to save model: %v", err)
	}
	router.RebuildCandidates()

	callCount := 0
	err = router.RouteWithFallback(context.Background(), &RouteRequest{
		ModelID:  "test-model",
		Strategy: RoutingStrategyPriority,
	}, func(result *RouteResult) error {
		callCount++
		return errors.New("upstream 500: internal server error")
	})

	if err == nil {
		t.Fatal("Expected RouteWithFallback to fail")
	}
	if callCount != 1 {
		t.Errorf("Expected only 1 attempt for non-auth error, got %d", callCount)
	}
}

func TestRouterPriorityStrategy_PrefersAnthropicForClaudeModels(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "router-claude-affinity-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)
	discovery := NewModelDiscovery(registry, storage, time.Hour)
	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	openAIProvider := &Provider{
		ID:        "provider-openai-relay",
		Name:      "OpenAI Relay",
		Type:      ProviderTypeCustom,
		Enabled:   true,
		Status:    ProviderStatusActive,
		Priority:  100,
		APIFormat: APIFormatOpenAI,
		APIKeys:   []APIKey{{ID: "k-openai", Key: "sk-openai", Enabled: true}},
	}
	anthropicProvider := &Provider{
		ID:        "provider-anthropic",
		Name:      "Anthropic",
		Type:      ProviderTypeCustom,
		Enabled:   true,
		Status:    ProviderStatusActive,
		Priority:  90,
		APIFormat: APIFormatAnthropic,
		APIKeys:   []APIKey{{ID: "k-anthropic", Key: "sk-anthropic", Enabled: true}},
	}
	if err := registry.Register(openAIProvider); err != nil {
		t.Fatalf("register openai provider failed: %v", err)
	}
	if err := registry.Register(anthropicProvider); err != nil {
		t.Fatalf("register anthropic provider failed: %v", err)
	}

	models := []*Model{{
		ID:           "claude-sonnet-4-5",
		Name:         "claude-sonnet-4-5",
		DisplayName:  "Claude Sonnet 4.5",
		Enabled:      true,
		Capabilities: ModelCapabilities{Chat: true},
	}}
	if err := storage.SaveModels(openAIProvider.ID, []*Model{{
		ID: models[0].ID, ProviderID: openAIProvider.ID, Name: models[0].Name, DisplayName: models[0].DisplayName, Enabled: true,
		Capabilities: models[0].Capabilities,
	}}); err != nil {
		t.Fatalf("save openai models failed: %v", err)
	}
	if err := storage.SaveModels(anthropicProvider.ID, []*Model{{
		ID: models[0].ID, ProviderID: anthropicProvider.ID, Name: models[0].Name, DisplayName: models[0].DisplayName, Enabled: true,
		Capabilities: models[0].Capabilities,
	}}); err != nil {
		t.Fatalf("save anthropic models failed: %v", err)
	}
	router.RebuildCandidates()

	result, err := router.Route(&RouteRequest{ModelID: "claude-sonnet-4-5", Strategy: RoutingStrategyPriority})
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}
	if result.Provider.ID != anthropicProvider.ID {
		t.Fatalf("Expected claude model to prefer anthropic provider, got %s", result.Provider.ID)
	}
}

func TestRouterLatencyTracking(t *testing.T) {
	router, cleanup := setupRouterTest(t)
	defer cleanup()

	// Initial latency
	router.UpdateLatency("test-provider", 100*time.Millisecond)
	lat := router.GetLatency("test-provider")
	if lat != 100*time.Millisecond {
		t.Errorf("Expected 100ms, got %v", lat)
	}

	// Update with EMA
	router.UpdateLatency("test-provider", 200*time.Millisecond)
	lat = router.GetLatency("test-provider")

	// EMA: 100*0.7 + 200*0.3 = 70 + 60 = 130ms
	expected := 130 * time.Millisecond
	if lat != expected {
		t.Errorf("Expected %v (EMA), got %v", expected, lat)
	}
}

func TestRouterCapabilityFiltering(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "router-cap-test-*")
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)
	discovery := NewModelDiscovery(registry, storage, time.Hour)
	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	// Register provider with vision model
	provider := &Provider{
		ID:       "vision-provider",
		Name:     "Vision Provider",
		Type:     ProviderTypeCustom,
		Enabled:  true,
		Status:   ProviderStatusActive,
		Priority: 100,
		APIKeys:  []APIKey{{ID: "k1", Key: "key1", Enabled: true}},
	}
	registry.Register(provider)

	// Save model with vision capability
	models := []*Model{
		{
			ID:         "vision-model",
			ProviderID: "vision-provider",
			Name:       "vision-model",
			Enabled:    true,
			Capabilities: ModelCapabilities{
				Chat:   true,
				Vision: true,
			},
		},
	}
	storage.SaveModels("vision-provider", models)

	// Route with vision requirement
	result, err := router.Route(&RouteRequest{
		ModelID:  "vision-model",
		Strategy: RoutingStrategyPriority,
		RequireCap: &ModelCapabilities{
			Vision: true,
		},
	})

	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	if !result.Model.Capabilities.Vision {
		t.Error("Selected model should have vision capability")
	}
}

func TestFindBestProvider(t *testing.T) {
	router, cleanup := setupRouterTest(t)
	defer cleanup()

	provider, model, err := router.FindBestProvider("test-model")
	if err != nil {
		t.Fatalf("FindBestProvider failed: %v", err)
	}

	if provider == nil {
		t.Error("Provider should not be nil")
	}

	if model == nil {
		t.Error("Model should not be nil")
	}

	if provider.ID != "provider-high" {
		t.Errorf("Expected provider-high, got %s", provider.ID)
	}
}

func TestListAvailableModels(t *testing.T) {
	router, cleanup := setupRouterTest(t)
	defer cleanup()

	models := router.ListAvailableModels()

	// Should have at least 3 models (one from each provider)
	if len(models) < 3 {
		t.Errorf("Expected at least 3 models, got %d", len(models))
	}
}

// setupRouterWithLocationTest creates a router with cloud and local providers for testing routing modes
func setupRouterWithLocationTest(t *testing.T) (*Router, func()) {
	tmpDir, err := os.MkdirTemp("", "router-location-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)
	discovery := NewModelDiscovery(registry, storage, time.Hour)
	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	// Register cloud providers
	cloudProviders := []*Provider{
		{
			ID:       "openai",
			Name:     "OpenAI",
			Type:     ProviderTypeBuiltin,
			Location: ProviderLocationCloud,
			Enabled:  true,
			Status:   ProviderStatusActive,
			Priority: 100,
			APIKeys:  []APIKey{{ID: "k1", Key: "sk-openai", Enabled: true}},
		},
		{
			ID:       "anthropic",
			Name:     "Anthropic",
			Type:     ProviderTypeBuiltin,
			Location: ProviderLocationCloud,
			Enabled:  true,
			Status:   ProviderStatusActive,
			Priority: 90,
			APIKeys:  []APIKey{{ID: "k2", Key: "sk-anthropic", Enabled: true}},
		},
	}

	// Register local providers
	localProviders := []*Provider{
		{
			ID:       "ollama",
			Name:     "Ollama",
			Type:     ProviderTypeCustom,
			Location: ProviderLocationLocal,
			Enabled:  true,
			Status:   ProviderStatusActive,
			Priority: 80,
			APIKeys:  []APIKey{{ID: "k3", Key: "local-key", Enabled: true}},
		},
		{
			ID:       "lmstudio",
			Name:     "LM Studio",
			Type:     ProviderTypeCustom,
			Location: ProviderLocationLocal,
			Enabled:  true,
			Status:   ProviderStatusActive,
			Priority: 70,
			APIKeys:  []APIKey{{ID: "k4", Key: "local-key2", Enabled: true}},
		},
	}

	for _, p := range cloudProviders {
		registry.Register(p)
	}
	for _, p := range localProviders {
		registry.Register(p)
	}

	// Save models for each provider - all have the same model "gpt-4"
	cloudModel := &Model{
		ID:          "gpt-4",
		Name:        "gpt-4",
		DisplayName: "GPT-4",
		Enabled:     true,
		Capabilities: ModelCapabilities{
			Chat:      true,
			Streaming: true,
		},
	}

	// OpenAI
	cloudModel.ProviderID = "openai"
	cloudModel.InputPrice = 30.0
	cloudModel.OutputPrice = 60.0
	storage.SaveModels("openai", []*Model{cloudModel})

	// Anthropic
	cloudModel.ProviderID = "anthropic"
	cloudModel.InputPrice = 15.0
	cloudModel.OutputPrice = 75.0
	storage.SaveModels("anthropic", []*Model{cloudModel})

	// Ollama (local, free)
	localModel := &Model{
		ID:          "gpt-4",
		ProviderID:  "ollama",
		Name:        "gpt-4",
		DisplayName: "GPT-4 (Local)",
		Enabled:     true,
		InputPrice:  0,
		OutputPrice: 0,
		Capabilities: ModelCapabilities{
			Chat:      true,
			Streaming: true,
		},
	}
	storage.SaveModels("ollama", []*Model{localModel})

	// LM Studio (local, free)
	localModel.ProviderID = "lmstudio"
	storage.SaveModels("lmstudio", []*Model{localModel})

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return router, cleanup
}

func TestRouterRoutingModeCloud(t *testing.T) {
	router, cleanup := setupRouterWithLocationTest(t)
	defer cleanup()

	// Route with cloud mode - should only select cloud providers
	result, err := router.Route(&RouteRequest{
		ModelID:  "gpt-4",
		Strategy: RoutingStrategyPriority,
		Mode:     RoutingModeCloud,
	})

	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	// Should select OpenAI (highest priority cloud provider)
	if result.Provider.ID != "openai" {
		t.Errorf("Expected openai (highest priority cloud), got %s", result.Provider.ID)
	}

	// Verify provider is cloud
	if result.Provider.Location != ProviderLocationCloud {
		t.Errorf("Expected cloud provider, got location: %s", result.Provider.Location)
	}

	// Fallbacks should only contain cloud providers
	for _, fallback := range result.Fallbacks {
		if fallback.Provider.Location != ProviderLocationCloud {
			t.Errorf("Fallback should be cloud provider, got %s with location %s",
				fallback.Provider.ID, fallback.Provider.Location)
		}
	}

	// Should have 1 fallback (anthropic)
	if len(result.Fallbacks) != 1 {
		t.Errorf("Expected 1 cloud fallback, got %d", len(result.Fallbacks))
	}
}

func TestRouterRoutingModeLocal(t *testing.T) {
	router, cleanup := setupRouterWithLocationTest(t)
	defer cleanup()

	// Route with local mode - should only select local providers
	result, err := router.Route(&RouteRequest{
		ModelID:  "gpt-4",
		Strategy: RoutingStrategyPriority,
		Mode:     RoutingModeLocal,
	})

	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	// Should select Ollama (highest priority local provider)
	if result.Provider.ID != "ollama" {
		t.Errorf("Expected ollama (highest priority local), got %s", result.Provider.ID)
	}

	// Verify provider is local
	if result.Provider.Location != ProviderLocationLocal {
		t.Errorf("Expected local provider, got location: %s", result.Provider.Location)
	}

	// Fallbacks should only contain local providers
	for _, fallback := range result.Fallbacks {
		if fallback.Provider.Location != ProviderLocationLocal {
			t.Errorf("Fallback should be local provider, got %s with location %s",
				fallback.Provider.ID, fallback.Provider.Location)
		}
	}

	// Should have 1 fallback (lmstudio)
	if len(result.Fallbacks) != 1 {
		t.Errorf("Expected 1 local fallback, got %d", len(result.Fallbacks))
	}
}

func TestRouterRoutingModeAuto(t *testing.T) {
	router, cleanup := setupRouterWithLocationTest(t)
	defer cleanup()

	// Route with auto mode - should select from all providers based on priority
	result, err := router.Route(&RouteRequest{
		ModelID:  "gpt-4",
		Strategy: RoutingStrategyPriority,
		Mode:     RoutingModeAuto,
	})

	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	// Should select OpenAI (highest priority overall)
	if result.Provider.ID != "openai" {
		t.Errorf("Expected openai (highest priority), got %s", result.Provider.ID)
	}

	// Should have 3 fallbacks (anthropic, ollama, lmstudio)
	if len(result.Fallbacks) != 3 {
		t.Errorf("Expected 3 fallbacks, got %d", len(result.Fallbacks))
	}
}

func TestRouterRoutingModeEmptyDefaultsToAuto(t *testing.T) {
	router, cleanup := setupRouterWithLocationTest(t)
	defer cleanup()

	// Route without specifying mode - should behave like auto
	result, err := router.Route(&RouteRequest{
		ModelID:  "gpt-4",
		Strategy: RoutingStrategyPriority,
		// Mode not specified
	})

	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	// Should select OpenAI (highest priority overall)
	if result.Provider.ID != "openai" {
		t.Errorf("Expected openai (highest priority), got %s", result.Provider.ID)
	}

	// Should have 3 fallbacks (all providers)
	if len(result.Fallbacks) != 3 {
		t.Errorf("Expected 3 fallbacks, got %d", len(result.Fallbacks))
	}
}

func TestRouterRoutingModeCloudNoCloudProviders(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "router-no-cloud-test-*")
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)
	discovery := NewModelDiscovery(registry, storage, time.Hour)
	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	// Register only local provider
	localProvider := &Provider{
		ID:       "ollama",
		Name:     "Ollama",
		Type:     ProviderTypeCustom,
		Location: ProviderLocationLocal,
		Enabled:  true,
		Status:   ProviderStatusActive,
		Priority: 100,
		APIKeys:  []APIKey{{ID: "k1", Key: "local-key", Enabled: true}},
	}
	registry.Register(localProvider)

	storage.SaveModels("ollama", []*Model{{
		ID:         "gpt-4",
		ProviderID: "ollama",
		Name:       "gpt-4",
		Enabled:    true,
		Capabilities: ModelCapabilities{
			Chat: true,
		},
	}})

	// Route with cloud mode - should fail because no cloud providers
	_, err := router.Route(&RouteRequest{
		ModelID:  "gpt-4",
		Strategy: RoutingStrategyPriority,
		Mode:     RoutingModeCloud,
	})

	if err != ErrNoAvailableProvider {
		t.Errorf("Expected ErrNoAvailableProvider when no cloud providers, got %v", err)
	}
}

func TestRouterRoutingModeLocalNoLocalProviders(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "router-no-local-test-*")
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)
	discovery := NewModelDiscovery(registry, storage, time.Hour)
	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	// Register only cloud provider
	cloudProvider := &Provider{
		ID:       "openai",
		Name:     "OpenAI",
		Type:     ProviderTypeBuiltin,
		Location: ProviderLocationCloud,
		Enabled:  true,
		Status:   ProviderStatusActive,
		Priority: 100,
		APIKeys:  []APIKey{{ID: "k1", Key: "sk-openai", Enabled: true}},
	}
	registry.Register(cloudProvider)

	storage.SaveModels("openai", []*Model{{
		ID:         "gpt-4",
		ProviderID: "openai",
		Name:       "gpt-4",
		Enabled:    true,
		Capabilities: ModelCapabilities{
			Chat: true,
		},
	}})

	// Route with local mode - should fail because no local providers
	_, err := router.Route(&RouteRequest{
		ModelID:  "gpt-4",
		Strategy: RoutingStrategyPriority,
		Mode:     RoutingModeLocal,
	})

	if err != ErrNoAvailableProvider {
		t.Errorf("Expected ErrNoAvailableProvider when no local providers, got %v", err)
	}
}

func TestRouterRoutingModeWithFallback(t *testing.T) {
	router, cleanup := setupRouterWithLocationTest(t)
	defer cleanup()

	// Test cloud mode with fallback - should only fallback to cloud providers
	callCount := 0
	failedProviders := make(map[string]bool)

	err := router.RouteWithFallback(context.Background(), &RouteRequest{
		ModelID:  "gpt-4",
		Strategy: RoutingStrategyPriority,
		Mode:     RoutingModeCloud,
	}, func(result *RouteResult) error {
		callCount++
		// Fail the first provider (openai)
		if result.Provider.ID == "openai" {
			failedProviders[result.Provider.ID] = true
			return errors.New("simulated failure")
		}
		// Verify we're still using cloud provider
		if result.Provider.Location != ProviderLocationCloud {
			t.Errorf("Fallback should be cloud provider, got %s", result.Provider.Location)
		}
		return nil
	})

	if err != nil {
		t.Fatalf("RouteWithFallback failed: %v", err)
	}

	// Should have called 2 providers (openai failed, anthropic succeeded)
	if callCount != 2 {
		t.Errorf("Expected 2 calls, got %d", callCount)
	}

	if !failedProviders["openai"] {
		t.Error("OpenAI should have been tried and failed")
	}
}

func TestRouterRoutingModeCostStrategyWithCloudMode(t *testing.T) {
	router, cleanup := setupRouterWithLocationTest(t)
	defer cleanup()

	// Route with cloud mode and cost strategy
	result, err := router.Route(&RouteRequest{
		ModelID:  "gpt-4",
		Strategy: RoutingStrategyCost,
		Mode:     RoutingModeCloud,
	})

	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	// Should select Anthropic (cheaper cloud provider: 15+75=90 vs OpenAI 30+60=90)
	// Actually both are same total, but Anthropic has lower input price
	// The cost strategy sorts by total cost, so either could be selected
	// Let's verify it's a cloud provider
	if result.Provider.Location != ProviderLocationCloud {
		t.Errorf("Expected cloud provider, got location: %s", result.Provider.Location)
	}

	// Verify no local providers in fallbacks
	for _, fallback := range result.Fallbacks {
		if fallback.Provider.Location != ProviderLocationCloud {
			t.Errorf("Fallback should be cloud provider, got %s", fallback.Provider.Location)
		}
	}
}

func TestRouterRoutingModeCostStrategyWithLocalMode(t *testing.T) {
	router, cleanup := setupRouterWithLocationTest(t)
	defer cleanup()

	// Route with local mode and cost strategy
	result, err := router.Route(&RouteRequest{
		ModelID:  "gpt-4",
		Strategy: RoutingStrategyCost,
		Mode:     RoutingModeLocal,
	})

	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	// Should select a local provider (both are free, so either is fine)
	if result.Provider.Location != ProviderLocationLocal {
		t.Errorf("Expected local provider, got location: %s", result.Provider.Location)
	}

	// Local providers are free, so cost should be 0
	if result.Model.InputPrice != 0 || result.Model.OutputPrice != 0 {
		t.Errorf("Expected free local model, got input: %f, output: %f",
			result.Model.InputPrice, result.Model.OutputPrice)
	}
}

func TestRouterCooldown(t *testing.T) {
	router, cleanup := setupRouterTest(t)
	defer cleanup()

	// Configure short cooldown for testing
	router.SetCooldownConfig(&CooldownConfig{
		FailureThreshold:   2,
		InitialCooldown:    100 * time.Millisecond,
		MaxCooldown:        500 * time.Millisecond,
		CooldownMultiplier: 2.0,
		ResetAfter:         time.Minute,
	})

	providerID := "provider-high"

	// Initially not in cooldown
	if router.IsInCooldown(providerID) {
		t.Error("Provider should not be in cooldown initially")
	}

	// Record failures below threshold
	router.RecordFailure(providerID, errors.New("test error 1"))
	if router.IsInCooldown(providerID) {
		t.Error("Provider should not be in cooldown after 1 failure")
	}

	// Record failure at threshold - should trigger cooldown
	router.RecordFailure(providerID, errors.New("test error 2"))
	if !router.IsInCooldown(providerID) {
		t.Error("Provider should be in cooldown after 2 failures")
	}

	// Wait for cooldown to expire
	time.Sleep(150 * time.Millisecond)
	if router.IsInCooldown(providerID) {
		t.Error("Provider should not be in cooldown after cooldown expires")
	}
}

func TestRouterSingleProvider_NoCooldownPenalty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "router-single-provider-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)
	discovery := NewModelDiscovery(registry, storage, time.Hour)
	router := NewRouter(registry, discovery, RoutingStrategyPriority)
	router.SetCooldownConfig(&CooldownConfig{
		FailureThreshold:   1,
		InitialCooldown:    time.Hour,
		MaxCooldown:        time.Hour,
		CooldownMultiplier: 1.0,
		ResetAfter:         time.Hour,
	})

	provider := &Provider{
		ID:      "provider-only",
		Name:    "Only Provider",
		Type:    ProviderTypeCustom,
		Enabled: true,
		Status:  ProviderStatusActive,
		APIKeys: []APIKey{{ID: "k1", Key: "key1", Enabled: true}},
	}
	registry.Register(provider)
	storage.SaveModels(provider.ID, []*Model{{
		ID:          "test-model",
		ProviderID:  provider.ID,
		Name:        "test-model",
		DisplayName: "Test Model",
		Enabled:     true,
	}})
	router.RebuildCandidates()

	// In single-provider mode, failures should not trigger cooldown self-isolation.
	router.RecordFailure(provider.ID, errors.New("upstream 500"))
	if router.IsInCooldown(provider.ID) {
		t.Fatal("single provider should not enter cooldown")
	}

	result, routeErr := router.Route(&RouteRequest{ModelID: "test-model"})
	if routeErr != nil {
		t.Fatalf("Route failed: %v", routeErr)
	}
	if result.Provider.ID != provider.ID {
		t.Fatalf("expected provider %s, got %s", provider.ID, result.Provider.ID)
	}
}

func TestRouterSingleProvider_ErrorStatusStillRoutes(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "router-single-provider-error-status-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)
	discovery := NewModelDiscovery(registry, storage, time.Hour)
	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	provider := &Provider{
		ID:      "provider-only-error",
		Name:    "Only Provider Error",
		Type:    ProviderTypeCustom,
		Enabled: true,
		Status:  ProviderStatusError,
		APIKeys: []APIKey{{ID: "k1", Key: "key1", Enabled: true}},
	}
	registry.Register(provider)
	storage.SaveModels(provider.ID, []*Model{{
		ID:          "test-model",
		ProviderID:  provider.ID,
		Name:        "test-model",
		DisplayName: "Test Model",
		Enabled:     true,
	}})
	router.RebuildCandidates()

	result, routeErr := router.Route(&RouteRequest{ModelID: "test-model"})
	if routeErr != nil {
		t.Fatalf("Route failed: %v", routeErr)
	}
	if result.Provider.ID != provider.ID {
		t.Fatalf("expected provider %s, got %s", provider.ID, result.Provider.ID)
	}
}

func TestRouterSingleProvider_BlindFallbackBypassesErrorStatus(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "router-single-provider-error-blind-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)
	discovery := NewModelDiscovery(registry, storage, time.Hour)
	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	provider := &Provider{
		ID:      "provider-only-error",
		Name:    "Only Provider Error",
		Type:    ProviderTypeCustom,
		Enabled: true,
		Status:  ProviderStatusError,
		APIKeys: []APIKey{{ID: "k1", Key: "key1", Enabled: true}},
	}
	registry.Register(provider)
	router.RebuildCandidates()

	var tried []string
	err = router.RouteWithFallback(context.Background(), &RouteRequest{
		ModelID: "nonexistent-model",
	}, func(result *RouteResult) error {
		tried = append(tried, result.Provider.ID)
		return nil
	})

	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if len(tried) != 1 || tried[0] != provider.ID {
		t.Fatalf("Expected blind fallback to try only %s, got %v", provider.ID, tried)
	}
}

func TestRouterPreferredProviderPinned(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "router-preferred-provider-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)
	discovery := NewModelDiscovery(registry, storage, time.Hour)
	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	highPriority := &Provider{
		ID: "provider-high", Name: "High Priority", Type: ProviderTypeCustom,
		Enabled: true, Status: ProviderStatusActive, Priority: 100,
		Location: ProviderLocationCloud,
		APIKeys:  []APIKey{{ID: "k1", Key: "key1", Enabled: true}},
	}
	lowPriority := &Provider{
		ID: "provider-low", Name: "Low Priority", Type: ProviderTypeCustom,
		Enabled: true, Status: ProviderStatusActive, Priority: 10,
		Location: ProviderLocationCloud,
		APIKeys:  []APIKey{{ID: "k2", Key: "key2", Enabled: true}},
	}

	if err := registry.Register(highPriority); err != nil {
		t.Fatalf("Register highPriority failed: %v", err)
	}
	if err := registry.Register(lowPriority); err != nil {
		t.Fatalf("Register lowPriority failed: %v", err)
	}

	model := &Model{
		ID: "gpt-5.3-codex-spark", ProviderID: "provider-high", Name: "gpt-5.3-codex-spark",
		Enabled: true, Capabilities: ModelCapabilities{Chat: true, FunctionCall: true},
	}
	if err := storage.SaveModels("provider-high", []*Model{model}); err != nil {
		t.Fatalf("SaveModels provider-high failed: %v", err)
	}
	model2 := *model
	model2.ProviderID = "provider-low"
	if err := storage.SaveModels("provider-low", []*Model{&model2}); err != nil {
		t.Fatalf("SaveModels provider-low failed: %v", err)
	}
	router.RebuildCandidates()

	result, err := router.Route(&RouteRequest{
		ModelID:             "gpt-5.3-codex-spark",
		PreferredProviderID: "provider-low",
	})
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}
	if result.Provider.ID != "provider-low" {
		t.Fatalf("expected preferred provider-low, got %s", result.Provider.ID)
	}
	if len(result.Fallbacks) == 0 || result.Fallbacks[0].Provider.ID != "provider-high" {
		t.Fatalf("expected provider-high as fallback, got %+v", result.Fallbacks)
	}
}

func TestRouterDynamicProviderRegisterUnregisterUpdatesRouting(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "router-dynamic-provider-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)
	discovery := NewModelDiscovery(registry, storage, time.Hour)
	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	base := &Provider{
		ID: "provider-base", Name: "Base", Type: ProviderTypeCustom,
		Enabled: true, Status: ProviderStatusActive, Priority: 50,
		Location: ProviderLocationCloud,
		APIKeys:  []APIKey{{ID: "kb", Key: "key-base", Enabled: true}},
	}
	if err := registry.Register(base); err != nil {
		t.Fatalf("Register base failed: %v", err)
	}
	if err := storage.SaveModels(base.ID, []*Model{{
		ID: "gpt-5.3-codex-spark", ProviderID: base.ID, Name: "gpt-5.3-codex-spark",
		Enabled: true, Capabilities: ModelCapabilities{Chat: true, FunctionCall: true},
	}}); err != nil {
		t.Fatalf("SaveModels base failed: %v", err)
	}
	router.RebuildCandidates()

	req := &RouteRequest{ModelID: "gpt-5.3-codex-spark", RequireCap: &ModelCapabilities{FunctionCall: true}}
	result, err := router.Route(req)
	if err != nil {
		t.Fatalf("initial Route failed: %v", err)
	}
	if result.Provider.ID != base.ID {
		t.Fatalf("expected base provider, got %s", result.Provider.ID)
	}

	dynamic := &Provider{
		ID: "provider-dynamic", Name: "Dynamic", Type: ProviderTypeCustom,
		Enabled: true, Status: ProviderStatusActive, Priority: 120,
		Location: ProviderLocationCloud,
		APIKeys:  []APIKey{{ID: "kd", Key: "key-dynamic", Enabled: true}},
	}
	if err := storage.SaveModels(dynamic.ID, []*Model{{
		ID: "gpt-5.3-codex-spark", ProviderID: dynamic.ID, Name: "gpt-5.3-codex-spark",
		Enabled: true, Capabilities: ModelCapabilities{Chat: true, FunctionCall: true},
	}}); err != nil {
		t.Fatalf("SaveModels dynamic failed: %v", err)
	}
	if err := registry.Register(dynamic); err != nil {
		t.Fatalf("Register dynamic failed: %v", err)
	}
	router.RebuildCandidates()

	result, err = router.Route(req)
	if err != nil {
		t.Fatalf("Route after dynamic register failed: %v", err)
	}
	if result.Provider.ID != dynamic.ID {
		t.Fatalf("expected dynamic provider after register, got %s", result.Provider.ID)
	}

	if err := registry.Unregister(dynamic.ID); err != nil {
		t.Fatalf("Unregister dynamic failed: %v", err)
	}
	router.RebuildCandidates()

	result, err = router.Route(req)
	if err != nil {
		t.Fatalf("Route after dynamic unregister failed: %v", err)
	}
	if result.Provider.ID != base.ID {
		t.Fatalf("expected base provider after dynamic unregister, got %s", result.Provider.ID)
	}
}

func TestRouterDynamicAPIKeyChangesAffectRouting(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "router-dynamic-apikey-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)
	discovery := NewModelDiscovery(registry, storage, time.Hour)
	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	provider := &Provider{
		ID: "provider-keyed", Name: "Keyed", Type: ProviderTypeCustom,
		Enabled: true, Status: ProviderStatusActive, Priority: 100,
		Location: ProviderLocationCloud,
		APIKeys:  []APIKey{{ID: "k1", Key: "key-1", Enabled: true}},
	}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("Register provider failed: %v", err)
	}
	if err := storage.SaveModels(provider.ID, []*Model{{
		ID: "gpt-5.3-codex-spark", ProviderID: provider.ID, Name: "gpt-5.3-codex-spark",
		Enabled: true, Capabilities: ModelCapabilities{Chat: true, FunctionCall: true},
	}}); err != nil {
		t.Fatalf("SaveModels failed: %v", err)
	}
	router.RebuildCandidates()

	req := &RouteRequest{ModelID: "gpt-5.3-codex-spark", RequireCap: &ModelCapabilities{FunctionCall: true}}
	result, err := router.Route(req)
	if err != nil {
		t.Fatalf("initial Route failed: %v", err)
	}
	if result.Provider.ID != provider.ID {
		t.Fatalf("expected provider-keyed initially, got %s", result.Provider.ID)
	}

	if err := registry.RemoveAPIKey(provider.ID, "k1"); err != nil {
		t.Fatalf("RemoveAPIKey failed: %v", err)
	}
	router.RebuildCandidates()
	if _, err := router.Route(req); !errors.Is(err, ErrNoAvailableProvider) {
		t.Fatalf("expected ErrNoAvailableProvider after key removal, got %v", err)
	}

	if err := registry.AddAPIKey(provider.ID, &APIKey{
		ID: "k2", Key: "key-2", Enabled: true,
	}); err != nil {
		t.Fatalf("AddAPIKey failed: %v", err)
	}
	router.RebuildCandidates()
	result, err = router.Route(req)
	if err != nil {
		t.Fatalf("Route after key add failed: %v", err)
	}
	if result.Provider.ID != provider.ID {
		t.Fatalf("expected provider-keyed after key add, got %s", result.Provider.ID)
	}
}

func TestRouterCooldownReset(t *testing.T) {
	router, cleanup := setupRouterTest(t)
	defer cleanup()

	router.SetCooldownConfig(&CooldownConfig{
		FailureThreshold:   2,
		InitialCooldown:    100 * time.Millisecond,
		MaxCooldown:        500 * time.Millisecond,
		CooldownMultiplier: 2.0,
		ResetAfter:         time.Minute,
	})

	providerID := "provider-high"

	// Trigger cooldown
	router.RecordFailure(providerID, errors.New("error 1"))
	router.RecordFailure(providerID, errors.New("error 2"))
	if !router.IsInCooldown(providerID) {
		t.Error("Provider should be in cooldown")
	}

	// Record success should reset
	router.RecordSuccess(providerID)
	if router.IsInCooldown(providerID) {
		t.Error("Provider should not be in cooldown after success")
	}

	// Verify failure count was reset
	entry := router.GetCooldownEntry(providerID)
	if entry != nil && entry.FailureCount != 0 {
		t.Errorf("Failure count should be 0 after success, got %d", entry.FailureCount)
	}
}

func TestRouterCooldownSkipsProvider(t *testing.T) {
	router, cleanup := setupRouterTest(t)
	defer cleanup()

	router.SetCooldownConfig(&CooldownConfig{
		FailureThreshold:   1,
		InitialCooldown:    time.Hour, // Long cooldown
		MaxCooldown:        time.Hour,
		CooldownMultiplier: 1.0,
		ResetAfter:         time.Hour,
	})

	// Put high priority provider in cooldown
	router.RecordFailure("provider-high", errors.New("error"))

	// Route should skip the cooldown provider
	result, err := router.Route(&RouteRequest{
		ModelID:  "test-model",
		Strategy: RoutingStrategyPriority,
	})

	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	// Should select medium priority (next available)
	if result.Provider.ID == "provider-high" {
		t.Error("Should not select provider in cooldown")
	}
}

func TestRouterTransientCooldown(t *testing.T) {
	router, cleanup := setupRouterTest(t)
	defer cleanup()

	// Configure: persistent errors get long cooldown, transient (502/503) get short
	router.SetCooldownConfig(&CooldownConfig{
		FailureThreshold:          2,
		InitialCooldown:           time.Hour, // persistent: very long
		MaxCooldown:               time.Hour,
		CooldownMultiplier:        1.0,
		ResetAfter:                time.Hour,
		TransientFailureThreshold: 3,                     // need more transient failures
		TransientInitialCooldown:  50 * time.Millisecond, // but much shorter cooldown
		TransientMaxCooldown:      200 * time.Millisecond,
	})

	providerID := "provider-high"

	// 2 transient (502) failures should NOT trigger cooldown (threshold is 3)
	router.RecordFailure(providerID, fmt.Errorf("upstream 502: bad gateway"))
	router.RecordFailure(providerID, fmt.Errorf("upstream 502: bad gateway"))
	if router.IsInCooldown(providerID) {
		t.Error("Provider should not be in cooldown after 2 transient failures (threshold=3)")
	}

	// 3rd transient failure triggers short cooldown
	router.RecordFailure(providerID, fmt.Errorf("upstream 503: service unavailable"))
	if !router.IsInCooldown(providerID) {
		t.Error("Provider should be in cooldown after 3 transient failures")
	}

	// Short cooldown expires quickly
	deadline := time.Now().Add(500 * time.Millisecond)
	for router.IsInCooldown(providerID) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if router.IsInCooldown(providerID) {
		t.Error("Transient cooldown should have expired after 80ms (initial=50ms)")
	}

	// Verify: persistent error with same provider triggers long cooldown at threshold=2
	// (FailureCount is already 3 from above, well past persistent threshold of 2)
	router.RecordSuccess(providerID) // reset
	router.RecordFailure(providerID, errors.New("auth error"))
	router.RecordFailure(providerID, errors.New("auth error"))
	if !router.IsInCooldown(providerID) {
		t.Error("Provider should be in long cooldown after 2 persistent failures")
	}

	// Long cooldown should NOT expire quickly
	time.Sleep(100 * time.Millisecond)
	if !router.IsInCooldown(providerID) {
		t.Error("Persistent cooldown should still be active after 100ms (initial=1h)")
	}
}

func TestRouterTransientCooldown_WrappedOverloaded500(t *testing.T) {
	router, cleanup := setupRouterTest(t)
	defer cleanup()

	router.SetCooldownConfig(&CooldownConfig{
		FailureThreshold:          2,
		InitialCooldown:           time.Hour,
		MaxCooldown:               time.Hour,
		CooldownMultiplier:        1.0,
		ResetAfter:                time.Hour,
		TransientFailureThreshold: 3,
		TransientInitialCooldown:  50 * time.Millisecond,
		TransientMaxCooldown:      200 * time.Millisecond,
	})

	providerID := "provider-high"
	wrappedOverloadErr := errors.New(`upstream 500: {"error":{"type":"overloaded_error","message":"构建请求失败"},"type":"error"}`)

	router.RecordFailure(providerID, wrappedOverloadErr)
	router.RecordFailure(providerID, wrappedOverloadErr)
	if router.IsInCooldown(providerID) {
		t.Fatal("provider should not be in cooldown after 2 wrapped overloaded 500 failures")
	}

	router.RecordFailure(providerID, wrappedOverloadErr)
	if !router.IsInCooldown(providerID) {
		t.Fatal("provider should enter transient cooldown after 3 wrapped overloaded 500 failures")
	}

	time.Sleep(120 * time.Millisecond)
	if router.IsInCooldown(providerID) {
		t.Fatal("wrapped overloaded transient cooldown should expire quickly")
	}
}

func TestRouterTransientCooldownSuccessResets(t *testing.T) {
	router, cleanup := setupRouterTest(t)
	defer cleanup()

	router.SetCooldownConfig(&CooldownConfig{
		FailureThreshold:          2,
		InitialCooldown:           time.Hour,
		MaxCooldown:               time.Hour,
		CooldownMultiplier:        1.0,
		ResetAfter:                time.Hour,
		TransientFailureThreshold: 3,
		TransientInitialCooldown:  50 * time.Millisecond,
		TransientMaxCooldown:      200 * time.Millisecond,
	})

	providerID := "provider-high"

	// Record 2 transient failures
	router.RecordFailure(providerID, fmt.Errorf("upstream 502: bad gateway"))
	router.RecordFailure(providerID, fmt.Errorf("upstream 503: service unavailable"))

	// Success resets both counters
	router.RecordSuccess(providerID)

	entry := router.GetCooldownEntry(providerID)
	if entry != nil && entry.TransientFailureCount != 0 {
		t.Errorf("TransientFailureCount should be 0 after success, got %d", entry.TransientFailureCount)
	}
}

func TestRouterFailoverTracking(t *testing.T) {
	router, cleanup := setupRouterTest(t)
	defer cleanup()

	var capturedResult *FailoverResult
	router.SetFailoverCallback(func(result *FailoverResult) {
		capturedResult = result
	})

	failFirst := true
	err := router.RouteWithFallback(context.Background(), &RouteRequest{
		ModelID:  "test-model",
		Strategy: RoutingStrategyPriority,
	}, func(result *RouteResult) error {
		if failFirst && result.Provider.ID == "provider-high" {
			failFirst = false
			return errors.New("provider provider-high auth error (401): invalid api key")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("RouteWithFallback failed: %v", err)
	}

	// Verify failover was tracked
	if capturedResult == nil {
		t.Fatal("Failover callback was not called")
	}

	if capturedResult.TotalAttempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", capturedResult.TotalAttempts)
	}

	if len(capturedResult.FailedAttempts) != 1 {
		t.Errorf("Expected 1 failed attempt, got %d", len(capturedResult.FailedAttempts))
	}

	if capturedResult.FailedAttempts[0].Reason != FailoverReasonAuthError {
		t.Errorf("Expected auth error reason, got %s", capturedResult.FailedAttempts[0].Reason)
	}

	if capturedResult.SuccessProvider == "" {
		t.Error("Success provider should be set")
	}
}

func TestFailoverResultSummary(t *testing.T) {
	result := &FailoverResult{
		SuccessProvider: "provider-b",
		SuccessModel:    "claude-opus-4-6",
		FailedAttempts: []*FailoverRecord{
			{
				ProviderID:     "provider-a",
				Reason:         FailoverReasonRateLimit,
				Latency:        250 * time.Millisecond,
				NextProviderID: "provider-b",
			},
		},
	}

	got := result.Summary()
	want := "provider-a[rate_limit,250ms]->provider-b | provider-b[success:claude-opus-4-6]"
	if got != want {
		t.Fatalf("Summary() = %q, want %q", got, want)
	}
}

func TestRouterEmptyModelID(t *testing.T) {
	router, cleanup := setupRouterTest(t)
	defer cleanup()

	// Route with empty model ID should select first available
	result, err := router.Route(&RouteRequest{
		ModelID:  "",
		Strategy: RoutingStrategyPriority,
	})

	if err != nil {
		t.Fatalf("Route with empty model ID failed: %v", err)
	}

	if result.Provider == nil {
		t.Error("Provider should not be nil")
	}

	if result.Model == nil {
		t.Error("Model should not be nil")
	}
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		err      error
		expected FailoverReason
	}{
		{errors.New("request timeout"), FailoverReasonTimeout},
		{errors.New("context deadline exceeded"), FailoverReasonTimeout},
		{errors.New("rate limit exceeded"), FailoverReasonRateLimit},
		{errors.New("429 too many requests"), FailoverReasonRateLimit},
		{errors.New("401 unauthorized"), FailoverReasonAuthError},
		{errors.New("invalid api key"), FailoverReasonAuthError},
		{errors.New("model not found"), FailoverReasonModelNotFound},
		{errors.New("404 resource does not exist"), FailoverReasonModelNotFound},
		{errors.New("internal server error"), FailoverReasonAPIError},
		{nil, FailoverReasonUnknown},
	}

	for _, tt := range tests {
		result := classifyError(tt.err)
		if result != tt.expected {
			errStr := "<nil>"
			if tt.err != nil {
				errStr = tt.err.Error()
			}
			t.Errorf("classifyError(%q) = %s, want %s", errStr, result, tt.expected)
		}
	}
}

// TestCustomProviderWithProvIDFormat tests routing with custom provider IDs like "prov_xxx"
// This simulates the actual user configuration scenario
func TestCustomProviderWithProvIDFormat(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "router-custom-prov-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)
	discovery := NewModelDiscovery(registry, storage, time.Hour)
	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	// Register custom provider with prov_xxx ID format (like user's actual config)
	customProvider := &Provider{
		ID:        "prov_dedd1b4d0489003f",
		Name:      "xxxxxxx",
		Type:      ProviderTypeCustom,
		Location:  ProviderLocationCloud,
		Enabled:   true,
		Status:    ProviderStatusActive,
		BaseURL:   "https://yyy.xxxxxxx.com/",
		APIFormat: APIFormatOpenAI,
		Priority:  90,
		APIKeys: []APIKey{
			{
				ID:      "key_f4821c438a434c2d",
				Key:     "sk-test-key-12345",
				KeyHash: "sk-test...5",
				Enabled: true,
			},
		},
	}
	if err := registry.Register(customProvider); err != nil {
		t.Fatalf("Failed to register provider: %v", err)
	}

	// Save models for the custom provider
	models := []*Model{
		{
			ID:          "claude-haiku-4-5",
			ProviderID:  "prov_dedd1b4d0489003f",
			Name:        "claude-haiku-4-5",
			DisplayName: "Haiku 4 5",
			Enabled:     true,
			Capabilities: ModelCapabilities{
				Chat:         true,
				Streaming:    true,
				FunctionCall: true,
			},
		},
		{
			ID:          "claude-sonnet-4-5",
			ProviderID:  "prov_dedd1b4d0489003f",
			Name:        "claude-sonnet-4-5",
			DisplayName: "Sonnet 4 5",
			Enabled:     true,
			Capabilities: ModelCapabilities{
				Chat:         true,
				Streaming:    true,
				FunctionCall: true,
			},
		},
	}
	storage.SaveModels("prov_dedd1b4d0489003f", models)

	// Test 1: Route to specific model
	result, err := router.Route(&RouteRequest{
		ModelID:  "claude-haiku-4-5",
		Strategy: RoutingStrategyPriority,
	})
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	if result.Provider.ID != "prov_dedd1b4d0489003f" {
		t.Errorf("Expected provider prov_dedd1b4d0489003f, got %s", result.Provider.ID)
	}

	if result.Provider.APIFormat != APIFormatOpenAI {
		t.Errorf("Expected APIFormat openai, got %s", result.Provider.APIFormat)
	}

	if result.Model.ID != "claude-haiku-4-5" {
		t.Errorf("Expected model claude-haiku-4-5, got %s", result.Model.ID)
	}

	// Test 2: Verify API key is accessible
	if result.APIKey == nil {
		t.Error("APIKey should not be nil")
	} else if result.APIKey.Key != "sk-test-key-12345" {
		t.Errorf("Expected API key sk-test-key-12345, got %s", result.APIKey.Key)
	}

	// Test 3: Route with empty model ID should select first available
	result2, err := router.Route(&RouteRequest{
		ModelID:  "",
		Strategy: RoutingStrategyPriority,
	})
	if err != nil {
		t.Fatalf("Route with empty model failed: %v", err)
	}

	if result2.Provider.ID != "prov_dedd1b4d0489003f" {
		t.Errorf("Expected provider prov_dedd1b4d0489003f, got %s", result2.Provider.ID)
	}

	if result2.Model == nil {
		t.Error("Model should not be nil when routing with empty model ID")
	}

	// Test 4: RouteWithFallback should work with custom provider
	callCount := 0
	err = router.RouteWithFallback(context.Background(), &RouteRequest{
		ModelID:  "claude-sonnet-4-5",
		Strategy: RoutingStrategyPriority,
	}, func(result *RouteResult) error {
		callCount++
		// Verify we got the right provider and model
		if result.Provider.ID != "prov_dedd1b4d0489003f" {
			t.Errorf("Expected provider prov_dedd1b4d0489003f in callback, got %s", result.Provider.ID)
		}
		if result.Model.ID != "claude-sonnet-4-5" {
			t.Errorf("Expected model claude-sonnet-4-5 in callback, got %s", result.Model.ID)
		}
		return nil
	})

	if err != nil {
		t.Fatalf("RouteWithFallback failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}
}

// TestRegistryGetCustomProvider tests that Registry.Get works with custom provider IDs
func TestRegistryGetCustomProvider(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "registry-custom-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)

	// Register custom provider
	customProvider := &Provider{
		ID:        "prov_abc123def456",
		Name:      "Custom Provider",
		Type:      ProviderTypeCustom,
		Enabled:   true,
		Status:    ProviderStatusActive,
		BaseURL:   "https://api.example.com",
		APIFormat: APIFormatOpenAI,
		APIKeys: []APIKey{
			{ID: "k1", Key: "test-api-key", Enabled: true},
		},
	}
	registry.Register(customProvider)

	// Test Get
	retrieved, err := registry.Get("prov_abc123def456")
	if err != nil {
		t.Fatalf("Registry.Get failed: %v", err)
	}

	if retrieved.ID != "prov_abc123def456" {
		t.Errorf("Expected ID prov_abc123def456, got %s", retrieved.ID)
	}

	if retrieved.APIFormat != APIFormatOpenAI {
		t.Errorf("Expected APIFormat openai, got %s", retrieved.APIFormat)
	}

	// Verify API key is loaded
	if len(retrieved.APIKeys) == 0 {
		t.Error("APIKeys should not be empty")
	} else if retrieved.APIKeys[0].Key != "test-api-key" {
		t.Errorf("Expected API key test-api-key, got %s", retrieved.APIKeys[0].Key)
	}

	// Test GetAPIKey
	apiKey, err := registry.GetAPIKey("prov_abc123def456")
	if err != nil {
		t.Fatalf("GetAPIKey failed: %v", err)
	}

	if apiKey.Key != "test-api-key" {
		t.Errorf("Expected API key test-api-key, got %s", apiKey.Key)
	}
}

// TestRouterBlindFallback_ModelNotInSnapshot verifies that when the requested model
// is not in any provider's model list, RouteWithFallback still tries healthy providers
// by forwarding the original model name (blind fallback).
func TestRouterBlindFallback_ModelNotInSnapshot(t *testing.T) {
	router, cleanup := setupRouterTest(t)
	defer cleanup()

	var triedProviders []string

	err := router.RouteWithFallback(context.Background(), &RouteRequest{
		ModelID:  "unknown-model-xyz",
		Strategy: RoutingStrategyPriority,
	}, func(result *RouteResult) error {
		triedProviders = append(triedProviders, result.Provider.ID)
		// Simulate: second provider succeeds
		if result.Provider.ID == "provider-medium" {
			return nil
		}
		return errors.New("model not found")
	})

	if err != nil {
		t.Fatalf("RouteWithFallback should succeed via blind fallback, got: %v", err)
	}

	// Should have tried providers in priority order: high, medium
	if len(triedProviders) != 2 {
		t.Fatalf("Expected 2 attempts, got %d: %v", len(triedProviders), triedProviders)
	}
	if triedProviders[0] != "provider-high" {
		t.Errorf("First attempt should be provider-high, got %s", triedProviders[0])
	}
	if triedProviders[1] != "provider-medium" {
		t.Errorf("Second attempt should be provider-medium, got %s", triedProviders[1])
	}
}

// TestRouterBlindFallback_AllSameModelFail verifies that when the model exists in the
// snapshot but all same-model providers fail, blind fallback tries remaining providers.
func TestRouterBlindFallback_AllSameModelFail(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "router-blind-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)
	discovery := NewModelDiscovery(registry, storage, time.Hour)
	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	// Provider A has the model, Provider B does NOT have the model
	providerA := &Provider{
		ID: "provider-a", Name: "Provider A", Type: ProviderTypeCustom,
		Enabled: true, Status: ProviderStatusActive, Priority: 100,
		Location: ProviderLocationCloud,
		APIKeys:  []APIKey{{ID: "k1", Key: "key-a", Enabled: true}},
	}
	providerB := &Provider{
		ID: "provider-b", Name: "Provider B", Type: ProviderTypeCustom,
		Enabled: true, Status: ProviderStatusActive, Priority: 50,
		Location: ProviderLocationCloud,
		APIKeys:  []APIKey{{ID: "k2", Key: "key-b", Enabled: true}},
	}
	registry.Register(providerA)
	registry.Register(providerB)

	// Only provider-a has "special-model"
	storage.SaveModels("provider-a", []*Model{{
		ID: "special-model", ProviderID: "provider-a", Name: "special-model",
		Enabled: true, Capabilities: ModelCapabilities{Chat: true},
	}})
	// provider-b has a different model
	storage.SaveModels("provider-b", []*Model{{
		ID: "other-model", ProviderID: "provider-b", Name: "other-model",
		Enabled: true, Capabilities: ModelCapabilities{Chat: true},
	}})
	router.RebuildCandidates()

	var triedProviders []string

	err = router.RouteWithFallback(context.Background(), &RouteRequest{
		ModelID:  "special-model",
		Strategy: RoutingStrategyPriority,
	}, func(result *RouteResult) error {
		triedProviders = append(triedProviders, result.Provider.ID)
		if result.Provider.ID == "provider-b" {
			return nil // provider-b succeeds (upstream supports the model even though we don't know)
		}
		return errors.New("upstream error")
	})

	if err != nil {
		t.Fatalf("Expected blind fallback to succeed, got: %v", err)
	}

	// provider-a tried first (same-model match), then provider-b (blind fallback)
	if len(triedProviders) != 2 {
		t.Fatalf("Expected 2 attempts, got %d: %v", len(triedProviders), triedProviders)
	}
	if triedProviders[0] != "provider-a" {
		t.Errorf("First attempt should be provider-a, got %s", triedProviders[0])
	}
	if triedProviders[1] != "provider-b" {
		t.Errorf("Second attempt should be provider-b (blind fallback), got %s", triedProviders[1])
	}
}

// TestRouterBlindFallback_RespectsExplicitExclude verifies that blind fallback
// does not resurrect providers the caller explicitly excluded.
func TestRouterBlindFallback_RespectsExplicitExclude(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "router-blind-exclude-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)
	discovery := NewModelDiscovery(registry, storage, time.Hour)
	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	providerA := &Provider{
		ID: "provider-a", Name: "Provider A", Type: ProviderTypeCustom,
		Enabled: true, Status: ProviderStatusActive, Priority: 100,
		Location: ProviderLocationCloud,
		APIKeys:  []APIKey{{ID: "k1", Key: "key-a", Enabled: true}},
	}
	providerB := &Provider{
		ID: "provider-b", Name: "Provider B", Type: ProviderTypeCustom,
		Enabled: true, Status: ProviderStatusActive, Priority: 50,
		Location: ProviderLocationCloud,
		APIKeys:  []APIKey{{ID: "k2", Key: "key-b", Enabled: true}},
	}
	registry.Register(providerA)
	registry.Register(providerB)

	storage.SaveModels("provider-a", []*Model{{
		ID: "special-model", ProviderID: "provider-a", Name: "special-model",
		Enabled: true, Capabilities: ModelCapabilities{Chat: true},
	}})
	storage.SaveModels("provider-b", []*Model{{
		ID: "other-model", ProviderID: "provider-b", Name: "other-model",
		Enabled: true, Capabilities: ModelCapabilities{Chat: true},
	}})
	router.RebuildCandidates()

	var triedProviders []string
	err = router.RouteWithFallback(context.Background(), &RouteRequest{
		ModelID:  "special-model",
		Exclude:  []string{"provider-a"},
		Strategy: RoutingStrategyPriority,
	}, func(result *RouteResult) error {
		triedProviders = append(triedProviders, result.Provider.ID)
		return nil
	})

	if err != nil {
		t.Fatalf("Expected blind fallback to succeed on non-excluded provider, got: %v", err)
	}
	if len(triedProviders) != 1 {
		t.Fatalf("expected exactly 1 attempt, got %d (%v)", len(triedProviders), triedProviders)
	}
	if triedProviders[0] != "provider-b" {
		t.Fatalf("expected blind fallback to skip excluded provider-a and use provider-b, got %v", triedProviders)
	}
}

// TestRouterBlindFallback_RespectsRoutingMode verifies that blind fallback
// only tries providers matching the requested routing mode (cloud/local).
func TestRouterBlindFallback_RespectsRoutingMode(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "router-blind-mode-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)
	discovery := NewModelDiscovery(registry, storage, time.Hour)
	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	cloudProvider := &Provider{
		ID: "cloud-prov", Name: "Cloud", Type: ProviderTypeCustom,
		Enabled: true, Status: ProviderStatusActive, Priority: 100,
		Location: ProviderLocationCloud,
		APIKeys:  []APIKey{{ID: "k1", Key: "cloud-key", Enabled: true}},
	}
	localProvider := &Provider{
		ID: "local-prov", Name: "Local", Type: ProviderTypeCustom,
		Enabled: true, Status: ProviderStatusActive, Priority: 90,
		Location: ProviderLocationLocal,
		APIKeys:  []APIKey{{ID: "k2", Key: "local-key", Enabled: true}},
	}
	registry.Register(cloudProvider)
	registry.Register(localProvider)

	// Neither provider has the model in their list
	router.RebuildCandidates()

	// Request with cloud mode — should only try cloud provider
	var triedProviders []string
	err = router.RouteWithFallback(context.Background(), &RouteRequest{
		ModelID: "new-model",
		Mode:    RoutingModeCloud,
	}, func(result *RouteResult) error {
		triedProviders = append(triedProviders, result.Provider.ID)
		return nil // succeed on first try
	})

	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if len(triedProviders) != 1 || triedProviders[0] != "cloud-prov" {
		t.Errorf("Expected only cloud-prov, got: %v", triedProviders)
	}

	// Request with local mode — should only try local provider
	triedProviders = nil
	err = router.RouteWithFallback(context.Background(), &RouteRequest{
		ModelID: "new-model",
		Mode:    RoutingModeLocal,
	}, func(result *RouteResult) error {
		triedProviders = append(triedProviders, result.Provider.ID)
		return nil
	})

	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if len(triedProviders) != 1 || triedProviders[0] != "local-prov" {
		t.Errorf("Expected only local-prov, got: %v", triedProviders)
	}
}

// TestRouterBlindFallback_AllFail verifies that when all providers fail
// (including blind fallback), the error is properly returned.
func TestRouterBlindFallback_AllFail(t *testing.T) {
	router, cleanup := setupRouterTest(t)
	defer cleanup()

	err := router.RouteWithFallback(context.Background(), &RouteRequest{
		ModelID: "nonexistent-model",
	}, func(result *RouteResult) error {
		return errors.New("always fail")
	})

	if err == nil {
		t.Fatal("Expected error when all providers fail")
	}
}

// TestRouterBlindFallback_EmptyModelSkipsBlindFallback verifies that model=""
// (auto mode) does not enter blind fallback when no routable models exist.
func TestRouterBlindFallback_EmptyModelSkipsBlindFallback(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "router-empty-model-no-blind-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)
	discovery := NewModelDiscovery(registry, storage, time.Hour)
	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	// Provider exists but has no discovered/allowed models.
	provider := &Provider{
		ID: "provider-no-models", Name: "Provider No Models", Type: ProviderTypeCustom,
		Enabled: true, Status: ProviderStatusActive, Priority: 100,
		Location: ProviderLocationCloud,
		APIKeys:  []APIKey{{ID: "k1", Key: "key1", Enabled: true}},
	}
	registry.Register(provider)
	router.RebuildCandidates()

	attempts := 0
	err = router.RouteWithFallback(context.Background(), &RouteRequest{
		ModelID: "",
	}, func(result *RouteResult) error {
		attempts++
		return nil
	})

	if !errors.Is(err, ErrNoAvailableProvider) {
		t.Fatalf("expected ErrNoAvailableProvider, got %v", err)
	}
	if attempts != 0 {
		t.Fatalf("expected 0 attempts for empty-model blind fallback, got %d", attempts)
	}
}

// TestRouterBlindFallback_SkipsUnhealthyAndCooldown verifies that blind fallback
// skips providers that are unhealthy or in cooldown.
func TestRouterBlindFallback_SkipsUnhealthyAndCooldown(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "router-blind-skip-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)
	discovery := NewModelDiscovery(registry, storage, time.Hour)
	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	healthyProvider := &Provider{
		ID: "healthy", Name: "Healthy", Type: ProviderTypeCustom,
		Enabled: true, Status: ProviderStatusActive, Priority: 50,
		Location: ProviderLocationCloud,
		APIKeys:  []APIKey{{ID: "k1", Key: "key1", Enabled: true}},
	}
	unhealthyProvider := &Provider{
		ID: "unhealthy", Name: "Unhealthy", Type: ProviderTypeCustom,
		Enabled: true, Status: ProviderStatusError, Priority: 100,
		Location: ProviderLocationCloud,
		APIKeys:  []APIKey{{ID: "k2", Key: "key2", Enabled: true}},
	}
	registry.Register(healthyProvider)
	registry.Register(unhealthyProvider)
	router.RebuildCandidates()

	var triedProviders []string
	err = router.RouteWithFallback(context.Background(), &RouteRequest{
		ModelID: "unknown-model",
	}, func(result *RouteResult) error {
		triedProviders = append(triedProviders, result.Provider.ID)
		return nil
	})

	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}

	// Should only try the healthy provider, not the unhealthy one
	if len(triedProviders) != 1 || triedProviders[0] != "healthy" {
		t.Errorf("Expected only healthy provider, got: %v", triedProviders)
	}
}

// TestRouterBlindFallback_PassthroughModel verifies that blind fallback
// sends the original model ID to the provider (passthrough).
func TestRouterBlindFallback_PassthroughModel(t *testing.T) {
	router, cleanup := setupRouterTest(t)
	defer cleanup()

	var receivedModel string
	err := router.RouteWithFallback(context.Background(), &RouteRequest{
		ModelID: "gpt-5.3-codex",
	}, func(result *RouteResult) error {
		receivedModel = result.Model.ID
		return nil
	})

	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if receivedModel != "gpt-5.3-codex" {
		t.Errorf("Expected passthrough model gpt-5.3-codex, got %s", receivedModel)
	}
}

// TestRouterBlindFallback_RespectsAllowedModels verifies blind fallback does not
// route to providers that disallow the requested model via AllowedModels.
func TestRouterBlindFallback_RespectsAllowedModels(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "router-blind-allowlist-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)
	discovery := NewModelDiscovery(registry, storage, time.Hour)
	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	providerA := &Provider{
		ID: "provider-a", Name: "Provider A", Type: ProviderTypeCustom,
		Enabled: true, Status: ProviderStatusActive, Priority: 100,
		Location: ProviderLocationCloud,
		APIKeys:  []APIKey{{ID: "k1", Key: "key-a", Enabled: true}},
	}
	// Higher priority than provider-c, but should be skipped in blind fallback.
	providerB := &Provider{
		ID: "provider-b", Name: "Provider B", Type: ProviderTypeCustom,
		Enabled: true, Status: ProviderStatusActive, Priority: 90,
		Location:            ProviderLocationCloud,
		APIKeys:             []APIKey{{ID: "k2", Key: "key-b", Enabled: true}},
		AllowedModels:       []string{"other-model"},
		AllowlistConfigured: true,
	}
	providerC := &Provider{
		ID: "provider-c", Name: "Provider C", Type: ProviderTypeCustom,
		Enabled: true, Status: ProviderStatusActive, Priority: 80,
		Location: ProviderLocationCloud,
		APIKeys:  []APIKey{{ID: "k3", Key: "key-c", Enabled: true}},
	}
	registry.Register(providerA)
	registry.Register(providerB)
	registry.Register(providerC)

	// provider-a has the requested model; provider-b/provider-c do not.
	storage.SaveModels("provider-a", []*Model{{
		ID: "special-model", ProviderID: "provider-a", Name: "special-model",
		Enabled: true, Capabilities: ModelCapabilities{Chat: true},
	}})
	storage.SaveModels("provider-b", []*Model{{
		ID: "other-model", ProviderID: "provider-b", Name: "other-model",
		Enabled: true, Capabilities: ModelCapabilities{Chat: true},
	}})
	storage.SaveModels("provider-c", []*Model{{
		ID: "third-model", ProviderID: "provider-c", Name: "third-model",
		Enabled: true, Capabilities: ModelCapabilities{Chat: true},
	}})
	router.RebuildCandidates()

	var tried []string
	err = router.RouteWithFallback(context.Background(), &RouteRequest{
		ModelID: "special-model",
	}, func(result *RouteResult) error {
		tried = append(tried, result.Provider.ID)
		if result.Provider.ID == "provider-a" {
			return errors.New("upstream error")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Expected fallback success, got: %v", err)
	}
	if len(tried) != 2 {
		t.Fatalf("Expected 2 attempts, got %d: %v", len(tried), tried)
	}
	if tried[0] != "provider-a" {
		t.Errorf("First attempt should be provider-a, got %s", tried[0])
	}
	if tried[1] != "provider-c" {
		t.Errorf("Second attempt should skip provider-b due allowlist and use provider-c, got %s", tried[1])
	}
}

// --- OAuth provider routing tests ---

// setupOAuthRouterTest creates a router with one API-key provider and one OAuth-only provider.
func setupOAuthRouterTest(t *testing.T) (*Router, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "router-oauth-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)
	discovery := NewModelDiscovery(registry, storage, time.Hour)
	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	apiKeyProvider := &Provider{
		ID: "apikey-prov", Name: "API Key Provider", Type: ProviderTypeBuiltin,
		Location: ProviderLocationCloud, Enabled: true, Status: ProviderStatusActive,
		Priority: 80,
		APIKeys:  []APIKey{{ID: "k1", Key: "sk-test", Enabled: true}},
	}
	oauthProvider := &Provider{
		ID: "oauth-prov", Name: "OAuth Provider", Type: ProviderTypeBuiltin,
		Location: ProviderLocationCloud, Enabled: true, Status: ProviderStatusActive,
		Priority: 100,
		// No APIKeys — only OAuth
		OAuth: &OAuthConfig{Connected: true, ProviderType: "antigravity"},
	}

	registry.Register(apiKeyProvider)
	registry.Register(oauthProvider)

	model := &Model{
		ID: "gemini-2.5-pro", Name: "gemini-2.5-pro", DisplayName: "Gemini 2.5 Pro",
		Enabled: true, Capabilities: ModelCapabilities{Chat: true, Streaming: true},
	}
	model.ProviderID = "apikey-prov"
	model.InputPrice = 10.0
	model.OutputPrice = 20.0
	storage.SaveModels("apikey-prov", []*Model{model})

	model.ProviderID = "oauth-prov"
	model.InputPrice = 5.0
	model.OutputPrice = 10.0
	storage.SaveModels("oauth-prov", []*Model{model})

	router.RebuildCandidates()

	return router, func() { os.RemoveAll(tmpDir) }
}

// TestOAuthProviderInCandidateSnapshot verifies OAuth-only cloud providers
// are included in the candidate snapshot (not skipped).
func TestOAuthProviderInCandidateSnapshot(t *testing.T) {
	router, cleanup := setupOAuthRouterTest(t)
	defer cleanup()

	// Route should find both providers as candidates
	result, err := router.Route(&RouteRequest{
		ModelID:  "gemini-2.5-pro",
		Strategy: RoutingStrategyPriority,
	})
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	// oauth-prov (priority 100) should be primary, apikey-prov (80) should be fallback
	if result.Provider.ID != "oauth-prov" {
		t.Errorf("Expected oauth-prov as primary, got %s", result.Provider.ID)
	}
	if len(result.Fallbacks) != 1 {
		t.Fatalf("Expected 1 fallback, got %d", len(result.Fallbacks))
	}
	if result.Fallbacks[0].Provider.ID != "apikey-prov" {
		t.Errorf("Expected apikey-prov as fallback, got %s", result.Fallbacks[0].Provider.ID)
	}
}

// TestOAuthProviderRoutesPriority verifies OAuth provider wins when it has higher priority.
func TestOAuthProviderRoutesPriority(t *testing.T) {
	router, cleanup := setupOAuthRouterTest(t)
	defer cleanup()

	result, err := router.Route(&RouteRequest{
		ModelID:  "gemini-2.5-pro",
		Strategy: RoutingStrategyPriority,
	})
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	// oauth-prov has priority 100 > apikey-prov 80
	if result.Provider.ID != "oauth-prov" {
		t.Errorf("Expected oauth-prov (priority 100), got %s", result.Provider.ID)
	}
	if result.OAuth == nil || !result.OAuth.Connected {
		t.Error("Route result should carry OAuth config")
	}
}

// TestOAuthProviderRoutesCost verifies OAuth provider participates in cost-based routing.
func TestOAuthProviderRoutesCost(t *testing.T) {
	router, cleanup := setupOAuthRouterTest(t)
	defer cleanup()

	result, err := router.Route(&RouteRequest{
		ModelID:  "gemini-2.5-pro",
		Strategy: RoutingStrategyCost,
	})
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	// oauth-prov has lower cost (5+10=15) vs apikey-prov (10+20=30)
	if result.Provider.ID != "oauth-prov" {
		t.Errorf("Expected oauth-prov (cheapest), got %s", result.Provider.ID)
	}
}

// TestOAuthProviderFailover verifies OAuth provider participates in failover.
func TestOAuthProviderFailover(t *testing.T) {
	router, cleanup := setupOAuthRouterTest(t)
	defer cleanup()

	var tried []string
	err := router.RouteWithFallback(context.Background(), &RouteRequest{
		ModelID:  "gemini-2.5-pro",
		Strategy: RoutingStrategyPriority,
	}, func(result *RouteResult) error {
		tried = append(tried, result.Provider.ID)
		if result.Provider.ID == "oauth-prov" {
			return errors.New("simulated oauth failure")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("RouteWithFallback failed: %v", err)
	}
	if len(tried) != 2 {
		t.Fatalf("Expected 2 attempts, got %d: %v", len(tried), tried)
	}
	if tried[0] != "oauth-prov" {
		t.Errorf("First attempt should be oauth-prov, got %s", tried[0])
	}
	if tried[1] != "apikey-prov" {
		t.Errorf("Fallback should be apikey-prov, got %s", tried[1])
	}
}

// TestOAuthDisconnectedExcluded verifies that an OAuth provider with Connected=false
// is excluded from routing (same as having no credentials).
func TestOAuthDisconnectedExcluded(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "router-oauth-disc-test-*")
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)
	discovery := NewModelDiscovery(registry, storage, time.Hour)
	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	// OAuth provider with Connected=false, no API keys
	disconnected := &Provider{
		ID: "disc-prov", Name: "Disconnected", Type: ProviderTypeBuiltin,
		Location: ProviderLocationCloud, Enabled: true, Status: ProviderStatusActive,
		Priority: 100,
		OAuth:    &OAuthConfig{Connected: false, ProviderType: "copilot"},
	}
	registry.Register(disconnected)

	storage.SaveModels("disc-prov", []*Model{{
		ID: "test-model", ProviderID: "disc-prov", Name: "test-model",
		Enabled: true, Capabilities: ModelCapabilities{Chat: true},
	}})
	router.RebuildCandidates()

	_, err := router.Route(&RouteRequest{ModelID: "test-model"})
	if err != ErrNoAvailableProvider {
		t.Errorf("Expected ErrNoAvailableProvider for disconnected OAuth, got %v", err)
	}
}

// TestOAuthProviderBlindFallback verifies OAuth providers participate in blind fallback.
func TestOAuthProviderBlindFallback(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "router-oauth-blind-test-*")
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)
	discovery := NewModelDiscovery(registry, storage, time.Hour)
	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	// Only an OAuth provider, no models registered
	oauthProv := &Provider{
		ID: "oauth-only", Name: "OAuth Only", Type: ProviderTypeBuiltin,
		Location: ProviderLocationCloud, Enabled: true, Status: ProviderStatusActive,
		Priority: 100,
		OAuth:    &OAuthConfig{Connected: true, ProviderType: "gemini-cli"},
	}
	registry.Register(oauthProv)
	router.RebuildCandidates()

	var tried []string
	err := router.RouteWithFallback(context.Background(), &RouteRequest{
		ModelID: "unknown-model",
	}, func(result *RouteResult) error {
		tried = append(tried, result.Provider.ID)
		return nil
	})

	if err != nil {
		t.Fatalf("Expected blind fallback to succeed, got: %v", err)
	}
	if len(tried) != 1 || tried[0] != "oauth-only" {
		t.Errorf("Expected oauth-only in blind fallback, got: %v", tried)
	}
}
