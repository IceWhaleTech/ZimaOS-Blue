package providerpool

import (
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type modelFamilyHint int

const (
	modelFamilyDefault modelFamilyHint = iota
	modelFamilyClaude
	modelFamilyResponsesNative
)

const preferredModelPriorityThreshold = 100

var (
	modelVersionNumberRe     *regexp.Regexp
	modelVersionNumberReOnce sync.Once
)

func modelVersionRegexp() *regexp.Regexp {
	modelVersionNumberReOnce.Do(func() {
		modelVersionNumberRe = regexp.MustCompile(`\d+`)
	})
	return modelVersionNumberRe
}

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
	if isResponsesNativeModelID(id) {
		return modelFamilyResponsesNative
	}
	if strings.Contains(id, "claude") ||
		strings.Contains(id, "sonnet") ||
		strings.Contains(id, "opus") ||
		strings.Contains(id, "haiku") {
		return modelFamilyClaude
	}
	return modelFamilyDefault
}

func isResponsesNativeModelID(modelID string) bool {
	id := normalizeModelPreferenceID(modelID)
	if id == "" {
		return false
	}
	return strings.Contains(id, "responses") ||
		strings.Contains(id, "codex") ||
		id == "gpt-5.4-pro" ||
		strings.HasPrefix(id, "gpt-5.4-pro-")
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
	case modelFamilyResponsesNative:
		preferred = []APIFormat{APIFormatResponses, APIFormatOpenAI, APIFormatAnthropic}
	default:
		preferred = []APIFormat{APIFormatOpenAI, APIFormatAnthropic, APIFormatResponses}
	}

	return preferred
}

func preferredAPIFormatsForProviderModel(provider *Provider, modelID string) []APIFormat {
	if shouldPreferResponsesForUnknownOpenAIModel(provider, modelID) {
		return []APIFormat{APIFormatResponses, APIFormatOpenAI, APIFormatAnthropic}
	}
	return preferredAPIFormatsForModel(modelID)
}

func shouldPreferResponsesForUnknownOpenAIModel(provider *Provider, modelID string) bool {
	normalized := normalizeModelPreferenceID(modelID)
	if normalized == "" {
		return false
	}
	if isResponsesNativeModelID(normalized) {
		return false
	}
	if !isOpenAIFirstPartyProvider(provider) {
		return false
	}
	return !isKnownOpenAIBuiltinModel(normalized)
}

func isKnownOpenAIBuiltinModel(modelID string) bool {
	normalized := normalizeModelPreferenceID(modelID)
	if normalized == "" {
		return false
	}
	for _, model := range GetBuiltinModels("openai") {
		if model == nil {
			continue
		}
		if normalizeModelPreferenceID(model.ID) == normalized || normalizeModelPreferenceID(model.Name) == normalized {
			return true
		}
	}
	return false
}

func isOpenAIFirstPartyProvider(provider *Provider) bool {
	if provider == nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(provider.ID), "openai") {
		return true
	}
	return isOpenAIFirstPartyURL(provider.EffectiveBaseURL()) ||
		isOpenAIFirstPartyURL(provider.BaseURL) ||
		isOpenAIFirstPartyURL(provider.DetectedEndpoint) ||
		isOpenAIFirstPartyURL(provider.Website)
}

func isOpenAIFirstPartyURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	host := ""
	if err == nil {
		host = u.Hostname()
	}
	if host == "" {
		host = strings.SplitN(strings.TrimPrefix(strings.TrimPrefix(raw, "https://"), "http://"), "/", 2)[0]
	}
	return isOpenAIFirstPartyHost(host)
}

func isOpenAIFirstPartyHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "" {
		return false
	}
	switch host {
	case "openai.com", "api.openai.com", "chat.openai.com", "platform.openai.com", "chatgpt.com":
		return true
	}
	return strings.HasSuffix(host, ".openai.com")
}

func recommendedAPIFormatForModel(modelID string, detectedFormat APIFormat, anthropicReachable, openAIReachable, responsesReachable, responsesOnly bool) APIFormat {
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
	if responsesReachable {
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

	preferredFormats := preferredAPIFormatsForProviderModel(provider, modelID)
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
	for index, preferred := range preferredFormats {
		if format == preferred {
			score = (len(preferredFormats) - index) * 100
			break
		}
	}

	providerID := strings.ToLower(strings.TrimSpace(provider.ID))
	switch inferModelFamily(modelID) {
	case modelFamilyClaude:
		if nativeFormat == APIFormatAnthropic || strings.Contains(providerID, "anthropic") {
			score += 25
		}
	case modelFamilyResponsesNative:
		if nativeFormat == APIFormatResponses || strings.Contains(providerID, "codex") {
			score += 25
		}
	default:
		return 0
	}

	return score
}

func sortModelIDsByPreference(modelIDs []string) []string {
	out := append([]string(nil), modelIDs...)
	sort.SliceStable(out, func(i, j int) bool {
		return compareModelPreference(out[i], out[j]) < 0
	})
	return out
}

func sortModelsByPreference(models []*Model) []*Model {
	out := append([]*Model(nil), models...)
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
	leftNormalized := normalizeModelPreferenceID(left)
	rightNormalized := normalizeModelPreferenceID(right)
	if leftNormalized > rightNormalized {
		return -1
	}
	if leftNormalized < rightNormalized {
		return 1
	}

	leftRaw := strings.ToLower(strings.TrimSpace(left))
	rightRaw := strings.ToLower(strings.TrimSpace(right))
	if leftRaw > rightRaw {
		return -1
	}
	if leftRaw < rightRaw {
		return 1
	}
	return 0
}

func modelVersionScore(modelID string) int {
	matches := modelVersionRegexp().FindAllString(normalizeModelPreferenceID(modelID), -1)
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
