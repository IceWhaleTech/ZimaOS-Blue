package messenger

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

type messengerRoundTripFunc func(*http.Request) (*http.Response, error)

func (f messengerRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestChannel_Send_ImagePreservesCaptionAsText(t *testing.T) {
	var payloads []sendRequest
	ch := New(Config{PageAccessToken: "token"}, zap.NewNop())
	ch.client = &http.Client{Transport: messengerRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var payload sendRequest
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
	if payloads[1].Message.Text != "caption" {
		t.Fatalf("second payload text = %q, want caption", payloads[1].Message.Text)
	}
}

func TestChannel_Send_AttachmentWithoutURLFallsBackToName(t *testing.T) {
	var payloads []sendRequest
	ch := New(Config{PageAccessToken: "token"}, zap.NewNop())
	ch.client = &http.Client{Transport: messengerRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var payload sendRequest
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
	if payloads[0].Message.Text != "caption\nagenda.pdf" {
		t.Fatalf("fallback text = %q, want caption+name", payloads[0].Message.Text)
	}
}

func TestChannel_ProcessMessage_PreservesReplyReference(t *testing.T) {
	ch := New(Config{}, zap.NewNop())
	ch.processMessage(messagingEvent{
		Sender:    webhookUser{ID: "user-1"},
		Recipient: webhookUser{ID: "page-1"},
		Timestamp: time.Now().UnixMilli(),
		Message: &incomingMessage{
			MID:     "msg-2",
			Text:    "hello",
			ReplyTo: &incomingReplyTo{MID: "msg-1"},
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
		t.Fatal("timed out waiting for messenger message")
	}
}

func TestChannel_ProcessMessage_SkipsEcho(t *testing.T) {
	ch := New(Config{}, zap.NewNop())
	ch.processMessage(messagingEvent{
		Sender:    webhookUser{ID: "user-1"},
		Recipient: webhookUser{ID: "page-1"},
		Timestamp: time.Now().UnixMilli(),
		Message: &incomingMessage{
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
