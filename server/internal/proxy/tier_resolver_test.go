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

func TestTierResolver_Resolve_BigAndBuiltinSmall(t *testing.T) {
	tr := NewTierResolver()
	models := []*providerpool.Model{
		makeModel("gpt-4o", "openai", 2.5, 10.0),                       // large
		makeModel("gpt-4o-mini", "openai", 0.15, 0.6),                  // small (builtin allowlist)
		makeModel("claude-opus-4-5-20251101", "anthropic", 15.0, 75.0), // large
		makeModel("claude-3-5-haiku-20241022", "anthropic", 0.8, 4.0),  // small (builtin allowlist)
		makeModel("gemini-1.5-pro", "google", 1.25, 5.0),               // large
		makeModel("gemini-2.0-flash", "google", 0.1, 0.4),              // small (builtin allowlist)
	}

	if ok := tr.Resolve(models); !ok {
		t.Fatal("Resolve() = false, want true when both large and small tiers exist")
	}
	if !tr.IsEnabled() {
		t.Fatal("IsEnabled() = false, want true")
	}

	if got := tr.ModelTierOf("gpt-4o"); got != TierLarge {
		t.Errorf("ModelTierOf(gpt-4o) = %q, want %q", got, TierLarge)
	}
	if got := tr.ModelTierOf("gpt-4o-mini"); got != TierSmall {
		t.Errorf("ModelTierOf(gpt-4o-mini) = %q, want %q", got, TierSmall)
	}
}

func TestTierResolver_Resolve_UnknownSmallNameNotAutoAccepted(t *testing.T) {
	tr := NewTierResolver()

	// gpt-5-nano is intentionally not in builtin small allowlist.
	models := []*providerpool.Model{
		makeModel("gpt-5.3-codex-spark", "openai", 8.0, 32.0),
		makeModel("gpt-5-nano", "openai", 0.2, 0.8),
	}

	if ok := tr.Resolve(models); ok {
		t.Fatal("Resolve() = true, want false (all should be treated as TierLarge)")
	}
	if tr.IsEnabled() {
		t.Fatal("IsEnabled() = true, want false when small tier is unavailable")
	}

	if got := tr.ModelTierOf("gpt-5-nano"); got != TierLarge {
		t.Errorf("ModelTierOf(gpt-5-nano) = %q, want %q", got, TierLarge)
	}
}

func TestTierResolver_BestModelForTier_SmallPriority(t *testing.T) {
	tr := NewTierResolver()
	models := []*providerpool.Model{
		makeModel("gemini-2.0-flash", "google", 0.1, 0.4),
		makeModel("gpt-4o-mini", "openai", 0.15, 0.6),
		makeModel("claude-3-5-haiku-20241022", "anthropic", 0.8, 4.0),
		makeModel("gpt-4o", "openai", 2.5, 10.0),
	}
	tr.Resolve(models)

	// Priority order prefers gpt-4o-mini over other small models.
	if got := tr.BestModelForTier(TierSmall); got != "gpt-4o-mini" {
		t.Errorf("BestModelForTier(TierSmall) = %q, want gpt-4o-mini", got)
	}
}

func TestTierResolver_BestModelForTier_DirectTiers(t *testing.T) {
	tr := NewTierResolver()
	models := []*providerpool.Model{
		makeModel("gpt-4o", "openai", 2.5, 10.0),
		makeModel("gpt-4o-mini", "openai", 0.15, 0.6),
	}
	tr.Resolve(models)

	if got := tr.BestModelForTier(TierSmall); got != "gpt-4o-mini" {
		t.Errorf("BestModelForTier(TierSmall) = %q, want gpt-4o-mini", got)
	}
	if got := tr.BestModelForTier(TierLarge); got != "gpt-4o" {
		t.Errorf("BestModelForTier(TierLarge) = %q, want gpt-4o", got)
	}
}

func TestTierResolver_BestModelForTier_Fallback(t *testing.T) {
	tr := NewTierResolver()
	models := []*providerpool.Model{
		makeModel("gpt-4o", "openai", 2.5, 10.0), // only large
	}
	tr.Resolve(models)

	if got := tr.BestModelForTier(TierSmall); got != "gpt-4o" {
		t.Errorf("BestModelForTier(TierSmall) fallback = %q, want gpt-4o", got)
	}
	if got := tr.BestModelForTier(TierLarge); got != "gpt-4o" {
		t.Errorf("BestModelForTier(TierLarge) = %q, want gpt-4o", got)
	}
}

func TestTierResolver_Resolve_DisabledModels(t *testing.T) {
	tr := NewTierResolver()
	models := []*providerpool.Model{
		makeModel("gpt-4o-mini", "openai", 0.15, 0.6),
		{ID: "gpt-4o", ProviderID: "openai", Enabled: false, InputPrice: 2.5, OutputPrice: 10.0},
	}
	tr.Resolve(models)

	if tier := tr.ModelTierOf("gpt-4o"); tier != "" {
		t.Errorf("disabled model tier = %q, want empty", tier)
	}
}

func TestTierResolver_ModelTierOf_Unknown(t *testing.T) {
	tr := NewTierResolver()
	tr.Resolve([]*providerpool.Model{makeModel("gpt-4o", "openai", 2.5, 10.0)})

	if tier := tr.ModelTierOf("unknown-model"); tier != "" {
		t.Errorf("ModelTierOf(unknown) = %q, want empty", tier)
	}
}

func TestTierResolver_Resolve_Empty(t *testing.T) {
	tr := NewTierResolver()
	if ok := tr.Resolve(nil); ok {
		t.Fatal("Resolve(nil) = true, want false")
	}
	if tr.IsEnabled() {
		t.Fatal("IsEnabled() = true after empty resolve, want false")
	}
}

func TestTierResolver_Stats(t *testing.T) {
	tr := NewTierResolver()
	models := []*providerpool.Model{
		makeModel("gpt-4o", "openai", 2.5, 10.0),
		makeModel("gpt-4o-mini", "openai", 0.15, 0.6),
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
