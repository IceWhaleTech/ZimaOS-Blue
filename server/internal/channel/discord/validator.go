package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/validator"
)

// DiscordAPIBaseURL is the base URL for Discord API.
const DiscordAPIBaseURL = "https://discord.com/api/v10"

// Validator validates Discord bot configuration by calling the /users/@me API.
type Validator struct {
	client  *http.Client
	timeout time.Duration
	baseURL string
}

// NewValidator creates a new Discord validator.
func NewValidator() *Validator {
	return &Validator{
		client: &http.Client{
			Timeout: validator.DefaultTimeout,
		},
		timeout: validator.DefaultTimeout,
		baseURL: DiscordAPIBaseURL,
	}
}

// NewValidatorWithTimeout creates a new Discord validator with custom timeout.
func NewValidatorWithTimeout(timeout time.Duration) *Validator {
	return &Validator{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
		baseURL: DiscordAPIBaseURL,
	}
}

// NewValidatorWithOptions creates a new Discord validator with custom options.
func NewValidatorWithOptions(timeout time.Duration, baseURL string) *Validator {
	if baseURL == "" {
		baseURL = DiscordAPIBaseURL
	}
	return &Validator{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
		baseURL: baseURL,
	}
}

// discordUser represents a Discord user from /users/@me response.
type discordUser struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	GlobalName    string `json:"global_name,omitempty"`
	Avatar        string `json:"avatar,omitempty"`
	Bot           bool   `json:"bot,omitempty"`
	Verified      bool   `json:"verified,omitempty"`
}

// discordError represents a Discord API error response.
type discordError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Validate tests the Discord bot connection by calling /users/@me API.
func (v *Validator) Validate(ctx context.Context, config map[string]string) validator.Result {
	botToken := config["bot_token"]
	if botToken == "" {
		return validator.NewMissingFieldResult("botToken")
	}

	// Create request with context
	url := fmt.Sprintf("%s/users/@me", v.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("failed to create request: %v", err))
	}

	// Set authorization header
	req.Header.Set("Authorization", "Bot "+botToken)
	req.Header.Set("Content-Type", "application/json")

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
		var discordErr discordError
		if err := json.Unmarshal(body, &discordErr); err == nil && discordErr.Message != "" {
			switch resp.StatusCode {
			case http.StatusUnauthorized:
				return validator.NewErrorResult("invalidToken", "The bot token is invalid or has been revoked")
			case http.StatusForbidden:
				return validator.NewErrorResult("invalidToken", "Access forbidden - check bot permissions")
			default:
				return validator.NewErrorResult("connectionFailed", discordErr.Message)
			}
		}
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("API returned status %d", resp.StatusCode))
	}

	// Parse user info
	var userInfo discordUser
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("failed to parse response: %v", err))
	}

	// Return success with bot information
	return validator.NewSuccessResult("testSuccess", map[string]interface{}{
		"bot_id":       userInfo.ID,
		"bot_name":     userInfo.Username,
		"bot_username": userInfo.Username,
		"global_name":  userInfo.GlobalName,
		"is_bot":       userInfo.Bot,
	})
}
