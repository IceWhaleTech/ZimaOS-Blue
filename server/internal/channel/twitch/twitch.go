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
	Enabled      bool   `mapstructure:"enabled"`
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	OAuthToken   string `mapstructure:"oauth_token"`
	BotUsername  string `mapstructure:"bot_username"`
	Channels     string `mapstructure:"channels"`
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

func (c *Channel) Name() string                    { return "twitch" }
func (c *Channel) Type() string                    { return "twitch" }
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
	// IRCv3 tags are prefixed with @
	var tags map[string]string
	if strings.HasPrefix(line, "@") {
		idx := strings.Index(line, " ")
		if idx == -1 {
			return
		}
		tags = parseTags(line[1:idx])
		line = line[idx+1:]
	}

	// Format: :user!user@user.tmi.twitch.tv PRIVMSG #channel :message
	if !strings.Contains(line, "PRIVMSG") {
		return
	}

	parts := strings.SplitN(line, " ", 4)
	if len(parts) < 4 {
		return
	}

	prefix := parts[0]
	// command := parts[1] // PRIVMSG
	target := parts[2]
	text := strings.TrimPrefix(parts[3], ":")

	// Extract username from :user!user@user.tmi.twitch.tv
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
	}
	if displayName, ok := tags["display-name"]; ok && displayName != "" {
		metadata["display_name"] = displayName
	}
	if msgID, ok := tags["id"]; ok {
		metadata["twitch_msg_id"] = msgID
	}
	if color, ok := tags["color"]; ok {
		metadata["color"] = color
	}
	if badges, ok := tags["badges"]; ok {
		metadata["badges"] = badges
	}

	displayName := username
	if dn, ok := tags["display-name"]; ok && dn != "" {
		displayName = dn
	}

	msgID := tags["id"]
	if msgID == "" {
		msgID = fmt.Sprintf("twitch-%d", time.Now().UnixNano())
	}

	msg := channel.Message{
		ID:          msgID,
		ChannelName: "twitch",
		ChatID:      target,
		UserID:      username,
		Username:    displayName,
		Type:        channel.MessageTypeText,
		Content:     text,
		Timestamp:   time.Now(),
		IsGroup:     true,
		GroupName:   target,
		Metadata:    metadata,
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

	for _, chunk := range splitMessage(msg.Content, maxIRCMsgLen) {
		if err := c.sendRaw(fmt.Sprintf("PRIVMSG %s :%s", target, chunk)); err != nil {
			return fmt.Errorf("failed to send PRIVMSG: %w", err)
		}
	}

	c.msgsSent.Add(1)
	return nil
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
