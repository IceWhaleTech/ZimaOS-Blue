// Package matrix provides a Matrix protocol channel implementation.
package matrix

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel"
)

// Channel implements the channel.Channel interface for Matrix.
type Channel struct {
	config   channel.MatrixConfig
	logger   *zap.Logger
	client   *mautrix.Client
	messages chan channel.Message

	mu          sync.RWMutex
	status      channel.Status
	connectedAt *time.Time
	lastError   string
	lastErrorAt *time.Time
	msgCount      atomic.Int64
	msgsReceived  atomic.Int64
	msgsSent      atomic.Int64
	lastMessageAt *time.Time
	lastReplyAt   *time.Time

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a new Matrix channel.
func New(cfg channel.MatrixConfig, logger *zap.Logger) *Channel {
	return &Channel{
		config:   cfg,
		logger:   logger.With(zap.String("channel", "matrix")),
		messages: make(chan channel.Message, 100),
		status:   channel.StatusDisconnected,
	}
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "matrix"
}

// Type returns the channel type.
func (c *Channel) Type() string {
	return "matrix"
}

// Start initializes and starts the Matrix client.
func (c *Channel) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.status == channel.StatusConnected || c.status == channel.StatusConnecting {
		c.mu.Unlock()
		return fmt.Errorf("channel already started")
	}
	c.status = channel.StatusConnecting
	c.mu.Unlock()

	c.ctx, c.cancel = context.WithCancel(ctx)

	// Create Matrix client
	client, err := mautrix.NewClient(c.config.Homeserver, id.UserID(c.config.UserID), c.config.AccessToken)
	if err != nil {
		c.setError(fmt.Sprintf("failed to create client: %v", err))
		return fmt.Errorf("failed to create Matrix client: %w", err)
	}

	c.client = client

	// Set device ID if provided
	if c.config.DeviceID != "" {
		client.DeviceID = id.DeviceID(c.config.DeviceID)
	}

	// Verify credentials
	whoami, err := client.Whoami(ctx)
	if err != nil {
		c.setError(fmt.Sprintf("failed to verify credentials: %v", err))
		return fmt.Errorf("failed to verify Matrix credentials: %w", err)
	}

	c.logger.Info("matrix client authenticated",
		zap.String("user_id", whoami.UserID.String()),
		zap.String("device_id", whoami.DeviceID.String()))

	// Set up event handler
	syncer := client.Syncer.(*mautrix.DefaultSyncer)
	syncer.OnEventType(event.EventMessage, func(ctx context.Context, evt *event.Event) {
		c.handleMessageEvent(evt)
	})

	// Start sync
	c.wg.Add(1)
	go c.syncLoop()

	now := time.Now()
	c.mu.Lock()
	c.status = channel.StatusConnected
	c.connectedAt = &now
	c.lastError = ""
	c.lastErrorAt = nil
	c.mu.Unlock()

	c.logger.Info("matrix channel started")
	return nil
}

// syncLoop runs the Matrix sync loop.
func (c *Channel) syncLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			if err := c.client.SyncWithContext(c.ctx); err != nil {
				if c.ctx.Err() != nil {
					return
				}
				c.logger.Error("sync error", zap.Error(err))
				c.setError(fmt.Sprintf("sync error: %v", err))

				// Wait before retrying
				select {
				case <-c.ctx.Done():
					return
				case <-time.After(5 * time.Second):
				}
			}
		}
	}
}

// handleMessageEvent handles incoming message events.
func (c *Channel) handleMessageEvent(evt *event.Event) {
	// Ignore messages from ourselves
	if evt.Sender == id.UserID(c.config.UserID) {
		return
	}

	// Check if room is allowed
	if !c.isRoomAllowed(evt.RoomID.String()) {
		c.logger.Debug("ignoring message from non-allowed room",
			zap.String("room_id", evt.RoomID.String()))
		return
	}

	// Parse message content
	content := evt.Content.AsMessage()
	if content == nil {
		return
	}

	// Convert to unified message format
	channelMsg := c.convertMessage(evt, content)
	c.msgCount.Add(1)
	c.msgsReceived.Add(1)
	now := time.Now()
	c.mu.Lock()
	c.lastMessageAt = &now
	c.mu.Unlock()

	select {
	case c.messages <- channelMsg:
	default:
		c.logger.Warn("message channel full, dropping message",
			zap.String("message_id", channelMsg.ID))
	}
}

// convertMessage converts a Matrix message to the unified format.
func (c *Channel) convertMessage(evt *event.Event, content *event.MessageEventContent) channel.Message {
	msgType := channel.MessageTypeText
	switch content.MsgType {
	case event.MsgImage:
		msgType = channel.MessageTypeImage
	case event.MsgAudio:
		msgType = channel.MessageTypeAudio
	case event.MsgVideo:
		msgType = channel.MessageTypeVideo
	case event.MsgFile:
		msgType = channel.MessageTypeFile
	}

	channelMsg := channel.Message{
		ID:          evt.ID.String(),
		ChannelName: "matrix",
		ChatID:      evt.RoomID.String(),
		UserID:      evt.Sender.String(),
		Type:        msgType,
		Content:     content.Body,
		Timestamp:   time.UnixMilli(evt.Timestamp),
		IsGroup:     true, // Matrix rooms are always "group" chats
		Metadata: map[string]interface{}{
			"msg_type":       string(content.MsgType),
			"format":         content.Format,
			"formatted_body": content.FormattedBody,
		},
	}

	// Handle reply
	if content.RelatesTo != nil && content.RelatesTo.InReplyTo != nil {
		channelMsg.ReplyToID = content.RelatesTo.InReplyTo.EventID.String()
	}

	// Handle attachments
	if content.URL != "" {
		channelMsg.Attachments = append(channelMsg.Attachments, channel.Attachment{
			ID:       string(content.URL),
			Type:     msgType,
			Name:     content.Body,
			URL:      string(content.URL),
			Size:     int64(content.Info.Size),
			MimeType: content.Info.MimeType,
		})
	}

	return channelMsg
}

// isRoomAllowed checks if a room is allowed.
func (c *Channel) isRoomAllowed(roomID string) bool {
	if len(c.config.AllowedRooms) == 0 {
		return true
	}

	for _, allowed := range c.config.AllowedRooms {
		if allowed == roomID {
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

	// Wait for sync loop to finish
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
	c.logger.Info("matrix channel stopped")
	return nil
}

// Send sends a message through Matrix.
func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	if c.client == nil {
		return fmt.Errorf("client not initialized")
	}

	roomID := id.RoomID(msg.ChatID)

	// Build message content
	content := &event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    msg.Content,
	}

	// Handle formatted content
	if msg.Format == "html" {
		content.Format = event.FormatHTML
		content.FormattedBody = msg.Content
	} else if msg.Format == "markdown" {
		// Convert markdown to HTML (simplified)
		content.Format = event.FormatHTML
		content.FormattedBody = markdownToHTML(msg.Content)
	}

	// Handle reply
	if msg.ReplyToID != "" {
		content.RelatesTo = &event.RelatesTo{
			InReplyTo: &event.InReplyTo{
				EventID: id.EventID(msg.ReplyToID),
			},
		}
	}

	_, err := c.client.SendMessageEvent(ctx, roomID, event.EventMessage, content)
	if err != nil {
		c.logger.Error("failed to send message",
			zap.String("room_id", msg.ChatID),
			zap.Error(err))
		return fmt.Errorf("failed to send message: %w", err)
	}

	c.msgsSent.Add(1)
	nowSent := time.Now()
	c.mu.Lock()
	c.lastReplyAt = &nowSent
	c.mu.Unlock()

	return nil
}

// SendStreaming sends a message with streaming support.
func (c *Channel) SendStreaming(ctx context.Context, chatID string, replyToID string, content <-chan string, done chan<- struct{}) error {
	defer close(done)

	if c.client == nil {
		return fmt.Errorf("client not initialized")
	}

	roomID := id.RoomID(chatID)
	var fullContent strings.Builder
	var sentEventID id.EventID
	lastUpdate := time.Now()
	updateInterval := 500 * time.Millisecond

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk, ok := <-content:
			if !ok {
				// Channel closed, send final message
				if sentEventID != "" && fullContent.Len() > 0 {
					c.editMessage(ctx, roomID, sentEventID, fullContent.String())
				}
				return nil
			}

			fullContent.WriteString(chunk)

			// Update message periodically
			if time.Since(lastUpdate) >= updateInterval {
				if sentEventID == "" {
					// Send initial message
					msgContent := &event.MessageEventContent{
						MsgType: event.MsgText,
						Body:    fullContent.String(),
					}
					if replyToID != "" {
						msgContent.RelatesTo = &event.RelatesTo{
							InReplyTo: &event.InReplyTo{
								EventID: id.EventID(replyToID),
							},
						}
					}
					resp, err := c.client.SendMessageEvent(ctx, roomID, event.EventMessage, msgContent)
					if err != nil {
						c.logger.Error("failed to send streaming message", zap.Error(err))
						continue
					}
					sentEventID = resp.EventID
				} else {
					// Edit existing message
					if err := c.editMessage(ctx, roomID, sentEventID, fullContent.String()); err != nil {
						c.logger.Warn("failed to edit streaming message", zap.Error(err))
					}
				}
				lastUpdate = time.Now()
			}
		}
	}
}

// editMessage edits an existing message.
func (c *Channel) editMessage(ctx context.Context, roomID id.RoomID, eventID id.EventID, newContent string) error {
	content := &event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    "* " + newContent,
		NewContent: &event.MessageEventContent{
			MsgType: event.MsgText,
			Body:    newContent,
		},
		RelatesTo: &event.RelatesTo{
			Type:    event.RelReplace,
			EventID: eventID,
		},
	}

	_, err := c.client.SendMessageEvent(ctx, roomID, event.EventMessage, content)
	return err
}

// Info returns current information about the channel.
func (c *Channel) Info() channel.Info {
	c.mu.RLock()
	defer c.mu.RUnlock()

	info := channel.Info{
		Name:             "matrix",
		Type:             "matrix",
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
		Metadata: map[string]interface{}{
			"homeserver": c.config.Homeserver,
			"user_id":    c.config.UserID,
		},
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

// markdownToHTML converts simple markdown to HTML.
// This is a simplified implementation; consider using a proper markdown library.
func markdownToHTML(md string) string {
	// Simple replacements
	html := md
	html = strings.ReplaceAll(html, "**", "<strong>")
	html = strings.ReplaceAll(html, "*", "<em>")
	html = strings.ReplaceAll(html, "`", "<code>")
	return html
}
