// Package wechatilink provides a dedicated iLink-backed personal WeChat channel.
package wechatilink

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
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
)

const (
	channelName = "wechat_ilink"
	channelType = "wechat_ilink"
	iLinkBotAPI = "/ilink/bot"

	iLinkMessageTypeUser = 1
	iLinkItemTypeText    = 1
	iLinkItemTypeImage   = 2
	iLinkItemTypeVoice   = 3
	iLinkItemTypeFile    = 4
	iLinkItemTypeVideo   = 5
)

var (
	iLinkStartupProbeTimeout = 60 * time.Second
	iLinkHTTPTimeout         = 75 * time.Second
)

type Channel struct {
	config   channel.WeChatILinkConfig
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

	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	closeOnce sync.Once

	httpClient *http.Client

	ilinkMu            sync.RWMutex
	ilinkContextTokens map[string]string
	ilinkUINHeader     string
}

func New(cfg channel.WeChatILinkConfig, logger *zap.Logger) *Channel {
	return &Channel{
		config:   cfg,
		logger:   logger.With(zap.String("channel", channelName)),
		messages: make(chan channel.Message, 100),
		status:   channel.StatusDisconnected,
		httpClient: &http.Client{
			Timeout: iLinkHTTPTimeout,
		},
		ilinkContextTokens: make(map[string]string),
	}
}

func (c *Channel) Name() string {
	return channelName
}

func (c *Channel) Type() string {
	return channelType
}

func (c *Channel) OutboundCapabilities() channel.OutboundCapabilities {
	return channel.OutboundCapabilities{
		MarkdownMode:           channel.OutboundMarkdownModeChunked,
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

	if strings.TrimSpace(c.config.APIBaseURL) == "" {
		c.setError("missing iLink api_base_url")
		return fmt.Errorf("missing iLink api_base_url")
	}
	if strings.TrimSpace(c.config.BotToken) == "" {
		c.setError("missing iLink bot_token")
		return fmt.Errorf("missing iLink bot_token")
	}
	if c.httpClient == nil {
		c.httpClient = &http.Client{Timeout: iLinkHTTPTimeout}
	}
	_ = c.ensureILinkUINHeader()

	probeCtx, cancel := context.WithTimeout(c.ctx, iLinkStartupProbeTimeout)
	defer cancel()

	initialResp, err := c.ilinkGetUpdates(probeCtx, "")
	if err != nil {
		if probeCtx.Err() == context.DeadlineExceeded {
			now := time.Now()
			c.mu.Lock()
			c.status = channel.StatusConnected
			c.connectedAt = &now
			c.lastError = ""
			c.lastErrorAt = nil
			c.mu.Unlock()

			c.wg.Add(1)
			go c.runILinkPoller("", false, 0)

			c.logger.Info("wechat iLink startup probe timed out; continuing with background poller",
				zap.Duration("probe_timeout", iLinkStartupProbeTimeout))
			return nil
		}
		c.setError(fmt.Sprintf("failed to initialize iLink polling: %v", err))
		return fmt.Errorf("failed to initialize iLink channel: %w", err)
	}

	now := time.Now()
	c.mu.Lock()
	c.status = channel.StatusConnected
	c.connectedAt = &now
	c.lastError = ""
	c.lastErrorAt = nil
	c.mu.Unlock()

	c.handleILinkIncomingMessages(initialResp.Msgs)

	c.wg.Add(1)
	go c.runILinkPoller(initialResp.GetUpdatesBuf, len(initialResp.Msgs) == 0, initialResp.LongPollingTimeoutMS)

	c.logger.Info("wechat iLink channel started", zap.String("api_base_url", c.config.APIBaseURL))
	return nil
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

	c.wg.Wait()
	c.closeOnce.Do(func() {
		close(c.messages)
	})
	c.logger.Info("wechat iLink channel stopped")
	return nil
}

func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	content := strings.TrimSpace(msg.Content)
	if content == "" {
		content = iLinkAttachmentFallbackText(msg)
	}
	if content == "" {
		return fmt.Errorf("no sendable iLink content")
	}

	req := iLinkSendMessageRequest{
		Msg: iLinkOutgoingMessage{
			ToUserID:     msg.ChatID,
			ContextToken: c.resolveILinkContextToken(msg),
			ItemList: []iLinkMessageItem{
				{
					Type:     iLinkItemTypeText,
					TextItem: &iLinkTextItem{Text: content},
				},
			},
		},
	}

	var resp iLinkResponseEnvelope
	if err := c.postILinkJSON(ctx, "sendmessage", req, &resp); err != nil {
		return fmt.Errorf("send iLink message: %w", err)
	}
	if err := validateILinkResponse(resp); err != nil {
		return err
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

	var fullContent strings.Builder

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk, ok := <-content:
			if !ok {
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

func (c *Channel) Info() channel.Info {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return channel.Info{
		Name:             c.Name(),
		Type:             c.Type(),
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
			"api_base_url":  c.config.APIBaseURL,
			"has_bot_token": c.config.BotToken != "",
		},
	}
}

func (c *Channel) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status == channel.StatusConnected
}

func (c *Channel) Messages() <-chan channel.Message {
	return c.messages
}

func (c *Channel) setError(err string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastError = err
	now := time.Now()
	c.lastErrorAt = &now
	c.status = channel.StatusError
}

type iLinkResponseEnvelope struct {
	Ret     int    `json:"ret"`
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

type iLinkGetUpdatesRequest struct {
	GetUpdatesBuf string `json:"get_updates_buf"`
}

type iLinkGetUpdatesResponse struct {
	iLinkResponseEnvelope
	Msgs                 []iLinkMessage `json:"msgs"`
	GetUpdatesBuf        string         `json:"get_updates_buf"`
	LongPollingTimeoutMS int            `json:"longpolling_timeout_ms"`
}

type iLinkSendMessageRequest struct {
	Msg iLinkOutgoingMessage `json:"msg"`
}

type iLinkOutgoingMessage struct {
	ToUserID     string             `json:"to_user_id"`
	ContextToken string             `json:"context_token,omitempty"`
	ItemList     []iLinkMessageItem `json:"item_list"`
}

type iLinkMessage struct {
	MessageID    int64              `json:"message_id"`
	FromUserID   string             `json:"from_user_id"`
	ToUserID     string             `json:"to_user_id"`
	CreateTimeMS int64              `json:"create_time_ms"`
	SessionID    string             `json:"session_id"`
	MessageType  int                `json:"message_type"`
	MessageState int                `json:"message_state"`
	ItemList     []iLinkMessageItem `json:"item_list"`
	ContextToken string             `json:"context_token"`
}

type iLinkMessageItem struct {
	Type      int             `json:"type"`
	TextItem  *iLinkTextItem  `json:"text_item,omitempty"`
	ImageItem *iLinkNamedItem `json:"image_item,omitempty"`
	VoiceItem *iLinkNamedItem `json:"voice_item,omitempty"`
	FileItem  *iLinkNamedItem `json:"file_item,omitempty"`
	VideoItem *iLinkNamedItem `json:"video_item,omitempty"`
}

type iLinkTextItem struct {
	Text string `json:"text"`
}

type iLinkNamedItem struct {
	FileName string `json:"file_name,omitempty"`
	Name     string `json:"filename,omitempty"`
}

func (c *Channel) runILinkPoller(cursor string, waitBeforeNext bool, longPollingTimeoutMS int) {
	defer c.wg.Done()

	if waitBeforeNext {
		if !sleepWithContext(c.ctx, iLinkPollDelay(longPollingTimeoutMS)) {
			return
		}
	}

	for {
		resp, err := c.ilinkGetUpdates(c.ctx, cursor)
		if err != nil {
			if c.ctx == nil || c.ctx.Err() != nil {
				return
			}
			c.setError(fmt.Sprintf("failed to poll iLink updates: %v", err))
			if !sleepWithContext(c.ctx, time.Second) {
				return
			}
			continue
		}

		if resp.GetUpdatesBuf != "" {
			cursor = resp.GetUpdatesBuf
		}

		c.mu.Lock()
		if c.status != channel.StatusConnected {
			c.status = channel.StatusConnected
		}
		c.lastError = ""
		c.lastErrorAt = nil
		c.mu.Unlock()

		c.handleILinkIncomingMessages(resp.Msgs)

		if len(resp.Msgs) == 0 {
			if !sleepWithContext(c.ctx, iLinkPollDelay(resp.LongPollingTimeoutMS)) {
				return
			}
		}
	}
}

func iLinkPollDelay(longPollingTimeoutMS int) time.Duration {
	delay := 250 * time.Millisecond
	if longPollingTimeoutMS > 0 && longPollingTimeoutMS < 250 {
		delay = time.Duration(longPollingTimeoutMS) * time.Millisecond
	}
	return delay
}

func (c *Channel) handleILinkIncomingMessages(messages []iLinkMessage) {
	for _, incoming := range messages {
		msg, ok := c.convertILinkMessage(incoming)
		if !ok {
			continue
		}
		if incoming.ContextToken != "" {
			c.storeILinkContextToken(msg.ChatID, incoming.ContextToken)
		}

		c.msgCount.Add(1)
		c.msgsReceived.Add(1)
		now := time.Now()
		c.mu.Lock()
		c.lastMessageAt = &now
		c.mu.Unlock()

		select {
		case <-c.ctx.Done():
			return
		case c.messages <- msg:
		default:
			c.logger.Warn("iLink message channel full, dropping message",
				zap.String("message_id", msg.ID))
		}
	}
}

func (c *Channel) ilinkGetUpdates(ctx context.Context, cursor string) (iLinkGetUpdatesResponse, error) {
	var resp iLinkGetUpdatesResponse
	if err := c.postILinkJSON(ctx, "getupdates", iLinkGetUpdatesRequest{GetUpdatesBuf: cursor}, &resp); err != nil {
		return resp, err
	}
	if err := validateILinkResponse(resp.iLinkResponseEnvelope); err != nil {
		return resp, err
	}
	return resp, nil
}

func (c *Channel) postILinkJSON(ctx context.Context, endpoint string, payload any, out any) error {
	baseURL := resolveILinkBotBaseURL(c.config.APIBaseURL)
	if baseURL == "" {
		return fmt.Errorf("missing iLink api_base_url")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal iLink payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/"+strings.TrimLeft(endpoint, "/"), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create iLink request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("AuthorizationType", "ilink_bot_token")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.config.BotToken))
	req.Header.Set("X-WECHAT-UIN", c.ensureILinkUINHeader())

	client := c.httpClient
	if client == nil {
		client = &http.Client{Timeout: iLinkHTTPTimeout}
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("iLink authentication failed")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("iLink API status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode iLink response: %w", err)
	}
	return nil
}

func resolveILinkBotBaseURL(raw string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(raw), "/")
	if baseURL == "" {
		return ""
	}
	if strings.HasSuffix(baseURL, iLinkBotAPI) {
		return baseURL
	}
	return baseURL + iLinkBotAPI
}

func validateILinkResponse(resp iLinkResponseEnvelope) error {
	if resp.Ret != 0 {
		if strings.TrimSpace(resp.ErrMsg) != "" {
			return fmt.Errorf("iLink API error: %s", strings.TrimSpace(resp.ErrMsg))
		}
		return fmt.Errorf("iLink API returned ret=%d", resp.Ret)
	}
	if resp.ErrCode != 0 {
		if strings.TrimSpace(resp.ErrMsg) != "" {
			return fmt.Errorf("iLink API error: %s", strings.TrimSpace(resp.ErrMsg))
		}
		return fmt.Errorf("iLink API returned errcode=%d", resp.ErrCode)
	}
	return nil
}

func (c *Channel) ensureILinkUINHeader() string {
	c.ilinkMu.Lock()
	defer c.ilinkMu.Unlock()
	if c.ilinkUINHeader != "" {
		return c.ilinkUINHeader
	}

	var raw [4]byte
	if _, err := rand.Read(raw[:]); err != nil {
		binary.BigEndian.PutUint32(raw[:], uint32(time.Now().UnixNano()))
	}
	c.ilinkUINHeader = base64.StdEncoding.EncodeToString(raw[:])
	return c.ilinkUINHeader
}

func (c *Channel) resolveILinkContextToken(msg channel.OutgoingMessage) string {
	if msg.Metadata != nil {
		if raw, ok := msg.Metadata["context_token"].(string); ok && strings.TrimSpace(raw) != "" {
			return strings.TrimSpace(raw)
		}
	}

	c.ilinkMu.RLock()
	defer c.ilinkMu.RUnlock()
	return c.ilinkContextTokens[msg.ChatID]
}

func (c *Channel) storeILinkContextToken(chatID, token string) {
	chatID = strings.TrimSpace(chatID)
	token = strings.TrimSpace(token)
	if chatID == "" || token == "" {
		return
	}
	c.ilinkMu.Lock()
	defer c.ilinkMu.Unlock()
	c.ilinkContextTokens[chatID] = token
}

func (c *Channel) convertILinkMessage(msg iLinkMessage) (channel.Message, bool) {
	if msg.MessageType != 0 && msg.MessageType != iLinkMessageTypeUser {
		return channel.Message{}, false
	}
	fromUserID := strings.TrimSpace(msg.FromUserID)
	if fromUserID == "" {
		return channel.Message{}, false
	}

	msgType, content, attachments := convertILinkItems(msg.ItemList)
	if content == "" && len(attachments) == 0 {
		return channel.Message{}, false
	}

	messageID := strconv.FormatInt(msg.MessageID, 10)
	if messageID == "0" {
		messageID = fmt.Sprintf("%s:%d", fromUserID, msg.CreateTimeMS)
	}

	timestamp := time.Now()
	if msg.CreateTimeMS > 0 {
		timestamp = time.Unix(0, msg.CreateTimeMS*int64(time.Millisecond))
	}

	return channel.Message{
		ID:          messageID,
		ChannelName: channelName,
		ChatID:      fromUserID,
		UserID:      fromUserID,
		Username:    fromUserID,
		Type:        msgType,
		Content:     content,
		Attachments: attachments,
		Timestamp:   timestamp,
		Metadata: map[string]interface{}{
			"context_token": msg.ContextToken,
			"session_id":    msg.SessionID,
			"to_user_id":    msg.ToUserID,
		},
	}, true
}

func convertILinkItems(items []iLinkMessageItem) (channel.MessageType, string, []channel.Attachment) {
	msgType := channel.MessageTypeText
	textParts := make([]string, 0, len(items))
	attachments := make([]channel.Attachment, 0)

	for _, item := range items {
		switch item.Type {
		case iLinkItemTypeText:
			if item.TextItem != nil && strings.TrimSpace(item.TextItem.Text) != "" {
				textParts = append(textParts, strings.TrimSpace(item.TextItem.Text))
			}
		case iLinkItemTypeImage:
			msgType = channel.MessageTypeImage
			attachments = append(attachments, iLinkAttachmentFromItem(item, channel.MessageTypeImage))
		case iLinkItemTypeVoice:
			msgType = channel.MessageTypeAudio
			attachments = append(attachments, iLinkAttachmentFromItem(item, channel.MessageTypeAudio))
		case iLinkItemTypeFile:
			msgType = channel.MessageTypeFile
			attachments = append(attachments, iLinkAttachmentFromItem(item, channel.MessageTypeFile))
		case iLinkItemTypeVideo:
			msgType = channel.MessageTypeVideo
			attachments = append(attachments, iLinkAttachmentFromItem(item, channel.MessageTypeVideo))
		}
	}

	content := strings.Join(textParts, "\n")
	if content == "" && len(attachments) > 0 {
		content = defaultILinkAttachmentText(attachments[0].Type)
	}
	if len(attachments) == 0 {
		msgType = channel.MessageTypeText
	}
	return msgType, content, attachments
}

func iLinkAttachmentFromItem(item iLinkMessageItem, msgType channel.MessageType) channel.Attachment {
	name := ""
	switch msgType {
	case channel.MessageTypeImage:
		name = iLinkNamedItemValue(item.ImageItem)
	case channel.MessageTypeAudio:
		name = iLinkNamedItemValue(item.VoiceItem)
	case channel.MessageTypeVideo:
		name = iLinkNamedItemValue(item.VideoItem)
	default:
		name = iLinkNamedItemValue(item.FileItem)
	}
	if name == "" {
		name = string(msgType)
	}
	return channel.Attachment{
		Type: msgType,
		Name: name,
	}
}

func iLinkNamedItemValue(item *iLinkNamedItem) string {
	if item == nil {
		return ""
	}
	if strings.TrimSpace(item.FileName) != "" {
		return strings.TrimSpace(item.FileName)
	}
	return strings.TrimSpace(item.Name)
}

func defaultILinkAttachmentText(msgType channel.MessageType) string {
	switch msgType {
	case channel.MessageTypeImage:
		return "[Image]"
	case channel.MessageTypeAudio:
		return "[Voice]"
	case channel.MessageTypeVideo:
		return "[Video]"
	case channel.MessageTypeFile:
		return "[File]"
	default:
		return "[Attachment]"
	}
}

func iLinkAttachmentFallbackText(msg channel.OutgoingMessage) string {
	parts := make([]string, 0, len(msg.Attachments)+1)
	if strings.TrimSpace(msg.Content) != "" {
		parts = append(parts, strings.TrimSpace(msg.Content))
	}
	for _, att := range msg.Attachments {
		switch {
		case strings.TrimSpace(att.URL) != "":
			parts = append(parts, strings.TrimSpace(att.URL))
		case strings.TrimSpace(att.Name) != "":
			parts = append(parts, strings.TrimSpace(att.Name))
		default:
			parts = append(parts, defaultILinkAttachmentText(att.Type))
		}
	}
	return strings.Join(parts, "\n")
}

func sleepWithContext(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return true
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
