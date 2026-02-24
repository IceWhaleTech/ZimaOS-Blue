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

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
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
	// Handle callback queries from inline keyboards
	if update.CallbackQuery != nil {
		c.handleCallbackQuery(update.CallbackQuery)
		return
	}

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
				errMsg := channel.OutgoingMessage{ChatID: chatID, Content: "处理消息时发生错误，请稍后重试。"}
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
			outMsg := channel.OutgoingMessage{ChatID: chatID, Content: response}
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
		channelMsg := channel.Message{
			ID:          fmt.Sprintf("cb_%s", query.ID),
			ChannelName: "telegram",
			ChatID:      fmt.Sprintf("%d", query.Message.Chat.ID),
			UserID:      fmt.Sprintf("%d", query.From.ID),
			Username:    query.From.UserName,
			Type:        channel.MessageTypeText,
			Content:     query.Data,
			Timestamp:   time.Now(),
			IsGroup:     query.Message.Chat.IsGroup() || query.Message.Chat.IsSuperGroup(),
			Metadata: map[string]interface{}{
				"is_callback":    true,
				"callback_id":    query.ID,
				"callback_data":  query.Data,
				"message_id":     query.Message.MessageID,
				"inline_message": query.InlineMessageID,
			},
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

	// Send attachments first (image/video/audio/file)
	for _, att := range msg.Attachments {
		if err := c.sendAttachment(chatID, msg.ReplyToID, msg.Content, att); err != nil {
			c.logger.Warn("failed to send attachment, falling back to URL",
				zap.String("type", string(att.Type)), zap.Error(err))
			if att.URL != "" {
				fallback := tgbotapi.NewMessage(chatID, att.URL)
				c.bot.Send(fallback)
			}
		}
	}

	// Send text content (skip if already sent as caption with a single attachment)
	if msg.Content != "" && len(msg.Attachments) == 0 {
		tgMsg := tgbotapi.NewMessage(chatID, msg.Content)
		switch msg.Format {
		case "markdown", "md":
			tgMsg.ParseMode = tgbotapi.ModeMarkdown
		case "html":
			tgMsg.ParseMode = tgbotapi.ModeHTML
		case "markdownv2":
			tgMsg.ParseMode = tgbotapi.ModeMarkdownV2
		}
		if msg.ReplyToID != "" {
			if replyID, err := parseMessageID(msg.ReplyToID); err == nil {
				tgMsg.ReplyToMessageID = replyID
			}
		}
		if _, err := c.bot.Send(tgMsg); err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}
	} else if msg.Content != "" && len(msg.Attachments) > 0 {
		// Attachments were sent; send remaining text as separate message
		tgMsg := tgbotapi.NewMessage(chatID, msg.Content)
		c.bot.Send(tgMsg)
	}

	c.msgsSent.Add(1)
	now := time.Now()
	c.mu.Lock()
	c.lastReplyAt = &now
	c.mu.Unlock()
	return nil
}

// sendAttachment sends a single media attachment via Telegram Bot API.
func (c *Channel) sendAttachment(chatID int64, replyToID string, caption string, att channel.Attachment) error {
	if len(att.Data) == 0 && att.URL == "" {
		return fmt.Errorf("no data or URL for attachment")
	}

	var chattable tgbotapi.Chattable

	// Prefer binary upload; fall back to URL
	file := tgbotapi.FileBytes{Name: att.Name, Bytes: att.Data}
	if len(att.Data) == 0 {
		// No binary data — send by URL
		switch att.Type {
		case channel.MessageTypeImage:
			photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileURL(att.URL))
			photo.Caption = caption
			chattable = photo
		case channel.MessageTypeVideo:
			video := tgbotapi.NewVideo(chatID, tgbotapi.FileURL(att.URL))
			video.Caption = caption
			chattable = video
		case channel.MessageTypeAudio:
			audio := tgbotapi.NewAudio(chatID, tgbotapi.FileURL(att.URL))
			audio.Caption = caption
			chattable = audio
		default:
			doc := tgbotapi.NewDocument(chatID, tgbotapi.FileURL(att.URL))
			doc.Caption = caption
			chattable = doc
		}
	} else {
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
			photo.Caption = caption
			chattable = photo
		case channel.MessageTypeVideo:
			video := tgbotapi.NewVideo(chatID, file)
			video.Caption = caption
			chattable = video
		case channel.MessageTypeAudio:
			audio := tgbotapi.NewAudio(chatID, file)
			audio.Caption = caption
			chattable = audio
		default:
			doc := tgbotapi.NewDocument(chatID, file)
			doc.Caption = caption
			chattable = doc
		}
	}

	_, err := c.bot.Send(chattable)
	return err
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
