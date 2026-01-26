// Package discord provides a Discord bot channel implementation.
package discord

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel"
)

// Channel implements the channel.Channel interface for Discord.
type Channel struct {
	config   channel.DiscordConfig
	logger   *zap.Logger
	session  *discordgo.Session
	messages chan channel.Message

	mu          sync.RWMutex
	status      channel.Status
	connectedAt *time.Time
	lastError   string
	lastErrorAt *time.Time
	msgCount    atomic.Int64

	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new Discord channel.
func New(cfg channel.DiscordConfig, logger *zap.Logger) *Channel {
	return &Channel{
		config:   cfg,
		logger:   logger.With(zap.String("channel", "discord")),
		messages: make(chan channel.Message, 100),
		status:   channel.StatusDisconnected,
	}
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "discord"
}

// Type returns the channel type.
func (c *Channel) Type() string {
	return "discord"
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

	// Set intents
	session.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent |
		discordgo.IntentsGuilds

	// Add message handler
	session.AddHandler(c.handleMessage)

	// Add ready handler
	session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		c.logger.Info("discord bot ready",
			zap.String("username", r.User.Username),
			zap.String("discriminator", r.User.Discriminator))
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

	select {
	case c.messages <- channelMsg:
	default:
		c.logger.Warn("message channel full, dropping message",
			zap.String("message_id", channelMsg.ID))
	}
}

// convertMessage converts a Discord message to the unified format.
func (c *Channel) convertMessage(m *discordgo.MessageCreate) channel.Message {
	channelMsg := channel.Message{
		ID:          m.ID,
		ChannelName: "discord",
		ChatID:      m.ChannelID,
		UserID:      m.Author.ID,
		Username:    m.Author.Username,
		Type:        channel.MessageTypeText,
		Content:     m.Content,
		Timestamp:   m.Timestamp,
		IsGroup:     m.GuildID != "",
		Metadata: map[string]interface{}{
			"guild_id":   m.GuildID,
			"channel_id": m.ChannelID,
		},
	}

	// Get guild name if in a guild
	if m.GuildID != "" {
		guild, err := c.session.Guild(m.GuildID)
		if err == nil {
			channelMsg.GroupName = guild.Name
		}
	}

	// Handle reply
	if m.ReferencedMessage != nil {
		channelMsg.ReplyToID = m.ReferencedMessage.ID
	}

	// Handle attachments
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
	}

	return channelMsg
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

	// Handle embeds from metadata
	if msg.Metadata != nil {
		if embeds, ok := msg.Metadata["embeds"].([]*discordgo.MessageEmbed); ok {
			data.Embeds = embeds
		}
	}

	_, err := c.session.ChannelMessageSendComplex(msg.ChatID, data)
	if err != nil {
		c.logger.Error("failed to send message",
			zap.String("channel_id", msg.ChatID),
			zap.Error(err))
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
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
		Name:         "discord",
		Type:         "discord",
		Status:       c.status,
		Enabled:      c.config.Enabled,
		ConnectedAt:  c.connectedAt,
		LastError:    c.lastError,
		LastErrorAt:  c.lastErrorAt,
		MessageCount: c.msgCount.Load(),
		Metadata:     make(map[string]interface{}),
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
