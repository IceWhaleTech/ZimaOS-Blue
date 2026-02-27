package providerpool

import (
	"testing"
	"time"
)

// mockStorage implements Storage interface for testing
type mockStorage struct {
	providers     map[string]*Provider
	models        map[string][]*Model
	pricingConfig *PricingConfig
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		providers:     make(map[string]*Provider),
		models:        make(map[string][]*Model),
		pricingConfig: DefaultPricingConfig(),
	}
}

func (m *mockStorage) SaveProvider(provider *Provider) error                          { return nil }
func (m *mockStorage) LoadProvider(id string) (*Provider, error)                      { return nil, nil }
func (m *mockStorage) LoadAllProviders() ([]*Provider, error)                         { return nil, nil }
func (m *mockStorage) DeleteProvider(id string) error                                 { return nil }
func (m *mockStorage) SaveModels(providerID string, models []*Model) error            { return nil }
func (m *mockStorage) LoadModels(providerID string) ([]*Model, error)                 { return nil, nil }
func (m *mockStorage) AppendUsage(record *UsageRecord) error                          { return nil }
func (m *mockStorage) LoadUsage(providerID string, start, end time.Time) ([]*UsageRecord, error) {
	return nil, nil
}
func (m *mockStorage) SavePricingConfig(config *PricingConfig) error {
	m.pricingConfig = config
	return nil
}
func (m *mockStorage) LoadPricingConfig() (*PricingConfig, error) {
	return m.pricingConfig, nil
}
func (m *mockStorage) LoadConfig() (*PoolConfig, error) {
	return &PoolConfig{
		DefaultStrategy:    RoutingStrategyPriority,
		DefaultRoutingMode: RoutingModeAuto,
	}, nil
}
func (m *mockStorage) SaveConfig(config *PoolConfig) error {
	return nil
}

func TestNormalizeModelID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"claude-opus-4-5-20251101", "claude-opus-4-5"},
		{"gpt-4o-2024-08-06", "gpt-4o"},
		{"claude-3-5-sonnet-20241022", "claude-3-5-sonnet"},
		{"gpt-4-turbo-latest", "gpt-4-turbo"},
		{"gemini-1.5-pro", "gemini-1.5-pro"},
		{"CLAUDE-OPUS-4-5", "claude-opus-4-5"},
		// Spaces → hyphens
		{"Claude Opus 4.5", "claude-opus-4.5"},
		{"GPT 4o Mini", "gpt-4o-mini"},
		// Underscores → hyphens
		{"gpt_4o_mini", "gpt-4o-mini"},
		{"claude_sonnet_4", "claude-sonnet-4"},
		// Provider prefixes stripped
		{"azure/gpt-4o", "gpt-4o"},
		{"bedrock/claude-3-5-sonnet-20241022", "claude-3-5-sonnet"},
		{"openrouter/deepseek-chat", "deepseek-chat"},
		// Version suffixes stripped
		{"gpt-4o:v1", "gpt-4o"},
		{"claude-3-opus-v2", "claude-3-opus"},
		// Preview suffix stripped
		{"gemini-2.5-pro-preview", "gemini-2.5-pro"},
		// Mixed: spaces + date + uppercase
		{"Claude Sonnet 4.5 20250929", "claude-sonnet-4.5"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeModelID(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeModelID(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractKeywords(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"claude-opus-4-5", []string{"claude", "opus"}},
		{"gpt-4o-mini", []string{"gpt", "4o", "mini"}},
		{"gemini-1.5-pro", []string{"gemini", "pro"}},
		{"deepseek-chat", []string{"deepseek", "chat"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractKeywords(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("extractKeywords(%q) = %v, want %v", tt.input, result, tt.expected)
				return
			}
			for i, kw := range result {
				if kw != tt.expected[i] {
					t.Errorf("extractKeywords(%q)[%d] = %q, want %q", tt.input, i, kw, tt.expected[i])
				}
			}
		})
	}
}

func TestMatchModelPricing_ExactMatch(t *testing.T) {
	tests := []struct {
		modelID       string
		expectedInput float64
	}{
		{"claude-opus-4-5", 5.0},
		{"claude-sonnet-4-5", 3.0},
		{"claude-opus-4-6", 5.0},
		{"claude-sonnet-4-6", 3.0},
		{"gpt-4o", 2.5},
		{"gpt-4o-mini", 0.15},
		{"gpt-5", 1.25},
		{"gpt-5.2", 1.75},
		{"gemini-1.5-pro", 1.25},
		{"gemini-2.5-pro", 1.25},
		{"gemini-2.5-flash", 0.30},
		{"deepseek-chat", 0.28},
		{"deepseek-reasoner", 0.28},
		{"o3", 2.0},
		{"o4-mini", 1.1},
		{"grok-3", 2.0},
		{"grok-3-mini", 0.10},
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			result := MatchModelPricing(tt.modelID)
			if result == nil {
				t.Errorf("MatchModelPricing(%q) = nil, want non-nil", tt.modelID)
				return
			}
			if result.InputPrice != tt.expectedInput {
				t.Errorf("MatchModelPricing(%q).InputPrice = %v, want %v", tt.modelID, result.InputPrice, tt.expectedInput)
			}
		})
	}
}

func TestMatchModelPricing_WithDateSuffix(t *testing.T) {
	tests := []struct {
		modelID       string
		expectedInput float64
	}{
		{"claude-opus-4-5-20251101", 5.0},
		{"claude-sonnet-4-5-20250929", 3.0},
		{"gpt-4o-2024-08-06", 2.5},
		{"claude-3-5-sonnet-20241022", 3.0},
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			result := MatchModelPricing(tt.modelID)
			if result == nil {
				t.Errorf("MatchModelPricing(%q) = nil, want non-nil", tt.modelID)
				return
			}
			if result.InputPrice != tt.expectedInput {
				t.Errorf("MatchModelPricing(%q).InputPrice = %v, want %v", tt.modelID, result.InputPrice, tt.expectedInput)
			}
		})
	}
}

func TestMatchModelPricing_FamilyMatch(t *testing.T) {
	tests := []struct {
		modelID       string
		expectedInput float64
		description   string
	}{
		// Claude family
		{"claude-haiku-4-5", 1.0, "Claude haiku exact match"},
		{"claude-haiku-4-5-20251001", 1.0, "Claude haiku with date"},
		{"anthropic-claude-opus", 5.0, "Anthropic prefix"},

		// GPT family — unknown models fall back to DefaultTier "4o"
		{"gpt-4o-mini-2024-07-18", 0.15, "GPT-4o-mini with date"},
		{"openai-gpt-4-turbo", 10.0, "OpenAI prefix"},
		{"gpt-5.3-codex", 1.25, "Unknown GPT model matches gpt-5 prefix"},

		// Gemini family
		{"gemini-2.0-flash-exp", 0.1, "Gemini flash variant"},
		{"google-gemini-pro", 2.0, "Google prefix"},

		// DeepSeek family
		{"deepseek-coder-v2", 0.28, "DeepSeek coder"},

		// Grok family
		{"grok-2-1212", 2.0, "Grok with version"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			result := MatchModelPricing(tt.modelID)
			if result == nil {
				t.Errorf("MatchModelPricing(%q) = nil, want non-nil for %s", tt.modelID, tt.description)
				return
			}
			if result.InputPrice != tt.expectedInput {
				t.Errorf("MatchModelPricing(%q).InputPrice = %v, want %v for %s", tt.modelID, result.InputPrice, tt.expectedInput, tt.description)
			}
		})
	}
}

func TestMatchModelPricing_NoMatch(t *testing.T) {
	tests := []string{
		"unknown-model",
		"custom-llm-v1",
		"my-fine-tuned-model",
	}

	for _, modelID := range tests {
		t.Run(modelID, func(t *testing.T) {
			result := MatchModelPricing(modelID)
			if result != nil {
				t.Errorf("MatchModelPricing(%q) = %+v, want nil", modelID, result)
			}
		})
	}
}

func TestMatchModelPricing_FuzzyNames(t *testing.T) {
	tests := []struct {
		modelID       string
		expectedInput float64
		description   string
	}{
		// Spaces instead of hyphens
		{"Claude Opus 4.6", 5.0, "Spaces: Claude Opus 4.6"},
		{"GPT 4o Mini", 0.15, "Spaces: GPT 4o Mini"},
		// Underscores
		{"gpt_4o_mini", 0.15, "Underscores: gpt_4o_mini"},
		{"deepseek_chat", 0.28, "Underscores: deepseek_chat"},
		// Provider prefix
		{"azure/gpt-4o", 2.5, "Azure prefix"},
		{"bedrock/claude-3-5-sonnet-20241022", 3.0, "Bedrock prefix"},
		// Mixed case + spaces + date
		{"Claude Sonnet 4.5 20250929", 3.0, "Mixed: Claude Sonnet 4.5 with date"},
		// Version suffix
		{"gpt-4o:v1", 2.5, "Version suffix :v1"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			result := MatchModelPricing(tt.modelID)
			if result == nil {
				t.Errorf("MatchModelPricing(%q) = nil, want non-nil for %s", tt.modelID, tt.description)
				return
			}
			if result.InputPrice != tt.expectedInput {
				t.Errorf("MatchModelPricing(%q).InputPrice = %v, want %v for %s", tt.modelID, result.InputPrice, tt.expectedInput, tt.description)
			}
		})
	}
}

func TestMatchModelPricing_LongestPrefixWins(t *testing.T) {
	// gpt-4o-mini should match "gpt-4o-mini" ($0.15), not "gpt-4o" ($2.5) or "gpt-4" ($30)
	result := MatchModelPricing("gpt-4o-mini-2024-07-18")
	if result == nil {
		t.Fatal("MatchModelPricing(\"gpt-4o-mini-2024-07-18\") = nil")
	}
	if result.InputPrice != 0.15 {
		t.Errorf("InputPrice = %v, want 0.15 (should match gpt-4o-mini, not gpt-4o or gpt-4)", result.InputPrice)
	}
}

func TestMatchModelFamily(t *testing.T) {
	tests := []struct {
		modelID      string
		expectedFam  string
		expectedTier string
	}{
		{"claude-opus-4-5", "claude", "opus"},
		{"claude-sonnet-4", "claude", "sonnet"},
		{"claude-haiku-3-5", "claude", "haiku"},
		{"gpt-4o", "gpt", "4o"},
		{"gpt-4o-mini", "gpt", "4o-mini"},
		{"gemini-1.5-pro", "gemini", "pro"},
		{"gemini-2.0-flash", "gemini", "flash"},
		{"deepseek-chat", "deepseek", "chat"},
		{"deepseek-reasoner", "deepseek", "reasoner"},
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			family, tier := matchModelFamily(tt.modelID)
			if family == nil {
				t.Errorf("matchModelFamily(%q) returned nil family", tt.modelID)
				return
			}
			if family.Name != tt.expectedFam {
				t.Errorf("matchModelFamily(%q) family = %q, want %q", tt.modelID, family.Name, tt.expectedFam)
			}
			if tier != tt.expectedTier {
				t.Errorf("matchModelFamily(%q) tier = %q, want %q", tt.modelID, tier, tt.expectedTier)
			}
		})
	}
}

func TestMatchModelFamily_DefaultTier(t *testing.T) {
	// gpt-5.3-codex has no matching tier → should fall back to DefaultTier "4o"
	family, tier := matchModelFamily("gpt-5.3-codex")
	if family == nil {
		t.Fatal("matchModelFamily(\"gpt-5.3-codex\") returned nil family")
	}
	if family.Name != "gpt" {
		t.Errorf("family = %q, want \"gpt\"", family.Name)
	}
	if tier != "4o" {
		t.Errorf("tier = %q, want \"4o\" (DefaultTier)", tier)
	}
}

func TestPricingManager_GetModelPricing_WithHeuristics(t *testing.T) {
	// Create a mock storage
	storage := newMockStorage()

	pm, err := NewPricingManager(storage)
	if err != nil {
		t.Fatalf("NewPricingManager() error = %v", err)
	}

	// Test heuristic matching through PricingManager
	tests := []struct {
		modelID       string
		expectedInput float64
	}{
		{"claude-opus-4-5-20251101", 5.0},
		{"gpt-4o-mini", 0.15},
		{"unknown-model", 5.0}, // Should fall back to default
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			result := pm.GetModelPricing(tt.modelID, "")
			if result.InputPrice != tt.expectedInput {
				t.Errorf("GetModelPricing(%q).InputPrice = %v, want %v", tt.modelID, result.InputPrice, tt.expectedInput)
			}
		})
	}
}

func TestPricingManager_CustomPricingOverridesHeuristic(t *testing.T) {
	storage := newMockStorage()

	pm, err := NewPricingManager(storage)
	if err != nil {
		t.Fatalf("NewPricingManager() error = %v", err)
	}

	// Set custom pricing for a model
	customPricing := &ModelPricing{
		ModelID:     "claude-opus-4-5",
		InputPrice:  99.0, // Override the builtin price
		OutputPrice: 199.0,
	}
	if err := pm.SetModelPricing(customPricing); err != nil {
		t.Fatalf("SetModelPricing() error = %v", err)
	}

	// Custom pricing should take precedence
	result := pm.GetModelPricing("claude-opus-4-5", "")
	if result.InputPrice != 99.0 {
		t.Errorf("GetModelPricing() with custom pricing: InputPrice = %v, want 99.0", result.InputPrice)
	}
}
