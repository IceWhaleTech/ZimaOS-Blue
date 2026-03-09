// Package dingtalk provides a DingTalk channel implementation.
package dingtalk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/validator"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

const (
	dtAPIBase    = "https://oapi.dingtalk.com"
	dtNewAPIBase = "https://api.dingtalk.com"
	dtTokenPath  = "/v1.0/oauth2/accessToken"
	dtUploadPath = "/media/upload"
	dtRobotSend  = "/v1.0/robot/oToMessages/batchSend"
)

// Config contains DingTalk channel configuration.
type Config struct {
	Enabled    bool   `yaml:"enabled"`
	AppKey     string `yaml:"app_key"`
	AppSecret  string `yaml:"app_secret"`
	AgentID    string `yaml:"agent_id"`
	RobotCode  string `yaml:"robot_code"`
	WebhookURL string `yaml:"webhook_url"`
	SignSecret string `yaml:"sign_secret"`
}

// Channel implements the channel.Channel interface for DingTalk.
type Channel struct {
	config   Config
	logger   *zap.Logger
	client   *http.Client
	messages chan channel.Message

	mu          sync.RWMutex
	status      channel.Status
	connectedAt *time.Time
	lastError   string
	lastErrorAt *time.Time
	msgCount    atomic.Int64
	msgsSent    atomic.Int64

	// Access token management
	tokenMu     sync.RWMutex
	accessToken string
	tokenExpiry time.Time

	ctx    context.Context
	cancel context.CancelFunc

	sendTextFunc       func(ctx context.Context, chatID, content string) error
	sendAttachmentFunc func(ctx context.Context, chatID, caption string, att channel.Attachment) error
}

// New creates a new DingTalk channel.
func New(cfg Config, logger *zap.Logger) *Channel {
	ch := &Channel{
		config:   cfg,
		logger:   logger.With(zap.String("channel", "dingtalk")),
		client:   &http.Client{Timeout: 30 * time.Second},
		messages: make(chan channel.Message, 100),
		status:   channel.StatusDisconnected,
	}
	ch.sendTextFunc = ch.sendText
	ch.sendAttachmentFunc = ch.sendAttachment
	return ch
}

func (c *Channel) Name() string                     { return "dingtalk" }
func (c *Channel) Type() string                     { return "dingtalk" }
func (c *Channel) Messages() <-chan channel.Message { return c.messages }

func (c *Channel) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status == channel.StatusConnected
}

func (c *Channel) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.status == channel.StatusConnected {
		c.mu.Unlock()
		return fmt.Errorf("channel already started")
	}
	c.ctx, c.cancel = context.WithCancel(ctx)
	now := timeutil.NowTime()
	c.status = channel.StatusConnected
	c.connectedAt = &now
	c.mu.Unlock()
	c.logger.Info("DingTalk channel started")
	return nil
}

func (c *Channel) Stop(ctx context.Context) error {
	c.mu.Lock()
	c.status = channel.StatusDisconnected
	c.mu.Unlock()
	if c.cancel != nil {
		c.cancel()
	}
	close(c.messages)
	c.logger.Info("DingTalk channel stopped")
	return nil
}

// refreshAccessToken obtains an access token from DingTalk new API.
func (c *Channel) refreshAccessToken(ctx context.Context) error {
	c.tokenMu.RLock()
	if c.accessToken != "" && timeutil.NowTime().Before(c.tokenExpiry) {
		c.tokenMu.RUnlock()
		return nil
	}
	c.tokenMu.RUnlock()

	body, _ := json.Marshal(map[string]string{
		"appKey":    c.config.AppKey,
		"appSecret": c.config.AppSecret,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dtNewAPIBase+dtTokenPath, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request access token: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"accessToken"`
		ExpireIn    int    `json:"expireIn"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode token response: %w", err)
	}
	if result.AccessToken == "" {
		return fmt.Errorf("empty access token")
	}

	c.tokenMu.Lock()
	c.accessToken = result.AccessToken
	c.tokenExpiry = timeutil.NowTime().Add(time.Duration(result.ExpireIn-300) * time.Second)
	c.tokenMu.Unlock()
	return nil
}

func (c *Channel) getAccessToken(ctx context.Context) (string, error) {
	if err := c.refreshAccessToken(ctx); err != nil {
		return "", err
	}
	c.tokenMu.RLock()
	defer c.tokenMu.RUnlock()
	return c.accessToken, nil
}

// Send sends a message through DingTalk.
func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	captionConsumed := false
	sentSomething := false
	if c.sendTextFunc == nil {
		c.sendTextFunc = c.sendText
	}
	if c.sendAttachmentFunc == nil {
		c.sendAttachmentFunc = c.sendAttachment
	}

	// Try attachments first.
	for _, att := range msg.Attachments {
		caption := ""
		includeCaption := !captionConsumed && strings.TrimSpace(msg.Content) != ""
		if includeCaption {
			caption = msg.Content
		}
		if err := c.sendAttachmentFunc(ctx, msg.ChatID, caption, att); err != nil {
			c.logger.Warn("failed to send attachment, falling back to text",
				zap.String("channel", "dingtalk"), zap.String("type", string(att.Type)), zap.Error(err))
			fallback := dingtalkAttachmentFallbackText(msg.Content, att, includeCaption)
			if fallback == "" {
				continue
			}
			if err2 := c.sendTextFunc(ctx, msg.ChatID, fallback); err2 != nil {
				return fmt.Errorf("dingtalk send fallback: %w", err2)
			}
			sentSomething = true
			if includeCaption {
				captionConsumed = true
			}
			continue
		}
		sentSomething = true
		if includeCaption {
			captionConsumed = true
		}
	}

	if strings.TrimSpace(msg.Content) != "" && !captionConsumed {
		if err := c.sendTextFunc(ctx, msg.ChatID, msg.Content); err != nil {
			return err
		}
		sentSomething = true
	}
	if !sentSomething {
		return fmt.Errorf("no sendable DingTalk content")
	}
	return nil
}

func dingtalkAttachmentFallbackText(caption string, att channel.Attachment, includeCaption bool) string {
	parts := make([]string, 0, 2)
	if includeCaption && strings.TrimSpace(caption) != "" {
		parts = append(parts, strings.TrimSpace(caption))
	}
	if strings.TrimSpace(att.URL) != "" {
		parts = append(parts, strings.TrimSpace(att.URL))
	} else {
		name := strings.TrimSpace(att.Name)
		if name == "" && len(att.Data) > 0 {
			name = "Attachment"
		}
		if name != "" {
			parts = append(parts, name)
		}
	}
	return strings.Join(parts, "\n")
}

// sendText sends a text message via DingTalk webhook or robot API.
func (c *Channel) sendText(ctx context.Context, chatID, content string) error {
	// Prefer webhook if configured.
	if c.config.WebhookURL != "" {
		return c.sendWebhook(ctx, map[string]interface{}{
			"msgtype": "text",
			"text":    map[string]string{"content": content},
		})
	}

	// Use robot OTO API.
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("get access token: %w", err)
	}
	msgParam, err := marshalJSONString(map[string]string{"content": content})
	if err != nil {
		return fmt.Errorf("marshal text params: %w", err)
	}
	return c.sendRobotOTO(ctx, token, chatID, map[string]interface{}{
		"msgKey":   "sampleText",
		"msgParam": msgParam,
	})
}

// sendAttachment sends a media attachment via DingTalk.
func (c *Channel) sendAttachment(ctx context.Context, chatID, caption string, att channel.Attachment) error {
	// DingTalk webhook supports image via picURL and link messages.
	if c.config.WebhookURL != "" {
		switch att.Type {
		case channel.MessageTypeImage:
			if att.URL != "" {
				return c.sendWebhook(ctx, map[string]interface{}{
					"msgtype": "link",
					"link": map[string]string{
						"title":      att.Name,
						"text":       caption,
						"picUrl":     att.URL,
						"messageUrl": att.URL,
					},
				})
			}
		}
		// For other types, send as markdown with URL or fallback text.
		text := ""
		if att.URL != "" {
			label := strings.TrimSpace(att.Name)
			if label == "" {
				label = "Attachment"
			}
			text = caption
			if text != "" {
				text += "\n"
			}
			text += fmt.Sprintf("[%s](%s)", label, att.URL)
		} else {
			text = dingtalkAttachmentFallbackText(caption, att, caption != "")
		}
		if strings.TrimSpace(text) == "" {
			return fmt.Errorf("attachment has no sendable webhook content")
		}
		return c.sendWebhook(ctx, map[string]interface{}{
			"msgtype":  "markdown",
			"markdown": map[string]string{"title": att.Name, "text": text},
		})
	}

	// Robot OTO API with media upload.
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("get access token: %w", err)
	}

	if len(att.Data) > 0 {
		mediaID, err := c.uploadMedia(ctx, token, att)
		if err != nil {
			return fmt.Errorf("upload media: %w", err)
		}

		var msgKey string
		var msgParam string
		switch att.Type {
		case channel.MessageTypeImage:
			msgKey = "sampleImageMsg"
			msgParam, err = marshalJSONString(map[string]string{"photoURL": mediaID})
		case channel.MessageTypeVideo:
			msgKey = "sampleVideo"
			msgParam, err = marshalJSONString(map[string]string{"mediaId": mediaID, "videoType": "mp4", "duration": "0"})
		case channel.MessageTypeAudio:
			msgKey = "sampleAudio"
			msgParam, err = marshalJSONString(map[string]string{"mediaId": mediaID, "duration": "0"})
		default:
			msgKey = "sampleFile"
			msgParam, err = marshalJSONString(map[string]string{"mediaId": mediaID, "fileName": att.Name})
		}
		if err != nil {
			return fmt.Errorf("marshal attachment params: %w", err)
		}
		return c.sendRobotOTO(ctx, token, chatID, map[string]interface{}{
			"msgKey":   msgKey,
			"msgParam": msgParam,
		})
	}

	// No binary data — send URL/name as text.
	text := dingtalkAttachmentFallbackText(caption, att, caption != "")
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("attachment has no binary data or fallback text")
	}
	return c.sendText(ctx, chatID, text)
}

// uploadMedia uploads a file to DingTalk and returns the media_id.
func (c *Channel) uploadMedia(ctx context.Context, token string, att channel.Attachment) (string, error) {
	filename := att.Name
	if filename == "" {
		filename = "file.bin"
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// type field
	mediaType := "file"
	switch att.Type {
	case channel.MessageTypeImage:
		mediaType = "image"
	case channel.MessageTypeAudio:
		mediaType = "voice"
	case channel.MessageTypeVideo:
		mediaType = "video"
	}
	w.WriteField("type", mediaType)

	part, err := w.CreateFormFile("media", filepath.Base(filename))
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(att.Data); err != nil {
		return "", fmt.Errorf("write data: %w", err)
	}
	w.Close()

	url := fmt.Sprintf("%s%s?access_token=%s", dtAPIBase, dtUploadPath, token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return "", fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		MediaID string `json:"media_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode upload response: %w", err)
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("DingTalk upload error: %d - %s", result.ErrCode, result.ErrMsg)
	}
	return result.MediaID, nil
}

// sendWebhook sends a message via DingTalk webhook.
func (c *Channel) sendWebhook(ctx context.Context, payload map[string]interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if result.ErrCode != 0 {
		return fmt.Errorf("DingTalk webhook error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	c.msgsSent.Add(1)
	return nil
}

// sendRobotOTO sends a one-to-one robot message via DingTalk new API.
func (c *Channel) sendRobotOTO(ctx context.Context, token, userID string, msgBody map[string]interface{}) error {
	payload := map[string]interface{}{
		"robotCode": c.config.RobotCode,
		"userIds":   []string{userID},
	}
	for k, v := range msgBody {
		payload[k] = v
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dtNewAPIBase+dtRobotSend, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-acs-dingtalk-access-token", token)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("send robot message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("DingTalk robot API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	c.msgsSent.Add(1)
	return nil
}

// marshalJSONString marshals v to a JSON string.
func marshalJSONString(v interface{}) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (c *Channel) SendStreaming(ctx context.Context, chatID string, replyToID string, content <-chan string, done chan<- struct{}) error {
	defer close(done)
	var fullContent strings.Builder
	for chunk := range content {
		fullContent.WriteString(chunk)
	}
	if fullContent.Len() > 0 {
		return c.Send(ctx, channel.OutgoingMessage{ChatID: chatID, Content: fullContent.String()})
	}
	return nil
}

func (c *Channel) Info() channel.Info {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return channel.Info{
		Name: "dingtalk", Type: "dingtalk", Status: c.status, Enabled: c.config.Enabled,
		ConnectedAt: c.connectedAt, LastError: c.lastError, MessageCount: c.msgCount.Load(),
		MessagesSent: c.msgsSent.Load(),
		Metadata:     map[string]interface{}{"robot_code": c.config.RobotCode},
	}
}

// Validator for DingTalk configuration.
type Validator struct{}

func NewValidator() *Validator { return &Validator{} }

func (v *Validator) Validate(ctx context.Context, config map[string]string) validator.Result {
	if config["app_key"] == "" || config["app_secret"] == "" {
		return validator.Result{Success: false, Error: "app_key and app_secret are required"}
	}
	return validator.Result{Success: true, MessageKey: "channels.connectionSuccess"}
}
