package providerpool

import "strings"

// CapabilityProfile describes the tool/runtime surface a provider can support.
type CapabilityProfile struct {
	ToolCallMode               string `json:"tool_call_mode,omitempty"`
	SupportsParallelToolCalls  bool   `json:"supports_parallel_tool_calls,omitempty"`
	SupportsStreamingToolCalls bool   `json:"supports_streaming_tool_calls,omitempty"`
	SupportsJSONSchema         bool   `json:"supports_json_schema,omitempty"`
	MaxToolSchemaBytes         int    `json:"max_tool_schema_bytes,omitempty"`
	SupportsVision             bool   `json:"supports_vision,omitempty"`
	SupportsAudio              bool   `json:"supports_audio,omitempty"`
}

func deriveCapabilityProfile(provider *Provider, verification *providerVerificationResult) CapabilityProfile {
	format := APIFormat("")
	switch {
	case verification != nil && verification.RecommendedAPIFormat != "":
		format = verification.RecommendedAPIFormat
	case provider != nil && provider.DetectedFormat != "":
		format = provider.DetectedFormat
	case provider != nil:
		format = provider.APIFormat
	}

	profile := CapabilityProfile{
		ToolCallMode:       "text_only",
		MaxToolSchemaBytes: 8192,
	}

	switch format {
	case APIFormatResponses:
		profile.ToolCallMode = "full_native"
		profile.SupportsParallelToolCalls = true
		profile.SupportsStreamingToolCalls = true
		profile.SupportsJSONSchema = true
		profile.MaxToolSchemaBytes = 65536
	case APIFormatOpenAI, APIFormatAnthropic, APIFormatGoogle, APIFormatCopilot, APIFormatCloudCode:
		profile.ToolCallMode = "lite_native"
		profile.SupportsStreamingToolCalls = true
		profile.SupportsJSONSchema = true
		profile.MaxToolSchemaBytes = 32768
	case APIFormatOllama:
		profile.ToolCallMode = "exec_only"
		profile.MaxToolSchemaBytes = 0
	default:
		if provider != nil && provider.Location == ProviderLocationLocal {
			profile.ToolCallMode = "exec_only"
			profile.MaxToolSchemaBytes = 0
		}
	}

	if verification != nil && verification.ResponsesOnly {
		profile.ToolCallMode = "full_native"
		profile.SupportsParallelToolCalls = true
		profile.SupportsStreamingToolCalls = true
		profile.SupportsJSONSchema = true
		if profile.MaxToolSchemaBytes < 65536 {
			profile.MaxToolSchemaBytes = 65536
		}
	}

	if provider != nil {
		lowerID := strings.ToLower(strings.TrimSpace(provider.ID))
		if strings.Contains(lowerID, "vision") || strings.Contains(lowerID, "gpt-4o") || strings.Contains(lowerID, "gemini") {
			profile.SupportsVision = true
		}
		if provider.Type == ProviderTypeMedia {
			profile.SupportsVision = true
			profile.SupportsAudio = true
		}
	}

	return profile
}
