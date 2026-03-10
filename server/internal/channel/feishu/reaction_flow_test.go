package feishu

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

func TestChannel_OnMessageReceive_AddsAndRemovesTypingReaction(t *testing.T) {
	var (
		mu              sync.Mutex
		calls           []string
		addReactionBody string
	)
	removed := make(chan struct{})

	srv := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/reactions"):
			body, _ := io.ReadAll(r.Body)
			addReactionBody = string(body)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"msg":  "ok",
				"data": map[string]any{"reaction_id": "reaction-1"},
			})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/reply"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"msg":  "ok",
				"data": map[string]any{"message_id": "out-1"},
			})
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/reactions/reaction-1"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"msg":  "ok",
				"data": map[string]any{},
			})
			select {
			case <-removed:
			default:
				close(removed)
			}
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	defer srv.Close()

	ch := New(channel.FeishuConfig{
		Enabled:   true,
		AppID:     "test-app-id",
		AppSecret: "test-app-secret",
	}, zap.NewNop())
	ch.ctx = context.Background()
	ch.client = newLarkClient("test-app-id", "test-app-secret")
	ch.client.baseURL = srv.URL + "/open-apis"
	ch.client.http = srv.Client()
	ch.client.token = "test-token"
	ch.client.tokenExp = time.Now().Add(time.Hour)
	ch.SetMessageHandler(func(ctx context.Context, msg channel.Message) (string, error) {
		return "reply", nil
	})

	ch.onMessageReceive(context.Background(), json.RawMessage(`{
		"message": {
			"message_id": "msg_1",
			"chat_id": "oc_test_chat",
			"chat_type": "p2p",
			"message_type": "text",
			"content": "{\"text\":\"hello\"}",
			"parent_id": ""
		},
		"sender": {
			"sender_id": {
				"open_id": "ou_test_user"
			}
		}
	}`))

	select {
	case <-removed:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for typing reaction removal")
	}

	deadline := time.After(2 * time.Second)
	var got []string
	addIdx := -1
	replyIdx := -1
	delIdx := -1
	for {
		mu.Lock()
		got = append([]string(nil), calls...)
		mu.Unlock()
		addIdx = -1
		replyIdx = -1
		delIdx = -1
		for i, call := range got {
			switch call {
			case "POST /open-apis/im/v1/messages/msg_1/reactions":
				if addIdx == -1 {
					addIdx = i
				}
			case "POST /open-apis/im/v1/messages/msg_1/reply":
				if replyIdx == -1 {
					replyIdx = i
				}
			case "DELETE /open-apis/im/v1/messages/msg_1/reactions/reaction-1":
				if delIdx == -1 {
					delIdx = i
				}
			}
		}
		if addIdx != -1 && delIdx != -1 && replyIdx != -1 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for add/delete/reply sequence, calls: %v", got)
		case <-time.After(20 * time.Millisecond):
		}
	}

	if addIdx == -1 {
		t.Fatalf("add reaction call not found: %v", got)
	}
	if replyIdx == -1 {
		t.Fatalf("reply call not found: %v", got)
	}
	if delIdx == -1 {
		t.Fatalf("delete reaction call not found: %v", got)
	}
	if delIdx <= addIdx {
		t.Fatalf("delete must happen after add, got calls: %v", got)
	}
	if replyIdx <= delIdx {
		// keep this strict: final reply should be sent only after typing reaction is removed.
		t.Fatalf("reply must happen after delete, got calls: %v", got)
	}
	if !strings.Contains(addReactionBody, `"emoji_type":"Typing"`) {
		t.Fatalf("add reaction should use Typing emoji, got body: %s", addReactionBody)
	}
}

func TestChannel_OnMessageReceive_DisableTypingReaction_DoesNotUseReactionAPI(t *testing.T) {
	var (
		mu    sync.Mutex
		calls []string
	)

	srv := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/reply"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"msg":  "ok",
				"data": map[string]any{"message_id": "out-1"},
			})
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	defer srv.Close()

	ch := New(channel.FeishuConfig{
		Enabled:               true,
		AppID:                 "test-app-id",
		AppSecret:             "test-app-secret",
		DisableTypingReaction: true,
	}, zap.NewNop())
	ch.ctx = context.Background()
	ch.client = newLarkClient("test-app-id", "test-app-secret")
	ch.client.baseURL = srv.URL + "/open-apis"
	ch.client.http = srv.Client()
	ch.client.token = "test-token"
	ch.client.tokenExp = time.Now().Add(time.Hour)
	ch.SetMessageHandler(func(ctx context.Context, msg channel.Message) (string, error) {
		return "reply", nil
	})

	ch.onMessageReceive(context.Background(), json.RawMessage(`{
		"message": {
			"message_id": "msg_1",
			"chat_id": "oc_test_chat",
			"chat_type": "p2p",
			"message_type": "text",
			"content": "{\"text\":\"hello\"}",
			"parent_id": ""
		},
		"sender": {
			"sender_id": {
				"open_id": "ou_test_user"
			}
		}
	}`))

	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		got := append([]string(nil), calls...)
		mu.Unlock()
		hasReply := false
		hasReaction := false
		for _, c := range got {
			if c == "POST /open-apis/im/v1/messages/msg_1/reply" {
				hasReply = true
			}
			if c == "POST /open-apis/im/v1/messages/msg_1/reactions" ||
				c == "DELETE /open-apis/im/v1/messages/msg_1/reactions/reaction-1" {
				hasReaction = true
			}
		}
		if hasReply {
			if hasReaction {
				t.Fatalf("typing reaction api should not be called when disabled, got calls: %v", got)
			}
			return
		}

		select {
		case <-deadline:
			t.Fatalf("timed out waiting for reply call, calls: %v", got)
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func TestChannel_OnMessageReceive_EmptyHandlerResponseRemovesTypingReaction(t *testing.T) {
	var (
		mu    sync.Mutex
		calls []string
	)
	removed := make(chan struct{})

	srv := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/reactions"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"msg":  "ok",
				"data": map[string]any{"reaction_id": "reaction-1"},
			})
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/reactions/reaction-1"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"msg":  "ok",
				"data": map[string]any{},
			})
			select {
			case <-removed:
			default:
				close(removed)
			}
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	defer srv.Close()

	ch := New(channel.FeishuConfig{
		Enabled:   true,
		AppID:     "test-app-id",
		AppSecret: "test-app-secret",
	}, zap.NewNop())
	ch.ctx = context.Background()
	ch.client = newLarkClient("test-app-id", "test-app-secret")
	ch.client.baseURL = srv.URL + "/open-apis"
	ch.client.http = srv.Client()
	ch.client.token = "test-token"
	ch.client.tokenExp = time.Now().Add(time.Hour)
	ch.SetMessageHandler(func(ctx context.Context, msg channel.Message) (string, error) {
		return "", nil
	})

	ch.onMessageReceive(context.Background(), json.RawMessage(`{
		"message": {
			"message_id": "msg_1",
			"chat_id": "oc_test_chat",
			"chat_type": "p2p",
			"message_type": "text",
			"content": "{\"text\":\"hello\"}",
			"parent_id": ""
		},
		"sender": {
			"sender_id": {
				"open_id": "ou_test_user"
			}
		}
	}`))

	select {
	case <-removed:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for typing reaction removal")
	}

	mu.Lock()
	got := append([]string(nil), calls...)
	mu.Unlock()
	for _, call := range got {
		if call == "POST /open-apis/im/v1/messages/msg_1/reply" {
			t.Fatalf("did not expect reply call for empty handler response, calls: %v", got)
		}
	}
}
