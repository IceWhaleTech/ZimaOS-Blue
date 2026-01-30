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

	enc, _ := NewEncryptor("test-key")
	storage, _ := NewFileStorage(tmpDir, enc)
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

	storage, _ := NewFileStorage(tmpDir, nil)
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
