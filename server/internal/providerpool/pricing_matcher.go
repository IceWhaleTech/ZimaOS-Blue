package providerpool

import (
	"regexp"
	"strings"
)

// Pre-compiled regexps to avoid re-compiling on every call.
var (
	datePatternRe    = regexp.MustCompile(`-\d{4}[-]?\d{2}[-]?\d{2}$`)
	delimiterSplitRe = regexp.MustCompile(`[-_.:@/]`)
	pureDigitsRe     = regexp.MustCompile(`^\d+$`)
)

// BuiltinModelPricing contains known model pricing (per 1M tokens, USD)
// Updated: 2025-01
var BuiltinModelPricing = map[string]*ModelPricing{
	// ===== Anthropic Claude =====
	// Claude 4.5
	"claude-opus-4-5":   {ModelID: "claude-opus-4-5", InputPrice: 15.0, OutputPrice: 75.0, CachePrice: 1.875},
	"claude-sonnet-4-5": {ModelID: "claude-sonnet-4-5", InputPrice: 3.0, OutputPrice: 15.0, CachePrice: 0.375},
	// Claude 4
	"claude-sonnet-4":   {ModelID: "claude-sonnet-4", InputPrice: 3.0, OutputPrice: 15.0, CachePrice: 0.375},
	"claude-opus-4":     {ModelID: "claude-opus-4", InputPrice: 15.0, OutputPrice: 75.0, CachePrice: 1.875},
	// Claude 3.5
	"claude-3-5-sonnet": {ModelID: "claude-3-5-sonnet", InputPrice: 3.0, OutputPrice: 15.0, CachePrice: 0.375},
	"claude-3-5-haiku":  {ModelID: "claude-3-5-haiku", InputPrice: 0.8, OutputPrice: 4.0, CachePrice: 0.1},
	// Claude 3
	"claude-3-opus":   {ModelID: "claude-3-opus", InputPrice: 15.0, OutputPrice: 75.0, CachePrice: 1.875},
	"claude-3-sonnet": {ModelID: "claude-3-sonnet", InputPrice: 3.0, OutputPrice: 15.0, CachePrice: 0.375},
	"claude-3-haiku":  {ModelID: "claude-3-haiku", InputPrice: 0.25, OutputPrice: 1.25, CachePrice: 0.03},

	// ===== OpenAI GPT =====
	// GPT-4o
	"gpt-4o":      {ModelID: "gpt-4o", InputPrice: 2.5, OutputPrice: 10.0, CachePrice: 1.25},
	"gpt-4o-mini": {ModelID: "gpt-4o-mini", InputPrice: 0.15, OutputPrice: 0.6, CachePrice: 0.075},
	// GPT-4.1
	"gpt-4.1":      {ModelID: "gpt-4.1", InputPrice: 2.0, OutputPrice: 8.0, CachePrice: 0.5},
	"gpt-4.1-mini": {ModelID: "gpt-4.1-mini", InputPrice: 0.4, OutputPrice: 1.6, CachePrice: 0.1},
	"gpt-4.1-nano": {ModelID: "gpt-4.1-nano", InputPrice: 0.1, OutputPrice: 0.4, CachePrice: 0.025},
	// GPT-4 Turbo
	"gpt-4-turbo": {ModelID: "gpt-4-turbo", InputPrice: 10.0, OutputPrice: 30.0, CachePrice: 5.0},
	// GPT-4
	"gpt-4": {ModelID: "gpt-4", InputPrice: 30.0, OutputPrice: 60.0, CachePrice: 15.0},
	// GPT-3.5
	"gpt-3.5-turbo": {ModelID: "gpt-3.5-turbo", InputPrice: 0.5, OutputPrice: 1.5, CachePrice: 0.25},
	// o1/o3 reasoning
	"o1":         {ModelID: "o1", InputPrice: 15.0, OutputPrice: 60.0, CachePrice: 7.5},
	"o1-mini":    {ModelID: "o1-mini", InputPrice: 1.1, OutputPrice: 4.4, CachePrice: 0.55},
	"o1-preview": {ModelID: "o1-preview", InputPrice: 15.0, OutputPrice: 60.0, CachePrice: 7.5},
	"o3-mini":    {ModelID: "o3-mini", InputPrice: 1.1, OutputPrice: 4.4, CachePrice: 0.55},

	// ===== Google Gemini =====
	"gemini-2.0-flash":       {ModelID: "gemini-2.0-flash", InputPrice: 0.1, OutputPrice: 0.4, CachePrice: 0.025},
	"gemini-2.0-flash-lite":  {ModelID: "gemini-2.0-flash-lite", InputPrice: 0.075, OutputPrice: 0.3, CachePrice: 0.01875},
	"gemini-1.5-pro":         {ModelID: "gemini-1.5-pro", InputPrice: 1.25, OutputPrice: 5.0, CachePrice: 0.3125},
	"gemini-1.5-flash":       {ModelID: "gemini-1.5-flash", InputPrice: 0.075, OutputPrice: 0.3, CachePrice: 0.01875},
	"gemini-1.5-flash-8b":    {ModelID: "gemini-1.5-flash-8b", InputPrice: 0.0375, OutputPrice: 0.15, CachePrice: 0.01},
	"gemini-2.5-pro-preview": {ModelID: "gemini-2.5-pro-preview", InputPrice: 1.25, OutputPrice: 10.0, CachePrice: 0.3125},
	"gemini-2.5-flash":       {ModelID: "gemini-2.5-flash", InputPrice: 0.15, OutputPrice: 0.6, CachePrice: 0.0375},

	// ===== DeepSeek =====
	"deepseek-chat":     {ModelID: "deepseek-chat", InputPrice: 0.27, OutputPrice: 1.1, CachePrice: 0.07},
	"deepseek-reasoner": {ModelID: "deepseek-reasoner", InputPrice: 0.55, OutputPrice: 2.19, CachePrice: 0.14},

	// ===== Mistral =====
	"mistral-large":  {ModelID: "mistral-large", InputPrice: 2.0, OutputPrice: 6.0, CachePrice: 0.5},
	"mistral-medium": {ModelID: "mistral-medium", InputPrice: 2.7, OutputPrice: 8.1, CachePrice: 0.675},
	"mistral-small":  {ModelID: "mistral-small", InputPrice: 0.2, OutputPrice: 0.6, CachePrice: 0.05},
	"codestral":      {ModelID: "codestral", InputPrice: 0.3, OutputPrice: 0.9, CachePrice: 0.075},
	"pixtral-large":  {ModelID: "pixtral-large", InputPrice: 2.0, OutputPrice: 6.0, CachePrice: 0.5},

	// ===== Qwen (Alibaba) =====
	"qwen-max":        {ModelID: "qwen-max", InputPrice: 1.6, OutputPrice: 6.4, CachePrice: 0.4},
	"qwen-plus":       {ModelID: "qwen-plus", InputPrice: 0.8, OutputPrice: 2.0, CachePrice: 0.2},
	"qwen-turbo":      {ModelID: "qwen-turbo", InputPrice: 0.3, OutputPrice: 0.6, CachePrice: 0.075},
	"qwen2.5-coder":   {ModelID: "qwen2.5-coder", InputPrice: 0.8, OutputPrice: 2.0, CachePrice: 0.2},
	"qwen-coder-plus": {ModelID: "qwen-coder-plus", InputPrice: 0.8, OutputPrice: 2.0, CachePrice: 0.2},

	// ===== Cohere =====
	"command-r-plus": {ModelID: "command-r-plus", InputPrice: 2.5, OutputPrice: 10.0, CachePrice: 0.625},
	"command-r":      {ModelID: "command-r", InputPrice: 0.15, OutputPrice: 0.6, CachePrice: 0.0375},

	// ===== xAI Grok =====
	"grok-2":      {ModelID: "grok-2", InputPrice: 2.0, OutputPrice: 10.0, CachePrice: 0.5},
	"grok-2-mini": {ModelID: "grok-2-mini", InputPrice: 0.2, OutputPrice: 1.0, CachePrice: 0.05},
	"grok-3":      {ModelID: "grok-3", InputPrice: 3.0, OutputPrice: 15.0, CachePrice: 0.75},
	"grok-3-mini": {ModelID: "grok-3-mini", InputPrice: 0.3, OutputPrice: 0.5, CachePrice: 0.075},

	// ===== Meta Llama =====
	"llama-3.3-70b":  {ModelID: "llama-3.3-70b", InputPrice: 0.8, OutputPrice: 0.8, CachePrice: 0.2},
	"llama-3.2-90b":  {ModelID: "llama-3.2-90b", InputPrice: 0.9, OutputPrice: 0.9, CachePrice: 0.225},
	"llama-3.1-405b": {ModelID: "llama-3.1-405b", InputPrice: 3.0, OutputPrice: 3.0, CachePrice: 0.75},
	"llama-3.1-70b":  {ModelID: "llama-3.1-70b", InputPrice: 0.8, OutputPrice: 0.8, CachePrice: 0.2},
	"llama-3.1-8b":   {ModelID: "llama-3.1-8b", InputPrice: 0.1, OutputPrice: 0.1, CachePrice: 0.025},
}

// ModelFamily represents a family of models with similar pricing tiers
type ModelFamily struct {
	Name     string   // Family name (e.g., "claude", "gpt")
	Keywords []string // Keywords to match (e.g., ["claude", "anthropic"])
	Tiers    map[string]*ModelPricing // Tier name -> pricing
}

// builtinModelFamilies defines pricing tiers for model families
var builtinModelFamilies = []ModelFamily{
	{
		Name:     "claude",
		Keywords: []string{"claude", "anthropic"},
		Tiers: map[string]*ModelPricing{
			"opus":   {InputPrice: 15.0, OutputPrice: 75.0, CachePrice: 1.875},
			"sonnet": {InputPrice: 3.0, OutputPrice: 15.0, CachePrice: 0.375},
			"haiku":  {InputPrice: 0.8, OutputPrice: 4.0, CachePrice: 0.1},
		},
	},
	{
		Name:     "gpt",
		Keywords: []string{"gpt", "openai", "chatgpt"},
		Tiers: map[string]*ModelPricing{
			"4o":      {InputPrice: 2.5, OutputPrice: 10.0, CachePrice: 1.25},
			"4o-mini": {InputPrice: 0.15, OutputPrice: 0.6, CachePrice: 0.075},
			"4":       {InputPrice: 30.0, OutputPrice: 60.0, CachePrice: 15.0},
			"4-turbo": {InputPrice: 10.0, OutputPrice: 30.0, CachePrice: 5.0},
			"3.5":     {InputPrice: 0.5, OutputPrice: 1.5, CachePrice: 0.25},
			"mini":    {InputPrice: 0.15, OutputPrice: 0.6, CachePrice: 0.075},
			"nano":    {InputPrice: 0.1, OutputPrice: 0.4, CachePrice: 0.025},
		},
	},
	{
		Name:     "gemini",
		Keywords: []string{"gemini", "google", "bard"},
		Tiers: map[string]*ModelPricing{
			"pro":        {InputPrice: 1.25, OutputPrice: 5.0, CachePrice: 0.3125},
			"flash":      {InputPrice: 0.075, OutputPrice: 0.3, CachePrice: 0.01875},
			"flash-lite": {InputPrice: 0.075, OutputPrice: 0.3, CachePrice: 0.01875},
			"ultra":      {InputPrice: 5.0, OutputPrice: 15.0, CachePrice: 1.25},
		},
	},
	{
		Name:     "deepseek",
		Keywords: []string{"deepseek"},
		Tiers: map[string]*ModelPricing{
			"chat":     {InputPrice: 0.27, OutputPrice: 1.1, CachePrice: 0.07},
			"reasoner": {InputPrice: 0.55, OutputPrice: 2.19, CachePrice: 0.14},
			"coder":    {InputPrice: 0.27, OutputPrice: 1.1, CachePrice: 0.07},
		},
	},
	{
		Name:     "mistral",
		Keywords: []string{"mistral", "mixtral"},
		Tiers: map[string]*ModelPricing{
			"large":  {InputPrice: 2.0, OutputPrice: 6.0, CachePrice: 0.5},
			"medium": {InputPrice: 2.7, OutputPrice: 8.1, CachePrice: 0.675},
			"small":  {InputPrice: 0.2, OutputPrice: 0.6, CachePrice: 0.05},
		},
	},
	{
		Name:     "qwen",
		Keywords: []string{"qwen", "alibaba", "tongyi"},
		Tiers: map[string]*ModelPricing{
			"max":   {InputPrice: 1.6, OutputPrice: 6.4, CachePrice: 0.4},
			"plus":  {InputPrice: 0.8, OutputPrice: 2.0, CachePrice: 0.2},
			"turbo": {InputPrice: 0.3, OutputPrice: 0.6, CachePrice: 0.075},
		},
	},
	{
		Name:     "grok",
		Keywords: []string{"grok", "xai"},
		Tiers: map[string]*ModelPricing{
			"2":      {InputPrice: 2.0, OutputPrice: 10.0, CachePrice: 0.5},
			"2-mini": {InputPrice: 0.2, OutputPrice: 1.0, CachePrice: 0.05},
			"3":      {InputPrice: 3.0, OutputPrice: 15.0, CachePrice: 0.75},
			"3-mini": {InputPrice: 0.3, OutputPrice: 0.5, CachePrice: 0.075},
			"mini":   {InputPrice: 0.2, OutputPrice: 1.0, CachePrice: 0.05},
		},
	},
	{
		Name:     "llama",
		Keywords: []string{"llama", "meta"},
		Tiers: map[string]*ModelPricing{
			"405b": {InputPrice: 3.0, OutputPrice: 3.0, CachePrice: 0.75},
			"70b":  {InputPrice: 0.8, OutputPrice: 0.8, CachePrice: 0.2},
			"8b":   {InputPrice: 0.1, OutputPrice: 0.1, CachePrice: 0.025},
		},
	},
	{
		Name:     "o1",
		Keywords: []string{"o1", "o3"},
		Tiers: map[string]*ModelPricing{
			"":        {InputPrice: 15.0, OutputPrice: 60.0, CachePrice: 7.5},
			"mini":    {InputPrice: 1.1, OutputPrice: 4.4, CachePrice: 0.55},
			"preview": {InputPrice: 15.0, OutputPrice: 60.0, CachePrice: 7.5},
		},
	},
}

// normalizeModelID normalizes a model ID for matching
// e.g., "claude-opus-4-5-20251101" -> "claude-opus-4-5"
//       "gpt-4o-2024-08-06" -> "gpt-4o"
func normalizeModelID(modelID string) string {
	normalized := strings.ToLower(modelID)

	// Remove date suffixes (e.g., "-20251101", "-2024-08-06")
	normalized = datePatternRe.ReplaceAllString(normalized, "")

	// Remove version suffixes like "-latest", "-preview"
	normalized = strings.TrimSuffix(normalized, "-latest")

	return normalized
}

// extractKeywords extracts keywords from a model ID
func extractKeywords(modelID string) []string {
	normalized := strings.ToLower(modelID)

	// Split by common delimiters
	parts := delimiterSplitRe.Split(normalized, -1)

	// Filter out empty strings and pure numbers
	keywords := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" && !pureDigitsRe.MatchString(part) {
			keywords = append(keywords, part)
		}
	}

	return keywords
}

// matchModelFamily finds the best matching model family for a model ID
func matchModelFamily(modelID string) (*ModelFamily, string) {
	normalized := normalizeModelID(modelID)
	keywords := extractKeywords(normalized)

	var bestFamily *ModelFamily
	var bestTier string
	bestFamilyScore := 0

	for i := range builtinModelFamilies {
		family := &builtinModelFamilies[i]

		// Check if any family keyword matches
		familyScore := 0
		for _, fk := range family.Keywords {
			// Check if family keyword appears in the normalized model ID
			if strings.Contains(normalized, fk) {
				familyScore += len(fk) * 10 // Weight by keyword length
			}
			// Also check extracted keywords
			for _, mk := range keywords {
				if mk == fk {
					familyScore += len(fk) * 5
				}
			}
		}

		if familyScore == 0 {
			continue
		}

		// Find the best matching tier for this family
		tierName := ""
		tierScore := 0

		for tn := range family.Tiers {
			if tn == "" {
				continue
			}

			score := 0
			tierKeywords := extractKeywords(tn)

			// Check if tier keywords appear in model ID
			for _, tk := range tierKeywords {
				if strings.Contains(normalized, tk) {
					score += len(tk) * 10
				}
				for _, mk := range keywords {
					if mk == tk {
						score += len(tk) * 5
					}
				}
			}

			// Penalize if tier has extra keywords not in model ID
			// This prevents "4o-mini" from matching "gpt-4o"
			for _, tk := range tierKeywords {
				if !strings.Contains(normalized, tk) {
					score -= len(tk) * 8
				}
			}

			if score > tierScore {
				tierScore = score
				tierName = tn
			}
		}

		// If no tier matched but family matched, use default tier if exists
		if tierName == "" {
			if _, exists := family.Tiers[""]; exists {
				tierName = ""
			} else {
				// Use first tier as fallback
				for tn := range family.Tiers {
					tierName = tn
					break
				}
			}
		}

		// Update best match if this family has higher score
		totalScore := familyScore + tierScore
		if totalScore > bestFamilyScore {
			bestFamilyScore = totalScore
			bestFamily = family
			bestTier = tierName
		}
	}

	return bestFamily, bestTier
}

// MatchModelPricing attempts to match a model ID to known pricing using heuristics
// Returns nil if no match found
func MatchModelPricing(modelID string) *ModelPricing {
	normalized := normalizeModelID(modelID)

	// 1. Try exact match in builtin pricing
	if pricing, exists := BuiltinModelPricing[normalized]; exists {
		result := *pricing
		result.ModelID = modelID
		return &result
	}

	// 2. Try partial match in builtin pricing (prefix match)
	for key, pricing := range BuiltinModelPricing {
		if strings.HasPrefix(normalized, key) || strings.HasPrefix(key, normalized) {
			result := *pricing
			result.ModelID = modelID
			return &result
		}
	}

	// 3. Try family + tier matching
	if family, tier := matchModelFamily(modelID); family != nil {
		if tierPricing, exists := family.Tiers[tier]; exists {
			return &ModelPricing{
				ModelID:     modelID,
				InputPrice:  tierPricing.InputPrice,
				OutputPrice: tierPricing.OutputPrice,
				CachePrice:  tierPricing.CachePrice,
				IsCustom:    false,
			}
		}
	}

	return nil
}

// GetModelPricingWithHeuristics returns pricing for a model, using heuristic matching
// if no exact match is found. This is the enhanced version of GetModelPricing.
func (pm *PricingManager) GetModelPricingWithHeuristics(modelID, providerID string) *ModelPricing {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// 1. Check provider-specific custom pricing
	if providerID != "" {
		key := providerID + ":" + modelID
		if pricing, exists := pm.config.CustomPricing[key]; exists {
			return pricing
		}
	}

	// 2. Check model-specific custom pricing (any provider)
	if pricing, exists := pm.config.CustomPricing[modelID]; exists {
		return pricing
	}

	// 3. Try heuristic matching against builtin pricing
	if matched := MatchModelPricing(modelID); matched != nil {
		matched.ProviderID = providerID
		return matched
	}

	// 4. Return default pricing for truly unknown models
	return &ModelPricing{
		ModelID:     modelID,
		ProviderID:  providerID,
		InputPrice:  pm.config.DefaultInputPrice,
		OutputPrice: pm.config.DefaultOutputPrice,
		CachePrice:  pm.config.DefaultCachePrice,
		IsCustom:    false,
		UpdatedAt:   pm.config.UpdatedAt,
	}
}
