package providerpool

import "sync"

type officialProviderCatalog struct {
	Providers map[string]officialProviderCatalogProvider `json:"providers"`
	Models    map[string][]officialProviderCatalogModel  `json:"models"`
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

func SetOfficialProviderCatalog(catalog officialProviderCatalog) {
	officialProviderCatalogState.mu.Lock()
	defer officialProviderCatalogState.mu.Unlock()

	normalized := officialProviderCatalog{
		Providers: make(map[string]officialProviderCatalogProvider, len(catalog.Providers)),
		Models:    make(map[string][]officialProviderCatalogModel, len(catalog.Models)),
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

	officialProviderCatalogState.catalog = normalized
}

func ClearOfficialProviderCatalog() {
	officialProviderCatalogState.mu.Lock()
	defer officialProviderCatalogState.mu.Unlock()
	officialProviderCatalogState.catalog = officialProviderCatalog{}
}

func applyOfficialProviderCatalogToProviders(providers []*Provider) {
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
