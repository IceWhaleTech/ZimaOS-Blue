// Package teams provides a Microsoft Teams bot channel implementation.
package teams

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel"
)

const (
	// DefaultOAuthURL is the default Microsoft OAuth endpoint.
	DefaultOAuthURL = "https://login.microsoftonline.com"
	// BotFrameworkScope is the scope for Bot Framework API.
	BotFrameworkScope = "https://api.botframework.com/.default"
)

// Channel implements the channel.Channel interface for Microsoft Teams.
type Channel struct {
	config   channel.TeamsConfig
	logger   *zap.Logger
	messages chan channel.Message

	mu          sync.RWMutex
	status      channel.Status
	connectedAt *time.Time
	lastError   string
	lastErrorAt *time.Time
	msgCount    atomic.Int64

	// OAuth token management
	accessToken     string
	tokenExpiry     time.Time
	oauthBaseURL    string
	httpClient      *http.Client

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a new Teams channel.
func New(cfg channel.TeamsConfig, logger *zap.Logger) *Channel {
	return NewWithOptions(cfg, logger, DefaultOAuthURL)
}

// NewWithOptions creates a new Teams channel with custom options.
func NewWithOptions(cfg channel.TeamsConfig, logger *zap.Logger, oauthBaseURL string) *Channel {
	if oauthBaseURL == "" {
		oauthBaseURL = DefaultOAuthURL
	}
	return &Channel{
		config:       cfg,
		logger:       logger.With(zap.String("channel", "teams")),
		messages:     make(chan channel.Message, 100),
		status:       channel.StatusDisconnected,
		oauthBaseURL: oauthBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "teams"
}

// Type returns the channel type.
func (c *Channel) Type() string {
	return "teams"
}

// Start initializes and starts the Teams channel.
func (c *Channel) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.status == channel.StatusConnected || c.status == channel.StatusConnecting {
		c.mu.Unlock()
		return fmt.Errorf("channel already started")
	}
	c.status = channel.StatusConnecting
	c.mu.Unlock()

	c.ctx, c.cancel = context.WithCancel(ctx)

	// Acquire OAuth token
	if err := c.refreshToken(ctx); err != nil {
		c.setError(fmt.Sprintf("failed to acquire token: %v", err))
		return fmt.Errorf("failed to acquire OAuth token: %w", err)
	}

	// Start token refresh goroutine
	c.wg.Add(1)
	go c.tokenRefreshLoop()

	now := time.Now()
	c.mu.Lock()
	c.status = channel.StatusConnected
	c.connectedAt = &now
	c.lastError = ""
	c.lastErrorAt = nil
	c.mu.Unlock()

	c.logger.Info("teams channel started")
	return nil
}

// tokenRefreshLoop periodically refreshes the OAuth token.
func (c *Channel) tokenRefreshLoop() {
	defer c.wg.Done()

	// Refresh token 5 minutes before expiry
	refreshBuffer := 5 * time.Minute

	for {
		c.mu.RLock()
		expiry := c.tokenExpiry
		c.mu.RUnlock()

		// Calculate time until refresh
		refreshTime := expiry.Add(-refreshBuffer)
		sleepDuration := time.Until(refreshTime)
		if sleepDuration < 0 {
			sleepDuration = time.Minute // Minimum sleep
		}

		select {
		case <-c.ctx.Done():
			return
		case <-time.After(sleepDuration):
			if err := c.refreshToken(c.ctx); err != nil {
				c.logger.Error("failed to refresh token", zap.Error(err))
				c.setError(fmt.Sprintf("token refresh failed: %v", err))
			}
		}
	}
}

// refreshToken acquires or refreshes the OAuth token.
func (c *Channel) refreshToken(ctx context.Context) error {
	tenantID := c.config.TenantID
	if tenantID == "" {
		tenantID = "botframework.com"
	}

	tokenURL := fmt.Sprintf("%s/%s/oauth2/v2.0/token", c.oauthBaseURL, tenantID)

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", c.config.AppID)
	data.Set("client_secret", c.config.AppPassword)
	data.Set("scope", BotFrameworkScope)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		json.Unmarshal(body, &errResp)
		return fmt.Errorf("OAuth error: %s - %s", errResp.Error, errResp.ErrorDescription)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return fmt.Errorf("failed to parse token response: %w", err)
	}

	c.mu.Lock()
	c.accessToken = tokenResp.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	c.mu.Unlock()

	c.logger.Debug("OAuth token refreshed", zap.Time("expiry", c.tokenExpiry))
	return nil
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
	c.logger.Info("teams channel stopped")
	return nil
}

// Send sends a message through Teams.
func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	c.mu.RLock()
	token := c.accessToken
	c.mu.RUnlock()

	if token == "" {
		return fmt.Errorf("channel not initialized")
	}

	// Parse conversation reference from ChatID
	// ChatID format: serviceUrl|conversationId
	parts := strings.SplitN(msg.ChatID, "|", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid chat ID format, expected 'serviceUrl|conversationId'")
	}

	serviceURL := parts[0]
	conversationID := parts[1]

	// Build activity
	activity := map[string]interface{}{
		"type": "message",
		"text": msg.Content,
	}

	if msg.ReplyToID != "" {
		activity["replyToId"] = msg.ReplyToID
	}

	// Send to Bot Framework
	apiURL := fmt.Sprintf("%s/v3/conversations/%s/activities", serviceURL, conversationID)

	activityJSON, err := json.Marshal(activity)
	if err != nil {
		return fmt.Errorf("failed to marshal activity: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(string(activityJSON)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to send message: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// SendStreaming sends a message with streaming support.
func (c *Channel) SendStreaming(ctx context.Context, chatID string, replyToID string, content <-chan string, done chan<- struct{}) error {
	defer close(done)

	c.mu.RLock()
	token := c.accessToken
	c.mu.RUnlock()

	if token == "" {
		return fmt.Errorf("channel not initialized")
	}

	// Teams doesn't support true streaming, so we accumulate and send periodically
	var fullContent strings.Builder
	lastUpdate := time.Now()
	updateInterval := 1 * time.Second

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk, ok := <-content:
			if !ok {
				// Channel closed, send final message
				if fullContent.Len() > 0 {
					return c.Send(ctx, channel.OutgoingMessage{
						ChatID:    chatID,
						Content:   fullContent.String(),
						ReplyToID: replyToID,
					})
				}
				return nil
			}

			fullContent.WriteString(chunk)

			// Update message periodically
			if time.Since(lastUpdate) >= updateInterval && fullContent.Len() > 0 {
				// For Teams, we just accumulate - true streaming would require
				// updating the same message which has rate limits
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
		Name:         "teams",
		Type:         "teams",
		Status:       c.status,
		Enabled:      c.config.Enabled,
		ConnectedAt:  c.connectedAt,
		LastError:    c.lastError,
		LastErrorAt:  c.lastErrorAt,
		MessageCount: c.msgCount.Load(),
		Metadata:     make(map[string]interface{}),
	}

	info.Metadata["app_id"] = c.config.AppID
	if c.config.TenantID != "" {
		info.Metadata["tenant_id"] = c.config.TenantID
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

// isTeamAllowed checks if a team is allowed.
func (c *Channel) isTeamAllowed(teamID string) bool {
	if len(c.config.AllowedTeams) == 0 {
		return true
	}

	for _, allowed := range c.config.AllowedTeams {
		if allowed == teamID {
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

// HandleActivity processes an incoming Bot Framework activity.
// This should be called from a webhook handler.
func (c *Channel) HandleActivity(activity *Activity) {
	if activity == nil || activity.Type != "message" {
		return
	}

	// Check if user is allowed
	if activity.From != nil && !c.isUserAllowed(activity.From.ID) {
		c.logger.Debug("ignoring message from non-allowed user",
			zap.String("user_id", activity.From.ID))
		return
	}

	// Check if team is allowed (for team conversations)
	if activity.ChannelData != nil {
		if teamID, ok := activity.ChannelData["team"].(map[string]interface{}); ok {
			if id, ok := teamID["id"].(string); ok && !c.isTeamAllowed(id) {
				c.logger.Debug("ignoring message from non-allowed team",
					zap.String("team_id", id))
				return
			}
		}
	}

	// Convert to unified message format
	msg := c.convertActivity(activity)
	c.msgCount.Add(1)

	select {
	case c.messages <- msg:
	default:
		c.logger.Warn("message channel full, dropping message",
			zap.String("message_id", msg.ID))
	}
}

// convertActivity converts a Bot Framework activity to the unified format.
func (c *Channel) convertActivity(activity *Activity) channel.Message {
	username := ""
	userID := ""
	if activity.From != nil {
		username = activity.From.Name
		userID = activity.From.ID
	}

	// Build ChatID as serviceUrl|conversationId for reply routing
	chatID := ""
	if activity.ServiceURL != "" && activity.Conversation != nil {
		chatID = activity.ServiceURL + "|" + activity.Conversation.ID
	}

	isGroup := false
	groupName := ""
	if activity.Conversation != nil {
		isGroup = activity.Conversation.IsGroup
		groupName = activity.Conversation.Name
	}

	msg := channel.Message{
		ID:          activity.ID,
		ChannelName: "teams",
		ChatID:      chatID,
		UserID:      userID,
		Username:    username,
		Type:        channel.MessageTypeText,
		Content:     activity.Text,
		Timestamp:   time.Now(),
		IsGroup:     isGroup,
		GroupName:   groupName,
		Metadata: map[string]interface{}{
			"service_url": activity.ServiceURL,
		},
	}

	if activity.ReplyToID != "" {
		msg.ReplyToID = activity.ReplyToID
	}

	// Handle attachments
	for _, att := range activity.Attachments {
		msgAtt := channel.Attachment{
			Name:     att.Name,
			URL:      att.ContentURL,
			MimeType: att.ContentType,
		}

		switch {
		case strings.HasPrefix(att.ContentType, "image/"):
			msgAtt.Type = channel.MessageTypeImage
		case strings.HasPrefix(att.ContentType, "audio/"):
			msgAtt.Type = channel.MessageTypeAudio
		case strings.HasPrefix(att.ContentType, "video/"):
			msgAtt.Type = channel.MessageTypeVideo
		default:
			msgAtt.Type = channel.MessageTypeFile
		}

		msg.Attachments = append(msg.Attachments, msgAtt)
	}

	if len(msg.Attachments) > 0 {
		msg.Type = msg.Attachments[0].Type
	}

	return msg
}

// Activity represents a Bot Framework activity.
type Activity struct {
	Type         string                 `json:"type"`
	ID           string                 `json:"id"`
	Timestamp    string                 `json:"timestamp"`
	ServiceURL   string                 `json:"serviceUrl"`
	ChannelID    string                 `json:"channelId"`
	From         *ChannelAccount        `json:"from"`
	Conversation *ConversationAccount   `json:"conversation"`
	Recipient    *ChannelAccount        `json:"recipient"`
	Text         string                 `json:"text"`
	ReplyToID    string                 `json:"replyToId"`
	Attachments  []ActivityAttachment   `json:"attachments"`
	ChannelData  map[string]interface{} `json:"channelData"`
}

// ChannelAccount represents a user or bot account.
type ChannelAccount struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ConversationAccount represents a conversation.
type ConversationAccount struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	IsGroup bool   `json:"isGroup"`
}

// ActivityAttachment represents an attachment in an activity.
type ActivityAttachment struct {
	ContentType string `json:"contentType"`
	ContentURL  string `json:"contentUrl"`
	Name        string `json:"name"`
}
