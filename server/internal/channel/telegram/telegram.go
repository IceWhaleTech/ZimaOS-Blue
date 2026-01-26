// Package telegram provides a Telegram bot channel implementation.
package telegram

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel"
)

// Channel implements the channel.Channel interface for Telegram.
type Channel struct {
	config   channel.TelegramConfig
	logger   *zap.Logger
	bot      *tgbotapi.BotAPI
	messages chan channel.Message

	mu          sync.RWMutex
	status      channel.Status
	connectedAt *time.Time
	lastError   string
	lastErrorAt *time.Time
	msgCount    atomic.Int64

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a new Telegram channel.
func New(cfg channel.TelegramConfig, logger *zap.Logger) *Channel {
	return &Channel{
		config:   cfg,
		logger:   logger.With(zap.String("channel", "telegram")),
		messages: make(chan channel.Message, 100),
		status:   channel.StatusDisconnected,
	}
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "telegram"
}

// Type returns the channel type.
func (c *Channel) Type() string {
	return "telegram"
}

// Start initializes and starts the Telegram bot.
func (c *Channel) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.status == channel.StatusConnected || c.status == channel.StatusConnecting {
		c.mu.Unlock()
		return fmt.Errorf("channel already started")
	}
	c.status = channel.StatusConnecting
	c.mu.Unlock()

	c.ctx, c.cancel = context.WithCancel(ctx)

	// Create bot API client
	var bot *tgbotapi.BotAPI
	var err error

	if c.config.Proxy != "" {
		proxyURL, err := url.Parse(c.config.Proxy)
		if err != nil {
			c.setError(fmt.Sprintf("invalid proxy URL: %v", err))
			return fmt.Errorf("invalid proxy URL: %w", err)
		}
		httpClient := &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			},
		}
		bot, err = tgbotapi.NewBotAPIWithClient(c.config.BotToken, tgbotapi.APIEndpoint, httpClient)
	} else {
		bot, err = tgbotapi.NewBotAPI(c.config.BotToken)
	}

	if err != nil {
		c.setError(fmt.Sprintf("failed to create bot: %v", err))
		return fmt.Errorf("failed to create Telegram bot: %w", err)
	}

	c.bot = bot
	c.logger.Info("telegram bot authorized", zap.String("username", bot.Self.UserName))

	// Set up webhook or long polling
	if c.config.WebhookURL != "" {
		wh, err := tgbotapi.NewWebhook(c.config.WebhookURL)
		if err != nil {
			c.setError(fmt.Sprintf("failed to create webhook: %v", err))
			return fmt.Errorf("failed to create webhook: %w", err)
		}
		_, err = bot.Request(wh)
		if err != nil {
			c.setError(fmt.Sprintf("failed to set webhook: %v", err))
			return fmt.Errorf("failed to set webhook: %w", err)
		}
		c.logger.Info("webhook set", zap.String("url", c.config.WebhookURL))
	} else {
		// Remove any existing webhook
		bot.Request(tgbotapi.DeleteWebhookConfig{})

		// Start long polling
		c.wg.Add(1)
		go c.pollUpdates()
	}

	now := time.Now()
	c.mu.Lock()
	c.status = channel.StatusConnected
	c.connectedAt = &now
	c.lastError = ""
	c.lastErrorAt = nil
	c.mu.Unlock()

	c.logger.Info("telegram channel started")
	return nil
}

// pollUpdates polls for updates using long polling.
func (c *Channel) pollUpdates() {
	defer c.wg.Done()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := c.bot.GetUpdatesChan(u)

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Debug("stopping update polling")
			return
		case update, ok := <-updates:
			if !ok {
				c.logger.Debug("updates channel closed")
				return
			}
			c.handleUpdate(update)
		}
	}
}

// handleUpdate processes a single update from Telegram.
func (c *Channel) handleUpdate(update tgbotapi.Update) {
	if update.Message == nil {
		return
	}

	msg := update.Message

	// Check if user is allowed
	if !c.isUserAllowed(msg.From) {
		c.logger.Debug("ignoring message from non-allowed user",
			zap.Int64("user_id", msg.From.ID),
			zap.String("username", msg.From.UserName))
		return
	}

	// Check if group is allowed
	if msg.Chat.IsGroup() || msg.Chat.IsSuperGroup() {
		if !c.isGroupAllowed(msg.Chat.ID) {
			c.logger.Debug("ignoring message from non-allowed group",
				zap.Int64("chat_id", msg.Chat.ID),
				zap.String("title", msg.Chat.Title))
			return
		}
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

// convertMessage converts a Telegram message to the unified format.
func (c *Channel) convertMessage(msg *tgbotapi.Message) channel.Message {
	username := msg.From.UserName
	if username == "" {
		username = strings.TrimSpace(msg.From.FirstName + " " + msg.From.LastName)
	}

	channelMsg := channel.Message{
		ID:          fmt.Sprintf("%d", msg.MessageID),
		ChannelName: "telegram",
		ChatID:      fmt.Sprintf("%d", msg.Chat.ID),
		UserID:      fmt.Sprintf("%d", msg.From.ID),
		Username:    username,
		Type:        channel.MessageTypeText,
		Content:     msg.Text,
		Timestamp:   time.Unix(int64(msg.Date), 0),
		IsGroup:     msg.Chat.IsGroup() || msg.Chat.IsSuperGroup(),
		Metadata: map[string]interface{}{
			"chat_type": msg.Chat.Type,
		},
	}

	if channelMsg.IsGroup {
		channelMsg.GroupName = msg.Chat.Title
	}

	// Handle reply
	if msg.ReplyToMessage != nil {
		channelMsg.ReplyToID = fmt.Sprintf("%d", msg.ReplyToMessage.MessageID)
	}

	// Handle attachments
	if msg.Photo != nil && len(msg.Photo) > 0 {
		channelMsg.Type = channel.MessageTypeImage
		// Get the largest photo
		photo := msg.Photo[len(msg.Photo)-1]
		channelMsg.Attachments = append(channelMsg.Attachments, channel.Attachment{
			ID:   photo.FileID,
			Type: channel.MessageTypeImage,
			Name: "photo.jpg",
		})
		if msg.Caption != "" {
			channelMsg.Content = msg.Caption
		}
	}

	if msg.Document != nil {
		channelMsg.Type = channel.MessageTypeFile
		channelMsg.Attachments = append(channelMsg.Attachments, channel.Attachment{
			ID:       msg.Document.FileID,
			Type:     channel.MessageTypeFile,
			Name:     msg.Document.FileName,
			MimeType: msg.Document.MimeType,
			Size:     int64(msg.Document.FileSize),
		})
		if msg.Caption != "" {
			channelMsg.Content = msg.Caption
		}
	}

	return channelMsg
}

// isUserAllowed checks if a user is allowed to interact with the bot.
func (c *Channel) isUserAllowed(user *tgbotapi.User) bool {
	if len(c.config.AllowedUsers) == 0 {
		return true
	}

	userID := fmt.Sprintf("%d", user.ID)
	for _, allowed := range c.config.AllowedUsers {
		if allowed == userID || allowed == user.UserName {
			return true
		}
	}
	return false
}

// isGroupAllowed checks if a group is allowed.
func (c *Channel) isGroupAllowed(chatID int64) bool {
	if len(c.config.AllowedGroups) == 0 {
		return true
	}

	chatIDStr := fmt.Sprintf("%d", chatID)
	for _, allowed := range c.config.AllowedGroups {
		if allowed == chatIDStr {
			return true
		}
	}
	return false
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

	if c.bot != nil {
		c.bot.StopReceivingUpdates()
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
	c.logger.Info("telegram channel stopped")
	return nil
}

// Send sends a message through Telegram.
func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	if c.bot == nil {
		return fmt.Errorf("bot not initialized")
	}

	chatID, err := parseChatID(msg.ChatID)
	if err != nil {
		return fmt.Errorf("invalid chat ID: %w", err)
	}

	// Create message
	tgMsg := tgbotapi.NewMessage(chatID, msg.Content)

	// Set parse mode based on format
	switch msg.Format {
	case "markdown", "md":
		tgMsg.ParseMode = tgbotapi.ModeMarkdown
	case "html":
		tgMsg.ParseMode = tgbotapi.ModeHTML
	case "markdownv2":
		tgMsg.ParseMode = tgbotapi.ModeMarkdownV2
	}

	// Set reply
	if msg.ReplyToID != "" {
		replyID, err := parseMessageID(msg.ReplyToID)
		if err == nil {
			tgMsg.ReplyToMessageID = replyID
		}
	}

	_, err = c.bot.Send(tgMsg)
	if err != nil {
		c.logger.Error("failed to send message",
			zap.String("chat_id", msg.ChatID),
			zap.Error(err))
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// SendStreaming sends a message with streaming support.
func (c *Channel) SendStreaming(ctx context.Context, chatID string, replyToID string, content <-chan string, done chan<- struct{}) error {
	defer close(done)

	if c.bot == nil {
		return fmt.Errorf("bot not initialized")
	}

	parsedChatID, err := parseChatID(chatID)
	if err != nil {
		return fmt.Errorf("invalid chat ID: %w", err)
	}

	var fullContent strings.Builder
	var sentMsgID int
	lastUpdate := time.Now()
	updateInterval := 500 * time.Millisecond

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk, ok := <-content:
			if !ok {
				// Channel closed, send final message
				if sentMsgID > 0 && fullContent.Len() > 0 {
					editMsg := tgbotapi.NewEditMessageText(parsedChatID, sentMsgID, fullContent.String())
					c.bot.Send(editMsg)
				}
				return nil
			}

			fullContent.WriteString(chunk)

			// Update message periodically to avoid rate limiting
			if time.Since(lastUpdate) >= updateInterval {
				if sentMsgID == 0 {
					// Send initial message
					msg := tgbotapi.NewMessage(parsedChatID, fullContent.String())
					if replyToID != "" {
						if replyID, err := parseMessageID(replyToID); err == nil {
							msg.ReplyToMessageID = replyID
						}
					}
					sent, err := c.bot.Send(msg)
					if err != nil {
						c.logger.Error("failed to send streaming message", zap.Error(err))
						continue
					}
					sentMsgID = sent.MessageID
				} else {
					// Edit existing message
					editMsg := tgbotapi.NewEditMessageText(parsedChatID, sentMsgID, fullContent.String())
					c.bot.Send(editMsg)
				}
				lastUpdate = time.Now()
			}
		}
	}
}

// Info returns current information about the channel.
func (c *Channel) Info() channel.Info {
	c.mu.RLock()
	defer c.mu.RUnlock()

	info := channel.Info{
		Name:         "telegram",
		Type:         "telegram",
		Status:       c.status,
		Enabled:      c.config.Enabled,
		ConnectedAt:  c.connectedAt,
		LastError:    c.lastError,
		LastErrorAt:  c.lastErrorAt,
		MessageCount: c.msgCount.Load(),
		Metadata:     make(map[string]interface{}),
	}

	if c.bot != nil {
		info.Metadata["bot_username"] = c.bot.Self.UserName
		info.Metadata["bot_id"] = c.bot.Self.ID
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

// parseChatID parses a chat ID string to int64.
func parseChatID(s string) (int64, error) {
	var chatID int64
	_, err := fmt.Sscanf(s, "%d", &chatID)
	return chatID, err
}

// parseMessageID parses a message ID string to int.
func parseMessageID(s string) (int, error) {
	var msgID int
	_, err := fmt.Sscanf(s, "%d", &msgID)
	return msgID, err
}
