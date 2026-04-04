package providerpool

import "testing"

func TestUsesResponsesIntegration(t *testing.T) {
	if !UsesResponsesIntegration(&Provider{BaseURL: "https://chatgpt.com/backend-api/codex/responses"}) {
		t.Fatal("codex responses endpoint base url should be detected")
	}
	if !UsesResponsesIntegration(&Provider{ID: "openai-codex", APIFormat: APIFormatResponses}) {
		t.Fatal("builtin codex provider should be detected")
	}
	if UsesResponsesIntegration(&Provider{ID: "relay", Type: ProviderTypeCustom, APIFormat: APIFormatResponses, BaseURL: "https://relay.example.com/responses"}) {
		t.Fatal("generic responses relay should not be treated as codex integration")
	}
	if UsesResponsesIntegration(&Provider{ID: "openai", APIFormat: APIFormatOpenAI, BaseURL: "https://api.openai.com/v1"}) {
		t.Fatal("plain openai provider should not be treated as responses integration")
	}
}
