package tools

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

const (
	defaultMiniMaxImageVisionTimeout = 2 * time.Minute
	minimaxImageVisionSourceHeader   = "MM-API-Source"
	minimaxImageVisionSourceValue    = "ZimaOS-Blue"
)

type miniMaxImageVision struct {
	pool           *providerpool.Pool
	client         *http.Client
	insecureClient *http.Client
}

type miniMaxImageVisionRequest struct {
	Prompt   string `json:"prompt"`
	ImageURL string `json:"image_url"`
}

type miniMaxImageVisionResponse struct {
	BaseResp struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
	Content string `json:"content"`
}

// NewMiniMaxImageVision creates a MiniMax-only direct image-understanding client.
func NewMiniMaxImageVision(pool *providerpool.Pool) ProviderAwareImageVision {
	return &miniMaxImageVision{
		pool: pool,
		client: &http.Client{
			Timeout: defaultMiniMaxImageVisionTimeout,
		},
		insecureClient: &http.Client{
			Timeout: defaultMiniMaxImageVisionTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // user-configured provider may require self-signed TLS
			},
		},
	}
}

func (m *miniMaxImageVision) Analyze(ctx context.Context, prompt, imageBase64 string) (string, bool, error) {
	if !isMiniMaxImageVisionProvider(ctx) {
		return "", false, nil
	}
	if m == nil || m.pool == nil || m.pool.Registry == nil {
		return "", true, errImageVisionUnavailable
	}

	provider, err := m.pool.Registry.Get("minimax")
	if err != nil {
		return "", true, fmt.Errorf("minimax vision provider unavailable: %w", err)
	}
	if provider == nil || !provider.Enabled {
		return "", true, errImageVisionUnavailable
	}

	apiKey := firstEnabledProviderAPIKey(provider)
	if apiKey == "" {
		return "", true, errImageVisionUnavailable
	}

	endpoint, err := miniMaxImageVisionEndpoint(provider.EffectiveBaseURL())
	if err != nil {
		return "", true, err
	}

	reqBody, err := json.Marshal(miniMaxImageVisionRequest{
		Prompt:   strings.TrimSpace(prompt),
		ImageURL: "data:image/png;base64," + strings.TrimSpace(imageBase64),
	})
	if err != nil {
		return "", true, fmt.Errorf("marshal minimax vision request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return "", true, fmt.Errorf("build minimax vision request: %w", err)
	}
	for key, value := range provider.Headers {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		req.Header.Set(key, value)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(minimaxImageVisionSourceHeader, minimaxImageVisionSourceValue)

	resp, err := m.clientFor(provider).Do(req)
	if err != nil {
		return "", true, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", true, fmt.Errorf("read minimax vision response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = resp.Status
		}
		return "", true, fmt.Errorf("minimax vision request failed: %s", message)
	}

	var parsed miniMaxImageVisionResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", true, fmt.Errorf("parse minimax vision response: %w", err)
	}
	if parsed.BaseResp.StatusCode != 0 {
		message := strings.TrimSpace(parsed.BaseResp.StatusMsg)
		if message == "" {
			message = fmt.Sprintf("status_code=%d", parsed.BaseResp.StatusCode)
		}
		return "", true, fmt.Errorf("minimax vision API error: %s", message)
	}

	return strings.TrimSpace(parsed.Content), true, nil
}

func (m *miniMaxImageVision) clientFor(provider *providerpool.Provider) *http.Client {
	if provider != nil && provider.SkipTLSVerify && m.insecureClient != nil {
		return m.insecureClient
	}
	if m != nil && m.client != nil {
		return m.client
	}
	return &http.Client{Timeout: defaultMiniMaxImageVisionTimeout}
}

func isMiniMaxImageVisionProvider(ctx context.Context) bool {
	if strings.EqualFold(strings.TrimSpace(GetProviderID(ctx)), "minimax") {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(GetProvider(ctx)), "minimax")
}

func firstEnabledProviderAPIKey(provider *providerpool.Provider) string {
	if provider == nil {
		return ""
	}
	for _, key := range provider.APIKeys {
		if key.Enabled && strings.TrimSpace(key.Key) != "" {
			return strings.TrimSpace(key.Key)
		}
	}
	for _, key := range provider.APIKeys {
		if strings.TrimSpace(key.Key) != "" {
			return strings.TrimSpace(key.Key)
		}
	}
	return ""
}

func miniMaxImageVisionEndpoint(rawBaseURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawBaseURL))
	if err != nil {
		return "", fmt.Errorf("parse minimax base URL: %w", err)
	}
	if parsed == nil || strings.TrimSpace(parsed.Scheme) == "" || strings.TrimSpace(parsed.Host) == "" {
		return "", fmt.Errorf("invalid minimax base URL: %q", strings.TrimSpace(rawBaseURL))
	}
	return fmt.Sprintf("%s://%s/v1/coding_plan/vlm", parsed.Scheme, parsed.Host), nil
}
