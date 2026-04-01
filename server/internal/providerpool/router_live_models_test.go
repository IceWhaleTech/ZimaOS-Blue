package providerpool

import (
	"os"
	"testing"
	"time"
)

func TestListAvailableModelsExcludesUnroutableProviders(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "router-live-models-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := NewFileStorage(tmpDir)
	registry, _ := NewRegistry(storage)

	enabled := &Provider{
		ID:       "enabled-cloud",
		Name:     "Enabled Cloud",
		Type:     ProviderTypeCustom,
		Location: ProviderLocationCloud,
		Enabled:  true,
		Status:   ProviderStatusActive,
		APIKeys:  []APIKey{{ID: "k1", Key: "live-key", Enabled: true}},
	}
	noKey := &Provider{
		ID:       "no-key-cloud",
		Name:     "No Key Cloud",
		Type:     ProviderTypeCustom,
		Location: ProviderLocationCloud,
		Enabled:  true,
		Status:   ProviderStatusActive,
	}
	disabled := &Provider{
		ID:       "disabled-cloud",
		Name:     "Disabled Cloud",
		Type:     ProviderTypeCustom,
		Location: ProviderLocationCloud,
		Enabled:  false,
		Status:   ProviderStatusActive,
		APIKeys:  []APIKey{{ID: "k2", Key: "disabled-key", Enabled: true}},
	}
	registry.Register(enabled)
	registry.Register(noKey)
	registry.Register(disabled)

	if err := storage.SaveModels("enabled-cloud", []*Model{{
		ID:         "claude-sonnet-4-6",
		Name:       "claude-sonnet-4-6",
		ProviderID: "enabled-cloud",
		Enabled:    true,
	}}); err != nil {
		t.Fatalf("save enabled models: %v", err)
	}
	if err := storage.SaveModels("no-key-cloud", []*Model{{
		ID:         "claude-3-5-haiku-20241022",
		Name:       "claude-3-5-haiku-20241022",
		ProviderID: "no-key-cloud",
		Enabled:    true,
	}}); err != nil {
		t.Fatalf("save no-key models: %v", err)
	}
	if err := storage.SaveModels("disabled-cloud", []*Model{{
		ID:         "gpt-4o-mini",
		Name:       "gpt-4o-mini",
		ProviderID: "disabled-cloud",
		Enabled:    true,
	}}); err != nil {
		t.Fatalf("save disabled models: %v", err)
	}

	discovery := NewModelDiscovery(registry, storage, time.Hour)
	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	models := router.ListAvailableModels()
	if len(models) != 1 {
		t.Fatalf("available models len = %d, want 1", len(models))
	}
	if models[0].ID != "claude-sonnet-4-6" {
		t.Fatalf("available model id = %q, want claude-sonnet-4-6", models[0].ID)
	}
	if models[0].ProviderID != "enabled-cloud" {
		t.Fatalf("available model provider = %q, want enabled-cloud", models[0].ProviderID)
	}
}
