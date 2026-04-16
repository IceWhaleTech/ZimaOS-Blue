package wechatilink

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/validator"
)

// Validator validates WeChat iLink configuration by probing getupdates.
type Validator struct {
	client  *http.Client
	timeout time.Duration
	baseURL string
}

func NewValidator() *Validator {
	return &Validator{
		client: &http.Client{
			Timeout: validator.DefaultTimeout,
		},
		timeout: validator.DefaultTimeout,
	}
}

func NewValidatorWithTimeout(timeout time.Duration) *Validator {
	return &Validator{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

func NewValidatorWithOptions(timeout time.Duration, baseURL string) *Validator {
	return &Validator{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
		baseURL: baseURL,
	}
}

func (v *Validator) Validate(ctx context.Context, config map[string]string) validator.Result {
	baseURL := strings.TrimSpace(config["api_base_url"])
	if baseURL == "" {
		return validator.NewMissingFieldResult("apiBaseURL")
	}

	botToken := strings.TrimSpace(config["bot_token"])
	if botToken == "" {
		return validator.NewMissingFieldResult("botToken")
	}

	targetURL := resolveILinkBotBaseURL(baseURL) + "/getupdates"
	if strings.TrimSpace(v.baseURL) != "" {
		targetURL = resolveILinkBotBaseURL(v.baseURL) + "/getupdates"
	}

	payload := []byte(`{"get_updates_buf":""}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(payload))
	if err != nil {
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("failed to create request: %v", err))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("AuthorizationType", "ilink_bot_token")
	req.Header.Set("Authorization", "Bearer "+botToken)
	req.Header.Set("X-WECHAT-UIN", validatorILinkUINHeader())

	resp, err := v.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return validator.NewErrorResult("timeout", "connection timed out")
		}
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("failed to connect: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return validator.NewErrorResult("invalidToken", "iLink authentication failed")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("failed to read response: %v", err))
	}

	var result iLinkGetUpdatesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return validator.NewErrorResult("connectionFailed", fmt.Sprintf("failed to parse response: %v", err))
	}
	if err := validateILinkResponse(result.iLinkResponseEnvelope); err != nil {
		return validator.NewErrorResult("connectionFailed", err.Error())
	}

	return validator.NewSuccessResult("testSuccess", map[string]interface{}{
		"api_base_url":    baseURL,
		"has_cursor":      result.GetUpdatesBuf != "",
		"long_polling_ms": result.LongPollingTimeoutMS,
	})
}

func validatorILinkUINHeader() string {
	var raw [4]byte
	binary.BigEndian.PutUint32(raw[:], uint32(time.Now().UnixNano()))
	return base64.StdEncoding.EncodeToString(raw[:])
}
