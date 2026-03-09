// Package zalo provides a Zalo Official Account channel implementation.
package zalo

import (
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
)

const (
	// DefaultZaloAPIURL is the default Zalo API endpoint.
	DefaultZaloAPIURL = "https://openapi.zalo.me"
)

// Channel implements the channel.Channel interface for Zalo.
type Channel struct {
	config   channel.ZaloConfig
	logger   *zap.Logger
	messages chan channel.Message

	mu          sync.RWMutex
	status      channel.Status
	connectedAt *time.Time
	lastError   string
	lastErrorAt *time.Time
	msgCount    atomic.Int64

	// OA info
	oaName     string
	isVerified bool

	apiBaseURL string
	httpClient *http.Client

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a new Zalo channel.
func New(cfg channel.ZaloConfig, logger *zap.Logger) *Channel {
	return NewWithOptions(cfg, logger, DefaultZaloAPIURL)
}

// NewWithOptions creates a new Zalo channel with custom options.
func NewWithOptions(cfg channel.ZaloConfig, logger *zap.Logger, apiBaseURL string) *Channel {
	if apiBaseURL == "" {
		apiBaseURL = DefaultZaloAPIURL
	}
	return &Channel{
		config:     cfg,
		logger:     logger.With(zap.String("channel", "zalo")),
		messages:   make(chan channel.Message, 100),
		status:     channel.StatusDisconnected,
		apiBaseURL: apiBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "zalo"
}

// Type returns the channel type.
func (c *Channel) Type() string {
	return "zalo"
}

// Start initializes and starts the Zalo channel.
func (c *Channel) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.status == channel.StatusConnected || c.status == channel.StatusConnecting {
		c.mu.Unlock()
		return fmt.Errorf("channel already started")
	}
	c.status = channel.StatusConnecting
	c.mu.Unlock()

	c.ctx, c.cancel = context.WithCancel(ctx)

	// Verify credentials by getting OA info
	if err := c.verifyCredentials(ctx); err != nil {
		c.setError(fmt.Sprintf("failed to verify credentials: %v", err))
		return fmt.Errorf("failed to verify credentials: %w", err)
	}

	now := time.Now()
	c.mu.Lock()
	c.status = channel.StatusConnected
	c.connectedAt = &now
	c.lastError = ""
	c.lastErrorAt = nil
	c.mu.Unlock()

	c.logger.Info("zalo channel started",
		zap.String("oa_id", c.config.OAID),
		zap.String("oa_name", c.oaName),
		zap.Bool("is_verified", c.isVerified))
	return nil
}

// verifyCredentials verifies the access token by calling /v2.0/oa/getoa.
func (c *Channel) verifyCredentials(ctx context.Context) error {
	url := fmt.Sprintf("%s/v2.0/oa/getoa", c.apiBaseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("access_token", c.config.AccessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var oaResp struct {
		Error   int    `json:"error"`
		Message string `json:"message"`
		Data    struct {
			OAID       string `json:"oa_id"`
			Name       string `json:"name"`
			IsVerified bool   `json:"is_verified"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &oaResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if oaResp.Error != 0 {
		return fmt.Errorf("Zalo API error: %s (code: %d)", oaResp.Message, oaResp.Error)
	}

	c.oaName = oaResp.Data.Name
	c.isVerified = oaResp.Data.IsVerified

	return nil
}

// Stop gracefully shuts down the channel.
func (c *Channel) Stop(ctx context.Context) error {
	c.mu.Lock()
	if c.status == channel.StatusDisconnected {
		c.mu.Unlock()
		return nil
	}
	c.status = channel.StatusDisconnected
	c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}

	// Wait for goroutines to finish
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	close(c.messages)
	c.logger.Info("zalo channel stopped")
	return nil
}

// Send sends a message through Zalo OA, including media attachments.
func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	c.mu.RLock()
	status := c.status
	c.mu.RUnlock()

	if status != channel.StatusConnected {
		return fmt.Errorf("channel not connected")
	}

	// Send media attachments first.
	for i, att := range msg.Attachments {
		includeCaption := i == 0 && msg.Content != ""
		fallbackText := buildAttachmentFallbackText(msg.Content, att, includeCaption)
		if att.URL == "" {
			if fallbackText != "" {
				if err := c.sendJSON(ctx, zaloTextMessage(msg.ChatID, fallbackText)); err != nil {
					c.logger.Warn("failed to send attachment fallback via Zalo",
						zap.String("type", string(att.Type)), zap.Error(err))
				} else if includeCaption {
					msg.Content = ""
				}
			}
			continue
		}

		switch att.Type {
		case channel.MessageTypeImage:
			messageReq := map[string]interface{}{
				"recipient": map[string]string{"user_id": msg.ChatID},
				"message": map[string]interface{}{
					"attachment": map[string]interface{}{
						"type": "template",
						"payload": map[string]interface{}{
							"template_type": "media",
							"elements": []map[string]interface{}{
								{"media_type": "image", "url": att.URL},
							},
						},
					},
				},
			}
			if err := c.sendJSON(ctx, messageReq); err != nil {
				c.logger.Warn("failed to send media via Zalo",
					zap.String("type", string(att.Type)), zap.Error(err))
				if fallbackText != "" {
					if fallbackErr := c.sendJSON(ctx, zaloTextMessage(msg.ChatID, fallbackText)); fallbackErr != nil {
						c.logger.Warn("failed to send attachment fallback via Zalo",
							zap.String("type", string(att.Type)), zap.Error(fallbackErr))
					} else if includeCaption {
						msg.Content = ""
					}
				}
			}
		default:
			if fallbackText == "" {
				continue
			}
			if err := c.sendJSON(ctx, zaloTextMessage(msg.ChatID, fallbackText)); err != nil {
				c.logger.Warn("failed to send media via Zalo",
					zap.String("type", string(att.Type)), zap.Error(err))
			} else if includeCaption {
				msg.Content = ""
			}
		}
	}

	// Send remaining text.
	if msg.Content != "" {
		if err := c.sendJSON(ctx, zaloTextMessage(msg.ChatID, msg.Content)); err != nil {
			return err
		}
	}

	return nil
}

func buildAttachmentFallbackText(caption string, att channel.Attachment, includeCaption bool) string {
	parts := make([]string, 0, 2)
	if includeCaption && strings.TrimSpace(caption) != "" {
		parts = append(parts, strings.TrimSpace(caption))
	}
	if strings.TrimSpace(att.URL) != "" {
		parts = append(parts, strings.TrimSpace(att.URL))
	} else if strings.TrimSpace(att.Name) != "" {
		parts = append(parts, strings.TrimSpace(att.Name))
	}
	return strings.Join(parts, "\n")
}

func zaloTextMessage(chatID string, text string) map[string]interface{} {
	return map[string]interface{}{
		"recipient": map[string]string{"user_id": chatID},
		"message":   map[string]string{"text": text},
	}
}

// sendJSON posts a JSON payload to the Zalo OA message API.
func (c *Channel) sendJSON(ctx context.Context, payload interface{}) error {
	messageJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	url := fmt.Sprintf("%s/v2.0/oa/message", c.apiBaseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(messageJSON)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("access_token", c.config.AccessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var sendResp struct {
		Error   int    `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &sendResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if sendResp.Error != 0 {
		return fmt.Errorf("failed to send message: %s (code: %d)", sendResp.Message, sendResp.Error)
	}

	return nil
}

// SendStreaming sends a message with streaming support.
func (c *Channel) SendStreaming(ctx context.Context, chatID string, replyToID string, content <-chan string, done chan<- struct{}) error {
	defer close(done)

	c.mu.RLock()
	status := c.status
	c.mu.RUnlock()

	if status != channel.StatusConnected {
		return fmt.Errorf("channel not connected")
	}

	// Zalo doesn't support streaming, so we accumulate and send at the end
	var fullContent strings.Builder

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk, ok := <-content:
			if !ok {
				// Channel closed, send final message
				if fullContent.Len() > 0 {
					return c.Send(ctx, channel.OutgoingMessage{
						ChatID:    chatID,
						Content:   fullContent.String(),
						ReplyToID: replyToID,
					})
				}
				return nil
			}
			fullContent.WriteString(chunk)
		}
	}
}

// Info returns current information about the channel.
func (c *Channel) Info() channel.Info {
	c.mu.RLock()
	defer c.mu.RUnlock()

	info := channel.Info{
		Name:         "zalo",
		Type:         "zalo",
		Status:       c.status,
		Enabled:      c.config.Enabled,
		ConnectedAt:  c.connectedAt,
		LastError:    c.lastError,
		LastErrorAt:  c.lastErrorAt,
		MessageCount: c.msgCount.Load(),
		Metadata:     make(map[string]interface{}),
	}

	info.Metadata["oa_id"] = c.config.OAID
	if c.oaName != "" {
		info.Metadata["oa_name"] = c.oaName
		info.Metadata["is_verified"] = c.isVerified
	}

	return info
}

// IsConnected returns true if the channel is connected.
func (c *Channel) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status == channel.StatusConnected
}

// Messages returns the channel for receiving incoming messages.
func (c *Channel) Messages() <-chan channel.Message {
	return c.messages
}

// setError sets the last error.
func (c *Channel) setError(err string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastError = err
	now := time.Now()
	c.lastErrorAt = &now
	c.status = channel.StatusError
}

// HandleWebhook processes an incoming webhook from Zalo.
// This should be called from a webhook handler.
func (c *Channel) HandleWebhook(event *WebhookEvent) {
	if event == nil || !zaloIsIncomingUserMessage(event) {
		return
	}

	// Convert to unified message format
	msg := c.convertEvent(event)
	c.msgCount.Add(1)

	select {
	case c.messages <- msg:
	default:
		c.logger.Warn("message channel full, dropping message",
			zap.String("message_id", msg.ID))
	}
}

// convertEvent converts a Zalo webhook event to the unified format.
func (c *Channel) convertEvent(event *WebhookEvent) channel.Message {
	msg := channel.Message{
		ID:          event.MsgID,
		ChannelName: "zalo",
		ChatID:      event.Sender.ID,
		UserID:      event.Sender.ID,
		Username:    "", // Zalo doesn't provide username in webhook
		Type:        channel.MessageTypeText,
		Content:     event.Message.Text,
		Timestamp:   time.UnixMilli(event.Timestamp),
		IsGroup:     false, // Zalo OA messages are always 1:1
		Metadata: map[string]interface{}{
			"app_id":     event.AppID,
			"event_name": event.EventName,
		},
	}

	// Handle attachments
	for _, att := range event.Message.Attachments {
		msgAtt := channel.Attachment{
			ID:  att.Payload.ID,
			URL: att.Payload.URL,
		}

		switch att.Type {
		case "image":
			msgAtt.Type = channel.MessageTypeImage
		case "audio":
			msgAtt.Type = channel.MessageTypeAudio
		case "video":
			msgAtt.Type = channel.MessageTypeVideo
		case "file":
			msgAtt.Type = channel.MessageTypeFile
			msgAtt.Name = att.Payload.Name
			msgAtt.Size = att.Payload.Size
		default:
			msgAtt.Type = channel.MessageTypeFile
		}

		msg.Attachments = append(msg.Attachments, msgAtt)
	}

	if len(msg.Attachments) > 0 {
		msg.Type = msg.Attachments[0].Type
		msg.Metadata["attachment_count"] = len(msg.Attachments)
	}

	return msg
}

func zaloIsIncomingUserMessage(event *WebhookEvent) bool {
	eventName := strings.TrimSpace(event.EventName)
	if !strings.HasPrefix(eventName, "user_send") {
		return false
	}
	return strings.TrimSpace(event.Message.Text) != "" || len(event.Message.Attachments) > 0
}

// WebhookEvent represents a Zalo webhook event.
type WebhookEvent struct {
	AppID     string         `json:"app_id"`
	OAID      string         `json:"oa_id"`
	EventName string         `json:"event_name"`
	MsgID     string         `json:"msg_id"`
	Timestamp int64          `json:"timestamp"`
	Sender    WebhookSender  `json:"sender"`
	Message   WebhookMessage `json:"message"`
}

// WebhookSender represents the sender in a webhook event.
type WebhookSender struct {
	ID string `json:"id"`
}

// WebhookMessage represents the message in a webhook event.
type WebhookMessage struct {
	Text        string              `json:"text"`
	Attachments []WebhookAttachment `json:"attachments"`
}

// WebhookAttachment represents an attachment in a webhook message.
type WebhookAttachment struct {
	Type    string            `json:"type"`
	Payload AttachmentPayload `json:"payload"`
}

// AttachmentPayload represents the payload of an attachment.
type AttachmentPayload struct {
	ID   string `json:"id"`
	URL  string `json:"url"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}
