package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/validator"
)

// FeishuAPIBaseURL is the base URL for Feishu Open API.
const FeishuAPIBaseURL = "https://open.feishu.cn/open-apis"

// Validator validates Feishu app configuration by getting tenant_access_token.
type Validator struct {
	client  *http.Client
	timeout time.Duration
	baseURL string
}

// NewValidator creates a new Feishu validator.
func NewValidator() *Validator {
	return &Validator{
		client: &http.Client{
			Timeout: validator.DefaultTimeout,
		},
		timeout: validator.DefaultTimeout,
		baseURL: FeishuAPIBaseURL,
	}
}

// NewValidatorWithTimeout creates a new Feishu validator with custom timeout.
func NewValidatorWithTimeout(timeout time.Duration) *Validator {
	return &Validator{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
		baseURL: FeishuAPIBaseURL,
	}
}

// NewValidatorWithOptions creates a new Feishu validator with custom options.
func NewValidatorWithOptions(timeout time.Duration, baseURL string) *Validator {
	if baseURL == "" {
		baseURL = FeishuAPIBaseURL
	}
	return &Validator{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
		baseURL: baseURL,
	}
}

// feishuTokenRequest represents the request body for tenant_access_token API.
type feishuTokenRequest struct {
	AppID     string `json:"app_id"`
	AppSecret string `json:"app_secret"`
}

// feishuTokenResponse represents the response from tenant_access_token API.
type feishuTokenResponse struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token,omitempty"`
	Expire            int    `json:"expire,omitempty"`
}

// Validate tests the Feishu app connection by getting tenant_access_token.
func (v *Validator) Validate(ctx context.Context, config map[string]string) validator.Result {
	appID := config["app_id"]
	if appID == "" {
		return validator.NewMissingFieldResult("appId")
	}

	appSecret := config["app_secret"]
	if appSecret == "" {
		return validator.NewMissingFieldResult("appSecret")
	}

	// Create request body
	reqBody := feishuTokenRequest{
		AppID:     appID,
		AppSecret: appSecret,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("failed to create request: %v", err))
	}

	// Create request with context
	url := fmt.Sprintf("%s/auth/v3/tenant_access_token/internal", v.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("failed to create request: %v", err))
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")

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
	var tokenResp feishuTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("failed to parse response: %v", err))
	}

	// Check if request was successful
	if tokenResp.Code != 0 {
		switch tokenResp.Code {
		case 10003:
			return validator.NewErrorResult("invalidToken", "Invalid app_id - application not found")
		case 10014:
			return validator.NewErrorResult("invalidToken", "Invalid app_secret - authentication failed")
		case 10015:
			return validator.NewErrorResult("invalidToken", "App has been disabled")
		default:
			return validator.NewErrorResult("connectionFailed", fmt.Sprintf("Feishu API error: %s (code: %d)", tokenResp.Msg, tokenResp.Code))
		}
	}

	// Return success with token info
	return validator.NewSuccessResult("testSuccess", map[string]interface{}{
		"app_id":       appID,
		"token_expire": tokenResp.Expire,
		"has_token":    tokenResp.TenantAccessToken != "",
	})
}
