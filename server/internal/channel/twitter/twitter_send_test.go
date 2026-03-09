package twitter

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

type twitterRoundTripFunc func(*http.Request) (*http.Response, error)

func (f twitterRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestChannel_Send_URLOnlyAttachmentFallsBackToText(t *testing.T) {
	var payload map[string]interface{}
	ch := New(Config{}, zap.NewNop())
	ch.client = &http.Client{Transport: twitterRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "42",
		Content: "caption",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeFile,
			URL:  "https://example.com/file.pdf",
		}},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if payload["text"] != "caption\nhttps://example.com/file.pdf" {
		t.Fatalf("text = %v, want caption+url", payload["text"])
	}
}

func TestChannel_Send_NoSendableContentErrors(t *testing.T) {
	ch := New(Config{}, zap.NewNop())
	ch.client = &http.Client{Transport: twitterRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID: "42",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeFile,
		}},
	})
	if err == nil {
		t.Fatal("expected no sendable content error")
	}
}

func TestChannel_ProcessMessage_UsesParticipantIDForReplies(t *testing.T) {
	ch := New(Config{}, zap.NewNop())
	ch.selfUserID = "bot-1"
	ch.selfUsername = "bot"

	ch.processMessage(dmEvent{
		ID:        "evt-1",
		EventType: "MessageCreate",
		Text:      "hello",
		SenderID:  "user-1",
		DMID:      "conv-1",
		CreatedAt: time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC),
	}, nil)

	select {
	case got := <-ch.Messages():
		if got.ChatID != "user-1" {
			t.Fatalf("ChatID = %q, want participant id", got.ChatID)
		}
		if got.Metadata["dm_conversation_id"] != "conv-1" {
			t.Fatalf("dm_conversation_id = %v", got.Metadata["dm_conversation_id"])
		}
		if got.Metadata["participant_id"] != "user-1" {
			t.Fatalf("participant_id = %v", got.Metadata["participant_id"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for twitter message")
	}
}

func TestChannel_ProcessMessage_SkipsSelfSentEvents(t *testing.T) {
	ch := New(Config{}, zap.NewNop())
	ch.selfUserID = "bot-1"

	ch.processMessage(dmEvent{
		ID:        "evt-2",
		EventType: "MessageCreate",
		Text:      "hello",
		SenderID:  "bot-1",
		DMID:      "conv-1",
		CreatedAt: time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC),
	}, nil)

	select {
	case got := <-ch.Messages():
		t.Fatalf("unexpected self message queued: %#v", got)
	case <-time.After(150 * time.Millisecond):
	}
}

func TestChannel_FetchSelfProfile(t *testing.T) {
	ch := New(Config{BearerToken: "bearer-token"}, zap.NewNop())
	ch.client = &http.Client{Transport: twitterRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != usersMeURL {
			t.Fatalf("url = %s, want %s", req.URL.String(), usersMeURL)
		}
		if auth := req.Header.Get("Authorization"); auth != "Bearer bearer-token" {
			t.Fatalf("Authorization = %q", auth)
		}
		body := `{"data":{"id":"bot-1","username":"bluebot"}}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}

	if err := ch.fetchSelfProfile(context.Background()); err != nil {
		t.Fatalf("fetchSelfProfile error = %v", err)
	}
	if ch.selfUserID != "bot-1" {
		t.Fatalf("selfUserID = %q", ch.selfUserID)
	}
	if ch.selfUsername != "bluebot" {
		t.Fatalf("selfUsername = %q", ch.selfUsername)
	}
}

func TestChannel_Send_UsesConversationEndpointWhenConversationIDPresent(t *testing.T) {
	var requestURL string
	ch := New(Config{}, zap.NewNop())
	ch.client = &http.Client{Transport: twitterRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestURL = req.URL.String()
		return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "conv-1",
		Content: "hello",
		Metadata: map[string]interface{}{
			"dm_conversation_id": "123456-789012",
		},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	want := twitterAPIBase + "/dm_conversations/123456-789012/messages"
	if requestURL != want {
		t.Fatalf("requestURL = %q, want %q", requestURL, want)
	}
}
