package session

import "testing"

func TestNormalizeTokenBudgetDefaultsToModernFallback(t *testing.T) {
	if got := NormalizeTokenBudget(0); got != DefaultContextTokenBudget {
		t.Fatalf("NormalizeTokenBudget(0) = %d, want %d", got, DefaultContextTokenBudget)
	}
}

func TestNormalizeTokenBudgetUpgradesLegacyDefault(t *testing.T) {
	if got := NormalizeTokenBudget(LegacyDefaultContextTokenBudget); got != DefaultContextTokenBudget {
		t.Fatalf("NormalizeTokenBudget(%d) = %d, want %d", LegacyDefaultContextTokenBudget, got, DefaultContextTokenBudget)
	}
}

func TestNormalizeTokenBudgetPreservesExplicitOverride(t *testing.T) {
	if got := NormalizeTokenBudget(64000); got != 64000 {
		t.Fatalf("NormalizeTokenBudget(64000) = %d, want 64000", got)
	}
}
