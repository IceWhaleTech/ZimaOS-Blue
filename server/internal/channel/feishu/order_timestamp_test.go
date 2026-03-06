package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	ws "github.com/gorilla/websocket"
	"go.uber.org/zap"
)

func TestLarkWSClient_ReceiveLoop_ProcessesFramesSequentially(t *testing.T) {
	upgrader := ws.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	serverSentBoth := make(chan struct{})
	srv := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		if err := conn.WriteMessage(ws.BinaryMessage, mustMarshalEventFrame(t, "msg-1", []byte(`{"order":1}`))); err != nil {
			return
		}
		if err := conn.WriteMessage(ws.BinaryMessage, mustMarshalEventFrame(t, "msg-2", []byte(`{"order":2}`))); err != nil {
			return
		}
		close(serverSentBoth)

		_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		for i := 0; i < 2; i++ {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := ws.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws failed: %v", err)
	}
	defer conn.Close()

	var (
		mu      sync.Mutex
		seenRaw []string
	)
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondSeen := make(chan struct{})

	client := &larkWSClient{
		logger:         zap.NewNop(),
		conn:           conn,
		reconnectCount: 0,
		onEvent: func(_ context.Context, payload []byte) {
			mu.Lock()
			seenRaw = append(seenRaw, string(payload))
			idx := len(seenRaw)
			mu.Unlock()

			if idx == 1 {
				close(firstStarted)
				<-releaseFirst
			}
			if idx == 2 {
				close(secondSeen)
			}
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	loopDone := make(chan struct{})
	go func() {
		client.receiveLoop(ctx)
		close(loopDone)
	}()

	select {
	case <-firstStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first event was not received in time")
	}

	select {
	case <-serverSentBoth:
	case <-time.After(2 * time.Second):
		t.Fatal("server did not send both frames in time")
	}

	select {
	case <-secondSeen:
		t.Fatal("second event should not be handled before first handler returns")
	case <-time.After(200 * time.Millisecond):
	}

	close(releaseFirst)

	select {
	case <-secondSeen:
	case <-time.After(2 * time.Second):
		t.Fatal("second event was not received after releasing first handler")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(seenRaw) != 2 {
		t.Fatalf("expected 2 events, got %d (%v)", len(seenRaw), seenRaw)
	}
	if seenRaw[0] != `{"order":1}` || seenRaw[1] != `{"order":2}` {
		t.Fatalf("unexpected event order: %v", seenRaw)
	}
}

func TestChannel_EndToEnd_WebSocketTwentyMessages_OrderAndTimestamp(t *testing.T) {
	const total = 20
	const baseUnixMs int64 = 1730000000000

	upgrader := ws.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	serverSentAll := make(chan struct{})
	srv := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		for i := 1; i <= total; i++ {
			payload := buildMessageReceiveEventPayload(i, baseUnixMs+int64(i))
			if err := conn.WriteMessage(ws.BinaryMessage, mustMarshalEventFrame(t, fmt.Sprintf("frame-%02d", i), payload)); err != nil {
				return
			}
		}
		close(serverSentAll)

		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		for i := 0; i < total; i++ {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	ch := New(channel.FeishuConfig{
		Enabled:   true,
		AppID:     "test-app-id",
		AppSecret: "test-app-secret",
	}, zap.NewNop())
	ch.ctx = context.Background()
	ch.mu.Lock()
	ch.status = channel.StatusConnected
	ch.mu.Unlock()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := ws.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws failed: %v", err)
	}
	defer conn.Close()

	client := &larkWSClient{
		logger:         zap.NewNop(),
		conn:           conn,
		reconnectCount: 0,
		onEvent:        ch.onWSEvent,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	loopDone := make(chan struct{})
	go func() {
		client.receiveLoop(ctx)
		close(loopDone)
	}()

	select {
	case <-serverSentAll:
	case <-time.After(2 * time.Second):
		t.Fatal("server did not send all frames in time")
	}

	got := make([]channel.Message, 0, total)
	timeout := time.After(3 * time.Second)
	for len(got) < total {
		select {
		case msg := <-ch.Messages():
			got = append(got, msg)
		case <-timeout:
			t.Fatalf("timed out waiting for messages, got=%d", len(got))
		}
	}

	for i := 1; i <= total; i++ {
		msg := got[i-1]
		wantID := fmt.Sprintf("msg_%02d", i)
		wantContent := fmt.Sprintf("m%02d", i)
		wantTs := time.UnixMilli(baseUnixMs + int64(i))
		if msg.ID != wantID {
			t.Fatalf("message[%d] id mismatch: got=%q want=%q", i-1, msg.ID, wantID)
		}
		if msg.Content != wantContent {
			t.Fatalf("message[%d] content mismatch: got=%q want=%q", i-1, msg.Content, wantContent)
		}
		if !msg.Timestamp.Equal(wantTs) {
			t.Fatalf("message[%d] timestamp mismatch: got=%s want=%s",
				i-1, msg.Timestamp.Format(time.RFC3339Nano), wantTs.Format(time.RFC3339Nano))
		}
	}

	for i := 1; i < len(got); i++ {
		if got[i].Timestamp.Before(got[i-1].Timestamp) {
			t.Fatalf("timestamp order regressed at index %d: prev=%s cur=%s",
				i,
				got[i-1].Timestamp.Format(time.RFC3339Nano),
				got[i].Timestamp.Format(time.RFC3339Nano))
		}
	}
}

func TestChannel_OnMessageReceive_UsesCreateTimeTimestamp(t *testing.T) {
	ch := New(channel.FeishuConfig{
		Enabled:   true,
		AppID:     "test-app-id",
		AppSecret: "test-app-secret",
	}, zap.NewNop())
	ch.ctx = context.Background()

	got := make(chan channel.Message, 1)
	ch.SetMessageHandler(func(ctx context.Context, msg channel.Message) (string, error) {
		got <- msg
		return "", nil
	})

	ch.onMessageReceive(context.Background(), json.RawMessage(`{
		"message": {
			"message_id": "msg_1",
			"chat_id": "oc_test_chat",
			"chat_type": "p2p",
			"message_type": "text",
			"content": "{\"text\":\"hello\"}",
			"parent_id": "",
			"create_time": "1730000000123"
		},
		"sender": {
			"sender_id": {
				"open_id": "ou_test_user"
			}
		}
	}`))

	select {
	case msg := <-got:
		want := time.UnixMilli(1730000000123)
		if !msg.Timestamp.Equal(want) {
			t.Fatalf("timestamp mismatch, want %s, got %s", want.Format(time.RFC3339Nano), msg.Timestamp.Format(time.RFC3339Nano))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("message handler was not called")
	}
}

func TestChannel_OnMessageReceive_InvalidCreateTimeFallsBackToNow(t *testing.T) {
	ch := New(channel.FeishuConfig{
		Enabled:   true,
		AppID:     "test-app-id",
		AppSecret: "test-app-secret",
	}, zap.NewNop())
	ch.ctx = context.Background()

	got := make(chan channel.Message, 1)
	ch.SetMessageHandler(func(ctx context.Context, msg channel.Message) (string, error) {
		got <- msg
		return "", nil
	})

	start := time.Now()
	ch.onMessageReceive(context.Background(), json.RawMessage(`{
		"message": {
			"message_id": "msg_1",
			"chat_id": "oc_test_chat",
			"chat_type": "p2p",
			"message_type": "text",
			"content": "{\"text\":\"hello\"}",
			"parent_id": "",
			"create_time": "not-a-number"
		},
		"sender": {
			"sender_id": {
				"open_id": "ou_test_user"
			}
		}
	}`))

	select {
	case msg := <-got:
		end := time.Now()
		if msg.Timestamp.Before(start.Add(-200*time.Millisecond)) || msg.Timestamp.After(end.Add(200*time.Millisecond)) {
			t.Fatalf("timestamp should fallback to now, got %s not in [%s, %s]",
				msg.Timestamp.Format(time.RFC3339Nano),
				start.Add(-200*time.Millisecond).Format(time.RFC3339Nano),
				end.Add(200*time.Millisecond).Format(time.RFC3339Nano))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("message handler was not called")
	}
}

func mustMarshalEventFrame(t *testing.T, messageID string, payload []byte) []byte {
	t.Helper()

	headers := wsHeaders{}
	headers.add("type", "event")
	headers.add("message_id", messageID)
	headers.add("sum", "1")
	headers.add("seq", "0")

	frame := &wsFrame{
		Method:  1,
		Service: 1,
		Headers: []wsHeader(headers),
		Payload: payload,
	}
	bs, err := frame.marshal()
	if err != nil {
		t.Fatalf("marshal ws frame failed: %v", err)
	}
	return bs
}

func buildMessageReceiveEventPayload(i int, createTimeMs int64) []byte {
	return []byte(fmt.Sprintf(`{
		"schema":"2.0",
		"header":{"event_type":"im.message.receive_v1"},
		"event":{
			"message":{
				"message_id":"msg_%02d",
				"chat_id":"oc_test_chat",
				"chat_type":"p2p",
				"message_type":"text",
				"content":"{\"text\":\"m%02d\"}",
				"parent_id":"",
				"create_time":"%d"
			},
			"sender":{"sender_id":{"open_id":"ou_test_user"}}
		}
	}`, i, i, createTimeMs))
}
