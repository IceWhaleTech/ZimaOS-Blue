package providerpool

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var newProbeHTTPClient = func(skipTLS bool) *http.Client {
	if skipTLS {
		return &http.Client{
			Timeout: 4 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // user-opted skip for self-signed certs
			},
		}
	}
	return &http.Client{Timeout: 4 * time.Second}
}

type formatProbeResult struct {
	format           APIFormat
	score            int
	selectedBaseURL  string
	foundAnyEndpoint bool
}

type formatCandidate struct {
	name         string
	format       APIFormat
	basePaths    []string
	toolPaths    []string
	visionPaths  []string
	fixedBaseURL bool
}

var thirdPartyFormatCandidates = []formatCandidate{
	{
		name:      "openai",
		format:    APIFormatOpenAI,
		basePaths: []string{"/v1/chat/completions", "/chat/completions"},
		toolPaths: []string{"/v1/chat/completions", "/chat/completions"},
		visionPaths: []string{
			"/v1/chat/completions",
			"/chat/completions",
		},
	},
	{
		name:         "codex",
		format:       APIFormatResponses,
		basePaths:    []string{"/backend-api/codex/responses", "/v1/responses", "/responses"},
		toolPaths:    []string{"/backend-api/codex/responses", "/v1/responses", "/responses"},
		visionPaths:  []string{"/backend-api/codex/responses", "/v1/responses", "/responses"},
		fixedBaseURL: true,
	},
	{
		name:      "anthropic",
		format:    APIFormatAnthropic,
		basePaths: []string{"/v1/messages", "/messages"},
		toolPaths: []string{"/v1/messages", "/messages"},
		visionPaths: []string{
			"/v1/messages",
			"/messages",
		},
	},
	{
		name:      "gemini",
		format:    APIFormatGoogle,
		basePaths: []string{"/v1beta/models", "/v1/models"},
		toolPaths: []string{
			"/v1beta/models/gemini-1.5-flash:generateContent",
			"/v1/models/gemini-1.5-flash:generateContent",
		},
		visionPaths: []string{
			"/v1beta/models/gemini-1.5-flash:generateContent",
			"/v1/models/gemini-1.5-flash:generateContent",
		},
	},
}

func activeThirdPartyFormatCandidates() []formatCandidate {
	return thirdPartyFormatCandidates
}

// canonicalAPIFormatForProvider returns the best single API format for built-in/non-third-party providers.
func canonicalAPIFormatForProvider(provider *Provider) APIFormat {
	if provider == nil {
		return APIFormatOpenAI
	}

	if provider.OAuth != nil {
		switch provider.OAuth.ProviderType {
		case "antigravity", "gemini-cli":
			return APIFormatCloudCode
		case "copilot":
			return APIFormatCopilot
		case "codex":
			return APIFormatResponses
		}
	}

	switch provider.ID {
	case "anthropic", "minimax":
		return APIFormatAnthropic
	case "google":
		return APIFormatGoogle
	case "ollama":
		return APIFormatOllama
	case "google-antigravity", "google-gemini-cli":
		return APIFormatCloudCode
	case "github-copilot":
		return APIFormatCopilot
	default:
		return APIFormatOpenAI
	}
}

func isThirdPartyProvider(provider *Provider) bool {
	if provider == nil {
		return false
	}
	return provider.Type == ProviderTypeCustom || strings.HasPrefix(provider.ID, "custom-")
}

func autoDetectAPIFormat(ctx context.Context, provider *Provider) (APIFormat, string) {
	if provider == nil {
		return APIFormatOpenAI, ""
	}
	if !isThirdPartyProvider(provider) {
		return canonicalAPIFormatForProvider(provider), ""
	}
	best := probeThirdPartyAPIFormat(ctx, provider)
	if best.format == "" {
		return APIFormatOpenAI, ""
	}
	return best.format, normalizeDetectedBaseURL(provider.BaseURL, best)
}

func probeThirdPartyAPIFormat(ctx context.Context, provider *Provider) formatProbeResult {
	baseURL := strings.TrimSuffix(provider.BaseURL, "/")
	if baseURL == "" {
		return formatProbeResult{format: APIFormatOpenAI}
	}

	client := newProbeHTTPClient(provider.SkipTLSVerify)

	activeCandidates := activeThirdPartyFormatCandidates()
	results := make([]formatProbeResult, 0, len(activeCandidates))
	for _, c := range activeCandidates {
		r := probeCandidate(ctx, client, provider, baseURL, c)
		if r.foundAnyEndpoint {
			results = append(results, r)
		}
	}
	if len(results) == 0 {
		return formatProbeResult{format: APIFormatOpenAI}
	}

	best := results[0]
	for i := 1; i < len(results); i++ {
		if results[i].score > best.score {
			best = results[i]
			continue
		}
		if results[i].score == best.score && tieBreakPrefer(results[i], best) {
			best = results[i]
		}
	}
	return best
}

func normalizeDetectedBaseURL(originalBaseURL string, best formatProbeResult) string {
	if best.selectedBaseURL == "" {
		return ""
	}
	if best.format != APIFormatResponses {
		return best.selectedBaseURL
	}

	base := strings.TrimSuffix(strings.TrimSpace(originalBaseURL), "/")
	if base == "" {
		return best.selectedBaseURL
	}

	detectedURL, err := url.Parse(best.selectedBaseURL)
	if err != nil {
		return best.selectedBaseURL
	}
	path := strings.TrimSuffix(strings.ToLower(detectedURL.Path), "/")

	// Keep official ChatGPT Codex endpoint rewrite for direct ChatGPT hosts.
	if path == "/backend-api/codex/responses" {
		host := strings.ToLower(detectedURL.Hostname())
		switch host {
		case "chatgpt.com", "chat.openai.com", "openai.com":
			return best.selectedBaseURL
		default:
			return base
		}
	}

	// For generic responses relays, preserve the original base URL so model discovery
	// can use /v1/models or /models successfully.
	if strings.HasSuffix(path, "/responses") {
		return base
	}
	return best.selectedBaseURL
}

func tieBreakPrefer(a, b formatProbeResult) bool {
	rank := map[APIFormat]int{
		APIFormatOpenAI:    4,
		APIFormatResponses: 3,
		APIFormatAnthropic: 2,
		APIFormatGoogle:    1,
	}
	return rank[a.format] > rank[b.format]
}

func probeCandidate(ctx context.Context, client *http.Client, provider *Provider, baseURL string, c formatCandidate) formatProbeResult {
	res := formatProbeResult{format: c.format}

	baseHits := 0
	for _, p := range c.basePaths {
		full := firstURL(baseURL, p)
		exists, _ := endpointExists(ctx, client, provider, c, full)
		if exists {
			baseHits++
			res.foundAnyEndpoint = true
			if c.fixedBaseURL && res.selectedBaseURL == "" {
				res.selectedBaseURL = full
			}
		}
	}
	if baseHits == 0 {
		return res
	}
	res.score += 10 + (baseHits-1)*2

	if probeCapability(ctx, client, provider, c, baseURL, c.toolPaths) {
		res.score += 4
	}
	if probeCapability(ctx, client, provider, c, baseURL, c.visionPaths) {
		res.score += 3
	}

	return res
}

func probeCapability(ctx context.Context, client *http.Client, provider *Provider, c formatCandidate, baseURL string, paths []string) bool {
	for _, p := range paths {
		full := firstURL(baseURL, p)
		status, body, err := probeWithPayload(ctx, client, provider, c, full, true)
		if err != nil {
			continue
		}
		if supportsCapability(c, status, body) {
			return true
		}
	}
	return false
}

func supportsCapability(c formatCandidate, status int, body string) bool {
	if candidateExplicitlyRejected(c, status, body) {
		return false
	}
	if status >= 200 && status < 300 {
		return true
	}
	if status == http.StatusNotFound || status == http.StatusGone {
		return false
	}
	lower := strings.ToLower(body)
	if strings.Contains(lower, "unknown field") ||
		strings.Contains(lower, "unrecognized field") ||
		strings.Contains(lower, "unsupported field") ||
		strings.Contains(lower, "does not support tools") ||
		strings.Contains(lower, "invalid image") {
		return false
	}
	return status == http.StatusUnauthorized ||
		status == http.StatusForbidden ||
		status == http.StatusBadRequest ||
		status == http.StatusMethodNotAllowed
}

func endpointExists(ctx context.Context, client *http.Client, provider *Provider, c formatCandidate, fullURL string) (bool, int) {
	status, body, err := probeWithPayload(ctx, client, provider, c, fullURL, false)
	if err != nil {
		return false, 0
	}
	if status == http.StatusNotFound || status == http.StatusGone {
		return false, status
	}
	if candidateExplicitlyRejected(c, status, body) {
		return false, status
	}
	return true, status
}

func candidateExplicitlyRejected(c formatCandidate, status int, body string) bool {
	switch c.format {
	case APIFormatOpenAI:
		return indicatesResponsesOnlyProvider(status, body)
	case APIFormatResponses:
		return looksLikeProviderHTMLDocument(body)
	default:
		return false
	}
}

func indicatesResponsesOnlyProvider(status int, body string) bool {
	if status < 400 || status >= 600 {
		return false
	}
	lower := strings.ToLower(body)
	if lower == "" {
		return false
	}
	if !(strings.Contains(lower, "/v1/responses") || strings.Contains(lower, "responses api")) {
		return false
	}
	return strings.Contains(lower, "unsupported legacy protocol") ||
		strings.Contains(lower, "legacy protocol") ||
		strings.Contains(lower, "/v1/chat/completions") ||
		strings.Contains(lower, "chat/completions")
}

func looksLikeProviderHTMLDocument(body string) bool {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	return strings.HasPrefix(lower, "<!doctype html") ||
		strings.HasPrefix(lower, "<html") ||
		strings.Contains(lower, "<head") ||
		strings.Contains(lower, "<body")
}

func probeWithPayload(ctx context.Context, client *http.Client, provider *Provider, c formatCandidate, fullURL string, featureProbe bool) (int, string, error) {
	method, body := probeRequestPayload(c.name, featureProbe)
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bytes.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	applyProbeAuth(req, provider, c.format)
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return resp.StatusCode, string(respBody), nil
}

func applyProbeAuth(req *http.Request, provider *Provider, format APIFormat) {
	if provider == nil || len(provider.APIKeys) == 0 || provider.APIKeys[0].Key == "" {
		return
	}
	key := provider.APIKeys[0].Key
	switch format {
	case APIFormatAnthropic:
		if usesMiniMaxAnthropicAuth(provider, req.URL.String(), format) {
			applyMiniMaxAnthropicProbeAuth(req, key)
		} else {
			applyAnthropicProbeAuth(req, key)
		}
	case APIFormatGoogle:
		q := req.URL.Query()
		q.Set("key", key)
		req.URL.RawQuery = q.Encode()
		req.Header.Set("Authorization", "Bearer "+key)
	default:
		req.Header.Set("Authorization", "Bearer "+key)
	}
}

func probeRequestPayload(candidate string, featureProbe bool) (string, []byte) {
	switch candidate {
	case "anthropic":
		if featureProbe {
			return http.MethodPost, []byte(`{"model":"claude-3-5-haiku-20241022","max_tokens":1,"messages":[{"role":"user","content":[{"type":"text","text":"ping"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+5VQAAAAASUVORK5CYII="}}]}],"tools":[{"name":"noop","description":"noop","input_schema":{"type":"object","properties":{}}}]}`)
		}
		return http.MethodPost, []byte(`{"model":"claude-3-5-haiku-20241022","max_tokens":1,"messages":[{"role":"user","content":"ping"}]}`)
	case "gemini":
		if featureProbe {
			return http.MethodPost, []byte(`{"contents":[{"role":"user","parts":[{"text":"ping"},{"inline_data":{"mime_type":"image/png","data":"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+5VQAAAAASUVORK5CYII="}}]}],"tools":[{"function_declarations":[{"name":"noop","description":"noop","parameters":{"type":"object","properties":{}}}]}]}`)
		}
		return http.MethodGet, nil
	case "codex":
		if featureProbe {
			return http.MethodPost, []byte(`{"model":"o4-mini","input":[{"role":"user","content":[{"type":"input_text","text":"ping"},{"type":"input_image","image_url":"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+5VQAAAAASUVORK5CYII="}]}],"tools":[{"type":"function","name":"noop","parameters":{"type":"object","properties":{}}}],"max_output_tokens":1}`)
		}
		return http.MethodPost, []byte(`{"model":"o4-mini","input":"ping","max_output_tokens":1}`)
	default: // openai
		if featureProbe {
			return http.MethodPost, []byte(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":[{"type":"text","text":"ping"},{"type":"image_url","image_url":{"url":"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+5VQAAAAASUVORK5CYII="}}]}],"tools":[{"type":"function","function":{"name":"noop","description":"noop","parameters":{"type":"object","properties":{}}}}],"max_tokens":1}`)
		}
		return http.MethodPost, []byte(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"ping"}],"max_tokens":1}`)
	}
}

func firstURL(baseURL, path string) string {
	base, err := url.Parse(strings.TrimSuffix(baseURL, "/"))
	if err != nil {
		return strings.TrimSuffix(baseURL, "/") + path
	}
	basePath := strings.TrimSuffix(base.Path, "/")
	if basePath == "" {
		base.Path = path
		return base.String()
	}
	if strings.HasPrefix(path, basePath+"/") || path == basePath {
		base.Path = path
		return base.String()
	}
	base.Path = basePath + path
	return base.String()
}
