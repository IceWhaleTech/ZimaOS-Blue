package providerpool

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCanonicalAPIFormatForProvider_NonThirdParty(t *testing.T) {
	p := &Provider{
		ID:   "minimax",
		Type: ProviderTypeBuiltin,
	}
	got, detectedURL := autoDetectAPIFormat(context.Background(), p)
	if got != APIFormatAnthropic {
		t.Fatalf("format = %q, want %q", got, APIFormatAnthropic)
	}
	if detectedURL != "" {
		t.Fatalf("detectedURL = %q, want empty", detectedURL)
	}
}

func TestProbeThirdPartyAPIFormat_OpenAIWins(t *testing.T) {
	restore := installProbeTransport(func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			return http.StatusBadRequest, `{"error":"invalid_request"}`
		case "/chat/completions":
			return http.StatusMethodNotAllowed, `{}`
		default:
			return http.StatusNotFound, `{}`
		}
	})
	defer restore()

	p := &Provider{
		ID:      "prov_custom",
		Type:    ProviderTypeCustom,
		BaseURL: "https://example.test",
	}
	got, detectedURL := autoDetectAPIFormat(context.Background(), p)
	if got != APIFormatOpenAI {
		t.Fatalf("format = %q, want %q", got, APIFormatOpenAI)
	}
	if detectedURL != "" {
		t.Fatalf("detectedURL = %q, want empty", detectedURL)
	}
}

func TestProbeThirdPartyAPIFormat_CodexFixedEndpoint(t *testing.T) {
	restore := installProbeTransport(func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/backend-api/codex/responses":
			return http.StatusBadRequest, `{"error":"invalid_request"}`
		default:
			return http.StatusNotFound, `{}`
		}
	})
	defer restore()

	p := &Provider{
		ID:      "custom-codex-like",
		Type:    ProviderTypeCustom,
		BaseURL: "https://chatgpt.com",
	}
	got, detectedURL := autoDetectAPIFormat(context.Background(), p)
	if got != APIFormatResponses {
		t.Fatalf("format = %q, want %q", got, APIFormatResponses)
	}
	wantURL := "https://chatgpt.com/backend-api/codex/responses"
	if detectedURL != wantURL {
		t.Fatalf("detectedURL = %q, want %q", detectedURL, wantURL)
	}
}

func installProbeTransport(fn func(*http.Request) (int, string)) func() {
	oldFactory := newProbeHTTPClient
	newProbeHTTPClient = func(_ bool) *http.Client {
		return &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				status, body := fn(r)
				return &http.Response{
					StatusCode: status,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(body)),
					Request:    r,
				}, nil
			}),
		}
	}
	return func() {
		newProbeHTTPClient = oldFactory
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
