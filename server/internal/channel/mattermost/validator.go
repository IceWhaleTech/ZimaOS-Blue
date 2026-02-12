package mattermost

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/validator"
)

// Validator validates Mattermost bot configuration.
type Validator struct {
	client  *http.Client
	timeout time.Duration
}

// NewValidator creates a new Mattermost validator.
func NewValidator() *Validator {
	return &Validator{
		client: &http.Client{
			Timeout: validator.DefaultTimeout,
		},
		timeout: validator.DefaultTimeout,
	}
}

// NewValidatorWithTimeout creates a new Mattermost validator with custom timeout.
func NewValidatorWithTimeout(timeout time.Duration) *Validator {
	return &Validator{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

// mattermostUser represents a Mattermost user from /api/v4/users/me response.
type mattermostUser struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Nickname  string `json:"nickname"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	IsBot     bool   `json:"is_bot"`
}

// mattermostError represents a Mattermost API error response.
type mattermostError struct {
	ID         string `json:"id"`
	Message    string `json:"message"`
	StatusCode int    `json:"status_code"`
}

// Validate tests the Mattermost bot connection by calling /api/v4/users/me.
func (v *Validator) Validate(ctx context.Context, config map[string]string) validator.Result {
	serverURL := config["server_url"]
	if serverURL == "" {
		return validator.NewMissingFieldResult("serverUrl")
	}

	botToken := config["bot_token"]
	if botToken == "" {
		return validator.NewMissingFieldResult("botToken")
	}

	// Normalize server URL
	serverURL = strings.TrimSuffix(serverURL, "/")

	// Create request with context
	url := fmt.Sprintf("%s/api/v4/users/me", serverURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("failed to create request: %v", err))
	}

	// Set authorization header
	req.Header.Set("Authorization", "Bearer "+botToken)

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

	// Handle error responses
	if resp.StatusCode != http.StatusOK {
		var mmErr mattermostError
		if err := json.Unmarshal(body, &mmErr); err == nil && mmErr.Message != "" {
			switch resp.StatusCode {
			case http.StatusUnauthorized:
				return validator.NewErrorResult("invalidToken", "Invalid bot token - authentication failed")
			case http.StatusForbidden:
				return validator.NewErrorResult("invalidToken", "Access forbidden - check bot permissions")
			default:
				return validator.NewErrorResult("connectionFailed", mmErr.Message)
			}
		}
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("API returned status %d", resp.StatusCode))
	}

	// Parse user info
	var userInfo mattermostUser
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("failed to parse response: %v", err))
	}

	// Return success with bot information
	return validator.NewSuccessResult("testSuccess", map[string]interface{}{
		"bot_id":       userInfo.ID,
		"bot_username": userInfo.Username,
		"bot_name":     userInfo.Nickname,
		"is_bot":       userInfo.IsBot,
		"server_url":   serverURL,
	})
}
