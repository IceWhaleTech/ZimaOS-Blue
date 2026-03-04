package proxy

import (
	"sort"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

// TieredModel holds a model with its resolved tier and cost.
type TieredModel struct {
	ModelID    string    `json:"model_id"`
	ProviderID string    `json:"provider_id"`
	Tier       ModelTier `json:"tier"`
	TotalCost  float64   `json:"total_cost"` // InputPrice + OutputPrice per 1M tokens
}

// TierResolver assigns models into two stable tiers:
// - TierLarge: all non-small models (typically big LLMs)
// - TierSmall: fixed built-in small-model allowlist
// Smart routing is enabled only when both tiers are present.
type TierResolver struct {
	mu       sync.RWMutex
	tiers    map[ModelTier][]*TieredModel // tier -> models sorted by selection preference
	byModel  map[string]*TieredModel      // modelID -> tiered model (first match)
	resolved bool                         // true if both large and small tiers exist
}

// Priority for built-in small models. Lower index = higher priority.
var builtinSmallModelPriority = []string{
	"qwen3.5-0.8b-onnx-q4",
	"qwen3.5-0.8b-q4kxl",
	"gpt-4o-mini",
	"o4-mini",
	"o3-mini",
	"o1-mini",
	"claude-3-haiku",
	"claude-haiku-4-5",
	"claude-3-5-haiku-20241022",
	"gemini-2.0-flash",
	"gemini-2.0-flash-thinking",
	"gemini-1.5-flash",
	"qwen-turbo",
	"llama-3.2-3b",
	"glm-4-flash",
	"glm-4-air",
	"amazon.nova-lite-v1:0",
}

var builtinSmallModelRank = func() map[string]int {
	rank := make(map[string]int, len(builtinSmallModelPriority))
	for i, id := range builtinSmallModelPriority {
		rank[id] = i
	}
	return rank
}()

var builtinSmallModelSet = buildBuiltinSmallModelSet()

func buildBuiltinSmallModelSet() map[string]struct{} {
	out := make(map[string]struct{}, len(builtinSmallModelPriority))
	for _, id := range builtinSmallModelPriority {
		norm := normalizeModelID(id)
		if norm != "" {
			out[norm] = struct{}{}
		}
	}

	for _, models := range providerpool.BuiltinModels() {
		for _, m := range models {
			id := normalizeModelID(m.ID)
			if id == "" {
				continue
			}
			if _, ok := builtinSmallModelRank[id]; ok {
				out[id] = struct{}{}
			}
		}
	}
	// Ensure fixed product small-model IDs are always treated as TierSmall.
	out["qwen3.5-0.8b-onnx-q4"] = struct{}{}
	out["qwen3.5-0.8b-q4kxl"] = struct{}{}
	return out
}

func normalizeModelID(modelID string) string {
	return strings.ToLower(strings.TrimSpace(modelID))
}

func resolveModelCost(m *providerpool.Model) float64 {
	cost := m.InputPrice + m.OutputPrice
	if cost == 0 {
		if matched := providerpool.MatchModelPricing(m.ID); matched != nil {
			cost = matched.InputPrice + matched.OutputPrice
		}
	}
	return cost
}

func isBuiltinSmallModel(m *providerpool.Model) bool {
	_, ok := builtinSmallModelSet[normalizeModelID(m.ID)]
	return ok
}

// NewTierResolver creates an empty TierResolver.
func NewTierResolver() *TierResolver {
	return &TierResolver{
		tiers:   make(map[ModelTier][]*TieredModel),
		byModel: make(map[string]*TieredModel),
	}
}

// Resolve analyzes available models and assigns large/small tiers deterministically.
// Returns true when both tiers are present (smart routing viable).
// Called on provider change (same hook as candidateSnapshot rebuild).
func (tr *TierResolver) Resolve(models []*providerpool.Model) bool {
	tiers := make(map[ModelTier][]*TieredModel)
	byModel := make(map[string]*TieredModel)

	addModel := func(m *providerpool.Model, tier ModelTier) {
		tm := &TieredModel{
			ModelID:    m.ID,
			ProviderID: m.ProviderID,
			Tier:       tier,
			TotalCost:  resolveModelCost(m),
		}
		tiers[tier] = append(tiers[tier], tm)
		if _, exists := byModel[m.ID]; !exists {
			byModel[m.ID] = tm
		}
	}

	for _, m := range models {
		if m == nil || !m.Enabled {
			continue
		}
		if isBuiltinSmallModel(m) {
			addModel(m, TierSmall)
			continue
		}
		addModel(m, TierLarge)
	}

	for tier, tierModels := range tiers {
		sort.Slice(tierModels, func(i, j int) bool {
			a := tierModels[i]
			b := tierModels[j]
			if tier == TierSmall {
				aRank, aOK := builtinSmallModelRank[normalizeModelID(a.ModelID)]
				bRank, bOK := builtinSmallModelRank[normalizeModelID(b.ModelID)]
				if aOK != bOK {
					return aOK
				}
				if aOK && bOK && aRank != bRank {
					return aRank < bRank
				}
			}
			if a.TotalCost == b.TotalCost {
				if a.ModelID == b.ModelID {
					return a.ProviderID < b.ProviderID
				}
				return a.ModelID < b.ModelID
			}
			return a.TotalCost < b.TotalCost
		})
		tiers[tier] = tierModels
	}

	resolved := len(tiers[TierLarge]) > 0 && len(tiers[TierSmall]) > 0

	tr.mu.Lock()
	tr.tiers = tiers
	tr.byModel = byModel
	tr.resolved = resolved
	tr.mu.Unlock()

	return resolved
}

// IsEnabled returns true when both large and small tiers are available.
func (tr *TierResolver) IsEnabled() bool {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	return tr.resolved
}

// BestModelForTier returns the top-ranked model in the requested tier.
// Returns "" if no model is available.
func (tr *TierResolver) BestModelForTier(tier ModelTier) string {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	canonical := normalizeModelTier(tier)
	if models := tr.tiers[canonical]; len(models) > 0 {
		return models[0].ModelID
	}

	switch canonical {
	case TierSmall:
		if models := tr.tiers[TierLarge]; len(models) > 0 {
			return models[0].ModelID
		}
	case TierLarge:
		if models := tr.tiers[TierSmall]; len(models) > 0 {
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
