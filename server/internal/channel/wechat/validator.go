package wechat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/validator"
)

// WeChatWorkAPIBaseURL is the base URL for WeChat Work API.
const WeChatWorkAPIBaseURL = "https://qyapi.weixin.qq.com/cgi-bin"

// Validator validates WeChat Work configuration by getting access_token.
type Validator struct {
	client  *http.Client
	timeout time.Duration
	baseURL string
}

// NewValidator creates a new WeChat Work validator.
func NewValidator() *Validator {
	return &Validator{
		client: &http.Client{
			Timeout: validator.DefaultTimeout,
		},
		timeout: validator.DefaultTimeout,
		baseURL: WeChatWorkAPIBaseURL,
	}
}

// NewValidatorWithTimeout creates a new WeChat Work validator with custom timeout.
func NewValidatorWithTimeout(timeout time.Duration) *Validator {
	return &Validator{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
		baseURL: WeChatWorkAPIBaseURL,
	}
}

// NewValidatorWithOptions creates a new WeChat Work validator with custom options.
func NewValidatorWithOptions(timeout time.Duration, baseURL string) *Validator {
	if baseURL == "" {
		baseURL = WeChatWorkAPIBaseURL
	}
	return &Validator{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
		baseURL: baseURL,
	}
}

// wechatTokenResponse represents the response from gettoken API.
type wechatTokenResponse struct {
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	AccessToken string `json:"access_token,omitempty"`
	ExpiresIn   int    `json:"expires_in,omitempty"`
}

// Validate tests the WeChat Work connection by getting access_token.
func (v *Validator) Validate(ctx context.Context, config map[string]string) validator.Result {
	corpID := config["corp_id"]
	if corpID == "" {
		return validator.NewMissingFieldResult("corpId")
	}

	secret := config["secret"]
	if secret == "" {
		return validator.NewMissingFieldResult("secret")
	}

	// Create request with context
	url := fmt.Sprintf("%s/gettoken?corpid=%s&corpsecret=%s", v.baseURL, corpID, secret)
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
	var tokenResp wechatTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("failed to parse response: %v", err))
	}

	// Check if request was successful
	if tokenResp.ErrCode != 0 {
		switch tokenResp.ErrCode {
		case 40001:
			return validator.NewErrorResult("invalidToken", "Invalid secret - authentication failed")
		case 40013:
			return validator.NewErrorResult("invalidToken", "Invalid corp_id - enterprise not found")
		case 40056:
			return validator.NewErrorResult("invalidToken", "Invalid agent_id")
		case 40091:
			return validator.NewErrorResult("invalidToken", "Secret does not match agent_id")
		case 60011:
			return validator.NewErrorResult("invalidToken", "No permission to access this API")
		case 60020:
			return validator.NewErrorResult("invalidToken", "Agent not enabled")
		default:
			return validator.NewErrorResult("connectionFailed", fmt.Sprintf("WeChat Work API error: %s (code: %d)", tokenResp.ErrMsg, tokenResp.ErrCode))
		}
	}

	// Return success with token info
	return validator.NewSuccessResult("testSuccess", map[string]interface{}{
		"corp_id":      corpID,
		"token_expire": tokenResp.ExpiresIn,
		"has_token":    tokenResp.AccessToken != "",
	})
}
