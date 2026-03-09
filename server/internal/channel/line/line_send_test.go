package line

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

type lineRoundTripFunc func(*http.Request) (*http.Response, error)

func (f lineRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestChannel_Send_AttachmentFallbackConsumesCaption(t *testing.T) {
	var payload map[string]interface{}
	ch := New(Config{ChannelAccessToken: "token"}, zap.NewNop())
	ch.client = &http.Client{Transport: lineRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
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
	messages, _ := payload["messages"].([]interface{})
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	msg, _ := messages[0].(map[string]interface{})
	if got := msg["text"]; got != "caption\nagenda.pdf" {
		t.Fatalf("fallback text = %v, want %q", got, "caption\nagenda.pdf")
	}
}

func TestChannel_Send_ImageKeepsCaptionAsSeparateText(t *testing.T) {
	var payload map[string]interface{}
	ch := New(Config{ChannelAccessToken: "token"}, zap.NewNop())
	ch.client = &http.Client{Transport: lineRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
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
	messages, _ := payload["messages"].([]interface{})
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	first, _ := messages[0].(map[string]interface{})
	second, _ := messages[1].(map[string]interface{})
	if first["type"] != "image" {
		t.Fatalf("first type = %v, want image", first["type"])
	}
	if second["text"] != "caption" {
		t.Fatalf("second text = %v, want caption", second["text"])
	}
}

func TestChannel_Send_ReplyToIDUsesReplyEndpoint(t *testing.T) {
	var (
		payload map[string]interface{}
		pathHit string
	)
	ch := New(Config{ChannelAccessToken: "token"}, zap.NewNop())
	ch.client = &http.Client{Transport: lineRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		pathHit = req.URL.Path
		body, _ := io.ReadAll(req.Body)
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:    "user-1",
		ReplyToID: "reply-token-1",
		Content:   "caption",
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if pathHit != "/v2/bot/message/reply" {
		t.Fatalf("path = %q, want reply endpoint", pathHit)
	}
	if payload["replyToken"] != "reply-token-1" {
		t.Fatalf("replyToken = %v", payload["replyToken"])
	}
	if _, ok := payload["to"]; ok {
		t.Fatalf("unexpected push target in reply payload: %#v", payload)
	}
}

func TestChannel_SendStreaming_PreservesReplyToID(t *testing.T) {
	var payload map[string]interface{}
	ch := New(Config{ChannelAccessToken: "token"}, zap.NewNop())
	ch.client = &http.Client{Transport: lineRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})}

	content := make(chan string, 1)
	content <- "stream"
	close(content)
	done := make(chan struct{})
	if err := ch.SendStreaming(context.Background(), "user-1", "reply-token-1", content, done); err != nil {
		t.Fatalf("SendStreaming error = %v", err)
	}
	if payload["replyToken"] != "reply-token-1" {
		t.Fatalf("replyToken = %v", payload["replyToken"])
	}
}
