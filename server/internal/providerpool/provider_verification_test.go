package providerpool

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

func TestBuildProviderVerificationURLs(t *testing.T) {
	tests := []struct {
		name             string
		baseURL          string
		wantModelsURL    string
		wantChatURL      string
		wantResponsesV1  string
		wantResponsesRaw string
	}{
		{
			name:             "root base",
			baseURL:          "https://relay.example.com",
			wantModelsURL:    "https://relay.example.com/v1/models",
			wantChatURL:      "https://relay.example.com/v1/chat/completions",
			wantResponsesV1:  "https://relay.example.com/v1/responses",
			wantResponsesRaw: "https://relay.example.com/responses",
		},
		{
			name:             "v1 base",
			baseURL:          "https://relay.example.com/v1",
			wantModelsURL:    "https://relay.example.com/v1/models",
			wantChatURL:      "https://relay.example.com/v1/chat/completions",
			wantResponsesV1:  "https://relay.example.com/v1/responses",
			wantResponsesRaw: "https://relay.example.com/responses",
		},
		{
			name:             "fixed responses endpoint",
			baseURL:          "https://relay.example.com/v1/responses",
			wantModelsURL:    "https://relay.example.com/v1/models",
			wantChatURL:      "https://relay.example.com/v1/chat/completions",
			wantResponsesV1:  "https://relay.example.com/v1/responses",
			wantResponsesRaw: "https://relay.example.com/responses",
		},
		{
			name:             "localhost v1 base",
			baseURL:          "http://127.0.0.1:11434/v1",
			wantModelsURL:    "http://127.0.0.1:11434/v1/models",
			wantChatURL:      "http://127.0.0.1:11434/v1/chat/completions",
			wantResponsesV1:  "http://127.0.0.1:11434/v1/responses",
			wantResponsesRaw: "http://127.0.0.1:11434/responses",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildProviderVerificationURLs(tt.baseURL)
			if got.modelsURL != tt.wantModelsURL {
				t.Fatalf("modelsURL = %q, want %q", got.modelsURL, tt.wantModelsURL)
			}
			if got.chatURL != tt.wantChatURL {
				t.Fatalf("chatURL = %q, want %q", got.chatURL, tt.wantChatURL)
			}
			if got.responsesV1 != tt.wantResponsesV1 {
				t.Fatalf("responsesV1 = %q, want %q", got.responsesV1, tt.wantResponsesV1)
			}
			if got.responsesRaw != tt.wantResponsesRaw {
				t.Fatalf("responsesRaw = %q, want %q", got.responsesRaw, tt.wantResponsesRaw)
			}
		})
	}
}

func TestVerifyProviderCandidate_LocalhostBaseURLAndAPIKey(t *testing.T) {
	var verifyAuthHeaders []string
	var verifyHosts []string
	var probeHosts []string

	restoreVerify := installProviderVerifyTransport(func(r *http.Request) (int, string) {
		verifyHosts = append(verifyHosts, r.URL.Host)
		verifyAuthHeaders = append(verifyAuthHeaders, r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/v1/models":
			return http.StatusOK, `{"data":[{"id":"llama3.1"}]}`
		case "/v1/chat/completions":
			return http.StatusOK, `{"choices":[{"message":{"content":"pong"}}]}`
		case "/v1/responses":
			return http.StatusNotFound, `{"error":"not found"}`
		case "/responses":
			return http.StatusNotFound, `{"error":"not found"}`
		default:
			return http.StatusNotFound, `{}`
		}
	})
	defer restoreVerify()

	restoreProbe := installProbeTransport(func(r *http.Request) (int, string) {
		probeHosts = append(probeHosts, r.URL.Host)
		switch r.URL.Path {
		case "/v1/chat/completions":
			return http.StatusBadRequest, `{"error":"probe"}`
		case "/v1/responses":
			return http.StatusNotFound, `{"error":"not found"}`
		default:
			return http.StatusNotFound, `{}`
		}
	})
	defer restoreProbe()

	result, err := verifyProviderCandidate(context.Background(), providerVerificationRequest{
		BaseURL: "http://127.0.0.1:11434/v1/",
		APIKey:  "local-dev-key",
		Model:   "llama3.1",
	})
	if err != nil {
		t.Fatalf("verifyProviderCandidate returned error: %v", err)
	}

	if result.BaseURL != "http://127.0.0.1:11434/v1" {
		t.Fatalf("BaseURL = %q, want %q", result.BaseURL, "http://127.0.0.1:11434/v1")
	}
	if result.RecommendedBaseURL == "" {
		t.Fatal("RecommendedBaseURL should not be empty")
	}
	if len(verifyHosts) == 0 {
		t.Fatal("expected verification requests to be issued")
	}
	for _, host := range verifyHosts {
		if host != "127.0.0.1:11434" {
			t.Fatalf("request host = %q, want %q", host, "127.0.0.1:11434")
		}
	}
	for _, host := range probeHosts {
		if host != "127.0.0.1:11434" {
			t.Fatalf("probe request host = %q, want %q", host, "127.0.0.1:11434")
		}
	}
	for _, auth := range verifyAuthHeaders {
		if auth != "Bearer local-dev-key" {
			t.Fatalf("Authorization header = %q, want %q", auth, "Bearer local-dev-key")
		}
	}
}

func TestVerifyProviderCandidate_ResponsesOnly(t *testing.T) {
	restoreVerify := installProviderVerifyTransport(func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/v1/models":
			return http.StatusOK, `{"data":[{"id":"gpt-5.3-codex"}]}`
		case "/v1/chat/completions":
			return http.StatusBadRequest, `{"error":{"message":"Unsupported legacy protocol: /v1/chat/completions is not supported. Please use /v1/responses."}}`
		case "/backend-api/codex/responses":
			return http.StatusNotFound, `{"error":"not found"}`
		case "/v1/responses":
			return http.StatusOK, `{"status":"completed","output":[{"content":[{"text":"pong"}]}]}`
		case "/responses":
			return http.StatusOK, `{"status":"completed"}`
		default:
			return http.StatusNotFound, `{}`
		}
	})
	defer restoreVerify()

	restoreProbe := installProbeTransport(func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			return http.StatusBadRequest, `{"error":{"message":"Unsupported legacy protocol: /v1/chat/completions is not supported. Please use /v1/responses."}}`
		case "/backend-api/codex/responses":
			return http.StatusNotFound, `{"error":"not found"}`
		case "/v1/responses":
			return http.StatusBadRequest, `{"error":"invalid_request"}`
		default:
			return http.StatusNotFound, `{}`
		}
	})
	defer restoreProbe()

	result, err := verifyProviderCandidate(context.Background(), providerVerificationRequest{
		BaseURL: "https://relay.example.com",
		APIKey:  "sk-test",
		Model:   "gpt-5.3-codex",
	})
	if err != nil {
		t.Fatalf("verifyProviderCandidate returned error: %v", err)
	}

	if result.RecommendedAPIFormat != APIFormatResponses {
		t.Fatalf("RecommendedAPIFormat = %q, want %q", result.RecommendedAPIFormat, APIFormatResponses)
	}
	if result.RecommendedBaseURL != "https://relay.example.com/v1/responses" {
		t.Fatalf("RecommendedBaseURL = %q, want %q", result.RecommendedBaseURL, "https://relay.example.com/v1/responses")
	}
	if !result.ResponsesOnly {
		t.Fatal("ResponsesOnly = false, want true")
	}
	if !strings.Contains(strings.ToLower(result.ChatError), "unsupported legacy protocol") {
		t.Fatalf("ChatError = %q, want legacy protocol message", result.ChatError)
	}
	if got := result.Probes["chat_completions"].StatusCode; got != http.StatusBadRequest {
		t.Fatalf("chat_completions.status = %d, want 400", got)
	}
	if got := result.Probes["responses_v1"].StatusCode; got != http.StatusOK {
		t.Fatalf("responses_v1.status = %d, want 200", got)
	}
}

func TestApplyProviderVerificationRecommendation(t *testing.T) {
	provider := &Provider{
		ID:        "custom",
		BaseURL:   "https://relay.example.com",
		APIFormat: APIFormatOpenAI,
	}
	result := &providerVerificationResult{
		RecommendedAPIFormat: APIFormatResponses,
		RecommendedBaseURL:   "https://relay.example.com/v1/responses",
	}
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	changed := applyProviderVerificationRecommendation(provider, result, now)
	if !changed {
		t.Fatal("expected recommendation to change provider")
	}
	if provider.APIFormat != APIFormatResponses {
		t.Fatalf("APIFormat = %q, want %q", provider.APIFormat, APIFormatResponses)
	}
	if provider.DetectedFormat != APIFormatResponses {
		t.Fatalf("DetectedFormat = %q, want %q", provider.DetectedFormat, APIFormatResponses)
	}
	if provider.BaseURL != "https://relay.example.com/v1/responses" {
		t.Fatalf("BaseURL = %q, want %q", provider.BaseURL, "https://relay.example.com/v1/responses")
	}
	if !provider.DetectedAt.Equal(now) {
		t.Fatalf("DetectedAt = %v, want %v", provider.DetectedAt, now)
	}
}

func TestHandlerVerifyProviderCandidate(t *testing.T) {
	restoreVerify := installProviderVerifyTransport(func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/v1/models":
			return http.StatusOK, `{"data":[{"id":"gpt-5.3-codex"}]}`
		case "/v1/chat/completions":
			return http.StatusBadRequest, `{"error":{"message":"Unsupported legacy protocol: /v1/chat/completions is not supported. Please use /v1/responses."}}`
		case "/v1/responses":
			return http.StatusOK, `{"status":"completed"}`
		case "/responses":
			return http.StatusOK, `{"status":"completed"}`
		default:
			return http.StatusNotFound, `{}`
		}
	})
	defer restoreVerify()

	restoreProbe := installProbeTransport(func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			return http.StatusBadRequest, `{"error":{"message":"Unsupported legacy protocol: /v1/chat/completions is not supported. Please use /v1/responses."}}`
		case "/v1/responses":
			return http.StatusBadRequest, `{"error":"invalid_request"}`
		default:
			return http.StatusNotFound, `{}`
		}
	})
	defer restoreProbe()

	h := newProviderVerificationTestHandler(t)
	e := echo.New()
	body := map[string]interface{}{
		"base_url": "https://relay.example.com",
		"api_key":  "sk-test",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/providers/verify", bytes.NewReader(b))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.VerifyProvider(c); err != nil {
		t.Fatalf("VerifyProvider returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var out providerVerificationResult
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if out.RecommendedAPIFormat != APIFormatResponses {
		t.Fatalf("RecommendedAPIFormat = %q, want %q", out.RecommendedAPIFormat, APIFormatResponses)
	}
	if out.RecommendedBaseURL != "https://relay.example.com/v1/responses" {
		t.Fatalf("RecommendedBaseURL = %q, want %q", out.RecommendedBaseURL, "https://relay.example.com/v1/responses")
	}
}

func TestHandlerVerifyProviderByID_Apply(t *testing.T) {
	restoreVerify := installProviderVerifyTransport(func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/v1/models":
			return http.StatusOK, `{"data":[{"id":"gpt-5.3-codex"}]}`
		case "/v1/chat/completions":
			return http.StatusBadRequest, `{"error":{"message":"Unsupported legacy protocol: /v1/chat/completions is not supported. Please use /v1/responses."}}`
		case "/v1/responses":
			return http.StatusOK, `{"status":"completed"}`
		case "/responses":
			return http.StatusOK, `{"status":"completed"}`
		default:
			return http.StatusNotFound, `{}`
		}
	})
	defer restoreVerify()

	restoreProbe := installProbeTransport(func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			return http.StatusBadRequest, `{"error":{"message":"Unsupported legacy protocol: /v1/chat/completions is not supported. Please use /v1/responses."}}`
		case "/v1/responses":
			return http.StatusBadRequest, `{"error":"invalid_request"}`
		default:
			return http.StatusNotFound, `{}`
		}
	})
	defer restoreProbe()

	h := newProviderVerificationTestHandler(t)
	provider := &Provider{
		ID:        "custom-verify",
		Name:      "Custom Verify",
		Type:      ProviderTypeCustom,
		Enabled:   true,
		BaseURL:   "https://relay.example.com",
		APIFormat: APIFormatOpenAI,
		APIKeys: []APIKey{
			{ID: "k1", Key: "sk-test", Enabled: true},
		},
	}
	if err := h.pool.Registry.Register(provider); err != nil {
		t.Fatalf("register provider failed: %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/providers/custom-verify/verify", strings.NewReader(`{"apply":true}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("custom-verify")

	if err := h.VerifyProviderByID(c); err != nil {
		t.Fatalf("VerifyProviderByID returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	updated, err := h.pool.Registry.Get("custom-verify")
	if err != nil {
		t.Fatalf("registry get failed: %v", err)
	}
	if updated.APIFormat != APIFormatResponses {
		t.Fatalf("APIFormat = %q, want %q", updated.APIFormat, APIFormatResponses)
	}
	if updated.BaseURL != "https://relay.example.com/v1/responses" {
		t.Fatalf("BaseURL = %q, want %q", updated.BaseURL, "https://relay.example.com/v1/responses")
	}
}

func newProviderVerificationTestHandler(t *testing.T) *Handler {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "provider-verify-handler-*")
	if err != nil {
		t.Fatalf("create temp dir failed: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	storage, err := NewFileStorage(tmpDir)
	if err != nil {
		t.Fatalf("create storage failed: %v", err)
	}
	registry, err := NewRegistry(storage)
	if err != nil {
		t.Fatalf("create registry failed: %v", err)
	}

	return NewHandler(&Pool{Registry: registry})
}

func installProviderVerifyTransport(fn func(*http.Request) (int, string)) func() {
	oldFactory := newProviderVerifyHTTPClient
	newProviderVerifyHTTPClient = func(_ bool) *http.Client {
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
		newProviderVerifyHTTPClient = oldFactory
	}
}
