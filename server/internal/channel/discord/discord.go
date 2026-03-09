// Package discord provides a Discord bot channel implementation.
package discord

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

// SlashCommand represents a Discord slash command.
type SlashCommand struct {
	Name        string
	Description string
	Options     []*discordgo.ApplicationCommandOption
	Handler     SlashCommandHandler
}

// SlashCommandHandler handles slash command interactions.
type SlashCommandHandler func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error

// ComponentHandler handles component interactions (buttons, select menus).
type ComponentHandler func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error

// MessageHandler handles incoming messages and returns AI response.
type MessageHandler func(ctx context.Context, msg channel.Message) (string, error)

// MessageComponent represents a message component (button or select menu).
type MessageComponent struct {
	Type        discordgo.ComponentType
	CustomID    string
	Label       string
	Style       discordgo.ButtonStyle
	URL         string
	Disabled    bool
	Emoji       *discordgo.ComponentEmoji
	Options     []discordgo.SelectMenuOption // For select menus
	Placeholder string                       // For select menus
	MinValues   *int                         // For select menus
	MaxValues   int                          // For select menus
}

// ActionRow represents a row of components.
type ActionRow struct {
	Components []MessageComponent
}

// Channel implements the channel.Channel interface for Discord.
type Channel struct {
	config   channel.DiscordConfig
	logger   *zap.Logger
	session  *discordgo.Session
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

	// Slash commands
	slashCommands     map[string]*SlashCommand
	registeredCmdIDs  []string
	componentHandlers map[string]ComponentHandler
	commandMu         sync.RWMutex

	// Sharding
	shardID    int
	shardCount int

	// Message handler for AI processing
	messageHandler MessageHandler

	// Bot session manager for monitoring
	sessionManager *channel.BotSessionManager

	sendMessageFunc func(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error)

	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new Discord channel.
func New(cfg channel.DiscordConfig, logger *zap.Logger) *Channel {
	c := &Channel{
		config:            cfg,
		logger:            logger.With(zap.String("channel", "discord")),
		messages:          make(chan channel.Message, 100),
		status:            channel.StatusDisconnected,
		slashCommands:     make(map[string]*SlashCommand),
		componentHandlers: make(map[string]ComponentHandler),
		shardID:           0,
		shardCount:        1,
	}
	c.sendMessageFunc = c.sendMessage
	return c
}

// NewWithSharding creates a new Discord channel with sharding support.
func NewWithSharding(cfg channel.DiscordConfig, logger *zap.Logger, shardID, shardCount int) *Channel {
	c := New(cfg, logger)
	c.shardID = shardID
	c.shardCount = shardCount
	c.logger = logger.With(
		zap.String("channel", "discord"),
		zap.Int("shard_id", shardID),
		zap.Int("shard_count", shardCount),
	)
	return c
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "discord"
}

// Type returns the channel type.
func (c *Channel) Type() string {
	return "discord"
}

// SetMessageHandler sets the handler for processing messages.
func (c *Channel) SetMessageHandler(handler MessageHandler) {
	c.messageHandler = handler
}

// SetSessionManager sets the bot session manager for monitoring.
func (c *Channel) SetSessionManager(manager *channel.BotSessionManager) {
	c.sessionManager = manager
}

// Start initializes and starts the Discord bot.
func (c *Channel) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.status == channel.StatusConnected || c.status == channel.StatusConnecting {
		c.mu.Unlock()
		return fmt.Errorf("channel already started")
	}
	c.status = channel.StatusConnecting
	c.mu.Unlock()

	c.ctx, c.cancel = context.WithCancel(ctx)

	// Create Discord session
	session, err := discordgo.New("Bot " + c.config.BotToken)
	if err != nil {
		c.setError(fmt.Sprintf("failed to create session: %v", err))
		return fmt.Errorf("failed to create Discord session: %w", err)
	}

	c.session = session

	// Configure sharding
	if c.shardCount > 1 {
		session.ShardID = c.shardID
		session.ShardCount = c.shardCount
		c.logger.Info("sharding enabled",
			zap.Int("shard_id", c.shardID),
			zap.Int("shard_count", c.shardCount))
	}

	// Set intents
	session.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent |
		discordgo.IntentsGuilds

	// Add message handler
	session.AddHandler(c.handleMessage)

	// Add interaction handler for slash commands and components
	session.AddHandler(c.handleInteraction)

	// Add ready handler
	session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		c.logger.Info("discord bot ready",
			zap.String("username", r.User.Username),
			zap.String("discriminator", r.User.Discriminator),
			zap.Int("guilds", len(r.Guilds)))

		// Register slash commands after ready
		c.registerSlashCommands(s)
	})

	// Open connection
	if err := session.Open(); err != nil {
		c.setError(fmt.Sprintf("failed to open connection: %v", err))
		return fmt.Errorf("failed to open Discord connection: %w", err)
	}

	now := time.Now()
	c.mu.Lock()
	c.status = channel.StatusConnected
	c.connectedAt = &now
	c.lastError = ""
	c.lastErrorAt = nil
	c.mu.Unlock()

	c.logger.Info("discord channel started")
	return nil
}

// handleMessage handles incoming Discord messages.
func (c *Channel) handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Check if guild is allowed
	if m.GuildID != "" && !c.isGuildAllowed(m.GuildID) {
		c.logger.Debug("ignoring message from non-allowed guild",
			zap.String("guild_id", m.GuildID))
		return
	}

	// Check if user is allowed
	if !c.isUserAllowed(m.Author.ID) {
		c.logger.Debug("ignoring message from non-allowed user",
			zap.String("user_id", m.Author.ID),
			zap.String("username", m.Author.Username))
		return
	}

	// Convert to unified message format
	channelMsg := c.convertMessage(m)
	c.msgCount.Add(1)
	c.msgsReceived.Add(1)
	now := time.Now()
	c.mu.Lock()
	c.lastMessageAt = &now
	c.mu.Unlock()

	chatID := m.ChannelID
	userID := m.Author.ID

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

// convertMessage converts a Discord message to the unified format.
func (c *Channel) convertMessage(m *discordgo.MessageCreate) channel.Message {
	authorID := ""
	authorName := ""
	if m.Author != nil {
		authorID = m.Author.ID
		authorName = m.Author.Username
	}

	metadata := map[string]interface{}{
		"guild_id":   m.GuildID,
		"channel_id": m.ChannelID,
	}
	if m.MentionEveryone {
		metadata["mention_everyone"] = true
	}
	if mentions, mentionIDs := discordMentionsMetadata(m.Mentions); len(mentions) > 0 {
		metadata["mentions"] = mentions
		metadata["mention_ids"] = mentionIDs
	}
	if embeds := discordEmbedsMetadata(m.Embeds); len(embeds) > 0 {
		metadata["embeds"] = embeds
	}
	if components := discordComponentsMetadata(m.Components); len(components) > 0 {
		metadata["components"] = components
	}

	channelMsg := channel.Message{
		ID:          m.ID,
		ChannelName: "discord",
		ChatID:      m.ChannelID,
		UserID:      authorID,
		Username:    authorName,
		Type:        channel.MessageTypeText,
		Content:     m.Content,
		Timestamp:   m.Timestamp,
		IsGroup:     m.GuildID != "",
		Metadata:    metadata,
	}

	if m.GuildID != "" && c.session != nil {
		guild, err := c.session.Guild(m.GuildID)
		if err == nil {
			channelMsg.GroupName = guild.Name
		}
	}

	if m.ReferencedMessage != nil {
		channelMsg.ReplyToID = m.ReferencedMessage.ID
	} else if m.MessageReference != nil && strings.TrimSpace(m.MessageReference.MessageID) != "" {
		channelMsg.ReplyToID = strings.TrimSpace(m.MessageReference.MessageID)
	}
	if strings.TrimSpace(channelMsg.ReplyToID) != "" {
		channelMsg.Metadata["reference_message_id"] = channelMsg.ReplyToID
	}

	for _, att := range m.Attachments {
		msgType := channel.MessageTypeFile
		if strings.HasPrefix(att.ContentType, "image/") {
			msgType = channel.MessageTypeImage
		} else if strings.HasPrefix(att.ContentType, "audio/") {
			msgType = channel.MessageTypeAudio
		} else if strings.HasPrefix(att.ContentType, "video/") {
			msgType = channel.MessageTypeVideo
		}

		channelMsg.Attachments = append(channelMsg.Attachments, channel.Attachment{
			ID:       att.ID,
			Type:     msgType,
			Name:     att.Filename,
			URL:      att.URL,
			Size:     int64(att.Size),
			MimeType: att.ContentType,
		})
	}

	if len(channelMsg.Attachments) > 0 && channelMsg.Content == "" {
		channelMsg.Type = channelMsg.Attachments[0].Type
	} else if len(m.Embeds) > 0 && strings.TrimSpace(channelMsg.Content) == "" {
		channelMsg.Type = channel.MessageTypeCard
	}

	return channelMsg
}

func discordMentionsMetadata(mentions []*discordgo.User) ([]map[string]interface{}, []string) {
	if len(mentions) == 0 {
		return nil, nil
	}
	items := make([]map[string]interface{}, 0, len(mentions))
	ids := make([]string, 0, len(mentions))
	seen := make(map[string]struct{}, len(mentions))
	for _, mention := range mentions {
		if mention == nil {
			continue
		}
		mentionID := strings.TrimSpace(mention.ID)
		if mentionID == "" {
			continue
		}
		if _, exists := seen[mentionID]; exists {
			continue
		}
		seen[mentionID] = struct{}{}
		item := map[string]interface{}{"id": mentionID}
		if strings.TrimSpace(mention.Username) != "" {
			item["username"] = strings.TrimSpace(mention.Username)
		}
		if strings.TrimSpace(mention.GlobalName) != "" {
			item["global_name"] = strings.TrimSpace(mention.GlobalName)
		}
		items = append(items, item)
		ids = append(ids, mentionID)
	}
	return items, ids
}

func discordEmbedsMetadata(embeds []*discordgo.MessageEmbed) []map[string]interface{} {
	if len(embeds) == 0 {
		return nil
	}
	items := make([]map[string]interface{}, 0, len(embeds))
	for _, embed := range embeds {
		if embed == nil {
			continue
		}
		item := map[string]interface{}{}
		if strings.TrimSpace(embed.Title) != "" {
			item["title"] = strings.TrimSpace(embed.Title)
		}
		if strings.TrimSpace(embed.Description) != "" {
			item["description"] = strings.TrimSpace(embed.Description)
		}
		if strings.TrimSpace(embed.URL) != "" {
			item["url"] = strings.TrimSpace(embed.URL)
		}
		if strings.TrimSpace(string(embed.Type)) != "" {
			item["type"] = strings.TrimSpace(string(embed.Type))
		}
		if embed.Author != nil && strings.TrimSpace(embed.Author.Name) != "" {
			item["author"] = strings.TrimSpace(embed.Author.Name)
		}
		if len(embed.Fields) > 0 {
			item["field_count"] = len(embed.Fields)
		}
		if len(item) > 0 {
			items = append(items, item)
		}
	}
	return items
}

func discordComponentsMetadata(components []discordgo.MessageComponent) []map[string]interface{} {
	if len(components) == 0 {
		return nil
	}
	items := make([]map[string]interface{}, 0, len(components))
	for _, component := range components {
		item := discordComponentMetadata(component)
		if len(item) > 0 {
			items = append(items, item)
		}
	}
	return items
}

func discordComponentMetadata(component discordgo.MessageComponent) map[string]interface{} {
	switch cmp := component.(type) {
	case discordgo.ActionsRow:
		return map[string]interface{}{"type": int(cmp.Type()), "components": discordComponentsMetadata(cmp.Components)}
	case *discordgo.ActionsRow:
		if cmp == nil {
			return nil
		}
		return map[string]interface{}{"type": int(cmp.Type()), "components": discordComponentsMetadata(cmp.Components)}
	case discordgo.Button:
		return discordButtonMetadata(cmp)
	case *discordgo.Button:
		if cmp == nil {
			return nil
		}
		return discordButtonMetadata(*cmp)
	case discordgo.SelectMenu:
		return discordSelectMenuMetadata(cmp)
	case *discordgo.SelectMenu:
		if cmp == nil {
			return nil
		}
		return discordSelectMenuMetadata(*cmp)
	case discordgo.TextInput:
		return discordTextInputMetadata(cmp)
	case *discordgo.TextInput:
		if cmp == nil {
			return nil
		}
		return discordTextInputMetadata(*cmp)
	default:
		if component == nil {
			return nil
		}
		return map[string]interface{}{"type": int(component.Type())}
	}
}

func discordButtonMetadata(button discordgo.Button) map[string]interface{} {
	item := map[string]interface{}{
		"type":  int(button.Type()),
		"label": button.Label,
		"style": int(button.Style),
	}
	if strings.TrimSpace(button.CustomID) != "" {
		item["custom_id"] = strings.TrimSpace(button.CustomID)
	}
	if strings.TrimSpace(button.URL) != "" {
		item["url"] = strings.TrimSpace(button.URL)
	}
	if button.Disabled {
		item["disabled"] = true
	}
	return item
}

func discordSelectMenuMetadata(menu discordgo.SelectMenu) map[string]interface{} {
	item := map[string]interface{}{
		"type":         int(menu.Type()),
		"custom_id":    menu.CustomID,
		"option_count": len(menu.Options),
	}
	if strings.TrimSpace(menu.Placeholder) != "" {
		item["placeholder"] = strings.TrimSpace(menu.Placeholder)
	}
	if menu.MinValues != nil {
		item["min_values"] = *menu.MinValues
	}
	if menu.MaxValues > 0 {
		item["max_values"] = menu.MaxValues
	}
	if menu.Disabled {
		item["disabled"] = true
	}
	return item
}

func discordTextInputMetadata(input discordgo.TextInput) map[string]interface{} {
	item := map[string]interface{}{
		"type":      int(input.Type()),
		"custom_id": input.CustomID,
		"label":     input.Label,
	}
	if strings.TrimSpace(input.Placeholder) != "" {
		item["placeholder"] = strings.TrimSpace(input.Placeholder)
	}
	if input.Required {
		item["required"] = true
	}
	return item
}

// isGuildAllowed checks if a guild is allowed.
func (c *Channel) isGuildAllowed(guildID string) bool {
	if len(c.config.AllowedGuilds) == 0 {
		return true
	}

	for _, allowed := range c.config.AllowedGuilds {
		if allowed == guildID {
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

	if c.session != nil {
		if err := c.session.Close(); err != nil {
			c.logger.Warn("error closing Discord session", zap.Error(err))
		}
	}

	close(c.messages)
	c.logger.Info("discord channel stopped")
	return nil
}

// Send sends a message through Discord.
func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	if c.session == nil {
		return fmt.Errorf("session not initialized")
	}
	if c.sendMessageFunc == nil {
		c.sendMessageFunc = c.sendMessage
	}

	// Build message send data
	data := &discordgo.MessageSend{
		Content: msg.Content,
	}

	// Set reply reference
	if msg.ReplyToID != "" {
		data.Reference = &discordgo.MessageReference{
			MessageID: msg.ReplyToID,
			ChannelID: msg.ChatID,
		}
	}

	// Handle rich message metadata
	if msg.Metadata != nil {
		if embeds, ok := msg.Metadata["embeds"].([]*discordgo.MessageEmbed); ok {
			data.Embeds = embeds
		}
		if rows, ok := msg.Metadata["components"].([]ActionRow); ok {
			data.Components = c.buildComponents(rows)
		} else if components, ok := msg.Metadata["components"].([]discordgo.MessageComponent); ok {
			data.Components = components
		}
		if allowedMentions, ok := msg.Metadata["allowed_mentions"].(*discordgo.MessageAllowedMentions); ok {
			data.AllowedMentions = allowedMentions
		} else if allowedMentions, ok := msg.Metadata["allowed_mentions"].(discordgo.MessageAllowedMentions); ok {
			data.AllowedMentions = &allowedMentions
		}
		if tts, ok := msg.Metadata["tts"].(bool); ok {
			data.TTS = tts
		}
	}

	var fallbackParts []string
	for _, att := range msg.Attachments {
		if len(att.Data) > 0 {
			name := att.Name
			if name == "" {
				name = "file"
			}
			data.Files = append(data.Files, &discordgo.File{
				Name:        name,
				ContentType: att.MimeType,
				Reader:      bytes.NewReader(att.Data),
			})
			continue
		}
		if fallback := discordAttachmentFallbackText(att); fallback != "" {
			fallbackParts = append(fallbackParts, fallback)
		}
	}
	if len(fallbackParts) > 0 {
		if data.Content != "" {
			data.Content += "\n" + strings.Join(fallbackParts, "\n")
		} else {
			data.Content = strings.Join(fallbackParts, "\n")
		}
	}

	if data.Content == "" && len(data.Files) == 0 && len(data.Embeds) == 0 && len(data.Components) == 0 {
		return fmt.Errorf("no sendable Discord content")
	}

	_, err := c.sendMessageFunc(msg.ChatID, data)
	if err != nil {
		c.logger.Error("failed to send message",
			zap.String("channel_id", msg.ChatID),
			zap.Error(err))
		return fmt.Errorf("failed to send message: %w", err)
	}

	c.msgsSent.Add(1)
	now := time.Now()
	c.mu.Lock()
	c.lastReplyAt = &now
	c.mu.Unlock()

	return nil
}

func (c *Channel) sendMessage(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
	if c.session == nil {
		return nil, fmt.Errorf("session not initialized")
	}
	return c.session.ChannelMessageSendComplex(channelID, data)
}

func discordAttachmentFallbackText(att channel.Attachment) string {
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

// SendTyping sends a typing indicator to the given Discord channel.
func (c *Channel) SendTyping(_ context.Context, chatID string) error {
	if c.session == nil {
		return fmt.Errorf("session not initialized")
	}
	return c.session.ChannelTyping(chatID)
}

// SendStreaming sends a message with streaming support.
func (c *Channel) SendStreaming(ctx context.Context, chatID string, replyToID string, content <-chan string, done chan<- struct{}) error {
	defer close(done)

	if c.session == nil {
		return fmt.Errorf("session not initialized")
	}

	var fullContent strings.Builder
	var sentMsgID string
	lastUpdate := time.Now()
	updateInterval := 500 * time.Millisecond

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk, ok := <-content:
			if !ok {
				// Channel closed, send final message
				if sentMsgID != "" && fullContent.Len() > 0 {
					c.session.ChannelMessageEdit(chatID, sentMsgID, fullContent.String())
				}
				return nil
			}

			fullContent.WriteString(chunk)

			// Update message periodically to avoid rate limiting
			if time.Since(lastUpdate) >= updateInterval {
				if sentMsgID == "" {
					// Send initial message
					data := &discordgo.MessageSend{
						Content: fullContent.String(),
					}
					if replyToID != "" {
						data.Reference = &discordgo.MessageReference{
							MessageID: replyToID,
							ChannelID: chatID,
						}
					}
					sent, err := c.session.ChannelMessageSendComplex(chatID, data)
					if err != nil {
						c.logger.Error("failed to send streaming message", zap.Error(err))
						continue
					}
					sentMsgID = sent.ID
				} else {
					// Edit existing message
					_, err := c.session.ChannelMessageEdit(chatID, sentMsgID, fullContent.String())
					if err != nil {
						c.logger.Warn("failed to edit streaming message", zap.Error(err))
					}
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
		Name:             "discord",
		Type:             "discord",
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

	if c.session != nil && c.session.State != nil && c.session.State.User != nil {
		info.Metadata["bot_username"] = c.session.State.User.Username
		info.Metadata["bot_id"] = c.session.State.User.ID
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

// RegisterSlashCommand registers a slash command.
func (c *Channel) RegisterSlashCommand(cmd *SlashCommand) {
	c.commandMu.Lock()
	defer c.commandMu.Unlock()
	c.slashCommands[cmd.Name] = cmd
}

// RegisterComponentHandler registers a handler for component interactions.
func (c *Channel) RegisterComponentHandler(customID string, handler ComponentHandler) {
	c.commandMu.Lock()
	defer c.commandMu.Unlock()
	c.componentHandlers[customID] = handler
}

// registerSlashCommands registers all slash commands with Discord.
func (c *Channel) registerSlashCommands(s *discordgo.Session) {
	c.commandMu.RLock()
	commands := make([]*SlashCommand, 0, len(c.slashCommands))
	for _, cmd := range c.slashCommands {
		commands = append(commands, cmd)
	}
	c.commandMu.RUnlock()

	if len(commands) == 0 {
		return
	}

	// Register commands globally or per guild
	for _, cmd := range commands {
		appCmd := &discordgo.ApplicationCommand{
			Name:        cmd.Name,
			Description: cmd.Description,
			Options:     cmd.Options,
		}

		var registeredCmd *discordgo.ApplicationCommand
		var err error

		if c.config.ApplicationID != "" {
			// Register globally
			registeredCmd, err = s.ApplicationCommandCreate(c.config.ApplicationID, "", appCmd)
		} else {
			// Register for each allowed guild
			for _, guildID := range c.config.AllowedGuilds {
				registeredCmd, err = s.ApplicationCommandCreate(s.State.User.ID, guildID, appCmd)
				if err != nil {
					c.logger.Error("failed to register slash command for guild",
						zap.String("command", cmd.Name),
						zap.String("guild_id", guildID),
						zap.Error(err))
				}
			}
		}

		if err != nil {
			c.logger.Error("failed to register slash command",
				zap.String("command", cmd.Name),
				zap.Error(err))
		} else if registeredCmd != nil {
			c.registeredCmdIDs = append(c.registeredCmdIDs, registeredCmd.ID)
			c.logger.Info("registered slash command",
				zap.String("command", cmd.Name),
				zap.String("id", registeredCmd.ID))
		}
	}
}

// handleInteraction handles Discord interactions (slash commands, buttons, select menus).
func (c *Channel) handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		c.handleSlashCommand(s, i)
	case discordgo.InteractionMessageComponent:
		c.handleComponentInteraction(s, i)
	}
}

// handleSlashCommand handles slash command interactions.
func (c *Channel) handleSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	cmdName := i.ApplicationCommandData().Name

	c.commandMu.RLock()
	cmd, exists := c.slashCommands[cmdName]
	c.commandMu.RUnlock()

	if !exists {
		c.logger.Warn("unknown slash command", zap.String("command", cmdName))
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Unknown command",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	if err := cmd.Handler(c.ctx, s, i); err != nil {
		c.logger.Error("slash command handler error",
			zap.String("command", cmdName),
			zap.Error(err))
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "An error occurred while processing your command",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}
}

// handleComponentInteraction handles button and select menu interactions.
func (c *Channel) handleComponentInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	customID := i.MessageComponentData().CustomID

	c.commandMu.RLock()
	handler, exists := c.componentHandlers[customID]
	c.commandMu.RUnlock()

	// Try prefix matching if exact match not found
	if !exists {
		c.commandMu.RLock()
		for prefix, h := range c.componentHandlers {
			if strings.HasPrefix(customID, prefix) {
				handler = h
				exists = true
				break
			}
		}
		c.commandMu.RUnlock()
	}

	if !exists {
		// Send to message channel for AI handling
		channelMsg := channel.Message{
			ID:          fmt.Sprintf("comp_%s", i.ID),
			ChannelName: "discord",
			ChatID:      i.ChannelID,
			UserID:      i.Member.User.ID,
			Username:    i.Member.User.Username,
			Type:        channel.MessageTypeText,
			Content:     customID,
			Timestamp:   time.Now(),
			IsGroup:     i.GuildID != "",
			Metadata: map[string]interface{}{
				"is_component":   true,
				"component_type": i.MessageComponentData().ComponentType,
				"custom_id":      customID,
				"values":         i.MessageComponentData().Values,
			},
		}

		select {
		case c.messages <- channelMsg:
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredMessageUpdate,
			})
		default:
			c.logger.Warn("message channel full, dropping component interaction")
		}
		return
	}

	if err := handler(c.ctx, s, i); err != nil {
		c.logger.Error("component handler error",
			zap.String("custom_id", customID),
			zap.Error(err))
	}
}

// SendWithComponents sends a message with buttons or select menus.
func (c *Channel) SendWithComponents(ctx context.Context, channelID string, content string, components []ActionRow, embeds []*discordgo.MessageEmbed) (*discordgo.Message, error) {
	if c.session == nil {
		return nil, fmt.Errorf("session not initialized")
	}

	data := &discordgo.MessageSend{
		Content: content,
		Embeds:  embeds,
	}

	// Convert ActionRows to Discord components
	if len(components) > 0 {
		data.Components = c.buildComponents(components)
	}

	msg, err := c.session.ChannelMessageSendComplex(channelID, data)
	if err != nil {
		c.logger.Error("failed to send message with components",
			zap.String("channel_id", channelID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	return msg, nil
}

// buildComponents converts ActionRows to Discord message components.
func (c *Channel) buildComponents(rows []ActionRow) []discordgo.MessageComponent {
	var components []discordgo.MessageComponent

	for _, row := range rows {
		var rowComponents []discordgo.MessageComponent

		for _, comp := range row.Components {
			switch comp.Type {
			case discordgo.ButtonComponent:
				btn := discordgo.Button{
					Label:    comp.Label,
					Style:    comp.Style,
					CustomID: comp.CustomID,
					URL:      comp.URL,
					Disabled: comp.Disabled,
					Emoji:    comp.Emoji,
				}
				rowComponents = append(rowComponents, btn)

			case discordgo.SelectMenuComponent:
				menu := discordgo.SelectMenu{
					CustomID:    comp.CustomID,
					Placeholder: comp.Placeholder,
					Options:     comp.Options,
					MinValues:   comp.MinValues,
					MaxValues:   comp.MaxValues,
					Disabled:    comp.Disabled,
				}
				rowComponents = append(rowComponents, menu)
			}
		}

		if len(rowComponents) > 0 {
			components = append(components, discordgo.ActionsRow{
				Components: rowComponents,
			})
		}
	}

	return components
}

// EditMessageWithComponents edits a message with new components.
func (c *Channel) EditMessageWithComponents(ctx context.Context, channelID, messageID string, content string, components []ActionRow, embeds []*discordgo.MessageEmbed) error {
	if c.session == nil {
		return fmt.Errorf("session not initialized")
	}

	edit := &discordgo.MessageEdit{
		Channel: channelID,
		ID:      messageID,
		Content: &content,
		Embeds:  &embeds,
	}

	if len(components) > 0 {
		comps := c.buildComponents(components)
		edit.Components = &comps
	}

	_, err := c.session.ChannelMessageEditComplex(edit)
	if err != nil {
		c.logger.Error("failed to edit message with components",
			zap.String("channel_id", channelID),
			zap.String("message_id", messageID),
			zap.Error(err))
		return fmt.Errorf("failed to edit message: %w", err)
	}

	return nil
}

// RespondToInteraction sends a response to an interaction.
func (c *Channel) RespondToInteraction(i *discordgo.InteractionCreate, content string, components []ActionRow, embeds []*discordgo.MessageEmbed, ephemeral bool) error {
	if c.session == nil {
		return fmt.Errorf("session not initialized")
	}

	data := &discordgo.InteractionResponseData{
		Content: content,
		Embeds:  embeds,
	}

	if ephemeral {
		data.Flags = discordgo.MessageFlagsEphemeral
	}

	if len(components) > 0 {
		data.Components = c.buildComponents(components)
	}

	return c.session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: data,
	})
}

// UpdateInteractionResponse updates the original interaction response.
func (c *Channel) UpdateInteractionResponse(i *discordgo.InteractionCreate, content string, components []ActionRow, embeds []*discordgo.MessageEmbed) error {
	if c.session == nil {
		return fmt.Errorf("session not initialized")
	}

	edit := &discordgo.WebhookEdit{
		Content: &content,
		Embeds:  &embeds,
	}

	if len(components) > 0 {
		comps := c.buildComponents(components)
		edit.Components = &comps
	}

	_, err := c.session.InteractionResponseEdit(i.Interaction, edit)
	return err
}

// GetShardInfo returns the shard information.
func (c *Channel) GetShardInfo() (shardID, shardCount int) {
	return c.shardID, c.shardCount
}
