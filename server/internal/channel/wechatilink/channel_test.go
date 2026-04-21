package wechatilink

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTCP4Server(tb testing.TB, handler http.Handler) *httptest.Server {
	tb.Helper()
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		tb.Skipf("skip test server setup (tcp4 unavailable): %v", err)
	}
	srv := httptest.NewUnstartedServer(handler)
	srv.Listener = ln
	srv.Start()
	return srv
}

func decodeILinkUINHeader(tb testing.TB, raw string) uint32 {
	tb.Helper()

	if strings.TrimSpace(raw) == "" {
		tb.Fatal("expected X-WECHAT-UIN header to be set")
	}

	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		tb.Fatalf("decode X-WECHAT-UIN base64: %v", err)
	}

	value, err := strconv.ParseUint(string(decoded), 10, 32)
	if err != nil {
		tb.Fatalf("parse decoded X-WECHAT-UIN as uint32: %v (decoded=%q)", err, string(decoded))
	}

	return uint32(value)
}

func TestChannel_NameAndType(t *testing.T) {
	ch := New(channel.WeChatILinkConfig{
		Enabled:    true,
		APIBaseURL: "https://ilink.example.com",
		BotToken:   "bot-token",
	}, zap.NewNop())

	if ch.Name() != "wechat_ilink" {
		t.Fatalf("Name = %q, want %q", ch.Name(), "wechat_ilink")
	}
	if ch.Type() != "wechat_ilink" {
		t.Fatalf("Type = %q, want %q", ch.Type(), "wechat_ilink")
	}
	info := ch.Info()
	if got := info.Metadata["api_base_url"]; got != "https://ilink.example.com" {
		t.Fatalf("api_base_url = %v, want %q", got, "https://ilink.example.com")
	}
}

func TestChannel_Send_UsesBotAPI(t *testing.T) {
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/sendmessage" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if got := r.Header.Get("AuthorizationType"); got != "ilink_bot_token" {
			t.Fatalf("AuthorizationType = %q, want %q", got, "ilink_bot_token")
		}
		if got := r.Header.Get("Authorization"); got != "Bearer bot-token" {
			t.Fatalf("Authorization = %q, want Bearer bot-token", got)
		}
		if got := r.Header.Get("X-WECHAT-UIN"); strings.TrimSpace(got) == "" {
			t.Fatal("expected X-WECHAT-UIN header to be set")
		}
		if got := r.Header.Get("iLink-App-Id"); got != "bot" {
			t.Fatalf("iLink-App-Id = %q, want %q", got, "bot")
		}
		if got := r.Header.Get("iLink-App-ClientVersion"); strings.TrimSpace(got) == "" {
			t.Fatal("expected iLink-App-ClientVersion header to be set")
		}
		if got := strings.TrimSpace(r.Header.Get("iLink-App-ClientVersion")); got == "0" {
			t.Fatal("expected iLink-App-ClientVersion to be non-zero")
		}

		var req struct {
			BaseInfo struct {
				ChannelVersion string `json:"channel_version"`
			} `json:"base_info"`
			Msg struct {
				FromUserID   string `json:"from_user_id"`
				ToUserID     string `json:"to_user_id"`
				ClientID     string `json:"client_id"`
				MessageType  int    `json:"message_type"`
				MessageState int    `json:"message_state"`
				ContextToken string `json:"context_token"`
				ItemList     []struct {
					Type     int `json:"type"`
					TextItem struct {
						Text string `json:"text"`
					} `json:"text_item"`
				} `json:"item_list"`
			} `json:"msg"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Msg.ToUserID != "wxid-user" {
			t.Fatalf("to_user_id = %q, want %q", req.Msg.ToUserID, "wxid-user")
		}
		if req.Msg.FromUserID != "" {
			t.Fatalf("from_user_id = %q, want empty", req.Msg.FromUserID)
		}
		if strings.TrimSpace(req.Msg.ClientID) == "" {
			t.Fatal("expected client_id to be set")
		}
		if req.Msg.MessageType != 2 {
			t.Fatalf("message_type = %d, want %d", req.Msg.MessageType, 2)
		}
		if req.Msg.MessageState != 2 {
			t.Fatalf("message_state = %d, want %d", req.Msg.MessageState, 2)
		}
		if strings.TrimSpace(req.BaseInfo.ChannelVersion) == "" {
			t.Fatal("expected base_info.channel_version to be set")
		}
		if req.BaseInfo.ChannelVersion == "unknown" {
			t.Fatal("expected base_info.channel_version to avoid unknown fallback")
		}
		if req.Msg.ContextToken != "ctx-123" {
			t.Fatalf("context_token = %q, want %q", req.Msg.ContextToken, "ctx-123")
		}
		if len(req.Msg.ItemList) != 1 || req.Msg.ItemList[0].Type != 1 {
			t.Fatalf("unexpected item_list: %#v", req.Msg.ItemList)
		}
		if req.Msg.ItemList[0].TextItem.Text != "hello from Blue" {
			t.Fatalf("text = %q, want %q", req.Msg.ItemList[0].TextItem.Text, "hello from Blue")
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"ret": 0,
		})
	}))
	defer server.Close()

	ch := New(channel.WeChatILinkConfig{
		Enabled:    true,
		APIBaseURL: server.URL,
		BotToken:   "bot-token",
	}, zap.NewNop())

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "wxid-user",
		Content: "hello from Blue",
		Metadata: map[string]any{
			"context_token": "ctx-123",
		},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if got := ch.Info().MessagesSent; got != 1 {
		t.Fatalf("MessagesSent = %d, want 1", got)
	}
}

func TestChannel_Send_UsesChatIDByDefaultEvenWhenConfiguredUserIDPresent(t *testing.T) {
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/sendmessage" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var req struct {
			Msg struct {
				FromUserID   string `json:"from_user_id"`
				ToUserID     string `json:"to_user_id"`
				ClientID     string `json:"client_id"`
				MessageType  int    `json:"message_type"`
				MessageState int    `json:"message_state"`
			} `json:"msg"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Msg.ToUserID != "wxid-user" {
			t.Fatalf("to_user_id = %q, want %q", req.Msg.ToUserID, "wxid-user")
		}
		if req.Msg.FromUserID != "" {
			t.Fatalf("from_user_id = %q, want empty", req.Msg.FromUserID)
		}
		if strings.TrimSpace(req.Msg.ClientID) == "" {
			t.Fatal("expected client_id to be set")
		}
		if req.Msg.MessageType != 2 {
			t.Fatalf("message_type = %d, want %d", req.Msg.MessageType, 2)
		}
		if req.Msg.MessageState != 2 {
			t.Fatalf("message_state = %d, want %d", req.Msg.MessageState, 2)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"ret": 0,
		})
	}))
	defer server.Close()

	ch := New(channel.WeChatILinkConfig{
		Enabled:    true,
		APIBaseURL: server.URL,
		BotToken:   "bot-token",
		UserID:     "userdb-user-id",
	}, zap.NewNop())

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "wxid-user",
		Content: "hello from Blue",
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
}

func TestChannel_Send_UsesFreshDecimalStringUINHeaderPerRequest(t *testing.T) {
	headers := make([]string, 0, 2)

	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/sendmessage" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		headers = append(headers, r.Header.Get("X-WECHAT-UIN"))

		_ = json.NewEncoder(w).Encode(map[string]any{
			"ret": 0,
		})
	}))
	defer server.Close()

	ch := New(channel.WeChatILinkConfig{
		Enabled:    true,
		APIBaseURL: server.URL,
		BotToken:   "bot-token",
	}, zap.NewNop())

	for i := 0; i < 2; i++ {
		if err := ch.Send(context.Background(), channel.OutgoingMessage{
			ChatID:  "wxid-user",
			Content: "hello from Blue",
		}); err != nil {
			t.Fatalf("Send error on call %d = %v", i+1, err)
		}
	}

	if len(headers) != 2 {
		t.Fatalf("expected 2 X-WECHAT-UIN headers, got %d", len(headers))
	}

	first := decodeILinkUINHeader(t, headers[0])
	second := decodeILinkUINHeader(t, headers[1])
	if headers[0] == headers[1] {
		t.Fatalf("expected fresh X-WECHAT-UIN per request, got same header %q (%d, %d)", headers[0], first, second)
	}
}

func TestChannel_Send_AllowsExplicitTargetUserIDOverride(t *testing.T) {
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/sendmessage" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var req struct {
			Msg struct {
				FromUserID   string `json:"from_user_id"`
				ToUserID     string `json:"to_user_id"`
				ClientID     string `json:"client_id"`
				MessageType  int    `json:"message_type"`
				MessageState int    `json:"message_state"`
			} `json:"msg"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Msg.ToUserID != "explicit-target-user-id" {
			t.Fatalf("to_user_id = %q, want %q", req.Msg.ToUserID, "explicit-target-user-id")
		}
		if req.Msg.FromUserID != "" {
			t.Fatalf("from_user_id = %q, want empty", req.Msg.FromUserID)
		}
		if strings.TrimSpace(req.Msg.ClientID) == "" {
			t.Fatal("expected client_id to be set")
		}
		if req.Msg.MessageType != 2 {
			t.Fatalf("message_type = %d, want %d", req.Msg.MessageType, 2)
		}
		if req.Msg.MessageState != 2 {
			t.Fatalf("message_state = %d, want %d", req.Msg.MessageState, 2)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"ret": 0,
		})
	}))
	defer server.Close()

	ch := New(channel.WeChatILinkConfig{
		Enabled:    true,
		APIBaseURL: server.URL,
		BotToken:   "bot-token",
		UserID:     "userdb-user-id",
	}, zap.NewNop())

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "wxid-user",
		Content: "hello from Blue",
		Metadata: map[string]any{
			"target_user_id": "explicit-target-user-id",
		},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
}

func TestChannel_Send_RetriesTransientTransportErrorWithSameClientID(t *testing.T) {
	var attempts atomic.Int32
	clientIDs := make([]string, 0, 2)

	ch := New(channel.WeChatILinkConfig{
		Enabled:    true,
		APIBaseURL: "https://ilinkai.weixin.qq.com",
		BotToken:   "bot-token",
	}, zap.NewNop())
	ch.httpClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/ilink/bot/sendmessage" {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}

			var body struct {
				Msg struct {
					ClientID string `json:"client_id"`
				} `json:"msg"`
			}
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			clientIDs = append(clientIDs, body.Msg.ClientID)

			if attempts.Add(1) == 1 {
				return nil, io.EOF
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"ret":0}`)),
			}, nil
		}),
	}

	if err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "wxid-user",
		Content: "hello from Blue",
	}); err != nil {
		t.Fatalf("Send error = %v", err)
	}

	if got := attempts.Load(); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
	if len(clientIDs) != 2 {
		t.Fatalf("expected 2 client_id observations, got %d", len(clientIDs))
	}
	if clientIDs[0] == "" || clientIDs[1] == "" {
		t.Fatalf("expected non-empty client_id values, got %#v", clientIDs)
	}
	if clientIDs[0] != clientIDs[1] {
		t.Fatalf("expected retry to reuse client_id, got %q then %q", clientIDs[0], clientIDs[1])
	}
}

func TestChannel_Send_RetriesTransientHTTPStatusWithSameClientID(t *testing.T) {
	var attempts atomic.Int32
	clientIDs := make([]string, 0, 2)

	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/sendmessage" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var body struct {
			Msg struct {
				ClientID string `json:"client_id"`
			} `json:"msg"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		clientIDs = append(clientIDs, body.Msg.ClientID)

		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`temporary outage`))
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"ret": 0,
		})
	}))
	defer server.Close()

	ch := New(channel.WeChatILinkConfig{
		Enabled:    true,
		APIBaseURL: server.URL,
		BotToken:   "bot-token",
	}, zap.NewNop())

	if err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "wxid-user",
		Content: "hello from Blue",
	}); err != nil {
		t.Fatalf("Send error = %v", err)
	}

	if got := attempts.Load(); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
	if len(clientIDs) != 2 {
		t.Fatalf("expected 2 client_id observations, got %d", len(clientIDs))
	}
	if clientIDs[0] == "" || clientIDs[1] == "" {
		t.Fatalf("expected non-empty client_id values, got %#v", clientIDs)
	}
	if clientIDs[0] != clientIDs[1] {
		t.Fatalf("expected retry to reuse client_id, got %q then %q", clientIDs[0], clientIDs[1])
	}
}

func TestChannel_Send_DoesNotRetryNonTransientILinkError(t *testing.T) {
	var attempts atomic.Int32

	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/sendmessage" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		attempts.Add(1)

		_ = json.NewEncoder(w).Encode(map[string]any{
			"ret":    1,
			"errmsg": "business failure",
		})
	}))
	defer server.Close()

	ch := New(channel.WeChatILinkConfig{
		Enabled:    true,
		APIBaseURL: server.URL,
		BotToken:   "bot-token",
	}, zap.NewNop())

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "wxid-user",
		Content: "hello from Blue",
	})
	if err == nil {
		t.Fatal("expected Send to fail")
	}
	if !strings.Contains(err.Error(), "business failure") {
		t.Fatalf("expected business failure error, got %v", err)
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("attempts = %d, want 1", got)
	}
}

func TestChannel_HandleILinkIncomingMessages_DeduplicatesSameMessageID(t *testing.T) {
	ch := New(channel.WeChatILinkConfig{
		Enabled:    true,
		APIBaseURL: "https://ilink.example.com",
		BotToken:   "bot-token",
	}, zap.NewNop())
	ch.ctx = context.Background()

	incoming := iLinkMessage{
		MessageID:    101,
		FromUserID:   "wxid-user",
		ToUserID:     "wxid-bot",
		CreateTimeMS: 1710000000123,
		MessageType:  iLinkMessageTypeUser,
		MessageState: 0,
		ItemList: []iLinkMessageItem{
			{
				Type:     iLinkItemTypeText,
				TextItem: &iLinkTextItem{Text: "hello"},
			},
		},
	}

	ch.handleILinkIncomingMessages([]iLinkMessage{incoming})
	ch.handleILinkIncomingMessages([]iLinkMessage{incoming})

	select {
	case msg := <-ch.Messages():
		if msg.ID != "101" {
			t.Fatalf("ID = %q, want %q", msg.ID, "101")
		}
	default:
		t.Fatal("expected first inbound iLink message")
	}

	select {
	case msg := <-ch.Messages():
		t.Fatalf("unexpected duplicate inbound message: %#v", msg)
	case <-time.After(100 * time.Millisecond):
	}

	if got := ch.Info().MessagesReceived; got != 1 {
		t.Fatalf("MessagesReceived = %d, want 1", got)
	}
}

func TestChannel_HandleILinkIncomingMessages_AllowsDistinctMessageIDs(t *testing.T) {
	ch := New(channel.WeChatILinkConfig{
		Enabled:    true,
		APIBaseURL: "https://ilink.example.com",
		BotToken:   "bot-token",
	}, zap.NewNop())
	ch.ctx = context.Background()

	first := iLinkMessage{
		MessageID:    101,
		FromUserID:   "wxid-user",
		ToUserID:     "wxid-bot",
		CreateTimeMS: 1710000000123,
		MessageType:  iLinkMessageTypeUser,
		MessageState: 0,
		ItemList: []iLinkMessageItem{
			{
				Type:     iLinkItemTypeText,
				TextItem: &iLinkTextItem{Text: "hello"},
			},
		},
	}
	second := first
	second.MessageID = 102

	ch.handleILinkIncomingMessages([]iLinkMessage{first, second})

	select {
	case msg := <-ch.Messages():
		if msg.ID != "101" {
			t.Fatalf("first ID = %q, want %q", msg.ID, "101")
		}
	default:
		t.Fatal("expected first inbound iLink message")
	}

	select {
	case msg := <-ch.Messages():
		if msg.ID != "102" {
			t.Fatalf("second ID = %q, want %q", msg.ID, "102")
		}
	default:
		t.Fatal("expected second inbound iLink message")
	}

	if got := ch.Info().MessagesReceived; got != 2 {
		t.Fatalf("MessagesReceived = %d, want 2", got)
	}
}

func TestChannel_HandleILinkIncomingMessages_AllowsDistinctZeroIDMessages(t *testing.T) {
	ch := New(channel.WeChatILinkConfig{
		Enabled:    true,
		APIBaseURL: "https://ilink.example.com",
		BotToken:   "bot-token",
	}, zap.NewNop())
	ch.ctx = context.Background()

	first := iLinkMessage{
		FromUserID:   "wxid-user",
		ToUserID:     "wxid-bot",
		CreateTimeMS: 1710000000123,
		MessageType:  iLinkMessageTypeUser,
		MessageState: 0,
		ItemList: []iLinkMessageItem{
			{
				Type:     iLinkItemTypeText,
				TextItem: &iLinkTextItem{Text: "hello"},
			},
		},
	}
	second := first
	second.ItemList = []iLinkMessageItem{
		{
			Type:     iLinkItemTypeText,
			TextItem: &iLinkTextItem{Text: "world"},
		},
	}

	ch.handleILinkIncomingMessages([]iLinkMessage{first, second})

	select {
	case msg := <-ch.Messages():
		if msg.Content != "hello" {
			t.Fatalf("first Content = %q, want %q", msg.Content, "hello")
		}
	default:
		t.Fatal("expected first zero-id inbound iLink message")
	}

	select {
	case msg := <-ch.Messages():
		if msg.Content != "world" {
			t.Fatalf("second Content = %q, want %q", msg.Content, "world")
		}
	default:
		t.Fatal("expected second zero-id inbound iLink message")
	}

	if got := ch.Info().MessagesReceived; got != 2 {
		t.Fatalf("MessagesReceived = %d, want 2", got)
	}
}

func TestChannel_Start_PollsIncomingMessages(t *testing.T) {
	var calls atomic.Int32
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/getupdates" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if got := r.Header.Get("AuthorizationType"); got != "ilink_bot_token" {
			t.Fatalf("AuthorizationType = %q, want %q", got, "ilink_bot_token")
		}
		if got := r.Header.Get("Authorization"); got != "Bearer bot-token" {
			t.Fatalf("Authorization = %q, want Bearer bot-token", got)
		}
		if got := r.Header.Get("iLink-App-Id"); got != "bot" {
			t.Fatalf("iLink-App-Id = %q, want %q", got, "bot")
		}
		if got := r.Header.Get("iLink-App-ClientVersion"); strings.TrimSpace(got) == "" {
			t.Fatal("expected iLink-App-ClientVersion header to be set")
		}

		var req struct {
			GetUpdatesBuf string `json:"get_updates_buf"`
			BaseInfo      struct {
				ChannelVersion string `json:"channel_version"`
			} `json:"base_info"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if strings.TrimSpace(req.BaseInfo.ChannelVersion) == "" {
			t.Fatal("expected base_info.channel_version to be set")
		}

		call := calls.Add(1)
		if call == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ret":             0,
				"get_updates_buf": "cursor-1",
				"msgs": []map[string]any{
					{
						"message_id":     101,
						"from_user_id":   "wxid-user",
						"to_user_id":     "wxid-bot",
						"session_id":     "session-1",
						"context_token":  "ctx-1",
						"create_time_ms": float64(1710000000123),
						"message_type":   1,
						"message_state":  0,
						"item_list": []map[string]any{
							{
								"type": 1,
								"text_item": map[string]any{
									"text": "你好，Blue",
								},
							},
						},
					},
				},
				"longpolling_timeout_ms": 1,
			})
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"ret":             0,
			"msgs":            []any{},
			"get_updates_buf": "cursor-1",
		})
	}))
	defer server.Close()

	ch := New(channel.WeChatILinkConfig{
		Enabled:    true,
		APIBaseURL: server.URL,
		BotToken:   "bot-token",
	}, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ch.Start(ctx); err != nil {
		t.Fatalf("Start error = %v", err)
	}
	defer func() {
		if err := ch.Stop(context.Background()); err != nil {
			t.Fatalf("Stop error = %v", err)
		}
	}()

	select {
	case msg := <-ch.Messages():
		if msg.ChannelName != "wechat_ilink" {
			t.Fatalf("ChannelName = %q, want %q", msg.ChannelName, "wechat_ilink")
		}
		if msg.ChatID != "wxid-user" {
			t.Fatalf("ChatID = %q, want %q", msg.ChatID, "wxid-user")
		}
		if msg.UserID != "wxid-user" {
			t.Fatalf("UserID = %q, want %q", msg.UserID, "wxid-user")
		}
		if msg.Content != "你好，Blue" {
			t.Fatalf("Content = %q, want %q", msg.Content, "你好，Blue")
		}
		if got := msg.Metadata["context_token"]; got != "ctx-1" {
			t.Fatalf("context_token = %v, want %q", got, "ctx-1")
		}
		if got := msg.Metadata["target_user_id"]; got != "wxid-user" {
			t.Fatalf("target_user_id = %v, want %q", got, "wxid-user")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for iLink message")
	}
}

func TestChannel_Start_ReturnsAuthErrorBeforeConnecting(t *testing.T) {
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/getupdates" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ret":    1,
			"errmsg": "unauthorized",
		})
	}))
	defer server.Close()

	ch := New(channel.WeChatILinkConfig{
		Enabled:    true,
		APIBaseURL: server.URL,
		BotToken:   "bad-token",
	}, zap.NewNop())

	err := ch.Start(context.Background())
	if err == nil {
		t.Fatal("expected Start to fail for invalid iLink credentials")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("expected authentication error, got %v", err)
	}

	info := ch.Info()
	if info.Status != channel.StatusError {
		t.Fatalf("Status = %q, want %q", info.Status, channel.StatusError)
	}
	if info.ConnectedAt != nil {
		t.Fatal("expected ConnectedAt to remain nil")
	}
}

func TestChannel_Start_ContinuesWhenStartupProbeTimesOut(t *testing.T) {
	oldProbeTimeout := iLinkStartupProbeTimeout
	oldHTTPTimeout := iLinkHTTPTimeout
	t.Cleanup(func() {
		iLinkStartupProbeTimeout = oldProbeTimeout
		iLinkHTTPTimeout = oldHTTPTimeout
	})

	iLinkStartupProbeTimeout = 20 * time.Millisecond
	iLinkHTTPTimeout = 200 * time.Millisecond

	var calls atomic.Int32
	secondRequestStarted := make(chan struct{}, 1)
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/getupdates" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		call := calls.Add(1)
		if call == 1 {
			time.Sleep(50 * time.Millisecond)
			return
		}

		select {
		case secondRequestStarted <- struct{}{}:
		default:
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"ret":                    0,
			"msgs":                   []any{},
			"get_updates_buf":        "cursor-1",
			"longpolling_timeout_ms": 1,
		})
	}))
	defer server.Close()

	ch := New(channel.WeChatILinkConfig{
		Enabled:    true,
		APIBaseURL: server.URL,
		BotToken:   "bot-token",
	}, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ch.Start(ctx); err != nil {
		t.Fatalf("Start error = %v", err)
	}
	defer func() {
		if err := ch.Stop(context.Background()); err != nil {
			t.Fatalf("Stop error = %v", err)
		}
	}()

	select {
	case <-secondRequestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("expected background iLink poller to continue after startup probe timeout")
	}

	info := ch.Info()
	if info.Status != channel.StatusConnected {
		t.Fatalf("Status = %q, want %q", info.Status, channel.StatusConnected)
	}
	if info.ConnectedAt == nil {
		t.Fatal("expected ConnectedAt to be set after successful start")
	}
}

func TestChannel_Start_ContinuesWhenStartupProbeReturnsEOF(t *testing.T) {
	secondRequestStarted := make(chan struct{}, 1)
	var calls atomic.Int32

	ch := New(channel.WeChatILinkConfig{
		Enabled:    true,
		APIBaseURL: "https://ilinkai.weixin.qq.com",
		BotToken:   "bot-token",
	}, zap.NewNop())
	ch.httpClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/ilink/bot/getupdates" {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}

			call := calls.Add(1)
			if call == 1 {
				return nil, io.EOF
			}

			select {
			case secondRequestStarted <- struct{}{}:
			default:
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(
					`{"ret":0,"msgs":[],"get_updates_buf":"cursor-1","longpolling_timeout_ms":1}`,
				)),
			}, nil
		}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ch.Start(ctx); err != nil {
		t.Fatalf("Start error = %v", err)
	}
	defer func() {
		if err := ch.Stop(context.Background()); err != nil {
			t.Fatalf("Stop error = %v", err)
		}
	}()

	select {
	case <-secondRequestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("expected background iLink poller to continue after startup EOF")
	}

	info := ch.Info()
	if info.Status != channel.StatusConnected {
		t.Fatalf("Status = %q, want %q", info.Status, channel.StatusConnected)
	}
	if info.ConnectedAt == nil {
		t.Fatal("expected ConnectedAt to be set after successful start")
	}
}

func TestChannel_ilinkGetUpdates_RetriesTransientTransportError(t *testing.T) {
	var attempts atomic.Int32
	cursors := make([]string, 0, 2)

	ch := New(channel.WeChatILinkConfig{
		Enabled:    true,
		APIBaseURL: "https://ilinkai.weixin.qq.com",
		BotToken:   "bot-token",
	}, zap.NewNop())
	ch.httpClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/ilink/bot/getupdates" {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}

			var body struct {
				GetUpdatesBuf string `json:"get_updates_buf"`
			}
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			cursors = append(cursors, body.GetUpdatesBuf)

			if attempts.Add(1) == 1 {
				return nil, io.EOF
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(
					`{"ret":0,"msgs":[],"get_updates_buf":"cursor-2","longpolling_timeout_ms":1}`,
				)),
			}, nil
		}),
	}

	resp, err := ch.ilinkGetUpdates(context.Background(), "cursor-1")
	if err != nil {
		t.Fatalf("ilinkGetUpdates error = %v", err)
	}
	if got := attempts.Load(); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
	if len(cursors) != 2 || cursors[0] != "cursor-1" || cursors[1] != "cursor-1" {
		t.Fatalf("expected retry to reuse cursor-1, got %#v", cursors)
	}
	if resp.GetUpdatesBuf != "cursor-2" {
		t.Fatalf("get_updates_buf = %q, want %q", resp.GetUpdatesBuf, "cursor-2")
	}
}

func TestChannel_ilinkGetUpdates_RetriesTransientHTTPStatus(t *testing.T) {
	var attempts atomic.Int32
	cursors := make([]string, 0, 2)

	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/getupdates" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var body struct {
			GetUpdatesBuf string `json:"get_updates_buf"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		cursors = append(cursors, body.GetUpdatesBuf)

		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`temporary outage`))
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"ret":                    0,
			"msgs":                   []any{},
			"get_updates_buf":        "cursor-2",
			"longpolling_timeout_ms": 1,
		})
	}))
	defer server.Close()

	ch := New(channel.WeChatILinkConfig{
		Enabled:    true,
		APIBaseURL: server.URL,
		BotToken:   "bot-token",
	}, zap.NewNop())

	resp, err := ch.ilinkGetUpdates(context.Background(), "cursor-1")
	if err != nil {
		t.Fatalf("ilinkGetUpdates error = %v", err)
	}
	if got := attempts.Load(); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
	if len(cursors) != 2 || cursors[0] != "cursor-1" || cursors[1] != "cursor-1" {
		t.Fatalf("expected retry to reuse cursor-1, got %#v", cursors)
	}
	if resp.GetUpdatesBuf != "cursor-2" {
		t.Fatalf("get_updates_buf = %q, want %q", resp.GetUpdatesBuf, "cursor-2")
	}
}

func TestChannel_Start_DoesNotDuplicateBotSubpath(t *testing.T) {
	var calls atomic.Int32
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/getupdates" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ret":             0,
			"msgs":            []any{},
			"get_updates_buf": "cursor-1",
		})
	}))
	defer server.Close()

	ch := New(channel.WeChatILinkConfig{
		Enabled:    true,
		APIBaseURL: server.URL + "/ilink/bot",
		BotToken:   "bot-token",
	}, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ch.Start(ctx); err != nil {
		t.Fatalf("Start error = %v", err)
	}
	defer func() {
		if err := ch.Stop(context.Background()); err != nil {
			t.Fatalf("Stop error = %v", err)
		}
	}()

	if calls.Load() == 0 {
		t.Fatal("expected getupdates probe call")
	}
}

func TestILinkPollErrorBackoffDelay_CapsAndGrows(t *testing.T) {
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 0, want: 250 * time.Millisecond},
		{attempt: 1, want: 250 * time.Millisecond},
		{attempt: 2, want: 500 * time.Millisecond},
		{attempt: 3, want: 1 * time.Second},
		{attempt: 4, want: 2 * time.Second},
		{attempt: 5, want: 4 * time.Second},
		{attempt: 6, want: 5 * time.Second},
		{attempt: 9, want: 5 * time.Second},
	}

	for _, tt := range tests {
		if got := iLinkPollErrorBackoffDelay(tt.attempt); got != tt.want {
			t.Fatalf("iLinkPollErrorBackoffDelay(%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}
