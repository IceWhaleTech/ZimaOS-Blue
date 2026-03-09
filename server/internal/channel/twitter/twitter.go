// Package twitter provides a Twitter/X API v2 channel implementation.
package twitter

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/validator"
)

// Config contains Twitter/X channel configuration.
type Config struct {
	Enabled           bool   `yaml:"enabled"`
	APIKey            string `yaml:"api_key"`
	APISecret         string `yaml:"api_secret"`
	AccessToken       string `yaml:"access_token"`
	AccessTokenSecret string `yaml:"access_token_secret"`
	BearerToken       string `yaml:"bearer_token"`
}

const (
	twitterAPIBase    = "https://api.twitter.com/2"
	twitterUploadBase = "https://upload.twitter.com/1.1"
	dmEventsURL       = twitterAPIBase + "/dm_events"
	usersMeURL        = twitterAPIBase + "/users/me"
	mediaUploadURL    = twitterUploadBase + "/media/upload.json"
	pollInterval      = 30 * time.Second
)

// dmEvent represents a Twitter DM event from the v2 API.
type dmEvent struct {
	ID          string         `json:"id"`
	EventType   string         `json:"event_type"`
	Text        string         `json:"text"`
	SenderID    string         `json:"sender_id"`
	DMID        string         `json:"dm_conversation_id"`
	CreatedAt   time.Time      `json:"created_at"`
	Attachments *dmAttachments `json:"attachments,omitempty"`
}

type dmAttachments struct {
	MediaKeys []string `json:"media_keys,omitempty"`
}

type dmEventsResponse struct {
	Data []dmEvent `json:"data"`
	Meta struct {
		NextToken   string `json:"next_token"`
		ResultCount int    `json:"result_count"`
	} `json:"meta"`
	Includes *dmIncludes `json:"includes,omitempty"`
}

type dmIncludes struct {
	Media []dmMedia `json:"media,omitempty"`
}

type dmMedia struct {
	MediaKey   string `json:"media_key"`
	Type       string `json:"type"`
	URL        string `json:"url,omitempty"`
	PreviewURL string `json:"preview_image_url,omitempty"`
}

type mediaUploadResponse struct {
	MediaID       int64  `json:"media_id"`
	MediaIDString string `json:"media_id_string"`
}

// Channel implements the channel.Channel interface for Twitter/X.
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

	sinceID      string
	selfUserID   string
	selfUsername string
	ctx          context.Context
	cancel       context.CancelFunc
}

// New creates a new Twitter/X channel.
func New(cfg Config, logger *zap.Logger) *Channel {
	return &Channel{
		config:   cfg,
		logger:   logger.With(zap.String("channel", "twitter")),
		client:   &http.Client{Timeout: 30 * time.Second},
		messages: make(chan channel.Message, 100),
		status:   channel.StatusDisconnected,
	}
}

func (c *Channel) Name() string                     { return "twitter" }
func (c *Channel) Type() string                     { return "twitter" }
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
	c.mu.Unlock()

	if err := c.fetchSelfProfile(ctx); err != nil {
		c.setError(err.Error())
		if c.cancel != nil {
			c.cancel()
		}
		return err
	}

	c.mu.Lock()
	now := time.Now()
	c.status = channel.StatusConnected
	c.connectedAt = &now
	c.mu.Unlock()

	go c.pollDMEvents()

	c.logger.Info("Twitter channel started", zap.String("self_user_id", c.selfUserID))
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
	c.logger.Info("Twitter channel stopped")
	return nil
}

// pollDMEvents polls for new DM events using the v2 API.
func (c *Channel) fetchSelfProfile(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, usersMeURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create self profile request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.config.BearerToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch self profile: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Twitter self profile error (status %d): %s", resp.StatusCode, string(body))
	}

	var userResp struct {
		Data struct {
			ID       string `json:"id"`
			Username string `json:"username"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userResp); err != nil {
		return fmt.Errorf("failed to decode self profile: %w", err)
	}
	if strings.TrimSpace(userResp.Data.ID) == "" {
		return fmt.Errorf("twitter self profile missing id")
	}

	c.mu.Lock()
	c.selfUserID = strings.TrimSpace(userResp.Data.ID)
	c.selfUsername = strings.TrimSpace(userResp.Data.Username)
	c.mu.Unlock()
	return nil
}

func (c *Channel) pollDMEvents() {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if err := c.fetchDMEvents(); err != nil {
				c.logger.Error("failed to fetch DM events", zap.Error(err))
				c.setError(err.Error())
			}
		}
	}
}

func (c *Channel) fetchDMEvents() error {
	params := url.Values{}
	params.Set("dm_event.fields", "id,text,sender_id,dm_conversation_id,created_at,attachments")
	params.Set("expansions", "attachments.media_keys")
	params.Set("media.fields", "media_key,type,url,preview_image_url")
	if c.sinceID != "" {
		params.Set("since_id", c.sinceID)
	}

	reqURL := dmEventsURL + "?" + params.Encode()
	req, err := http.NewRequestWithContext(c.ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	c.signOAuth1a(req, http.MethodGet, dmEventsURL, params)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch DM events: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Twitter API error (status %d): %s", resp.StatusCode, string(body))
	}

	var eventsResp dmEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&eventsResp); err != nil {
		return fmt.Errorf("failed to decode DM events: %w", err)
	}

	// Build media lookup from includes
	mediaMap := make(map[string]dmMedia)
	if eventsResp.Includes != nil {
		for _, m := range eventsResp.Includes.Media {
			mediaMap[m.MediaKey] = m
		}
	}

	// Process events in reverse order (oldest first)
	for i := len(eventsResp.Data) - 1; i >= 0; i-- {
		event := eventsResp.Data[i]
		if event.EventType == "MessageCreate" {
			c.processMessage(event, mediaMap)
		}
	}

	// Update since_id to the newest event
	if len(eventsResp.Data) > 0 {
		c.sinceID = eventsResp.Data[0].ID
	}

	return nil
}

// processMessage converts a DM event into a channel.Message.
func (c *Channel) processMessage(event dmEvent, mediaMap map[string]dmMedia) {
	c.mu.RLock()
	selfUserID := c.selfUserID
	selfUsername := c.selfUsername
	c.mu.RUnlock()
	if strings.TrimSpace(event.SenderID) == "" {
		return
	}
	if strings.TrimSpace(selfUserID) != "" && event.SenderID == selfUserID {
		return
	}

	chatTarget := strings.TrimSpace(event.SenderID)
	if chatTarget == "" {
		chatTarget = event.DMID
	}
	metadata := map[string]interface{}{
		"dm_conversation_id": event.DMID,
		"participant_id":     event.SenderID,
	}
	if strings.TrimSpace(selfUserID) != "" {
		metadata["self_user_id"] = selfUserID
	}
	if strings.TrimSpace(selfUsername) != "" {
		metadata["self_username"] = selfUsername
	}
	msg := channel.Message{
		ID:          event.ID,
		ChannelName: "twitter",
		ChatID:      chatTarget,
		UserID:      event.SenderID,
		Username:    event.SenderID,
		Type:        channel.MessageTypeText,
		Content:     event.Text,
		Timestamp:   event.CreatedAt,
		Metadata:    metadata,
	}

	// Attach media if present
	if event.Attachments != nil {
		for _, key := range event.Attachments.MediaKeys {
			if media, ok := mediaMap[key]; ok {
				mediaURL := media.URL
				if mediaURL == "" {
					mediaURL = media.PreviewURL
				}
				att := channel.Attachment{
					ID:   media.MediaKey,
					Type: toMessageType(media.Type),
					URL:  mediaURL,
					Name: media.MediaKey,
				}
				msg.Attachments = append(msg.Attachments, att)
			}
		}
		if len(msg.Attachments) > 0 && msg.Content == "" {
			msg.Type = msg.Attachments[0].Type
		}
	}

	c.msgCount.Add(1)
	c.msgsRecv.Add(1)

	select {
	case c.messages <- msg:
	default:
		c.logger.Warn("message channel full, dropping message", zap.String("id", msg.ID))
	}
}

func toMessageType(twitterType string) channel.MessageType {
	switch twitterType {
	case "photo":
		return channel.MessageTypeImage
	case "video", "animated_gif":
		return channel.MessageTypeVideo
	default:
		return channel.MessageTypeFile
	}
}

// Send sends a DM via the Twitter v2 API, including media attachments.
func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	sendURL := twitterSendURL(msg)

	textParts := make([]string, 0, 1+len(msg.Attachments))
	if strings.TrimSpace(msg.Content) != "" {
		textParts = append(textParts, strings.TrimSpace(msg.Content))
	}

	payload := map[string]interface{}{}

	// Upload and attach media if present.
	if len(msg.Attachments) > 0 {
		mediaIDs := make([]map[string]string, 0, len(msg.Attachments))
		for _, att := range msg.Attachments {
			if att.ID != "" {
				mediaIDs = append(mediaIDs, map[string]string{"media_id": att.ID})
				continue
			}
			if len(att.Data) > 0 {
				mid, err := c.uploadMedia(ctx, att.Data)
				if err != nil {
					c.logger.Warn("failed to upload media to Twitter", zap.String("type", string(att.Type)), zap.Error(err))
					if fallback := twitterAttachmentFallbackText(att); fallback != "" {
						textParts = append(textParts, fallback)
					}
					continue
				}
				mediaIDs = append(mediaIDs, map[string]string{"media_id": mid})
				continue
			}
			if fallback := twitterAttachmentFallbackText(att); fallback != "" {
				textParts = append(textParts, fallback)
			}
		}
		if len(mediaIDs) > 0 {
			payload["attachments"] = mediaIDs
		}
	}

	if text := strings.Join(textParts, "\n"); text != "" {
		payload["text"] = text
	}
	if len(payload) == 0 {
		return fmt.Errorf("no sendable Twitter content")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sendURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.signOAuth1a(req, http.MethodPost, sendURL, nil)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send DM: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Twitter API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	c.msgsSent.Add(1)
	return nil
}

func twitterSendURL(msg channel.OutgoingMessage) string {
	if conversationID := twitterConversationID(msg); conversationID != "" {
		return fmt.Sprintf("%s/dm_conversations/%s/messages", twitterAPIBase, conversationID)
	}
	return fmt.Sprintf("%s/dm_conversations/with/%s/messages", twitterAPIBase, msg.ChatID)
}

func twitterConversationID(msg channel.OutgoingMessage) string {
	if msg.Metadata != nil {
		if conversationID, ok := msg.Metadata["dm_conversation_id"].(string); ok && strings.TrimSpace(conversationID) != "" {
			return strings.TrimSpace(conversationID)
		}
	}
	if strings.Contains(msg.ChatID, "-") {
		return strings.TrimSpace(msg.ChatID)
	}
	return ""
}

func twitterAttachmentFallbackText(att channel.Attachment) string {
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

// SendMedia uploads an image and sends it as a DM.
func (c *Channel) SendMedia(ctx context.Context, participantID string, imageData []byte, text string) error {
	// Step 1: Upload media via v1.1 media/upload
	mediaID, err := c.uploadMedia(ctx, imageData)
	if err != nil {
		return fmt.Errorf("failed to upload media: %w", err)
	}

	// Step 2: Send DM with media attachment
	sendURL := fmt.Sprintf("%s/dm_conversations/with/%s/messages", twitterAPIBase, participantID)
	payload := map[string]interface{}{
		"attachments": []map[string]string{
			{"media_id": mediaID},
		},
	}
	if text != "" {
		payload["text"] = text
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sendURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.signOAuth1a(req, http.MethodPost, sendURL, nil)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send media DM: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Twitter API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	c.msgsSent.Add(1)
	return nil
}

func (c *Channel) uploadMedia(ctx context.Context, data []byte) (string, error) {
	encoded := base64.StdEncoding.EncodeToString(data)
	params := url.Values{}
	params.Set("media_data", encoded)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mediaUploadURL, strings.NewReader(params.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create upload request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.signOAuth1a(req, http.MethodPost, mediaUploadURL, params)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to upload media: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("media upload error (status %d): %s", resp.StatusCode, string(body))
	}

	var uploadResp mediaUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
		return "", fmt.Errorf("failed to decode upload response: %w", err)
	}

	return uploadResp.MediaIDString, nil
}

// SendStreaming sends a message in chunks as separate DMs.
func (c *Channel) SendStreaming(ctx context.Context, chatID string, replyToID string, content <-chan string, done chan<- struct{}) error {
	defer close(done)

	const maxChunkLen = 10000 // Twitter DM character limit
	var buf strings.Builder

	flush := func() error {
		if buf.Len() == 0 {
			return nil
		}
		err := c.Send(ctx, channel.OutgoingMessage{ChatID: chatID, ReplyToID: replyToID, Content: buf.String()})
		buf.Reset()
		return err
	}

	for chunk := range content {
		buf.WriteString(chunk)
		if buf.Len() >= maxChunkLen {
			if err := flush(); err != nil {
				return err
			}
		}
	}

	return flush()
}

func (c *Channel) Info() channel.Info {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return channel.Info{
		Name: "twitter", Type: "twitter", Status: c.status, Enabled: c.config.Enabled,
		ConnectedAt: c.connectedAt, LastError: c.lastError, LastErrorAt: c.lastErrorAt,
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

// --- OAuth 1.0a signature helper ---

func (c *Channel) signOAuth1a(req *http.Request, method string, baseURL string, extraParams url.Values) {
	nonce := make([]byte, 16)
	_, _ = rand.Read(nonce)

	oauthParams := url.Values{
		"oauth_consumer_key":     {c.config.APIKey},
		"oauth_nonce":            {base64.RawURLEncoding.EncodeToString(nonce)},
		"oauth_signature_method": {"HMAC-SHA1"},
		"oauth_timestamp":        {fmt.Sprintf("%d", time.Now().Unix())},
		"oauth_token":            {c.config.AccessToken},
		"oauth_version":          {"1.0"},
	}

	// Combine oauth params + extra query/body params for signature base
	allParams := url.Values{}
	for k, v := range oauthParams {
		allParams[k] = v
	}
	if extraParams != nil {
		for k, v := range extraParams {
			allParams[k] = v
		}
	}

	// Build sorted parameter string
	var keys []string
	for k := range allParams {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var paramParts []string
	for _, k := range keys {
		for _, v := range allParams[k] {
			paramParts = append(paramParts, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}
	paramString := strings.Join(paramParts, "&")

	// Signature base string
	sigBase := strings.ToUpper(method) + "&" + url.QueryEscape(baseURL) + "&" + url.QueryEscape(paramString)

	// Signing key
	sigKey := url.QueryEscape(c.config.APISecret) + "&" + url.QueryEscape(c.config.AccessTokenSecret)

	mac := hmac.New(sha1.New, []byte(sigKey))
	mac.Write([]byte(sigBase))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	oauthParams.Set("oauth_signature", signature)

	// Build Authorization header
	var headerParts []string
	for k, v := range oauthParams {
		headerParts = append(headerParts, fmt.Sprintf(`%s="%s"`, url.QueryEscape(k), url.QueryEscape(v[0])))
	}
	sort.Strings(headerParts)
	req.Header.Set("Authorization", "OAuth "+strings.Join(headerParts, ", "))
}

// --- Validator ---

// Validator validates Twitter/X API credentials.
type Validator struct{}

// NewValidator creates a new Twitter validator.
func NewValidator() *Validator { return &Validator{} }

// Validate tests the connection by calling GET /2/users/me with the Bearer token.
func (v *Validator) Validate(ctx context.Context, config map[string]string) validator.Result {
	bearerToken := config["bearer_token"]
	if bearerToken == "" {
		return validator.Result{Success: false, Error: "bearer_token is required"}
	}
	if config["api_key"] == "" {
		return validator.Result{Success: false, Error: "api_key is required"}
	}
	if config["api_secret"] == "" {
		return validator.Result{Success: false, Error: "api_secret is required"}
	}
	if config["access_token"] == "" {
		return validator.Result{Success: false, Error: "access_token is required"}
	}
	if config["access_token_secret"] == "" {
		return validator.Result{Success: false, Error: "access_token_secret is required"}
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, usersMeURL, nil)
	if err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to create request: %v", err)}
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	resp, err := client.Do(req)
	if err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to connect to Twitter API: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return validator.Result{Success: false, Error: fmt.Sprintf("Twitter API returned status %d", resp.StatusCode)}
	}

	var userResp struct {
		Data struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Username string `json:"username"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userResp); err != nil {
		return validator.Result{Success: false, Error: fmt.Sprintf("failed to parse user info: %v", err)}
	}

	return validator.Result{
		Success:    true,
		MessageKey: "channels.connectionSuccess",
		Data: map[string]interface{}{
			"user_name": userResp.Data.Name,
			"user_id":   userResp.Data.ID,
			"username":  userResp.Data.Username,
		},
	}
}
