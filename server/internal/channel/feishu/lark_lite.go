package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"
	ws "github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// larkClient is a lightweight Feishu/Lark API client replacing larksuite/oapi-sdk-go/v3.
// Handles: tenant_access_token, send message, download resource.
type larkClient struct {
	appID     string
	appSecret string
	baseURL   string
	token     string
	tokenExp  time.Time
	mu        sync.Mutex
	http      *http.Client
}

func newLarkClient(appID, appSecret string) *larkClient {
	return &larkClient{
		appID:     appID,
		appSecret: appSecret,
		baseURL:   "https://open.feishu.cn/open-apis",
		http:      network.NewPooledHTTPClient(5 * time.Minute),
	}
}

func (c *larkClient) getToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.tokenExp) {
		return c.token, nil
	}
	body, _ := json.Marshal(map[string]string{"app_id": c.appID, "app_secret": c.appSecret})
	req, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/auth/v3/tenant_access_token/internal", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var r struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", err
	}
	if r.Code != 0 {
		return "", fmt.Errorf("feishu auth: %d %s", r.Code, r.Msg)
	}
	c.token = r.TenantAccessToken
	c.tokenExp = time.Now().Add(time.Duration(r.Expire-60) * time.Second)
	return c.token, nil
}

type larkAPIResp struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type larkBotInfo struct {
	BotName string
	OpenID  string
}

func (c *larkClient) getBotInfo(ctx context.Context) (larkBotInfo, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return larkBotInfo{}, err
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/bot/v3/info", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.http.Do(req)
	if err != nil {
		return larkBotInfo{}, err
	}
	defer resp.Body.Close()

	var r struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Bot  struct {
			BotName string `json:"bot_name"`
			OpenID  string `json:"open_id"`
		} `json:"bot"`
		Data struct {
			Bot struct {
				BotName string `json:"bot_name"`
				OpenID  string `json:"open_id"`
			} `json:"bot"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return larkBotInfo{}, err
	}
	if r.Code != 0 {
		return larkBotInfo{}, fmt.Errorf("feishu bot info: %d %s", r.Code, r.Msg)
	}
	bot := r.Bot
	if bot.OpenID == "" && bot.BotName == "" {
		bot = r.Data.Bot
	}
	return larkBotInfo{
		BotName: strings.TrimSpace(bot.BotName),
		OpenID:  strings.TrimSpace(bot.OpenID),
	}, nil
}

func (c *larkClient) addMessageReaction(ctx context.Context, messageID string, emojiType string) (string, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return "", err
	}
	body, _ := json.Marshal(map[string]any{
		"reaction_type": map[string]string{
			"emoji_type": emojiType,
		},
	})
	u := fmt.Sprintf("%s/im/v1/messages/%s/reactions", c.baseURL, url.PathEscape(messageID))
	req, _ := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var r larkAPIResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", err
	}
	if r.Code != 0 {
		return "", fmt.Errorf("feishu add reaction: %d %s", r.Code, r.Msg)
	}
	var data struct {
		ReactionID string `json:"reaction_id"`
	}
	if len(r.Data) > 0 {
		_ = json.Unmarshal(r.Data, &data)
	}
	return data.ReactionID, nil
}

func (c *larkClient) deleteMessageReaction(ctx context.Context, messageID string, reactionID string) error {
	token, err := c.getToken(ctx)
	if err != nil {
		return err
	}
	u := fmt.Sprintf("%s/im/v1/messages/%s/reactions/%s", c.baseURL, url.PathEscape(messageID), url.PathEscape(reactionID))
	req, _ := http.NewRequestWithContext(ctx, "DELETE", u, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var r larkAPIResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return err
	}
	if r.Code != 0 {
		return fmt.Errorf("feishu delete reaction: %d %s", r.Code, r.Msg)
	}
	return nil
}

func (c *larkClient) sendMessage(ctx context.Context, receiveIDType, receiveID, msgType, content, replyToID string) error {
	_, err := c.sendMessageWithID(ctx, receiveIDType, receiveID, msgType, content, replyToID)
	return err
}

func (c *larkClient) sendMessageWithID(ctx context.Context, receiveIDType, receiveID, msgType, content, replyToID string) (string, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return "", err
	}
	var u string
	var body []byte
	if replyToID != "" {
		// Use reply endpoint to quote the original message
		payload := map[string]string{
			"msg_type": msgType,
			"content":  content,
		}
		body, _ = json.Marshal(payload)
		u = fmt.Sprintf("%s/im/v1/messages/%s/reply", c.baseURL, replyToID)
	} else {
		payload := map[string]string{
			"receive_id": receiveID,
			"msg_type":   msgType,
			"content":    content,
		}
		body, _ = json.Marshal(payload)
		u = fmt.Sprintf("%s/im/v1/messages?receive_id_type=%s", c.baseURL, receiveIDType)
	}
	req, _ := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var r larkAPIResp
	json.NewDecoder(resp.Body).Decode(&r)
	if r.Code != 0 {
		return "", fmt.Errorf("feishu send: %d %s", r.Code, r.Msg)
	}
	var data struct {
		MessageID string `json:"message_id"`
	}
	if len(r.Data) > 0 {
		_ = json.Unmarshal(r.Data, &data)
	}
	return data.MessageID, nil
}

func (c *larkClient) updateMessage(ctx context.Context, messageID, msgType, content string) error {
	token, err := c.getToken(ctx)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]string{
		"msg_type": msgType,
		"content":  content,
	})
	u := fmt.Sprintf("%s/im/v1/messages/%s", c.baseURL, url.PathEscape(messageID))
	req, _ := http.NewRequestWithContext(ctx, "PATCH", u, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var r larkAPIResp
	json.NewDecoder(resp.Body).Decode(&r)
	if r.Code != 0 {
		return fmt.Errorf("feishu update: %d %s", r.Code, r.Msg)
	}
	return nil
}

// uploadImage uploads an image to Feishu and returns the image_key.
// API: POST /im/v1/images  (multipart/form-data: image_type=message, image=<binary>)
func (c *larkClient) uploadImage(ctx context.Context, data []byte, mimeType string) (string, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return "", err
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("image_type", "message")
	ext := ".png"
	if strings.Contains(mimeType, "jpeg") || strings.Contains(mimeType, "jpg") {
		ext = ".jpg"
	} else if strings.Contains(mimeType, "webp") {
		ext = ".webp"
	} else if strings.Contains(mimeType, "gif") {
		ext = ".gif"
	}
	part, err := writer.CreateFormFile("image", "image"+ext)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	writer.Close()

	req, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/im/v1/images", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var r struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			ImageKey string `json:"image_key"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", err
	}
	if r.Code != 0 {
		return "", fmt.Errorf("feishu upload image: %d %s", r.Code, r.Msg)
	}
	return r.Data.ImageKey, nil
}

// uploadFile uploads a file to Feishu and returns the file_key.
// API: POST /im/v1/files  (multipart/form-data: file_type=<type>, file=<binary>, file_name=<name>)
func (c *larkClient) uploadFile(ctx context.Context, data []byte, fileName, fileType string) (string, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return "", err
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("file_type", fileType)
	_ = writer.WriteField("file_name", fileName)
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	writer.Close()

	req, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/im/v1/files", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var r struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			FileKey string `json:"file_key"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", err
	}
	if r.Code != 0 {
		return "", fmt.Errorf("feishu upload file: %d %s", r.Code, r.Msg)
	}
	return r.Data.FileKey, nil
}

func (c *larkClient) getMessageResource(ctx context.Context, messageID, fileKey, resType string) ([]byte, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s/im/v1/messages/%s/resources/%s?type=%s", c.baseURL, messageID, fileKey, resType)
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("feishu resource: HTTP %d: %s", resp.StatusCode, b)
	}
	return io.ReadAll(resp.Body)
}

// larkWSClient is a lightweight WebSocket client for Feishu long-polling events.
type larkWSClient struct {
	appID     string
	appSecret string
	baseURL   string
	logger    *zap.Logger
	http      *http.Client
	conn      *ws.Conn
	serviceID string
	connID    string
	mu        sync.Mutex

	onEvent func(ctx context.Context, payload []byte)

	reconnectCount    int
	reconnectInterval time.Duration
	reconnectNonce    int
	pingInterval      time.Duration

	// message combining cache
	combineCache sync.Map
}

type combineEntry struct {
	parts [][]byte
	exp   time.Time
}

func newLarkWSClient(appID, appSecret string, logger *zap.Logger, onEvent func(ctx context.Context, payload []byte)) *larkWSClient {
	return &larkWSClient{
		appID:             appID,
		appSecret:         appSecret,
		baseURL:           "https://open.feishu.cn",
		logger:            logger,
		http:              network.NewPooledHTTPClient(5 * time.Minute),
		onEvent:           onEvent,
		reconnectCount:    -1,
		reconnectInterval: 2 * time.Minute,
		reconnectNonce:    30,
		pingInterval:      2 * time.Minute,
	}
}

func (c *larkWSClient) start(ctx context.Context) error {
	if err := c.connect(ctx); err != nil {
		c.disconnect()
		if c.reconnectCount != 0 {
			return c.reconnectLoop(ctx)
		}
		return err
	}
	go c.pingLoop(ctx)
	<-ctx.Done()
	return ctx.Err()
}

func (c *larkWSClient) connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return nil
	}

	body, _ := json.Marshal(map[string]string{"AppID": c.appID, "AppSecret": c.appSecret})
	req, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/callback/ws/endpoint", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("feishu ws endpoint: read body: %w", err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("feishu ws endpoint: HTTP %d: %s", resp.StatusCode, respBody)
	}
	var endResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data *struct {
			URL          string `json:"URL"`
			ClientConfig *struct {
				ReconnectCount    int `json:"ReconnectCount"`
				ReconnectInterval int `json:"ReconnectInterval"`
				ReconnectNonce    int `json:"ReconnectNonce"`
				PingInterval      int `json:"PingInterval"`
			} `json:"ClientConfig"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &endResp); err != nil {
		return fmt.Errorf("feishu ws endpoint: decode: %w (body: %s)", err, respBody)
	}
	if endResp.Code != 0 {
		return fmt.Errorf("feishu ws endpoint: %d %s", endResp.Code, endResp.Msg)
	}
	if endResp.Data == nil || endResp.Data.URL == "" {
		return fmt.Errorf("feishu ws: empty endpoint URL (response: %s)", respBody)
	}

	u, _ := url.Parse(endResp.Data.URL)
	c.connID = u.Query().Get("device_id")
	c.serviceID = u.Query().Get("service_id")

	if endResp.Data.ClientConfig != nil {
		cc := endResp.Data.ClientConfig
		if cc.ReconnectCount != 0 {
			c.reconnectCount = cc.ReconnectCount
		}
		if cc.ReconnectInterval > 0 {
			c.reconnectInterval = time.Duration(cc.ReconnectInterval) * time.Second
		}
		if cc.ReconnectNonce > 0 {
			c.reconnectNonce = cc.ReconnectNonce
		}
		if cc.PingInterval > 0 {
			c.pingInterval = time.Duration(cc.PingInterval) * time.Second
		}
	}

	conn, wsResp, err := ws.DefaultDialer.Dial(endResp.Data.URL, nil)
	if err != nil {
		return err
	}
	if wsResp.StatusCode != http.StatusSwitchingProtocols {
		return fmt.Errorf("feishu ws: unexpected status %d", wsResp.StatusCode)
	}
	c.conn = conn
	c.logger.Info("feishu ws connected", zap.String("conn_id", c.connID))

	go c.receiveLoop(ctx)
	return nil
}

func (c *larkWSClient) disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

func (c *larkWSClient) reconnectLoop(ctx context.Context) error {
	if c.reconnectNonce > 0 {
		time.Sleep(time.Duration(rand.Intn(c.reconnectNonce*1000)) * time.Millisecond)
	}
	for i := 0; c.reconnectCount < 0 || i < c.reconnectCount; i++ {
		c.logger.Info("feishu ws reconnecting", zap.Int("attempt", i+1))
		if err := c.connect(ctx); err == nil {
			c.logger.Info("feishu ws reconnected successfully", zap.Int("attempt", i+1))
			go c.pingLoop(ctx)
			<-ctx.Done()
			return ctx.Err()
		} else {
			c.logger.Warn("feishu ws reconnect failed", zap.Int("attempt", i+1), zap.Error(err))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(c.reconnectInterval):
		}
	}
	return fmt.Errorf("feishu ws: reconnect exhausted")
}

func (c *larkWSClient) pingLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(c.pingInterval):
		}
		c.mu.Lock()
		if c.conn == nil {
			c.mu.Unlock()
			return
		}
		sid, _ := strconv.ParseInt(c.serviceID, 10, 32)
		headers := wsHeaders{}
		headers.add("type", "ping")
		frame := &wsFrame{Method: 0, Service: int32(sid), Headers: []wsHeader(headers)}
		bs, _ := frame.marshal()
		err := c.conn.WriteMessage(ws.BinaryMessage, bs)
		c.mu.Unlock()
		if err != nil {
			c.logger.Warn("feishu ws ping failed", zap.Error(err))
		}
	}
}

func (c *larkWSClient) receiveLoop(ctx context.Context) {
	defer func() {
		c.disconnect()
		if c.reconnectCount != 0 {
			c.reconnectLoop(ctx)
		}
	}()
	for {
		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()
		if conn == nil {
			return
		}
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			c.logger.Error("feishu ws read error", zap.Error(err))
			return
		}
		if mt != ws.BinaryMessage {
			continue
		}
		// Process frames serially to preserve event order on a single websocket stream.
		c.handleFrame(ctx, msg)
	}
}

func (c *larkWSClient) handleFrame(ctx context.Context, msg []byte) {
	defer func() { recover() }()

	var frame wsFrame
	if err := frame.unmarshal(msg); err != nil {
		c.logger.Error("feishu ws unmarshal error", zap.Error(err))
		return
	}

	hs := wsHeaders(frame.Headers)
	switch frame.Method {
	case 0: // control
		// pong — ignore
	case 1: // data
		msgType := hs.getString("type")
		if msgType != "event" {
			return
		}
		msgID := hs.getString("message_id")
		sum := hs.getInt("sum")
		seq := hs.getInt("seq")

		payload := frame.Payload
		if sum > 1 {
			payload = c.combine(msgID, sum, seq, payload)
			if payload == nil {
				return
			}
		}

		if c.onEvent != nil {
			c.onEvent(ctx, payload)
		}

		// Send ack response
		resp, _ := json.Marshal(map[string]interface{}{"code": 200})
		frame.Payload = resp
		bs, _ := frame.marshal()
		c.mu.Lock()
		if c.conn != nil {
			c.conn.WriteMessage(ws.BinaryMessage, bs)
		}
		c.mu.Unlock()
	}
}

func (c *larkWSClient) combine(msgID string, sum, seq int, data []byte) []byte {
	val, _ := c.combineCache.LoadOrStore(msgID, &combineEntry{
		parts: make([][]byte, sum),
		exp:   time.Now().Add(5 * time.Second),
	})
	entry := val.(*combineEntry)
	entry.parts[seq] = data

	total := 0
	for _, p := range entry.parts {
		if len(p) == 0 {
			return nil
		}
		total += len(p)
	}
	c.combineCache.Delete(msgID)

	combined := make([]byte, 0, total)
	for _, p := range entry.parts {
		combined = append(combined, p...)
	}
	return combined
}
