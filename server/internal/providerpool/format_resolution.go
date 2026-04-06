package providerpool

import (
	"net/url"
	"strings"
)

// FormatResolutionSource describes why a format was chosen.
type FormatResolutionSource string

const (
	FormatResolutionSourceEndpointLock  FormatResolutionSource = "endpoint_lock"
	FormatResolutionSourceUserPinned    FormatResolutionSource = "user_pinned"
	FormatResolutionSourceDetected      FormatResolutionSource = "detected"
	FormatResolutionSourceModelMemory   FormatResolutionSource = "model_memory"
	FormatResolutionSourceFamilyDefault FormatResolutionSource = "family_default"
)

// FormatResolutionBaseURLMode describes how the provider base URL should be interpreted.
type FormatResolutionBaseURLMode string

const (
	FormatResolutionBaseURLModeRoot          FormatResolutionBaseURLMode = "root"
	FormatResolutionBaseURLModeFixedEndpoint FormatResolutionBaseURLMode = "fixed_endpoint"
	FormatResolutionBaseURLModeDetected      FormatResolutionBaseURLMode = "detected_endpoint"
)

// FormatResolutionRequest carries the facts used to resolve a format plan.
type FormatResolutionRequest struct {
	Provider             *Provider
	ModelID              string
	DetectedFormat       APIFormat
	ModelMemoryFormat    APIFormat
	ProviderMemoryFormat APIFormat
}

// FormatResolutionPlan is the shared format-selection output used by verify/router/proxy.
type FormatResolutionPlan struct {
	SelectedFormat   APIFormat                   `json:"selected_format"`
	CandidateFormats []APIFormat                 `json:"candidate_formats"`
	Source           FormatResolutionSource      `json:"resolution_source"`
	BaseURLMode      FormatResolutionBaseURLMode `json:"base_url_mode"`
	Mutable          bool                        `json:"mutable"`
	TrySelectedFirst bool                        `json:"try_selected_first"`
}

func (p FormatResolutionPlan) Known() bool {
	return p.TrySelectedFirst
}

// ResolveAPIFormatPlan returns the unified API format decision for a provider/model pair.
func ResolveAPIFormatPlan(req FormatResolutionRequest) FormatResolutionPlan {
	provider := req.Provider
	baseURLMode := formatResolutionBaseURLMode(provider)

	if provider == nil {
		return FormatResolutionPlan{
			SelectedFormat:   APIFormatOpenAI,
			CandidateFormats: []APIFormat{APIFormatOpenAI},
			Source:           FormatResolutionSourceFamilyDefault,
			BaseURLMode:      baseURLMode,
		}
	}

	if endpointFormat, ok := providerEndpointLockedFormat(provider); ok {
		return FormatResolutionPlan{
			SelectedFormat:   endpointFormat,
			CandidateFormats: []APIFormat{endpointFormat},
			Source:           FormatResolutionSourceEndpointLock,
			BaseURLMode:      FormatResolutionBaseURLModeFixedEndpoint,
			TrySelectedFirst: true,
		}
	}

	mode := ProviderAPIFormatMode(provider)
	if mode == APIFormatModePinned {
		if provider.APIFormatMode != APIFormatModePinned && shouldPreferResponsesForUnknownOpenAIModel(provider, req.ModelID) {
			candidates := buildAutoCandidateFormats(req.ModelID, provider, "", req.DetectedFormat, req.ProviderMemoryFormat)
			selected := APIFormatOpenAI
			if len(candidates) > 0 {
				selected = candidates[0]
			}
			return FormatResolutionPlan{
				SelectedFormat:   selected,
				CandidateFormats: candidates,
				Source:           FormatResolutionSourceFamilyDefault,
				BaseURLMode:      baseURLMode,
			}
		}
		selected := firstNonEmptyFormat(provider.APIFormat, req.DetectedFormat, provider.DetectedFormat, canonicalAPIFormatForProvider(provider))
		if selected == "" {
			selected = APIFormatOpenAI
		}
		source := FormatResolutionSourceFamilyDefault
		trySelectedFirst := false
		if provider.APIFormatMode == APIFormatModePinned {
			source = FormatResolutionSourceUserPinned
			trySelectedFirst = true
		}
		return FormatResolutionPlan{
			SelectedFormat:   selected,
			CandidateFormats: []APIFormat{selected},
			Source:           source,
			BaseURLMode:      baseURLMode,
			TrySelectedFirst: trySelectedFirst,
		}
	}

	detected := firstNonEmptyFormat(req.DetectedFormat, provider.DetectedFormat)
	if req.ModelMemoryFormat != "" {
		return FormatResolutionPlan{
			SelectedFormat:   req.ModelMemoryFormat,
			CandidateFormats: buildAutoCandidateFormats(req.ModelID, provider, req.ModelMemoryFormat, detected, req.ProviderMemoryFormat),
			Source:           FormatResolutionSourceModelMemory,
			BaseURLMode:      baseURLMode,
			Mutable:          true,
			TrySelectedFirst: true,
		}
	}

	if detected != "" {
		return FormatResolutionPlan{
			SelectedFormat:   detected,
			CandidateFormats: buildAutoCandidateFormats(req.ModelID, provider, detected, detected, req.ProviderMemoryFormat),
			Source:           FormatResolutionSourceDetected,
			BaseURLMode:      baseURLMode,
			Mutable:          true,
			TrySelectedFirst: true,
		}
	}

	candidates := buildAutoCandidateFormats(req.ModelID, provider, "", detected, req.ProviderMemoryFormat)
	selected := APIFormatOpenAI
	if len(candidates) > 0 {
		selected = candidates[0]
	}
	return FormatResolutionPlan{
		SelectedFormat:   selected,
		CandidateFormats: candidates,
		Source:           FormatResolutionSourceFamilyDefault,
		BaseURLMode:      baseURLMode,
	}
}

// ProviderAPIFormatMode returns the effective format mode, applying sensible defaults.
func ProviderAPIFormatMode(provider *Provider) APIFormatMode {
	if provider == nil {
		return APIFormatModePinned
	}
	switch provider.APIFormatMode {
	case APIFormatModeAuto, APIFormatModePinned:
		return provider.APIFormatMode
	default:
		return defaultAPIFormatModeForProvider(provider)
	}
}

func defaultAPIFormatModeForProvider(provider *Provider) APIFormatMode {
	if provider == nil {
		return APIFormatModePinned
	}
	if _, ok := providerEndpointLockedFormat(provider); ok {
		return APIFormatModePinned
	}
	if isThirdPartyProvider(provider) {
		return APIFormatModeAuto
	}
	switch provider.APIFormat {
	case APIFormatResponses, APIFormatOllama, APIFormatCloudCode, APIFormatCopilot:
		return APIFormatModePinned
	}
	if provider.Type == "" {
		return APIFormatModeAuto
	}
	return APIFormatModePinned
}

func buildAutoCandidateFormats(modelID string, provider *Provider, selected APIFormat, detected APIFormat, providerMemory APIFormat) []APIFormat {
	candidates := make([]APIFormat, 0, 4)
	add := func(format APIFormat) {
		if format == "" {
			return
		}
		for _, existing := range candidates {
			if existing == format {
				return
			}
		}
		candidates = append(candidates, format)
	}

	for _, format := range preferredAPIFormatsForProviderModel(provider, modelID) {
		add(format)
	}
	add(selected)
	add(detected)
	add(providerMemory)
	add(provider.APIFormat)
	add(provider.DetectedFormat)
	add(canonicalAPIFormatForProvider(provider))

	if len(candidates) == 0 {
		add(APIFormatOpenAI)
	}
	return candidates
}

func firstNonEmptyFormat(formats ...APIFormat) APIFormat {
	for _, format := range formats {
		if format != "" {
			return format
		}
	}
	return ""
}

func formatResolutionBaseURLMode(provider *Provider) FormatResolutionBaseURLMode {
	if provider == nil {
		return FormatResolutionBaseURLModeRoot
	}
	if strings.TrimSpace(provider.DetectedEndpoint) != "" {
		return FormatResolutionBaseURLModeDetected
	}
	if _, ok := providerEndpointLockedFormat(provider); ok {
		return FormatResolutionBaseURLModeFixedEndpoint
	}
	return FormatResolutionBaseURLModeRoot
}

func providerEndpointLockedFormat(provider *Provider) (APIFormat, bool) {
	if provider == nil {
		return "", false
	}
	format, ok := DetectEndpointFixedFormat(provider.EffectiveBaseURL())
	if !ok {
		return "", false
	}
	if !isThirdPartyProvider(provider) {
		return format, true
	}
	u, err := url.Parse(strings.TrimSpace(provider.EffectiveBaseURL()))
	if err != nil {
		return format, true
	}
	if isGenericEndpointLockPath(u.Path) {
		return "", false
	}
	return format, true
}

func isGenericEndpointLockPath(path string) bool {
	path = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(path)), "/")
	switch path {
	case "/responses", "/v1/responses", "/messages", "/v1/messages":
		return true
	default:
		return false
	}
}

// DetectEndpointFixedFormat infers a fixed API family from an endpoint-specific base URL.
func DetectEndpointFixedFormat(raw string) (APIFormat, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return DetectEndpointFixedFormatFromPath(raw)
	}
	return DetectEndpointFixedFormatFromPath(u.Path)
}

// DetectEndpointFixedFormatFromPath infers a fixed API family from a URL path.
func DetectEndpointFixedFormatFromPath(path string) (APIFormat, bool) {
	path = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(path)), "/")
	switch {
	case strings.HasSuffix(path, "/responses"):
		return APIFormatResponses, true
	case strings.HasSuffix(path, "/messages"), strings.HasSuffix(path, "/anthropic"):
		return APIFormatAnthropic, true
	default:
		return "", false
	}
}
