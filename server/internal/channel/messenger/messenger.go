// Package messenger provides a Facebook Messenger channel implementation.
package messenger

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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
)

// Config contains Facebook Messenger channel configuration.
type Config struct {
	Enabled         bool   `yaml:"enabled"`
	PageAccessToken string `yaml:"page_access_token"`
	AppSecret       string `yaml:"app_secret"`
	VerifyToken     string `yaml:"verify_token"`
}

const (
	graphAPIBase = "https://graph.facebook.com/v18.0"
	messagesURL  = graphAPIBase + "/me/messages"
	profileURL   = graphAPIBase + "/me"
)

// Webhook payload types from Facebook.
type webhookBody struct {
	Object string         `json:"object"`
	Entry  []webhookEntry `json:"entry"`
}

type webhookEntry struct {
	ID        string             `json:"id"`
	Time      int64              `json:"time"`
	Messaging []messagingEvent   `json:"messaging"`
}

type messagingEvent struct {
	Sender    webhookUser        `json:"sender"`
	Recipient webhookUser        `json:"recipient"`
	Timestamp int64              `json:"timestamp"`
	Message   *incomingMessage   `json:"message,omitempty"`
}

type webhookUser struct {
	ID string `json:"id"`
}

type incomingMessage struct {
	MID         string              `json:"mid"`
	Text        string              `json:"text,omitempty"`
	Attachments []incomingAttachment `json:"attachments,omitempty"`
}

type incomingAttachment struct {
	Type    string           `json:"type"`
	Payload attachmentPayload `json:"payload"`
}

type attachmentPayload struct {
	URL string `json:"url,omitempty"`
}

// Send API types.
type sendRequest struct {
	Recipient   sendUser    `json:"recipient"`
	Message     sendMessage `json:"message,omitempty"`
	SenderAction string    `json:"sender_action,omitempty"`
}

type sendUser struct {
	ID string `json:"id"`
}

type sendMessage struct {
	Text       string          `json:"text,omitempty"`
	Attachment *sendAttachment `json:"attachment,omitempty"`
}

type sendAttachment struct {
	Type    string              `json:"type"`
	Payload sendAttachmentPayload `json:"payload"`
}

type sendAttachmentPayload struct {
	URL        string `json:"url,omitempty"`
	IsReusable bool   `json:"is_reusable,omitempty"`
}

// Channel implements the channel.Channel interface for Facebook Messenger.
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

// New creates a new Facebook Messenger channel.
func New(cfg Config, logger *zap.Logger) *Channel {
	return &Channel{
		config:   cfg,
		logger:   logger.With(zap.String("channel", "messenger")),
		client:   &http.Client{Timeout: 30 * time.Second},
		messages: make(chan channel.Message, 100),
		status:   channel.StatusDisconnected,
	}
}

func (c *Channel) Name() string                    { return "messenger" }
func (c *Channel) Type() string                    { return "messenger" }
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
	now := time.Now()
	c.status = channel.StatusConnected
	c.connectedAt = &now
	c.mu.Unlock()
	c.logger.Info("Messenger channel started")
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
	c.logger.Info("Messenger channel stopped")
	return nil
}

// HandleWebhook processes incoming Facebook Messenger webhook requests.
// It verifies the X-Hub-Signature-256 header and parses messaging events.
func (c *Channel) HandleWebhook(body []byte, signature string) error {
	// Verify signature using HMAC-SHA256
	if c.config.AppSecret != "" {
		mac := hmac.New(sha256.New, []byte(c.config.AppSecret))
		mac.Write(body)
		expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(signature), []byte(expected)) {
			return fmt.Errorf("invalid signature")
		}
	}

	var wb webhookBody
	if err := json.Unmarshal(body, &wb); err != nil {
		return fmt.Errorf("failed to parse webhook body: %w", err)
	}

	if wb.Object != "page" {
		return fmt.Errorf("unexpected webhook object: %s", wb.Object)
	}

	for _, entry := range wb.Entry {
		for _, event := range entry.Messaging {
			if event.Message != nil {
				c.processMessage(event)
			}
		}
	}
	return nil
}

// VerifyWebhook handles the Facebook webhook verification challenge.
func (c *Channel) VerifyWebhook(mode, token, challenge string) (string, error) {
	if mode == "subscribe" && token == c.config.VerifyToken {
		return challenge, nil
	}
	return "", fmt.Errorf("webhook verification failed")
}

func (c *Channel) processMessage(event messagingEvent) {
	msg := event.Message
	chatID := event.Sender.ID

	// Determine message type and process attachments
	if msg.Text != "" {
		c.enqueueMessage(channel.Message{
			ID:          msg.MID,
			ChannelName: "messenger",
			ChatID:      chatID,
			UserID:      event.Sender.ID,
			Username:    event.Sender.ID,
			Type:        channel.MessageTypeText,
			Content:     msg.Text,
			Timestamp:   time.UnixMilli(event.Timestamp),
		})
	}

	for _, att := range msg.Attachments {
		msgType := mapAttachmentType(att.Type)
		c.enqueueMessage(channel.Message{
			ID:          msg.MID,
			ChannelName: "messenger",
			ChatID:      chatID,
			UserID:      event.Sender.ID,
			Username:    event.Sender.ID,
			Type:        msgType,
			Timestamp:   time.UnixMilli(event.Timestamp),
			Attachments: []channel.Attachment{
				{
					Type:     msgType,
					URL:      att.Payload.URL,
					MimeType: att.Type,
				},
			},
		})
	}
}

func mapAttachmentType(fbType string) channel.MessageType {
	switch fbType {
	case "image":
		return channel.MessageTypeImage
	case "audio":
		return channel.MessageTypeAudio
	case "video":
		return channel.MessageTypeVideo
	case "file":
		return channel.MessageTypeFile
	default:
		return channel.MessageTypeFile
	}
}

func (c *Channel) enqueueMessage(msg channel.Message) {
	c.msgCount.Add(1)
	c.msgsRecv.Add(1)

	select {
	case c.messages <- msg:
	default:
		c.logger.Warn("message channel full, dropping message", zap.String("id", msg.ID))
	}
}

// Send sends a message via the Messenger Send API, including media attachments.
func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	// Send media attachments first.
	for i, att := range msg.Attachments {
		if att.URL == "" {
			continue
		}
		mediaType := mapOutgoingAttachmentType(att.Type)
		payload := sendRequest{
			Recipient: sendUser{ID: msg.ChatID},
			Message: sendMessage{
				Attachment: &sendAttachment{
					Type: mediaType,
					Payload: sendAttachmentPayload{
						URL:        att.URL,
						IsReusable: true,
					},
				},
			},
		}
		if err := c.callSendAPI(ctx, payload); err != nil {
			c.logger.Warn("failed to send media via Messenger",
				zap.String("type", mediaType), zap.Error(err))
			// Fall back to text with URL.
			fallback := att.URL
			if i == 0 && msg.Content != "" {
				fallback = msg.Content + "\n" + att.URL
			}
			_ = c.callSendAPI(ctx, sendRequest{
				Recipient: sendUser{ID: msg.ChatID},
				Message:   sendMessage{Text: fallback},
			})
		}
		if i == 0 {
			msg.Content = "" // Caption sent with first attachment.
		}
	}

	// Send remaining text.
	if msg.Content != "" {
		payload := sendRequest{
			Recipient: sendUser{ID: msg.ChatID},
			Message:   sendMessage{Text: msg.Content},
		}
		if err := c.callSendAPI(ctx, payload); err != nil {
			c.setError(err.Error())
			return err
		}
	}

	c.msgsSent.Add(1)
	return nil
}

func mapOutgoingAttachmentType(t channel.MessageType) string {
	switch t {
	case channel.MessageTypeImage:
		return "image"
	case channel.MessageTypeVideo:
		return "video"
	case channel.MessageTypeAudio:
		return "audio"
	default:
		return "file"
	}
}

// SendMedia sends a media attachment (image, audio, video, file) via the Messenger Send API.
func (c *Channel) SendMedia(ctx context.Context, recipientID string, mediaType string, url string) error {
	payload := sendRequest{
		Recipient: sendUser{ID: recipientID},
		Message: sendMessage{
			Attachment: &sendAttachment{
				Type: mediaType,
				Payload: sendAttachmentPayload{
					URL:        url,
					IsReusable: true,
				},
			},
		},
	}
	if err := c.callSendAPI(ctx, payload); err != nil {
		c.setError(err.Error())
		return err
	}
	c.msgsSent.Add(1)
	return nil
}

// SendTyping sends a typing indicator to the given Messenger chat.
func (c *Channel) SendTyping(ctx context.Context, chatID string) error {
	return c.sendAction(ctx, chatID, "typing_on")
}

// SendStreaming sends a streaming response with typing indicators and periodic chunked messages.
func (c *Channel) SendStreaming(ctx context.Context, chatID string, replyToID string, content <-chan string, done chan<- struct{}) error {
	defer close(done)

	// Send initial typing indicator
	_ = c.sendAction(ctx, chatID, "typing_on")

	var fullContent strings.Builder
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case chunk, ok := <-content:
			if !ok {
				// Channel closed, send final accumulated content
				if fullContent.Len() > 0 {
					return c.Send(ctx, channel.OutgoingMessage{ChatID: chatID, Content: fullContent.String()})
				}
				return nil
			}
			fullContent.WriteString(chunk)
		case <-ticker.C:
			// Periodically send accumulated content and refresh typing indicator
			if fullContent.Len() > 0 {
				if err := c.Send(ctx, channel.OutgoingMessage{ChatID: chatID, Content: fullContent.String()}); err != nil {
					return err
				}
				fullContent.Reset()
			}
			_ = c.sendAction(ctx, chatID, "typing_on")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c *Channel) sendAction(ctx context.Context, recipientID string, action string) error {
	payload := sendRequest{
		Recipient:    sendUser{ID: recipientID},
		SenderAction: action,
	}
	return c.callSendAPI(ctx, payload)
}

func (c *Channel) callSendAPI(ctx context.Context, payload sendRequest) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	url := messagesURL + "?access_token=" + c.config.PageAccessToken
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call Send API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Messenger API error (status %d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (c *Channel) Info() channel.Info {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return channel.Info{
		Name: "messenger", Type: "messenger", Status: c.status, Enabled: c.config.Enabled,
		ConnectedAt: c.connectedAt, LastError: c.lastError, LastErrorAt: c.lastErrorAt,
		MessageCount: c.msgCount.Load(), MessagesReceived: c.msgsRecv.Load(),
		MessagesSent: c.msgsSent.Load(),
	}
}

func (c *Channel) setError(errMsg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastError = errMsg
	now := time.Now()
	c.lastErrorAt = &now
	c.status = channel.StatusError
}

// Validator for Facebook Messenger configuration.
type Validator struct{}

// NewValidator creates a new Messenger validator.
func NewValidator() *Validator { return &Validator{} }

// Validate tests the Messenger connection by calling the Graph API /me endpoint.
func (v *Validator) Validate(ctx context.Context, config map[string]string) validator.Result {
	token := config["page_access_token"]
	if token == "" {
		return validator.Result{Success: false, Error: "page_access_token is required"}
	}
	if config["app_secret"] == "" {
		return validator.Result{Success: false, Error: "app_secret is required"}
	}
	if config["verify_token"] == "" {
		return validator.Result{Success: false, Error: "verify_token is required"}
	}

	// Verify token by calling GET /me
	client := &http.Client{Timeout: 10 * time.Second}
	url := profileURL + "?access_token=" + token
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to create request: %v", err)}
	}

	resp, err := client.Do(req)
	if err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to connect to Messenger API: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return validator.Result{Success: false, Error: fmt.Sprintf("Messenger API returned status %d", resp.StatusCode)}
	}

	var pageInfo struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pageInfo); err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to parse page info: %v", err)}
	}

	return validator.Result{
		Success:    true,
		MessageKey: "channels.connectionSuccess",
		Data: map[string]interface{}{
			"page_name": pageInfo.Name,
			"page_id":   pageInfo.ID,
		},
	}
}
