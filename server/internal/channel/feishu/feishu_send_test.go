package feishu

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

func TestChannel_Send_CardMetadataUsesInteractiveCard(t *testing.T) {
	var gotBody []byte
	srv := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = body
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"msg":  "ok",
			"data": map[string]any{"message_id": "out_1"},
		})
	}))
	defer srv.Close()

	ch := New(channel.FeishuConfig{Enabled: true, AppID: "test-app-id", AppSecret: "test-app-secret"}, zap.NewNop())
	ch.client = newLarkClient("test-app-id", "test-app-secret")
	ch.client.baseURL = srv.URL + "/open-apis"
	ch.client.http = srv.Client()
	ch.client.token = "test-token"
	ch.client.tokenExp = time.Now().Add(time.Hour)

	card := map[string]any{
		"config":   map[string]any{"wide_screen_mode": true},
		"elements": []map[string]any{{"tag": "markdown", "content": "hello card"}},
	}
	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID: "oc_test_chat",
		Metadata: map[string]interface{}{
			"card": card,
		},
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	var payload struct {
		MsgType string `json:"msg_type"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("invalid send payload: %v", err)
	}
	if payload.MsgType != "interactive" {
		t.Fatalf("expected interactive msg_type, got %q", payload.MsgType)
	}
	if !strings.Contains(payload.Content, "hello card") {
		t.Fatalf("unexpected card content: %s", payload.Content)
	}
}

func TestChannel_Send_AttachmentWithoutDataFallsBackToText(t *testing.T) {
	var gotBody []byte
	srv := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = body
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"msg":  "ok",
			"data": map[string]any{"message_id": "out_1"},
		})
	}))
	defer srv.Close()

	ch := New(channel.FeishuConfig{Enabled: true, AppID: "test-app-id", AppSecret: "test-app-secret"}, zap.NewNop())
	ch.client = newLarkClient("test-app-id", "test-app-secret")
	ch.client.baseURL = srv.URL + "/open-apis"
	ch.client.http = srv.Client()
	ch.client.token = "test-token"
	ch.client.tokenExp = time.Now().Add(time.Hour)

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID: "oc_test_chat",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeFile,
			Name: "agenda.pdf",
		}},
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	var payload struct {
		MsgType string `json:"msg_type"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("invalid send payload: %v", err)
	}
	if payload.MsgType != "text" {
		t.Fatalf("expected text msg_type, got %q", payload.MsgType)
	}
	if !strings.Contains(payload.Content, "agenda.pdf") {
		t.Fatalf("unexpected text content: %s", payload.Content)
	}
}
