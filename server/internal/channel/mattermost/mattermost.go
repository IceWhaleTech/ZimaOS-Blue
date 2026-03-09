// Package mattermost provides a Mattermost bot channel implementation.
package mattermost

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

// Channel implements the channel.Channel interface for Mattermost.
type Channel struct {
	config   channel.MattermostConfig
	logger   *zap.Logger
	messages chan channel.Message

	mu          sync.RWMutex
	status      channel.Status
	connectedAt *time.Time
	lastError   string
	lastErrorAt *time.Time
	msgCount    atomic.Int64

	// Bot info
	botUserID   string
	botUsername string

	httpClient *http.Client

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a new Mattermost channel.
func New(cfg channel.MattermostConfig, logger *zap.Logger) *Channel {
	// Normalize server URL
	serverURL := strings.TrimSuffix(cfg.ServerURL, "/")
	cfg.ServerURL = serverURL

	return &Channel{
		config:   cfg,
		logger:   logger.With(zap.String("channel", "mattermost")),
		messages: make(chan channel.Message, 100),
		status:   channel.StatusDisconnected,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "mattermost"
}

// Type returns the channel type.
func (c *Channel) Type() string {
	return "mattermost"
}

// Start initializes and starts the Mattermost channel.
func (c *Channel) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.status == channel.StatusConnected || c.status == channel.StatusConnecting {
		c.mu.Unlock()
		return fmt.Errorf("channel already started")
	}
	c.status = channel.StatusConnecting
	c.mu.Unlock()

	c.ctx, c.cancel = context.WithCancel(ctx)

	// Verify credentials by getting bot user info
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

	c.logger.Info("mattermost channel started",
		zap.String("bot_user_id", c.botUserID),
		zap.String("bot_username", c.botUsername))
	return nil
}

// verifyCredentials verifies the bot token by calling /api/v4/users/me.
func (c *Channel) verifyCredentials(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/v4/users/me", c.config.ServerURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.BotToken)

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

	var user struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	}
	if err := json.Unmarshal(body, &user); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	c.botUserID = user.ID
	c.botUsername = user.Username

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
	c.logger.Info("mattermost channel stopped")
	return nil
}

// Send sends a message through Mattermost.
func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	c.mu.RLock()
	status := c.status
	c.mu.RUnlock()

	if status != channel.StatusConnected {
		return fmt.Errorf("channel not connected")
	}

	var fileIDs []string
	var contentParts []string
	if msg.Content != "" {
		contentParts = append(contentParts, msg.Content)
	}

	for _, att := range msg.Attachments {
		if len(att.Data) > 0 {
			name := att.Name
			if name == "" {
				name = "file"
			}
			fid, err := c.uploadFile(ctx, msg.ChatID, name, att.Data)
			if err == nil {
				fileIDs = append(fileIDs, fid)
				continue
			}
			c.logger.Warn("failed to upload file to mattermost",
				zap.String("type", string(att.Type)),
				zap.String("name", name),
				zap.Error(err))
		}
		if fallback := mattermostAttachmentFallbackText(att); fallback != "" {
			contentParts = append(contentParts, fallback)
		}
	}

	message := strings.Join(contentParts, "\n")
	props, hasProps, err := mattermostPostProps(msg.Metadata)
	if err != nil {
		return err
	}
	if strings.TrimSpace(message) == "" && len(fileIDs) == 0 && !hasProps {
		return fmt.Errorf("no sendable Mattermost content")
	}

	post := map[string]interface{}{
		"channel_id": msg.ChatID,
		"message":    message,
	}
	if msg.ReplyToID != "" {
		post["root_id"] = msg.ReplyToID
	}
	if len(fileIDs) > 0 {
		post["file_ids"] = fileIDs
	}
	if hasProps {
		post["props"] = props
	}

	postJSON, err := json.Marshal(post)
	if err != nil {
		return fmt.Errorf("failed to marshal post: %w", err)
	}

	url := fmt.Sprintf("%s/api/v4/posts", c.config.ServerURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(postJSON)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.BotToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to send message: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

func mattermostPostProps(metadata map[string]interface{}) (map[string]interface{}, bool, error) {
	if metadata == nil {
		return nil, false, nil
	}
	props := map[string]interface{}{}
	if rawProps, ok := metadata["props"]; ok {
		switch value := rawProps.(type) {
		case map[string]interface{}:
			for key, item := range value {
				props[key] = item
			}
		default:
			body, err := json.Marshal(value)
			if err != nil {
				return nil, false, fmt.Errorf("failed to marshal Mattermost props: %w", err)
			}
			if err := json.Unmarshal(body, &props); err != nil {
				return nil, false, fmt.Errorf("invalid Mattermost props metadata: %w", err)
			}
		}
	}
	if rawAttachments, ok := metadata["attachments"]; ok {
		switch value := rawAttachments.(type) {
		case []interface{}:
			props["attachments"] = value
		case []map[string]interface{}:
			attachments := make([]interface{}, 0, len(value))
			for _, item := range value {
				attachments = append(attachments, item)
			}
			props["attachments"] = attachments
		default:
			body, err := json.Marshal(value)
			if err != nil {
				return nil, false, fmt.Errorf("failed to marshal Mattermost attachments: %w", err)
			}
			var attachments []interface{}
			if err := json.Unmarshal(body, &attachments); err != nil {
				return nil, false, fmt.Errorf("invalid Mattermost attachments metadata: %w", err)
			}
			props["attachments"] = attachments
		}
	}
	if len(props) == 0 {
		return nil, false, nil
	}
	return props, true, nil
}

func mattermostAttachmentFallbackText(att channel.Attachment) string {
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

// uploadFile uploads a file to Mattermost and returns the file ID.
func (c *Channel) uploadFile(ctx context.Context, channelID, filename string, data []byte) (string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.WriteField("channel_id", channelID)
	part, err := w.CreateFormFile("files", filename)
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	part.Write(data)
	w.Close()

	url := fmt.Sprintf("%s/api/v4/files", c.config.ServerURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.config.BotToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("mattermost upload error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		FileInfos []struct {
			ID string `json:"id"`
		} `json:"file_infos"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode upload response: %w", err)
	}
	if len(result.FileInfos) == 0 {
		return "", fmt.Errorf("no file info in upload response")
	}
	return result.FileInfos[0].ID, nil
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

	// Mattermost doesn't support true streaming, so we accumulate and send at the end
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
		Name:         "mattermost",
		Type:         "mattermost",
		Status:       c.status,
		Enabled:      c.config.Enabled,
		ConnectedAt:  c.connectedAt,
		LastError:    c.lastError,
		LastErrorAt:  c.lastErrorAt,
		MessageCount: c.msgCount.Load(),
		Metadata:     make(map[string]interface{}),
	}

	info.Metadata["server_url"] = c.config.ServerURL
	if c.botUserID != "" {
		info.Metadata["bot_user_id"] = c.botUserID
		info.Metadata["bot_username"] = c.botUsername
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

// isChannelAllowed checks if a channel is allowed.
func (c *Channel) isChannelAllowed(channelID string) bool {
	if len(c.config.AllowedChannels) == 0 {
		return true
	}

	for _, allowed := range c.config.AllowedChannels {
		if allowed == channelID {
			return true
		}
	}
	return false
}

// isUserAllowed checks if a user is allowed.
func (c *Channel) isUserAllowed(userID string) bool {
	if len(c.config.AllowedUsers) == 0 {
		return true
	}

	for _, allowed := range c.config.AllowedUsers {
		if allowed == userID {
			return true
		}
	}
	return false
}

// HandleWebhook processes an incoming webhook from Mattermost.
// This should be called from a webhook handler.
func (c *Channel) HandleWebhook(event *WebhookEvent) {
	if event == nil {
		return
	}

	// Only handle posted events
	if event.Event != "posted" {
		return
	}

	// Parse the post data
	var post Post
	if err := json.Unmarshal([]byte(event.Data.Post), &post); err != nil {
		c.logger.Error("failed to parse post data", zap.Error(err))
		return
	}

	// Ignore messages from the bot itself
	if post.UserID == c.botUserID {
		return
	}

	// Check if channel is allowed
	if !c.isChannelAllowed(post.ChannelID) {
		c.logger.Debug("ignoring message from non-allowed channel",
			zap.String("channel_id", post.ChannelID))
		return
	}

	// Check if user is allowed
	if !c.isUserAllowed(post.UserID) {
		c.logger.Debug("ignoring message from non-allowed user",
			zap.String("user_id", post.UserID))
		return
	}

	// Convert to unified message format
	msg := c.convertPost(&post, event)
	c.msgCount.Add(1)

	select {
	case c.messages <- msg:
	default:
		c.logger.Warn("message channel full, dropping message",
			zap.String("message_id", msg.ID))
	}
}

// convertPost converts a Mattermost post to the unified format.
func (c *Channel) convertPost(post *Post, event *WebhookEvent) channel.Message {
	username := ""
	if event.Data.SenderName != "" {
		username = strings.TrimPrefix(event.Data.SenderName, "@")
	}

	msg := channel.Message{
		ID:          post.ID,
		ChannelName: "mattermost",
		ChatID:      post.ChannelID,
		UserID:      post.UserID,
		Username:    username,
		Type:        channel.MessageTypeText,
		Content:     post.Message,
		Timestamp:   time.UnixMilli(post.CreateAt),
		IsGroup:     event.Data.ChannelType != "D", // D = direct message
		GroupName:   event.Data.ChannelDisplayName,
		Metadata: map[string]interface{}{
			"team_id":      event.Data.TeamID,
			"channel_type": event.Data.ChannelType,
		},
	}

	if post.RootID != "" {
		msg.ReplyToID = post.RootID
	}

	return msg
}

// WebhookEvent represents a Mattermost webhook event.
type WebhookEvent struct {
	Event string           `json:"event"`
	Data  WebhookEventData `json:"data"`
}

// WebhookEventData contains the data for a webhook event.
type WebhookEventData struct {
	ChannelDisplayName string `json:"channel_display_name"`
	ChannelName        string `json:"channel_name"`
	ChannelType        string `json:"channel_type"`
	Post               string `json:"post"` // JSON string of Post
	SenderName         string `json:"sender_name"`
	TeamID             string `json:"team_id"`
}

// Post represents a Mattermost post.
type Post struct {
	ID        string `json:"id"`
	CreateAt  int64  `json:"create_at"`
	UpdateAt  int64  `json:"update_at"`
	DeleteAt  int64  `json:"delete_at"`
	UserID    string `json:"user_id"`
	ChannelID string `json:"channel_id"`
	RootID    string `json:"root_id"`
	Message   string `json:"message"`
	Type      string `json:"type"`
}
