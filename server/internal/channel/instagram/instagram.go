// Package instagram provides an Instagram Messaging API channel implementation.
package instagram

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

// Config contains Instagram channel configuration.
type Config struct {
	Enabled         bool   `yaml:"enabled"`
	PageAccessToken string `yaml:"page_access_token"`
	AppSecret       string `yaml:"app_secret"`
	VerifyToken     string `yaml:"verify_token"`
	IGAccountID     string `yaml:"ig_account_id"`
}

const (
	graphAPIBase = "https://graph.facebook.com/v18.0"
	sendAPIURL   = graphAPIBase + "/me/messages"
	profileURL   = graphAPIBase + "/me"

	// Chunked streaming settings.
	streamingChunkSize = 1800
	typingDelayMS      = 500
)

// Webhook payload types from Meta platform.

type webhookBody struct {
	Object string         `json:"object"`
	Entry  []webhookEntry `json:"entry"`
}

type webhookEntry struct {
	ID        string             `json:"id"`
	Time      int64              `json:"time"`
	Messaging []webhookMessaging `json:"messaging"`
}

type webhookMessaging struct {
	Sender    webhookUser     `json:"sender"`
	Recipient webhookUser     `json:"recipient"`
	Timestamp int64           `json:"timestamp"`
	Message   *webhookMessage `json:"message,omitempty"`
}

type webhookUser struct {
	ID string `json:"id"`
}

type webhookMessage struct {
	MID         string              `json:"mid"`
	Text        string              `json:"text,omitempty"`
	Attachments []webhookAttachment `json:"attachments,omitempty"`
}

type webhookAttachment struct {
	Type    string                   `json:"type"`
	Payload webhookAttachmentPayload `json:"payload"`
}

type webhookAttachmentPayload struct {
	URL string `json:"url,omitempty"`
}

// Channel implements the channel.Channel interface for Instagram.
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

// New creates a new Instagram channel.
func New(cfg Config, logger *zap.Logger) *Channel {
	return &Channel{
		config:   cfg,
		logger:   logger.With(zap.String("channel", "instagram")),
		client:   &http.Client{Timeout: 30 * time.Second},
		messages: make(chan channel.Message, 100),
		status:   channel.StatusDisconnected,
	}
}

func (c *Channel) Name() string                    { return "instagram" }
func (c *Channel) Type() string                    { return "instagram" }
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
	c.logger.Info("Instagram channel started")
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
	c.logger.Info("Instagram channel stopped")
	return nil
}

// WebhookVerify handles the GET verification challenge from Meta.
func (c *Channel) WebhookVerify(mode, token, challenge string) (string, error) {
	if mode == "subscribe" && token == c.config.VerifyToken {
		c.logger.Info("Instagram webhook verified")
		return challenge, nil
	}
	return "", fmt.Errorf("webhook verification failed")
}

// HandleWebhook processes incoming Instagram webhook requests.
// It verifies the X-Hub-Signature-256 header and dispatches messages.
func (c *Channel) HandleWebhook(body []byte, signature string) error {
	// Verify X-Hub-Signature-256
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

	if wb.Object != "instagram" {
		return fmt.Errorf("unexpected object type: %s", wb.Object)
	}

	for _, entry := range wb.Entry {
		for _, messaging := range entry.Messaging {
			if messaging.Message != nil {
				c.processMessage(messaging)
			}
		}
	}
	return nil
}

// processMessage handles a single incoming message, including text and media attachments.
func (c *Channel) processMessage(m webhookMessaging) {
	msg := channel.Message{
		ID:          m.Message.MID,
		ChannelName: "instagram",
		ChatID:      m.Sender.ID,
		UserID:      m.Sender.ID,
		Username:    m.Sender.ID,
		Timestamp:   time.UnixMilli(m.Timestamp),
		Metadata:    map[string]interface{}{},
	}

	// Process attachments (image, video, story_mention, etc.)
	if len(m.Message.Attachments) > 0 {
		for _, att := range m.Message.Attachments {
			switch att.Type {
			case "image":
				msg.Type = channel.MessageTypeImage
				msg.Attachments = append(msg.Attachments, channel.Attachment{
					Type:     channel.MessageTypeImage,
					URL:      att.Payload.URL,
					MimeType: "image/jpeg",
				})
			case "video":
				msg.Type = channel.MessageTypeVideo
				msg.Attachments = append(msg.Attachments, channel.Attachment{
					Type:     channel.MessageTypeVideo,
					URL:      att.Payload.URL,
					MimeType: "video/mp4",
				})
			case "audio":
				msg.Type = channel.MessageTypeAudio
				msg.Attachments = append(msg.Attachments, channel.Attachment{
					Type:     channel.MessageTypeAudio,
					URL:      att.Payload.URL,
					MimeType: "audio/mpeg",
				})
			default:
				// story_mention, story_reply, share, etc.
				msg.Type = channel.MessageTypeFile
				msg.Metadata["attachment_type"] = att.Type
				if att.Payload.URL != "" {
					msg.Attachments = append(msg.Attachments, channel.Attachment{
						Type: channel.MessageTypeFile,
						URL:  att.Payload.URL,
					})
				}
			}
		}
		// Include text alongside attachments if present.
		if m.Message.Text != "" {
			msg.Content = m.Message.Text
		}
	} else {
		msg.Type = channel.MessageTypeText
		msg.Content = m.Message.Text
	}

	c.msgCount.Add(1)
	c.msgsRecv.Add(1)

	select {
	case c.messages <- msg:
	default:
		c.logger.Warn("message channel full, dropping message", zap.String("id", msg.ID))
	}
}

// Send sends a message via the Instagram Messaging API, including media attachments.
func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	// Send media attachments first.
	for i, att := range msg.Attachments {
		if att.URL == "" {
			continue
		}
		mediaType := "image"
		switch att.Type {
		case channel.MessageTypeVideo:
			mediaType = "video"
		case channel.MessageTypeAudio:
			mediaType = "audio"
		case channel.MessageTypeFile:
			mediaType = "file"
		}
		payload := map[string]interface{}{
			"recipient": map[string]string{"id": msg.ChatID},
			"message": map[string]interface{}{
				"attachment": map[string]interface{}{
					"type":    mediaType,
					"payload": map[string]string{"url": att.URL},
				},
			},
		}
		if err := c.callSendAPI(ctx, payload); err != nil {
			c.logger.Warn("failed to send media via Instagram",
				zap.String("type", mediaType), zap.Error(err))
			// Fall back to text with URL.
			fallback := att.URL
			if i == 0 && msg.Content != "" {
				fallback = msg.Content + "\n" + att.URL
			}
			_ = c.callSendAPI(ctx, map[string]interface{}{
				"recipient": map[string]string{"id": msg.ChatID},
				"message":   map[string]string{"text": fallback},
			})
		}
		if i == 0 {
			msg.Content = "" // Caption sent with first attachment.
		}
	}

	// Send remaining text.
	if msg.Content != "" {
		payload := map[string]interface{}{
			"recipient": map[string]string{"id": msg.ChatID},
			"message":   map[string]string{"text": msg.Content},
		}
		if err := c.callSendAPI(ctx, payload); err != nil {
			c.setError(err.Error())
			return err
		}
	}

	c.msgsSent.Add(1)
	return nil
}

// SendMedia sends an image attachment via the Instagram Messaging API.
func (c *Channel) SendMedia(ctx context.Context, chatID string, imageURL string) error {
	payload := map[string]interface{}{
		"recipient": map[string]string{"id": chatID},
		"message": map[string]interface{}{
			"attachment": map[string]interface{}{
				"type": "image",
				"payload": map[string]string{
					"url": imageURL,
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

// SendTyping sends a typing indicator to the given Instagram chat.
func (c *Channel) SendTyping(ctx context.Context, chatID string) error {
	return c.sendTypingIndicator(ctx, chatID, "typing_on")
}

// SendStreaming sends a message with typing indicator and chunked text sends.
func (c *Channel) SendStreaming(ctx context.Context, chatID string, replyToID string, content <-chan string, done chan<- struct{}) error {
	defer close(done)

	// Send initial typing indicator.
	_ = c.sendTypingIndicator(ctx, chatID, "typing_on")

	var fullContent strings.Builder
	for chunk := range content {
		fullContent.WriteString(chunk)
	}

	text := fullContent.String()
	if text == "" {
		_ = c.sendTypingIndicator(ctx, chatID, "typing_off")
		return nil
	}

	// Send in chunks to avoid message length limits.
	chunks := splitChunks(text, streamingChunkSize)
	for i, chunk := range chunks {
		if i > 0 {
			// Brief typing indicator between chunks.
			_ = c.sendTypingIndicator(ctx, chatID, "typing_on")
			time.Sleep(time.Duration(typingDelayMS) * time.Millisecond)
		}
		if err := c.Send(ctx, channel.OutgoingMessage{ChatID: chatID, Content: chunk}); err != nil {
			return err
		}
	}

	_ = c.sendTypingIndicator(ctx, chatID, "typing_off")
	return nil
}

// sendTypingIndicator sends a sender_action to show/hide typing indicator.
func (c *Channel) sendTypingIndicator(ctx context.Context, chatID string, action string) error {
	payload := map[string]interface{}{
		"recipient":     map[string]string{"id": chatID},
		"sender_action": action,
	}
	return c.callSendAPI(ctx, payload)
}

// callSendAPI posts a payload to the Instagram Send API.
func (c *Channel) callSendAPI(ctx context.Context, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	url := sendAPIURL + "?access_token=" + c.config.PageAccessToken
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
		return fmt.Errorf("Instagram API error (status %d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (c *Channel) Info() channel.Info {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return channel.Info{
		Name: "instagram", Type: "instagram", Status: c.status, Enabled: c.config.Enabled,
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

// splitChunks splits text into chunks of at most maxLen bytes, breaking at the last space.
func splitChunks(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}
	var chunks []string
	for len(text) > 0 {
		if len(text) <= maxLen {
			chunks = append(chunks, text)
			break
		}
		cut := maxLen
		if idx := strings.LastIndex(text[:cut], " "); idx > 0 {
			cut = idx
		}
		chunks = append(chunks, text[:cut])
		text = strings.TrimLeft(text[cut:], " ")
	}
	return chunks
}

// Validator for Instagram configuration.
type Validator struct{}

// NewValidator creates a new Instagram validator.
func NewValidator() *Validator { return &Validator{} }

// Validate checks the Instagram configuration by calling the Graph API.
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

	// Validate token by calling the Graph API /me endpoint.
	client := &http.Client{Timeout: 10 * time.Second}
	url := profileURL + "?access_token=" + token
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to create request: %v", err)}
	}

	resp, err := client.Do(req)
	if err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to connect to Instagram API: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return validator.Result{Success: false, Error: fmt.Sprintf("Instagram API returned status %d: %s", resp.StatusCode, string(respBody))}
	}

	var profile struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to parse profile: %v", err)}
	}

	return validator.Result{
		Success:    true,
		MessageKey: "channels.connectionSuccess",
		Data: map[string]interface{}{
			"page_name": profile.Name,
			"page_id":   profile.ID,
		},
	}
}
