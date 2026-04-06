package metrics

import (
	"testing"
)

func TestNewTokenTracker_ReusesSharedDefaultPricingBacking(t *testing.T) {
	first := NewTokenTracker()
	second := NewTokenTracker()

	if len(first.pricing) == 0 || len(second.pricing) == 0 {
		t.Fatal("expected default pricing entries")
	}
	if &first.pricing[0] != &second.pricing[0] {
		t.Fatal("expected NewTokenTracker to reuse shared default pricing backing")
	}
}

func TestDefaultTokenPricing_ReturnsIndependentCopy(t *testing.T) {
	first := DefaultTokenPricing()
	second := DefaultTokenPricing()

	if len(first) == 0 || len(second) == 0 {
		t.Fatal("expected default pricing entries")
	}
	if &first[0] == &second[0] {
		t.Fatal("expected DefaultTokenPricing to return independent copies")
	}

	first[0].InputPrice = 999
	if second[0].InputPrice == 999 {
		t.Fatal("expected modifying one DefaultTokenPricing result not to affect another")
	}
}

func TestTokenTracker_RecordTokenUsage(t *testing.T) {
	tracker := NewTokenTracker()

	tracker.RecordTokenUsage("sonnet", 1000, 500, 100, 50)

	usage := tracker.GetTokenUsage()
	if usage.InputTokens != 1000 {
		t.Errorf("Expected 1000 input tokens, got %d", usage.InputTokens)
	}
	if usage.OutputTokens != 500 {
		t.Errorf("Expected 500 output tokens, got %d", usage.OutputTokens)
	}
	if usage.TotalTokens != 1500 {
		t.Errorf("Expected 1500 total tokens, got %d", usage.TotalTokens)
	}
	if usage.CacheReadTokens != 100 {
		t.Errorf("Expected 100 cache read tokens, got %d", usage.CacheReadTokens)
	}
	if usage.CacheWriteTokens != 50 {
		t.Errorf("Expected 50 cache write tokens, got %d", usage.CacheWriteTokens)
	}
}

func TestTokenTracker_GetTokenUsageByModel(t *testing.T) {
	tracker := NewTokenTracker()

	tracker.RecordTokenUsage("sonnet", 1000, 500, 0, 0)
	tracker.RecordTokenUsage("opus", 2000, 1000, 0, 0)

	sonnetUsage := tracker.GetTokenUsageByModel("sonnet")
	if sonnetUsage == nil {
		t.Fatal("Expected usage for sonnet, got nil")
	}
	if sonnetUsage.InputTokens != 1000 {
		t.Errorf("Expected 1000 input tokens for sonnet, got %d", sonnetUsage.InputTokens)
	}

	opusUsage := tracker.GetTokenUsageByModel("opus")
	if opusUsage == nil {
		t.Fatal("Expected usage for opus, got nil")
	}
	if opusUsage.InputTokens != 2000 {
		t.Errorf("Expected 2000 input tokens for opus, got %d", opusUsage.InputTokens)
	}

	// Non-existent model
	unknownUsage := tracker.GetTokenUsageByModel("nonexistent")
	if unknownUsage != nil {
		t.Error("Expected nil for non-existent model")
	}
}

func TestTokenTracker_CalculateCost(t *testing.T) {
	tracker := NewTokenTracker()

	// Test sonnet pricing: $3/$15 per million tokens
	cost := tracker.CalculateCost("sonnet", 1_000_000, 1_000_000, 0, 0)
	expectedCost := 3.0 + 15.0 // $3 input + $15 output
	if cost != expectedCost {
		t.Errorf("Expected cost %.2f, got %.2f", expectedCost, cost)
	}

	// Test opus pricing: $5/$25 per million tokens
	cost = tracker.CalculateCost("opus", 1_000_000, 1_000_000, 0, 0)
	expectedCost = 5.0 + 25.0 // $5 input + $25 output
	if cost != expectedCost {
		t.Errorf("Expected cost %.2f, got %.2f", expectedCost, cost)
	}

	// Test haiku pricing: $1/$5 per million tokens
	cost = tracker.CalculateCost("haiku", 1_000_000, 1_000_000, 0, 0)
	expectedCost = 1.0 + 5.0 // $1 input + $5 output
	if cost != expectedCost {
		t.Errorf("Expected cost %.2f, got %.2f", expectedCost, cost)
	}
}

func TestNormalizeModelName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"claude-sonnet-4-5-20250929", "sonnet"},
		{"claude-opus-4-5-20251101", "opus"},
		{"claude-3-5-haiku-20241022", "3-5-haiku"}, // Note: this is a tricky case
		{"sonnet", "sonnet"},
		{"opus", "opus"},
		{"haiku", "haiku"},
		{"gpt-4o-2024-08-06", "gpt-4o"},
		{"gemini-1.5-pro-latest", "gemini-1.5-pro-latest"},
	}

	for _, tt := range tests {
		result := normalizeModelName(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeModelName(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		model    string
		pattern  string
		expected bool
	}{
		{"sonnet", "sonnet*", true},
		{"sonnet", "sonnet", true},
		{"sonnet-4", "sonnet*", true},
		{"opus", "sonnet*", false},
		{"opus", "opus*", true},
		{"haiku", "haiku*", true},
		{"gpt-4o", "gpt-4o*", true},
		{"gpt-4o-mini", "gpt-4o*", true},
		{"anything", "*", true},
		{"", "*", true},
	}

	for _, tt := range tests {
		result := matchPattern(tt.model, tt.pattern)
		if result != tt.expected {
			t.Errorf("matchPattern(%s, %s) = %v, expected %v", tt.model, tt.pattern, result, tt.expected)
		}
	}
}

func TestTokenTracker_GetPricingForModel(t *testing.T) {
	tracker := NewTokenTracker()

	// Test Claude models
	pricing := tracker.GetPricingForModel("claude-sonnet-4-5-20250929")
	if pricing.InputPrice != 3.0 {
		t.Errorf("Expected input price 3.0 for sonnet, got %.2f", pricing.InputPrice)
	}

	pricing = tracker.GetPricingForModel("opus")
	if pricing.InputPrice != 5.0 {
		t.Errorf("Expected input price 5.0 for opus, got %.2f", pricing.InputPrice)
	}

	// Test GPT models
	pricing = tracker.GetPricingForModel("gpt-4o")
	if pricing.InputPrice != 2.5 {
		t.Errorf("Expected input price 2.5 for gpt-4o, got %.2f", pricing.InputPrice)
	}

	// Test unknown model (should use default)
	pricing = tracker.GetPricingForModel("unknown-model")
	if pricing.InputPrice != 1.0 {
		t.Errorf("Expected default input price 1.0, got %.2f", pricing.InputPrice)
	}
}

func TestTokenTracker_Reset(t *testing.T) {
	tracker := NewTokenTracker()

	tracker.RecordTokenUsage("sonnet", 1000, 500, 0, 0)

	usage := tracker.GetTokenUsage()
	if usage.TotalTokens != 1500 {
		t.Errorf("Expected 1500 tokens before reset, got %d", usage.TotalTokens)
	}

	tracker.Reset()

	usage = tracker.GetTokenUsage()
	if usage.TotalTokens != 0 {
		t.Errorf("Expected 0 tokens after reset, got %d", usage.TotalTokens)
	}
}

func TestTokenTracker_CacheCost(t *testing.T) {
	tracker := NewTokenTracker()

	// Test with cache tokens for sonnet
	// Cache read: $0.30/M, Cache write: $3.75/M
	cost := tracker.CalculateCost("sonnet", 0, 0, 1_000_000, 1_000_000)
	expectedCost := 0.30 + 3.75
	if cost != expectedCost {
		t.Errorf("Expected cache cost %.2f, got %.2f", expectedCost, cost)
	}
}

func TestTokenTracker_GetAllModelUsage(t *testing.T) {
	tracker := NewTokenTracker()

	tracker.RecordTokenUsage("sonnet", 1000, 500, 0, 0)
	tracker.RecordTokenUsage("opus", 2000, 1000, 0, 0)
	tracker.RecordTokenUsage("haiku", 500, 250, 0, 0)

	allUsage := tracker.GetAllModelUsage()
	if len(allUsage) != 3 {
		t.Errorf("Expected 3 models, got %d", len(allUsage))
	}

	// Check that all models are present
	models := make(map[string]bool)
	for _, u := range allUsage {
		models[u.Model] = true
	}

	if !models["sonnet"] || !models["opus"] || !models["haiku"] {
		t.Error("Not all models present in usage")
	}
}
