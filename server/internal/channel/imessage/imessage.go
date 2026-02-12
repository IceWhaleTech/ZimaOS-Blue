// Package imessage provides an iMessage channel implementation for macOS.
// This channel uses the Messages.app database and AppleScript for sending messages.
// Note: This only works on macOS with Messages.app configured.
package imessage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

// Channel implements the channel.Channel interface for iMessage.
type Channel struct {
	config   Config
	logger   *zap.Logger
	db       *sql.DB
	messages chan channel.Message

	mu          sync.RWMutex
	status      channel.Status
	connectedAt *time.Time
	lastError   string
	lastErrorAt *time.Time
	msgCount    atomic.Int64

	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	lastRowID  int64
	processedIDs map[int64]bool
}

// Config contains iMessage channel configuration.
type Config struct {
	Enabled        bool     `mapstructure:"enabled"`
	DatabasePath   string   `mapstructure:"database_path"`
	PollInterval   int      `mapstructure:"poll_interval_ms"`
	AllowedNumbers []string `mapstructure:"allowed_numbers"`
	AllowedEmails  []string `mapstructure:"allowed_emails"`
}

// DefaultConfig returns the default iMessage configuration.
func DefaultConfig() Config {
	homeDir, _ := os.UserHomeDir()
	return Config{
		Enabled:      false,
		DatabasePath: filepath.Join(homeDir, "Library", "Messages", "chat.db"),
		PollInterval: 1000,
	}
}

// New creates a new iMessage channel.
func New(cfg Config, logger *zap.Logger) *Channel {
	return &Channel{
		config:       cfg,
		logger:       logger.With(zap.String("channel", "imessage")),
		messages:     make(chan channel.Message, 100),
		status:       channel.StatusDisconnected,
		processedIDs: make(map[int64]bool),
	}
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "imessage"
}

// Type returns the channel type.
func (c *Channel) Type() string {
	return "imessage"
}

// Start initializes and starts the iMessage channel.
func (c *Channel) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.status == channel.StatusConnected || c.status == channel.StatusConnecting {
		c.mu.Unlock()
		return fmt.Errorf("channel already started")
	}
	c.status = channel.StatusConnecting
	c.mu.Unlock()

	// Check if running on macOS
	if !isMacOS() {
		c.setError("iMessage channel only works on macOS")
		return fmt.Errorf("iMessage channel only works on macOS")
	}

	// Check if database exists
	if _, err := os.Stat(c.config.DatabasePath); os.IsNotExist(err) {
		c.setError(fmt.Sprintf("Messages database not found: %s", c.config.DatabasePath))
		return fmt.Errorf("Messages database not found: %s", c.config.DatabasePath)
	}

	// Open database connection (read-only)
	db, err := sql.Open("sqlite3", c.config.DatabasePath+"?mode=ro")
	if err != nil {
		c.setError(fmt.Sprintf("failed to open database: %v", err))
		return fmt.Errorf("failed to open Messages database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		c.setError(fmt.Sprintf("failed to connect to database: %v", err))
		return fmt.Errorf("failed to connect to Messages database: %w", err)
	}

	c.db = db
	c.ctx, c.cancel = context.WithCancel(ctx)

	// Get the latest message ID to start from
	c.lastRowID, err = c.getLatestMessageID()
	if err != nil {
		c.logger.Warn("failed to get latest message ID, starting from 0", zap.Error(err))
		c.lastRowID = 0
	}

	// Start polling for new messages
	c.wg.Add(1)
	go c.pollMessages()

	now := time.Now()
	c.mu.Lock()
	c.status = channel.StatusConnected
	c.connectedAt = &now
	c.lastError = ""
	c.lastErrorAt = nil
	c.mu.Unlock()

	c.logger.Info("iMessage channel started",
		zap.String("database", c.config.DatabasePath),
		zap.Int64("starting_row_id", c.lastRowID))
	return nil
}

// getLatestMessageID returns the latest message ROWID from the database.
func (c *Channel) getLatestMessageID() (int64, error) {
	var rowID int64
	err := c.db.QueryRow("SELECT MAX(ROWID) FROM message").Scan(&rowID)
	if err != nil {
		return 0, err
	}
	return rowID, nil
}

// pollMessages polls the Messages database for new messages.
func (c *Channel) pollMessages() {
	defer c.wg.Done()

	pollInterval := time.Duration(c.config.PollInterval) * time.Millisecond
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Debug("stopping message polling")
			return
		case <-ticker.C:
			c.checkNewMessages()
		}
	}
}

// checkNewMessages checks for new messages in the database.
func (c *Channel) checkNewMessages() {
	query := `
		SELECT
			m.ROWID,
			m.guid,
			m.text,
			m.date,
			m.is_from_me,
			m.handle_id,
			h.id as sender_id,
			COALESCE(c.display_name, '') as chat_name,
			c.chat_identifier,
			CASE WHEN c.chat_identifier LIKE 'chat%' THEN 1 ELSE 0 END as is_group
		FROM message m
		LEFT JOIN handle h ON m.handle_id = h.ROWID
		LEFT JOIN chat_message_join cmj ON m.ROWID = cmj.message_id
		LEFT JOIN chat c ON cmj.chat_id = c.ROWID
		WHERE m.ROWID > ?
		AND m.is_from_me = 0
		ORDER BY m.ROWID ASC
		LIMIT 100
	`

	rows, err := c.db.Query(query, c.lastRowID)
	if err != nil {
		c.logger.Error("failed to query messages", zap.Error(err))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var (
			rowID          int64
			guid           string
			text           sql.NullString
			date           int64
			isFromMe       int
			handleID       sql.NullInt64
			senderID       sql.NullString
			chatName       string
			chatIdentifier sql.NullString
			isGroup        int
		)

		err := rows.Scan(&rowID, &guid, &text, &date, &isFromMe, &handleID, &senderID, &chatName, &chatIdentifier, &isGroup)
		if err != nil {
			c.logger.Error("failed to scan message row", zap.Error(err))
			continue
		}

		// Skip if already processed
		if c.processedIDs[rowID] {
			continue
		}
		c.processedIDs[rowID] = true

		// Update last row ID
		if rowID > c.lastRowID {
			c.lastRowID = rowID
		}

		// Skip empty messages
		if !text.Valid || text.String == "" {
			continue
		}

		// Check if sender is allowed
		sender := ""
		if senderID.Valid {
			sender = senderID.String
		}
		if !c.isSenderAllowed(sender) {
			c.logger.Debug("ignoring message from non-allowed sender",
				zap.String("sender", sender))
			continue
		}

		// Convert macOS timestamp (nanoseconds since 2001-01-01) to time.Time
		timestamp := convertMacOSTimestamp(date)

		// Create unified message
		chatID := ""
		if chatIdentifier.Valid {
			chatID = chatIdentifier.String
		} else if senderID.Valid {
			chatID = senderID.String
		}

		msg := channel.Message{
			ID:          guid,
			ChannelName: "imessage",
			ChatID:      chatID,
			UserID:      sender,
			Username:    sender,
			Type:        channel.MessageTypeText,
			Content:     text.String,
			Timestamp:   timestamp,
			IsGroup:     isGroup == 1,
			GroupName:   chatName,
			Metadata: map[string]interface{}{
				"row_id":    rowID,
				"handle_id": handleID.Int64,
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

	// Clean up old processed IDs to prevent memory leak
	if len(c.processedIDs) > 10000 {
		newProcessed := make(map[int64]bool)
		for id := range c.processedIDs {
			if id > c.lastRowID-1000 {
				newProcessed[id] = true
			}
		}
		c.processedIDs = newProcessed
	}
}

// isSenderAllowed checks if a sender is allowed to interact with the bot.
func (c *Channel) isSenderAllowed(sender string) bool {
	// If no restrictions, allow all
	if len(c.config.AllowedNumbers) == 0 && len(c.config.AllowedEmails) == 0 {
		return true
	}

	// Check phone numbers
	for _, allowed := range c.config.AllowedNumbers {
		if normalizePhoneNumber(sender) == normalizePhoneNumber(allowed) {
			return true
		}
	}

	// Check emails
	for _, allowed := range c.config.AllowedEmails {
		if strings.EqualFold(sender, allowed) {
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

	if c.db != nil {
		c.db.Close()
	}

	close(c.messages)
	c.logger.Info("iMessage channel stopped")
	return nil
}

// Send sends a message through iMessage using AppleScript.
func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	if !isMacOS() {
		return fmt.Errorf("iMessage sending only works on macOS")
	}

	// Escape content for AppleScript
	content := escapeAppleScript(msg.Content)
	recipient := msg.ChatID

	// Build AppleScript command
	script := fmt.Sprintf(`
		tell application "Messages"
			set targetService to 1st service whose service type = iMessage
			set targetBuddy to buddy "%s" of targetService
			send "%s" to targetBuddy
		end tell
	`, recipient, content)

	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.logger.Error("failed to send iMessage",
			zap.String("recipient", recipient),
			zap.String("output", string(output)),
			zap.Error(err))
		return fmt.Errorf("failed to send iMessage: %w", err)
	}

	c.logger.Debug("iMessage sent",
		zap.String("recipient", recipient),
		zap.Int("content_length", len(msg.Content)))

	return nil
}

// SendStreaming sends a message with streaming support.
// For iMessage, we accumulate the content and send as a single message.
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
						ChatID:  chatID,
						Content: fullContent.String(),
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
		Name:         "imessage",
		Type:         "imessage",
		Status:       c.status,
		Enabled:      c.config.Enabled,
		ConnectedAt:  c.connectedAt,
		LastError:    c.lastError,
		LastErrorAt:  c.lastErrorAt,
		MessageCount: c.msgCount.Load(),
		Metadata: map[string]interface{}{
			"database_path": c.config.DatabasePath,
			"last_row_id":   c.lastRowID,
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

// isMacOS checks if the current OS is macOS.
func isMacOS() bool {
	return os.Getenv("GOOS") == "darwin" || fileExists("/System/Library/CoreServices/SystemVersion.plist")
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// convertMacOSTimestamp converts macOS timestamp to time.Time.
// macOS timestamps are nanoseconds since 2001-01-01 00:00:00 UTC.
func convertMacOSTimestamp(timestamp int64) time.Time {
	// macOS epoch is 2001-01-01 00:00:00 UTC
	macEpoch := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	// Convert nanoseconds to duration and add to epoch
	return macEpoch.Add(time.Duration(timestamp) * time.Nanosecond)
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

// escapeAppleScript escapes a string for use in AppleScript.
func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}
