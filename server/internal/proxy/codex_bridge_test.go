package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

func TestCodexModelResponsesBridgeOpenAI_Match(t *testing.T) {
	tests := []struct {
		name            string
		requestPath     string
		effectiveFormat providerpool.APIFormat
		modelInBody     string
		wantMatch       bool
	}{
		{
			name:            "gpt-5.3-codex on OpenAI format matches",
			requestPath:    "/v1/chat/completions",
			effectiveFormat: providerpool.APIFormatOpenAI,
			modelInBody:    "gpt-5.3-codex",
			wantMatch:       true,
		},
		{
			name:            "gpt-5.2-codex on OpenAI format matches",
			requestPath:    "/v1/chat/completions",
			effectiveFormat: providerpool.APIFormatOpenAI,
			modelInBody:    "gpt-5.2-codex",
			wantMatch:       true,
		},
		{
			name:            "gpt-5.4 on OpenAI format does NOT match (not a codex model)",
			requestPath:    "/v1/chat/completions",
			effectiveFormat: providerpool.APIFormatOpenAI,
			modelInBody:    "gpt-5.4",
			wantMatch:       false,
		},
		{
			name:            "gpt-4o on OpenAI format does NOT match",
			requestPath:    "/v1/chat/completions",
			effectiveFormat: providerpool.APIFormatOpenAI,
			modelInBody:    "gpt-4o",
			wantMatch:       false,
		},
		{
			name:            "codex model on Responses format does NOT match (handled by other bridge)",
			requestPath:    "/v1/chat/completions",
			effectiveFormat: providerpool.APIFormatResponses,
			modelInBody:    "gpt-5.3-codex",
			wantMatch:       false,
		},
		{
			name:            "non-chat path does NOT match",
			requestPath:    "/v1/models",
			effectiveFormat: providerpool.APIFormatOpenAI,
			modelInBody:    "gpt-5.3-codex",
			wantMatch:       false,
		},
		{
			name:            "case insensitive codex detection",
			requestPath:    "/v1/chat/completions",
			effectiveFormat: providerpool.APIFormatOpenAI,
			modelInBody:    "GPT-5.3-CODEX",
			wantMatch:       true,
		},
	}

	bridge := codexModelResponsesBridgeOpenAI{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &UpstreamRequestBridgeContext{
				RequestPath:    tt.requestPath,
				EffectiveFormat: tt.effectiveFormat,
				Body:           []byte(`{"model":"` + tt.modelInBody + `"}`),
			}
			got := bridge.Match(ctx)
			if got != tt.wantMatch {
				t.Errorf("Match() = %v, want %v", got, tt.wantMatch)
			}
		})
	}
}

func TestCodexModelResponsesBridgeOpenAI_Build(t *testing.T) {
	// Create a mock Responses endpoint server
	responsesServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Errorf("expected request to /v1/responses, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"resp_123","object":"response","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"test"}]}]}`))
	}))
	defer responsesServer.Close()

	// Parse the mock server URL
	parsedURL, err := url.Parse(responsesServer.URL)
	if err != nil {
		t.Fatalf("failed to parse mock URL: %v", err)
	}

	ctx := &UpstreamRequestBridgeContext{
		RequestPath:    "/v1/chat/completions",
		EffectiveFormat: providerpool.APIFormatOpenAI,
		TargetURL:      parsedURL,
		Body:           []byte(`{"model":"gpt-5.3-codex","messages":[{"role":"user","content":"hi"}]}`),
	}

	bridge := codexModelResponsesBridgeOpenAI{}
	if !bridge.Match(ctx) {
		t.Fatal("expected Match() to return true")
	}

	err = bridge.Build(ctx)
	if err != nil {
		t.Fatalf("Build() failed: %v", err)
	}

	if ctx.RequestPath != "/v1/responses" {
		t.Errorf("expected RequestPath to be /v1/responses, got %s", ctx.RequestPath)
	}

	if ctx.EffectiveFormat != providerpool.APIFormatResponses {
		t.Errorf("expected EffectiveFormat to be APIFormatResponses, got %v", ctx.EffectiveFormat)
	}

	// Verify body was converted (should contain "input" instead of "messages")
	if strings.Contains(string(ctx.Body), `"messages"`) {
		t.Error("body should not contain 'messages' after conversion")
	}
	if !strings.Contains(string(ctx.Body), `"input"`) {
		t.Error("body should contain 'input' after conversion")
	}
}

// url.Parse is needed for the test, so we import "net/url" in the actual test file
// This file doesn't use url.Parse directly but tests the bridge logic