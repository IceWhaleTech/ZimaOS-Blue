package providercatalogseed

import (
	"encoding/json"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

type (
	APIFormat            = providerpool.APIFormat
	Model                = providerpool.Model
	ModelCapabilities    = providerpool.ModelCapabilities
	Provider             = providerpool.Provider
	ProviderLocation     = providerpool.ProviderLocation
	ProviderMetadataMode = providerpool.ProviderMetadataMode
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

func builtinProvidersFallback() []*Provider {
	return providerpool.BuiltinProvidersSnapshot()
}

func builtinModelsFallback() map[string][]*Model {
	return providerpool.BuiltinModelsSnapshot()
}

func defaultProviderMetadataMode(provider *Provider) ProviderMetadataMode {
	return providerpool.EffectiveProviderMetadataMode(provider)
}

func capabilitiesToStrings(c ModelCapabilities) []string {
	var caps []string
	if c.Chat {
		caps = append(caps, "chat")
	}
	if c.Completion {
		caps = append(caps, "completion")
	}
	if c.Vision {
		caps = append(caps, "vision")
	}
	if c.FunctionCall {
		caps = append(caps, "function_call")
	}
	if c.Streaming {
		caps = append(caps, "streaming")
	}
	if c.Thinking {
		caps = append(caps, "thinking")
	}
	if c.JSON {
		caps = append(caps, "json")
	}
	if c.SystemPrompt {
		caps = append(caps, "system_prompt")
	}
	if c.ImageGeneration {
		caps = append(caps, "image_generation")
	}
	if c.VideoGeneration {
		caps = append(caps, "video_generation")
	}
	if c.AudioGeneration {
		caps = append(caps, "audio_generation")
	}
	if caps == nil {
		caps = []string{}
	}
	return caps
}

var officialCatalogProviderIDs = []string{
	"anthropic",
	"bedrock",
	"deepseek",
	"glm",
	"google",
	"grok",
	"minimax",
	"moonshot",
	"ollama",
	"openai",
	"qwen",
}

var officialCatalogModelAugmentations = map[string][]officialProviderCatalogModel{
	"anthropic": {
		{
			ID:           "claude-opus-4-6",
			DisplayName:  "Claude Opus 4.6",
			Capabilities: catalogVisionThinkingCapabilities(),
		},
		{
			ID:           "claude-haiku-4-5",
			DisplayName:  "Claude Haiku 4.5",
			Capabilities: catalogVisionCapabilities(),
		},
		{
			ID:           "claude-sonnet-4-6",
			DisplayName:  "Claude Sonnet 4.6",
			Capabilities: catalogVisionThinkingCapabilities(),
		},
		{
			ID:           "claude-sonnet-4",
			DisplayName:  "Claude Sonnet 4",
			Capabilities: catalogVisionThinkingCapabilities(),
		},
	},
	"deepseek": {
	},
	"glm": {
		{
			ID:           "glm-5-turbo",
			DisplayName:  "GLM-5 Turbo",
			Capabilities: catalogChatCapabilities(),
		},
		{
			ID:           "glm-5",
			DisplayName:  "GLM-5",
			Capabilities: catalogChatCapabilities(),
		},
		{
			ID:           "glm-4.5-air",
			DisplayName:  "GLM-4.5 Air",
			Capabilities: catalogChatCapabilities(),
		},
	},
	"google": {
		{
			ID:           "gemini-3.1-pro-preview",
			DisplayName:  "Gemini 3.1 Pro Preview",
			Capabilities: catalogVisionCapabilities(),
		},
		{
			ID:           "gemini-3-flash-preview",
			DisplayName:  "Gemini 3 Flash Preview",
			Capabilities: catalogVisionCapabilities(),
		},
		{
			ID:           "gemini-3-pro-preview",
			DisplayName:  "Gemini 3 Pro Preview",
			Capabilities: catalogVisionCapabilities(),
		},
		{
			ID:           "gemini-2.5-pro",
			DisplayName:  "Gemini 2.5 Pro",
			Capabilities: catalogVisionCapabilities(),
		},
		{
			ID:           "gemini-2.5-flash",
			DisplayName:  "Gemini 2.5 Flash",
			Capabilities: catalogVisionCapabilities(),
		},
	},
	"grok": {
		{
			ID:           "grok-4.1-fast",
			DisplayName:  "Grok 4.1 Fast",
			Capabilities: catalogChatCapabilities(),
		},
	},
	"minimax": {
		{
			ID:            "MiniMax-M2.7",
			DisplayName:   "MiniMax M2.7",
			ContextWindow: 1000000,
			MaxOutput:     16384,
			Capabilities:  catalogChatCapabilities(),
		},
		{
			ID:            "MiniMax-M2.7-highspeed",
			DisplayName:   "MiniMax M2.7 Highspeed",
			ContextWindow: 1000000,
			MaxOutput:     16384,
			Capabilities:  catalogChatCapabilities(),
		},
		{
			ID:           "minimax-m2.7",
			DisplayName:  "MiniMax M2.7",
			Capabilities: catalogChatCapabilities(),
		},
		{
			ID:           "minimax-m2.5",
			DisplayName:  "MiniMax M2.5",
			Capabilities: catalogChatCapabilities(),
		},
		{
			ID:           "minimax-m2.1",
			DisplayName:  "MiniMax M2.1",
			Capabilities: catalogChatCapabilities(),
		},
	},
	"moonshot": {
		{
			ID:           "kimi-k2.5",
			DisplayName:  "Kimi K2.5",
			Capabilities: catalogChatCapabilities(),
		},
	},
	"openai": {
		{
			ID:            "gpt-5.4",
			DisplayName:   "GPT-5.4",
			ContextWindow: 1050000,
			MaxOutput:     128000,
			Capabilities:  catalogVisionThinkingCapabilities(),
		},
		{
			ID:            "gpt-5.4-mini",
			DisplayName:   "GPT-5.4 Mini",
			ContextWindow: 1050000,
			MaxOutput:     128000,
			Capabilities:  catalogVisionThinkingCapabilities(),
		},
		{
			ID:            "gpt-5.4-nano",
			DisplayName:   "GPT-5.4 Nano",
			ContextWindow: 400000,
			MaxOutput:     128000,
			Capabilities:  catalogVisionThinkingCapabilities(),
		},
		{
			ID:            "gpt-5.4-pro",
			DisplayName:   "GPT-5.4 Pro",
			ContextWindow: 1050000,
			MaxOutput:     128000,
			Capabilities:  catalogVisionThinkingCapabilities(),
		},
		{
			ID:           "gpt-5-mini",
			DisplayName:  "GPT-5 Mini",
			Capabilities: catalogVisionThinkingCapabilities(),
		},
		{
			ID:           "gpt-5-nano",
			DisplayName:  "GPT-5 Nano",
			Capabilities: catalogVisionThinkingCapabilities(),
		},
		{
			ID:           "gpt-oss-120b",
			DisplayName:  "GPT-OSS 120B",
			Capabilities: catalogChatCapabilities(),
		},
	},
	"qwen": {
		{
			ID:           "qwen3.6-plus-preview",
			DisplayName:  "Qwen 3.6 Plus Preview",
			Capabilities: catalogChatCapabilities(),
		},
		{
			ID:           "qwen3.5-plus-02-15",
			DisplayName:  "Qwen 3.5 Plus 02-15",
			Capabilities: catalogChatCapabilities(),
		},
		{
			ID:           "qwen3.5-397b-a17b",
			DisplayName:  "Qwen 3.5 397B A17B",
			Capabilities: catalogChatCapabilities(),
		},
		{
			ID:           "qwen3.5-122b-a10b",
			DisplayName:  "Qwen 3.5 122B A10B",
			Capabilities: catalogChatCapabilities(),
		},
		{
			ID:           "qwen3.5-35b-a3b",
			DisplayName:  "Qwen 3.5 35B A3B",
			Capabilities: catalogChatCapabilities(),
		},
		{
			ID:           "qwen3.5-27b",
			DisplayName:  "Qwen 3.5 27B",
			Capabilities: catalogChatCapabilities(),
		},
		{
			ID:           "qwen3-max-thinking",
			DisplayName:  "Qwen 3 Max Thinking",
			Capabilities: catalogThinkingCapabilities(),
		},
		{
			ID:           "qwen3-coder-next",
			DisplayName:  "Qwen 3 Coder Next",
			Capabilities: catalogChatCapabilities(),
		},
	},
}

type officialCatalogPinchBenchSeed struct {
	ID    string
	Score float64
	URL   string
}

var officialCatalogPinchBenchSeeds = []officialCatalogPinchBenchSeed{
	{ID: "anthropic/claude-opus-4.6", Score: 93.3, URL: "https://pinchbench.com/submission/1aff7c69-35c5-439e-ba52-4768586cc773"},
	{ID: "arcee-ai/trinity-large-thinking", Score: 91.9, URL: "https://pinchbench.com/submission/9b7f851c-7ff8-4c0c-be0a-2ac4a0476682"},
	{ID: "openai/gpt-5.4", Score: 90.5, URL: "https://pinchbench.com/submission/5d73c775-fb81-4df1-ac2f-a08434541601"},
	{ID: "qwen/qwen3.5-27b", Score: 90.0, URL: "https://pinchbench.com/submission/6769a9a6-825d-4848-af0c-8e7f590dcc7f"},
	{ID: "minimax/minimax-m2.7", Score: 89.8, URL: "https://pinchbench.com/submission/0e379397-42e2-440e-a466-5e3adcb3eadb"},
	{ID: "anthropic/claude-haiku-4.5", Score: 89.5, URL: "https://pinchbench.com/submission/f29c3e4c-b73c-4fa9-9c8d-2eb0afb050c8"},
	{ID: "qwen/qwen3.5-397b-a17b", Score: 89.1, URL: "https://pinchbench.com/submission/3a06230b-8189-44ca-83e1-8111f05552ed"},
	{ID: "xiaomi/mimo-v2-flash", Score: 88.8, URL: "https://pinchbench.com/submission/9f510963-d3df-486f-a9a7-ad7a20290088"},
	{ID: "qwen/qwen3.6-plus-preview", Score: 88.6, URL: "https://pinchbench.com/submission/1561ccd1-3f5a-419d-9b7f-4dd9027b6644"},
	{ID: "nvidia/nemotron-3-super-120b-a12b", Score: 88.6, URL: "https://pinchbench.com/submission/75b0e342-e702-432b-8733-747d547996bc"},
	{ID: "anthropic/claude-sonnet-4.5", Score: 88.6, URL: "https://pinchbench.com/submission/99c90752-c44f-440f-a236-05d460d6f986"},
	{ID: "minimax/minimax-m2.1", Score: 88.4, URL: "https://pinchbench.com/submission/9f44be09-3193-4e0a-af5b-769c42f8f3f3"},
	{ID: "anthropic/claude-sonnet-4.6", Score: 88.0, URL: "https://pinchbench.com/submission/1935b2ec-6d50-47d6-b2b3-6be233d30aca"},
	{ID: "minimax/minimax-m2.5", Score: 87.8, URL: "https://pinchbench.com/submission/e010a739-c548-4808-90a6-7b574731ce49"},
	{ID: "anthropic/claude-opus-4.5", Score: 87.2, URL: "https://pinchbench.com/submission/7bb7d213-9fd3-42b1-b58b-7827d0414ae8"},
	{ID: "google/gemini-3.1-pro-preview", Score: 86.7, URL: "https://pinchbench.com/submission/684113a8-8421-4795-a5c9-6134b4b06222"},
	{ID: "z-ai/glm-5-turbo", Score: 86.5, URL: "https://pinchbench.com/submission/bc57bf80-2020-4a8c-be16-d8be99f94cea"},
	{ID: "z-ai/glm-5", Score: 86.4, URL: "https://pinchbench.com/submission/220a7484-2a99-441f-b101-08ed8fb8870f"},
	{ID: "qwen/qwen3.5-plus-02-15", Score: 85.8, URL: "https://pinchbench.com/submission/a3f8c8d8-0b0b-4a28-a61e-a8f4c7193c5a"},
	{ID: "z-ai/glm-4.5-air", Score: 85.7, URL: "https://pinchbench.com/submission/048edef1-bb7b-4ee4-8023-8320b019a510"},
	{ID: "xiaomi/mimo-v2-omni", Score: 85.6, URL: "https://pinchbench.com/submission/2ffb5134-7a76-4274-bfde-644ae00f33ff"},
	{ID: "qwen/qwen3.5-122b-a10b", Score: 85.5, URL: "https://pinchbench.com/submission/ad4e56aa-e193-4188-a0a0-69d27bdadc71"},
	{ID: "stepfun/step-3.5-flash", Score: 85.3, URL: "https://pinchbench.com/submission/7419a7a1-bc80-4df2-9a03-2c6beccc942e"},
	{ID: "google/gemini-3-flash-preview", Score: 85.2, URL: "https://pinchbench.com/submission/8892d4e7-1ba2-415d-883c-ca7ec30bf7e0"},
	{ID: "moonshotai/kimi-k2.5", Score: 84.8, URL: "https://pinchbench.com/submission/ce9bbcbd-f78b-4655-af1f-c97781320ce6"},
	{ID: "xiaomi/mimo-v2-pro", Score: 84.0, URL: "https://pinchbench.com/submission/6236740b-edec-4725-a3ba-948ca7d7db14"},
	{ID: "openrouter/hunter-alpha", Score: 83.3},
	{ID: "x-ai/grok-4.1-fast", Score: 82.4},
	{ID: "mistralai/devstral-2512", Score: 82.0},
	{ID: "openrouter/healer-alpha", Score: 80.8},
	{ID: "arcee-ai/trinity-large-preview", Score: 80.6},
	{ID: "anthropic/claude-sonnet-4", Score: 80.5},
	{ID: "qwen/qwen3-max-thinking", Score: 80.3},
	{ID: "openai/gpt-5-mini", Score: 80.3},
	{ID: "qwen/qwen3-coder-next", Score: 79.1},
	{ID: "qwen/qwen3.5-35b-a3b", Score: 78.4},
	{ID: "inception/mercury-2", Score: 78.0},
	{ID: "arcee-ai/trinity-large-preview:free", Score: 77.7},
	{ID: "amazon/nova-2-lite-v1", Score: 75.4},
	{ID: "openai/gpt-4o-mini", Score: 75.0},
	{ID: "nvidia/nemotron-3-super-120b-a12b:free", Score: 75.0},
	{ID: "mistralai/mistral-large-2512", Score: 72.2},
	{ID: "google/gemini-2.5-pro", Score: 71.9},
	{ID: "deepseek/deepseek-chat", Score: 71.7},
	{ID: "openai/gpt-4o", Score: 71.1},
	{ID: "google/gemini-3-pro-preview", Score: 70.7},
	{ID: "google/gemini-2.5-flash", Score: 70.7},
	{ID: "openai/gpt-5-nano", Score: 68.8},
	{ID: "openai/gpt-oss-120b", Score: 67.1},
}

// GenerateOfficialProviderCatalogJSON returns the canonical provider catalog JSON
// used for both docs/provider_catalog.json and the embedded startup fallback.
func GenerateOfficialProviderCatalogJSON() ([]byte, error) {
	content, err := json.MarshalIndent(generateOfficialProviderCatalog(), "", "  ")
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}

func generateOfficialProviderCatalog() officialProviderCatalog {
	catalog := officialProviderCatalog{
		Providers:        make(map[string]officialProviderCatalogProvider, len(officialCatalogProviderIDs)),
		Models:           make(map[string][]officialProviderCatalogModel, len(officialCatalogProviderIDs)),
		PinchBenchModels: make(map[string][]officialProviderCatalogModel),
	}

	builtinProviders := builtinProvidersFallback()
	builtinModels := builtinModelsFallback()

	for _, providerID := range officialCatalogProviderIDs {
		if provider := catalogProviderByID(builtinProviders, providerID); provider != nil {
			catalog.Providers[providerID] = officialCatalogProviderFromBuiltin(provider)
		}

		models := dedupeOfficialCatalogModels(
			officialCatalogModelAugmentations[providerID],
			officialCatalogModelsFromBuiltin(builtinModels[providerID]),
		)
		catalog.Models[providerID] = models
	}

	for _, seed := range officialCatalogPinchBenchSeeds {
		providerID, modelName := pinchBenchProviderAndModel(seed.ID)
		if providerID == "" || modelName == "" {
			continue
		}
		entry := officialProviderCatalogModel{
			ID:              seed.ID,
			DisplayName:     modelName,
			PinchBenchScore: cloneFloat64(seed.Score),
			PinchBenchURL:   seed.URL,
		}
		catalog.PinchBenchModels[providerID] = append(catalog.PinchBenchModels[providerID], entry)
	}

	return catalog
}

func officialCatalogProviderFromBuiltin(provider *Provider) officialProviderCatalogProvider {
	if provider == nil {
		return officialProviderCatalogProvider{}
	}

	metadataMode := provider.MetadataMode
	if metadataMode == "" {
		metadataMode = defaultProviderMetadataMode(provider)
	}

	return officialProviderCatalogProvider{
		BaseURL:      optionalString(provider.BaseURL),
		APIFormat:    optionalAPIFormat(provider.APIFormat),
		Location:     optionalProviderLocation(provider.Location),
		Website:      optionalString(provider.Website),
		APIKeyURL:    optionalString(provider.APIKeyURL),
		Icon:         optionalString(provider.Icon),
		MetadataMode: optionalProviderMetadataMode(metadataMode),
	}
}

func officialCatalogModelsFromBuiltin(models []*Model) []officialProviderCatalogModel {
	result := make([]officialProviderCatalogModel, 0, len(models))
	for _, model := range models {
		if model == nil || model.ID == "" {
			continue
		}
		result = append(result, officialProviderCatalogModel{
			ID:            model.ID,
			DisplayName:   model.DisplayName,
			ContextWindow: model.ContextWindow,
			MaxOutput:     model.MaxOutput,
			Capabilities:  capabilitiesToStrings(model.Capabilities),
		})
	}
	return result
}

func dedupeOfficialCatalogModels(groups ...[]officialProviderCatalogModel) []officialProviderCatalogModel {
	seen := make(map[string]struct{})
	var merged []officialProviderCatalogModel
	for _, group := range groups {
		for _, model := range group {
			if model.ID == "" {
				continue
			}
			if _, ok := seen[model.ID]; ok {
				continue
			}
			seen[model.ID] = struct{}{}
			merged = append(merged, model)
		}
	}
	return merged
}

func catalogProviderByID(providers []*Provider, providerID string) *Provider {
	for _, provider := range providers {
		if provider != nil && provider.ID == providerID {
			return provider
		}
	}
	return nil
}

func pinchBenchProviderAndModel(id string) (string, string) {
	providerID, modelName, ok := strings.Cut(id, "/")
	if !ok {
		return "", ""
	}
	return providerID, modelName
}

func catalogChatCapabilities() []string {
	return []string{"chat", "function_call", "streaming", "json", "system_prompt"}
}

func catalogVisionCapabilities() []string {
	return []string{"chat", "vision", "function_call", "streaming", "json", "system_prompt"}
}

func catalogThinkingCapabilities() []string {
	return []string{"chat", "function_call", "streaming", "thinking", "json", "system_prompt"}
}

func catalogVisionThinkingCapabilities() []string {
	return []string{"chat", "vision", "function_call", "streaming", "thinking", "json", "system_prompt"}
}

func cloneFloat64(value float64) *float64 {
	copied := value
	return &copied
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	copied := value
	return &copied
}

func optionalAPIFormat(value APIFormat) *APIFormat {
	if value == "" {
		return nil
	}
	copied := value
	return &copied
}

func optionalProviderLocation(value ProviderLocation) *ProviderLocation {
	if value == "" {
		return nil
	}
	copied := value
	return &copied
}

func optionalProviderMetadataMode(value ProviderMetadataMode) *ProviderMetadataMode {
	if value == "" {
		return nil
	}
	copied := value
	return &copied
}
