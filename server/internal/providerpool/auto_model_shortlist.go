package providerpool

import (
	"sort"
	"strings"
)

const (
	autoModelDirectRoundRobinLimit = 8
	autoModelVeryLongListThreshold = 24
	autoModelFinalCandidateLimit   = 8
)

var autoModelPrimaryKeywords = []string{
	"claude",
	"opus",
	"sonnet",
	"haiku",
	"gpt",
	"glm",
	"m2.7",
	"m2.5",
	"k2.5",
}

var autoModelSecondaryKeywords = []string{
	"claude",
	"codex",
	"gpt",
}

func compareModelNameDescending(left, right string) int {
	leftNormalized := normalizeModelPreferenceID(left)
	rightNormalized := normalizeModelPreferenceID(right)
	switch {
	case leftNormalized > rightNormalized:
		return -1
	case leftNormalized < rightNormalized:
		return 1
	}

	leftRaw := strings.ToLower(strings.TrimSpace(left))
	rightRaw := strings.ToLower(strings.TrimSpace(right))
	switch {
	case leftRaw > rightRaw:
		return -1
	case leftRaw < rightRaw:
		return 1
	default:
		return 0
	}
}

func sortModelsByNameDescending(models []*Model) []*Model {
	out := append([]*Model(nil), models...)
	sort.SliceStable(out, func(i, j int) bool {
		return compareModelNameDescending(preferredModelID(out[i]), preferredModelID(out[j])) < 0
	})
	return out
}

func shortlistModelsForAuto(models []*Model) []*Model {
	ordered := sortModelsByNameDescending(models)
	if len(ordered) <= autoModelDirectRoundRobinLimit {
		return ordered
	}

	primary := filterModelsForAutoKeywordsOrFallback(ordered, autoModelPrimaryKeywords, ordered)
	if len(primary) <= autoModelVeryLongListThreshold {
		return primary
	}

	secondary := filterModelsForAutoKeywordsOrFallback(ordered, autoModelSecondaryKeywords, primary)
	if len(secondary) <= autoModelVeryLongListThreshold {
		return secondary
	}

	return append([]*Model(nil), secondary[:autoModelFinalCandidateLimit]...)
}

func filterModelsForAutoKeywordsOrFallback(models []*Model, keywords []string, fallback []*Model) []*Model {
	filtered := make([]*Model, 0, len(models))
	for _, model := range models {
		if modelMatchesAutoKeywords(preferredModelID(model), keywords) {
			filtered = append(filtered, model)
		}
	}
	if len(filtered) == 0 {
		return fallback
	}
	return filtered
}

func shortlistRouteCandidatesForAuto(candidates []*RouteCandidate) []*RouteCandidate {
	ordered := append([]*RouteCandidate(nil), candidates...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return compareModelNameDescending(preferredRouteCandidateModelID(ordered[i]), preferredRouteCandidateModelID(ordered[j])) < 0
	})
	if len(ordered) <= autoModelDirectRoundRobinLimit {
		return ordered
	}

	primary := filterRouteCandidatesForAutoKeywordsOrFallback(ordered, autoModelPrimaryKeywords, ordered)
	if len(primary) <= autoModelVeryLongListThreshold {
		return primary
	}

	secondary := filterRouteCandidatesForAutoKeywordsOrFallback(ordered, autoModelSecondaryKeywords, primary)
	if len(secondary) <= autoModelVeryLongListThreshold {
		return secondary
	}

	return append([]*RouteCandidate(nil), secondary[:autoModelFinalCandidateLimit]...)
}

func filterRouteCandidatesForAutoKeywordsOrFallback(candidates []*RouteCandidate, keywords []string, fallback []*RouteCandidate) []*RouteCandidate {
	filtered := make([]*RouteCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if modelMatchesAutoKeywords(preferredRouteCandidateModelID(candidate), keywords) {
			filtered = append(filtered, candidate)
		}
	}
	if len(filtered) == 0 {
		return fallback
	}
	return filtered
}

func preferredRouteCandidateModelID(candidate *RouteCandidate) string {
	if candidate == nil {
		return ""
	}
	return preferredModelID(candidate.Model)
}

func modelMatchesAutoKeywords(modelID string, keywords []string) bool {
	normalized := normalizeModelPreferenceID(modelID)
	if normalized == "" {
		return false
	}
	for _, keyword := range keywords {
		if strings.Contains(normalized, keyword) {
			return true
		}
	}
	return false
}
