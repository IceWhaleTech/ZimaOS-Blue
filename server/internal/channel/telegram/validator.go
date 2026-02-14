package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/validator"
)

// TelegramAPIBaseURL is the base URL for Telegram Bot API.
const TelegramAPIBaseURL = "https://api.telegram.org"

// Validator validates Telegram bot configuration by calling the getMe API.
type Validator struct {
	client  *http.Client
	timeout time.Duration
	baseURL string
}

// NewValidator creates a new Telegram validator.
func NewValidator() *Validator {
	return &Validator{
		client: &http.Client{
			Timeout: validator.DefaultTimeout,
		},
		timeout: validator.DefaultTimeout,
		baseURL: TelegramAPIBaseURL,
	}
}

// NewValidatorWithTimeout creates a new Telegram validator with custom timeout.
func NewValidatorWithTimeout(timeout time.Duration) *Validator {
	return &Validator{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
		baseURL: TelegramAPIBaseURL,
	}
}

// NewValidatorWithOptions creates a new Telegram validator with custom options.
func NewValidatorWithOptions(timeout time.Duration, baseURL string) *Validator {
	if baseURL == "" {
		baseURL = TelegramAPIBaseURL
	}
	return &Validator{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
		baseURL: baseURL,
	}
}

// telegramResponse represents the response from Telegram API.
type telegramResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result,omitempty"`
	Description string          `json:"description,omitempty"`
	ErrorCode   int             `json:"error_code,omitempty"`
}

// telegramUser represents a Telegram user/bot from getMe response.
type telegramUser struct {
	ID                      int64  `json:"id"`
	IsBot                   bool   `json:"is_bot"`
	FirstName               string `json:"first_name"`
	Username                string `json:"username,omitempty"`
	CanJoinGroups           bool   `json:"can_join_groups,omitempty"`
	CanReadAllGroupMessages bool   `json:"can_read_all_group_messages,omitempty"`
	SupportsInlineQueries   bool   `json:"supports_inline_queries,omitempty"`
}

// Validate tests the Telegram bot connection by calling getMe API.
func (v *Validator) Validate(ctx context.Context, config map[string]string) validator.Result {
	botToken := config["bot_token"]
	if botToken == "" {
		return validator.NewMissingFieldResult("botToken")
	}

	// Create request with context
	url := fmt.Sprintf("%s/bot%s/getMe", v.baseURL, botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("failed to create request: %v", err))
	}

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
	var tgResp telegramResponse
	if err := json.Unmarshal(body, &tgResp); err != nil {
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("failed to parse response: %v", err))
	}

	// Check if request was successful
	if !tgResp.OK {
		// Handle specific error codes
		switch tgResp.ErrorCode {
		case 401:
			return validator.NewErrorResult("invalidToken", "The bot token is invalid or has been revoked")
		case 404:
			return validator.NewErrorResult("invalidToken", "Bot not found - the token may be incorrect")
		default:
			return validator.NewErrorResult("connectionFailed", tgResp.Description)
		}
	}

	// Parse bot info
	var botInfo telegramUser
	if err := json.Unmarshal(tgResp.Result, &botInfo); err != nil {
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("failed to parse bot info: %v", err))
	}

	// Return success with bot information
	return validator.NewSuccessResult("testSuccess", map[string]interface{}{
		"bot_id":       botInfo.ID,
		"bot_name":     botInfo.FirstName,
		"bot_username": botInfo.Username,
		"is_bot":       botInfo.IsBot,
	})
}
