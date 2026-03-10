package providerpool

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestValidateResponsesIntegrationAllowed(t *testing.T) {
	SetResponsesIntegrationEnabled(false)
	defer SetResponsesIntegrationEnabled(true)

	if err := validateResponsesIntegrationAllowed(&Provider{BaseURL: "https://chatgpt.com/backend-api/codex/responses"}); err == nil {
		t.Fatal("expected codex responses endpoint to be blocked when integration is disabled")
	}

	if err := validateResponsesIntegrationAllowed(&Provider{ID: "openai", APIFormat: APIFormatOpenAI}); err != nil {
		t.Fatalf("openai provider should remain allowed: %v", err)
	}

	if err := validateResponsesIntegrationAllowed(&Provider{ID: "relay", Type: ProviderTypeCustom, APIFormat: APIFormatResponses, BaseURL: "https://relay.example.com/responses"}); err != nil {
		t.Fatalf("generic relay should remain allowed: %v", err)
	}

	if err := validateResponsesIntegrationAllowed(&Provider{OAuth: &OAuthConfig{ProviderType: "codex"}}); err == nil {
		t.Fatal("expected codex oauth provider to be blocked when integration is disabled")
	}
}

func TestUsesResponsesIntegration(t *testing.T) {
	if !UsesResponsesIntegration(&Provider{BaseURL: "https://chatgpt.com/backend-api/codex/responses"}) {
		t.Fatal("codex responses endpoint base url should be detected")
	}
	if !UsesResponsesIntegration(&Provider{ID: "openai-codex", APIFormat: APIFormatResponses}) {
		t.Fatal("builtin codex provider should be detected")
	}
	if UsesResponsesIntegration(&Provider{ID: "relay", Type: ProviderTypeCustom, APIFormat: APIFormatResponses, BaseURL: "https://relay.example.com/responses"}) {
		t.Fatal("generic responses relay should not be treated as disabled codex integration")
	}
	if UsesResponsesIntegration(&Provider{ID: "openai", APIFormat: APIFormatOpenAI, BaseURL: "https://api.openai.com/v1"}) {
		t.Fatal("plain openai provider should not be treated as responses integration")
	}
}

func TestHandlerTestProvider_AllowsGenericResponsesRelayWhenIntegrationDisabled(t *testing.T) {
	SetResponsesIntegrationEnabled(false)
	defer SetResponsesIntegrationEnabled(true)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/responses", "/responses":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()

	h := newProviderVerificationTestHandler(t)
	provider := &Provider{
		ID:        "generic-relay",
		Name:      "Generic Relay",
		Type:      ProviderTypeCustom,
		Enabled:   true,
		Status:    ProviderStatusActive,
		BaseURL:   upstream.URL,
		APIFormat: APIFormatResponses,
	}
	if err := h.pool.Registry.Register(provider); err != nil {
		t.Fatalf("register provider failed: %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/providers/generic-relay/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("generic-relay")

	if err := h.TestProvider(c); err != nil {
		t.Fatalf("TestProvider returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
}

func TestNormalizeLegacyCustomResponsesProviders(t *testing.T) {
	SetResponsesIntegrationEnabled(false)
	defer SetResponsesIntegrationEnabled(true)

	h := newProviderVerificationTestHandler(t)
	generic := &Provider{
		ID:               "generic-relay",
		Name:             "Generic Relay",
		Type:             ProviderTypeCustom,
		BaseURL:          "https://relay.example.com/v1/responses",
		DetectedEndpoint: "https://relay.example.com/v1/responses",
		APIFormat:        APIFormatResponses,
		DetectedFormat:   APIFormatResponses,
	}
	codex := &Provider{
		ID:        "codex-relay",
		Name:      "Codex Relay",
		Type:      ProviderTypeCustom,
		BaseURL:   "https://chatgpt.com/backend-api/codex/responses",
		APIFormat: APIFormatResponses,
	}
	if err := h.pool.Registry.Register(generic); err != nil {
		t.Fatalf("register generic provider failed: %v", err)
	}
	if err := h.pool.Registry.Register(codex); err != nil {
		t.Fatalf("register codex provider failed: %v", err)
	}

	normalized, err := h.pool.NormalizeLegacyCustomResponsesProviders()
	if err != nil {
		t.Fatalf("NormalizeLegacyCustomResponsesProviders returned error: %v", err)
	}
	if normalized != 1 {
		t.Fatalf("normalized = %d, want 1", normalized)
	}

	updatedGeneric, err := h.pool.Registry.Get("generic-relay")
	if err != nil {
		t.Fatalf("get generic provider failed: %v", err)
	}
	if updatedGeneric.APIFormat != APIFormatOpenAI {
		t.Fatalf("generic APIFormat = %q, want %q", updatedGeneric.APIFormat, APIFormatOpenAI)
	}
	if updatedGeneric.BaseURL != "https://relay.example.com" {
		t.Fatalf("generic BaseURL = %q, want %q", updatedGeneric.BaseURL, "https://relay.example.com")
	}
	if updatedGeneric.DetectedEndpoint != "" {
		t.Fatalf("generic DetectedEndpoint = %q, want empty", updatedGeneric.DetectedEndpoint)
	}
	if updatedGeneric.DetectedFormat != "" {
		t.Fatalf("generic DetectedFormat = %q, want empty", updatedGeneric.DetectedFormat)
	}

	updatedCodex, err := h.pool.Registry.Get("codex-relay")
	if err != nil {
		t.Fatalf("get codex provider failed: %v", err)
	}
	if updatedCodex.APIFormat != APIFormatResponses {
		t.Fatalf("codex APIFormat = %q, want %q", updatedCodex.APIFormat, APIFormatResponses)
	}
	if updatedCodex.BaseURL != "https://chatgpt.com/backend-api/codex/responses" {
		t.Fatalf("codex BaseURL = %q, want unchanged", updatedCodex.BaseURL)
	}
}
