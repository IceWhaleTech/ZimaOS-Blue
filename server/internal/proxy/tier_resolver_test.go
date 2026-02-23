package proxy

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

func makeModel(id, providerID string, inputPrice, outputPrice float64) *providerpool.Model {
	return &providerpool.Model{
		ID:          id,
		ProviderID:  providerID,
		Enabled:     true,
		InputPrice:  inputPrice,
		OutputPrice: outputPrice,
	}
}

func TestTierResolver_Resolve_Percentile(t *testing.T) {
	tr := NewTierResolver()

	// 6 models sorted by cost: [0.3, 0.5, 5.0, 7.0, 25.0, 45.0]
	// n=6, P50=index 3=7.0, P75=index 4=25.0
	// <7.0 → economy, ≥7.0 & <25.0 → standard, ≥25.0 → premium
	models := []*providerpool.Model{
		makeModel("xtest-cheap-1", "p1", 0.1, 0.2),       // total 0.3 → economy
		makeModel("xtest-cheap-2", "p1", 0.2, 0.3),       // total 0.5 → economy
		makeModel("xtest-mid-1", "p2", 2.0, 3.0),         // total 5.0 → economy (< P50)
		makeModel("xtest-mid-2", "p2", 3.0, 4.0),         // total 7.0 → standard (= P50)
		makeModel("xtest-expensive-1", "p3", 10.0, 15.0), // total 25.0 → premium (= P75)
		makeModel("xtest-expensive-2", "p3", 15.0, 30.0), // total 45.0 → premium
	}

	ok := tr.Resolve(models)
	if !ok {
		t.Fatal("Resolve() returned false, want true (≥2 tiers)")
	}
	if !tr.IsEnabled() {
		t.Fatal("IsEnabled() = false after successful Resolve")
	}

	// Check tier assignments
	tests := []struct {
		modelID  string
		wantTier ModelTier
	}{
		{"xtest-cheap-1", TierEconomy},
		{"xtest-cheap-2", TierEconomy},
		{"xtest-mid-1", TierEconomy},
		{"xtest-mid-2", TierStandard},
		{"xtest-expensive-1", TierPremium},
		{"xtest-expensive-2", TierPremium},
	}
	for _, tt := range tests {
		got := tr.ModelTierOf(tt.modelID)
		if got != tt.wantTier {
			t.Errorf("ModelTierOf(%q) = %q, want %q", tt.modelID, got, tt.wantTier)
		}
	}
}

func TestTierResolver_Resolve_AbsoluteThreshold(t *testing.T) {
	tr := NewTierResolver()

	// Only 2 models — uses absolute thresholds
	models := []*providerpool.Model{
		makeModel("cheap", "p1", 0.5, 0.5),   // total 1.0 → economy (<$2)
		makeModel("pricey", "p2", 8.0, 12.0), // total 20.0 → premium (>$10)
	}

	ok := tr.Resolve(models)
	if !ok {
		t.Fatal("Resolve() returned false, want true")
	}

	if tier := tr.ModelTierOf("cheap"); tier != TierEconomy {
		t.Errorf("cheap tier = %q, want economy", tier)
	}
	if tier := tr.ModelTierOf("pricey"); tier != TierPremium {
		t.Errorf("pricey tier = %q, want premium", tier)
	}
}

func TestTierResolver_Resolve_WithFreeModels(t *testing.T) {
	tr := NewTierResolver()

	models := []*providerpool.Model{
		makeModel("xtest-local-mymodel", "ollama", 0, 0), // unrecognizable name → free
		makeModel("xtest-cloud-model", "openai", 5.0, 15.0), // total 20.0
	}

	ok := tr.Resolve(models)
	if !ok {
		t.Fatal("Resolve() returned false, want true (free + premium = 2 tiers)")
	}

	if tier := tr.ModelTierOf("xtest-local-mymodel"); tier != TierFree {
		t.Errorf("xtest-local-mymodel tier = %q, want free", tier)
	}
}

func TestTierResolver_Resolve_SingleTier(t *testing.T) {
	tr := NewTierResolver()

	// All models same price range → single tier → not enabled
	models := []*providerpool.Model{
		makeModel("a", "p1", 0.5, 0.5),
		makeModel("b", "p1", 0.6, 0.6),
	}

	ok := tr.Resolve(models)
	if ok {
		t.Fatal("Resolve() returned true, want false (all same tier)")
	}
	if tr.IsEnabled() {
		t.Fatal("IsEnabled() = true, want false")
	}
}

func TestTierResolver_Resolve_DisabledModels(t *testing.T) {
	tr := NewTierResolver()

	models := []*providerpool.Model{
		makeModel("enabled", "p1", 0.5, 0.5),
		{ID: "disabled", ProviderID: "p2", Enabled: false, InputPrice: 50, OutputPrice: 50},
	}

	tr.Resolve(models)
	if tier := tr.ModelTierOf("disabled"); tier != "" {
		t.Errorf("disabled model tier = %q, want empty", tier)
	}
}

func TestTierResolver_BestModelForTier(t *testing.T) {
	tr := NewTierResolver()

	models := []*providerpool.Model{
		makeModel("cheap-a", "p1", 0.1, 0.1),
		makeModel("cheap-b", "p1", 0.2, 0.2),
		makeModel("mid", "p2", 3.0, 4.0),
		makeModel("expensive", "p3", 15.0, 30.0),
	}
	tr.Resolve(models)

	// Economy should return cheapest
	if got := tr.BestModelForTier(TierEconomy); got != "cheap-a" {
		t.Errorf("BestModelForTier(economy) = %q, want cheap-a", got)
	}

	// Premium should return the premium model
	if got := tr.BestModelForTier(TierPremium); got != "expensive" {
		t.Errorf("BestModelForTier(premium) = %q, want expensive", got)
	}
}

func TestTierResolver_BestModelForTier_Fallback(t *testing.T) {
	tr := NewTierResolver()

	// Only economy and premium — no standard
	models := []*providerpool.Model{
		makeModel("cheap", "p1", 0.1, 0.1),
		makeModel("pricey", "p2", 15.0, 30.0),
	}
	tr.Resolve(models)

	// Standard should fall back to economy
	if got := tr.BestModelForTier(TierStandard); got == "" {
		t.Error("BestModelForTier(standard) = empty, want fallback to economy or premium")
	}
}

func TestTierResolver_ModelTierOf_Unknown(t *testing.T) {
	tr := NewTierResolver()
	tr.Resolve([]*providerpool.Model{makeModel("known", "p1", 1, 1)})

	if tier := tr.ModelTierOf("unknown-model"); tier != "" {
		t.Errorf("ModelTierOf(unknown) = %q, want empty", tier)
	}
}

func TestTierResolver_Resolve_Empty(t *testing.T) {
	tr := NewTierResolver()
	ok := tr.Resolve(nil)
	if ok {
		t.Fatal("Resolve(nil) returned true, want false")
	}
	if tr.IsEnabled() {
		t.Fatal("IsEnabled() = true after empty resolve")
	}
}

func TestTierResolver_Stats(t *testing.T) {
	tr := NewTierResolver()
	models := []*providerpool.Model{
		makeModel("cheap", "p1", 0.1, 0.1),
		makeModel("pricey", "p2", 15.0, 30.0),
	}
	tr.Resolve(models)

	stats := tr.Stats()
	if stats["enabled"] != true {
		t.Errorf("Stats().enabled = %v, want true", stats["enabled"])
	}
	if stats["total_models"].(int) != 2 {
		t.Errorf("Stats().total_models = %v, want 2", stats["total_models"])
	}
}

func TestClassifyByAbsoluteThreshold(t *testing.T) {
	tests := []struct {
		cost float64
		want ModelTier
	}{
		{0.5, TierEconomy},
		{1.9, TierEconomy},
		{2.1, TierStandard},
		{9.9, TierStandard},
		{10.1, TierPremium},
		{50.0, TierPremium},
	}
	for _, tt := range tests {
		got := classifyByAbsoluteThreshold(tt.cost)
		if got != tt.want {
			t.Errorf("classifyByAbsoluteThreshold(%v) = %q, want %q", tt.cost, got, tt.want)
		}
	}
}
