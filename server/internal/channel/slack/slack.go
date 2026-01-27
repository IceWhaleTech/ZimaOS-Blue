// Package slack provides a Slack bot channel implementation.
package slack

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel"
)

// Channel implements the channel.Channel interface for Slack.
type Channel struct {
	config       channel.SlackConfig
	logger       *zap.Logger
	client       *slack.Client
	socketClient *socketmode.Client
	messages     chan channel.Message

	mu          sync.RWMutex
	status      channel.Status
	connectedAt *time.Time
	lastError   string
	lastErrorAt *time.Time
	msgCount    atomic.Int64
	botUserID   string

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a new Slack channel.
func New(cfg channel.SlackConfig, logger *zap.Logger) *Channel {
	return &Channel{
		config:   cfg,
		logger:   logger.With(zap.String("channel", "slack")),
		messages: make(chan channel.Message, 100),
		status:   channel.StatusDisconnected,
	}
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "slack"
}

// Type returns the channel type.
func (c *Channel) Type() string {
	return "slack"
}

// Start initializes and starts the Slack bot.
func (c *Channel) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.status == channel.StatusConnected || c.status == channel.StatusConnecting {
		c.mu.Unlock()
		return fmt.Errorf("channel already started")
	}
	c.status = channel.StatusConnecting
	c.mu.Unlock()

	c.ctx, c.cancel = context.WithCancel(ctx)

	// Create Slack client
	c.client = slack.New(
		c.config.BotToken,
		slack.OptionAppLevelToken(c.config.AppToken),
	)

	// Test authentication
	authTest, err := c.client.AuthTest()
	if err != nil {
		c.setError(fmt.Sprintf("auth test failed: %v", err))
		return fmt.Errorf("Slack auth test failed: %w", err)
	}
	c.botUserID = authTest.UserID
	c.logger.Info("slack bot authenticated",
		zap.String("user_id", authTest.UserID),
		zap.String("team", authTest.Team))

	// Create socket mode client
	c.socketClient = socketmode.New(
		c.client,
		socketmode.OptionDebug(false),
	)

	// Start event handler
	c.wg.Add(1)
	go c.handleEvents()

	// Start socket mode client
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		if err := c.socketClient.RunContext(c.ctx); err != nil {
			if c.ctx.Err() == nil {
				c.logger.Error("socket mode client error", zap.Error(err))
				c.setError(fmt.Sprintf("socket mode error: %v", err))
			}
		}
	}()

	now := time.Now()
	c.mu.Lock()
	c.status = channel.StatusConnected
	c.connectedAt = &now
	c.lastError = ""
	c.lastErrorAt = nil
	c.mu.Unlock()

	c.logger.Info("slack channel started")
	return nil
}

// handleEvents handles incoming Slack events.
func (c *Channel) handleEvents() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		case evt, ok := <-c.socketClient.Events:
			if !ok {
				return
			}
			c.processEvent(evt)
		}
	}
}

// processEvent processes a single Slack event.
func (c *Channel) processEvent(evt socketmode.Event) {
	switch evt.Type {
	case socketmode.EventTypeEventsAPI:
		eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
		if !ok {
			return
		}
		c.socketClient.Ack(*evt.Request)
		c.handleEventsAPIEvent(eventsAPIEvent)

	case socketmode.EventTypeSlashCommand:
		cmd, ok := evt.Data.(slack.SlashCommand)
		if !ok {
			return
		}
		c.socketClient.Ack(*evt.Request)
		c.handleSlashCommand(cmd)
	}
}

// handleEventsAPIEvent handles Events API events.
func (c *Channel) handleEventsAPIEvent(event slackevents.EventsAPIEvent) {
	switch event.Type {
	case slackevents.CallbackEvent:
		innerEvent := event.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.MessageEvent:
			c.handleMessageEvent(ev)
		case *slackevents.AppMentionEvent:
			c.handleAppMentionEvent(ev)
		}
	}
}

// handleMessageEvent handles message events.
func (c *Channel) handleMessageEvent(ev *slackevents.MessageEvent) {
	// Ignore bot messages
	if ev.BotID != "" || ev.User == c.botUserID {
		return
	}

	// Check if channel is allowed
	if !c.isChannelAllowed(ev.Channel) {
		return
	}

	// Check if user is allowed
	if !c.isUserAllowed(ev.User) {
		return
	}

	channelMsg := c.convertMessageEvent(ev)
	c.msgCount.Add(1)

	select {
	case c.messages <- channelMsg:
	default:
		c.logger.Warn("message channel full, dropping message",
			zap.String("message_id", channelMsg.ID))
	}
}

// handleAppMentionEvent handles app mention events.
func (c *Channel) handleAppMentionEvent(ev *slackevents.AppMentionEvent) {
	// Check if channel is allowed
	if !c.isChannelAllowed(ev.Channel) {
		return
	}

	// Check if user is allowed
	if !c.isUserAllowed(ev.User) {
		return
	}

	channelMsg := channel.Message{
		ID:          ev.TimeStamp,
		ChannelName: "slack",
		ChatID:      ev.Channel,
		UserID:      ev.User,
		Type:        channel.MessageTypeText,
		Content:     ev.Text,
		Timestamp:   parseSlackTimestamp(ev.TimeStamp),
		IsGroup:     true,
		Metadata: map[string]interface{}{
			"event_type": "app_mention",
			"thread_ts":  ev.ThreadTimeStamp,
		},
	}

	c.msgCount.Add(1)

	select {
	case c.messages <- channelMsg:
	default:
		c.logger.Warn("message channel full, dropping message",
			zap.String("message_id", channelMsg.ID))
	}
}

// handleSlashCommand handles slash commands.
func (c *Channel) handleSlashCommand(cmd slack.SlashCommand) {
	// Check if channel is allowed
	if !c.isChannelAllowed(cmd.ChannelID) {
		return
	}

	// Check if user is allowed
	if !c.isUserAllowed(cmd.UserID) {
		return
	}

	channelMsg := channel.Message{
		ID:          cmd.TriggerID,
		ChannelName: "slack",
		ChatID:      cmd.ChannelID,
		UserID:      cmd.UserID,
		Username:    cmd.UserName,
		Type:        channel.MessageTypeText,
		Content:     cmd.Command + " " + cmd.Text,
		Timestamp:   time.Now(),
		IsGroup:     true,
		Metadata: map[string]interface{}{
			"event_type":  "slash_command",
			"command":     cmd.Command,
			"response_url": cmd.ResponseURL,
		},
	}

	c.msgCount.Add(1)

	select {
	case c.messages <- channelMsg:
	default:
		c.logger.Warn("message channel full, dropping message",
			zap.String("message_id", channelMsg.ID))
	}
}

// convertMessageEvent converts a Slack message event to the unified format.
func (c *Channel) convertMessageEvent(ev *slackevents.MessageEvent) channel.Message {
	// Get user info
	username := ev.User
	userInfo, err := c.client.GetUserInfo(ev.User)
	if err == nil {
		username = userInfo.Name
	}

	channelMsg := channel.Message{
		ID:          ev.TimeStamp,
		ChannelName: "slack",
		ChatID:      ev.Channel,
		UserID:      ev.User,
		Username:    username,
		Type:        channel.MessageTypeText,
		Content:     ev.Text,
		Timestamp:   parseSlackTimestamp(ev.TimeStamp),
		IsGroup:     strings.HasPrefix(ev.Channel, "C") || strings.HasPrefix(ev.Channel, "G"),
		Metadata: map[string]interface{}{
			"thread_ts":    ev.ThreadTimeStamp,
			"channel_type": ev.ChannelType,
		},
	}

	// Handle thread reply
	if ev.ThreadTimeStamp != "" {
		channelMsg.ReplyToID = ev.ThreadTimeStamp
	}

	return channelMsg
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
	c.logger.Info("slack channel stopped")
	return nil
}

// Send sends a message through Slack.
func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	if c.client == nil {
		return fmt.Errorf("client not initialized")
	}

	options := []slack.MsgOption{
		slack.MsgOptionText(msg.Content, false),
	}

	// Handle thread reply
	if msg.ReplyToID != "" {
		options = append(options, slack.MsgOptionTS(msg.ReplyToID))
	}

	// Handle blocks from metadata
	if msg.Metadata != nil {
		if blocks, ok := msg.Metadata["blocks"].([]slack.Block); ok {
			options = append(options, slack.MsgOptionBlocks(blocks...))
		}
	}

	_, _, err := c.client.PostMessageContext(ctx, msg.ChatID, options...)
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

	if c.client == nil {
		return fmt.Errorf("client not initialized")
	}

	var fullContent strings.Builder
	var sentMsgTS string
	lastUpdate := time.Now()
	updateInterval := 500 * time.Millisecond

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk, ok := <-content:
			if !ok {
				// Channel closed, send final message
				if sentMsgTS != "" && fullContent.Len() > 0 {
					c.client.UpdateMessageContext(ctx, chatID, sentMsgTS,
						slack.MsgOptionText(fullContent.String(), false))
				}
				return nil
			}

			fullContent.WriteString(chunk)

			// Update message periodically to avoid rate limiting
			if time.Since(lastUpdate) >= updateInterval {
				if sentMsgTS == "" {
					// Send initial message
					options := []slack.MsgOption{
						slack.MsgOptionText(fullContent.String(), false),
					}
					if replyToID != "" {
						options = append(options, slack.MsgOptionTS(replyToID))
					}
					_, ts, err := c.client.PostMessageContext(ctx, chatID, options...)
					if err != nil {
						c.logger.Error("failed to send streaming message", zap.Error(err))
						continue
					}
					sentMsgTS = ts
				} else {
					// Update existing message
					_, _, _, err := c.client.UpdateMessageContext(ctx, chatID, sentMsgTS,
						slack.MsgOptionText(fullContent.String(), false))
					if err != nil {
						c.logger.Warn("failed to update streaming message", zap.Error(err))
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
		Name:         "slack",
		Type:         "slack",
		Status:       c.status,
		Enabled:      c.config.Enabled,
		ConnectedAt:  c.connectedAt,
		LastError:    c.lastError,
		LastErrorAt:  c.lastErrorAt,
		MessageCount: c.msgCount.Load(),
		Metadata:     make(map[string]interface{}),
	}

	if c.botUserID != "" {
		info.Metadata["bot_user_id"] = c.botUserID
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

// parseSlackTimestamp parses a Slack timestamp to time.Time.
func parseSlackTimestamp(ts string) time.Time {
	parts := strings.Split(ts, ".")
	if len(parts) != 2 {
		return time.Now()
	}

	var sec int64
	fmt.Sscanf(parts[0], "%d", &sec)
	return time.Unix(sec, 0)
}
