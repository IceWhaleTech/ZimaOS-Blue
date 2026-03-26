package providerpool

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type modelFamilyHint int

const (
	modelFamilyDefault modelFamilyHint = iota
	modelFamilyClaude
	modelFamilyCodex
)

const preferredModelPriorityThreshold = 100

var modelVersionNumberRe = regexp.MustCompile(`\d+`)

func shouldPrioritizePreferredModels(total int) bool {
	return total >= preferredModelPriorityThreshold
}

func normalizeModelPreferenceID(modelID string) string {
	id := strings.ToLower(strings.TrimSpace(modelID))
	if id == "" {
		return ""
	}
	if slash := strings.LastIndex(id, "/"); slash >= 0 && slash < len(id)-1 {
		id = id[slash+1:]
	}
	return id
}

func preferredModelPrefixRank(modelID string) int {
	id := normalizeModelPreferenceID(modelID)
	switch {
	case strings.HasPrefix(id, "claude"), strings.HasPrefix(id, "gpt"):
		return 0
	default:
		return 1
	}
}

func inferModelFamily(modelID string) modelFamilyHint {
	id := normalizeModelPreferenceID(modelID)
	if id == "" {
		return modelFamilyDefault
	}
	if strings.Contains(id, "codex") {
		return modelFamilyCodex
	}
	if strings.Contains(id, "claude") ||
		strings.Contains(id, "sonnet") ||
		strings.Contains(id, "opus") ||
		strings.Contains(id, "haiku") {
		return modelFamilyClaude
	}
	return modelFamilyDefault
}

func PreferredAPIFormatsForModel(modelID string) []APIFormat {
	preferred := preferredAPIFormatsForModel(modelID)
	return append([]APIFormat(nil), preferred...)
}

func preferredAPIFormatsForModel(modelID string) []APIFormat {
	var preferred []APIFormat
	switch inferModelFamily(modelID) {
	case modelFamilyClaude:
		preferred = []APIFormat{APIFormatAnthropic, APIFormatOpenAI, APIFormatResponses}
	case modelFamilyCodex:
		preferred = []APIFormat{APIFormatResponses, APIFormatOpenAI, APIFormatAnthropic}
	default:
		preferred = []APIFormat{APIFormatOpenAI, APIFormatAnthropic, APIFormatResponses}
	}

	if ResponsesIntegrationEnabled() {
		return preferred
	}

	filtered := preferred[:0]
	for _, format := range preferred {
		if format == APIFormatResponses {
			continue
		}
		filtered = append(filtered, format)
	}
	return filtered
}

func recommendedAPIFormatForModel(modelID string, detectedFormat APIFormat, anthropicReachable, openAIReachable, responsesReachable, responsesOnly bool) APIFormat {
	responsesEnabled := ResponsesIntegrationEnabled()
	if !responsesEnabled {
		responsesReachable = false
		responsesOnly = false
		if detectedFormat == APIFormatResponses {
			detectedFormat = ""
		}
	}

	if responsesOnly {
		return APIFormatResponses
	}

	reachable := map[APIFormat]bool{
		APIFormatAnthropic: anthropicReachable,
		APIFormatOpenAI:    openAIReachable,
		APIFormatResponses: responsesReachable,
	}

	for _, format := range preferredAPIFormatsForModel(modelID) {
		if reachable[format] {
			return format
		}
	}

	if detectedFormat != "" && reachable[detectedFormat] {
		return detectedFormat
	}
	if openAIReachable {
		return APIFormatOpenAI
	}
	if anthropicReachable {
		return APIFormatAnthropic
	}
	if responsesEnabled && responsesReachable {
		return APIFormatResponses
	}
	if detectedFormat != "" {
		return detectedFormat
	}
	return APIFormatOpenAI
}

func providerFormatAffinityScore(modelID string, provider *Provider) int {
	if strings.TrimSpace(modelID) == "" || provider == nil {
		return 0
	}

	plan := ResolveAPIFormatPlan(FormatResolutionRequest{
		Provider:       provider,
		ModelID:        modelID,
		DetectedFormat: provider.DetectedFormat,
	})
	format := plan.SelectedFormat
	if len(plan.CandidateFormats) > 0 {
		format = plan.CandidateFormats[0]
	}
	nativeFormat := firstNonEmptyFormat(provider.APIFormat, provider.DetectedFormat, canonicalAPIFormatForProvider(provider))

	score := 0
	for index, preferred := range preferredAPIFormatsForModel(modelID) {
		if format == preferred {
			score = (len(preferredAPIFormatsForModel(modelID)) - index) * 100
			break
		}
	}

	providerID := strings.ToLower(strings.TrimSpace(provider.ID))
	switch inferModelFamily(modelID) {
	case modelFamilyClaude:
		if nativeFormat == APIFormatAnthropic || strings.Contains(providerID, "anthropic") {
			score += 25
		}
	case modelFamilyCodex:
		if ResponsesIntegrationEnabled() && (nativeFormat == APIFormatResponses || strings.Contains(providerID, "codex")) {
			score += 25
		}
	default:
		return 0
	}

	return score
}

func sortModelIDsByPreference(modelIDs []string) []string {
	out := append([]string(nil), modelIDs...)
	if !shouldPrioritizePreferredModels(len(out)) {
		return out
	}
	sort.SliceStable(out, func(i, j int) bool {
		return compareModelPreference(out[i], out[j]) < 0
	})
	return out
}

func sortModelsByPreference(models []*Model) []*Model {
	out := append([]*Model(nil), models...)
	if !shouldPrioritizePreferredModels(len(out)) {
		return out
	}
	sort.SliceStable(out, func(i, j int) bool {
		return compareModelPreference(preferredModelID(out[i]), preferredModelID(out[j])) < 0
	})
	return out
}

func preferredModelID(model *Model) string {
	if model == nil {
		return ""
	}
	if id := strings.TrimSpace(model.ID); id != "" {
		return id
	}
	return strings.TrimSpace(model.Name)
}

func compareModelPreference(left, right string) int {
	leftPrefixRank := preferredModelPrefixRank(left)
	rightPrefixRank := preferredModelPrefixRank(right)
	if leftPrefixRank != rightPrefixRank {
		if leftPrefixRank < rightPrefixRank {
			return -1
		}
		return 1
	}

	leftScore := modelIntelligenceScore(left)
	rightScore := modelIntelligenceScore(right)
	if leftScore != rightScore {
		if leftScore > rightScore {
			return -1
		}
		return 1
	}

	leftNormalized := strings.ToLower(strings.TrimSpace(left))
	rightNormalized := strings.ToLower(strings.TrimSpace(right))
	if leftNormalized < rightNormalized {
		return -1
	}
	if leftNormalized > rightNormalized {
		return 1
	}
	return 0
}

func modelVersionScore(modelID string) int {
	matches := modelVersionNumberRe.FindAllString(normalizeModelPreferenceID(modelID), -1)
	if len(matches) == 0 {
		return 0
	}

	weights := []int{100, 10, 1}
	total := 0
	weightIndex := 0
	for _, match := range matches {
		if weightIndex >= len(weights) {
			break
		}
		value, err := strconv.Atoi(match)
		if err != nil {
			continue
		}
		if value >= 1000 {
			continue
		}
		total += value * weights[weightIndex]
		weightIndex++
	}
	return total
}

func modelIntelligenceScore(modelID string) int {
	id := normalizeModelPreferenceID(modelID)
	if id == "" {
		return 0
	}

	score := modelVersionScore(id)

	boosts := []struct {
		key   string
		score int
	}{
		{"gpt-5", 520},
		{"o3", 480},
		{"o1", 430},
		{"opus", 420},
		{"reasoner", 380},
		{"thinking", 360},
		{"sonnet", 320},
		{"pro", 220},
		{"max", 210},
		{"ultra", 180},
		{"plus", 120},
		{"codex", 95},
		{"coder", 60},
		{"codestral", 60},
		{"turbo", 50},
		{"haiku", 40},
	}
	for _, item := range boosts {
		if strings.Contains(id, item.key) {
			score += item.score
		}
	}

	penalties := []struct {
		key   string
		score int
	}{
		{"mini", -180},
		{"nano", -200},
		{"lite", -150},
		{"flash", -130},
		{"spark", -120},
		{"small", -100},
		{"tiny", -100},
	}
	for _, item := range penalties {
		if strings.Contains(id, item.key) {
			score += item.score
		}
	}

	return score
}
