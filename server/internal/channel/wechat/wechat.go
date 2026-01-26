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
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel"
)

// Channel implements the channel.Channel interface for WeChat Work.
type Channel struct {
	config   channel.WeChatWorkConfig
	logger   *zap.Logger
	messages chan channel.Message

	mu          sync.RWMutex
	status      channel.Status
	connectedAt *time.Time
	lastError   string
	lastErrorAt *time.Time
	msgCount    atomic.Int64

	accessToken     string
	tokenExpireTime time.Time
	tokenMu         sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	server *http.Server
}

// New creates a new WeChat Work channel.
func New(cfg channel.WeChatWorkConfig, logger *zap.Logger) *Channel {
	return &Channel{
		config:   cfg,
		logger:   logger.With(zap.String("channel", "wechat_work")),
		messages: make(chan channel.Message, 100),
		status:   channel.StatusDisconnected,
	}
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "wechat_work"
}

// Type returns the channel type.
func (c *Channel) Type() string {
	return "wechat_work"
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
			ID:   msg.MediaId,
			Type: msgType,
			URL:  msg.PicUrl,
		})
	}

	return channelMsg
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
	token, err := c.getAccessToken()
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	// Build message payload
	payload := map[string]interface{}{
		"touser":  msg.ChatID,
		"msgtype": "text",
		"agentid": c.config.AgentID,
		"text": map[string]string{
			"content": msg.Content,
		},
	}

	// Check for markdown format
	if msg.Format == "markdown" {
		payload["msgtype"] = "markdown"
		payload["markdown"] = map[string]string{
			"content": msg.Content,
		}
		delete(payload, "text")
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
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if result.ErrCode != 0 {
		return fmt.Errorf("WeChat API error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	return nil
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
		Name:         "wechat_work",
		Type:         "wechat_work",
		Status:       c.status,
		Enabled:      c.config.Enabled,
		ConnectedAt:  c.connectedAt,
		LastError:    c.lastError,
		LastErrorAt:  c.lastErrorAt,
		MessageCount: c.msgCount.Load(),
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
