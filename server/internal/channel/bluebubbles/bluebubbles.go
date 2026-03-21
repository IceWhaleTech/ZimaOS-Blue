// Package bluebubbles provides a BlueBubbles (iMessage bridge) channel implementation.
package bluebubbles

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
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"
)

// Channel implements the channel.Channel interface for BlueBubbles.
type Channel struct {
	config   channel.BlueBubblesConfig
	logger   *zap.Logger
	messages chan channel.Message

	mu          sync.RWMutex
	status      channel.Status
	connectedAt *time.Time
	lastError   string
	lastErrorAt *time.Time
	msgCount    atomic.Int64

	// Server info
	serverVersion string
	osVersion     string
	privateAPI    bool

	httpClient *http.Client

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a new BlueBubbles channel.
func New(cfg channel.BlueBubblesConfig, logger *zap.Logger) *Channel {
	// Normalize server URL
	serverURL := strings.TrimSuffix(cfg.ServerURL, "/")
	cfg.ServerURL = serverURL

	return &Channel{
		config:     cfg,
		logger:     logger.With(zap.String("channel", "bluebubbles")),
		messages:   make(chan channel.Message, 100),
		status:     channel.StatusDisconnected,
		httpClient: network.NewPooledHTTPClient(30 * time.Second),
	}
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "bluebubbles"
}

// Type returns the channel type.
func (c *Channel) Type() string {
	return "bluebubbles"
}

// Start initializes and starts the BlueBubbles channel.
func (c *Channel) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.status == channel.StatusConnected || c.status == channel.StatusConnecting {
		c.mu.Unlock()
		return fmt.Errorf("channel already started")
	}
	c.status = channel.StatusConnecting
	c.mu.Unlock()

	c.ctx, c.cancel = context.WithCancel(ctx)

	// Verify connection by getting server info
	if err := c.verifyConnection(ctx); err != nil {
		c.setError(fmt.Sprintf("failed to verify connection: %v", err))
		return fmt.Errorf("failed to verify connection: %w", err)
	}

	now := time.Now()
	c.mu.Lock()
	c.status = channel.StatusConnected
	c.connectedAt = &now
	c.lastError = ""
	c.lastErrorAt = nil
	c.mu.Unlock()

	c.logger.Info("bluebubbles channel started",
		zap.String("server_version", c.serverVersion),
		zap.String("os_version", c.osVersion),
		zap.Bool("private_api", c.privateAPI))
	return nil
}

// verifyConnection verifies the connection by calling /api/v1/server/info.
func (c *Channel) verifyConnection(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/v1/server/info?password=%s", c.config.ServerURL, c.config.Password)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authentication failed: status %d", resp.StatusCode)
	}

	var serverResp struct {
		Status int `json:"status"`
		Data   struct {
			OSVersion       string `json:"os_version"`
			ServerVersion   string `json:"server_version"`
			PrivateAPI      bool   `json:"private_api"`
			HelperConnected bool   `json:"helper_connected"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &serverResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if serverResp.Status != 200 {
		return fmt.Errorf("server returned error status: %d", serverResp.Status)
	}

	c.serverVersion = serverResp.Data.ServerVersion
	c.osVersion = serverResp.Data.OSVersion
	c.privateAPI = serverResp.Data.PrivateAPI

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
	c.logger.Info("bluebubbles channel stopped")
	return nil
}

// Send sends a message through BlueBubbles (iMessage).
func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	c.mu.RLock()
	status := c.status
	c.mu.RUnlock()

	if status != channel.StatusConnected {
		return fmt.Errorf("channel not connected")
	}

	var contentParts []string
	if msg.Content != "" {
		contentParts = append(contentParts, msg.Content)
	}
	for _, att := range msg.Attachments {
		if fallback := blueBubblesAttachmentFallbackText(att); fallback != "" {
			contentParts = append(contentParts, fallback)
		}
	}
	message := strings.Join(contentParts, "\n")
	if strings.TrimSpace(message) == "" {
		return fmt.Errorf("no sendable BlueBubbles content")
	}

	messageReq := map[string]interface{}{
		"chatGuid": msg.ChatID,
		"message":  message,
		"method":   "private-api",
	}

	messageJSON, err := json.Marshal(messageReq)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/message/text?password=%s", c.config.ServerURL, c.config.Password)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(messageJSON)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to send message: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

func blueBubblesAttachmentFallbackText(att channel.Attachment) string {
	if strings.TrimSpace(att.URL) != "" {
		return strings.TrimSpace(att.URL)
	}
	if strings.TrimSpace(att.Name) != "" {
		return strings.TrimSpace(att.Name)
	}
	if len(att.Data) > 0 {
		return "[Attachment]"
	}
	return ""
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

	// BlueBubbles/iMessage doesn't support streaming, so we accumulate and send at the end
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
		Name:         "bluebubbles",
		Type:         "bluebubbles",
		Status:       c.status,
		Enabled:      c.config.Enabled,
		ConnectedAt:  c.connectedAt,
		LastError:    c.lastError,
		LastErrorAt:  c.lastErrorAt,
		MessageCount: c.msgCount.Load(),
		Metadata:     make(map[string]interface{}),
	}

	info.Metadata["server_url"] = c.config.ServerURL
	if c.serverVersion != "" {
		info.Metadata["server_version"] = c.serverVersion
		info.Metadata["os_version"] = c.osVersion
		info.Metadata["private_api"] = c.privateAPI
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

// isChatAllowed checks if a chat is allowed.
func (c *Channel) isChatAllowed(chatGUID string) bool {
	if len(c.config.AllowedChats) == 0 {
		return true
	}

	for _, allowed := range c.config.AllowedChats {
		if allowed == chatGUID {
			return true
		}
	}
	return false
}

// HandleWebhook processes an incoming webhook from BlueBubbles.
// This should be called from a webhook handler.
func (c *Channel) HandleWebhook(event *WebhookEvent) {
	if event == nil || event.Data == nil {
		return
	}

	// Only handle new message events
	if event.Type != "new-message" {
		return
	}

	msg := event.Data

	// Check if chat is allowed
	if !c.isChatAllowed(msg.ChatGUID) {
		c.logger.Debug("ignoring message from non-allowed chat",
			zap.String("chat_guid", msg.ChatGUID))
		return
	}

	// Ignore messages sent by us
	if msg.IsFromMe {
		return
	}

	// Convert to unified message format
	channelMsg := c.convertMessage(msg)
	c.msgCount.Add(1)

	select {
	case c.messages <- channelMsg:
	default:
		c.logger.Warn("message channel full, dropping message",
			zap.String("message_id", channelMsg.ID))
	}
}

// convertMessage converts a BlueBubbles message to the unified format.
func (c *Channel) convertMessage(msg *MessageData) channel.Message {
	// Determine sender info
	senderName := msg.Handle
	if senderName == "" && len(msg.Chats) > 0 {
		senderName = msg.Chats[0].DisplayName
	}

	// Determine if group chat
	isGroup := false
	groupName := ""
	if len(msg.Chats) > 0 {
		isGroup = msg.Chats[0].Participants > 1
		groupName = msg.Chats[0].DisplayName
	}

	content := strings.TrimSpace(msg.Text)
	if content == "" {
		content = strings.TrimSpace(msg.Subject)
	}

	metadata := map[string]interface{}{
		"is_from_me": msg.IsFromMe,
		"service":    msg.Service,
	}
	if strings.TrimSpace(msg.Subject) != "" {
		metadata["subject"] = strings.TrimSpace(msg.Subject)
	}
	if msg.DateRead > 0 {
		metadata["date_read"] = msg.DateRead
	}
	if len(msg.Attachments) > 0 {
		rawAttachments := make([]map[string]interface{}, 0, len(msg.Attachments))
		for _, att := range msg.Attachments {
			raw := map[string]interface{}{}
			if strings.TrimSpace(att.GUID) != "" {
				raw["guid"] = strings.TrimSpace(att.GUID)
			}
			if strings.TrimSpace(att.TransferName) != "" {
				raw["transfer_name"] = strings.TrimSpace(att.TransferName)
			}
			if strings.TrimSpace(att.MimeType) != "" {
				raw["mime_type"] = strings.TrimSpace(att.MimeType)
			}
			if att.TotalBytes > 0 {
				raw["total_bytes"] = att.TotalBytes
			}
			rawAttachments = append(rawAttachments, raw)
		}
		metadata["attachments"] = rawAttachments
		metadata["attachment_count"] = len(rawAttachments)
	}

	channelMsg := channel.Message{
		ID:          msg.GUID,
		ChannelName: "bluebubbles",
		ChatID:      msg.ChatGUID,
		UserID:      msg.Handle,
		Username:    senderName,
		Type:        channel.MessageTypeText,
		Content:     content,
		Timestamp:   time.UnixMilli(msg.DateCreated),
		IsGroup:     isGroup,
		GroupName:   groupName,
		Metadata:    metadata,
	}

	// Handle attachments
	for _, att := range msg.Attachments {
		msgAtt := channel.Attachment{
			ID:       att.GUID,
			Name:     att.TransferName,
			Size:     att.TotalBytes,
			MimeType: att.MimeType,
		}

		switch {
		case strings.HasPrefix(att.MimeType, "image/"):
			msgAtt.Type = channel.MessageTypeImage
		case strings.HasPrefix(att.MimeType, "audio/"):
			msgAtt.Type = channel.MessageTypeAudio
		case strings.HasPrefix(att.MimeType, "video/"):
			msgAtt.Type = channel.MessageTypeVideo
		default:
			msgAtt.Type = channel.MessageTypeFile
		}

		channelMsg.Attachments = append(channelMsg.Attachments, msgAtt)
	}

	if len(channelMsg.Attachments) > 0 && strings.TrimSpace(channelMsg.Content) == "" {
		channelMsg.Type = channelMsg.Attachments[0].Type
	}

	return channelMsg
}

// WebhookEvent represents a BlueBubbles webhook event.
type WebhookEvent struct {
	Type string       `json:"type"`
	Data *MessageData `json:"data"`
}

// MessageData represents a BlueBubbles message.
type MessageData struct {
	GUID        string           `json:"guid"`
	ChatGUID    string           `json:"chatGuid"`
	Handle      string           `json:"handle"`
	Text        string           `json:"text"`
	Subject     string           `json:"subject"`
	Service     string           `json:"service"`
	IsFromMe    bool             `json:"isFromMe"`
	DateCreated int64            `json:"dateCreated"`
	DateRead    int64            `json:"dateRead"`
	Attachments []AttachmentData `json:"attachments"`
	Chats       []ChatData       `json:"chats"`
}

// AttachmentData represents an attachment in a message.
type AttachmentData struct {
	GUID         string `json:"guid"`
	TransferName string `json:"transferName"`
	MimeType     string `json:"mimeType"`
	TotalBytes   int64  `json:"totalBytes"`
}

// ChatData represents chat information.
type ChatData struct {
	GUID         string `json:"guid"`
	DisplayName  string `json:"displayName"`
	Participants int    `json:"participants"`
}
