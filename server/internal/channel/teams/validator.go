package teams

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/validator"
)

// MicrosoftLoginURL is the base URL for Microsoft OAuth2.
const MicrosoftLoginURL = "https://login.microsoftonline.com"

// BotFrameworkAPIURL is the base URL for Bot Framework API.
const BotFrameworkAPIURL = "https://smba.trafficmanager.net"

// Validator validates Microsoft Teams bot configuration.
type Validator struct {
	client   *http.Client
	timeout  time.Duration
	loginURL string
}

// NewValidator creates a new Teams validator.
func NewValidator() *Validator {
	return &Validator{
		client: &http.Client{
			Timeout: validator.DefaultTimeout,
		},
		timeout:  validator.DefaultTimeout,
		loginURL: MicrosoftLoginURL,
	}
}

// NewValidatorWithTimeout creates a new Teams validator with custom timeout.
func NewValidatorWithTimeout(timeout time.Duration) *Validator {
	return &Validator{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout:  timeout,
		loginURL: MicrosoftLoginURL,
	}
}

// NewValidatorWithOptions creates a new Teams validator with custom options.
func NewValidatorWithOptions(timeout time.Duration, loginURL string) *Validator {
	if loginURL == "" {
		loginURL = MicrosoftLoginURL
	}
	return &Validator{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout:  timeout,
		loginURL: loginURL,
	}
}

// tokenResponse represents the OAuth2 token response from Microsoft.
type tokenResponse struct {
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	AccessToken  string `json:"access_token"`
	Error        string `json:"error,omitempty"`
	ErrorDesc    string `json:"error_description,omitempty"`
}

// Validate tests the Teams bot connection by acquiring an OAuth2 token.
func (v *Validator) Validate(ctx context.Context, config map[string]string) validator.Result {
	appID := config["app_id"]
	if appID == "" {
		return validator.NewMissingFieldResult("appId")
	}

	appPassword := config["app_password"]
	if appPassword == "" {
		return validator.NewMissingFieldResult("appPassword")
	}

	// Use tenant ID if provided, otherwise use botframework.com for multi-tenant
	tenantID := config["tenant_id"]
	if tenantID == "" {
		tenantID = "botframework.com"
	}

	// Build token request
	tokenURL := fmt.Sprintf("%s/%s/oauth2/v2.0/token", v.loginURL, tenantID)

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", appID)
	data.Set("client_secret", appPassword)
	data.Set("scope", "https://api.botframework.com/.default")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("failed to create request: %v", err))
	}

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
	var tokenResp tokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("failed to parse response: %v", err))
	}

	// Check for errors
	if tokenResp.Error != "" {
		switch tokenResp.Error {
		case "invalid_client":
			return validator.NewErrorResult("invalidToken", "Invalid app ID or password")
		case "unauthorized_client":
			return validator.NewErrorResult("invalidToken", "App is not authorized for this tenant")
		case "invalid_grant":
			return validator.NewErrorResult("invalidToken", "Invalid credentials or expired secret")
		default:
			return validator.NewErrorResult("connectionFailed", fmt.Sprintf("OAuth error: %s - %s", tokenResp.Error, tokenResp.ErrorDesc))
		}
	}

	// Verify we got a token
	if tokenResp.AccessToken == "" {
		return validator.NewErrorResult("connectionFailed", "No access token received")
	}

	// Return success
	return validator.NewSuccessResult("testSuccess", map[string]interface{}{
		"app_id":       appID,
		"tenant_id":    tenantID,
		"token_type":   tokenResp.TokenType,
		"expires_in":   tokenResp.ExpiresIn,
		"has_token":    true,
	})
}

// validateBotRegistration validates that the bot is properly registered (optional additional check).
func (v *Validator) validateBotRegistration(ctx context.Context, accessToken string) error {
	// This would call the Bot Framework API to verify the bot registration
	// For now, we just verify we can get a token which is sufficient
	return nil
}
