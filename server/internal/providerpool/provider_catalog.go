package providerpool

import (
	"encoding/json"
	"strings"
	"sync"
	"unicode"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool/embedded"
)

type officialProviderCatalog struct {
	Providers        map[string]officialProviderCatalogProvider `json:"providers"`
	Models           map[string][]officialProviderCatalogModel  `json:"models"`
	PinchBenchModels map[string][]officialProviderCatalogModel  `json:"pinchbench_models,omitempty"`
}

type officialProviderCatalogProvider struct {
	BaseURL      *string               `json:"base_url,omitempty"`
	APIFormat    *APIFormat            `json:"api_format,omitempty"`
	Location     *ProviderLocation     `json:"location,omitempty"`
	Website      *string               `json:"website,omitempty"`
	APIKeyURL    *string               `json:"api_key_url,omitempty"`
	Icon         *string               `json:"icon,omitempty"`
	MetadataMode *ProviderMetadataMode `json:"metadata_mode,omitempty"`
}

type officialProviderCatalogModel struct {
	ID              string   `json:"id"`
	DisplayName     string   `json:"display_name,omitempty"`
	ContextWindow   int      `json:"context_window,omitempty"`
	MaxOutput       int      `json:"max_output,omitempty"`
	Capabilities    []string `json:"capabilities,omitempty"`
	PinchBenchScore *float64 `json:"pinchbench_score,omitempty"`
	PinchBenchURL   string   `json:"pinchbench_url,omitempty"`
}

var officialProviderCatalogState struct {
	mu      sync.RWMutex
	catalog officialProviderCatalog
}

var officialProviderCatalogLoadState struct {
	mu     sync.Mutex
	loaded bool
}

func markOfficialProviderCatalogLoaded() {
	officialProviderCatalogLoadState.mu.Lock()
	officialProviderCatalogLoadState.loaded = true
	officialProviderCatalogLoadState.mu.Unlock()
}

func ensureOfficialProviderCatalogLoaded() {
	officialProviderCatalogLoadState.mu.Lock()
	if officialProviderCatalogLoadState.loaded {
		officialProviderCatalogLoadState.mu.Unlock()
		return
	}
	officialProviderCatalogLoadState.loaded = true
	officialProviderCatalogLoadState.mu.Unlock()

	content, err := embedded.ProviderCatalogFS.ReadFile("provider_catalog.json")
	if err != nil {
		officialProviderCatalogLoadState.mu.Lock()
		officialProviderCatalogLoadState.loaded = false
		officialProviderCatalogLoadState.mu.Unlock()
		return
	}
	var catalog officialProviderCatalog
	if err := json.Unmarshal(content, &catalog); err != nil {
		officialProviderCatalogLoadState.mu.Lock()
		officialProviderCatalogLoadState.loaded = false
		officialProviderCatalogLoadState.mu.Unlock()
		return
	}
	SetOfficialProviderCatalog(catalog)
}

func SetOfficialProviderCatalog(catalog officialProviderCatalog) {
	officialProviderCatalogState.mu.Lock()
	defer officialProviderCatalogState.mu.Unlock()

	normalized := officialProviderCatalog{
		Providers:        make(map[string]officialProviderCatalogProvider, len(catalog.Providers)),
		Models:           make(map[string][]officialProviderCatalogModel, len(catalog.Models)),
		PinchBenchModels: make(map[string][]officialProviderCatalogModel, len(catalog.PinchBenchModels)),
	}

	for providerID, provider := range catalog.Providers {
		if providerID == "" {
			continue
		}
		normalized.Providers[providerID] = provider
	}

	for providerID, models := range catalog.Models {
		if providerID == "" {
			continue
		}
		copied := make([]officialProviderCatalogModel, 0, len(models))
		for _, model := range models {
			if model.ID == "" {
				continue
			}
			copied = append(copied, officialProviderCatalogModel{
				ID:              model.ID,
				DisplayName:     model.DisplayName,
				ContextWindow:   model.ContextWindow,
				MaxOutput:       model.MaxOutput,
				Capabilities:    append([]string(nil), model.Capabilities...),
				PinchBenchScore: cloneOptionalFloat64(model.PinchBenchScore),
				PinchBenchURL:   model.PinchBenchURL,
			})
		}
		normalized.Models[providerID] = copied
	}

	for providerID, models := range catalog.PinchBenchModels {
		if providerID == "" {
			continue
		}
		copied := make([]officialProviderCatalogModel, 0, len(models))
		for _, model := range models {
			if model.ID == "" {
				continue
			}
			copied = append(copied, officialProviderCatalogModel{
				ID:              model.ID,
				DisplayName:     model.DisplayName,
				ContextWindow:   model.ContextWindow,
				MaxOutput:       model.MaxOutput,
				Capabilities:    append([]string(nil), model.Capabilities...),
				PinchBenchScore: cloneOptionalFloat64(model.PinchBenchScore),
				PinchBenchURL:   model.PinchBenchURL,
			})
		}
		normalized.PinchBenchModels[providerID] = copied
	}

	officialProviderCatalogState.catalog = normalized
	markOfficialProviderCatalogLoaded()
}

func ClearOfficialProviderCatalog() {
	officialProviderCatalogState.mu.Lock()
	defer officialProviderCatalogState.mu.Unlock()
	officialProviderCatalogState.catalog = officialProviderCatalog{}
}

func applyOfficialProviderCatalogToProviders(providers []*Provider) {
	ensureOfficialProviderCatalogLoaded()
	officialProviderCatalogState.mu.RLock()
	defer officialProviderCatalogState.mu.RUnlock()

	if len(officialProviderCatalogState.catalog.Providers) == 0 {
		return
	}

	for _, provider := range providers {
		if provider == nil {
			continue
		}

		if provider.MetadataMode == "" {
			provider.MetadataMode = defaultProviderMetadataMode(provider)
		}

		override, ok := officialProviderCatalogState.catalog.Providers[provider.ID]
		if !ok {
			continue
		}

		if override.BaseURL != nil {
			provider.BaseURL = *override.BaseURL
		}
		if override.APIFormat != nil {
			provider.APIFormat = *override.APIFormat
		}
		if override.Location != nil {
			provider.Location = *override.Location
		}
		if override.Website != nil {
			provider.Website = *override.Website
		}
		if override.APIKeyURL != nil {
			provider.APIKeyURL = *override.APIKeyURL
		}
		if override.Icon != nil {
			provider.Icon = *override.Icon
		}
		if override.MetadataMode != nil {
			provider.MetadataMode = *override.MetadataMode
		}
	}
}

func applyOfficialProviderCatalogToModels(models map[string][]*Model) {
	ensureOfficialProviderCatalogLoaded()
	officialProviderCatalogState.mu.RLock()
	defer officialProviderCatalogState.mu.RUnlock()

	if len(officialProviderCatalogState.catalog.Models) == 0 {
		return
	}

	for providerID, catalogModels := range officialProviderCatalogState.catalog.Models {
		fallbackModels := models[providerID]
		fallbackByID := make(map[string]*Model, len(fallbackModels))
		for _, model := range fallbackModels {
			if model == nil || model.ID == "" {
				continue
			}
			fallbackByID[model.ID] = model
		}

		merged := make([]*Model, 0, len(catalogModels)+len(fallbackModels))
		seen := make(map[string]struct{}, len(catalogModels))

		for _, catalogModel := range catalogModels {
			model := cloneBuiltinModel(fallbackByID[catalogModel.ID])
			if model == nil {
				model = &Model{
					ID:         catalogModel.ID,
					ProviderID: providerID,
					Name:       catalogModel.ID,
					Enabled:    true,
				}
			}

			model.ProviderID = providerID
			model.ID = catalogModel.ID
			model.Name = catalogModel.ID
			if catalogModel.DisplayName != "" {
				model.DisplayName = catalogModel.DisplayName
			} else if model.DisplayName == "" {
				model.DisplayName = model.ID
			}
			if catalogModel.ContextWindow > 0 {
				model.ContextWindow = catalogModel.ContextWindow
			}
			if catalogModel.MaxOutput > 0 {
				model.MaxOutput = catalogModel.MaxOutput
			}
			if len(catalogModel.Capabilities) > 0 {
				model.Capabilities = capabilitiesFromCatalogStrings(catalogModel.Capabilities)
			}
			if catalogModel.PinchBenchScore != nil {
				model.PinchBenchScore = cloneOptionalFloat64(catalogModel.PinchBenchScore)
			}
			if catalogModel.PinchBenchURL != "" {
				model.PinchBenchURL = catalogModel.PinchBenchURL
			}

			merged = append(merged, model)
			seen[catalogModel.ID] = struct{}{}
		}

		for _, fallbackModel := range fallbackModels {
			if fallbackModel == nil {
				continue
			}
			if _, ok := seen[fallbackModel.ID]; ok {
				continue
			}
			merged = append(merged, cloneBuiltinModel(fallbackModel))
		}

		models[providerID] = merged
	}
}

func cloneBuiltinModel(model *Model) *Model {
	if model == nil {
		return nil
	}
	copied := *model
	copied.PinchBenchScore = cloneOptionalFloat64(model.PinchBenchScore)
	return &copied
}

func cloneOptionalFloat64(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func effectivePinchBenchMetadata(providerID string, model *Model) (*float64, string) {
	if model == nil {
		return nil, ""
	}
	if model.PinchBenchScore != nil && model.PinchBenchURL != "" {
		return cloneOptionalFloat64(model.PinchBenchScore), model.PinchBenchURL
	}

	score, url := pinchBenchMetadataForModel(providerID, model)
	if score == nil && url == "" {
		return cloneOptionalFloat64(model.PinchBenchScore), model.PinchBenchURL
	}

	if model.PinchBenchScore != nil {
		score = cloneOptionalFloat64(model.PinchBenchScore)
	}
	if model.PinchBenchURL != "" {
		url = model.PinchBenchURL
	}
	return score, url
}

func pinchBenchMetadataForModel(providerID string, model *Model) (*float64, string) {
	if model == nil || providerID == "" {
		return nil, ""
	}

	bestScore := -1
	var best officialProviderCatalogModel
	for providerRank, candidateProviderID := range pinchBenchProviderCandidates(providerID, model) {
		officialProviderCatalogState.mu.RLock()
		catalogModels := append([]officialProviderCatalogModel(nil), officialProviderCatalogState.catalog.Models[candidateProviderID]...)
		catalogModels = append(catalogModels, officialProviderCatalogState.catalog.PinchBenchModels[candidateProviderID]...)
		officialProviderCatalogState.mu.RUnlock()

		if len(catalogModels) == 0 {
			continue
		}

		modelIDKeys := pinchBenchModelKeySet(candidateProviderID, model.ID)
		modelAllKeys := pinchBenchModelKeySet(candidateProviderID, model.ID, model.Name, model.DisplayName)

		for _, candidate := range catalogModels {
			if candidate.PinchBenchScore == nil {
				continue
			}
			candidateIDKeys := pinchBenchModelKeySet(candidateProviderID, candidate.ID)
			candidateAllKeys := pinchBenchModelKeySet(candidateProviderID, candidate.ID, candidate.DisplayName)

			matchScore := 0
			switch {
			case pinchBenchSetsIntersect(modelIDKeys, candidateIDKeys):
				matchScore = 100
			case pinchBenchSetsIntersect(modelAllKeys, candidateIDKeys):
				matchScore = 90
			case pinchBenchSetsIntersect(modelAllKeys, candidateAllKeys):
				matchScore = 80
			}

			if matchScore <= 0 {
				continue
			}

			totalScore := matchScore*10 - providerRank
			if totalScore > bestScore {
				bestScore = totalScore
				best = candidate
			}
		}
	}

	if bestScore <= 0 {
		return nil, ""
	}
	return cloneOptionalFloat64(best.PinchBenchScore), best.PinchBenchURL
}

func pinchBenchProviderCandidates(providerID string, model *Model) []string {
	seen := make(map[string]struct{})
	var candidates []string
	addProvider := func(raw string) {
		for _, provider := range pinchBenchProviderAliases(raw) {
			if provider == "" {
				continue
			}
			if _, ok := seen[provider]; ok {
				continue
			}
			seen[provider] = struct{}{}
			candidates = append(candidates, provider)
		}
	}

	addProvider(providerID)
	providerTokens := pinchBenchTokens(providerID)
	for i := range providerTokens {
		for _, inferred := range pinchBenchInferredProviders(providerTokens[i:]) {
			addProvider(inferred)
		}
	}
	if model == nil {
		return candidates
	}

	for _, value := range []string{model.ID, model.Name, model.DisplayName} {
		tokens := pinchBenchTokens(value)
		if len(tokens) == 0 {
			continue
		}
		if strings.Contains(value, "/") {
			addProvider(tokens[0])
		}
		for _, inferred := range pinchBenchInferredProviders(tokens) {
			addProvider(inferred)
		}
	}

	return candidates
}

func pinchBenchProviderAliases(providerID string) []string {
	providerID = strings.TrimSpace(strings.ToLower(providerID))
	if providerID == "" {
		return nil
	}

	aliases := map[string][]string{
		"amazon":     {"amazon", "bedrock"},
		"bedrock":    {"bedrock", "amazon"},
		"glm":        {"glm", "z-ai"},
		"grok":       {"grok", "x-ai"},
		"mistral":    {"mistral", "mistralai"},
		"mistralai":  {"mistral", "mistralai"},
		"moonshot":   {"moonshot", "moonshotai"},
		"moonshotai": {"moonshot", "moonshotai"},
		"x-ai":       {"grok", "x-ai"},
		"z-ai":       {"glm", "z-ai"},
	}
	if mapped, ok := aliases[providerID]; ok {
		return mapped
	}
	return []string{providerID}
}

func pinchBenchInferredProviders(tokens []string) []string {
	if len(tokens) == 0 {
		return nil
	}

	joined := strings.Join(tokens, "")
	switch {
	case strings.HasPrefix(joined, "openai"):
		return []string{"openai"}
	case strings.HasPrefix(joined, "gpt") || strings.HasPrefix(joined, "o1") || strings.HasPrefix(joined, "o3"):
		return []string{"openai"}
	case strings.HasPrefix(joined, "anthropic"):
		return []string{"anthropic"}
	case strings.HasPrefix(joined, "claude"):
		return []string{"anthropic"}
	case strings.HasPrefix(joined, "google"):
		return []string{"google"}
	case strings.HasPrefix(joined, "gemini"):
		return []string{"google"}
	case strings.HasPrefix(joined, "deepseek"):
		return []string{"deepseek"}
	case strings.HasPrefix(joined, "qwen"):
		return []string{"qwen"}
	case strings.HasPrefix(joined, "xai") || strings.HasPrefix(joined, "grok"):
		return []string{"grok", "x-ai"}
	case strings.HasPrefix(joined, "moonshot") || strings.HasPrefix(joined, "moonshotai") || strings.HasPrefix(joined, "kimi"):
		return []string{"moonshot", "moonshotai"}
	case strings.HasPrefix(joined, "zai") || strings.HasPrefix(joined, "glm"):
		return []string{"glm", "z-ai"}
	case strings.HasPrefix(joined, "minimax") || strings.HasPrefix(joined, "codexminimax"):
		return []string{"minimax"}
	case strings.HasPrefix(joined, "amazon") || strings.HasPrefix(joined, "bedrock") || strings.HasPrefix(joined, "nova"):
		return []string{"bedrock", "amazon"}
	case strings.HasPrefix(joined, "openrouter") || strings.HasPrefix(joined, "hunter") || strings.HasPrefix(joined, "healer"):
		return []string{"openrouter"}
	case strings.HasPrefix(joined, "mimo") || strings.HasPrefix(joined, "xiaomi"):
		return []string{"xiaomi"}
	case strings.HasPrefix(joined, "step") || strings.HasPrefix(joined, "stepfun"):
		return []string{"stepfun"}
	case strings.HasPrefix(joined, "nemotron") || strings.HasPrefix(joined, "nvidia"):
		return []string{"nvidia"}
	case strings.HasPrefix(joined, "trinity") || strings.HasPrefix(joined, "arcee") || strings.HasPrefix(joined, "arceeai"):
		return []string{"arcee-ai"}
	case strings.HasPrefix(joined, "devstral") || strings.HasPrefix(joined, "mistral") || strings.HasPrefix(joined, "mistralai"):
		return []string{"mistral", "mistralai"}
	case strings.HasPrefix(joined, "mercury") || strings.HasPrefix(joined, "inception"):
		return []string{"inception"}
	}
	return nil
}

func pinchBenchSetsIntersect(left, right map[string]struct{}) bool {
	if len(left) == 0 || len(right) == 0 {
		return false
	}
	for key := range left {
		if _, ok := right[key]; ok {
			return true
		}
	}
	return false
}

func pinchBenchModelKeySet(providerID string, values ...string) map[string]struct{} {
	keys := make(map[string]struct{})
	for _, value := range values {
		for _, variant := range pinchBenchKeyVariants(providerID, value) {
			if variant == "" {
				continue
			}
			keys[variant] = struct{}{}
		}
	}
	return keys
}

func pinchBenchKeyVariants(providerID, raw string) []string {
	tokens := pinchBenchTokens(raw)
	if len(tokens) == 0 {
		return nil
	}

	var variants []string
	add := func(parts []string) {
		if len(parts) == 0 {
			return
		}
		variants = append(variants, strings.Join(parts, ""))
	}

	add(tokens)

	providerTokens := pinchBenchTokens(providerID)
	if len(providerTokens) > 0 && len(tokens) > len(providerTokens) && pinchBenchHasPrefix(tokens, providerTokens) {
		trimmed := append([]string(nil), tokens[len(providerTokens):]...)
		add(trimmed)
		tokens = append([]string(nil), tokens...)
	}

	if isPinchBenchVersionSuffix(tokens[len(tokens)-1]) {
		add(tokens[:len(tokens)-1])
		if len(providerTokens) > 0 && len(tokens)-1 > len(providerTokens) && pinchBenchHasPrefix(tokens[:len(tokens)-1], providerTokens) {
			add(tokens[len(providerTokens) : len(tokens)-1])
		}
	}

	seen := make(map[string]struct{}, len(variants))
	deduped := make([]string, 0, len(variants))
	for _, variant := range variants {
		if variant == "" {
			continue
		}
		if _, ok := seen[variant]; ok {
			continue
		}
		seen[variant] = struct{}{}
		deduped = append(deduped, variant)
	}
	return deduped
}

func pinchBenchTokens(raw string) []string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return nil
	}
	var tokens []string
	var current []rune
	flush := func() {
		if len(current) == 0 {
			return
		}
		tokens = append(tokens, string(current))
		current = current[:0]
	}
	for _, r := range raw {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current = append(current, r)
			continue
		}
		flush()
	}
	flush()
	return tokens
}

func pinchBenchHasPrefix(tokens, prefix []string) bool {
	if len(prefix) == 0 || len(tokens) < len(prefix) {
		return false
	}
	for i := range prefix {
		if tokens[i] != prefix[i] {
			return false
		}
	}
	return true
}

func isPinchBenchVersionSuffix(token string) bool {
	if len(token) < 6 {
		return false
	}
	for _, r := range token {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func capabilitiesFromCatalogStrings(values []string) ModelCapabilities {
	var caps ModelCapabilities
	for _, value := range values {
		switch value {
		case "chat":
			caps.Chat = true
		case "completion":
			caps.Completion = true
		case "vision":
			caps.Vision = true
		case "function_call":
			caps.FunctionCall = true
		case "streaming":
			caps.Streaming = true
		case "thinking":
			caps.Thinking = true
		case "json":
			caps.JSON = true
		case "system_prompt":
			caps.SystemPrompt = true
		case "image_generation":
			caps.ImageGeneration = true
		case "video_generation":
			caps.VideoGeneration = true
		case "audio_generation":
			caps.AudioGeneration = true
		}
	}
	return caps
}
