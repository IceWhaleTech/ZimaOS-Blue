// Package googlechat provides a Google Chat channel implementation.
package googlechat

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
)

// Config contains Google Chat channel configuration.
type Config struct {
	Enabled    bool   `yaml:"enabled"`
	WebhookURL string `yaml:"webhook_url"`
	SpaceID    string `yaml:"space_id"`
	APIKey     string `yaml:"api_key"`
}

// webhookEvent represents a Google Chat webhook event.
type webhookEvent struct {
	Type      string       `json:"type"`
	EventTime string       `json:"eventTime"`
	Message   *chatMessage `json:"message,omitempty"`
	User      *chatUser    `json:"user,omitempty"`
	Space     *chatSpace   `json:"space,omitempty"`
}

type chatMessage struct {
	Name       string      `json:"name"`
	Text       string      `json:"text"`
	CreateTime string      `json:"createTime"`
	Sender     *chatUser   `json:"sender,omitempty"`
	Thread     *chatThread `json:"thread,omitempty"`
}

type chatUser struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Type        string `json:"type"`
}

type chatSpace struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Type        string `json:"type"`
}

type chatThread struct {
	Name string `json:"name"`
}

// Channel implements the channel.Channel interface for Google Chat.
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

// New creates a new Google Chat channel.
func New(cfg Config, logger *zap.Logger) *Channel {
	return &Channel{
		config:   cfg,
		logger:   logger.With(zap.String("channel", "googlechat")),
		client:   &http.Client{Timeout: 30 * time.Second},
		messages: make(chan channel.Message, 100),
		status:   channel.StatusDisconnected,
	}
}

func (c *Channel) Name() string                    { return "googlechat" }
func (c *Channel) Type() string                    { return "googlechat" }
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
	c.logger.Info("Google Chat channel started")
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
	c.logger.Info("Google Chat channel stopped")
	return nil
}

// HandleWebhook processes incoming Google Chat webhook events.
func (c *Channel) HandleWebhook(body []byte) error {
	var event webhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("failed to parse webhook event: %w", err)
	}

	if event.Type == "MESSAGE" && event.Message != nil {
		c.processMessage(event)
	}
	return nil
}

func (c *Channel) processMessage(event webhookEvent) {
	chatID := ""
	if event.Space != nil {
		chatID = event.Space.Name
	}

	userID := ""
	username := ""
	if event.Message.Sender != nil {
		userID = event.Message.Sender.Name
		username = event.Message.Sender.DisplayName
	}

	isGroup := false
	groupName := ""
	if event.Space != nil && event.Space.Type == "ROOM" {
		isGroup = true
		groupName = event.Space.DisplayName
	}

	threadName := ""
	if event.Message.Thread != nil {
		threadName = event.Message.Thread.Name
	}

	msg := channel.Message{
		ID:          event.Message.Name,
		ChannelName: "googlechat",
		ChatID:      chatID,
		UserID:      userID,
		Username:    username,
		Type:        channel.MessageTypeText,
		Content:     event.Message.Text,
		Timestamp:   time.Now(),
		IsGroup:     isGroup,
		GroupName:   groupName,
		Metadata: map[string]interface{}{
			"space_type":  event.Space.Type,
			"thread_name": threadName,
		},
	}

	c.msgCount.Add(1)
	c.msgsRecv.Add(1)

	select {
	case c.messages <- msg:
	default:
		c.logger.Warn("message channel full, dropping message", zap.String("id", msg.ID))
	}
}

// Send sends a message via Google Chat webhook, including media as cards.
func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	if c.config.WebhookURL == "" {
		return fmt.Errorf("webhook URL not configured")
	}

	payload := map[string]interface{}{}

	if msg.Content != "" {
		payload["text"] = msg.Content
	}

	// Attach images as cards (Google Chat webhook supports cardsV2 with image widgets).
	if len(msg.Attachments) > 0 {
		var widgets []map[string]interface{}
		for _, att := range msg.Attachments {
			if att.URL == "" {
				continue
			}
			switch att.Type {
			case channel.MessageTypeImage:
				widgets = append(widgets, map[string]interface{}{
					"image": map[string]interface{}{
						"imageUrl": att.URL,
					},
				})
			default:
				// Non-image: add as a clickable button/link.
				name := att.Name
				if name == "" {
					name = "Download"
				}
				widgets = append(widgets, map[string]interface{}{
					"buttonList": map[string]interface{}{
						"buttons": []map[string]interface{}{
							{
								"text": name,
								"onClick": map[string]interface{}{
									"openLink": map[string]string{"url": att.URL},
								},
							},
						},
					},
				})
			}
		}
		if len(widgets) > 0 {
			payload["cardsV2"] = []map[string]interface{}{
				{
					"cardId": "media",
					"card": map[string]interface{}{
						"sections": []map[string]interface{}{
							{"widgets": widgets},
						},
					},
				},
			}
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Google Chat API error (status %d): %s", resp.StatusCode, string(respBody))
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
		Name: "googlechat", Type: "googlechat", Status: c.status, Enabled: c.config.Enabled,
		ConnectedAt: c.connectedAt, LastError: c.lastError,
		MessageCount: c.msgCount.Load(), MessagesReceived: c.msgsRecv.Load(),
		MessagesSent: c.msgsSent.Load(),
		Metadata: map[string]interface{}{"space_id": c.config.SpaceID},
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

// Validator for Google Chat configuration.
type Validator struct{}

func NewValidator() *Validator { return &Validator{} }

func (v *Validator) Validate(ctx context.Context, config map[string]string) validator.Result {
	webhookURL := config["webhook_url"]
	if webhookURL == "" {
		return validator.Result{Success: false, Error: "webhook_url is required"}
	}

	// Test webhook connectivity
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader([]byte(`{"text":"Connection test"}`)))
	if err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to create request: %v", err)}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to connect to Google Chat: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return validator.Result{Success: false, Error: fmt.Sprintf("Google Chat returned status %d", resp.StatusCode)}
	}

	return validator.Result{
		Success:    true,
		MessageKey: "channels.connectionSuccess",
		Data: map[string]interface{}{
			"space_id": config["space_id"],
		},
	}
}
