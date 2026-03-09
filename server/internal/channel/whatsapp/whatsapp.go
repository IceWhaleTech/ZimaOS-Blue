// Package whatsapp provides a WhatsApp channel implementation backed by the wacli CLI.
// This channel allows interaction with WhatsApp through the WhatsApp CLI backend.
package whatsapp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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

	// WhatsApp client state
	qrCode     string
	deviceID   string
	isLoggedIn bool
	cliPath    string

	runCommand func(ctx context.Context, name string, args ...string) ([]byte, error)
	ensureCLI  func(ctx context.Context) (string, error)

	sendTextFunc  func(ctx context.Context, jid, text, replyToID string) error
	sendImageFunc func(ctx context.Context, jid string, imageData []byte, caption string, mimeType string) error
	sendFileFunc  func(ctx context.Context, jid string, fileData []byte, filename string, caption string, mimeType string) error
	sendAudioFunc func(ctx context.Context, jid string, audioData []byte, mimeType string, ptt bool) error
	sendVideoFunc func(ctx context.Context, jid string, videoData []byte, caption string, mimeType string) error
}

// Config contains WhatsApp channel configuration.
type Config struct {
	Enabled        bool     `yaml:"enabled"`
	PhoneNumber    string   `yaml:"phone_number"`
	SessionPath    string   `yaml:"session_path"`
	CLIPath        string   `yaml:"cli_path"`
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
	c := &Channel{
		config:   cfg,
		logger:   logger.With(zap.String("channel", "whatsapp")),
		messages: make(chan channel.Message, 100),
		status:   channel.StatusDisconnected,
	}
	c.sendTextFunc = c.defaultSendText
	c.sendImageFunc = c.defaultSendImage
	c.sendFileFunc = c.defaultSendFile
	c.sendAudioFunc = c.defaultSendAudio
	c.sendVideoFunc = c.defaultSendVideo
	c.runCommand = execCommandOutput
	c.ensureCLI = ensureWACLI
	return c
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
	cliPath, err := c.resolveCLIPath(ctx)
	if err != nil {
		c.setAuthInstruction(err)
		return err
	}
	c.cliPath = cliPath

	if err := c.checkBackendReady(ctx); err != nil {
		c.setAuthInstruction(err)
		return err
	}

	now := time.Now()
	c.mu.Lock()
	c.status = channel.StatusConnected
	c.connectedAt = &now
	c.isLoggedIn = true
	c.lastError = ""
	c.lastErrorAt = nil
	c.qrCode = ""
	c.mu.Unlock()

	// Start health watcher for backend availability.
	c.wg.Add(1)
	go c.runClient()

	c.logger.Info("WhatsApp channel starting",
		zap.String("session_path", c.config.SessionPath),
		zap.String("cli_path", c.cliPath))
	return nil
}

// runClient runs the WhatsApp client connection loop.
func (c *Channel) runClient() {
	defer c.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			c.logger.Debug("WhatsApp client stopping")
			return
		case <-ticker.C:
			if err := c.checkBackendReady(c.ctx); err != nil {
				c.logger.Warn("WhatsApp backend health check failed", zap.Error(err))
				c.setAuthInstruction(err)
				continue
			}
			c.mu.Lock()
			c.status = channel.StatusConnected
			c.isLoggedIn = true
			c.lastError = ""
			c.lastErrorAt = nil
			c.qrCode = ""
			c.mu.Unlock()
		}
	}
}

// handleIncomingMessage processes an incoming WhatsApp message.
func (c *Channel) handleIncomingMessage(senderJID, chatJID, messageID, content string, timestamp time.Time, isGroup bool, groupName string) {
	// Extract phone number from JID (format: 1807890@s.whatsapp.net)
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
	c.msgsReceived.Add(1)
	now := time.Now()
	c.mu.Lock()
	c.lastMessageAt = &now
	c.mu.Unlock()

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

	// Route attachments to the appropriate send method.
	if len(msg.Attachments) > 0 {
		return c.SendWithAttachment(ctx, msg)
	}
	if strings.TrimSpace(msg.Content) == "" {
		return nil
	}

	// Convert chat ID to JID format
	jid := toJID(msg.ChatID)
	if err := c.sendTextFunc(ctx, jid, msg.Content, msg.ReplyToID); err != nil {
		return fmt.Errorf("failed to send WhatsApp text message: %w", err)
	}
	c.markReplySent()
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
		Name:             "whatsapp",
		Type:             "whatsapp",
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
			"phone_number": c.config.PhoneNumber,
			"is_logged_in": c.isLoggedIn,
			"device_id":    c.deviceID,
			"cli_path":     c.cliPath,
			"store_path":   c.config.SessionPath,
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
	c.isLoggedIn = false
}

// extractPhoneFromJID extracts the phone number from a WhatsApp JID.
// JID format: 1807890@s.whatsapp.net or 1807890-1807890@g.us (group)
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

	return c.sendImageFunc(ctx, toJID(chatID), imageData, caption, mimeType)
}

// SendFile sends a file/document message through WhatsApp.
func (c *Channel) SendFile(ctx context.Context, chatID string, fileData []byte, filename string, caption string, mimeType string) error {
	c.mu.RLock()
	if !c.isLoggedIn {
		c.mu.RUnlock()
		return fmt.Errorf("WhatsApp not logged in")
	}
	c.mu.RUnlock()

	return c.sendFileFunc(ctx, toJID(chatID), fileData, filename, caption, mimeType)
}

// SendAudio sends an audio message through WhatsApp.
func (c *Channel) SendAudio(ctx context.Context, chatID string, audioData []byte, mimeType string, ptt bool) error {
	c.mu.RLock()
	if !c.isLoggedIn {
		c.mu.RUnlock()
		return fmt.Errorf("WhatsApp not logged in")
	}
	c.mu.RUnlock()

	return c.sendAudioFunc(ctx, toJID(chatID), audioData, mimeType, ptt)
}

// SendVideo sends a video message through WhatsApp.
func (c *Channel) SendVideo(ctx context.Context, chatID string, videoData []byte, caption string, mimeType string) error {
	c.mu.RLock()
	if !c.isLoggedIn {
		c.mu.RUnlock()
		return fmt.Errorf("WhatsApp not logged in")
	}
	c.mu.RUnlock()

	return c.sendVideoFunc(ctx, toJID(chatID), videoData, caption, mimeType)
}

// SendWithAttachment sends a message with an attachment.
func (c *Channel) SendWithAttachment(ctx context.Context, msg channel.OutgoingMessage) error {
	if len(msg.Attachments) == 0 {
		return c.Send(ctx, msg)
	}

	captionConsumed := false
	sentSomething := false
	for _, att := range msg.Attachments {
		caption := ""
		if !captionConsumed {
			caption = msg.Content
		}

		if len(att.Data) == 0 {
			fallback := buildAttachmentFallback(caption, att)
			if fallback == "" {
				continue
			}
			if err := c.sendTextFunc(ctx, toJID(msg.ChatID), fallback, msg.ReplyToID); err != nil {
				return fmt.Errorf("failed to send WhatsApp attachment fallback %s: %w", att.Name, err)
			}
			sentSomething = true
			if caption != "" {
				captionConsumed = true
			}
			continue
		}

		var err error
		switch att.Type {
		case channel.MessageTypeImage:
			err = c.SendImage(ctx, msg.ChatID, att.Data, caption, att.MimeType)
		case channel.MessageTypeAudio:
			err = c.SendAudio(ctx, msg.ChatID, att.Data, att.MimeType, false)
		case channel.MessageTypeVideo:
			err = c.SendVideo(ctx, msg.ChatID, att.Data, caption, att.MimeType)
		case channel.MessageTypeFile:
			err = c.SendFile(ctx, msg.ChatID, att.Data, att.Name, caption, att.MimeType)
		default:
			err = c.SendFile(ctx, msg.ChatID, att.Data, att.Name, caption, att.MimeType)
		}
		if err != nil {
			return fmt.Errorf("failed to send attachment %s: %w", att.Name, err)
		}
		sentSomething = true
		if caption != "" {
			captionConsumed = true
		}
	}

	if msg.Content != "" && !captionConsumed {
		if err := c.sendTextFunc(ctx, toJID(msg.ChatID), msg.Content, msg.ReplyToID); err != nil {
			return fmt.Errorf("failed to send WhatsApp trailing text: %w", err)
		}
		sentSomething = true
	}

	if !sentSomething {
		return fmt.Errorf("no sendable WhatsApp content")
	}

	c.markReplySent()
	return nil
}

func (c *Channel) markReplySent() {
	c.msgsSent.Add(1)
	now := time.Now()
	c.mu.Lock()
	c.lastReplyAt = &now
	c.mu.Unlock()
}

func (c *Channel) defaultSendText(ctx context.Context, jid, text, replyToID string) error {
	args := append(c.baseWACLIArgs(), "send", "text", "--to", toCLIRecipient(jid), "--message", text)
	if replyToID != "" {
		c.logger.Debug("WhatsApp reply requested but wacli backend does not expose reply threading",
			zap.String("reply_to_id", replyToID))
	}
	_, err := c.runWACLI(ctx, args...)
	return err
}

func (c *Channel) defaultSendImage(ctx context.Context, jid string, imageData []byte, caption string, mimeType string) error {
	return c.sendBinaryFile(ctx, jid, imageData, fileNameWithFallback("image", mimeType, ".bin"), caption, mimeType)
}

func (c *Channel) defaultSendFile(ctx context.Context, jid string, fileData []byte, filename string, caption string, mimeType string) error {
	return c.sendBinaryFile(ctx, jid, fileData, fileNameWithFallback(filename, mimeType, ".bin"), caption, mimeType)
}

func (c *Channel) defaultSendAudio(ctx context.Context, jid string, audioData []byte, mimeType string, ptt bool) error {
	if ptt {
		c.logger.Debug("WhatsApp PTT requested; wacli backend sends as generic file")
	}
	return c.sendBinaryFile(ctx, jid, audioData, fileNameWithFallback("audio", mimeType, ".bin"), "", mimeType)
}

func (c *Channel) defaultSendVideo(ctx context.Context, jid string, videoData []byte, caption string, mimeType string) error {
	return c.sendBinaryFile(ctx, jid, videoData, fileNameWithFallback("video", mimeType, ".bin"), caption, mimeType)
}

func buildAttachmentFallback(caption string, att channel.Attachment) string {
	parts := make([]string, 0, 2)
	if strings.TrimSpace(caption) != "" {
		parts = append(parts, strings.TrimSpace(caption))
	}
	if strings.TrimSpace(att.URL) != "" {
		parts = append(parts, strings.TrimSpace(att.URL))
	} else if strings.TrimSpace(att.Name) != "" && len(parts) == 0 {
		parts = append(parts, strings.TrimSpace(att.Name))
	}
	return strings.Join(parts, "\n")
}

func (c *Channel) resolveCLIPath(ctx context.Context) (string, error) {
	path := strings.TrimSpace(c.config.CLIPath)
	if path != "" {
		resolved, err := exec.LookPath(path)
		if err != nil {
			return "", fmt.Errorf("failed to locate configured WhatsApp CLI %q: %w", path, err)
		}
		return resolved, nil
	}
	if resolved, err := exec.LookPath("wacli"); err == nil {
		return resolved, nil
	}
	if cached, ok := existingCachedWACLIPath(); ok {
		return cached, nil
	}
	if c.ensureCLI == nil {
		return "", fmt.Errorf("failed to locate WhatsApp CLI %q", "wacli")
	}
	resolved, err := c.ensureCLI(ctx)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

func (c *Channel) checkBackendReady(ctx context.Context) error {
	if c.cliPath == "" {
		return fmt.Errorf("WhatsApp CLI path is not resolved")
	}
	_, err := c.runWACLI(ctx, append(c.baseWACLIArgs(), "doctor")...)
	if err != nil {
		return fmt.Errorf("WhatsApp backend is not ready: %w", err)
	}
	return nil
}

func (c *Channel) setAuthInstruction(err error) {
	instruction := ""
	if err != nil {
		instruction = err.Error()
	}
	if strings.TrimSpace(c.cliPath) != "" {
		authInstruction := fmt.Sprintf("Run `%s auth --store %s` to log in to WhatsApp.", fallbackCLIName(c.cliPath), c.config.SessionPath)
		if instruction == "" {
			instruction = authInstruction
		} else {
			instruction += ". " + authInstruction
		}
	}
	c.mu.Lock()
	c.qrCode = instruction
	c.mu.Unlock()
	c.setError(instruction)
}

func (c *Channel) baseWACLIArgs() []string {
	args := []string{}
	if strings.TrimSpace(c.config.SessionPath) != "" {
		args = append(args, "--store", c.config.SessionPath)
	}
	return args
}

func (c *Channel) runWACLI(ctx context.Context, args ...string) ([]byte, error) {
	if c.runCommand == nil {
		c.runCommand = execCommandOutput
	}
	output, err := c.runCommand(ctx, c.cliPath, args...)
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed != "" {
			return output, fmt.Errorf("%w: %s", err, trimmed)
		}
		return output, err
	}
	return output, nil
}

func (c *Channel) sendBinaryFile(ctx context.Context, jid string, data []byte, filename string, caption string, mimeType string) error {
	if len(data) == 0 {
		return fmt.Errorf("empty attachment data")
	}
	if err := os.MkdirAll(c.config.SessionPath, 0755); err != nil {
		return fmt.Errorf("ensure WhatsApp store path: %w", err)
	}
	tmpFile, err := os.CreateTemp(c.config.SessionPath, "wacli-*")
	if err != nil {
		return fmt.Errorf("create temp attachment: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp attachment: %w", err)
	}
	finalPath := tmpPath + attachmentSuffix(filename, mimeType)
	if err := os.Rename(tmpPath, finalPath); err == nil {
		tmpPath = finalPath
		defer os.Remove(tmpPath)
	}
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("write temp attachment: %w", err)
	}
	args := append(c.baseWACLIArgs(), "send", "file", "--to", toCLIRecipient(jid), "--file", tmpPath)
	if strings.TrimSpace(caption) != "" {
		args = append(args, "--caption", caption)
	}
	_, err = c.runWACLI(ctx, args...)
	return err
}

func attachmentSuffix(filename string, mimeType string) string {
	if ext := filepath.Ext(filename); ext != "" {
		return ext
	}
	switch {
	case strings.Contains(mimeType, "jpeg"):
		return ".jpg"
	case strings.Contains(mimeType, "png"):
		return ".png"
	case strings.Contains(mimeType, "gif"):
		return ".gif"
	case strings.Contains(mimeType, "webp"):
		return ".webp"
	case strings.Contains(mimeType, "mp4"):
		return ".mp4"
	case strings.Contains(mimeType, "mpeg"):
		return ".mp3"
	case strings.Contains(mimeType, "ogg"):
		return ".ogg"
	case strings.Contains(mimeType, "pdf"):
		return ".pdf"
	default:
		return ""
	}
}

func fileNameWithFallback(filename string, mimeType string, fallbackExt string) string {
	filename = strings.TrimSpace(filename)
	if filename != "" {
		return filename
	}
	if ext := attachmentSuffix(filename, mimeType); ext != "" {
		return "attachment" + ext
	}
	return "attachment" + fallbackExt
}

func toCLIRecipient(jid string) string {
	jid = strings.TrimSpace(jid)
	if strings.HasSuffix(jid, "@g.us") {
		return jid
	}
	if strings.HasSuffix(jid, "@s.whatsapp.net") {
		return "+" + extractPhoneFromJID(jid)
	}
	if strings.Contains(jid, "@") {
		return jid
	}
	return "+" + normalizePhoneNumber(jid)
}

func fallbackCLIName(path string) string {
	if strings.TrimSpace(path) == "" {
		return "wacli"
	}
	return path
}

func execCommandOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}
