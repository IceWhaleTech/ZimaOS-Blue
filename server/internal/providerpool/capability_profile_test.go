package providerpool

import "testing"

func TestDeriveCapabilityProfile_ResponsesGetsFullNative(t *testing.T) {
	provider := &Provider{
		ID:         "prov-responses",
		APIFormat:  APIFormatResponses,
		Type:       ProviderTypeCustom,
		Location:   ProviderLocationCloud,
		Enabled:    true,
		Status:     ProviderStatusActive,
		BaseURL:    "https://relay.example.com/v1/responses",
		DetectedFormat: APIFormatResponses,
	}

	profile := deriveCapabilityProfile(provider, &providerVerificationResult{
		RecommendedAPIFormat: APIFormatResponses,
		ResponsesOnly:        true,
	})
	if profile.ToolCallMode != "full_native" {
		t.Fatalf("ToolCallMode = %q, want full_native", profile.ToolCallMode)
	}
	if !profile.SupportsJSONSchema {
		t.Fatal("SupportsJSONSchema = false, want true")
	}
	if !profile.SupportsStreamingToolCalls {
		t.Fatal("SupportsStreamingToolCalls = false, want true")
	}
	if profile.MaxToolSchemaBytes < 32768 {
		t.Fatalf("MaxToolSchemaBytes = %d, want >= 32768", profile.MaxToolSchemaBytes)
	}
}

func TestDeriveCapabilityProfile_OllamaFallsBackToExecOnly(t *testing.T) {
	provider := &Provider{
		ID:         "prov-ollama",
		APIFormat:  APIFormatOllama,
		Type:       ProviderTypeCustom,
		Location:   ProviderLocationLocal,
		Enabled:    true,
		Status:     ProviderStatusActive,
		BaseURL:    "http://127.0.0.1:11434/v1",
		DetectedFormat: APIFormatOllama,
	}

	profile := deriveCapabilityProfile(provider, nil)
	if profile.ToolCallMode != "exec_only" {
		t.Fatalf("ToolCallMode = %q, want exec_only", profile.ToolCallMode)
	}
	if profile.SupportsJSONSchema {
		t.Fatal("SupportsJSONSchema = true, want false")
	}
	if profile.SupportsParallelToolCalls {
		t.Fatal("SupportsParallelToolCalls = true, want false")
	}
}
