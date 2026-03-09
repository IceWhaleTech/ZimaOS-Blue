package instagram

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

type instagramRoundTripFunc func(*http.Request) (*http.Response, error)

func (f instagramRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestChannel_Send_ImagePreservesCaptionAsText(t *testing.T) {
	var payloads []map[string]interface{}
	ch := New(Config{PageAccessToken: "token"}, zap.NewNop())
	ch.client = &http.Client{Transport: instagramRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var payload map[string]interface{}
		body, _ := io.ReadAll(req.Body)
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		payloads = append(payloads, payload)
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "user-1",
		Content: "caption",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeImage,
			URL:  "https://example.com/image.png",
		}},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if len(payloads) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(payloads))
	}
	message, _ := payloads[1]["message"].(map[string]interface{})
	if message["text"] != "caption" {
		t.Fatalf("text payload = %#v, want caption", payloads[1])
	}
}

func TestChannel_Send_AttachmentWithoutURLFallsBackToName(t *testing.T) {
	var payloads []map[string]interface{}
	ch := New(Config{PageAccessToken: "token"}, zap.NewNop())
	ch.client = &http.Client{Transport: instagramRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var payload map[string]interface{}
		body, _ := io.ReadAll(req.Body)
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		payloads = append(payloads, payload)
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "user-1",
		Content: "caption",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeFile,
			Name: "agenda.pdf",
		}},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if len(payloads) != 1 {
		t.Fatalf("expected 1 request, got %d", len(payloads))
	}
	message, _ := payloads[0]["message"].(map[string]interface{})
	if message["text"] != "caption\nagenda.pdf" {
		t.Fatalf("fallback text = %#v, want caption+name", payloads[0])
	}
}

func TestChannel_ProcessMessage_PreservesReplyReference(t *testing.T) {
	ch := New(Config{}, zap.NewNop())
	ch.processMessage(webhookMessaging{
		Sender:    webhookUser{ID: "user-1"},
		Recipient: webhookUser{ID: "biz-1"},
		Timestamp: time.Now().UnixMilli(),
		Message: &webhookMessage{
			MID:     "msg-2",
			Text:    "hello",
			ReplyTo: &webhookReplyTo{MID: "msg-1"},
		},
	})

	select {
	case got := <-ch.Messages():
		if got.ReplyToID != "msg-1" {
			t.Fatalf("ReplyToID = %q, want msg-1", got.ReplyToID)
		}
		if got.Metadata["reply_to_mid"] != "msg-1" {
			t.Fatalf("reply_to_mid = %v", got.Metadata["reply_to_mid"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for instagram message")
	}
}

func TestChannel_ProcessMessage_SkipsEcho(t *testing.T) {
	ch := New(Config{}, zap.NewNop())
	ch.processMessage(webhookMessaging{
		Sender:    webhookUser{ID: "user-1"},
		Recipient: webhookUser{ID: "biz-1"},
		Timestamp: time.Now().UnixMilli(),
		Message: &webhookMessage{
			MID:    "msg-echo",
			Text:   "hello",
			IsEcho: true,
		},
	})

	select {
	case got := <-ch.Messages():
		t.Fatalf("unexpected echo message queued: %#v", got)
	case <-time.After(150 * time.Millisecond):
	}
}
