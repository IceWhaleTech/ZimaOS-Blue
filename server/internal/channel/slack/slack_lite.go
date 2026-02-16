package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	ws "github.com/gorilla/websocket"
)

// slackClient is a lightweight Slack API client replacing github.com/slack-go/slack.
type slackClient struct {
	botToken string
	appToken string
	baseURL  string
	http     *http.Client
}

func newSlackClient(botToken, appToken string) *slackClient {
	return &slackClient{
		botToken: botToken,
		appToken: appToken,
		baseURL:  "https://slack.com/api",
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *slackClient) apiCall(ctx context.Context, method string, body interface{}) (json.RawMessage, error) {
	var reqBody io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/"+method, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.botToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var r struct {
		OK    bool            `json:"ok"`
		Error string          `json:"error,omitempty"`
		Data  json.RawMessage `json:"-"`
	}
	json.Unmarshal(data, &r)
	if !r.OK {
		return nil, fmt.Errorf("slack: %s", r.Error)
	}
	return data, nil
}

type authTestResp struct {
	UserID string `json:"user_id"`
	Team   string `json:"team"`
}

func (c *slackClient) authTest(ctx context.Context) (*authTestResp, error) {
	data, err := c.apiCall(ctx, "auth.test", nil)
	if err != nil {
		return nil, err
	}
	var r authTestResp
	json.Unmarshal(data, &r)
	return &r, nil
}

func (c *slackClient) postMessage(ctx context.Context, channelID, text, threadTS string) (string, error) {
	body := map[string]interface{}{"channel": channelID, "text": text}
	if threadTS != "" {
		body["thread_ts"] = threadTS
	}
	data, err := c.apiCall(ctx, "chat.postMessage", body)
	if err != nil {
		return "", err
	}
	var r struct{ TS string `json:"ts"` }
	json.Unmarshal(data, &r)
	return r.TS, nil
}

func (c *slackClient) updateMessage(ctx context.Context, channelID, ts, text string) error {
	_, err := c.apiCall(ctx, "chat.update", map[string]interface{}{
		"channel": channelID, "ts": ts, "text": text,
	})
	return err
}

func (c *slackClient) getUserInfo(ctx context.Context, userID string) (string, error) {
	data, err := c.apiCall(ctx, "users.info", map[string]string{"user": userID})
	if err != nil {
		return userID, err
	}
	var r struct {
		User struct{ Name string `json:"name"` } `json:"user"`
	}
	json.Unmarshal(data, &r)
	if r.User.Name != "" {
		return r.User.Name, nil
	}
	return userID, nil
}

// slackSocketClient handles Socket Mode WebSocket connection.
type slackSocketClient struct {
	appToken string
	baseURL  string
	conn     *ws.Conn
	mu       sync.Mutex
	onEvent  func(envelope socketEnvelope)
}

type socketEnvelope struct {
	Type       string          `json:"type"`
	EnvelopeID string          `json:"envelope_id,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	// For events_api type
	RetryAttempt int `json:"retry_attempt,omitempty"`
}

func newSlackSocketClient(appToken string, onEvent func(socketEnvelope)) *slackSocketClient {
	return &slackSocketClient{
		appToken: appToken,
		baseURL:  "https://slack.com/api",
		onEvent:  onEvent,
	}
}

func (c *slackSocketClient) connect(ctx context.Context) error {
	req, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/apps.connections.open", nil)
	req.Header.Set("Authorization", "Bearer "+c.appToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var r struct {
		OK  bool   `json:"ok"`
		URL string `json:"url"`
		Err string `json:"error,omitempty"`
	}
	json.NewDecoder(resp.Body).Decode(&r)
	if !r.OK {
		return fmt.Errorf("slack socket: %s", r.Err)
	}
	conn, _, err := ws.DefaultDialer.DialContext(ctx, r.URL, nil)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	return nil
}

func (c *slackSocketClient) run(ctx context.Context) error {
	if err := c.connect(ctx); err != nil {
		return err
	}
	defer func() {
		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		c.mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()
		if conn == nil {
			return fmt.Errorf("connection closed")
		}
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// Reconnect
			if reconnErr := c.connect(ctx); reconnErr != nil {
				return reconnErr
			}
			continue
		}
		var env socketEnvelope
		if err := json.Unmarshal(msg, &env); err != nil {
			continue
		}
		if env.EnvelopeID != "" {
			c.ack(env.EnvelopeID)
		}
		if c.onEvent != nil {
			go c.onEvent(env)
		}
	}
}

func (c *slackSocketClient) ack(envelopeID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return
	}
	resp, _ := json.Marshal(map[string]string{"envelope_id": envelopeID})
	c.conn.WriteMessage(ws.TextMessage, resp)
}
