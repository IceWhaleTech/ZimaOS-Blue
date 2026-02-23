// Package viber provides a Viber REST API channel implementation.
package viber

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/validator"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// Config contains Viber channel configuration.
type Config struct {
	Enabled    bool   `yaml:"enabled"`
	AuthToken  string `yaml:"auth_token"`
	BotName    string `yaml:"bot_name"`
	BotAvatar  string `yaml:"bot_avatar"`
	WebhookURL string `yaml:"webhook_url"`
}

const (
	viberAPIBase       = "https://chatapi.viber.com/pa"
	viberSendURL       = viberAPIBase + "/send_message"
	viberSetWebhookURL = viberAPIBase + "/set_webhook"
	viberAccountURL    = viberAPIBase + "/get_account_info"

	// maxTextLength is the Viber limit for a single text message.
	maxTextLength = 7000
)

// --- Viber callback types ---

type callbackEvent struct {
	Event        string           `json:"event"`
	Timestamp    int64            `json:"timestamp"`
	MessageToken int64            `json:"message_token"`
	Sender       *callbackSender  `json:"sender,omitempty"`
	Message      *callbackMessage `json:"message,omitempty"`
	UserID       string           `json:"user_id,omitempty"`
}

type callbackSender struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar,omitempty"`
}

type callbackMessage struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Media    string `json:"media,omitempty"`
	FileName string `json:"file_name,omitempty"`
	FileSize int64  `json:"file_size,omitempty"`
}

// --- Viber send payload types ---

type sendPayload struct {
	Receiver  string       `json:"receiver"`
	Type      string       `json:"type"`
	Text      string       `json:"text,omitempty"`
	Media     string       `json:"media,omitempty"`
	FileName  string       `json:"file_name,omitempty"`
	FileSize  int64        `json:"file_size,omitempty"`
	Thumbnail string       `json:"thumbnail,omitempty"`
	Sender    senderInfo   `json:"sender"`
}

type senderInfo struct {
	Name   string `json:"name"`
	Avatar string `json:"avatar,omitempty"`
}

// Channel implements the channel.Channel interface for Viber.
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
	msgsRecv    atomic.Int64

	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new Viber channel.
func New(cfg Config, logger *zap.Logger) *Channel {
	return &Channel{
		config:   cfg,
		logger:   logger.With(zap.String("channel", "viber")),
		client:   &http.Client{Timeout: 30 * time.Second},
		messages: make(chan channel.Message, 100),
		status:   channel.StatusDisconnected,
	}
}

func (c *Channel) Name() string                    { return "viber" }
func (c *Channel) Type() string                    { return "viber" }
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
	c.logger.Info("Viber channel started")
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
	c.logger.Info("Viber channel stopped")
	return nil
}

// HandleWebhook processes incoming Viber callback requests.
func (c *Channel) HandleWebhook(body []byte) error {
	var event callbackEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("failed to parse callback body: %w", err)
	}

	switch event.Event {
	case "message":
		if event.Sender != nil && event.Message != nil {
			c.processMessage(event)
		}
	case "seen", "delivered":
		c.logger.Debug("received status event", zap.String("event", event.Event),
			zap.Int64("message_token", event.MessageToken))
	case "webhook":
		c.logger.Info("webhook registered successfully")
	case "subscribed":
		c.logger.Info("user subscribed", zap.String("user_id", event.UserID))
	case "unsubscribed":
		c.logger.Info("user unsubscribed", zap.String("user_id", event.UserID))
	case "conversation_started":
		c.logger.Info("conversation started", zap.String("user_id", event.UserID))
	default:
		c.logger.Debug("unhandled callback event", zap.String("event", event.Event))
	}

	return nil
}

// processMessage handles incoming messages including text and media attachments.
func (c *Channel) processMessage(event callbackEvent) {
	msgType := channel.MessageTypeText
	content := event.Message.Text
	var attachments []channel.Attachment

	switch event.Message.Type {
	case "picture":
		msgType = channel.MessageTypeImage
		if event.Message.Media != "" {
			attachments = append(attachments, channel.Attachment{
				Type:     channel.MessageTypeImage,
				URL:      event.Message.Media,
				MimeType: "image/jpeg",
			})
		}
	case "video":
		msgType = channel.MessageTypeVideo
		if event.Message.Media != "" {
			attachments = append(attachments, channel.Attachment{
				Type:     channel.MessageTypeVideo,
				URL:      event.Message.Media,
				MimeType: "video/mp4",
			})
		}
	case "file":
		msgType = channel.MessageTypeFile
		if event.Message.Media != "" {
			attachments = append(attachments, channel.Attachment{
				Type:     channel.MessageTypeFile,
				Name:     event.Message.FileName,
				URL:      event.Message.Media,
				Size:     event.Message.FileSize,
			})
		}
	case "sticker":
		msgType = channel.MessageTypeImage
		if event.Message.Media != "" {
			attachments = append(attachments, channel.Attachment{
				Type:     channel.MessageTypeImage,
				URL:      event.Message.Media,
				MimeType: "image/png",
			})
		}
		if content == "" {
			content = "[sticker]"
		}
	case "text":
		// already handled by defaults
	default:
		c.logger.Debug("unsupported message type", zap.String("type", event.Message.Type))
		return
	}

	msg := channel.Message{
		ID:          fmt.Sprintf("%d", event.MessageToken),
		ChannelName: "viber",
		ChatID:      event.Sender.ID,
		UserID:      event.Sender.ID,
		Username:    event.Sender.Name,
		Type:        msgType,
		Content:     content,
		Attachments: attachments,
		Timestamp:   time.UnixMilli(event.Timestamp),
		Metadata: map[string]interface{}{
			"sender_avatar": event.Sender.Avatar,
			"message_type":  event.Message.Type,
		},
	}

	c.msgCount.Add(1)
	c.msgsRecv.Add(1)

	select {
	case c.messages <- msg:
	default:
		c.logger.Warn("message channel full, dropping message",
			zap.String("id", msg.ID))
	}
}

// Send sends a text message via Viber send_message API.
func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	payload := sendPayload{
		Receiver: msg.ChatID,
		Type:     "text",
		Text:     msg.Content,
		Sender:   c.senderInfo(),
	}

	if err := c.doSend(ctx, payload); err != nil {
		c.setError(err.Error())
		return err
	}

	c.msgsSent.Add(1)
	return nil
}

// SendMedia sends a media message (picture, video, or file) via Viber API.
func (c *Channel) SendMedia(ctx context.Context, chatID string, mediaType string, mediaURL string, text string, fileName string, fileSize int64) error {
	payload := sendPayload{
		Receiver: chatID,
		Type:     mediaType,
		Text:     text,
		Media:    mediaURL,
		Sender:   c.senderInfo(),
	}

	if mediaType == "file" {
		payload.FileName = fileName
		payload.FileSize = fileSize
	}

	if err := c.doSend(ctx, payload); err != nil {
		c.setError(err.Error())
		return err
	}

	c.msgsSent.Add(1)
	return nil
}

// SendStreaming accumulates streamed chunks and sends them as text messages,
// splitting at maxTextLength boundaries to stay within Viber limits.
func (c *Channel) SendStreaming(ctx context.Context, chatID string, replyToID string, content <-chan string, done chan<- struct{}) error {
	defer close(done)

	var buf strings.Builder
	for chunk := range content {
		buf.WriteString(chunk)

		// Flush when buffer exceeds max length.
		for buf.Len() >= maxTextLength {
			text := buf.String()
			part := text[:maxTextLength]
			remainder := text[maxTextLength:]

			if err := c.Send(ctx, channel.OutgoingMessage{ChatID: chatID, Content: part}); err != nil {
				return err
			}

			buf.Reset()
			buf.WriteString(remainder)
		}
	}

	// Flush remaining content.
	if buf.Len() > 0 {
		return c.Send(ctx, channel.OutgoingMessage{ChatID: chatID, Content: buf.String()})
	}
	return nil
}

// SetWebhook registers the webhook URL with Viber.
func (c *Channel) SetWebhook(ctx context.Context, url string) error {
	payload := map[string]interface{}{
		"url": url,
		"event_types": []string{
			"delivered", "seen", "message",
			"subscribed", "unsubscribed", "conversation_started",
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, viberSetWebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Viber-Auth-Token", c.config.AuthToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to set webhook: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Status    int    `json:"status"`
		StatusMsg string `json:"status_message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse webhook response: %w", err)
	}
	if result.Status != 0 {
		return fmt.Errorf("viber set_webhook error (status %d): %s", result.Status, result.StatusMsg)
	}

	c.logger.Info("Viber webhook set", zap.String("url", url))
	return nil
}

func (c *Channel) Info() channel.Info {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return channel.Info{
		Name:             "viber",
		Type:             "viber",
		Status:           c.status,
		Enabled:          c.config.Enabled,
		ConnectedAt:      c.connectedAt,
		LastError:        c.lastError,
		LastErrorAt:      c.lastErrorAt,
		MessageCount:     c.msgCount.Load(),
		MessagesReceived: c.msgsRecv.Load(),
		MessagesSent:     c.msgsSent.Load(),
	}
}

func (c *Channel) setError(errMsg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastError = errMsg
	now := timeutil.NowTime()
	c.lastErrorAt = &now
	c.status = channel.StatusError
}

func (c *Channel) senderInfo() senderInfo {
	return senderInfo{
		Name:   c.config.BotName,
		Avatar: c.config.BotAvatar,
	}
}

// doSend posts a sendPayload to the Viber send_message API.
func (c *Channel) doSend(ctx context.Context, payload sendPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, viberSendURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Viber-Auth-Token", c.config.AuthToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Status    int    `json:"status"`
		StatusMsg string `json:"status_message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to parse send response: %s", string(respBody))
	}
	if result.Status != 0 {
		return fmt.Errorf("viber API error (status %d): %s", result.Status, result.StatusMsg)
	}

	return nil
}

// Validator for Viber configuration.
type Validator struct{}

func NewValidator() *Validator { return &Validator{} }

func (v *Validator) Validate(ctx context.Context, config map[string]string) validator.Result {
	token := config["auth_token"]
	if token == "" {
		return validator.Result{Success: false, Error: "auth_token is required"}
	}

	// Validate token by calling get_account_info API.
	client := &http.Client{Timeout: 10 * time.Second}

	payload, _ := json.Marshal(map[string]string{})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, viberAccountURL, bytes.NewReader(payload))
	if err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to create request: %v", err)}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Viber-Auth-Token", token)

	resp, err := client.Do(req)
	if err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to connect to Viber API: %v", err)}
	}
	defer resp.Body.Close()

	var accountInfo struct {
		Status    int    `json:"status"`
		StatusMsg string `json:"status_message"`
		Name      string `json:"name"`
		ID        string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&accountInfo); err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to parse account info: %v", err)}
	}
	if accountInfo.Status != 0 {
		return validator.Result{Success: false, Error: fmt.Sprintf("Viber API error (status %d): %s", accountInfo.Status, accountInfo.StatusMsg)}
	}

	return validator.Result{
		Success:    true,
		MessageKey: "channels.connectionSuccess",
		Data: map[string]interface{}{
			"bot_name": accountInfo.Name,
			"bot_id":   accountInfo.ID,
		},
	}
}
