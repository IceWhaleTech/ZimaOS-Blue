// Package wechat provides a WeChat Work (Enterprise WeChat) channel implementation.
package wechat

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

// Channel implements the channel.Channel interface for WeChat Work.
type Channel struct {
	config   channel.WeChatWorkConfig
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

	accessToken     string
	tokenExpireTime time.Time
	tokenMu         sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	server *http.Server

	sendTextFunc       func(ctx context.Context, chatID, content, format string) error
	sendAttachmentFunc func(ctx context.Context, chatID, caption string, att channel.Attachment) error
}

// New creates a new WeChat Work channel.
func New(cfg channel.WeChatWorkConfig, logger *zap.Logger) *Channel {
	ch := &Channel{
		config:   cfg,
		logger:   logger.With(zap.String("channel", "wechat_work")),
		messages: make(chan channel.Message, 100),
		status:   channel.StatusDisconnected,
	}
	ch.sendTextFunc = ch.sendText
	ch.sendAttachmentFunc = ch.sendAttachment
	return ch
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "wechat_work"
}

// Type returns the channel type.
func (c *Channel) Type() string {
	return "wechat_work"
}

func (c *Channel) OutboundCapabilities() channel.OutboundCapabilities {
	return channel.OutboundCapabilities{
		MarkdownMode:           channel.OutboundMarkdownModeChunked,
		SupportsMarkdownFormat: true,
	}
}

// Start initializes and starts the WeChat Work bot.
func (c *Channel) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.status == channel.StatusConnected || c.status == channel.StatusConnecting {
		c.mu.Unlock()
		return fmt.Errorf("channel already started")
	}
	c.status = channel.StatusConnecting
	c.mu.Unlock()

	c.ctx, c.cancel = context.WithCancel(ctx)

	// Get initial access token
	if err := c.refreshAccessToken(); err != nil {
		c.setError(fmt.Sprintf("failed to get access token: %v", err))
		return fmt.Errorf("failed to get WeChat Work access token: %w", err)
	}

	c.logger.Info("wechat work access token obtained")

	now := time.Now()
	c.mu.Lock()
	c.status = channel.StatusConnected
	c.connectedAt = &now
	c.lastError = ""
	c.lastErrorAt = nil
	c.mu.Unlock()

	c.logger.Info("wechat work channel started")
	return nil
}

// refreshAccessToken refreshes the access token.
func (c *Channel) refreshAccessToken() error {
	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		c.config.CorpID, c.config.Secret)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to request access token: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if result.ErrCode != 0 {
		return fmt.Errorf("WeChat API error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	c.tokenMu.Lock()
	c.accessToken = result.AccessToken
	c.tokenExpireTime = time.Now().Add(time.Duration(result.ExpiresIn-300) * time.Second) // Refresh 5 minutes early
	c.tokenMu.Unlock()

	return nil
}

// getAccessToken returns the current access token, refreshing if necessary.
func (c *Channel) getAccessToken() (string, error) {
	c.tokenMu.RLock()
	if time.Now().Before(c.tokenExpireTime) {
		token := c.accessToken
		c.tokenMu.RUnlock()
		return token, nil
	}
	c.tokenMu.RUnlock()

	if err := c.refreshAccessToken(); err != nil {
		return "", err
	}

	c.tokenMu.RLock()
	defer c.tokenMu.RUnlock()
	return c.accessToken, nil
}

// HandleCallback handles incoming WeChat Work callback requests.
// This should be called by an HTTP handler.
func (c *Channel) HandleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		// URL verification
		c.handleVerification(w, r)
		return
	}

	// Message callback
	c.handleMessage(w, r)
}

// handleVerification handles URL verification requests.
func (c *Channel) handleVerification(w http.ResponseWriter, r *http.Request) {
	msgSignature := r.URL.Query().Get("msg_signature")
	timestamp := r.URL.Query().Get("timestamp")
	nonce := r.URL.Query().Get("nonce")
	echostr := r.URL.Query().Get("echostr")

	// Verify signature
	if !c.verifySignature(msgSignature, timestamp, nonce, echostr) {
		c.logger.Warn("invalid signature in verification request")
		http.Error(w, "Invalid signature", http.StatusForbidden)
		return
	}

	// Decrypt echostr
	decrypted, err := c.decryptMessage(echostr)
	if err != nil {
		c.logger.Error("failed to decrypt echostr", zap.Error(err))
		http.Error(w, "Decryption failed", http.StatusInternalServerError)
		return
	}

	w.Write([]byte(decrypted))
}

// handleMessage handles incoming message callbacks.
func (c *Channel) handleMessage(w http.ResponseWriter, r *http.Request) {
	msgSignature := r.URL.Query().Get("msg_signature")
	timestamp := r.URL.Query().Get("timestamp")
	nonce := r.URL.Query().Get("nonce")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		c.logger.Error("failed to read request body", zap.Error(err))
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	// Parse encrypted message
	var encryptedMsg struct {
		ToUserName string `xml:"ToUserName"`
		AgentID    string `xml:"AgentID"`
		Encrypt    string `xml:"Encrypt"`
	}
	if err := xml.Unmarshal(body, &encryptedMsg); err != nil {
		c.logger.Error("failed to parse encrypted message", zap.Error(err))
		http.Error(w, "Invalid XML", http.StatusBadRequest)
		return
	}

	// Verify signature
	if !c.verifySignature(msgSignature, timestamp, nonce, encryptedMsg.Encrypt) {
		c.logger.Warn("invalid signature in message callback")
		http.Error(w, "Invalid signature", http.StatusForbidden)
		return
	}

	// Decrypt message
	decrypted, err := c.decryptMessage(encryptedMsg.Encrypt)
	if err != nil {
		c.logger.Error("failed to decrypt message", zap.Error(err))
		http.Error(w, "Decryption failed", http.StatusInternalServerError)
		return
	}

	// Parse decrypted message
	var msg wechatMessage
	if err := xml.Unmarshal([]byte(decrypted), &msg); err != nil {
		c.logger.Error("failed to parse decrypted message", zap.Error(err))
		http.Error(w, "Invalid message", http.StatusBadRequest)
		return
	}

	// Convert to unified message format
	channelMsg := c.convertMessage(&msg)
	c.msgCount.Add(1)
	c.msgsReceived.Add(1)
	now := time.Now()
	c.mu.Lock()
	c.lastMessageAt = &now
	c.mu.Unlock()

	select {
	case c.messages <- channelMsg:
	default:
		c.logger.Warn("message channel full, dropping message",
			zap.String("message_id", channelMsg.ID))
	}

	w.WriteHeader(http.StatusOK)
}

// wechatMessage represents a WeChat Work message.
type wechatMessage struct {
	ToUserName   string `xml:"ToUserName"`
	FromUserName string `xml:"FromUserName"`
	CreateTime   int64  `xml:"CreateTime"`
	MsgType      string `xml:"MsgType"`
	Content      string `xml:"Content"`
	MsgId        string `xml:"MsgId"`
	AgentID      string `xml:"AgentID"`
	PicUrl       string `xml:"PicUrl"`
	MediaId      string `xml:"MediaId"`
}

// convertMessage converts a WeChat Work message to the unified format.
func (c *Channel) convertMessage(msg *wechatMessage) channel.Message {
	msgType := channel.MessageTypeText
	switch msg.MsgType {
	case "image":
		msgType = channel.MessageTypeImage
	case "voice":
		msgType = channel.MessageTypeAudio
	case "video":
		msgType = channel.MessageTypeVideo
	case "file":
		msgType = channel.MessageTypeFile
	}

	channelMsg := channel.Message{
		ID:          msg.MsgId,
		ChannelName: "wechat_work",
		ChatID:      msg.FromUserName,
		UserID:      msg.FromUserName,
		Type:        msgType,
		Content:     msg.Content,
		Timestamp:   time.Unix(msg.CreateTime, 0),
		IsGroup:     false,
		Metadata: map[string]interface{}{
			"agent_id": msg.AgentID,
			"msg_type": msg.MsgType,
		},
	}

	// Handle attachments
	if msg.MediaId != "" {
		channelMsg.Attachments = append(channelMsg.Attachments, channel.Attachment{
			ID:       msg.MediaId,
			Type:     msgType,
			URL:      msg.PicUrl,
			MimeType: wechatAttachmentMimeType(msg.MsgType),
		})
		channelMsg.Metadata["media_id"] = msg.MediaId
		if strings.TrimSpace(msg.PicUrl) != "" {
			channelMsg.Metadata["pic_url"] = strings.TrimSpace(msg.PicUrl)
		}
	}

	return channelMsg
}

func wechatAttachmentMimeType(msgType string) string {
	switch msgType {
	case "image":
		return "image/jpeg"
	case "voice":
		return "audio/amr"
	case "video":
		return "video/mp4"
	case "file":
		return "application/octet-stream"
	default:
		return ""
	}
}

// verifySignature verifies the message signature.
func (c *Channel) verifySignature(signature, timestamp, nonce, encrypt string) bool {
	strs := []string{c.config.Token, timestamp, nonce, encrypt}
	sort.Strings(strs)
	str := strings.Join(strs, "")

	hash := sha1.Sum([]byte(str))
	return fmt.Sprintf("%x", hash) == signature
}

// decryptMessage decrypts an encrypted message.
func (c *Channel) decryptMessage(encrypted string) (string, error) {
	aesKey, err := base64.StdEncoding.DecodeString(c.config.EncodingAESKey + "=")
	if err != nil {
		return "", fmt.Errorf("invalid encoding AES key: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("invalid base64: %w", err)
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	if len(ciphertext) < aes.BlockSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	iv := aesKey[:aes.BlockSize]
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ciphertext, ciphertext)

	// Remove PKCS7 padding
	padding := int(ciphertext[len(ciphertext)-1])
	if padding > aes.BlockSize || padding == 0 {
		return "", fmt.Errorf("invalid padding")
	}
	ciphertext = ciphertext[:len(ciphertext)-padding]

	// Parse decrypted content
	// Format: random(16) + msgLen(4) + msg + corpId
	if len(ciphertext) < 20 {
		return "", fmt.Errorf("decrypted content too short")
	}

	msgLen := binary.BigEndian.Uint32(ciphertext[16:20])
	if int(msgLen) > len(ciphertext)-20 {
		return "", fmt.Errorf("invalid message length")
	}

	return string(ciphertext[20 : 20+msgLen]), nil
}

// encryptMessage encrypts a message for sending.
func (c *Channel) encryptMessage(msg string) (string, error) {
	aesKey, err := base64.StdEncoding.DecodeString(c.config.EncodingAESKey + "=")
	if err != nil {
		return "", fmt.Errorf("invalid encoding AES key: %w", err)
	}

	// Build plaintext: random(16) + msgLen(4) + msg + corpId
	random := make([]byte, 16)
	msgBytes := []byte(msg)
	corpIdBytes := []byte(c.config.CorpID)

	var buf bytes.Buffer
	buf.Write(random)
	binary.Write(&buf, binary.BigEndian, uint32(len(msgBytes)))
	buf.Write(msgBytes)
	buf.Write(corpIdBytes)

	plaintext := buf.Bytes()

	// Add PKCS7 padding
	padding := aes.BlockSize - len(plaintext)%aes.BlockSize
	for i := 0; i < padding; i++ {
		plaintext = append(plaintext, byte(padding))
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	iv := aesKey[:aes.BlockSize]
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(plaintext, plaintext)

	return base64.StdEncoding.EncodeToString(plaintext), nil
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

	if c.server != nil {
		c.server.Shutdown(ctx)
	}

	close(c.messages)
	c.logger.Info("wechat work channel stopped")
	return nil
}

// Send sends a message through WeChat Work.
func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	captionConsumed := false
	sentSomething := false
	if c.sendTextFunc == nil {
		c.sendTextFunc = c.sendText
	}
	if c.sendAttachmentFunc == nil {
		c.sendAttachmentFunc = c.sendAttachment
	}

	// Send attachments first.
	for _, att := range msg.Attachments {
		caption := ""
		includeCaption := !captionConsumed && strings.TrimSpace(msg.Content) != ""
		if includeCaption {
			caption = msg.Content
		}
		if err := c.sendAttachmentFunc(ctx, msg.ChatID, caption, att); err != nil {
			c.logger.Warn("failed to send attachment, falling back to text",
				zap.String("channel", "wechat_work"), zap.String("type", string(att.Type)), zap.Error(err))
			fallback := wechatAttachmentFallbackText(msg.Content, att, includeCaption)
			if fallback == "" {
				continue
			}
			if err2 := c.sendTextFunc(ctx, msg.ChatID, fallback, msg.Format); err2 != nil {
				return fmt.Errorf("wechat send fallback: %w", err2)
			}
			sentSomething = true
			if includeCaption {
				captionConsumed = true
			}
			continue
		}
		sentSomething = true
		if includeCaption {
			captionConsumed = true
		}
	}

	if strings.TrimSpace(msg.Content) != "" && !captionConsumed {
		if err := c.sendTextFunc(ctx, msg.ChatID, msg.Content, msg.Format); err != nil {
			return err
		}
		sentSomething = true
	}
	if !sentSomething {
		return fmt.Errorf("no sendable WeChat Work content")
	}
	return nil
}

func wechatAttachmentFallbackText(caption string, att channel.Attachment, includeCaption bool) string {
	parts := make([]string, 0, 2)
	if includeCaption && strings.TrimSpace(caption) != "" {
		parts = append(parts, strings.TrimSpace(caption))
	}
	if strings.TrimSpace(att.URL) != "" {
		parts = append(parts, strings.TrimSpace(att.URL))
	} else {
		name := strings.TrimSpace(att.Name)
		if name == "" && len(att.Data) > 0 {
			name = "Attachment"
		}
		if name != "" {
			parts = append(parts, name)
		}
	}
	return strings.Join(parts, "\n")
}

// sendText sends a plain text or markdown message.
func (c *Channel) sendText(ctx context.Context, chatID, content, format string) error {
	token, err := c.getAccessToken()
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	payload := map[string]interface{}{
		"touser":  chatID,
		"agentid": c.config.AgentID,
	}

	if format == "markdown" {
		payload["msgtype"] = "markdown"
		payload["markdown"] = map[string]string{"content": content}
	} else {
		payload["msgtype"] = "text"
		payload["text"] = map[string]string{"content": content}
	}

	return c.postSendMessage(token, payload)
}

// sendAttachment uploads media and sends it via WeChat Work API.
func (c *Channel) sendAttachment(ctx context.Context, chatID, caption string, att channel.Attachment) error {
	if len(att.Data) == 0 {
		return fmt.Errorf("no binary data for attachment")
	}

	token, err := c.getAccessToken()
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	// Map attachment type to WeChat media type.
	var mediaType string
	switch att.Type {
	case channel.MessageTypeImage:
		mediaType = "image"
	case channel.MessageTypeAudio:
		mediaType = "voice"
	case channel.MessageTypeVideo:
		mediaType = "video"
	default:
		mediaType = "file"
	}

	// Upload temporary media.
	mediaID, err := c.uploadMedia(token, mediaType, att.Name, att.Data)
	if err != nil {
		return fmt.Errorf("upload media: %w", err)
	}

	// Build send payload.
	payload := map[string]interface{}{
		"touser":  chatID,
		"msgtype": mediaType,
		"agentid": c.config.AgentID,
	}

	switch mediaType {
	case "image":
		payload["image"] = map[string]string{"media_id": mediaID}
	case "voice":
		payload["voice"] = map[string]string{"media_id": mediaID}
	case "video":
		payload["video"] = map[string]interface{}{
			"media_id":    mediaID,
			"title":       att.Name,
			"description": caption,
		}
	case "file":
		payload["file"] = map[string]string{"media_id": mediaID}
	}

	return c.postSendMessage(token, payload)
}

// uploadMedia uploads a temporary media file to WeChat Work.
// Returns the media_id on success.
func (c *Channel) uploadMedia(token, mediaType, filename string, data []byte) (string, error) {
	if filename == "" {
		filename = "file" + extForType(mediaType)
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("media", filepath.Base(filename))
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return "", fmt.Errorf("write data: %w", err)
	}
	w.Close()

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/media/upload?access_token=%s&type=%s", token, mediaType)
	resp, err := http.Post(url, w.FormDataContentType(), &buf)
	if err != nil {
		return "", fmt.Errorf("upload request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		MediaID string `json:"media_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("WeChat API error: %d - %s", result.ErrCode, result.ErrMsg)
	}
	return result.MediaID, nil
}

// postSendMessage posts a message payload to the WeChat Work send API.
func (c *Channel) postSendMessage(token string, payload map[string]interface{}) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", token)
	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if result.ErrCode != 0 {
		return fmt.Errorf("WeChat API error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	c.msgsSent.Add(1)
	nowSent := time.Now()
	c.mu.Lock()
	c.lastReplyAt = &nowSent
	c.mu.Unlock()
	return nil
}

// extForType returns a default file extension for a WeChat media type.
func extForType(mediaType string) string {
	switch mediaType {
	case "image":
		return ".png"
	case "voice":
		return ".amr"
	case "video":
		return ".mp4"
	default:
		return ".bin"
	}
}

// SendStreaming sends a message with streaming support.
// WeChat Work doesn't support real-time message editing, so we collect all content and send once.
func (c *Channel) SendStreaming(ctx context.Context, chatID string, replyToID string, content <-chan string, done chan<- struct{}) error {
	defer close(done)

	var fullContent strings.Builder

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk, ok := <-content:
			if !ok {
				// Channel closed, send final message
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
		Name:             "wechat_work",
		Type:             "wechat_work",
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
			"corp_id":  c.config.CorpID,
			"agent_id": c.config.AgentID,
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

// Menu represents a WeChat Work application menu.
type Menu struct {
	Button []MenuButton `json:"button"`
}

// MenuButton represents a menu button.
type MenuButton struct {
	Type      string       `json:"type,omitempty"`
	Name      string       `json:"name"`
	Key       string       `json:"key,omitempty"`
	URL       string       `json:"url,omitempty"`
	SubButton []MenuButton `json:"sub_button,omitempty"`
}

// CreateMenu creates the application menu.
func (c *Channel) CreateMenu(ctx context.Context, menu *Menu) error {
	token, err := c.getAccessToken()
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	jsonPayload, err := json.Marshal(menu)
	if err != nil {
		return fmt.Errorf("failed to marshal menu: %w", err)
	}

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/menu/create?access_token=%s&agentid=%s",
		token, c.config.AgentID)

	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create menu: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if result.ErrCode != 0 {
		return fmt.Errorf("WeChat API error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	c.logger.Info("menu created successfully")
	return nil
}

// GetMenu retrieves the current application menu.
func (c *Channel) GetMenu(ctx context.Context) (*Menu, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/menu/get?access_token=%s&agentid=%s",
		token, c.config.AgentID)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get menu: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode int          `json:"errcode"`
		ErrMsg  string       `json:"errmsg"`
		Button  []MenuButton `json:"button"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.ErrCode != 0 {
		return nil, fmt.Errorf("WeChat API error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	return &Menu{Button: result.Button}, nil
}

// DeleteMenu deletes the application menu.
func (c *Channel) DeleteMenu(ctx context.Context) error {
	token, err := c.getAccessToken()
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/menu/delete?access_token=%s&agentid=%s",
		token, c.config.AgentID)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to delete menu: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if result.ErrCode != 0 {
		return fmt.Errorf("WeChat API error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	c.logger.Info("menu deleted successfully")
	return nil
}

// TargetedMessage represents a message with specific targeting options.
type TargetedMessage struct {
	ToUser  string // User IDs separated by |, max 1000
	ToParty string // Department IDs separated by |, max 100
	ToTag   string // Tag IDs separated by |, max 100
	ToAll   bool   // Send to all members (touser, toparty, totag must be empty)
	Content string
	Format  string // "text" or "markdown"
}

// SendTargeted sends a message to specific users, departments, or tags.
func (c *Channel) SendTargeted(ctx context.Context, msg TargetedMessage) error {
	token, err := c.getAccessToken()
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	// Build message payload
	payload := map[string]interface{}{
		"agentid": c.config.AgentID,
	}

	// Set targeting
	if msg.ToAll {
		payload["touser"] = "@all"
	} else {
		if msg.ToUser != "" {
			payload["touser"] = msg.ToUser
		}
		if msg.ToParty != "" {
			payload["toparty"] = msg.ToParty
		}
		if msg.ToTag != "" {
			payload["totag"] = msg.ToTag
		}
	}

	// Set message type and content
	if msg.Format == "markdown" {
		payload["msgtype"] = "markdown"
		payload["markdown"] = map[string]string{
			"content": msg.Content,
		}
	} else {
		payload["msgtype"] = "text"
		payload["text"] = map[string]string{
			"content": msg.Content,
		}
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", token)
	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode      int    `json:"errcode"`
		ErrMsg       string `json:"errmsg"`
		InvalidUser  string `json:"invaliduser"`
		InvalidParty string `json:"invalidparty"`
		InvalidTag   string `json:"invalidtag"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if result.ErrCode != 0 {
		return fmt.Errorf("WeChat API error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	// Log any invalid targets
	if result.InvalidUser != "" {
		c.logger.Warn("some users were invalid", zap.String("invalid_users", result.InvalidUser))
	}
	if result.InvalidParty != "" {
		c.logger.Warn("some departments were invalid", zap.String("invalid_parties", result.InvalidParty))
	}
	if result.InvalidTag != "" {
		c.logger.Warn("some tags were invalid", zap.String("invalid_tags", result.InvalidTag))
	}

	return nil
}

// Department represents a WeChat Work department.
type Department struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	ParentID int    `json:"parentid"`
	Order    int    `json:"order"`
}

// GetDepartmentList retrieves the department list.
func (c *Channel) GetDepartmentList(ctx context.Context, parentID int) ([]Department, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/department/list?access_token=%s", token)
	if parentID > 0 {
		url += fmt.Sprintf("&id=%d", parentID)
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get departments: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode    int          `json:"errcode"`
		ErrMsg     string       `json:"errmsg"`
		Department []Department `json:"department"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.ErrCode != 0 {
		return nil, fmt.Errorf("WeChat API error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	return result.Department, nil
}

// User represents a WeChat Work user.
type User struct {
	UserID     string `json:"userid"`
	Name       string `json:"name"`
	Department []int  `json:"department"`
	Position   string `json:"position"`
	Mobile     string `json:"mobile"`
	Email      string `json:"email"`
	Status     int    `json:"status"` // 1=activated, 2=disabled, 4=not activated
}

// GetUserList retrieves users in a department.
func (c *Channel) GetUserList(ctx context.Context, departmentID int, fetchChild bool) ([]User, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	fetchChildInt := 0
	if fetchChild {
		fetchChildInt = 1
	}

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/user/list?access_token=%s&department_id=%d&fetch_child=%d",
		token, departmentID, fetchChildInt)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode  int    `json:"errcode"`
		ErrMsg   string `json:"errmsg"`
		UserList []User `json:"userlist"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.ErrCode != 0 {
		return nil, fmt.Errorf("WeChat API error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	return result.UserList, nil
}

// GetUser retrieves a specific user's information.
func (c *Channel) GetUser(ctx context.Context, userID string) (*User, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/user/get?access_token=%s&userid=%s",
		token, userID)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		User
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.ErrCode != 0 {
		return nil, fmt.Errorf("WeChat API error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	return &result.User, nil
}

// Tag represents a WeChat Work tag.
type Tag struct {
	TagID   int    `json:"tagid"`
	TagName string `json:"tagname"`
}

// GetTagList retrieves the tag list.
func (c *Channel) GetTagList(ctx context.Context) ([]Tag, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/tag/list?access_token=%s", token)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		TagList []Tag  `json:"taglist"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.ErrCode != 0 {
		return nil, fmt.Errorf("WeChat API error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	return result.TagList, nil
}

// GetTagUsers retrieves users in a tag.
func (c *Channel) GetTagUsers(ctx context.Context, tagID int) ([]string, []int, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get access token: %w", err)
	}

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/tag/get?access_token=%s&tagid=%d",
		token, tagID)

	resp, err := http.Get(url)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get tag users: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode  int    `json:"errcode"`
		ErrMsg   string `json:"errmsg"`
		UserList []struct {
			UserID string `json:"userid"`
			Name   string `json:"name"`
		} `json:"userlist"`
		PartyList []int `json:"partylist"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.ErrCode != 0 {
		return nil, nil, fmt.Errorf("WeChat API error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	userIDs := make([]string, len(result.UserList))
	for i, u := range result.UserList {
		userIDs[i] = u.UserID
	}

	return userIDs, result.PartyList, nil
}
