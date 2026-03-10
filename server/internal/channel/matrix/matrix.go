// Package matrix provides a Matrix protocol channel implementation.
package matrix

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/humanizer"
)

// Channel implements the channel.Channel interface for Matrix.
type Channel struct {
	config   channel.MatrixConfig
	logger   *zap.Logger
	client   *matrixClient
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

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func New(cfg channel.MatrixConfig, logger *zap.Logger) *Channel {
	return &Channel{
		config:   cfg,
		logger:   logger.With(zap.String("channel", "matrix")),
		messages: make(chan channel.Message, 100),
		status:   channel.StatusDisconnected,
	}
}

func (c *Channel) Name() string { return "matrix" }
func (c *Channel) Type() string { return "matrix" }

func (c *Channel) OutboundCapabilities() channel.OutboundCapabilities {
	return channel.OutboundCapabilities{
		MarkdownMode:           channel.OutboundMarkdownModeChunked,
		HumanizerPreset:        "matrix",
		SupportsMarkdownFormat: true,
	}
}

func (c *Channel) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.status == channel.StatusConnected || c.status == channel.StatusConnecting {
		c.mu.Unlock()
		return fmt.Errorf("channel already started")
	}
	c.status = channel.StatusConnecting
	c.mu.Unlock()

	c.ctx, c.cancel = context.WithCancel(ctx)

	client := newMatrixClient(c.config.Homeserver, c.config.UserID, c.config.AccessToken)
	if c.config.DeviceID != "" {
		client.deviceID = c.config.DeviceID
	}
	c.client = client

	whoami, err := client.whoami(ctx)
	if err != nil {
		c.setError(fmt.Sprintf("failed to verify credentials: %v", err))
		return fmt.Errorf("failed to verify Matrix credentials: %w", err)
	}

	c.logger.Info("matrix client authenticated",
		zap.String("user_id", whoami.UserID),
		zap.String("device_id", whoami.DeviceID))

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

func (c *Channel) syncLoop() {
	defer c.wg.Done()
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			if err := c.client.sync(c.ctx, c.handleEvent); err != nil {
				if c.ctx.Err() != nil {
					return
				}
				c.logger.Error("sync error", zap.Error(err))
				c.setError(fmt.Sprintf("sync error: %v", err))
				select {
				case <-c.ctx.Done():
					return
				case <-time.After(5 * time.Second):
				}
			}
		}
	}
}

func (c *Channel) handleEvent(roomID string, evt matrixEvent) {
	if evt.Type != "m.room.message" {
		return
	}
	if evt.Sender == c.config.UserID {
		return
	}
	if !c.isRoomAllowed(roomID) {
		return
	}

	var content messageContent
	if err := json.Unmarshal(evt.Content, &content); err != nil {
		return
	}

	channelMsg := c.convertMessage(roomID, evt, content)
	c.msgCount.Add(1)
	c.msgsReceived.Add(1)
	now := time.Now()
	c.mu.Lock()
	c.lastMessageAt = &now
	c.mu.Unlock()

	select {
	case c.messages <- channelMsg:
	default:
		c.logger.Warn("message channel full, dropping message", zap.String("message_id", channelMsg.ID))
	}
}

func (c *Channel) convertMessage(roomID string, evt matrixEvent, content messageContent) channel.Message {
	msgType := channel.MessageTypeText
	switch content.MsgType {
	case "m.image":
		msgType = channel.MessageTypeImage
	case "m.audio":
		msgType = channel.MessageTypeAudio
	case "m.video":
		msgType = channel.MessageTypeVideo
	case "m.file":
		msgType = channel.MessageTypeFile
	}

	channelMsg := channel.Message{
		ID: evt.EventID, ChannelName: "matrix", ChatID: roomID, UserID: evt.Sender,
		Type: msgType, Content: content.Body, Timestamp: time.UnixMilli(evt.Time),
		IsGroup: true,
		Metadata: map[string]interface{}{
			"msg_type": content.MsgType, "format": content.Format, "formatted_body": content.FormattedBody,
		},
	}

	if content.RelatesTo != nil && content.RelatesTo.InReplyTo != nil {
		channelMsg.ReplyToID = content.RelatesTo.InReplyTo.EventID
	}
	if content.URL != "" {
		size := 0
		mime := ""
		if content.Info != nil {
			size = content.Info.Size
			mime = content.Info.MimeType
		}
		channelMsg.Attachments = append(channelMsg.Attachments, channel.Attachment{
			ID: content.URL, Type: msgType, Name: content.Body, URL: content.URL,
			Size: int64(size), MimeType: mime,
		})
	}
	return channelMsg
}

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
	done := make(chan struct{})
	go func() { c.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}
	close(c.messages)
	c.logger.Info("matrix channel stopped")
	return nil
}

func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	if c.client == nil {
		return fmt.Errorf("client not initialized")
	}

	fallbackLines := make([]string, 0)

	// Send attachments as media messages.
	for _, att := range msg.Attachments {
		if len(att.Data) == 0 {
			if link := buildAttachmentFallback(att); link != "" {
				fallbackLines = append(fallbackLines, link)
			}
			continue
		}

		mime := att.MimeType
		if mime == "" {
			mime = "application/octet-stream"
		}
		name := att.Name
		if name == "" {
			name = "file"
		}
		mxcURI, err := c.client.uploadMedia(ctx, name, mime, att.Data)
		if err != nil {
			return fmt.Errorf("failed to upload media to matrix: %w", err)
		}
		// Determine m.msgtype based on attachment type.
		msgType := "m.file"
		switch att.Type {
		case channel.MessageTypeImage:
			msgType = "m.image"
		case channel.MessageTypeAudio:
			msgType = "m.audio"
		case channel.MessageTypeVideo:
			msgType = "m.video"
		}
		mediaContent := map[string]interface{}{
			"msgtype": msgType,
			"body":    name,
			"url":     mxcURI,
			"info":    map[string]interface{}{"mimetype": mime, "size": len(att.Data)},
		}
		if msg.ReplyToID != "" {
			mediaContent["m.relates_to"] = map[string]interface{}{
				"m.in_reply_to": map[string]string{"event_id": msg.ReplyToID},
			}
		}
		if _, err := c.client.sendMessage(ctx, msg.ChatID, mediaContent); err != nil {
			return fmt.Errorf("failed to send matrix media message: %w", err)
		}
	}

	textBody := msg.Content
	if len(fallbackLines) > 0 {
		fallbackText := strings.Join(fallbackLines, "\n")
		if textBody == "" {
			textBody = fallbackText
		} else {
			textBody = textBody + "\n\n" + fallbackText
		}
	}

	// Send text if present and no attachments consumed it.
	if textBody != "" {
		content := &messageContent{MsgType: "m.text", Body: textBody}
		if msg.Format == "html" {
			content.Format = "org.matrix.custom.html"
			content.FormattedBody = textBody
		} else if msg.Format == "markdown" {
			content.Format = "org.matrix.custom.html"
			content.FormattedBody = markdownToHTML(textBody)
		}
		if msg.ReplyToID != "" {
			content.RelatesTo = &relatesTo{InReplyTo: &inReplyTo{EventID: msg.ReplyToID}}
		}
		_, err := c.client.sendMessage(ctx, msg.ChatID, content)
		if err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}
	}

	c.msgsSent.Add(1)
	now := time.Now()
	c.mu.Lock()
	c.lastReplyAt = &now
	c.mu.Unlock()
	return nil
}

func (c *Channel) SendStreaming(ctx context.Context, chatID string, replyToID string, content <-chan string, done chan<- struct{}) error {
	defer close(done)
	if c.client == nil {
		return fmt.Errorf("client not initialized")
	}
	var fullContent strings.Builder
	var sentEventID string
	lastUpdate := time.Now()
	updateInterval := 500 * time.Millisecond

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk, ok := <-content:
			if !ok {
				if sentEventID != "" && fullContent.Len() > 0 {
					c.editMessage(ctx, chatID, sentEventID, fullContent.String())
				}
				return nil
			}
			fullContent.WriteString(chunk)
			if time.Since(lastUpdate) >= updateInterval {
				if sentEventID == "" {
					mc := &messageContent{MsgType: "m.text", Body: fullContent.String()}
					if replyToID != "" {
						mc.RelatesTo = &relatesTo{InReplyTo: &inReplyTo{EventID: replyToID}}
					}
					eid, err := c.client.sendMessage(ctx, chatID, mc)
					if err != nil {
						c.logger.Error("failed to send streaming message", zap.Error(err))
						continue
					}
					sentEventID = eid
				} else {
					c.editMessage(ctx, chatID, sentEventID, fullContent.String())
				}
				lastUpdate = time.Now()
			}
		}
	}
}

func (c *Channel) editMessage(ctx context.Context, roomID, eventID, newContent string) error {
	content := &messageContent{
		MsgType:    "m.text",
		Body:       "* " + newContent,
		NewContent: &messageContent{MsgType: "m.text", Body: newContent},
		RelatesTo:  &relatesTo{RelType: "m.replace", EventID: eventID},
	}
	_, err := c.client.sendMessage(ctx, roomID, content)
	return err
}

// SendTyping sends a typing indicator to the given room.
func (c *Channel) SendTyping(ctx context.Context, chatID string) error {
	if c.client == nil {
		return fmt.Errorf("client not initialized")
	}
	return c.client.userTyping(ctx, chatID)
}

func (c *Channel) Info() channel.Info {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return channel.Info{
		Name: "matrix", Type: "matrix", Status: c.status, Enabled: c.config.Enabled,
		ConnectedAt: c.connectedAt, LastError: c.lastError, LastErrorAt: c.lastErrorAt,
		MessageCount: c.msgCount.Load(), MessagesReceived: c.msgsReceived.Load(), MessagesSent: c.msgsSent.Load(),
		LastMessageAt: c.lastMessageAt, LastReplyAt: c.lastReplyAt,
		Metadata: map[string]interface{}{"homeserver": c.config.Homeserver, "user_id": c.config.UserID},
	}
}

func (c *Channel) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status == channel.StatusConnected
}

func (c *Channel) Messages() <-chan channel.Message { return c.messages }

func (c *Channel) setError(err string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastError = err
	now := time.Now()
	c.lastErrorAt = &now
	c.status = channel.StatusError
}

func markdownToHTML(md string) string {
	if strings.TrimSpace(md) == "" {
		return ""
	}
	ir := humanizer.Parse(md, humanizer.ParseOptions{
		HeadingStyle:     "bold",
		BlockquotePrefix: "",
		TableMode:        "bullets",
	})
	return humanizer.RenderMatrix(ir)
}

func buildAttachmentFallback(att channel.Attachment) string {
	if strings.TrimSpace(att.URL) != "" {
		return strings.TrimSpace(att.URL)
	}
	return strings.TrimSpace(att.Name)
}
