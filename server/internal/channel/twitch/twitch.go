// Package twitch provides a Twitch IRC channel implementation.
package twitch

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/validator"
)

const (
	ircServer      = "irc.chat.twitch.tv:6697"
	validateURL    = "https://id.twitch.tv/oauth2/validate"
	maxIRCMsgLen   = 500
	reconnectDelay = 5 * time.Second
)

// Config contains Twitch channel configuration.
type Config struct {
	Enabled      bool   `yaml:"enabled"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	OAuthToken   string `yaml:"oauth_token"`
	BotUsername  string `yaml:"bot_username"`
	Channels     string `yaml:"channels"`
}

// Channel implements the channel.Channel interface for Twitch IRC.
type Channel struct {
	config   Config
	logger   *zap.Logger
	client   *http.Client
	messages chan channel.Message

	mu          sync.RWMutex
	status      channel.Status
	connectedAt *time.Time
	lastError   string
	lastErrorAt *time.Time
	msgCount    atomic.Int64
	msgsSent    atomic.Int64
	msgsRecv    atomic.Int64

	conn   net.Conn
	writer *bufio.Writer
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new Twitch channel.
func New(cfg Config, logger *zap.Logger) *Channel {
	return &Channel{
		config:   cfg,
		logger:   logger.With(zap.String("channel", "twitch")),
		client:   &http.Client{Timeout: 10 * time.Second},
		messages: make(chan channel.Message, 100),
		status:   channel.StatusDisconnected,
	}
}

func (c *Channel) Name() string                     { return "twitch" }
func (c *Channel) Type() string                     { return "twitch" }
func (c *Channel) Messages() <-chan channel.Message { return c.messages }

func (c *Channel) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status == channel.StatusConnected
}

func (c *Channel) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.status == channel.StatusConnected {
		c.mu.Unlock()
		return fmt.Errorf("channel already started")
	}
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.status = channel.StatusConnecting
	c.mu.Unlock()

	if err := c.connect(); err != nil {
		c.setError(fmt.Sprintf("failed to connect: %v", err))
		return err
	}

	c.mu.Lock()
	now := time.Now()
	c.status = channel.StatusConnected
	c.connectedAt = &now
	c.mu.Unlock()

	go c.readLoop()
	c.logger.Info("Twitch channel started", zap.String("channels", c.config.Channels))
	return nil
}

func (c *Channel) connect() error {
	tlsConn, err := tls.Dial("tcp", ircServer, &tls.Config{MinVersion: tls.VersionTLS12})
	if err != nil {
		return fmt.Errorf("TLS dial failed: %w", err)
	}
	c.conn = tlsConn
	c.writer = bufio.NewWriter(tlsConn)

	token := c.config.OAuthToken
	if !strings.HasPrefix(token, "oauth:") {
		token = "oauth:" + token
	}

	if err := c.sendRaw("PASS " + token); err != nil {
		c.conn.Close()
		return fmt.Errorf("PASS failed: %w", err)
	}
	if err := c.sendRaw("NICK " + c.config.BotUsername); err != nil {
		c.conn.Close()
		return fmt.Errorf("NICK failed: %w", err)
	}

	// Request tags capability for emote metadata
	if err := c.sendRaw("CAP REQ :twitch.tv/tags twitch.tv/commands"); err != nil {
		c.conn.Close()
		return fmt.Errorf("CAP REQ failed: %w", err)
	}

	// Join configured channels
	for _, ch := range c.channelList() {
		if ch == "" {
			continue
		}
		if !strings.HasPrefix(ch, "#") {
			ch = "#" + ch
		}
		if err := c.sendRaw("JOIN " + ch); err != nil {
			c.logger.Warn("failed to join channel", zap.String("target", ch), zap.Error(err))
		}
	}
	return nil
}

func (c *Channel) channelList() []string {
	var channels []string
	for _, ch := range strings.Split(c.config.Channels, ",") {
		ch = strings.TrimSpace(ch)
		if ch != "" {
			channels = append(channels, strings.ToLower(ch))
		}
	}
	return channels
}

func (c *Channel) sendRaw(line string) error {
	_, err := c.writer.WriteString(line + "\r\n")
	if err != nil {
		return err
	}
	return c.writer.Flush()
}

func (c *Channel) readLoop() {
	scanner := bufio.NewScanner(c.conn)
	for scanner.Scan() {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		if strings.HasPrefix(line, "PING") {
			pongPayload := strings.TrimPrefix(line, "PING")
			_ = c.sendRaw("PONG" + pongPayload)
			continue
		}
		c.parseLine(line)
	}

	if err := scanner.Err(); err != nil {
		select {
		case <-c.ctx.Done():
			return
		default:
			c.logger.Error("IRC read error", zap.Error(err))
			c.setError(fmt.Sprintf("read error: %v", err))
			go c.reconnect()
		}
	}
}

func (c *Channel) reconnect() {
	c.mu.Lock()
	c.status = channel.StatusReconnecting
	c.mu.Unlock()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-time.After(reconnectDelay):
		}

		c.logger.Info("attempting reconnect")
		if c.conn != nil {
			c.conn.Close()
		}
		if err := c.connect(); err != nil {
			c.logger.Warn("reconnect failed", zap.Error(err))
			continue
		}

		c.mu.Lock()
		now := time.Now()
		c.status = channel.StatusConnected
		c.connectedAt = &now
		c.lastError = ""
		c.mu.Unlock()

		go c.readLoop()
		c.logger.Info("reconnected successfully")
		return
	}
}

// parseLine handles a raw IRC line, extracting PRIVMSG into channel.Message.
func (c *Channel) parseLine(line string) {
	var tags map[string]string
	if strings.HasPrefix(line, "@") {
		idx := strings.Index(line, " ")
		if idx == -1 {
			return
		}
		tags = parseTags(line[1:idx])
		line = line[idx+1:]
	}

	if !strings.Contains(line, "PRIVMSG") {
		return
	}

	parts := strings.SplitN(line, " ", 4)
	if len(parts) < 4 {
		return
	}

	prefix := parts[0]
	target := parts[2]
	text := strings.TrimPrefix(parts[3], ":")

	username := ""
	if strings.HasPrefix(prefix, ":") {
		prefix = prefix[1:]
	}
	if bangIdx := strings.Index(prefix, "!"); bangIdx != -1 {
		username = prefix[:bangIdx]
	}

	metadata := make(map[string]interface{})
	if emotes, ok := tags["emotes"]; ok && emotes != "" {
		metadata["emotes"] = emotes
		if parsedEmotes := parseTwitchEmotes(emotes); len(parsedEmotes) > 0 {
			metadata["emote_ranges"] = parsedEmotes
		}
	}
	if displayName, ok := tags["display-name"]; ok && displayName != "" {
		metadata["display_name"] = displayName
	}
	if msgID, ok := tags["id"]; ok && msgID != "" {
		metadata["twitch_msg_id"] = msgID
	}
	if color, ok := tags["color"]; ok && color != "" {
		metadata["color"] = color
	}
	if badges, ok := tags["badges"]; ok && badges != "" {
		metadata["badges"] = badges
	}
	if roomID, ok := tags["room-id"]; ok && roomID != "" {
		metadata["room_id"] = roomID
	}
	if messageType, ok := tags["message-type"]; ok && messageType != "" {
		metadata["message_type"] = messageType
	}
	if firstMsg, ok := tags["first-msg"]; ok && firstMsg != "" {
		metadata["first_msg"] = firstMsg == "1"
	}
	if replyParentMsgID, ok := tags["reply-parent-msg-id"]; ok && replyParentMsgID != "" {
		metadata["reply_parent_msg_id"] = replyParentMsgID
	}
	if replyParentUserID, ok := tags["reply-parent-user-id"]; ok && replyParentUserID != "" {
		metadata["reply_parent_user_id"] = replyParentUserID
	}
	if replyParentUserLogin, ok := tags["reply-parent-user-login"]; ok && replyParentUserLogin != "" {
		metadata["reply_parent_user_login"] = replyParentUserLogin
	}
	if replyParentDisplayName, ok := tags["reply-parent-display-name"]; ok && replyParentDisplayName != "" {
		metadata["reply_parent_display_name"] = replyParentDisplayName
	}
	if replyParentMsgBody, ok := tags["reply-parent-msg-body"]; ok && replyParentMsgBody != "" {
		metadata["reply_parent_msg_body"] = replyParentMsgBody
	}

	displayName := username
	if dn, ok := tags["display-name"]; ok && dn != "" {
		displayName = dn
	}

	msgID := tags["id"]
	if msgID == "" {
		msgID = fmt.Sprintf("twitch-%d", time.Now().UnixNano())
	}

	userID := username
	if rawUserID, ok := tags["user-id"]; ok && rawUserID != "" {
		userID = rawUserID
	}

	msg := channel.Message{
		ID:          msgID,
		ChannelName: "twitch",
		ChatID:      target,
		UserID:      userID,
		Username:    displayName,
		Type:        channel.MessageTypeText,
		Content:     text,
		Timestamp:   twitchMessageTimestamp(tags),
		IsGroup:     true,
		GroupName:   target,
		Metadata:    metadata,
	}
	if replyParentMsgID, ok := tags["reply-parent-msg-id"]; ok && replyParentMsgID != "" {
		msg.ReplyToID = replyParentMsgID
	}

	c.msgCount.Add(1)
	c.msgsRecv.Add(1)

	select {
	case c.messages <- msg:
	default:
		c.logger.Warn("message channel full, dropping message", zap.String("id", msg.ID))
	}
}

func parseTags(raw string) map[string]string {
	tags := make(map[string]string)
	for _, pair := range strings.Split(raw, ";") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			tags[kv[0]] = kv[1]
		}
	}
	return tags
}

func twitchMessageTimestamp(tags map[string]string) time.Time {
	if tags != nil {
		if raw, ok := tags["tmi-sent-ts"]; ok && strings.TrimSpace(raw) != "" {
			if millis, err := time.ParseDuration(strings.TrimSpace(raw) + "ms"); err == nil {
				return time.Unix(0, millis.Nanoseconds())
			}
		}
	}
	return time.Now()
}

func parseTwitchEmotes(raw string) []map[string]interface{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	items := make([]map[string]interface{}, 0)
	for _, segment := range strings.Split(raw, "/") {
		parts := strings.SplitN(strings.TrimSpace(segment), ":", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			continue
		}
		ranges := make([]map[string]int, 0)
		for _, loc := range strings.Split(parts[1], ",") {
			bounds := strings.SplitN(strings.TrimSpace(loc), "-", 2)
			if len(bounds) != 2 {
				continue
			}
			var start, end int
			if _, err := fmt.Sscanf(bounds[0], "%d", &start); err != nil {
				continue
			}
			if _, err := fmt.Sscanf(bounds[1], "%d", &end); err != nil {
				continue
			}
			ranges = append(ranges, map[string]int{"start": start, "end": end})
		}
		item := map[string]interface{}{"id": parts[0]}
		if len(ranges) > 0 {
			item["ranges"] = ranges
		}
		items = append(items, item)
	}
	return items
}

// Send sends a PRIVMSG to the target channel.
func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	c.mu.RLock()
	if c.status != channel.StatusConnected {
		c.mu.RUnlock()
		return fmt.Errorf("not connected")
	}
	c.mu.RUnlock()

	target := msg.ChatID
	if !strings.HasPrefix(target, "#") {
		target = "#" + target
	}

	textParts := make([]string, 0, 1+len(msg.Attachments))
	if strings.TrimSpace(msg.Content) != "" {
		textParts = append(textParts, strings.TrimSpace(msg.Content))
	}
	for _, att := range msg.Attachments {
		if fallback := twitchAttachmentFallbackText(att); fallback != "" {
			textParts = append(textParts, fallback)
		}
	}
	content := strings.Join(textParts, "\n")
	if content == "" {
		return fmt.Errorf("no sendable Twitch content")
	}

	for _, chunk := range splitMessage(content, maxIRCMsgLen) {
		if err := c.sendRaw(fmt.Sprintf("PRIVMSG %s :%s", target, chunk)); err != nil {
			return fmt.Errorf("failed to send PRIVMSG: %w", err)
		}
	}

	c.msgsSent.Add(1)
	return nil
}

func twitchAttachmentFallbackText(att channel.Attachment) string {
	parts := make([]string, 0, 2)
	if strings.TrimSpace(att.Name) != "" {
		parts = append(parts, strings.TrimSpace(att.Name))
	}
	if strings.TrimSpace(att.URL) != "" {
		parts = append(parts, strings.TrimSpace(att.URL))
	}
	if len(parts) > 0 {
		return strings.Join(parts, "\n")
	}
	if len(att.Data) == 0 {
		return ""
	}
	switch att.Type {
	case channel.MessageTypeImage:
		return "Image attachment"
	case channel.MessageTypeVideo:
		return "Video attachment"
	case channel.MessageTypeAudio:
		return "Audio attachment"
	default:
		return "File attachment"
	}
}

// SendStreaming collects streamed chunks and sends them as chunked IRC messages.
func (c *Channel) SendStreaming(ctx context.Context, chatID string, replyToID string, content <-chan string, done chan<- struct{}) error {
	defer close(done)

	var buf strings.Builder
	for chunk := range content {
		buf.WriteString(chunk)
		// Flush when buffer exceeds limit to stream in real time
		if buf.Len() >= maxIRCMsgLen {
			if err := c.Send(ctx, channel.OutgoingMessage{ChatID: chatID, Content: buf.String()}); err != nil {
				return err
			}
			buf.Reset()
		}
	}
	// Send remaining content
	if buf.Len() > 0 {
		return c.Send(ctx, channel.OutgoingMessage{ChatID: chatID, Content: buf.String()})
	}
	return nil
}

// splitMessage splits text into chunks of at most maxLen bytes.
func splitMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}
	var chunks []string
	for len(text) > 0 {
		end := maxLen
		if end > len(text) {
			end = len(text)
		}
		chunks = append(chunks, text[:end])
		text = text[end:]
	}
	return chunks
}

func (c *Channel) Stop(ctx context.Context) error {
	c.mu.Lock()
	c.status = channel.StatusDisconnected
	c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}
	if c.conn != nil {
		_ = c.sendRaw("QUIT :bye")
		c.conn.Close()
	}
	close(c.messages)
	c.logger.Info("Twitch channel stopped")
	return nil
}

func (c *Channel) Info() channel.Info {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return channel.Info{
		Name: "twitch", Type: "twitch", Status: c.status, Enabled: c.config.Enabled,
		ConnectedAt: c.connectedAt, LastError: c.lastError,
		MessageCount: c.msgCount.Load(), MessagesReceived: c.msgsRecv.Load(),
		MessagesSent: c.msgsSent.Load(),
	}
}

func (c *Channel) setError(errMsg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastError = errMsg
	now := time.Now()
	c.lastErrorAt = &now
	c.status = channel.StatusError
}

// Validator for Twitch configuration.
type Validator struct{}

func NewValidator() *Validator { return &Validator{} }

func (v *Validator) Validate(ctx context.Context, config map[string]string) validator.Result {
	token := config["oauth_token"]
	if token == "" {
		return validator.Result{Success: false, Error: "oauth_token is required"}
	}
	if config["bot_username"] == "" {
		return validator.Result{Success: false, Error: "bot_username is required"}
	}
	if config["channels"] == "" {
		return validator.Result{Success: false, Error: "channels is required"}
	}

	// Validate token against Twitch API
	cleanToken := strings.TrimPrefix(token, "oauth:")
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, validateURL, nil)
	if err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to create request: %v", err)}
	}
	req.Header.Set("Authorization", "OAuth "+cleanToken)

	resp, err := client.Do(req)
	if err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to validate token: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return validator.Result{Success: false, Error: fmt.Sprintf("token validation failed (status %d): %s", resp.StatusCode, string(body))}
	}

	var info struct {
		Login    string `json:"login"`
		UserID   string `json:"user_id"`
		ClientID string `json:"client_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to parse validation response: %v", err)}
	}

	return validator.Result{
		Success:    true,
		MessageKey: "channels.connectionSuccess",
		Data: map[string]interface{}{
			"login":     info.Login,
			"user_id":   info.UserID,
			"client_id": info.ClientID,
		},
	}
}
