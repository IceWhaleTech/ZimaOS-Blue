package mediagen

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// TestResult holds the result of a media provider connectivity test.
type TestResult struct {
	Healthy   bool   `json:"healthy"`
	LatencyMs int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

// TestMediaProvider probes a media provider to check connectivity.
func TestMediaProvider(config *MediaProviderConfig) *TestResult {
	if config.APIKey == "" {
		return &TestResult{Error: "no API key configured"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	var err error

	switch config.ID {
	case "gemini-image":
		err = probeGemini(ctx, config)
	case "dashscope-image":
		err = probeDashScope(ctx, config)
	case "mulerouter":
		err = probeMuleRouter(ctx, config)
	case "minimax-media":
		err = probeMiniMax(ctx, config)
	default:
		err = probeOpenAICompat(ctx, config)
	}

	latency := time.Since(start).Milliseconds()
	if err != nil {
		return &TestResult{LatencyMs: latency, Error: err.Error()}
	}
	return &TestResult{Healthy: true, LatencyMs: latency}
}

// probeGemini checks Gemini API by listing models with the API key.
func probeGemini(ctx context.Context, config *MediaProviderConfig) error {
	url := fmt.Sprintf("%s/v1beta/models?key=%s&pageSize=1",
		trimRight(config.BaseURL), config.APIKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	return doProbe(req)
}

// probeDashScope checks DashScope API reachability.
func probeDashScope(ctx context.Context, config *MediaProviderConfig) error {
	url := fmt.Sprintf("%s/api/v1/services", trimRight(config.BaseURL))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	return doProbe(req)
}

// probeMuleRouter checks MuleRouter API reachability.
// MuleRouter has no /v1/models endpoint; we probe the base URL and accept any HTTP response.
func probeMuleRouter(ctx context.Context, config *MediaProviderConfig) error {
	url := fmt.Sprintf("%s/vendors", trimRight(config.BaseURL))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	resp.Body.Close()
	// Any HTTP response means the server is reachable
	return nil
}

// probeOpenAICompat checks OpenAI-compatible APIs via /v1/models.
func probeOpenAICompat(ctx context.Context, config *MediaProviderConfig) error {
	url := fmt.Sprintf("%s/v1/models", trimRight(config.BaseURL))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	return doProbe(req)
}

// doProbe executes the HTTP request and checks for a reachable response.
func doProbe(req *http.Request) error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	// 2xx/3xx = healthy, 401/403 = reachable (auth issue but endpoint exists)
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return nil
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("authentication error (status %d)", resp.StatusCode)
	}
	return fmt.Errorf("unexpected status %d", resp.StatusCode)
}

// probeMiniMax checks MiniMax media API reachability.
// Tries both domestic (.chat) and international (.io) domains.
func probeMiniMax(ctx context.Context, config *MediaProviderConfig) error {
	base := trimRight(config.BaseURL)
	urls := []string{fmt.Sprintf("%s/api/v1/video_generation", base)}
	if alt := minimaxMediaAlternateURL(base); alt != "" {
		urls = append(urls, fmt.Sprintf("%s/api/v1/video_generation", alt))
	}

	var lastErr error
	for i, url := range urls {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Authorization", "Bearer "+config.APIKey)
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("connection failed: %w", err)
			continue
		}
		resp.Body.Close()

		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 400:
			if i > 0 {
				config.BaseURL = trimRight(urls[i][:len(urls[i])-len("/api/v1/video_generation")])
			}
			return nil
		case resp.StatusCode == 400 || resp.StatusCode == 401 || resp.StatusCode == 403:
			// Reachable — 400 = missing body, 401/403 = auth issue
			if i > 0 {
				config.BaseURL = trimRight(urls[i][:len(urls[i])-len("/api/v1/video_generation")])
			}
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return fmt.Errorf("authentication error (status %d)", resp.StatusCode)
			}
			return nil
		case resp.StatusCode == 404:
			lastErr = fmt.Errorf("endpoint not found (status 404)")
		default:
			lastErr = fmt.Errorf("unexpected status %d", resp.StatusCode)
		}
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("no reachable endpoint")
}

// minimaxMediaAlternateURL swaps between MiniMax domestic and international domains.
func minimaxMediaAlternateURL(baseURL string) string {
	switch {
	case strings.Contains(baseURL, "api.minimax.chat"):
		return strings.Replace(baseURL, "api.minimax.chat", "api.minimax.io", 1)
	case strings.Contains(baseURL, "api.minimax.io"):
		return strings.Replace(baseURL, "api.minimax.io", "api.minimax.chat", 1)
	default:
		return ""
	}
}

func trimRight(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
