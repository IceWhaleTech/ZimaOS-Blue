package qq

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

type qqRoundTripFunc func(*http.Request) (*http.Response, error)

func (f qqRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestChannel_Send_FileWithoutURLFallsBackToName(t *testing.T) {
	var payloads []map[string]interface{}
	ch := New(Config{}, zap.NewNop())
	ch.accessToken = "token"
	ch.tokenExpiry = time.Now().Add(time.Hour)
	ch.client = &http.Client{Transport: qqRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		payloads = append(payloads, payload)
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "123",
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
		t.Fatalf("expected 1 payload, got %d", len(payloads))
	}
	if payloads[0]["content"] != "caption\nagenda.pdf" {
		t.Fatalf("content = %v, want caption+name", payloads[0]["content"])
	}
}

func TestChannel_Send_UnsuitableAttachmentKeepsTrailingText(t *testing.T) {
	var payloads []map[string]interface{}
	ch := New(Config{}, zap.NewNop())
	ch.accessToken = "token"
	ch.tokenExpiry = time.Now().Add(time.Hour)
	ch.client = &http.Client{Transport: qqRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		payloads = append(payloads, payload)
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})}

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:  "123",
		Content: "caption",
		Attachments: []channel.Attachment{{
			Type: channel.MessageTypeFile,
		}},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if len(payloads) != 1 {
		t.Fatalf("expected 1 payload, got %d", len(payloads))
	}
	if payloads[0]["content"] != "caption" {
		t.Fatalf("content = %v, want trailing caption", payloads[0]["content"])
	}
}

func TestChannel_ProcessMessage_PreservesReplyReference(t *testing.T) {
	ch := New(Config{}, zap.NewNop())

	ch.processMessage(messageEvent{
		ID:        "msg-2",
		ChannelID: "channel-1",
		GuildID:   "guild-1",
		Content:   "hello",
		Author:    messageAuthor{ID: "user-1", Username: "alice"},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		MessageReference: &messageReference{
			MessageID: "msg-1",
		},
	})

	select {
	case got := <-ch.Messages():
		if got.ReplyToID != "msg-1" {
			t.Fatalf("ReplyToID = %q, want msg-1", got.ReplyToID)
		}
		if got.Metadata["reference_message_id"] != "msg-1" {
			t.Fatalf("reference_message_id = %v, want msg-1", got.Metadata["reference_message_id"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for QQ message")
	}
}

func TestChannel_ProcessMessage_MapsAttachmentsAndEmbeds(t *testing.T) {
	ch := New(Config{}, zap.NewNop())

	ch.processMessage(messageEvent{
		ID:              "msg-3",
		ChannelID:       "channel-1",
		GuildID:         "guild-1",
		Author:          messageAuthor{ID: "user-1", Username: "alice"},
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
		MentionEveryone: true,
		Attachments: []messageAttachment{{
			URL:         "https://example.com/image.png",
			Filename:    "image.png",
			ContentType: "image/png",
			Size:        2048,
			Width:       800,
			Height:      600,
		}},
		Embeds: []map[string]interface{}{{
			"title":       "embed title",
			"description": "embed body",
		}},
	})

	select {
	case got := <-ch.Messages():
		if got.Type != channel.MessageTypeImage {
			t.Fatalf("Type = %q, want image", got.Type)
		}
		if len(got.Attachments) != 1 || got.Attachments[0].URL != "https://example.com/image.png" {
			t.Fatalf("Attachments = %#v", got.Attachments)
		}
		if got.Metadata["mention_everyone"] != true {
			t.Fatalf("mention_everyone = %#v", got.Metadata["mention_everyone"])
		}
		if got.Metadata["attachment_count"] != 1 {
			t.Fatalf("attachment_count = %#v", got.Metadata["attachment_count"])
		}
		rawAttachments, ok := got.Metadata["attachments"].([]map[string]interface{})
		if !ok || len(rawAttachments) != 1 {
			t.Fatalf("attachments metadata = %#v", got.Metadata["attachments"])
		}
		if rawAttachments[0]["filename"] != "image.png" {
			t.Fatalf("filename = %#v", rawAttachments[0]["filename"])
		}
		rawEmbeds, ok := got.Metadata["embeds"].([]map[string]interface{})
		if !ok || len(rawEmbeds) != 1 || rawEmbeds[0]["title"] != "embed title" {
			t.Fatalf("embeds = %#v", got.Metadata["embeds"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for QQ message")
	}
}

func TestChannel_ProcessMessage_EmbedOnlyBecomesCard(t *testing.T) {
	ch := New(Config{}, zap.NewNop())

	ch.processMessage(messageEvent{
		ID:        "msg-4",
		ChannelID: "channel-1",
		Author:    messageAuthor{ID: "user-1", Username: "alice"},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Embeds: []map[string]interface{}{{
			"title": "card title",
		}},
	})

	select {
	case got := <-ch.Messages():
		if got.Type != channel.MessageTypeCard {
			t.Fatalf("Type = %q, want card", got.Type)
		}
		if got.Metadata["embed_count"] != 1 {
			t.Fatalf("embed_count = %#v", got.Metadata["embed_count"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for QQ message")
	}
}
