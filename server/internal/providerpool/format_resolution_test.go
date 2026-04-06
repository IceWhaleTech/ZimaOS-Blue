package providerpool

import "testing"

func TestResolveAPIFormatPlan_UnknownOpenAIModelPrefersResponses(t *testing.T) {
	provider := GetBuiltinProvider("openai")
	if provider == nil {
		t.Fatal("expected builtin openai provider")
	}

	plan := ResolveAPIFormatPlan(FormatResolutionRequest{
		Provider: provider,
		ModelID:  "gpt-5.5-preview",
	})

	if len(plan.CandidateFormats) == 0 {
		t.Fatal("candidate formats should not be empty")
	}
	if got := plan.CandidateFormats[0]; got != APIFormatResponses {
		t.Fatalf("first candidate format = %q, want %q", got, APIFormatResponses)
	}
	if got := plan.SelectedFormat; got != APIFormatResponses {
		t.Fatalf("selected format = %q, want %q", got, APIFormatResponses)
	}
}

func TestResolveAPIFormatPlan_KnownOpenAIModelKeepsOpenAIPreference(t *testing.T) {
	provider := GetBuiltinProvider("openai")
	if provider == nil {
		t.Fatal("expected builtin openai provider")
	}

	plan := ResolveAPIFormatPlan(FormatResolutionRequest{
		Provider: provider,
		ModelID:  "gpt-5.4",
	})

	if len(plan.CandidateFormats) == 0 {
		t.Fatal("candidate formats should not be empty")
	}
	if got := plan.CandidateFormats[0]; got != APIFormatOpenAI {
		t.Fatalf("first candidate format = %q, want %q", got, APIFormatOpenAI)
	}
	if got := plan.SelectedFormat; got != APIFormatOpenAI {
		t.Fatalf("selected format = %q, want %q", got, APIFormatOpenAI)
	}
}

func TestResolveAPIFormatPlan_UnknownOpenAIModelOnCustomOpenAIBaseURLPrefersResponses(t *testing.T) {
	provider := &Provider{
		ID:       "custom-openai-direct",
		Type:     ProviderTypeCustom,
		BaseURL:  "https://api.openai.com/v1",
		Website:  "https://openai.com",
		Enabled:  true,
		Location: ProviderLocationCloud,
	}

	plan := ResolveAPIFormatPlan(FormatResolutionRequest{
		Provider: provider,
		ModelID:  "gpt-5.5-preview",
	})

	if len(plan.CandidateFormats) == 0 {
		t.Fatal("candidate formats should not be empty")
	}
	if got := plan.CandidateFormats[0]; got != APIFormatResponses {
		t.Fatalf("first candidate format = %q, want %q", got, APIFormatResponses)
	}
}

func TestResolveAPIFormatPlan_UnknownThirdPartyModelKeepsOpenAIPreference(t *testing.T) {
	provider := &Provider{
		ID:       "custom-relay",
		Type:     ProviderTypeCustom,
		BaseURL:  "https://relay.example.com/v1",
		Enabled:  true,
		Location: ProviderLocationCloud,
	}

	plan := ResolveAPIFormatPlan(FormatResolutionRequest{
		Provider: provider,
		ModelID:  "gpt-5.5-preview",
	})

	if len(plan.CandidateFormats) == 0 {
		t.Fatal("candidate formats should not be empty")
	}
	if got := plan.CandidateFormats[0]; got != APIFormatOpenAI {
		t.Fatalf("first candidate format = %q, want %q", got, APIFormatOpenAI)
	}
}
