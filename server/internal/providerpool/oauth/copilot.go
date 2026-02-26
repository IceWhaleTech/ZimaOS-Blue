package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DeviceCodeResponse is the response from the device code request.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// RequestDeviceCode initiates the GitHub device code flow.
func RequestDeviceCode(ctx context.Context, cfg *ProviderConfig) (*DeviceCodeResponse, error) {
	data := url.Values{
		"client_id": {cfg.ClientID},
		"scope":     {strings.Join(cfg.Scopes, " ")},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.DeviceCodeURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device code request failed (%d): %s", resp.StatusCode, body)
	}

	var result DeviceCodeResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// PollDeviceToken polls GitHub for the access token after the user authorizes.
// Blocks until the user completes auth, the code expires, or ctx is cancelled.
func PollDeviceToken(ctx context.Context, cfg *ProviderConfig, deviceCode string, interval int) (*tokenResponse, error) {
	if interval < 5 {
		interval = 5
	}
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			token, err := exchangeDeviceCode(ctx, cfg, deviceCode)
			if err != nil {
				// Check for "authorization_pending" — keep polling
				if strings.Contains(err.Error(), "authorization_pending") {
					continue
				}
				// "slow_down" — increase interval
				if strings.Contains(err.Error(), "slow_down") {
					ticker.Reset(time.Duration(interval+5) * time.Second)
					continue
				}
				return nil, err
			}
			return token, nil
		}
	}
}

func exchangeDeviceCode(ctx context.Context, cfg *ProviderConfig, deviceCode string) (*tokenResponse, error) {
	data := url.Values{
		"client_id":   {cfg.ClientID},
		"device_code": {deviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// GitHub returns 200 even for pending/errors with error field in JSON
	var result struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		Scope        string `json:"scope"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("%s: %s", result.Error, result.ErrorDesc)
	}

	if result.AccessToken == "" {
		return nil, fmt.Errorf("no access token in response")
	}

	return &tokenResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    result.ExpiresIn,
		TokenType:    result.TokenType,
		Scope:        result.Scope,
	}, nil
}

// GetCopilotToken exchanges a GitHub OAuth token for a Copilot-specific token.
// GitHub Copilot requires a separate token obtained from the Copilot API.
// Returns: token, expiry time, API endpoint, error
func GetCopilotToken(ctx context.Context, githubToken string) (string, time.Time, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.github.com/copilot_internal/v2/token", nil)
	if err != nil {
		return "", time.Time{}, "", err
	}
	req.Header.Set("Authorization", "token "+githubToken)
	req.Header.Set("Accept", "application/json")
	// GitHub Copilot API requires a specific User-Agent to be recognized as an approved client
	// Using VS Code's user agent as it's the most common approved editor
	req.Header.Set("User-Agent", "GitHubCopilot/1.0 (VSCode/1.85)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", time.Time{}, "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", time.Time{}, "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", time.Time{}, "", fmt.Errorf("copilot token request failed (%d): %s", resp.StatusCode, body)
	}

	var result struct {
		Token     string `json:"token"`
		ExpiresAt int64  `json:"expires_at"`
		Endpoints struct {
			API string `json:"api"`
		} `json:"endpoints"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", time.Time{}, "", err
	}

	// Return endpoint if available, otherwise use default
	endpoint := result.Endpoints.API
	if endpoint == "" {
		endpoint = "https://api.githubcopilot.com"
	}

	return result.Token, time.Unix(result.ExpiresAt, 0), endpoint, nil
}
