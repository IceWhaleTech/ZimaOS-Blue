package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/validator"
)

// SlackAPIBaseURL is the base URL for Slack API.
const SlackAPIBaseURL = "https://slack.com/api"

// Validator validates Slack bot configuration by calling the auth.test API.
type Validator struct {
	client  *http.Client
	timeout time.Duration
	baseURL string
}

// NewValidator creates a new Slack validator.
func NewValidator() *Validator {
	return &Validator{
		client: &http.Client{
			Timeout: validator.DefaultTimeout,
		},
		timeout: validator.DefaultTimeout,
		baseURL: SlackAPIBaseURL,
	}
}

// NewValidatorWithTimeout creates a new Slack validator with custom timeout.
func NewValidatorWithTimeout(timeout time.Duration) *Validator {
	return &Validator{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
		baseURL: SlackAPIBaseURL,
	}
}

// NewValidatorWithOptions creates a new Slack validator with custom options.
func NewValidatorWithOptions(timeout time.Duration, baseURL string) *Validator {
	if baseURL == "" {
		baseURL = SlackAPIBaseURL
	}
	return &Validator{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
		baseURL: baseURL,
	}
}

// slackAuthTestResponse represents the response from Slack auth.test API.
type slackAuthTestResponse struct {
	OK               bool   `json:"ok"`
	Error            string `json:"error,omitempty"`
	URL              string `json:"url,omitempty"`
	Team             string `json:"team,omitempty"`
	User             string `json:"user,omitempty"`
	TeamID           string `json:"team_id,omitempty"`
	UserID           string `json:"user_id,omitempty"`
	BotID            string `json:"bot_id,omitempty"`
	IsEnterpriseInstall bool `json:"is_enterprise_install,omitempty"`
}

// Validate tests the Slack bot connection by calling auth.test API.
func (v *Validator) Validate(ctx context.Context, config map[string]string) validator.Result {
	botToken := config["bot_token"]
	if botToken == "" {
		return validator.NewMissingFieldResult("botToken")
	}

	// App token is optional but recommended for Socket Mode
	appToken := config["app_token"]

	// Create request with context
	url := fmt.Sprintf("%s/auth.test", v.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("failed to create request: %v", err))
	}

	// Set authorization header
	req.Header.Set("Authorization", "Bearer "+botToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Execute request
	resp, err := v.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return validator.NewErrorResult("timeout", "connection timed out")
		}
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("failed to connect: %v", err))
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("failed to read response: %v", err))
	}

	// Parse response
	var authResp slackAuthTestResponse
	if err := json.Unmarshal(body, &authResp); err != nil {
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("failed to parse response: %v", err))
	}

	// Check if request was successful
	if !authResp.OK {
		switch authResp.Error {
		case "invalid_auth", "not_authed":
			return validator.NewErrorResult("invalidToken", "The bot token is invalid or has been revoked")
		case "token_revoked":
			return validator.NewErrorResult("invalidToken", "The bot token has been revoked")
		case "account_inactive":
			return validator.NewErrorResult("invalidToken", "The workspace has been deactivated")
		default:
			return validator.NewErrorResult("connectionFailed", authResp.Error)
		}
	}

	// Build result data
	data := map[string]interface{}{
		"team_name":    authResp.Team,
		"team_id":      authResp.TeamID,
		"bot_name":     authResp.User,
		"bot_id":       authResp.BotID,
		"user_id":      authResp.UserID,
		"workspace_url": authResp.URL,
	}

	// Add app token status if provided
	if appToken != "" {
		data["app_token_provided"] = true
	}

	return validator.NewSuccessResult("testSuccess", data)
}
