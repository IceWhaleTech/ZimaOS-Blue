package wechatilink

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

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

		var req struct {
			Msg struct {
				ToUserID     string `json:"to_user_id"`
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
