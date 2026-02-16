// Package nextcloudtalk provides a Nextcloud Talk (Spreed) channel implementation.
package nextcloudtalk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/validator"
)

// Config contains Nextcloud Talk channel configuration.
type Config struct {
	Enabled     bool   `yaml:"enabled"`
	ServerURL   string `yaml:"server_url"`
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
	RoomToken   string `yaml:"room_token"`
}

const (
	chatAPIPath    = "/ocs/v2.php/apps/spreed/api/v1/chat/"
	userAPIPath    = "/ocs/v2.php/cloud/user"
	shareAPIPath   = "/ocs/v2.php/apps/files_sharing/api/v1/shares"
	pollInterval   = 3 * time.Second
)

// ocsResponse wraps the standard OCS API response envelope.
type ocsResponse struct {
	OCS struct {
		Meta struct {
			StatusCode int    `json:"statuscode"`
			Message    string `json:"message"`
		} `json:"meta"`
		Data json.RawMessage `json:"data"`
	} `json:"ocs"`
}

// chatMessage represents a Nextcloud Talk chat message.
type chatMessage struct {
	ID                int               `json:"id"`
	Token             string            `json:"token"`
	ActorType         string            `json:"actorType"`
	ActorID           string            `json:"actorId"`
	ActorDisplayName  string            `json:"actorDisplayName"`
	Timestamp         int64             `json:"timestamp"`
	Message           string            `json:"message"`
	MessageParameters map[string]interface{} `json:"messageParameters"`
	SystemMessage     string            `json:"systemMessage"`
	MessageType       string            `json:"messageType"`
	IsReplyable       bool              `json:"isReplyable"`
	ReferenceID       string            `json:"referenceId"`
}

// Channel implements the channel.Channel interface for Nextcloud Talk.
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

	lastKnownMsgID int

	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new Nextcloud Talk channel.
func New(cfg Config, logger *zap.Logger) *Channel {
	return &Channel{
		config:   cfg,
		logger:   logger.With(zap.String("channel", "nextcloudtalk")),
		client:   &http.Client{Timeout: 60 * time.Second},
		messages: make(chan channel.Message, 100),
		status:   channel.StatusDisconnected,
	}
}

func (c *Channel) Name() string                    { return "nextcloudtalk" }
func (c *Channel) Type() string                    { return "nextcloudtalk" }
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
	now := time.Now()
	c.status = channel.StatusConnected
	c.connectedAt = &now
	c.mu.Unlock()

	go c.pollMessages()

	c.logger.Info("Nextcloud Talk channel started",
		zap.String("server", c.config.ServerURL),
		zap.String("room", c.config.RoomToken))
	return nil
}

func (c *Channel) Stop(ctx context.Context) error {
	c.mu.Lock()
	c.status = channel.StatusDisconnected
	c.mu.Unlock()
	if c.cancel != nil {
		c.cancel()
	}
	close(c.messages)
	c.logger.Info("Nextcloud Talk channel stopped")
	return nil
}

// newOCSRequest creates an HTTP request with Nextcloud OCS headers and Basic Auth.
func (c *Channel) newOCSRequest(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.config.Username, c.config.Password)
	req.Header.Set("OCS-APIRequest", "true")
	req.Header.Set("Accept", "application/json")
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// pollMessages long-polls for new messages in the configured room.
func (c *Channel) pollMessages() {
	// Initial fetch to set lastKnownMsgID without processing old messages.
	c.initLastKnownMsgID()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		msgs, err := c.fetchMessages()
		if err != nil {
			c.logger.Warn("failed to poll messages", zap.Error(err))
			select {
			case <-c.ctx.Done():
				return
			case <-time.After(pollInterval):
			}
			continue
		}

		for _, m := range msgs {
			// Skip system messages and our own messages.
			if m.SystemMessage != "" || m.ActorID == c.config.Username {
				continue
			}
			c.processMessage(m)
		}

		select {
		case <-c.ctx.Done():
			return
		case <-time.After(pollInterval):
		}
	}
}

func (c *Channel) initLastKnownMsgID() {
	url := c.config.ServerURL + chatAPIPath + c.config.RoomToken + "?lookIntoFuture=0&limit=1"
	req, err := c.newOCSRequest(c.ctx, http.MethodGet, url, nil)
	if err != nil {
		return
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var ocs ocsResponse
	if err := json.NewDecoder(resp.Body).Decode(&ocs); err != nil {
		return
	}
	var msgs []chatMessage
	if err := json.Unmarshal(ocs.OCS.Data, &msgs); err != nil {
		return
	}
	if len(msgs) > 0 {
		c.lastKnownMsgID = msgs[len(msgs)-1].ID
	}
}

func (c *Channel) fetchMessages() ([]chatMessage, error) {
	url := c.config.ServerURL + chatAPIPath + c.config.RoomToken + "?lookIntoFuture=1&timeout=30"
	if c.lastKnownMsgID > 0 {
		url += "&lastKnownMessageId=" + strconv.Itoa(c.lastKnownMsgID)
	}

	req, err := c.newOCSRequest(c.ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to poll: %w", err)
	}
	defer resp.Body.Close()

	// 304 means no new messages.
	if resp.StatusCode == http.StatusNotModified {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OCS error (status %d): %s", resp.StatusCode, string(body))
	}

	var ocs ocsResponse
	if err := json.NewDecoder(resp.Body).Decode(&ocs); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var msgs []chatMessage
	if err := json.Unmarshal(ocs.OCS.Data, &msgs); err != nil {
		return nil, fmt.Errorf("failed to decode messages: %w", err)
	}

	if len(msgs) > 0 {
		c.lastKnownMsgID = msgs[len(msgs)-1].ID
	}
	return msgs, nil
}

func (c *Channel) processMessage(m chatMessage) {
	msg := channel.Message{
		ID:          strconv.Itoa(m.ID),
		ChannelName: "nextcloudtalk",
		ChatID:      m.Token,
		UserID:      m.ActorID,
		Username:    m.ActorDisplayName,
		Type:        channel.MessageTypeText,
		Content:     m.Message,
		Timestamp:   time.Unix(m.Timestamp, 0),
		IsGroup:     true, // Talk rooms are group conversations.
		GroupName:   m.Token,
		Metadata: map[string]interface{}{
			"actor_type":   m.ActorType,
			"is_replyable": m.IsReplyable,
			"reference_id": m.ReferenceID,
		},
	}

	c.msgCount.Add(1)
	c.msgsRecv.Add(1)

	select {
	case c.messages <- msg:
	default:
		c.logger.Warn("message channel full, dropping message", zap.String("id", msg.ID))
	}
}

// Send sends a message to the Nextcloud Talk room.
func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	// Send text message.
	if msg.Content != "" {
		if err := c.sendText(ctx, msg.ChatID, msg.Content, msg.ReplyToID); err != nil {
			return err
		}
	}

	// Share file attachments via Nextcloud file sharing API.
	for _, att := range msg.Attachments {
		if err := c.shareFile(ctx, msg.ChatID, att); err != nil {
			c.logger.Warn("failed to share file", zap.String("name", att.Name), zap.Error(err))
		}
	}

	c.msgsSent.Add(1)
	return nil
}

func (c *Channel) sendText(ctx context.Context, roomToken, text, replyToID string) error {
	if roomToken == "" {
		roomToken = c.config.RoomToken
	}

	payload := map[string]interface{}{
		"message": text,
	}
	if replyToID != "" {
		if id, err := strconv.Atoi(replyToID); err == nil {
			payload["replyTo"] = id
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	url := c.config.ServerURL + chatAPIPath + roomToken
	req, err := c.newOCSRequest(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("OCS error (status %d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// shareFile shares a file to the Talk room via the Nextcloud file sharing API.
func (c *Channel) shareFile(ctx context.Context, roomToken string, att channel.Attachment) error {
	if roomToken == "" {
		roomToken = c.config.RoomToken
	}

	// shareType 10 = share to Talk room.
	payload := map[string]interface{}{
		"shareType":  10,
		"shareWith":  roomToken,
		"path":       att.URL,
		"permissions": 1, // read-only
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal share request: %w", err)
	}

	url := c.config.ServerURL + shareAPIPath
	req, err := c.newOCSRequest(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create share request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to share file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("share API error (status %d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// SendStreaming accumulates streamed chunks and sends them as a single message.
func (c *Channel) SendStreaming(ctx context.Context, chatID string, replyToID string, content <-chan string, done chan<- struct{}) error {
	defer close(done)

	var fullContent strings.Builder
	chunkBuf := strings.Builder{}
	const chunkSize = 2000

	for chunk := range content {
		fullContent.WriteString(chunk)
		chunkBuf.WriteString(chunk)

		// Send in chunks for long streaming responses.
		if chunkBuf.Len() >= chunkSize {
			if err := c.sendText(ctx, chatID, chunkBuf.String(), replyToID); err != nil {
				c.logger.Warn("failed to send streaming chunk", zap.Error(err))
			}
			chunkBuf.Reset()
			replyToID = "" // Only reply to original for first chunk.
		}
	}

	// Send remaining content.
	if chunkBuf.Len() > 0 {
		if err := c.sendText(ctx, chatID, chunkBuf.String(), replyToID); err != nil {
			return err
		}
	}

	if fullContent.Len() > 0 {
		c.msgsSent.Add(1)
	}
	return nil
}

func (c *Channel) Info() channel.Info {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return channel.Info{
		Name: "nextcloudtalk", Type: "nextcloudtalk", Status: c.status, Enabled: c.config.Enabled,
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

// Validator for Nextcloud Talk configuration.
type Validator struct{}

func NewValidator() *Validator { return &Validator{} }

func (v *Validator) Validate(ctx context.Context, config map[string]string) validator.Result {
	serverURL := config["server_url"]
	username := config["username"]
	password := config["password"]

	if serverURL == "" {
		return validator.Result{Success: false, Error: "server_url is required"}
	}
	if username == "" {
		return validator.Result{Success: false, Error: "username is required"}
	}
	if password == "" {
		return validator.Result{Success: false, Error: "password is required"}
	}

	// Validate credentials via GET /ocs/v2.php/cloud/user
	client := &http.Client{Timeout: validator.DefaultTimeout}
	url := strings.TrimRight(serverURL, "/") + userAPIPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to create request: %v", err)}
	}
	req.SetBasicAuth(username, password)
	req.Header.Set("OCS-APIRequest", "true")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to connect to Nextcloud: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return validator.Result{Success: false, Error: fmt.Sprintf("Nextcloud API returned status %d", resp.StatusCode)}
	}

	var ocs ocsResponse
	if err := json.NewDecoder(resp.Body).Decode(&ocs); err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to parse response: %v", err)}
	}

	var userData struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayname"`
	}
	if err := json.Unmarshal(ocs.OCS.Data, &userData); err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to parse user data: %v", err)}
	}

	return validator.Result{
		Success:    true,
		MessageKey: "channels.connectionSuccess",
		Data: map[string]interface{}{
			"user_id":      userData.ID,
			"display_name": userData.DisplayName,
			"server_url":   serverURL,
		},
	}
}
