package providerpool

import (
	"context"
	"errors"
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
			ID:          "vision-model",
			ProviderID:  "vision-provider",
			Name:        "vision-model",
			Enabled:     true,
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

	// Should have 3 models (one from each provider)
	if len(models) != 3 {
		t.Errorf("Expected 3 models, got %d", len(models))
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
			return errors.New("simulated timeout error")
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

	if capturedResult.FailedAttempts[0].Reason != FailoverReasonTimeout {
		t.Errorf("Expected timeout reason, got %s", capturedResult.FailedAttempts[0].Reason)
	}

	if capturedResult.SuccessProvider == "" {
		t.Error("Success provider should be set")
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
		Name:      "tribios",
		Type:      ProviderTypeCustom,
		Location:  ProviderLocationCloud,
		Enabled:   true,
		Status:    ProviderStatusActive,
		BaseURL:   "https://api-paid.tribios.top",
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
