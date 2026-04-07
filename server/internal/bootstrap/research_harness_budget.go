package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"

func deepResearchBudgetMaxSources(budget *deepresearch.Budget) int {
	if budget == nil {
		return 0
	}
	return budget.MaxSources
}

func deepResearchBudgetMaxSeconds(budget *deepresearch.Budget) int {
	if budget == nil {
		return 0
	}
	return budget.MaxSeconds
}
