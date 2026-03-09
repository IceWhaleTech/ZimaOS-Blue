package twitch

import (
	"bufio"
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

func TestChannel_Send_AttachmentFallbackToIRCText(t *testing.T) {
	var buf bytes.Buffer
	ch := New(Config{}, zap.NewNop())
	ch.writer = bufio.NewWriter(&buf)
	ch.status = channel.StatusConnected

	err := ch.Send(context.Background(), channel.OutgoingMessage{
		ChatID:      "room",
		Content:     "caption",
		Attachments: []channel.Attachment{{Type: channel.MessageTypeFile, URL: "https://example.com/file.pdf"}},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "PRIVMSG #room :caption\nhttps://example.com/file.pdf") {
		t.Fatalf("unexpected IRC output %q", got)
	}
}

func TestChannel_Send_NoSendableContentErrors(t *testing.T) {
	var buf bytes.Buffer
	ch := New(Config{}, zap.NewNop())
	ch.writer = bufio.NewWriter(&buf)
	ch.status = channel.StatusConnected

	err := ch.Send(context.Background(), channel.OutgoingMessage{ChatID: "room", Attachments: []channel.Attachment{{Type: channel.MessageTypeFile}}})
	if err == nil {
		t.Fatal("expected no sendable content error")
	}
}

func TestChannel_ParseLine_PreservesReplyTimestampAndUserID(t *testing.T) {
	ch := New(Config{}, zap.NewNop())
	ch.parseLine(`@badges=subscriber/12;color=#1E90FF;display-name=Alice;emotes=25:0-4;first-msg=1;id=msg-1;reply-parent-display-name=Bob;reply-parent-msg-body=hello\sworld;reply-parent-msg-id=parent-1;reply-parent-user-id=42;reply-parent-user-login=bob;room-id=777;tmi-sent-ts=1710000000123;user-id=123 :alice!alice@alice.tmi.twitch.tv PRIVMSG #room :Kappa hi`)

	select {
	case msg := <-ch.Messages():
		if msg.ID != "msg-1" {
			t.Fatalf("ID = %q, want msg-1", msg.ID)
		}
		if msg.ReplyToID != "parent-1" {
			t.Fatalf("ReplyToID = %q, want parent-1", msg.ReplyToID)
		}
		if msg.UserID != "123" {
			t.Fatalf("UserID = %q, want 123", msg.UserID)
		}
		if msg.Timestamp.UTC().Format(time.RFC3339Nano) != "2024-03-09T16:00:00.123Z" {
			t.Fatalf("Timestamp = %s", msg.Timestamp.UTC().Format(time.RFC3339Nano))
		}
		if msg.Metadata["reply_parent_user_id"] != "42" {
			t.Fatalf("reply_parent_user_id = %v", msg.Metadata["reply_parent_user_id"])
		}
		if msg.Metadata["room_id"] != "777" {
			t.Fatalf("room_id = %v", msg.Metadata["room_id"])
		}
		if msg.Metadata["first_msg"] != true {
			t.Fatalf("first_msg = %v", msg.Metadata["first_msg"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for twitch message")
	}
}

func TestChannel_ParseLine_ParsesEmoteRanges(t *testing.T) {
	ch := New(Config{}, zap.NewNop())
	ch.parseLine("@emotes=25:0-4,6-10/1902:12-16;id=msg-2;user-id=123 :alice!alice@alice.tmi.twitch.tv PRIVMSG #room :Kappa Kappa Keepo")

	select {
	case msg := <-ch.Messages():
		raw, ok := msg.Metadata["emote_ranges"].([]map[string]interface{})
		if !ok || len(raw) != 2 {
			t.Fatalf("emote_ranges = %#v", msg.Metadata["emote_ranges"])
		}
		if raw[0]["id"] != "25" {
			t.Fatalf("first emote = %#v", raw[0])
		}
		ranges, ok := raw[0]["ranges"].([]map[string]int)
		if !ok || len(ranges) != 2 {
			t.Fatalf("ranges = %#v", raw[0]["ranges"])
		}
		if ranges[0]["start"] != 0 || ranges[0]["end"] != 4 {
			t.Fatalf("first range = %#v", ranges[0])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for twitch message")
	}
}
