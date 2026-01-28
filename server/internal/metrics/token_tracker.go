package metrics

import (
	"regexp"
	"strings"
	"sync"
)

// TokenTracker tracks token usage and calculates costs.
type TokenTracker struct {
	mu sync.RWMutex

	// Token usage by model
	modelUsage map[string]*TokenUsage

	// Global usage
	globalUsage *TokenUsage

	// Pricing configuration
	pricing []TokenPricing
}

// DefaultTokenPricing returns the default token pricing configuration.
func DefaultTokenPricing() []TokenPricing {
	return []TokenPricing{
		// Anthropic Claude series
		{Pattern: "opus*", InputPrice: 5.00, OutputPrice: 25.00, CacheRead: 0.50, CacheWrite: 6.25},
		{Pattern: "sonnet*", InputPrice: 3.00, OutputPrice: 15.00, CacheRead: 0.30, CacheWrite: 3.75},
		{Pattern: "haiku*", InputPrice: 1.00, OutputPrice: 5.00, CacheRead: 0.10, CacheWrite: 1.25},

		// OpenAI GPT series
		{Pattern: "gpt-4o*", InputPrice: 2.50, OutputPrice: 10.00},
		{Pattern: "gpt-4-turbo*", InputPrice: 10.00, OutputPrice: 30.00},
		{Pattern: "gpt-4*", InputPrice: 30.00, OutputPrice: 60.00},
		{Pattern: "gpt-3.5*", InputPrice: 0.50, OutputPrice: 1.50},
		{Pattern: "o1-preview*", InputPrice: 15.00, OutputPrice: 60.00},
		{Pattern: "o1-mini*", InputPrice: 3.00, OutputPrice: 12.00},

		// Google Gemini series
		{Pattern: "gemini-1.5-pro*", InputPrice: 1.25, OutputPrice: 5.00},
		{Pattern: "gemini-1.5-flash*", InputPrice: 0.075, OutputPrice: 0.30},
		{Pattern: "gemini-2.0*", InputPrice: 0.10, OutputPrice: 0.40},

		// Meta Llama series
		{Pattern: "llama-3.1-405b*", InputPrice: 3.00, OutputPrice: 3.00},
		{Pattern: "llama-3.1-70b*", InputPrice: 0.88, OutputPrice: 0.88},
		{Pattern: "llama-3.1-8b*", InputPrice: 0.18, OutputPrice: 0.18},

		// Mistral series
		{Pattern: "mistral-large*", InputPrice: 2.00, OutputPrice: 6.00},
		{Pattern: "mistral-medium*", InputPrice: 2.70, OutputPrice: 8.10},
		{Pattern: "mistral-small*", InputPrice: 0.20, OutputPrice: 0.60},

		// DeepSeek series
		{Pattern: "deepseek-v3*", InputPrice: 0.27, OutputPrice: 1.10},
		{Pattern: "deepseek-r1*", InputPrice: 0.55, OutputPrice: 2.19},

		// Default (fallback)
		{Pattern: "*", InputPrice: 1.00, OutputPrice: 5.00},
	}
}

// NewTokenTracker creates a new TokenTracker with default pricing.
func NewTokenTracker() *TokenTracker {
	return &TokenTracker{
		modelUsage:  make(map[string]*TokenUsage),
		globalUsage: &TokenUsage{},
		pricing:     DefaultTokenPricing(),
	}
}

// NewTokenTrackerWithPricing creates a new TokenTracker with custom pricing.
func NewTokenTrackerWithPricing(pricing []TokenPricing) *TokenTracker {
	return &TokenTracker{
		modelUsage:  make(map[string]*TokenUsage),
		globalUsage: &TokenUsage{},
		pricing:     pricing,
	}
}

// RecordTokenUsage records token usage for a call.
func (t *TokenTracker) RecordTokenUsage(model string, inputTokens, outputTokens, cacheRead, cacheWrite int64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if model == "" {
		model = "unknown"
	}

	// Update model usage
	usage, ok := t.modelUsage[model]
	if !ok {
		usage = &TokenUsage{}
		t.modelUsage[model] = usage
	}

	usage.InputTokens += inputTokens
	usage.OutputTokens += outputTokens
	usage.TotalTokens += inputTokens + outputTokens
	usage.CacheReadTokens += cacheRead
	usage.CacheWriteTokens += cacheWrite

	// Calculate cost
	pricing := t.getPricingForModel(model)
	cost := t.calculateCost(inputTokens, outputTokens, cacheRead, cacheWrite, pricing)
	usage.EstimatedCost += cost

	// Update global usage
	t.globalUsage.InputTokens += inputTokens
	t.globalUsage.OutputTokens += outputTokens
	t.globalUsage.TotalTokens += inputTokens + outputTokens
	t.globalUsage.CacheReadTokens += cacheRead
	t.globalUsage.CacheWriteTokens += cacheWrite
	t.globalUsage.EstimatedCost += cost
}

// GetTokenUsage returns global token usage.
func (t *TokenTracker) GetTokenUsage() *TokenUsage {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return &TokenUsage{
		InputTokens:      t.globalUsage.InputTokens,
		OutputTokens:     t.globalUsage.OutputTokens,
		TotalTokens:      t.globalUsage.TotalTokens,
		CacheReadTokens:  t.globalUsage.CacheReadTokens,
		CacheWriteTokens: t.globalUsage.CacheWriteTokens,
		EstimatedCost:    t.globalUsage.EstimatedCost,
	}
}

// GetTokenUsageByModel returns token usage for a specific model.
func (t *TokenTracker) GetTokenUsageByModel(model string) *TokenUsage {
	t.mu.RLock()
	defer t.mu.RUnlock()

	usage, ok := t.modelUsage[model]
	if !ok {
		return nil
	}

	return &TokenUsage{
		InputTokens:      usage.InputTokens,
		OutputTokens:     usage.OutputTokens,
		TotalTokens:      usage.TotalTokens,
		CacheReadTokens:  usage.CacheReadTokens,
		CacheWriteTokens: usage.CacheWriteTokens,
		EstimatedCost:    usage.EstimatedCost,
	}
}

// GetAllModelUsage returns token usage for all models.
func (t *TokenTracker) GetAllModelUsage() []ModelTokenUsage {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]ModelTokenUsage, 0, len(t.modelUsage))
	for model, usage := range t.modelUsage {
		result = append(result, ModelTokenUsage{
			Model:         model,
			InputTokens:   usage.InputTokens,
			OutputTokens:  usage.OutputTokens,
			EstimatedCost: usage.EstimatedCost,
		})
	}

	return result
}

// GetPricing returns the pricing configuration.
func (t *TokenTracker) GetPricing() []TokenPricing {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]TokenPricing, len(t.pricing))
	copy(result, t.pricing)
	return result
}

// SetPricing updates the pricing configuration.
func (t *TokenTracker) SetPricing(pricing []TokenPricing) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.pricing = pricing
}

// GetPricingForModel returns the pricing for a specific model.
func (t *TokenTracker) GetPricingForModel(model string) *TokenPricing {
	t.mu.RLock()
	defer t.mu.RUnlock()

	pricing := t.getPricingForModel(model)
	return &pricing
}

// CalculateCost calculates the cost for given token usage.
func (t *TokenTracker) CalculateCost(model string, inputTokens, outputTokens, cacheRead, cacheWrite int64) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	pricing := t.getPricingForModel(model)
	return t.calculateCost(inputTokens, outputTokens, cacheRead, cacheWrite, pricing)
}

// Reset resets all token usage.
func (t *TokenTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.modelUsage = make(map[string]*TokenUsage)
	t.globalUsage = &TokenUsage{}
}

// getPricingForModel returns the pricing for a model (internal, no lock).
func (t *TokenTracker) getPricingForModel(model string) TokenPricing {
	normalizedModel := normalizeModelName(model)

	for _, p := range t.pricing {
		if matchPattern(normalizedModel, p.Pattern) {
			return p
		}
	}

	// Return default pricing
	return TokenPricing{
		Pattern:     "*",
		InputPrice:  1.00,
		OutputPrice: 5.00,
	}
}

// calculateCost calculates cost based on tokens and pricing (internal, no lock).
func (t *TokenTracker) calculateCost(inputTokens, outputTokens, cacheRead, cacheWrite int64, pricing TokenPricing) float64 {
	// Price is per million tokens
	inputCost := float64(inputTokens) / 1_000_000 * pricing.InputPrice
	outputCost := float64(outputTokens) / 1_000_000 * pricing.OutputPrice

	// Cache costs (if configured)
	cacheReadCost := float64(cacheRead) / 1_000_000 * pricing.CacheRead
	cacheWriteCost := float64(cacheWrite) / 1_000_000 * pricing.CacheWrite

	return inputCost + outputCost + cacheReadCost + cacheWriteCost
}

// normalizeModelName normalizes a model name for matching.
// It removes common prefixes like "claude-" and version/date suffixes.
func normalizeModelName(model string) string {
	model = strings.ToLower(model)

	// Remove "claude-" prefix
	model = strings.TrimPrefix(model, "claude-")

	// Remove version numbers and dates (e.g., "-4-5-20250929", "-3-5-20241022")
	// Pattern: -\d+(-\d+)*(-\d{8})?$
	re := regexp.MustCompile(`-\d+(-\d+)*(-\d{8})?$`)
	model = re.ReplaceAllString(model, "")

	return model
}

// matchPattern matches a model name against a pattern.
// Supports wildcards (*) at the end of the pattern.
func matchPattern(model, pattern string) bool {
	pattern = strings.ToLower(pattern)

	// Exact match
	if pattern == model {
		return true
	}

	// Wildcard match
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(model, prefix)
	}

	// Match all
	if pattern == "*" {
		return true
	}

	return false
}
