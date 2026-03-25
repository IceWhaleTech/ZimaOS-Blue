package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type cdpVersionResponse struct {
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

// ProbeCDPURL verifies that a CDP endpoint can be resolved into a usable
// websocket URL.
func ProbeCDPURL(ctx context.Context, raw string) error {
	_, err := resolveCDPWebSocketURL(ctx, raw)
	return err
}

func resolveCDPWebSocketURL(ctx context.Context, raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("browser cdp_url is required when using relay driver")
	}

	if !strings.Contains(trimmed, "://") {
		trimmed = "http://" + trimmed
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid browser cdp_url: %w", err)
	}

	switch parsed.Scheme {
	case "ws", "wss":
		return parsed.String(), nil
	case "http", "https":
	default:
		return "", fmt.Errorf("unsupported cdp_url scheme %q", parsed.Scheme)
	}

	if strings.HasSuffix(parsed.Path, "/cdp") {
		wsURL := *parsed
		if wsURL.Scheme == "https" {
			wsURL.Scheme = "wss"
		} else {
			wsURL.Scheme = "ws"
		}
		return wsURL.String(), nil
	}

	versionURL := *parsed
	if versionURL.Path == "" || versionURL.Path == "/" {
		versionURL.Path = "/json/version"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, versionURL.String(), nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("resolve cdp url: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("resolve cdp url: unexpected status %s", resp.Status)
	}

	var payload cdpVersionResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode cdp version response: %w", err)
	}
	if strings.TrimSpace(payload.WebSocketDebuggerURL) == "" {
		return "", fmt.Errorf("resolve cdp url: missing webSocketDebuggerUrl")
	}

	wsURL, err := url.Parse(payload.WebSocketDebuggerURL)
	if err != nil {
		return "", fmt.Errorf("parse resolved websocket url: %w", err)
	}
	if wsURL.RawQuery == "" && parsed.RawQuery != "" {
		wsURL.RawQuery = parsed.RawQuery
	}
	return wsURL.String(), nil
}
