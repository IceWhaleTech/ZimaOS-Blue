package matrix

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/validator"
)

// Validator validates Matrix configuration by calling the whoami API.
type Validator struct {
	client  *http.Client
	timeout time.Duration
}

// NewValidator creates a new Matrix validator.
func NewValidator() *Validator {
	return &Validator{
		client: &http.Client{
			Timeout: validator.DefaultTimeout,
		},
		timeout: validator.DefaultTimeout,
	}
}

// NewValidatorWithTimeout creates a new Matrix validator with custom timeout.
func NewValidatorWithTimeout(timeout time.Duration) *Validator {
	return &Validator{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

// matrixWhoamiResponse represents the response from /_matrix/client/v3/account/whoami API.
type matrixWhoamiResponse struct {
	UserID   string `json:"user_id"`
	DeviceID string `json:"device_id,omitempty"`
	IsGuest  bool   `json:"is_guest,omitempty"`
}

// matrixErrorResponse represents a Matrix API error response.
type matrixErrorResponse struct {
	ErrCode string `json:"errcode"`
	Error   string `json:"error"`
}

// Validate tests the Matrix connection by calling whoami API.
func (v *Validator) Validate(ctx context.Context, config map[string]string) validator.Result {
	homeserver := config["homeserver"]
	if homeserver == "" {
		return validator.NewMissingFieldResult("homeserver")
	}

	accessToken := config["access_token"]
	if accessToken == "" {
		return validator.NewMissingFieldResult("accessToken")
	}

	// Normalize homeserver URL
	homeserver = strings.TrimSuffix(homeserver, "/")

	// Create request with context
	url := fmt.Sprintf("%s/_matrix/client/v3/account/whoami", homeserver)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("failed to create request: %v", err))
	}

	// Set authorization header
	req.Header.Set("Authorization", "Bearer "+accessToken)

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
		var matrixErr matrixErrorResponse
		if err := json.Unmarshal(body, &matrixErr); err == nil && matrixErr.ErrCode != "" {
			switch matrixErr.ErrCode {
			case "M_UNKNOWN_TOKEN":
				return validator.NewErrorResult("invalidToken", "The access token is invalid or has expired")
			case "M_MISSING_TOKEN":
				return validator.NewErrorResult("invalidToken", "No access token provided")
			case "M_FORBIDDEN":
				return validator.NewErrorResult("invalidToken", "Access forbidden - check permissions")
			case "M_USER_DEACTIVATED":
				return validator.NewErrorResult("invalidToken", "User account has been deactivated")
			default:
				return validator.NewErrorResult("connectionFailed", fmt.Sprintf("Matrix error: %s - %s", matrixErr.ErrCode, matrixErr.Error))
			}
		}
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("API returned status %d", resp.StatusCode))
	}

	// Parse whoami response
	var whoami matrixWhoamiResponse
	if err := json.Unmarshal(body, &whoami); err != nil {
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("failed to parse response: %v", err))
	}

	// Return success with user info
	return validator.NewSuccessResult("testSuccess", map[string]interface{}{
		"user_id":    whoami.UserID,
		"device_id":  whoami.DeviceID,
		"is_guest":   whoami.IsGuest,
		"homeserver": homeserver,
	})
}
