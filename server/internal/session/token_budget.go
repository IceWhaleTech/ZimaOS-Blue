package session

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/claudecode"

const (
	// LegacyDefaultContextTokenBudget was the historical session default before
	// model-aware budgeting was introduced.
	LegacyDefaultContextTokenBudget = 8000
	// DefaultContextTokenBudget is the modern fallback when no explicit session
	// budget override is configured.
	DefaultContextTokenBudget = claudecode.DefaultContextTokens
)

// NormalizeTokenBudget converts config/session budget values into a realistic
// fallback budget for model-agnostic session contexts.
func NormalizeTokenBudget(maxTokens int) int {
	switch {
	case maxTokens <= 0:
		return DefaultContextTokenBudget
	case maxTokens == LegacyDefaultContextTokenBudget:
		return DefaultContextTokenBudget
	default:
		return maxTokens
	}
}
