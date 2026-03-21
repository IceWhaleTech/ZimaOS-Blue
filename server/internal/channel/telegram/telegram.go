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
	"unicode/utf16"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"
)

// CommandHandler is a function that handles bot commands.
type CommandHandler func(ctx context.Context, cmd string, args string, msg *tgbotapi.Message) (string, *InlineKeyboard, error)

// InlineKeyboard represents an inline keyboard for Telegram messages.
type InlineKeyboard struct {
	Rows [][]InlineButton `json:"rows"`
}

// InlineButton represents a button in an inline keyboard.
type InlineButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
	URL          string `json:"url,omitempty"`
}

// CallbackHandler is a function that handles callback queries from inline keyboards.
type CallbackHandler func(ctx context.Context, query *tgbotapi.CallbackQuery) (string, error)

// MessageHandler handles incoming messages and returns AI response.
type MessageHandler func(ctx context.Context, msg channel.Message) (string, error)

// Channel implements the channel.Channel interface for Telegram.
type Channel struct {
	config   channel.TelegramConfig
	logger   *zap.Logger
	bot      *tgbotapi.BotAPI
	messages chan channel.Message

	mu            sync.RWMutex
	status        channel.Status
	connectedAt   *time.Time
	lastError     string
	lastErrorAt   *time.Time
	msgCount      atomic.Int64
	msgsReceived  atomic.Int64
	msgsSent      atomic.Int64
	lastMessageAt *time.Time
	lastReplyAt   *time.Time

	// Command handlers
	commandHandlers  map[string]CommandHandler
	callbackHandlers map[string]CallbackHandler
	commandMu        sync.RWMutex

	// Message handler for AI processing
	messageHandler MessageHandler

	// Bot session manager for monitoring
	sessionManager *channel.BotSessionManager

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a new Telegram channel.
func New(cfg channel.TelegramConfig, logger *zap.Logger) *Channel {
	c := &Channel{
		config:           cfg,
		logger:           logger.With(zap.String("channel", "telegram")),
		messages:         make(chan channel.Message, 100),
		status:           channel.StatusDisconnected,
		commandHandlers:  make(map[string]CommandHandler),
		callbackHandlers: make(map[string]CallbackHandler),
	}

	// Register default command handlers
	c.RegisterCommand("start", c.handleStartCommand)
	c.RegisterCommand("help", c.handleHelpCommand)

	return c
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "telegram"
}

// Type returns the channel type.
func (c *Channel) Type() string {
	return "telegram"
}

func (c *Channel) OutboundCapabilities() channel.OutboundCapabilities {
	return channel.OutboundCapabilities{
		MarkdownMode:           channel.OutboundMarkdownModeChunked,
		HumanizerPreset:        "telegram",
		SupportsMarkdownFormat: true,
	}
}

// SetMessageHandler sets the handler for processing messages.
func (c *Channel) SetMessageHandler(handler MessageHandler) {
	c.messageHandler = handler
}

// SetSessionManager sets the bot session manager for monitoring.
func (c *Channel) SetSessionManager(manager *channel.BotSessionManager) {
	c.sessionManager = manager
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
		transport := network.NewPooledTransport(false)
		transport.Proxy = http.ProxyURL(proxyURL)
		httpClient := &http.Client{
			Transport: transport,
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
	// Handle callback queries from inline keyboards
	if update.CallbackQuery != nil {
		c.handleCallbackQuery(update.CallbackQuery)
		return
	}

	if update.Message == nil {
		return
	}

	msg := update.Message
	if msg.From == nil {
		c.logger.Debug("ignoring message without sender")
		return
	}

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

	// Handle commands
	if msg.IsCommand() {
		c.handleCommand(msg)
		return
	}

	// Convert to unified message format
	channelMsg := c.convertMessage(msg)
	c.msgCount.Add(1)
	c.msgsReceived.Add(1)
	now := time.Now()
	c.mu.Lock()
	c.lastMessageAt = &now
	c.mu.Unlock()

	chatID := fmt.Sprintf("%d", msg.Chat.ID)
	userID := fmt.Sprintf("%d", msg.From.ID)

	// Get or create session for monitoring
	var sessionID string
	if c.sessionManager != nil {
		sessionID, _ = c.sessionManager.GetOrCreateSession(c.ctx, chatID, userID)
		c.sessionManager.EmitMessageReceived(sessionID, userID, channelMsg.Content)
	}

	// If message handler is set, process and reply
	if c.messageHandler != nil {
		replyToID := channelMsg.ID
		go func() {
			response, err := c.messageHandler(c.ctx, channelMsg)
			if err != nil {
				c.logger.Error("message handler error",
					zap.Error(err),
					zap.String("chat_id", chatID),
					zap.String("user_id", userID))
				if c.sessionManager != nil && sessionID != "" {
					c.sessionManager.EmitError(sessionID, userID, err.Error())
				}
				errMsg := channel.OutgoingMessage{ChatID: chatID, ReplyToID: replyToID, Content: "处理消息时发生错误，请稍后重试。"}
				if sendErr := c.Send(c.ctx, errMsg); sendErr != nil {
					c.logger.Error("failed to send error response", zap.Error(sendErr))
				}
				return
			}
			if response == "" {
				c.logger.Warn("message handler returned empty response",
					zap.String("chat_id", chatID),
					zap.String("user_id", userID))
				return
			}
			outMsg := channel.OutgoingMessage{ChatID: chatID, ReplyToID: replyToID, Content: response}
			if err := c.Send(c.ctx, outMsg); err != nil {
				c.logger.Error("failed to send response", zap.Error(err))
			} else if c.sessionManager != nil && sessionID != "" {
				c.sessionManager.EmitMessageSent(sessionID, userID, response)
			}
		}()
		return
	}

	// Fallback: send to message channel for external processing
	select {
	case c.messages <- channelMsg:
	default:
		c.logger.Warn("message channel full, dropping message",
			zap.String("message_id", channelMsg.ID))
	}
}

// convertMessage converts a Telegram message to the unified format.
func (c *Channel) convertMessage(msg *tgbotapi.Message) channel.Message {
	content, entities, entitySource := telegramMessageContentAndEntities(msg)
	metadata := map[string]interface{}{}
	if msg.Chat != nil {
		metadata["chat_type"] = msg.Chat.Type
	}
	if entityMetadata, mentions, mentionIDs := telegramEntitiesMetadata(content, entities); len(entityMetadata) > 0 {
		metadata["entities"] = entityMetadata
		metadata["entity_source"] = entitySource
		if len(mentions) > 0 {
			metadata["mentions"] = mentions
		}
		if len(mentionIDs) > 0 {
			metadata["mention_ids"] = mentionIDs
		}
	}

	channelMsg := channel.Message{
		ID:          fmt.Sprintf("%d", msg.MessageID),
		ChannelName: "telegram",
		Type:        channel.MessageTypeText,
		Content:     content,
		Timestamp:   time.Unix(int64(msg.Date), 0),
		Metadata:    metadata,
	}
	if msg.Chat != nil {
		channelMsg.ChatID = fmt.Sprintf("%d", msg.Chat.ID)
		channelMsg.IsGroup = msg.Chat.IsGroup() || msg.Chat.IsSuperGroup()
		if channelMsg.IsGroup {
			channelMsg.GroupName = msg.Chat.Title
		}
	}
	if msg.From != nil {
		channelMsg.UserID = fmt.Sprintf("%d", msg.From.ID)
		channelMsg.Username = telegramUsername(msg.From)
	}

	if msg.ReplyToMessage != nil {
		channelMsg.ReplyToID = fmt.Sprintf("%d", msg.ReplyToMessage.MessageID)
	}

	if len(msg.Photo) > 0 {
		photo := msg.Photo[len(msg.Photo)-1]
		channelMsg.Attachments = append(channelMsg.Attachments, channel.Attachment{
			ID:   photo.FileID,
			Type: channel.MessageTypeImage,
			Name: "photo.jpg",
			Size: int64(photo.FileSize),
		})
	}
	if msg.Video != nil {
		name := strings.TrimSpace(msg.Video.FileName)
		if name == "" {
			name = "video.mp4"
		}
		channelMsg.Attachments = append(channelMsg.Attachments, channel.Attachment{
			ID:       msg.Video.FileID,
			Type:     channel.MessageTypeVideo,
			Name:     name,
			MimeType: msg.Video.MimeType,
			Size:     int64(msg.Video.FileSize),
		})
	}
	if msg.Audio != nil {
		name := strings.TrimSpace(msg.Audio.FileName)
		if name == "" && strings.TrimSpace(msg.Audio.Title) != "" {
			name = strings.TrimSpace(msg.Audio.Title)
		}
		if name == "" {
			name = "audio"
		}
		channelMsg.Attachments = append(channelMsg.Attachments, channel.Attachment{
			ID:       msg.Audio.FileID,
			Type:     channel.MessageTypeAudio,
			Name:     name,
			MimeType: msg.Audio.MimeType,
			Size:     int64(msg.Audio.FileSize),
		})
	}
	if msg.Voice != nil {
		channelMsg.Attachments = append(channelMsg.Attachments, channel.Attachment{
			ID:       msg.Voice.FileID,
			Type:     channel.MessageTypeAudio,
			Name:     "voice.ogg",
			MimeType: msg.Voice.MimeType,
			Size:     int64(msg.Voice.FileSize),
		})
	}
	if msg.Document != nil {
		name := strings.TrimSpace(msg.Document.FileName)
		if name == "" {
			name = "document"
		}
		channelMsg.Attachments = append(channelMsg.Attachments, channel.Attachment{
			ID:       msg.Document.FileID,
			Type:     channel.MessageTypeFile,
			Name:     name,
			MimeType: msg.Document.MimeType,
			Size:     int64(msg.Document.FileSize),
		})
	}
	if msg.Sticker != nil {
		stickerType := channel.MessageTypeImage
		name := "sticker.webp"
		if msg.Sticker.IsAnimated {
			stickerType = channel.MessageTypeFile
			name = "sticker.tgs"
		}
		channelMsg.Attachments = append(channelMsg.Attachments, channel.Attachment{
			ID:   msg.Sticker.FileID,
			Type: stickerType,
			Name: name,
			Size: int64(msg.Sticker.FileSize),
		})
		if strings.TrimSpace(msg.Sticker.Emoji) != "" {
			channelMsg.Metadata["sticker_emoji"] = strings.TrimSpace(msg.Sticker.Emoji)
		}
		if strings.TrimSpace(msg.Sticker.SetName) != "" {
			channelMsg.Metadata["sticker_set_name"] = strings.TrimSpace(msg.Sticker.SetName)
		}
	}

	if len(channelMsg.Attachments) > 0 {
		channelMsg.Type = channelMsg.Attachments[0].Type
	}

	return channelMsg
}

func telegramUsername(user *tgbotapi.User) string {
	if user == nil {
		return ""
	}
	if strings.TrimSpace(user.UserName) != "" {
		return strings.TrimSpace(user.UserName)
	}
	return strings.TrimSpace(user.FirstName + " " + user.LastName)
}

func telegramMessageContentAndEntities(msg *tgbotapi.Message) (string, []tgbotapi.MessageEntity, string) {
	if msg == nil {
		return "", nil, ""
	}
	if strings.TrimSpace(msg.Caption) != "" || len(msg.CaptionEntities) > 0 {
		return msg.Caption, msg.CaptionEntities, "caption"
	}
	return msg.Text, msg.Entities, "text"
}

func telegramEntitiesMetadata(text string, entities []tgbotapi.MessageEntity) ([]map[string]interface{}, []map[string]interface{}, []string) {
	if len(entities) == 0 {
		return nil, nil, nil
	}
	items := make([]map[string]interface{}, 0, len(entities))
	mentions := make([]map[string]interface{}, 0)
	mentionIDs := make([]string, 0)
	seenMentionIDs := make(map[string]struct{})
	for _, entity := range entities {
		item := map[string]interface{}{
			"type":   entity.Type,
			"offset": entity.Offset,
			"length": entity.Length,
		}
		entityText := strings.TrimSpace(telegramUTF16Substring(text, entity.Offset, entity.Length))
		if entityText != "" {
			item["text"] = entityText
		}
		if strings.TrimSpace(entity.URL) != "" {
			item["url"] = strings.TrimSpace(entity.URL)
		}
		if strings.TrimSpace(entity.Language) != "" {
			item["language"] = strings.TrimSpace(entity.Language)
		}
		if entity.User != nil {
			userMeta := map[string]interface{}{"id": fmt.Sprintf("%d", entity.User.ID)}
			if username := telegramUsername(entity.User); username != "" {
				userMeta["username"] = username
			}
			item["user"] = userMeta
		}
		items = append(items, item)

		switch entity.Type {
		case "mention":
			mention := map[string]interface{}{"type": "mention"}
			if entityText != "" {
				mention["text"] = entityText
				mention["username"] = strings.TrimPrefix(entityText, "@")
			}
			if len(mention) > 1 {
				mentions = append(mentions, mention)
			}
		case "text_mention":
			if entity.User == nil {
				continue
			}
			mentionID := fmt.Sprintf("%d", entity.User.ID)
			mention := map[string]interface{}{
				"type": "mention",
				"id":   mentionID,
			}
			if username := telegramUsername(entity.User); username != "" {
				mention["username"] = username
			}
			if entityText != "" {
				mention["text"] = entityText
			}
			mentions = append(mentions, mention)
			if _, exists := seenMentionIDs[mentionID]; !exists {
				seenMentionIDs[mentionID] = struct{}{}
				mentionIDs = append(mentionIDs, mentionID)
			}
		}
	}
	return items, mentions, mentionIDs
}

func telegramUTF16Substring(text string, offset int, length int) string {
	if offset < 0 || length <= 0 {
		return ""
	}
	encoded := utf16.Encode([]rune(text))
	if offset >= len(encoded) {
		return ""
	}
	end := offset + length
	if end > len(encoded) {
		end = len(encoded)
	}
	return string(utf16.Decode(encoded[offset:end]))
}

// isUserAllowed checks if a user is allowed to interact with the bot.
func (c *Channel) isUserAllowed(user *tgbotapi.User) bool {
	if user == nil {
		return false
	}
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

// handleCommand processes a bot command.
func (c *Channel) handleCommand(msg *tgbotapi.Message) {
	cmd := msg.Command()
	args := msg.CommandArguments()

	c.logger.Debug("handling command",
		zap.String("command", cmd),
		zap.String("args", args),
		zap.Int64("user_id", msg.From.ID))

	c.commandMu.RLock()
	handler, exists := c.commandHandlers[cmd]
	c.commandMu.RUnlock()

	if !exists {
		// Unknown command - send to message channel for AI handling
		channelMsg := c.convertMessage(msg)
		channelMsg.Metadata["is_command"] = true
		channelMsg.Metadata["command"] = cmd
		channelMsg.Metadata["command_args"] = args
		c.msgCount.Add(1)
		c.msgsReceived.Add(1)
		now := time.Now()
		c.mu.Lock()
		c.lastMessageAt = &now
		c.mu.Unlock()

		select {
		case c.messages <- channelMsg:
		default:
			c.logger.Warn("message channel full, dropping command",
				zap.String("command", cmd))
		}
		return
	}

	// Execute the command handler
	text, keyboard, err := handler(c.ctx, cmd, args, msg)
	if err != nil {
		c.logger.Error("command handler error",
			zap.String("command", cmd),
			zap.Error(err))
		text = "Sorry, an error occurred while processing your command."
	}

	if text != "" {
		chatID := fmt.Sprintf("%d", msg.Chat.ID)
		replyToID := fmt.Sprintf("%d", msg.MessageID)

		parseMode := ""
		if strings.Contains(text, "*") || strings.Contains(text, "_") {
			parseMode = tgbotapi.ModeMarkdown
		}

		if err := c.SendWithKeyboard(c.ctx, chatID, text, keyboard, replyToID, parseMode); err != nil {
			c.logger.Error("failed to send command response",
				zap.String("command", cmd),
				zap.Error(err))
		}
	}
}

// handleCallbackQuery processes a callback query from an inline keyboard.
func (c *Channel) handleCallbackQuery(query *tgbotapi.CallbackQuery) {
	if query == nil || query.From == nil {
		c.logger.Debug("ignoring callback without sender")
		return
	}
	c.logger.Debug("handling callback query",
		zap.String("data", query.Data),
		zap.Int64("user_id", query.From.ID))

	// Check if user is allowed
	if !c.isUserAllowed(query.From) {
		c.logger.Debug("ignoring callback from non-allowed user",
			zap.Int64("user_id", query.From.ID))
		return
	}

	// Handle built-in callbacks (cmd:xxx format)
	if strings.HasPrefix(query.Data, "cmd:") {
		cmdName := strings.TrimPrefix(query.Data, "cmd:")
		c.handleBuiltinCallback(query, cmdName)
		return
	}

	// Find matching callback handler by prefix
	c.commandMu.RLock()
	var matchedHandler CallbackHandler
	var matchedPrefix string
	for prefix, handler := range c.callbackHandlers {
		if strings.HasPrefix(query.Data, prefix) {
			matchedHandler = handler
			matchedPrefix = prefix
			break
		}
	}
	c.commandMu.RUnlock()

	if matchedHandler != nil {
		response, err := matchedHandler(c.ctx, query)
		if err != nil {
			c.logger.Error("callback handler error",
				zap.String("prefix", matchedPrefix),
				zap.Error(err))
			c.AnswerCallbackQuery(c.ctx, query.ID, "An error occurred", true)
			return
		}
		c.AnswerCallbackQuery(c.ctx, query.ID, response, false)
		return
	}

	// No handler found - send to message channel for AI handling
	if query.Message != nil {
		originMessageID := fmt.Sprintf("%d", query.Message.MessageID)
		channelMsg := channel.Message{
			ID:          fmt.Sprintf("cb_%s", query.ID),
			ChannelName: "telegram",
			ChatID:      fmt.Sprintf("%d", query.Message.Chat.ID),
			UserID:      fmt.Sprintf("%d", query.From.ID),
			Username:    telegramUsername(query.From),
			Type:        channel.MessageTypeText,
			Content:     query.Data,
			ReplyToID:   originMessageID,
			Timestamp:   time.Now(),
			IsGroup:     query.Message.Chat.IsGroup() || query.Message.Chat.IsSuperGroup(),
			Metadata: map[string]interface{}{
				"is_callback":       true,
				"callback_id":       query.ID,
				"callback_data":     query.Data,
				"message_id":        query.Message.MessageID,
				"origin_message_id": originMessageID,
				"inline_message":    query.InlineMessageID,
				"chat_instance":     query.ChatInstance,
			},
		}
		if channelMsg.IsGroup {
			channelMsg.GroupName = query.Message.Chat.Title
		}

		select {
		case c.messages <- channelMsg:
			c.AnswerCallbackQuery(c.ctx, query.ID, "", false)
		default:
			c.logger.Warn("message channel full, dropping callback")
			c.AnswerCallbackQuery(c.ctx, query.ID, "System busy, please try again", true)
		}
	}
}

// handleBuiltinCallback handles built-in callback commands.
func (c *Channel) handleBuiltinCallback(query *tgbotapi.CallbackQuery, cmdName string) {
	c.commandMu.RLock()
	handler, exists := c.commandHandlers[cmdName]
	c.commandMu.RUnlock()

	if !exists {
		c.AnswerCallbackQuery(c.ctx, query.ID, "Unknown command", true)
		return
	}

	// Create a synthetic message for the handler
	var msg *tgbotapi.Message
	if query.Message != nil {
		msg = query.Message
		msg.From = query.From
	}

	text, keyboard, err := handler(c.ctx, cmdName, "", msg)
	if err != nil {
		c.logger.Error("callback command handler error",
			zap.String("command", cmdName),
			zap.Error(err))
		c.AnswerCallbackQuery(c.ctx, query.ID, "An error occurred", true)
		return
	}

	// Answer the callback
	c.AnswerCallbackQuery(c.ctx, query.ID, "", false)

	// Edit the message with new content
	if text != "" && query.Message != nil {
		chatID := fmt.Sprintf("%d", query.Message.Chat.ID)
		messageID := fmt.Sprintf("%d", query.Message.MessageID)

		parseMode := ""
		if strings.Contains(text, "*") || strings.Contains(text, "_") {
			parseMode = tgbotapi.ModeMarkdown
		}

		if err := c.EditMessageWithKeyboard(c.ctx, chatID, messageID, text, keyboard, parseMode); err != nil {
			c.logger.Error("failed to edit message for callback",
				zap.String("command", cmdName),
				zap.Error(err))
		}
	}
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

	captionConsumed := false

	// Send attachments first (image/video/audio/file)
	for _, att := range msg.Attachments {
		caption := ""
		if !captionConsumed {
			caption = msg.Content
		}
		if err := c.sendAttachment(chatID, msg.ReplyToID, caption, msg.Format, att); err != nil {
			c.logger.Warn("failed to send attachment, falling back to URL",
				zap.String("type", string(att.Type)), zap.Error(err))
			if att.URL != "" {
				fallbackText := att.URL
				if caption != "" {
					fallbackText = caption + "\n" + att.URL
				}
				if err := c.sendTextMessage(chatID, msg.ReplyToID, fallbackText, msg.Format); err != nil {
					return fmt.Errorf("failed to send attachment fallback: %w", err)
				}
				if caption != "" {
					captionConsumed = true
				}
			}
		} else if caption != "" {
			captionConsumed = true
		}
	}

	// Send text content only if it wasn't already used as attachment caption.
	if msg.Content != "" && !captionConsumed {
		if err := c.sendTextMessage(chatID, msg.ReplyToID, msg.Content, msg.Format); err != nil {
			return err
		}
	}

	c.msgsSent.Add(1)
	now := time.Now()
	c.mu.Lock()
	c.lastReplyAt = &now
	c.mu.Unlock()
	return nil
}

// SendWithID sends a text-only Telegram message and returns the created message ID.
// For attachment-heavy payloads we fall back to Send and return an empty ID.
func (c *Channel) SendWithID(ctx context.Context, msg channel.OutgoingMessage) (string, error) {
	if c.bot == nil {
		return "", fmt.Errorf("bot not initialized")
	}
	if len(msg.Attachments) > 0 {
		if err := c.Send(ctx, msg); err != nil {
			return "", err
		}
		return "", nil
	}
	if msg.Content == "" {
		return "", fmt.Errorf("no sendable Telegram content")
	}
	chatID, err := parseChatID(msg.ChatID)
	if err != nil {
		return "", fmt.Errorf("invalid chat ID: %w", err)
	}
	tgMsg := tgbotapi.NewMessage(chatID, msg.Content)
	tgMsg.ParseMode = telegramParseMode(msg.Format)
	if replyID, ok := telegramReplyID(msg.ReplyToID); ok {
		tgMsg.ReplyToMessageID = replyID
	}
	sent, err := c.bot.Send(tgMsg)
	if err != nil {
		return "", fmt.Errorf("failed to send message: %w", err)
	}
	c.msgsSent.Add(1)
	now := time.Now()
	c.mu.Lock()
	c.lastReplyAt = &now
	c.mu.Unlock()
	return fmt.Sprintf("%d", sent.MessageID), nil
}

// EditMessage updates an existing Telegram message in place.
func (c *Channel) EditMessage(ctx context.Context, chatID string, messageID string, msg channel.OutgoingMessage) error {
	return c.EditMessageWithKeyboard(ctx, chatID, messageID, msg.Content, nil, telegramParseMode(msg.Format))
}

// sendAttachment sends a single media attachment via Telegram Bot API.
func (c *Channel) sendAttachment(chatID int64, replyToID string, caption string, format string, att channel.Attachment) error {
	if len(att.Data) == 0 && att.URL == "" {
		return fmt.Errorf("no data or URL for attachment")
	}

	chattable, err := buildAttachmentChattable(chatID, replyToID, caption, format, att)
	if err != nil {
		return err
	}

	_, err = c.bot.Send(chattable)
	return err
}

func (c *Channel) sendTextMessage(chatID int64, replyToID string, content string, format string) error {
	tgMsg := tgbotapi.NewMessage(chatID, content)
	tgMsg.ParseMode = telegramParseMode(format)
	if replyID, ok := telegramReplyID(replyToID); ok {
		tgMsg.ReplyToMessageID = replyID
	}
	if _, err := c.bot.Send(tgMsg); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	return nil
}

func buildAttachmentChattable(chatID int64, replyToID string, caption string, format string, att channel.Attachment) (tgbotapi.Chattable, error) {
	file := tgbotapi.FileBytes{Name: att.Name, Bytes: att.Data}
	if len(att.Data) == 0 {
		switch att.Type {
		case channel.MessageTypeImage:
			photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileURL(att.URL))
			applyPhotoOptions(&photo, replyToID, caption, format)
			return photo, nil
		case channel.MessageTypeVideo:
			video := tgbotapi.NewVideo(chatID, tgbotapi.FileURL(att.URL))
			applyVideoOptions(&video, replyToID, caption, format)
			return video, nil
		case channel.MessageTypeAudio:
			audio := tgbotapi.NewAudio(chatID, tgbotapi.FileURL(att.URL))
			applyAudioOptions(&audio, replyToID, caption, format)
			return audio, nil
		default:
			doc := tgbotapi.NewDocument(chatID, tgbotapi.FileURL(att.URL))
			applyDocumentOptions(&doc, replyToID, caption, format)
			return doc, nil
		}
	}

	if file.Name == "" {
		switch att.Type {
		case channel.MessageTypeImage:
			file.Name = "image.png"
		case channel.MessageTypeVideo:
			file.Name = "video.mp4"
		case channel.MessageTypeAudio:
			file.Name = "audio.ogg"
		default:
			file.Name = "file"
		}
	}

	switch att.Type {
	case channel.MessageTypeImage:
		photo := tgbotapi.NewPhoto(chatID, file)
		applyPhotoOptions(&photo, replyToID, caption, format)
		return photo, nil
	case channel.MessageTypeVideo:
		video := tgbotapi.NewVideo(chatID, file)
		applyVideoOptions(&video, replyToID, caption, format)
		return video, nil
	case channel.MessageTypeAudio:
		audio := tgbotapi.NewAudio(chatID, file)
		applyAudioOptions(&audio, replyToID, caption, format)
		return audio, nil
	default:
		doc := tgbotapi.NewDocument(chatID, file)
		applyDocumentOptions(&doc, replyToID, caption, format)
		return doc, nil
	}
}

func applyPhotoOptions(photo *tgbotapi.PhotoConfig, replyToID string, caption string, format string) {
	photo.Caption = caption
	photo.ParseMode = telegramParseMode(format)
	if replyID, ok := telegramReplyID(replyToID); ok {
		photo.ReplyToMessageID = replyID
	}
}

func applyVideoOptions(video *tgbotapi.VideoConfig, replyToID string, caption string, format string) {
	video.Caption = caption
	video.ParseMode = telegramParseMode(format)
	if replyID, ok := telegramReplyID(replyToID); ok {
		video.ReplyToMessageID = replyID
	}
}

func applyAudioOptions(audio *tgbotapi.AudioConfig, replyToID string, caption string, format string) {
	audio.Caption = caption
	audio.ParseMode = telegramParseMode(format)
	if replyID, ok := telegramReplyID(replyToID); ok {
		audio.ReplyToMessageID = replyID
	}
}

func applyDocumentOptions(doc *tgbotapi.DocumentConfig, replyToID string, caption string, format string) {
	doc.Caption = caption
	doc.ParseMode = telegramParseMode(format)
	if replyID, ok := telegramReplyID(replyToID); ok {
		doc.ReplyToMessageID = replyID
	}
}

func telegramParseMode(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "markdown", "md":
		return tgbotapi.ModeMarkdown
	case "html":
		return tgbotapi.ModeHTML
	case "markdownv2":
		return tgbotapi.ModeMarkdownV2
	default:
		return ""
	}
}

func telegramReplyID(replyToID string) (int, bool) {
	if replyToID == "" {
		return 0, false
	}
	replyID, err := parseMessageID(replyToID)
	if err != nil {
		return 0, false
	}
	return replyID, true
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

// SendTyping sends a "typing" chat action to the given Telegram chat.
func (c *Channel) SendTyping(ctx context.Context, chatID string) error {
	if c.bot == nil {
		return fmt.Errorf("bot not initialized")
	}
	parsed, err := parseChatID(chatID)
	if err != nil {
		return err
	}
	action := tgbotapi.NewChatAction(parsed, tgbotapi.ChatTyping)
	_, err = c.bot.Send(action)
	return err
}

// Info returns current information about the channel.
func (c *Channel) Info() channel.Info {
	c.mu.RLock()
	defer c.mu.RUnlock()

	info := channel.Info{
		Name:             "telegram",
		Type:             "telegram",
		Status:           c.status,
		Enabled:          c.config.Enabled,
		ConnectedAt:      c.connectedAt,
		LastError:        c.lastError,
		LastErrorAt:      c.lastErrorAt,
		MessageCount:     c.msgCount.Load(),
		MessagesReceived: c.msgsReceived.Load(),
		MessagesSent:     c.msgsSent.Load(),
		LastMessageAt:    c.lastMessageAt,
		LastReplyAt:      c.lastReplyAt,
		Metadata:         make(map[string]interface{}),
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

// RegisterCommand registers a command handler.
func (c *Channel) RegisterCommand(cmd string, handler CommandHandler) {
	c.commandMu.Lock()
	defer c.commandMu.Unlock()
	c.commandHandlers[cmd] = handler
}

// RegisterCallback registers a callback handler for inline keyboard buttons.
func (c *Channel) RegisterCallback(prefix string, handler CallbackHandler) {
	c.commandMu.Lock()
	defer c.commandMu.Unlock()
	c.callbackHandlers[prefix] = handler
}

// handleStartCommand handles the /start command.
func (c *Channel) handleStartCommand(ctx context.Context, cmd string, args string, msg *tgbotapi.Message) (string, *InlineKeyboard, error) {
	username := msg.From.FirstName
	if username == "" {
		username = msg.From.UserName
	}

	text := fmt.Sprintf("👋 Hello %s! Welcome to ZimaOS Blue.\n\nI'm your AI assistant. You can:\n• Send me any message to chat\n• Use /help to see available commands\n\nHow can I help you today?", username)

	keyboard := &InlineKeyboard{
		Rows: [][]InlineButton{
			{
				{Text: "📚 Help", CallbackData: "cmd:help"},
				{Text: "ℹ️ About", CallbackData: "cmd:about"},
			},
		},
	}

	return text, keyboard, nil
}

// handleHelpCommand handles the /help command.
func (c *Channel) handleHelpCommand(ctx context.Context, cmd string, args string, msg *tgbotapi.Message) (string, *InlineKeyboard, error) {
	text := `📚 *Available Commands*

/start - Start the bot and see welcome message
/help - Show this help message

*How to use:*
Simply send me any message and I'll respond using AI.

*Tips:*
• You can reply to my messages to continue a conversation
• Send images or files for analysis
• Use inline buttons when available for quick actions`

	return text, nil, nil
}

// SendWithKeyboard sends a message with an inline keyboard.
func (c *Channel) SendWithKeyboard(ctx context.Context, chatID string, text string, keyboard *InlineKeyboard, replyToID string, parseMode string) error {
	if c.bot == nil {
		return fmt.Errorf("bot not initialized")
	}

	parsedChatID, err := parseChatID(chatID)
	if err != nil {
		return fmt.Errorf("invalid chat ID: %w", err)
	}

	msg := tgbotapi.NewMessage(parsedChatID, text)

	if parseMode != "" {
		msg.ParseMode = parseMode
	}

	if replyToID != "" {
		if replyID, err := parseMessageID(replyToID); err == nil {
			msg.ReplyToMessageID = replyID
		}
	}

	if keyboard != nil && len(keyboard.Rows) > 0 {
		msg.ReplyMarkup = c.buildInlineKeyboard(keyboard)
	}

	_, err = c.bot.Send(msg)
	if err != nil {
		c.logger.Error("failed to send message with keyboard",
			zap.String("chat_id", chatID),
			zap.Error(err))
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// buildInlineKeyboard converts InlineKeyboard to tgbotapi.InlineKeyboardMarkup.
func (c *Channel) buildInlineKeyboard(keyboard *InlineKeyboard) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	for _, row := range keyboard.Rows {
		var buttons []tgbotapi.InlineKeyboardButton
		for _, btn := range row {
			if btn.URL != "" {
				buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonURL(btn.Text, btn.URL))
			} else if btn.CallbackData != "" {
				buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(btn.Text, btn.CallbackData))
			}
		}
		if len(buttons) > 0 {
			rows = append(rows, buttons)
		}
	}

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// EditMessageWithKeyboard edits an existing message with new text and keyboard.
func (c *Channel) EditMessageWithKeyboard(ctx context.Context, chatID string, messageID string, text string, keyboard *InlineKeyboard, parseMode string) error {
	if c.bot == nil {
		return fmt.Errorf("bot not initialized")
	}

	parsedChatID, err := parseChatID(chatID)
	if err != nil {
		return fmt.Errorf("invalid chat ID: %w", err)
	}

	msgID, err := parseMessageID(messageID)
	if err != nil {
		return fmt.Errorf("invalid message ID: %w", err)
	}

	editMsg := tgbotapi.NewEditMessageText(parsedChatID, msgID, text)

	if parseMode != "" {
		editMsg.ParseMode = parseMode
	}

	if keyboard != nil && len(keyboard.Rows) > 0 {
		markup := c.buildInlineKeyboard(keyboard)
		editMsg.ReplyMarkup = &markup
	}

	_, err = c.bot.Send(editMsg)
	if err != nil {
		c.logger.Error("failed to edit message",
			zap.String("chat_id", chatID),
			zap.String("message_id", messageID),
			zap.Error(err))
		return fmt.Errorf("failed to edit message: %w", err)
	}

	return nil
}

// AnswerCallbackQuery answers a callback query from an inline keyboard button.
func (c *Channel) AnswerCallbackQuery(ctx context.Context, queryID string, text string, showAlert bool) error {
	if c.bot == nil {
		return fmt.Errorf("bot not initialized")
	}

	callback := tgbotapi.NewCallback(queryID, text)
	callback.ShowAlert = showAlert

	_, err := c.bot.Request(callback)
	if err != nil {
		c.logger.Error("failed to answer callback query",
			zap.String("query_id", queryID),
			zap.Error(err))
		return fmt.Errorf("failed to answer callback: %w", err)
	}

	return nil
}
