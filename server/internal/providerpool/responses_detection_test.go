package providerpool

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

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

func TestHandlerTestProvider_AllowsGenericResponsesRelay(t *testing.T) {
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
