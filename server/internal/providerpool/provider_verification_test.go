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
		wantAnthropicURL string
		wantChatURL      string
		wantResponsesV1  string
		wantResponsesRaw string
	}{
		{
			name:             "root base",
			baseURL:          "https://relay.example.com",
			wantModelsURL:    "https://relay.example.com/v1/models",
			wantAnthropicURL: "https://relay.example.com/v1/messages",
			wantChatURL:      "https://relay.example.com/v1/chat/completions",
			wantResponsesV1:  "https://relay.example.com/v1/responses",
			wantResponsesRaw: "https://relay.example.com/responses",
		},
		{
			name:             "v1 base",
			baseURL:          "https://relay.example.com/v1",
			wantModelsURL:    "https://relay.example.com/v1/models",
			wantAnthropicURL: "https://relay.example.com/v1/messages",
			wantChatURL:      "https://relay.example.com/v1/chat/completions",
			wantResponsesV1:  "https://relay.example.com/v1/responses",
			wantResponsesRaw: "https://relay.example.com/responses",
		},
		{
			name:             "fixed responses endpoint",
			baseURL:          "https://relay.example.com/v1/responses",
			wantModelsURL:    "https://relay.example.com/v1/models",
			wantAnthropicURL: "https://relay.example.com/v1/messages",
			wantChatURL:      "https://relay.example.com/v1/chat/completions",
			wantResponsesV1:  "https://relay.example.com/v1/responses",
			wantResponsesRaw: "https://relay.example.com/responses",
		},
		{
			name:             "localhost v1 base",
			baseURL:          "http://127.0.0.1:11434/v1",
			wantModelsURL:    "http://127.0.0.1:11434/v1/models",
			wantAnthropicURL: "http://127.0.0.1:11434/v1/messages",
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
			if got.anthropicURL != tt.wantAnthropicURL {
				t.Fatalf("anthropicURL = %q, want %q", got.anthropicURL, tt.wantAnthropicURL)
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

func TestVerifyProviderCandidate_AutoSelectsModelFromModels(t *testing.T) {
	var chatPayload string
	var responsesPayload string

	restoreVerify := installProviderVerifyTransport(func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/v1/models":
			return http.StatusOK, `{"data":[{"id":"auto-picked-model"}]}`
		case "/v1/chat/completions":
			body, _ := io.ReadAll(r.Body)
			chatPayload = string(body)
			return http.StatusBadRequest, `{"error":{"message":"Unsupported legacy protocol: /v1/chat/completions is not supported. Please use /v1/responses."}}`
		case "/v1/responses":
			body, _ := io.ReadAll(r.Body)
			responsesPayload = string(body)
			return http.StatusBadRequest, `{"error":"invalid_request"}`
		case "/responses":
			return http.StatusNotFound, `{"error":"not found"}`
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

	result, err := verifyProviderCandidate(context.Background(), providerVerificationRequest{
		BaseURL: "https://relay.example.com",
		APIKey:  "sk-test",
	})
	if err != nil {
		t.Fatalf("verifyProviderCandidate returned error: %v", err)
	}

	if result.Model != "auto-picked-model" {
		t.Fatalf("result.Model = %q, want %q", result.Model, "auto-picked-model")
	}
	if !strings.Contains(chatPayload, `"model":"auto-picked-model"`) {
		t.Fatalf("chat payload = %q, want auto-picked model", chatPayload)
	}
	if !strings.Contains(responsesPayload, `"model":"auto-picked-model"`) {
		t.Fatalf("responses payload = %q, want auto-picked model", responsesPayload)
	}
}

func TestResolveProviderVerificationModelCandidates_PrefersHighIntelligence(t *testing.T) {
	candidates := resolveProviderVerificationModelCandidates("", `{"data":[{"id":"gpt-4o-mini"},{"id":"gpt-5.3-codex-spark"},{"id":"gpt-5.3-codex"}]}`)
	if len(candidates) < 3 {
		t.Fatalf("candidates length = %d, want >= 3", len(candidates))
	}
	if candidates[0] != "gpt-5.3-codex" {
		t.Fatalf("first candidate = %q, want %q", candidates[0], "gpt-5.3-codex")
	}
	if candidates[1] != "gpt-5.3-codex-spark" {
		t.Fatalf("second candidate = %q, want %q", candidates[1], "gpt-5.3-codex-spark")
	}
	if candidates[2] != "gpt-4o-mini" {
		t.Fatalf("third candidate = %q, want %q", candidates[2], "gpt-4o-mini")
	}
}

func TestExtractModelIDsFromModelsResponse_SupportsCommonShapes(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "id fields",
			body: `{"data":[{"id":"gpt-5.3-codex"}]}`,
			want: []string{"gpt-5.3-codex"},
		},
		{
			name: "name field",
			body: `{"data":[{"name":"claude-3-7-sonnet"}]}`,
			want: []string{"claude-3-7-sonnet"},
		},
		{
			name: "model field",
			body: `{"models":[{"model":"o3-mini"}]}`,
			want: []string{"o3-mini"},
		},
		{
			name: "string array",
			body: `{"data":["qwen-max"]}`,
			want: []string{"qwen-max"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractModelIDsFromModelsResponse(tt.body)
			if len(got) != len(tt.want) {
				t.Fatalf("len(got)=%d, want=%d, got=%v", len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("got[%d]=%q, want=%q, got=%v", i, got[i], tt.want[i], got)
				}
			}
		})
	}
}

func TestVerifyProviderCandidate_FallsBackWhenPreferredModelNotFound(t *testing.T) {
	chatModels := make([]string, 0, 2)

	restoreVerify := installProviderVerifyTransport(func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/v1/models":
			// Input order puts lower-intelligence model first; selection should still try smart-pro first.
			return http.StatusOK, `{"data":[{"id":"cheap-mini"},{"id":"smart-pro"}]}`
		case "/v1/chat/completions":
			body, _ := io.ReadAll(r.Body)
			payload := string(body)
			switch {
			case strings.Contains(payload, `"model":"smart-pro"`):
				chatModels = append(chatModels, "smart-pro")
				return http.StatusBadRequest, `{"error":{"code":"model_not_found","message":"model not found"}}`
			case strings.Contains(payload, `"model":"cheap-mini"`):
				chatModels = append(chatModels, "cheap-mini")
				return http.StatusBadRequest, `{"error":{"message":"Unsupported legacy protocol: /v1/chat/completions is not supported. Please use /v1/responses."}}`
			default:
				return http.StatusBadRequest, `{"error":"invalid_request"}`
			}
		case "/v1/responses":
			body, _ := io.ReadAll(r.Body)
			payload := string(body)
			switch {
			case strings.Contains(payload, `"model":"smart-pro"`):
				return http.StatusNotFound, `{"error":{"message":"no such model"}}`
			case strings.Contains(payload, `"model":"cheap-mini"`):
				return http.StatusOK, `{"status":"completed"}`
			default:
				return http.StatusBadRequest, `{"error":"invalid_request"}`
			}
		case "/responses":
			body, _ := io.ReadAll(r.Body)
			payload := string(body)
			switch {
			case strings.Contains(payload, `"model":"smart-pro"`):
				return http.StatusNotFound, `{"error":{"message":"model not found"}}`
			case strings.Contains(payload, `"model":"cheap-mini"`):
				return http.StatusOK, `{"status":"completed"}`
			default:
				return http.StatusBadRequest, `{"error":"invalid_request"}`
			}
		default:
			return http.StatusNotFound, `{}`
		}
	})
	defer restoreVerify()

	restoreProbe := installProbeTransport(func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			return http.StatusBadRequest, `{"error":"probe"}`
		case "/v1/responses":
			return http.StatusBadRequest, `{"error":"probe"}`
		default:
			return http.StatusNotFound, `{}`
		}
	})
	defer restoreProbe()

	result, err := verifyProviderCandidate(context.Background(), providerVerificationRequest{
		BaseURL: "https://relay.example.com",
		APIKey:  "sk-test",
	})
	if err != nil {
		t.Fatalf("verifyProviderCandidate returned error: %v", err)
	}

	if result.Model != "cheap-mini" {
		t.Fatalf("result.Model = %q, want %q", result.Model, "cheap-mini")
	}
	if len(chatModels) < 2 {
		t.Fatalf("chat model attempts = %v, want at least 2 attempts", chatModels)
	}
	if chatModels[0] != "smart-pro" || chatModels[1] != "cheap-mini" {
		t.Fatalf("chat model attempt order = %v, want [smart-pro cheap-mini]", chatModels)
	}
}

func TestVerifyProviderCandidate_PrioritizesOpenAIOverResponses(t *testing.T) {
	restoreVerify := installProviderVerifyTransport(func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/v1/models":
			return http.StatusOK, `{"data":[{"id":"gpt-4.1"}]}`
		case "/v1/messages":
			return http.StatusNotFound, `{"error":"not found"}`
		case "/v1/chat/completions":
			return http.StatusBadRequest, `{"error":{"message":"invalid_request"}}`
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
			return http.StatusBadRequest, `{"error":"probe"}`
		case "/v1/responses":
			return http.StatusBadRequest, `{"error":"probe"}`
		default:
			return http.StatusNotFound, `{}`
		}
	})
	defer restoreProbe()

	result, err := verifyProviderCandidate(context.Background(), providerVerificationRequest{
		BaseURL: "https://relay.example.com",
		APIKey:  "sk-test",
	})
	if err != nil {
		t.Fatalf("verifyProviderCandidate returned error: %v", err)
	}
	if result.RecommendedAPIFormat != APIFormatOpenAI {
		t.Fatalf("RecommendedAPIFormat = %q, want %q", result.RecommendedAPIFormat, APIFormatOpenAI)
	}
	if result.RecommendedBaseURL != "https://relay.example.com" {
		t.Fatalf("RecommendedBaseURL = %q, want %q", result.RecommendedBaseURL, "https://relay.example.com")
	}
}

func TestVerifyProviderCandidate_PrioritizesAnthropicOverOpenAI(t *testing.T) {
	restoreVerify := installProviderVerifyTransport(func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/v1/models":
			return http.StatusOK, `{"data":[{"id":"claude-3-7-sonnet"}]}`
		case "/v1/messages":
			return http.StatusBadRequest, `{"error":{"message":"invalid_request"}}`
		case "/v1/chat/completions":
			return http.StatusOK, `{"choices":[{"message":{"content":"pong"}}]}`
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
			return http.StatusBadRequest, `{"error":"probe"}`
		case "/v1/responses":
			return http.StatusBadRequest, `{"error":"probe"}`
		default:
			return http.StatusNotFound, `{}`
		}
	})
	defer restoreProbe()

	result, err := verifyProviderCandidate(context.Background(), providerVerificationRequest{
		BaseURL: "https://relay.example.com",
		APIKey:  "sk-test",
	})
	if err != nil {
		t.Fatalf("verifyProviderCandidate returned error: %v", err)
	}
	if result.RecommendedAPIFormat != APIFormatAnthropic {
		t.Fatalf("RecommendedAPIFormat = %q, want %q", result.RecommendedAPIFormat, APIFormatAnthropic)
	}
	if result.RecommendedBaseURL != "https://relay.example.com" {
		t.Fatalf("RecommendedBaseURL = %q, want %q", result.RecommendedBaseURL, "https://relay.example.com")
	}
}

func TestVerifyProviderCandidate_SuppressesMissingModelChatErrorWhenModelOmitted(t *testing.T) {
	restoreVerify := installProviderVerifyTransport(func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/v1/models":
			return http.StatusOK, `{"data":[]}`
		case "/v1/messages":
			return http.StatusNotFound, `{"error":"not found"}`
		case "/v1/chat/completions":
			return http.StatusBadRequest, `{"error":{"message":"未指定模型名称，模型名称不能为空"}}`
		case "/v1/responses":
			return http.StatusBadRequest, `{"error":{"message":"未指定模型名称，模型名称不能为空"}}`
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
			return http.StatusBadRequest, `{"error":"probe"}`
		case "/v1/responses":
			return http.StatusBadRequest, `{"error":"probe"}`
		default:
			return http.StatusNotFound, `{}`
		}
	})
	defer restoreProbe()

	result, err := verifyProviderCandidate(context.Background(), providerVerificationRequest{
		BaseURL: "http://1.95.142.151:3000",
		APIKey:  "sk-test",
	})
	if err != nil {
		t.Fatalf("verifyProviderCandidate returned error: %v", err)
	}
	if result.ChatError != "" {
		t.Fatalf("ChatError = %q, want empty", result.ChatError)
	}
}

func TestVerifyProviderCandidate_OmitsModelWhenNoModelDiscovered(t *testing.T) {
	var chatPayload string
	var responsesPayload string

	restoreVerify := installProviderVerifyTransport(func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/v1/models":
			return http.StatusOK, `{"data":[]}`
		case "/v1/chat/completions":
			body, _ := io.ReadAll(r.Body)
			chatPayload = string(body)
			return http.StatusBadRequest, `{"error":"invalid_request"}`
		case "/v1/responses":
			body, _ := io.ReadAll(r.Body)
			responsesPayload = string(body)
			return http.StatusBadRequest, `{"error":"invalid_request"}`
		case "/responses":
			return http.StatusNotFound, `{"error":"not found"}`
		default:
			return http.StatusNotFound, `{}`
		}
	})
	defer restoreVerify()

	restoreProbe := installProbeTransport(func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			return http.StatusBadRequest, `{"error":"invalid_request"}`
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
	})
	if err != nil {
		t.Fatalf("verifyProviderCandidate returned error: %v", err)
	}

	if result.Model != "" {
		t.Fatalf("result.Model = %q, want empty", result.Model)
	}
	if strings.Contains(chatPayload, `"model"`) {
		t.Fatalf("chat payload = %q, model should be omitted", chatPayload)
	}
	if strings.Contains(responsesPayload, `"model"`) {
		t.Fatalf("responses payload = %q, model should be omitted", responsesPayload)
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
