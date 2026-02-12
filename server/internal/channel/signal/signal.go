// Package signal provides a Signal channel implementation.
// This channel allows interaction with Signal through signal-cli or libsignal.
package signal

import (
	"bufio"
	"context"
	"encoding/json"
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

// Channel implements the channel.Channel interface for Signal.
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

	// Signal client state
	isRegistered bool
	isLinked     bool
	cmd          *exec.Cmd
}

// Config contains Signal channel configuration.
type Config struct {
	Enabled        bool     `mapstructure:"enabled"`
	PhoneNumber    string   `mapstructure:"phone_number"`
	ConfigPath     string   `mapstructure:"config_path"`
	SignalCLIPath  string   `mapstructure:"signal_cli_path"`
	AllowedNumbers []string `mapstructure:"allowed_numbers"`
	UseJsonRpc     bool     `mapstructure:"use_json_rpc"`
}

// DefaultConfig returns the default Signal configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:       false,
		ConfigPath:    "./data/signal",
		SignalCLIPath: "signal-cli",
		UseJsonRpc:    true,
	}
}

// SignalMessage represents a message from signal-cli JSON output.
type SignalMessage struct {
	Envelope struct {
		Source        string `json:"source"`
		SourceNumber  string `json:"sourceNumber"`
		SourceUUID    string `json:"sourceUuid"`
		SourceName    string `json:"sourceName"`
		SourceDevice  int    `json:"sourceDevice"`
		Timestamp     int64  `json:"timestamp"`
		DataMessage   *DataMessage `json:"dataMessage"`
		SyncMessage   *SyncMessage `json:"syncMessage"`
	} `json:"envelope"`
}

// DataMessage represents a Signal data message.
type DataMessage struct {
	Timestamp        int64       `json:"timestamp"`
	Message          string      `json:"message"`
	ExpiresInSeconds int         `json:"expiresInSeconds"`
	GroupInfo        *GroupInfo  `json:"groupInfo"`
}

// SyncMessage represents a Signal sync message.
type SyncMessage struct {
	SentMessage *SentMessage `json:"sentMessage"`
}

// SentMessage represents a sent message in sync.
type SentMessage struct {
	Destination string       `json:"destination"`
	Timestamp   int64        `json:"timestamp"`
	Message     string       `json:"message"`
	GroupInfo   *GroupInfo   `json:"groupInfo"`
}

// GroupInfo represents Signal group information.
type GroupInfo struct {
	GroupID string `json:"groupId"`
	Type    string `json:"type"`
	Name    string `json:"name"`
}

// New creates a new Signal channel.
func New(cfg Config, logger *zap.Logger) *Channel {
	return &Channel{
		config:   cfg,
		logger:   logger.With(zap.String("channel", "signal")),
		messages: make(chan channel.Message, 100),
		status:   channel.StatusDisconnected,
	}
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "signal"
}

// Type returns the channel type.
func (c *Channel) Type() string {
	return "signal"
}

// Start initializes and starts the Signal channel.
func (c *Channel) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.status == channel.StatusConnected || c.status == channel.StatusConnecting {
		c.mu.Unlock()
		return fmt.Errorf("channel already started")
	}
	c.status = channel.StatusConnecting
	c.mu.Unlock()

	// Ensure config directory exists
	if err := os.MkdirAll(c.config.ConfigPath, 0755); err != nil {
		c.setError(fmt.Sprintf("failed to create config directory: %v", err))
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Check if signal-cli is available
	if err := c.checkSignalCLI(); err != nil {
		c.setError(fmt.Sprintf("signal-cli not available: %v", err))
		return fmt.Errorf("signal-cli not available: %w", err)
	}

	c.ctx, c.cancel = context.WithCancel(ctx)

	// Start receiving messages
	c.wg.Add(1)
	go c.receiveMessages()

	c.logger.Info("Signal channel starting",
		zap.String("config_path", c.config.ConfigPath),
		zap.String("phone_number", c.config.PhoneNumber))
	return nil
}

// checkSignalCLI verifies that signal-cli is available.
func (c *Channel) checkSignalCLI() error {
	cmd := exec.Command(c.config.SignalCLIPath, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("signal-cli not found or not executable: %w", err)
	}
	c.logger.Debug("signal-cli version", zap.String("version", strings.TrimSpace(string(output))))
	return nil
}

// receiveMessages starts the signal-cli daemon and receives messages.
func (c *Channel) receiveMessages() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		// Start signal-cli in receive mode
		args := []string{
			"--config", c.config.ConfigPath,
			"-u", c.config.PhoneNumber,
			"receive",
			"--json",
			"--timeout", "-1", // Infinite timeout
		}

		c.cmd = exec.CommandContext(c.ctx, c.config.SignalCLIPath, args...)

		stdout, err := c.cmd.StdoutPipe()
		if err != nil {
			c.logger.Error("failed to get stdout pipe", zap.Error(err))
			c.setError(fmt.Sprintf("failed to start signal-cli: %v", err))
			time.Sleep(5 * time.Second)
			continue
		}

		stderr, err := c.cmd.StderrPipe()
		if err != nil {
			c.logger.Error("failed to get stderr pipe", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		if err := c.cmd.Start(); err != nil {
			c.logger.Error("failed to start signal-cli", zap.Error(err))
			c.setError(fmt.Sprintf("failed to start signal-cli: %v", err))
			time.Sleep(5 * time.Second)
			continue
		}

		// Mark as connected
		now := time.Now()
		c.mu.Lock()
		c.status = channel.StatusConnected
		c.connectedAt = &now
		c.isRegistered = true
		c.lastError = ""
		c.lastErrorAt = nil
		c.mu.Unlock()

		c.logger.Info("Signal channel connected")

		// Read stderr in background
		go func() {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.Contains(line, "ERROR") {
					c.logger.Error("signal-cli error", zap.String("message", line))
				} else {
					c.logger.Debug("signal-cli stderr", zap.String("message", line))
				}
			}
		}()

		// Read messages from stdout
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			c.processMessage(line)
		}

		if err := scanner.Err(); err != nil {
			c.logger.Error("error reading signal-cli output", zap.Error(err))
		}

		// Wait for process to exit
		if err := c.cmd.Wait(); err != nil {
			c.logger.Error("signal-cli exited with error", zap.Error(err))
		}

		c.mu.Lock()
		c.status = channel.StatusDisconnected
		c.mu.Unlock()

		// Check if we should restart
		select {
		case <-c.ctx.Done():
			return
		default:
			c.logger.Info("signal-cli exited, restarting...")
			time.Sleep(5 * time.Second)
		}
	}
}

// processMessage processes a JSON message from signal-cli.
func (c *Channel) processMessage(jsonLine string) {
	var msg SignalMessage
	if err := json.Unmarshal([]byte(jsonLine), &msg); err != nil {
		c.logger.Debug("failed to parse signal message",
			zap.Error(err),
			zap.String("line", jsonLine))
		return
	}

	// Handle data messages (incoming messages)
	if msg.Envelope.DataMessage != nil {
		c.handleDataMessage(&msg)
	}
}

// handleDataMessage processes an incoming data message.
func (c *Channel) handleDataMessage(msg *SignalMessage) {
	dm := msg.Envelope.DataMessage
	if dm.Message == "" {
		return
	}

	sender := msg.Envelope.SourceNumber
	if sender == "" {
		sender = msg.Envelope.Source
	}

	// Check if sender is allowed
	if !c.isSenderAllowed(sender) {
		c.logger.Debug("ignoring message from non-allowed sender",
			zap.String("sender", sender))
		return
	}

	// Determine if it's a group message
	isGroup := dm.GroupInfo != nil
	groupName := ""
	chatID := sender
	if isGroup {
		chatID = dm.GroupInfo.GroupID
		groupName = dm.GroupInfo.Name
	}

	// Convert timestamp (milliseconds to time.Time)
	timestamp := time.UnixMilli(dm.Timestamp)

	channelMsg := channel.Message{
		ID:          fmt.Sprintf("%d-%s", dm.Timestamp, sender),
		ChannelName: "signal",
		ChatID:      chatID,
		UserID:      sender,
		Username:    msg.Envelope.SourceName,
		Type:        channel.MessageTypeText,
		Content:     dm.Message,
		Timestamp:   timestamp,
		IsGroup:     isGroup,
		GroupName:   groupName,
		Metadata: map[string]interface{}{
			"source_uuid":   msg.Envelope.SourceUUID,
			"source_device": msg.Envelope.SourceDevice,
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

	// Kill the signal-cli process if running
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
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
	c.logger.Info("Signal channel stopped")
	return nil
}

// Send sends a message through Signal.
func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	c.mu.RLock()
	if !c.isRegistered {
		c.mu.RUnlock()
		return fmt.Errorf("Signal not registered")
	}
	c.mu.RUnlock()

	args := []string{
		"--config", c.config.ConfigPath,
		"-u", c.config.PhoneNumber,
		"send",
		"-m", msg.Content,
	}

	// Determine if sending to group or individual
	if strings.HasPrefix(msg.ChatID, "group.") || len(msg.ChatID) > 20 {
		// Assume it's a group ID
		args = append(args, "-g", msg.ChatID)
	} else {
		// Individual recipient
		args = append(args, msg.ChatID)
	}

	cmd := exec.CommandContext(ctx, c.config.SignalCLIPath, args...)
	cmd.Dir = c.config.ConfigPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		c.logger.Error("failed to send Signal message",
			zap.String("recipient", msg.ChatID),
			zap.String("output", string(output)),
			zap.Error(err))
		return fmt.Errorf("failed to send Signal message: %w", err)
	}

	c.logger.Debug("Signal message sent",
		zap.String("recipient", msg.ChatID),
		zap.Int("content_length", len(msg.Content)))

	return nil
}

// SendStreaming sends a message with streaming support.
// For Signal, we accumulate content and send as a single message.
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
		Name:         "signal",
		Type:         "signal",
		Status:       c.status,
		Enabled:      c.config.Enabled,
		ConnectedAt:  c.connectedAt,
		LastError:    c.lastError,
		LastErrorAt:  c.lastErrorAt,
		MessageCount: c.msgCount.Load(),
		Metadata: map[string]interface{}{
			"phone_number":  c.config.PhoneNumber,
			"is_registered": c.isRegistered,
			"is_linked":     c.isLinked,
			"config_path":   c.config.ConfigPath,
		},
	}

	return info
}

// IsConnected returns true if the channel is connected.
func (c *Channel) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status == channel.StatusConnected && c.isRegistered
}

// Messages returns the channel for receiving incoming messages.
func (c *Channel) Messages() <-chan channel.Message {
	return c.messages
}

// Register registers a new Signal account.
func (c *Channel) Register(ctx context.Context, captcha string) error {
	args := []string{
		"--config", c.config.ConfigPath,
		"-u", c.config.PhoneNumber,
		"register",
	}
	if captcha != "" {
		args = append(args, "--captcha", captcha)
	}

	cmd := exec.CommandContext(ctx, c.config.SignalCLIPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("registration failed: %s: %w", string(output), err)
	}

	c.logger.Info("Signal registration initiated", zap.String("output", string(output)))
	return nil
}

// Verify verifies the Signal account with the received code.
func (c *Channel) Verify(ctx context.Context, code string) error {
	args := []string{
		"--config", c.config.ConfigPath,
		"-u", c.config.PhoneNumber,
		"verify",
		code,
	}

	cmd := exec.CommandContext(ctx, c.config.SignalCLIPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("verification failed: %s: %w", string(output), err)
	}

	c.mu.Lock()
	c.isRegistered = true
	c.mu.Unlock()

	c.logger.Info("Signal verification successful")
	return nil
}

// Link links this device to an existing Signal account.
func (c *Channel) Link(ctx context.Context, deviceName string) (string, error) {
	args := []string{
		"--config", c.config.ConfigPath,
		"link",
		"-n", deviceName,
	}

	cmd := exec.CommandContext(ctx, c.config.SignalCLIPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("linking failed: %s: %w", string(output), err)
	}

	// The output should contain a URI for linking
	uri := strings.TrimSpace(string(output))
	c.logger.Info("Signal link URI generated", zap.String("uri", uri))
	return uri, nil
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

// normalizePhoneNumber removes common formatting from phone numbers.
func normalizePhoneNumber(phone string) string {
	// Remove common formatting characters
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")
	phone = strings.ReplaceAll(phone, "(", "")
	phone = strings.ReplaceAll(phone, ")", "")
	// Keep the + for international format
	return phone
}

// GetConfigPath returns the path to the Signal configuration directory.
func (c *Channel) GetConfigPath() string {
	return filepath.Join(c.config.ConfigPath, "data")
}

// SendImage sends an image message through Signal.
func (c *Channel) SendImage(ctx context.Context, chatID string, imagePath string, caption string) error {
	c.mu.RLock()
	if !c.isRegistered {
		c.mu.RUnlock()
		return fmt.Errorf("Signal not registered")
	}
	c.mu.RUnlock()

	args := []string{
		"--config", c.config.ConfigPath,
		"-u", c.config.PhoneNumber,
		"send",
		"-a", imagePath,
	}

	if caption != "" {
		args = append(args, "-m", caption)
	}

	// Determine if sending to group or individual
	if strings.HasPrefix(chatID, "group.") || len(chatID) > 20 {
		args = append(args, "-g", chatID)
	} else {
		args = append(args, chatID)
	}

	cmd := exec.CommandContext(ctx, c.config.SignalCLIPath, args...)
	cmd.Dir = c.config.ConfigPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		c.logger.Error("failed to send Signal image",
			zap.String("recipient", chatID),
			zap.String("image", imagePath),
			zap.String("output", string(output)),
			zap.Error(err))
		return fmt.Errorf("failed to send Signal image: %w", err)
	}

	c.logger.Debug("Signal image sent",
		zap.String("recipient", chatID),
		zap.String("image", imagePath))

	return nil
}

// SendFile sends a file/document message through Signal.
func (c *Channel) SendFile(ctx context.Context, chatID string, filePath string, caption string) error {
	c.mu.RLock()
	if !c.isRegistered {
		c.mu.RUnlock()
		return fmt.Errorf("Signal not registered")
	}
	c.mu.RUnlock()

	args := []string{
		"--config", c.config.ConfigPath,
		"-u", c.config.PhoneNumber,
		"send",
		"-a", filePath,
	}

	if caption != "" {
		args = append(args, "-m", caption)
	}

	// Determine if sending to group or individual
	if strings.HasPrefix(chatID, "group.") || len(chatID) > 20 {
		args = append(args, "-g", chatID)
	} else {
		args = append(args, chatID)
	}

	cmd := exec.CommandContext(ctx, c.config.SignalCLIPath, args...)
	cmd.Dir = c.config.ConfigPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		c.logger.Error("failed to send Signal file",
			zap.String("recipient", chatID),
			zap.String("file", filePath),
			zap.String("output", string(output)),
			zap.Error(err))
		return fmt.Errorf("failed to send Signal file: %w", err)
	}

	c.logger.Debug("Signal file sent",
		zap.String("recipient", chatID),
		zap.String("file", filePath))

	return nil
}

// SendImageData sends an image from raw data through Signal.
func (c *Channel) SendImageData(ctx context.Context, chatID string, imageData []byte, filename string, caption string) error {
	// Create a temporary file
	tmpDir := filepath.Join(c.config.ConfigPath, "tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	tmpFile := filepath.Join(tmpDir, filename)
	if err := os.WriteFile(tmpFile, imageData, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	defer os.Remove(tmpFile)

	return c.SendImage(ctx, chatID, tmpFile, caption)
}

// SendFileData sends a file from raw data through Signal.
func (c *Channel) SendFileData(ctx context.Context, chatID string, fileData []byte, filename string, caption string) error {
	// Create a temporary file
	tmpDir := filepath.Join(c.config.ConfigPath, "tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	tmpFile := filepath.Join(tmpDir, filename)
	if err := os.WriteFile(tmpFile, fileData, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	defer os.Remove(tmpFile)

	return c.SendFile(ctx, chatID, tmpFile, caption)
}

// SendWithAttachment sends a message with an attachment.
func (c *Channel) SendWithAttachment(ctx context.Context, msg channel.OutgoingMessage) error {
	if len(msg.Attachments) == 0 {
		return c.Send(ctx, msg)
	}

	for _, att := range msg.Attachments {
		var err error
		filename := att.Name
		if filename == "" {
			filename = fmt.Sprintf("attachment_%d", time.Now().UnixNano())
		}

		switch att.Type {
		case channel.MessageTypeImage:
			err = c.SendImageData(ctx, msg.ChatID, att.Data, filename, msg.Content)
		default:
			err = c.SendFileData(ctx, msg.ChatID, att.Data, filename, msg.Content)
		}
		if err != nil {
			return fmt.Errorf("failed to send attachment %s: %w", filename, err)
		}
	}

	return nil
}

// SendMultipleAttachments sends a message with multiple attachments.
func (c *Channel) SendMultipleAttachments(ctx context.Context, chatID string, attachmentPaths []string, caption string) error {
	c.mu.RLock()
	if !c.isRegistered {
		c.mu.RUnlock()
		return fmt.Errorf("Signal not registered")
	}
	c.mu.RUnlock()

	args := []string{
		"--config", c.config.ConfigPath,
		"-u", c.config.PhoneNumber,
		"send",
	}

	// Add all attachments
	for _, path := range attachmentPaths {
		args = append(args, "-a", path)
	}

	if caption != "" {
		args = append(args, "-m", caption)
	}

	// Determine if sending to group or individual
	if strings.HasPrefix(chatID, "group.") || len(chatID) > 20 {
		args = append(args, "-g", chatID)
	} else {
		args = append(args, chatID)
	}

	cmd := exec.CommandContext(ctx, c.config.SignalCLIPath, args...)
	cmd.Dir = c.config.ConfigPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		c.logger.Error("failed to send Signal attachments",
			zap.String("recipient", chatID),
			zap.Int("attachment_count", len(attachmentPaths)),
			zap.String("output", string(output)),
			zap.Error(err))
		return fmt.Errorf("failed to send Signal attachments: %w", err)
	}

	c.logger.Debug("Signal attachments sent",
		zap.String("recipient", chatID),
		zap.Int("attachment_count", len(attachmentPaths)))

	return nil
}
