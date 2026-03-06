package providerpool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

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
	anthropicURL string
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

	requestedModel := strings.TrimSpace(req.Model)

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
	modelsStatus, modelsBody, modelsErr := doProviderVerificationRequest(ctx, client, http.MethodGet, urls.modelsURL, "", req.APIKey)
	modelCandidates := resolveProviderVerificationModelCandidates(requestedModel, modelsBody)
	if requestedModel == "" {
		modelCandidates = append(modelCandidates, "")
	}
	if len(modelCandidates) == 0 {
		modelCandidates = []string{""}
	}

	probeModel := ""
	chatStatus := 0
	chatBody := ""
	var chatErr error
	respV1Status := 0
	respV1Body := ""
	var respV1Err error
	respRawStatus := 0
	respRawBody := ""
	var respRawErr error

	for _, candidateModel := range modelCandidates {
		chatStatus, chatBody, chatErr = doProviderVerificationRequest(ctx, client, http.MethodPost, urls.chatURL, openAIChatProbeBody(candidateModel), req.APIKey)
		respV1Status, respV1Body, respV1Err = doProviderVerificationRequest(ctx, client, http.MethodPost, urls.responsesV1, responsesProbeBody(candidateModel), req.APIKey)
		respRawStatus, respRawBody, respRawErr = doProviderVerificationRequest(ctx, client, http.MethodPost, urls.responsesRaw, responsesProbeBody(candidateModel), req.APIKey)
		probeModel = candidateModel

		if candidateModel == "" || !allProbeResultsModelNotFound(chatStatus, chatBody, respV1Status, respV1Body, respRawStatus, respRawBody) {
			break
		}
	}
	anthropicStatus, _, anthropicErr := doProviderVerificationRequest(ctx, client, http.MethodPost, urls.anthropicURL, anthropicProbeBody(probeModel), req.APIKey)

	chatError := extractErrorMessage(chatBody)
	if probeModel == "" && isMissingModelRequiredError(chatError) {
		// Model-less probe can trigger expected parameter errors on strict endpoints.
		chatError = ""
	}
	responsesStatus := extractFieldOrError(respV1Body, "status")
	responsesOnly := indicatesResponsesOnlyProvider(chatStatus, chatBody)
	anthropicReachable := isProviderVerificationReachable(anthropicStatus)
	openAIReachable := isProviderVerificationReachable(chatStatus)
	responsesReachable := isProviderVerificationReachable(respV1Status) || isProviderVerificationReachable(respRawStatus)

	recommendedFormat := detectedFormat
	switch {
	case anthropicReachable:
		recommendedFormat = APIFormatAnthropic
	case openAIReachable && !responsesOnly:
		recommendedFormat = APIFormatOpenAI
	case responsesReachable:
		recommendedFormat = APIFormatResponses
	case openAIReachable:
		recommendedFormat = APIFormatOpenAI
	}

	rootBaseURL := strings.TrimSuffix(urls.modelsURL, "/v1/models")
	if strings.TrimSpace(rootBaseURL) == "" {
		rootBaseURL = baseURL
	}
	recommendedBaseURL := strings.TrimSpace(detectedBaseURL)
	if recommendedBaseURL == "" {
		recommendedBaseURL = rootBaseURL
	}
	if recommendedFormat == APIFormatResponses {
		switch {
		case isProviderVerificationReachable(respV1Status):
			recommendedBaseURL = urls.responsesV1
		case isProviderVerificationReachable(respRawStatus):
			recommendedBaseURL = urls.responsesRaw
		default:
			recommendedBaseURL = rootBaseURL
		}
	} else {
		// For non-responses formats, prefer root base URL rather than a /responses suffix.
		recommendedBaseURL = rootBaseURL
	}
	// responses-only providers should prefer a /responses endpoint base.
	if recommendedFormat == APIFormatResponses && responsesOnly && !strings.HasSuffix(strings.TrimSuffix(recommendedBaseURL, "/"), "/responses") {
		switch {
		case isProviderVerificationReachable(respV1Status):
			recommendedBaseURL = urls.responsesV1
		case isProviderVerificationReachable(respRawStatus):
			recommendedBaseURL = urls.responsesRaw
		}
	}

	result := &providerVerificationResult{
		BaseURL:              baseURL,
		Model:                probeModel,
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
			"anthropic_messages": {
				URL:        urls.anthropicURL,
				StatusCode: anthropicStatus,
				Reachable:  isProviderVerificationReachable(anthropicStatus),
				Error:      sanitizeProbeError(anthropicErr),
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
		anthropicURL: rootForV1 + "/v1/messages",
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

func resolveProviderVerificationModelCandidates(requestedModel, modelsBody string) []string {
	if requestedModel = strings.TrimSpace(requestedModel); requestedModel != "" {
		return []string{requestedModel}
	}
	modelIDs := extractModelIDsFromModelsResponse(modelsBody)
	if len(modelIDs) == 0 {
		return nil
	}

	type scoredModel struct {
		id    string
		score int
		index int
	}

	scored := make([]scoredModel, 0, len(modelIDs))
	for idx, modelID := range modelIDs {
		scored = append(scored, scoredModel{
			id:    modelID,
			score: modelIntelligenceScore(modelID),
			index: idx,
		})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].index < scored[j].index
		}
		return scored[i].score > scored[j].score
	})

	out := make([]string, 0, len(scored))
	for _, item := range scored {
		out = append(out, item.id)
	}
	return out
}

func extractModelIDsFromModelsResponse(body string) []string {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil
	}

	type modelEntry struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Model string `json:"model"`
	}

	var payload struct {
		Data   []modelEntry `json:"data"`
		Models []modelEntry `json:"models"`
	}
	seen := map[string]struct{}{}
	var out []string
	appendUnique := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}

	if err := json.Unmarshal([]byte(body), &payload); err == nil {
		for _, model := range payload.Data {
			appendUnique(model.ID)
			appendUnique(model.Model)
			appendUnique(model.Name)
		}
		for _, model := range payload.Models {
			appendUnique(model.ID)
			appendUnique(model.Model)
			appendUnique(model.Name)
		}
		return out
	}

	var payloadStrings struct {
		Data   []string `json:"data"`
		Models []string `json:"models"`
	}
	if err := json.Unmarshal([]byte(body), &payloadStrings); err == nil {
		for _, model := range payloadStrings.Data {
			appendUnique(model)
		}
		for _, model := range payloadStrings.Models {
			appendUnique(model)
		}
		if len(out) > 0 {
			return out
		}
	}

	var arrayPayload []modelEntry
	if err := json.Unmarshal([]byte(body), &arrayPayload); err == nil {
		for _, model := range arrayPayload {
			appendUnique(model.ID)
			appendUnique(model.Model)
			appendUnique(model.Name)
		}
		return out
	}

	var arrayStrings []string
	if err := json.Unmarshal([]byte(body), &arrayStrings); err == nil {
		for _, model := range arrayStrings {
			appendUnique(model)
		}
		return out
	}

	return nil
}

func modelIntelligenceScore(modelID string) int {
	id := strings.ToLower(strings.TrimSpace(modelID))
	if id == "" {
		return 0
	}

	score := 0
	boosts := []struct {
		key   string
		score int
	}{
		{"gpt-5", 140},
		{"o3", 120},
		{"o1", 95},
		{"opus", 110},
		{"reasoner", 110},
		{"thinking", 95},
		{"sonnet", 80},
		{"pro", 65},
		{"max", 60},
		{"ultra", 60},
		{"codex", 55},
		{"r1", 45},
	}
	for _, item := range boosts {
		if strings.Contains(id, item.key) {
			score += item.score
		}
	}

	penalties := []struct {
		key   string
		score int
	}{
		{"mini", -80},
		{"nano", -95},
		{"lite", -70},
		{"flash", -70},
		{"haiku", -60},
		{"spark", -55},
		{"small", -45},
		{"tiny", -45},
	}
	for _, item := range penalties {
		if strings.Contains(id, item.key) {
			score += item.score
		}
	}

	return score
}

func allProbeResultsModelNotFound(chatStatus int, chatBody string, respV1Status int, respV1Body string, respRawStatus int, respRawBody string) bool {
	return isModelNotFoundProbe(chatStatus, chatBody) &&
		isModelNotFoundProbe(respV1Status, respV1Body) &&
		isModelNotFoundProbe(respRawStatus, respRawBody)
}

func isModelNotFoundProbe(status int, body string) bool {
	if status == 0 {
		return false
	}
	if status != http.StatusBadRequest && status != http.StatusNotFound && status != http.StatusUnprocessableEntity {
		return false
	}
	content := strings.ToLower(strings.TrimSpace(body))
	if content == "" {
		return false
	}

	hints := []string{
		"model_not_found",
		"unknown model",
		"no such model",
		"model not found",
		"model does not exist",
		"invalid model",
		"unsupported model",
		"unavailable model",
	}
	for _, hint := range hints {
		if strings.Contains(content, hint) {
			return true
		}
	}
	return strings.Contains(content, "model") && strings.Contains(content, "not found")
}

func isMissingModelRequiredError(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return false
	}
	hints := []string{
		"model name is required",
		"model is required",
		"missing required field: model",
		"must provide a model",
		"未指定模型名称",
		"模型名称不能为空",
		"缺少模型",
	}
	for _, hint := range hints {
		if strings.Contains(text, strings.ToLower(hint)) {
			return true
		}
	}
	return false
}

func openAIChatProbeBody(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return `{"messages":[{"role":"user","content":"ping"}],"max_tokens":8}`
	}
	return fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"ping"}],"max_tokens":8}`, model)
}

func anthropicProbeBody(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return `{"max_tokens":8,"messages":[{"role":"user","content":[{"type":"text","text":"ping"}]}]}`
	}
	return fmt.Sprintf(`{"model":%q,"max_tokens":8,"messages":[{"role":"user","content":[{"type":"text","text":"ping"}]}]}`, model)
}

func responsesProbeBody(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return `{"input":"ping","max_output_tokens":8}`
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
