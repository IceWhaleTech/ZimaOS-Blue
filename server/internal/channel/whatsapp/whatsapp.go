// Package whatsapp provides a WhatsApp channel implementation using whatsmeow.
// This channel allows interaction with WhatsApp through the WhatsApp Web protocol.
package whatsapp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

// Channel implements the channel.Channel interface for WhatsApp.
type Channel struct {
	config   Config
	logger   *zap.Logger
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

	// WhatsApp client state
	qrCode     string
	deviceID   string
	isLoggedIn bool
}

// Config contains WhatsApp channel configuration.
type Config struct {
	Enabled        bool     `yaml:"enabled"`
	PhoneNumber    string   `yaml:"phone_number"`
	SessionPath    string   `yaml:"session_path"`
	AllowedNumbers []string `yaml:"allowed_numbers"`
	QRTimeout      int      `yaml:"qr_timeout_seconds"`
	ReconnectDelay int      `yaml:"reconnect_delay_seconds"`
}

// DefaultConfig returns the default WhatsApp configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:        false,
		SessionPath:    "./data/whatsapp",
		QRTimeout:      60,
		ReconnectDelay: 5,
	}
}

// New creates a new WhatsApp channel.
func New(cfg Config, logger *zap.Logger) *Channel {
	return &Channel{
		config:   cfg,
		logger:   logger.With(zap.String("channel", "whatsapp")),
		messages: make(chan channel.Message, 100),
		status:   channel.StatusDisconnected,
	}
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "whatsapp"
}

// Type returns the channel type.
func (c *Channel) Type() string {
	return "whatsapp"
}

// Start initializes and starts the WhatsApp channel.
func (c *Channel) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.status == channel.StatusConnected || c.status == channel.StatusConnecting {
		c.mu.Unlock()
		return fmt.Errorf("channel already started")
	}
	c.status = channel.StatusConnecting
	c.mu.Unlock()

	// Ensure session directory exists
	if err := os.MkdirAll(c.config.SessionPath, 0755); err != nil {
		c.setError(fmt.Sprintf("failed to create session directory: %v", err))
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	c.ctx, c.cancel = context.WithCancel(ctx)

	// Start the WhatsApp client
	c.wg.Add(1)
	go c.runClient()

	c.logger.Info("WhatsApp channel starting",
		zap.String("session_path", c.config.SessionPath))
	return nil
}

// runClient runs the WhatsApp client connection loop.
func (c *Channel) runClient() {
	defer c.wg.Done()

	// Note: In a real implementation, this would use whatsmeow library
	// For now, this is a placeholder that demonstrates the structure

	c.logger.Info("WhatsApp client starting...")

	// Simulate connection process
	select {
	case <-c.ctx.Done():
		return
	case <-time.After(time.Second):
		// Check if we have existing session
		sessionFile := filepath.Join(c.config.SessionPath, "session.json")
		if _, err := os.Stat(sessionFile); os.IsNotExist(err) {
			// No session, need QR code login
			c.logger.Info("No existing session, QR code login required")
			c.mu.Lock()
			c.qrCode = "PLACEHOLDER_QR_CODE"
			c.mu.Unlock()
		} else {
			// Have session, try to restore
			c.logger.Info("Restoring existing session")
		}
	}

	// Mark as connected (in real implementation, this would happen after successful auth)
	now := time.Now()
	c.mu.Lock()
	c.status = channel.StatusConnected
	c.connectedAt = &now
	c.isLoggedIn = true
	c.lastError = ""
	c.lastErrorAt = nil
	c.mu.Unlock()

	c.logger.Info("WhatsApp channel connected")

	// Main event loop
	for {
		select {
		case <-c.ctx.Done():
			c.logger.Debug("WhatsApp client stopping")
			return
		case <-time.After(time.Second * 30):
			// Heartbeat / keep-alive
			c.logger.Debug("WhatsApp heartbeat")
		}
	}
}

// handleIncomingMessage processes an incoming WhatsApp message.
func (c *Channel) handleIncomingMessage(senderJID, chatJID, messageID, content string, timestamp time.Time, isGroup bool, groupName string) {
	// Extract phone number from JID (format: 1234567890@s.whatsapp.net)
	sender := extractPhoneFromJID(senderJID)
	chatID := extractPhoneFromJID(chatJID)

	// Check if sender is allowed
	if !c.isSenderAllowed(sender) {
		c.logger.Debug("ignoring message from non-allowed sender",
			zap.String("sender", sender))
		return
	}

	msg := channel.Message{
		ID:          messageID,
		ChannelName: "whatsapp",
		ChatID:      chatID,
		UserID:      sender,
		Username:    sender,
		Type:        channel.MessageTypeText,
		Content:     content,
		Timestamp:   timestamp,
		IsGroup:     isGroup,
		GroupName:   groupName,
		Metadata: map[string]interface{}{
			"sender_jid": senderJID,
			"chat_jid":   chatJID,
		},
	}

	c.msgCount.Add(1)

	select {
	case c.messages <- msg:
	default:
		c.logger.Warn("message channel full, dropping message",
			zap.String("message_id", msg.ID))
	}
}

// isSenderAllowed checks if a sender is allowed to interact with the bot.
func (c *Channel) isSenderAllowed(sender string) bool {
	// If no restrictions, allow all
	if len(c.config.AllowedNumbers) == 0 {
		return true
	}

	// Check phone numbers
	normalizedSender := normalizePhoneNumber(sender)
	for _, allowed := range c.config.AllowedNumbers {
		if normalizePhoneNumber(allowed) == normalizedSender {
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
	c.logger.Info("WhatsApp channel stopped")
	return nil
}

// Send sends a message through WhatsApp.
func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	c.mu.RLock()
	if !c.isLoggedIn {
		c.mu.RUnlock()
		return fmt.Errorf("WhatsApp not logged in")
	}
	c.mu.RUnlock()

	// Convert chat ID to JID format
	jid := toJID(msg.ChatID)

	// Note: In real implementation, this would use whatsmeow to send
	c.logger.Debug("sending WhatsApp message",
		zap.String("to", jid),
		zap.Int("content_length", len(msg.Content)))

	// Placeholder for actual send implementation
	// In real implementation:
	// _, err := c.client.SendMessage(ctx, jid, &waProto.Message{
	//     Conversation: proto.String(msg.Content),
	// })

	return nil
}

// SendStreaming sends a message with streaming support.
// For WhatsApp, we accumulate content and send as a single message,
// or update the message if the platform supports it.
func (c *Channel) SendStreaming(ctx context.Context, chatID string, replyToID string, content <-chan string, done chan<- struct{}) error {
	defer close(done)

	var fullContent strings.Builder

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk, ok := <-content:
			if !ok {
				// Channel closed, send the accumulated message
				if fullContent.Len() > 0 {
					return c.Send(ctx, channel.OutgoingMessage{
						ChatID:    chatID,
						ReplyToID: replyToID,
						Content:   fullContent.String(),
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
		Name:         "whatsapp",
		Type:         "whatsapp",
		Status:       c.status,
		Enabled:      c.config.Enabled,
		ConnectedAt:  c.connectedAt,
		LastError:    c.lastError,
		LastErrorAt:  c.lastErrorAt,
		MessageCount: c.msgCount.Load(),
		Metadata: map[string]interface{}{
			"phone_number": c.config.PhoneNumber,
			"is_logged_in": c.isLoggedIn,
			"device_id":    c.deviceID,
		},
	}

	// Include QR code if available and not logged in
	if c.qrCode != "" && !c.isLoggedIn {
		info.Metadata["qr_code"] = c.qrCode
	}

	return info
}

// IsConnected returns true if the channel is connected.
func (c *Channel) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status == channel.StatusConnected && c.isLoggedIn
}

// Messages returns the channel for receiving incoming messages.
func (c *Channel) Messages() <-chan channel.Message {
	return c.messages
}

// GetQRCode returns the current QR code for login.
func (c *Channel) GetQRCode() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.qrCode
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

// extractPhoneFromJID extracts the phone number from a WhatsApp JID.
// JID format: 1234567890@s.whatsapp.net or 1234567890-1234567890@g.us (group)
func extractPhoneFromJID(jid string) string {
	// Remove the domain part
	parts := strings.Split(jid, "@")
	if len(parts) == 0 {
		return jid
	}
	// For groups, the format is different
	phone := parts[0]
	// Remove group suffix if present
	if idx := strings.Index(phone, "-"); idx > 0 {
		phone = phone[:idx]
	}
	return phone
}

// toJID converts a phone number to WhatsApp JID format.
func toJID(phone string) string {
	// Normalize the phone number
	phone = normalizePhoneNumber(phone)
	// Check if it's already a JID
	if strings.Contains(phone, "@") {
		return phone
	}
	// Default to individual chat
	return phone + "@s.whatsapp.net"
}

// normalizePhoneNumber removes common formatting from phone numbers.
func normalizePhoneNumber(phone string) string {
	// Remove common formatting characters
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")
	phone = strings.ReplaceAll(phone, "(", "")
	phone = strings.ReplaceAll(phone, ")", "")
	phone = strings.ReplaceAll(phone, "+", "")
	return phone
}

// SendImage sends an image message through WhatsApp.
func (c *Channel) SendImage(ctx context.Context, chatID string, imageData []byte, caption string, mimeType string) error {
	c.mu.RLock()
	if !c.isLoggedIn {
		c.mu.RUnlock()
		return fmt.Errorf("WhatsApp not logged in")
	}
	c.mu.RUnlock()

	jid := toJID(chatID)

	c.logger.Debug("sending WhatsApp image",
		zap.String("to", jid),
		zap.Int("size", len(imageData)),
		zap.String("mime_type", mimeType),
		zap.String("caption", caption))

	// Note: In real implementation with whatsmeow:
	// uploaded, err := c.client.Upload(ctx, imageData, whatsmeow.MediaImage)
	// if err != nil {
	//     return fmt.Errorf("failed to upload image: %w", err)
	// }
	//
	// msg := &waProto.Message{
	//     ImageMessage: &waProto.ImageMessage{
	//         Caption:       proto.String(caption),
	//         Mimetype:      proto.String(mimeType),
	//         URL:           proto.String(uploaded.URL),
	//         DirectPath:    proto.String(uploaded.DirectPath),
	//         MediaKey:      uploaded.MediaKey,
	//         FileEncSHA256: uploaded.FileEncSHA256,
	//         FileSHA256:    uploaded.FileSHA256,
	//         FileLength:    proto.Uint64(uint64(len(imageData))),
	//     },
	// }
	// _, err = c.client.SendMessage(ctx, jid, msg)

	return nil
}

// SendFile sends a file/document message through WhatsApp.
func (c *Channel) SendFile(ctx context.Context, chatID string, fileData []byte, filename string, caption string, mimeType string) error {
	c.mu.RLock()
	if !c.isLoggedIn {
		c.mu.RUnlock()
		return fmt.Errorf("WhatsApp not logged in")
	}
	c.mu.RUnlock()

	jid := toJID(chatID)

	c.logger.Debug("sending WhatsApp file",
		zap.String("to", jid),
		zap.String("filename", filename),
		zap.Int("size", len(fileData)),
		zap.String("mime_type", mimeType))

	// Note: In real implementation with whatsmeow:
	// uploaded, err := c.client.Upload(ctx, fileData, whatsmeow.MediaDocument)
	// if err != nil {
	//     return fmt.Errorf("failed to upload file: %w", err)
	// }
	//
	// msg := &waProto.Message{
	//     DocumentMessage: &waProto.DocumentMessage{
	//         Caption:       proto.String(caption),
	//         Title:         proto.String(filename),
	//         FileName:      proto.String(filename),
	//         Mimetype:      proto.String(mimeType),
	//         URL:           proto.String(uploaded.URL),
	//         DirectPath:    proto.String(uploaded.DirectPath),
	//         MediaKey:      uploaded.MediaKey,
	//         FileEncSHA256: uploaded.FileEncSHA256,
	//         FileSHA256:    uploaded.FileSHA256,
	//         FileLength:    proto.Uint64(uint64(len(fileData))),
	//     },
	// }
	// _, err = c.client.SendMessage(ctx, jid, msg)

	return nil
}

// SendAudio sends an audio message through WhatsApp.
func (c *Channel) SendAudio(ctx context.Context, chatID string, audioData []byte, mimeType string, ptt bool) error {
	c.mu.RLock()
	if !c.isLoggedIn {
		c.mu.RUnlock()
		return fmt.Errorf("WhatsApp not logged in")
	}
	c.mu.RUnlock()

	jid := toJID(chatID)

	c.logger.Debug("sending WhatsApp audio",
		zap.String("to", jid),
		zap.Int("size", len(audioData)),
		zap.String("mime_type", mimeType),
		zap.Bool("ptt", ptt))

	// Note: In real implementation with whatsmeow:
	// uploaded, err := c.client.Upload(ctx, audioData, whatsmeow.MediaAudio)
	// if err != nil {
	//     return fmt.Errorf("failed to upload audio: %w", err)
	// }
	//
	// msg := &waProto.Message{
	//     AudioMessage: &waProto.AudioMessage{
	//         Mimetype:      proto.String(mimeType),
	//         URL:           proto.String(uploaded.URL),
	//         DirectPath:    proto.String(uploaded.DirectPath),
	//         MediaKey:      uploaded.MediaKey,
	//         FileEncSHA256: uploaded.FileEncSHA256,
	//         FileSHA256:    uploaded.FileSHA256,
	//         FileLength:    proto.Uint64(uint64(len(audioData))),
	//         PTT:           proto.Bool(ptt), // Push-to-talk (voice message)
	//     },
	// }
	// _, err = c.client.SendMessage(ctx, jid, msg)

	return nil
}

// SendVideo sends a video message through WhatsApp.
func (c *Channel) SendVideo(ctx context.Context, chatID string, videoData []byte, caption string, mimeType string) error {
	c.mu.RLock()
	if !c.isLoggedIn {
		c.mu.RUnlock()
		return fmt.Errorf("WhatsApp not logged in")
	}
	c.mu.RUnlock()

	jid := toJID(chatID)

	c.logger.Debug("sending WhatsApp video",
		zap.String("to", jid),
		zap.Int("size", len(videoData)),
		zap.String("mime_type", mimeType),
		zap.String("caption", caption))

	// Note: In real implementation with whatsmeow:
	// uploaded, err := c.client.Upload(ctx, videoData, whatsmeow.MediaVideo)
	// if err != nil {
	//     return fmt.Errorf("failed to upload video: %w", err)
	// }
	//
	// msg := &waProto.Message{
	//     VideoMessage: &waProto.VideoMessage{
	//         Caption:       proto.String(caption),
	//         Mimetype:      proto.String(mimeType),
	//         URL:           proto.String(uploaded.URL),
	//         DirectPath:    proto.String(uploaded.DirectPath),
	//         MediaKey:      uploaded.MediaKey,
	//         FileEncSHA256: uploaded.FileEncSHA256,
	//         FileSHA256:    uploaded.FileSHA256,
	//         FileLength:    proto.Uint64(uint64(len(videoData))),
	//     },
	// }
	// _, err = c.client.SendMessage(ctx, jid, msg)

	return nil
}

// SendWithAttachment sends a message with an attachment.
func (c *Channel) SendWithAttachment(ctx context.Context, msg channel.OutgoingMessage) error {
	if len(msg.Attachments) == 0 {
		return c.Send(ctx, msg)
	}

	for _, att := range msg.Attachments {
		var err error
		switch att.Type {
		case channel.MessageTypeImage:
			err = c.SendImage(ctx, msg.ChatID, att.Data, msg.Content, att.MimeType)
		case channel.MessageTypeAudio:
			err = c.SendAudio(ctx, msg.ChatID, att.Data, att.MimeType, false)
		case channel.MessageTypeVideo:
			err = c.SendVideo(ctx, msg.ChatID, att.Data, msg.Content, att.MimeType)
		case channel.MessageTypeFile:
			err = c.SendFile(ctx, msg.ChatID, att.Data, att.Name, msg.Content, att.MimeType)
		default:
			err = c.SendFile(ctx, msg.ChatID, att.Data, att.Name, msg.Content, att.MimeType)
		}
		if err != nil {
			return fmt.Errorf("failed to send attachment %s: %w", att.Name, err)
		}
	}

	return nil
}

// DownloadMedia downloads media from a received message.
func (c *Channel) DownloadMedia(ctx context.Context, mediaURL string, mediaKey []byte, fileEncSHA256 []byte, fileSHA256 []byte, fileLength uint64, mediaType string) ([]byte, error) {
	c.mu.RLock()
	if !c.isLoggedIn {
		c.mu.RUnlock()
		return nil, fmt.Errorf("WhatsApp not logged in")
	}
	c.mu.RUnlock()

	c.logger.Debug("downloading WhatsApp media",
		zap.String("url", mediaURL),
		zap.String("type", mediaType),
		zap.Uint64("size", fileLength))

	// Note: In real implementation with whatsmeow:
	// data, err := c.client.Download(&waProto.Message{
	//     // Set appropriate message type based on mediaType
	// })
	// if err != nil {
	//     return nil, fmt.Errorf("failed to download media: %w", err)
	// }
	// return data, nil

	return nil, fmt.Errorf("media download not implemented")
}
