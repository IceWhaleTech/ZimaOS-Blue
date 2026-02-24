// Package line provides a LINE Messaging API channel implementation.
package line

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
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

// Config contains LINE channel configuration.
type Config struct {
	Enabled            bool   `yaml:"enabled"`
	ChannelID          string `yaml:"channel_id"`
	ChannelSecret      string `yaml:"channel_secret"`
	ChannelAccessToken string `yaml:"channel_access_token"`
}

const (
	lineAPIBase    = "https://api.line.me/v2"
	lineDataAPI    = "https://api-data.line.me/v2"
	linePushURL    = lineAPIBase + "/bot/message/push"
	lineReplyURL   = lineAPIBase + "/bot/message/reply"
	lineBotInfoURL = lineAPIBase + "/bot/info"
)

// webhookEvent represents a LINE webhook event.
type webhookEvent struct {
	Type       string          `json:"type"`
	Timestamp  int64           `json:"timestamp"`
	Source     webhookSource   `json:"source"`
	ReplyToken string          `json:"replyToken"`
	Message    *webhookMessage `json:"message,omitempty"`
}

type webhookSource struct {
	Type    string `json:"type"`
	UserID  string `json:"userId"`
	GroupID string `json:"groupId,omitempty"`
	RoomID  string `json:"roomId,omitempty"`
}

type webhookMessage struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type webhookBody struct {
	Events []webhookEvent `json:"events"`
}

// Channel implements the channel.Channel interface for LINE.
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

// New creates a new LINE channel.
func New(cfg Config, logger *zap.Logger) *Channel {
	return &Channel{
		config:   cfg,
		logger:   logger.With(zap.String("channel", "line")),
		client:   &http.Client{Timeout: 30 * time.Second},
		messages: make(chan channel.Message, 100),
		status:   channel.StatusDisconnected,
	}
}

func (c *Channel) Name() string                    { return "line" }
func (c *Channel) Type() string                    { return "line" }
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
	c.logger.Info("LINE channel started")
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
	c.logger.Info("LINE channel stopped")
	return nil
}

// HandleWebhook processes incoming LINE webhook requests.
func (c *Channel) HandleWebhook(body []byte, signature string) error {
	// Verify signature
	if c.config.ChannelSecret != "" {
		mac := hmac.New(sha256.New, []byte(c.config.ChannelSecret))
		mac.Write(body)
		expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(signature), []byte(expected)) {
			return fmt.Errorf("invalid signature")
		}
	}

	var wb webhookBody
	if err := json.Unmarshal(body, &wb); err != nil {
		return fmt.Errorf("failed to parse webhook body: %w", err)
	}

	for _, event := range wb.Events {
		if event.Type == "message" && event.Message != nil && event.Message.Type == "text" {
			c.processTextMessage(event)
		}
	}
	return nil
}

func (c *Channel) processTextMessage(event webhookEvent) {
	chatID := event.Source.UserID
	if event.Source.GroupID != "" {
		chatID = event.Source.GroupID
	} else if event.Source.RoomID != "" {
		chatID = event.Source.RoomID
	}

	msg := channel.Message{
		ID:          event.Message.ID,
		ChannelName: "line",
		ChatID:      chatID,
		UserID:      event.Source.UserID,
		Username:    event.Source.UserID,
		Type:        channel.MessageTypeText,
		Content:     event.Message.Text,
		Timestamp:   time.UnixMilli(event.Timestamp),
		IsGroup:     event.Source.Type == "group" || event.Source.Type == "room",
		Metadata: map[string]interface{}{
			"reply_token": event.ReplyToken,
			"source_type": event.Source.Type,
		},
	}
	if msg.IsGroup {
		msg.GroupName = chatID
	}

	c.msgCount.Add(1)
	c.msgsRecv.Add(1)

	select {
	case c.messages <- msg:
	default:
		c.logger.Warn("message channel full, dropping message", zap.String("id", msg.ID))
	}
}

// Send sends a message via LINE push API.
func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	var messages []map[string]interface{}

	// Build media messages from attachments.
	for _, att := range msg.Attachments {
		if att.URL == "" {
			continue
		}
		switch att.Type {
		case channel.MessageTypeImage:
			messages = append(messages, map[string]interface{}{
				"type":               "image",
				"originalContentUrl": att.URL,
				"previewImageUrl":    att.URL,
			})
		case channel.MessageTypeVideo:
			m := map[string]interface{}{
				"type":               "video",
				"originalContentUrl": att.URL,
				"previewImageUrl":    att.URL, // LINE requires a preview; use same URL as fallback
			}
			messages = append(messages, m)
		case channel.MessageTypeAudio:
			messages = append(messages, map[string]interface{}{
				"type":               "audio",
				"originalContentUrl": att.URL,
				"duration":           60000, // default 60s; LINE requires duration
			})
		default:
			// Files: send as text with URL
			text := att.Name
			if text == "" { text = "File" }
			text += "\n" + att.URL
			messages = append(messages, map[string]interface{}{
				"type": "text", "text": text,
			})
		}
	}

	// Add text message if present.
	if msg.Content != "" {
		messages = append(messages, map[string]interface{}{
			"type": "text", "text": msg.Content,
		})
	}

	if len(messages) == 0 {
		return nil
	}

	payload := map[string]interface{}{
		"to":       msg.ChatID,
		"messages": messages,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, linePushURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.ChannelAccessToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("LINE API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	c.msgsSent.Add(1)
	return nil
}

// ReplyMessage sends a reply using a reply token.
func (c *Channel) ReplyMessage(ctx context.Context, replyToken string, text string) error {
	payload := map[string]interface{}{
		"replyToken": replyToken,
		"messages": []map[string]string{
			{"type": "text", "text": text},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal reply: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, lineReplyURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.ChannelAccessToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send reply: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("LINE reply API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	c.msgsSent.Add(1)
	return nil
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
		Name: "line", Type: "line", Status: c.status, Enabled: c.config.Enabled,
		ConnectedAt: c.connectedAt, LastError: c.lastError,
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

// Validator for LINE configuration.
type Validator struct{}

func NewValidator() *Validator { return &Validator{} }

func (v *Validator) Validate(ctx context.Context, config map[string]string) validator.Result {
	token := config["channel_access_token"]
	if token == "" {
		return validator.Result{Success: false, Error: "channel_access_token is required"}
	}

	// Validate token by calling bot info API
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, lineBotInfoURL, nil)
	if err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to create request: %v", err)}
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to connect to LINE API: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return validator.Result{Success: false, Error: fmt.Sprintf("LINE API returned status %d", resp.StatusCode)}
	}

	var botInfo struct {
		UserID      string `json:"userId"`
		DisplayName string `json:"displayName"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&botInfo); err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to parse bot info: %v", err)}
	}

	return validator.Result{
		Success:    true,
		MessageKey: "channels.connectionSuccess",
		Data: map[string]interface{}{
			"bot_name": botInfo.DisplayName,
			"bot_id":   botInfo.UserID,
		},
	}
}
