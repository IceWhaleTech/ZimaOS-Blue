package proxy

import (
	"sort"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

// TieredModel holds a model with its resolved tier and cost.
type TieredModel struct {
	ModelID    string    `json:"model_id"`
	ProviderID string   `json:"provider_id"`
	Tier       ModelTier `json:"tier"`
	TotalCost  float64   `json:"total_cost"` // InputPrice + OutputPrice per 1M tokens
}

// TierResolver dynamically assigns ModelTier to models based on pricing.
// It examines all available models and clusters them into tiers using
// price boundaries, ensuring at least 2 distinct tiers exist before
// enabling smart routing.
type TierResolver struct {
	mu       sync.RWMutex
	tiers    map[ModelTier][]*TieredModel // tier -> models sorted by cost (cheapest first)
	byModel  map[string]*TieredModel     // modelID -> tiered model (first match)
	resolved bool                         // true if ≥2 distinct tiers found
}

// NewTierResolver creates an empty TierResolver.
func NewTierResolver() *TierResolver {
	return &TierResolver{
		tiers:   make(map[ModelTier][]*TieredModel),
		byModel: make(map[string]*TieredModel),
	}
}

// Absolute price thresholds (USD per 1M tokens, input+output combined).
// Used when there are too few models for percentile-based clustering.
const (
	tierThresholdPremium  = 10.0 // > $10/M → premium
	tierThresholdStandard = 2.0  // $2-10/M → standard
	// < $2/M → economy
)

// Resolve analyzes available models and assigns tiers based on pricing.
// Returns true if at least 2 distinct tiers were found (smart routing viable).
// Called on provider change (same hook as candidateSnapshot rebuild).
func (tr *TierResolver) Resolve(models []*providerpool.Model) bool {
	type pricedModel struct {
		model     *providerpool.Model
		totalCost float64
	}

	var priced []pricedModel
	var freeModels []*providerpool.Model

	for _, m := range models {
		if !m.Enabled {
			continue
		}
		cost := m.InputPrice + m.OutputPrice

		// Try heuristic pricing if model has no price
		if cost == 0 {
			if matched := providerpool.MatchModelPricing(m.ID); matched != nil {
				cost = matched.InputPrice + matched.OutputPrice
			}
		}

		if cost > 0 {
			priced = append(priced, pricedModel{model: m, totalCost: cost})
		} else {
			freeModels = append(freeModels, m)
		}
	}

	// Sort by cost ascending
	sort.Slice(priced, func(i, j int) bool {
		return priced[i].totalCost < priced[j].totalCost
	})

	tiers := make(map[ModelTier][]*TieredModel)
	byModel := make(map[string]*TieredModel)

	addModel := func(m *providerpool.Model, tier ModelTier, cost float64) {
		tm := &TieredModel{
			ModelID:    m.ID,
			ProviderID: m.ProviderID,
			Tier:       tier,
			TotalCost:  cost,
		}
		tiers[tier] = append(tiers[tier], tm)
		if _, exists := byModel[m.ID]; !exists {
			byModel[m.ID] = tm
		}
	}

	// Assign tiers to priced models
	if n := len(priced); n > 0 {
		if n <= 2 {
			// Too few for percentiles — use absolute thresholds
			for _, pm := range priced {
				tier := classifyByAbsoluteThreshold(pm.totalCost)
				addModel(pm.model, tier, pm.totalCost)
			}
		} else {
			// Percentile-based clustering
			p50 := priced[n/2].totalCost
			p75 := priced[n*3/4].totalCost

			for _, pm := range priced {
				var tier ModelTier
				switch {
				case pm.totalCost >= p75:
					tier = TierPremium
				case pm.totalCost >= p50:
					tier = TierStandard
				default:
					tier = TierEconomy
				}
				addModel(pm.model, tier, pm.totalCost)
			}
		}
	}

	// Free/local models
	for _, m := range freeModels {
		addModel(m, TierFree, 0)
	}

	// Count distinct tiers
	distinctTiers := len(tiers)
	resolved := distinctTiers >= 2

	tr.mu.Lock()
	tr.tiers = tiers
	tr.byModel = byModel
	tr.resolved = resolved
	tr.mu.Unlock()

	return resolved
}

// classifyByAbsoluteThreshold assigns a tier based on fixed price boundaries.
func classifyByAbsoluteThreshold(totalCost float64) ModelTier {
	switch {
	case totalCost > tierThresholdPremium:
		return TierPremium
	case totalCost > tierThresholdStandard:
		return TierStandard
	default:
		return TierEconomy
	}
}

// IsEnabled returns true if at least 2 distinct tiers were found.
func (tr *TierResolver) IsEnabled() bool {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	return tr.resolved
}

// BestModelForTier returns the cheapest available model in the given tier.
// Falls back to adjacent tiers if the requested tier has no models:
// economy → free, standard → economy → free, premium → standard → economy.
// Returns "" if no model available.
func (tr *TierResolver) BestModelForTier(tier ModelTier) string {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	// Try exact tier first
	if models := tr.tiers[tier]; len(models) > 0 {
		return models[0].ModelID
	}

	// Fallback chain
	var fallbacks []ModelTier
	switch tier {
	case TierEconomy:
		fallbacks = []ModelTier{TierFree}
	case TierStandard:
		fallbacks = []ModelTier{TierEconomy, TierFree}
	case TierPremium:
		fallbacks = []ModelTier{TierStandard, TierEconomy}
	case TierFree:
		fallbacks = []ModelTier{TierEconomy}
	}

	for _, fb := range fallbacks {
		if models := tr.tiers[fb]; len(models) > 0 {
			return models[0].ModelID
		}
	}
	return ""
}

// ModelTierOf returns the tier for a specific model ID.
// Returns "" if the model is not known.
func (tr *TierResolver) ModelTierOf(modelID string) ModelTier {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	if tm, ok := tr.byModel[modelID]; ok {
		return tm.Tier
	}
	return ""
}

// Stats returns a summary of resolved tiers for diagnostics.
func (tr *TierResolver) Stats() map[string]interface{} {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	tierCounts := make(map[string]int, len(tr.tiers))
	tierModels := make(map[string][]string, len(tr.tiers))
	for tier, models := range tr.tiers {
		tierCounts[string(tier)] = len(models)
		names := make([]string, len(models))
		for i, m := range models {
			names[i] = m.ModelID
		}
		tierModels[string(tier)] = names
	}

	return map[string]interface{}{
		"enabled":      tr.resolved,
		"tier_counts":  tierCounts,
		"tier_models":  tierModels,
		"total_models": len(tr.byModel),
	}
}
