package bluebubbles

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

// Validator validates BlueBubbles server configuration.
type Validator struct {
	client  *http.Client
	timeout time.Duration
}

// NewValidator creates a new BlueBubbles validator.
func NewValidator() *Validator {
	return &Validator{
		client: &http.Client{
			Timeout: validator.DefaultTimeout,
		},
		timeout: validator.DefaultTimeout,
	}
}

// NewValidatorWithTimeout creates a new BlueBubbles validator with custom timeout.
func NewValidatorWithTimeout(timeout time.Duration) *Validator {
	return &Validator{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

// serverInfoResponse represents the response from /api/v1/server/info.
type serverInfoResponse struct {
	Status  int        `json:"status"`
	Message string     `json:"message"`
	Data    serverData `json:"data,omitempty"`
	Error   *apiError  `json:"error,omitempty"`
}

// serverData contains server information.
type serverData struct {
	OSVersion       string `json:"os_version"`
	ServerVersion   string `json:"server_version"`
	PrivateAPIMode  bool   `json:"private_api_mode"`
	HelperConnected bool   `json:"helper_connected"`
	ProxyService    string `json:"proxy_service"`
	DetectedICloud  string `json:"detected_icloud"`
}

// apiError represents a BlueBubbles API error.
type apiError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// Validate tests the BlueBubbles server connection by calling /api/v1/server/info.
func (v *Validator) Validate(ctx context.Context, config map[string]string) validator.Result {
	serverURL := config["server_url"]
	if serverURL == "" {
		return validator.NewMissingFieldResult("serverUrl")
	}

	password := config["password"]
	if password == "" {
		return validator.NewMissingFieldResult("password")
	}

	// Normalize server URL
	serverURL = strings.TrimSuffix(serverURL, "/")

	// Create request with context
	url := fmt.Sprintf("%s/api/v1/server/info?password=%s", serverURL, password)
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
	var infoResp serverInfoResponse
	if err := json.Unmarshal(body, &infoResp); err != nil {
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("failed to parse response: %v", err))
	}

	// Check for errors
	if infoResp.Status != 200 {
		if infoResp.Error != nil {
			switch infoResp.Error.Type {
			case "Authentication Error", "Unauthorized":
				return validator.NewErrorResult("invalidToken", "Invalid password - authentication failed")
			default:
				return validator.NewErrorResult("connectionFailed", infoResp.Error.Message)
			}
		}
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("Server returned status %d: %s", infoResp.Status, infoResp.Message))
	}

	// Return success with server information
	return validator.NewSuccessResult("testSuccess", map[string]interface{}{
		"server_url":       serverURL,
		"server_version":   infoResp.Data.ServerVersion,
		"os_version":       infoResp.Data.OSVersion,
		"private_api_mode": infoResp.Data.PrivateAPIMode,
		"helper_connected": infoResp.Data.HelperConnected,
		"detected_icloud":  infoResp.Data.DetectedICloud,
	})
}
