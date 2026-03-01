package providerpool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultProviderVerificationModel = "gpt-5.3-codex"

type providerVerificationRequest struct {
	BaseURL       string `json:"base_url"`
	APIKey        string `json:"api_key,omitempty"`
	SkipTLSVerify bool   `json:"skip_tls_verify,omitempty"`
	Model         string `json:"model,omitempty"`
}

type providerVerificationProbe struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code,omitempty"`
	Reachable  bool   `json:"reachable"`
	Error      string `json:"error,omitempty"`
}

type providerVerificationResult struct {
	BaseURL              string                               `json:"base_url"`
	Model                string                               `json:"model"`
	DetectedFormat       APIFormat                            `json:"detected_format"`
	RecommendedAPIFormat APIFormat                            `json:"recommended_api_format"`
	RecommendedBaseURL   string                               `json:"recommended_base_url"`
	ResponsesOnly        bool                                 `json:"responses_only"`
	ChatError            string                               `json:"chat_error,omitempty"`
	ResponsesStatus      string                               `json:"responses_status,omitempty"`
	Probes               map[string]providerVerificationProbe `json:"probes"`
}

type providerVerificationURLs struct {
	modelsURL    string
	chatURL      string
	responsesV1  string
	responsesRaw string
}

var newProviderVerifyHTTPClient = func(skipTLS bool) *http.Client {
	// Reuse probe client behavior (including optional TLS skip) for consistent results.
	return newProbeHTTPClient(skipTLS)
}

func verifyProviderCandidate(ctx context.Context, req providerVerificationRequest) (*providerVerificationResult, error) {
	baseURL := strings.TrimSuffix(strings.TrimSpace(req.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("base_url is required")
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = defaultProviderVerificationModel
	}

	urls := buildProviderVerificationURLs(baseURL)

	provider := &Provider{
		ID:            "verify-candidate",
		Type:          ProviderTypeCustom,
		BaseURL:       baseURL,
		SkipTLSVerify: req.SkipTLSVerify,
	}
	if key := strings.TrimSpace(req.APIKey); key != "" {
		provider.APIKeys = []APIKey{{
			ID:      GenerateID("key"),
			Key:     key,
			KeyHash: HashAPIKey(key),
			Enabled: true,
		}}
	}

	detectedFormat, detectedBaseURL := autoDetectAPIFormat(ctx, provider)
	if detectedFormat == "" {
		detectedFormat = APIFormatOpenAI
	}

	client := newProviderVerifyHTTPClient(req.SkipTLSVerify)
	modelsStatus, _, modelsErr := doProviderVerificationRequest(ctx, client, http.MethodGet, urls.modelsURL, "", req.APIKey)
	chatStatus, chatBody, chatErr := doProviderVerificationRequest(ctx, client, http.MethodPost, urls.chatURL, openAIChatProbeBody(model), req.APIKey)
	respV1Status, respV1Body, respV1Err := doProviderVerificationRequest(ctx, client, http.MethodPost, urls.responsesV1, responsesProbeBody(model), req.APIKey)
	respRawStatus, _, respRawErr := doProviderVerificationRequest(ctx, client, http.MethodPost, urls.responsesRaw, responsesProbeBody(model), req.APIKey)

	chatError := extractErrorMessage(chatBody)
	responsesStatus := extractFieldOrError(respV1Body, "status")
	responsesOnly := indicatesResponsesOnlyProvider(chatStatus, chatBody)

	recommendedBaseURL := strings.TrimSpace(detectedBaseURL)
	if recommendedBaseURL == "" {
		switch {
		case isProviderVerificationReachable(respV1Status):
			recommendedBaseURL = urls.responsesV1
		case isProviderVerificationReachable(respRawStatus):
			recommendedBaseURL = urls.responsesRaw
		default:
			recommendedBaseURL = baseURL
		}
	}
	// responses-only providers should prefer a /responses endpoint base.
	if responsesOnly && !strings.HasSuffix(strings.TrimSuffix(recommendedBaseURL, "/"), "/responses") {
		switch {
		case isProviderVerificationReachable(respV1Status):
			recommendedBaseURL = urls.responsesV1
		case isProviderVerificationReachable(respRawStatus):
			recommendedBaseURL = urls.responsesRaw
		}
	}

	recommendedFormat := detectedFormat
	if responsesOnly {
		recommendedFormat = APIFormatResponses
	}

	result := &providerVerificationResult{
		BaseURL:              baseURL,
		Model:                model,
		DetectedFormat:       detectedFormat,
		RecommendedAPIFormat: recommendedFormat,
		RecommendedBaseURL:   recommendedBaseURL,
		ResponsesOnly:        responsesOnly,
		ChatError:            chatError,
		ResponsesStatus:      responsesStatus,
		Probes: map[string]providerVerificationProbe{
			"models": {
				URL:        urls.modelsURL,
				StatusCode: modelsStatus,
				Reachable:  isProviderVerificationReachable(modelsStatus),
				Error:      sanitizeProbeError(modelsErr),
			},
			"chat_completions": {
				URL:        urls.chatURL,
				StatusCode: chatStatus,
				Reachable:  isProviderVerificationReachable(chatStatus),
				Error:      sanitizeProbeError(chatErr),
			},
			"responses_v1": {
				URL:        urls.responsesV1,
				StatusCode: respV1Status,
				Reachable:  isProviderVerificationReachable(respV1Status),
				Error:      sanitizeProbeError(respV1Err),
			},
			"responses_plain": {
				URL:        urls.responsesRaw,
				StatusCode: respRawStatus,
				Reachable:  isProviderVerificationReachable(respRawStatus),
				Error:      sanitizeProbeError(respRawErr),
			},
		},
	}

	return result, nil
}

func applyProviderVerificationRecommendation(provider *Provider, result *providerVerificationResult, now time.Time) bool {
	if provider == nil || result == nil {
		return false
	}

	changed := false
	if result.RecommendedAPIFormat != "" && provider.APIFormat != result.RecommendedAPIFormat {
		provider.APIFormat = result.RecommendedAPIFormat
		provider.DetectedFormat = result.RecommendedAPIFormat
		provider.DetectedAt = now
		changed = true
	}

	if recBaseURL := strings.TrimSpace(result.RecommendedBaseURL); recBaseURL != "" && provider.BaseURL != recBaseURL {
		provider.BaseURL = recBaseURL
		provider.ResetParsedURL()
		changed = true
	}

	if changed {
		provider.UpdatedAt = now
	}
	return changed
}

func doProviderVerificationRequest(ctx context.Context, client *http.Client, method, endpoint, body, apiKey string) (int, string, error) {
	var reader io.Reader
	if strings.TrimSpace(body) != "" {
		reader = bytes.NewBufferString(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return 0, "", err
	}
	if reader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key := strings.TrimSpace(apiKey); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return resp.StatusCode, string(payload), nil
}

func buildProviderVerificationURLs(baseURL string) providerVerificationURLs {
	rootForV1 := baseURL
	respV1URL := baseURL + "/v1/responses"
	respPlainURL := baseURL + "/responses"

	switch {
	case strings.HasSuffix(baseURL, "/v1/responses"):
		rootForV1 = strings.TrimSuffix(baseURL, "/v1/responses")
		respV1URL = baseURL
		respPlainURL = rootForV1 + "/responses"
	case strings.HasSuffix(baseURL, "/responses"):
		rootForV1 = strings.TrimSuffix(baseURL, "/responses")
		respPlainURL = baseURL
		if strings.HasSuffix(rootForV1, "/v1") {
			respV1URL = rootForV1 + "/responses"
		} else {
			respV1URL = rootForV1 + "/v1/responses"
		}
	case strings.HasSuffix(baseURL, "/v1"):
		rootForV1 = strings.TrimSuffix(baseURL, "/v1")
		respV1URL = baseURL + "/responses"
		respPlainURL = rootForV1 + "/responses"
	}

	return providerVerificationURLs{
		modelsURL:    rootForV1 + "/v1/models",
		chatURL:      rootForV1 + "/v1/chat/completions",
		responsesV1:  respV1URL,
		responsesRaw: respPlainURL,
	}
}

func isProviderVerificationReachable(status int) bool {
	switch status {
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusMethodNotAllowed:
		return true
	}
	return status >= 200 && status < 300
}

func extractErrorMessage(body string) string {
	if strings.TrimSpace(body) == "" {
		return ""
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return ""
	}
	if errObj, ok := payload["error"].(map[string]interface{}); ok {
		if msg, ok := errObj["message"].(string); ok {
			return strings.TrimSpace(msg)
		}
	}
	if msg, ok := payload["message"].(string); ok {
		return strings.TrimSpace(msg)
	}
	return ""
}

func extractFieldOrError(body, field string) string {
	if strings.TrimSpace(body) == "" {
		return ""
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return ""
	}
	if val, ok := payload[field].(string); ok {
		return strings.TrimSpace(val)
	}
	if errObj, ok := payload["error"].(map[string]interface{}); ok {
		if msg, ok := errObj["message"].(string); ok {
			return strings.TrimSpace(msg)
		}
	}
	if msg, ok := payload["message"].(string); ok {
		return strings.TrimSpace(msg)
	}
	return ""
}

func sanitizeProbeError(err error) string {
	if err == nil {
		return ""
	}
	return truncateRunes(strings.TrimSpace(err.Error()), 180)
}

func openAIChatProbeBody(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		model = defaultProviderVerificationModel
	}
	return fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"ping"}],"max_tokens":8}`, model)
}

func responsesProbeBody(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		model = defaultProviderVerificationModel
	}
	return fmt.Sprintf(`{"model":%q,"input":"ping","max_output_tokens":8}`, model)
}

func truncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	if len(s) == 0 {
		return ""
	}
	if runeCount := len([]rune(s)); runeCount <= maxRunes {
		return s
	}
	return string([]rune(s)[:maxRunes])
}
