package zalo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/validator"
)

// ZaloOAAPIBaseURL is the base URL for Zalo Official Account API.
const ZaloOAAPIBaseURL = "https://openapi.zalo.me"

// Validator validates Zalo Official Account configuration.
type Validator struct {
	client  *http.Client
	timeout time.Duration
	baseURL string
}

// NewValidator creates a new Zalo validator.
func NewValidator() *Validator {
	return &Validator{
		client: &http.Client{
			Timeout: validator.DefaultTimeout,
		},
		timeout: validator.DefaultTimeout,
		baseURL: ZaloOAAPIBaseURL,
	}
}

// NewValidatorWithTimeout creates a new Zalo validator with custom timeout.
func NewValidatorWithTimeout(timeout time.Duration) *Validator {
	return &Validator{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
		baseURL: ZaloOAAPIBaseURL,
	}
}

// NewValidatorWithOptions creates a new Zalo validator with custom options.
func NewValidatorWithOptions(timeout time.Duration, baseURL string) *Validator {
	if baseURL == "" {
		baseURL = ZaloOAAPIBaseURL
	}
	return &Validator{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
		baseURL: baseURL,
	}
}

// oaInfoResponse represents the response from /v2.0/oa/getoa API.
type oaInfoResponse struct {
	Error   int    `json:"error"`
	Message string `json:"message"`
	Data    *oaData `json:"data,omitempty"`
}

// oaData contains Official Account information.
type oaData struct {
	OAID        string `json:"oa_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Avatar      string `json:"avatar"`
	Cover       string `json:"cover"`
	IsVerified  bool   `json:"is_verified"`
	OAType      int    `json:"oa_type"`
	CateID      string `json:"cate_id"`
	NumFollower int    `json:"num_follower"`
}

// Validate tests the Zalo OA connection by calling /v2.0/oa/getoa API.
func (v *Validator) Validate(ctx context.Context, config map[string]string) validator.Result {
	accessToken := config["access_token"]
	if accessToken == "" {
		return validator.NewMissingFieldResult("accessToken")
	}

	// Create request with context
	url := fmt.Sprintf("%s/v2.0/oa/getoa", v.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("failed to create request: %v", err))
	}

	// Set authorization header
	req.Header.Set("access_token", accessToken)

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
	var oaResp oaInfoResponse
	if err := json.Unmarshal(body, &oaResp); err != nil {
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("failed to parse response: %v", err))
	}

	// Check for errors
	if oaResp.Error != 0 {
		switch oaResp.Error {
		case -124: // Invalid access token
			return validator.NewErrorResult("invalidToken", "Invalid access token")
		case -216: // Access token expired
			return validator.NewErrorResult("invalidToken", "Access token has expired - please refresh")
		case -201: // Invalid OA
			return validator.NewErrorResult("invalidToken", "Official Account not found or invalid")
		default:
			return validator.NewErrorResult("connectionFailed", fmt.Sprintf("Zalo API error %d: %s", oaResp.Error, oaResp.Message))
		}
	}

	// Verify we got OA data
	if oaResp.Data == nil {
		return validator.NewErrorResult("connectionFailed", "No OA data received")
	}

	// Return success with OA information
	return validator.NewSuccessResult("testSuccess", map[string]interface{}{
		"oa_id":        oaResp.Data.OAID,
		"oa_name":      oaResp.Data.Name,
		"is_verified":  oaResp.Data.IsVerified,
		"num_follower": oaResp.Data.NumFollower,
		"oa_type":      oaResp.Data.OAType,
	})
}
