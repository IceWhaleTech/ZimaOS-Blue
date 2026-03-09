package nextcloudtalk

import (
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

func TestChannel_processMessage_PreservesVisibleMetadata(t *testing.T) {
	ch := New(Config{}, zap.NewNop())

	ch.processMessage(chatMessage{
		ID:               17,
		Token:            "room-token",
		ActorType:        "users",
		ActorID:          "alice",
		ActorDisplayName: "Alice",
		Timestamp:        1700000000,
		Message:          "",
		MessageType:      "comment",
		IsReplyable:      true,
		ReferenceID:      "ref-17",
		MessageParameters: map[string]interface{}{
			"file": map[string]interface{}{
				"type": "file",
				"name": "report.pdf",
			},
		},
		Parent: map[string]interface{}{
			"id":      float64(42),
			"message": "root message",
		},
		Reactions: map[string]int{
			"👍": 2,
		},
		ReactionsSelf:            []string{"👍"},
		Markdown:                 true,
		Silent:                   true,
		ExpirationTimestamp:      1700003600,
		LastEditTimestamp:        1700001800,
		LastEditActorType:        "users",
		LastEditActorID:          "alice",
		LastEditActorDisplayName: "Alice",
	})

	select {
	case msg := <-ch.Messages():
		if msg.ID != "17" {
			t.Fatalf("ID = %q, want 17", msg.ID)
		}
		if msg.Type != channel.MessageTypeCard {
			t.Fatalf("Type = %q, want card", msg.Type)
		}
		if msg.ReplyToID != "42" {
			t.Fatalf("ReplyToID = %q, want 42", msg.ReplyToID)
		}
		if !msg.Timestamp.Equal(time.Unix(1700000000, 0)) {
			t.Fatalf("Timestamp = %v", msg.Timestamp)
		}
		if got := msg.Metadata["message_type"]; got != "comment" {
			t.Fatalf("message_type = %#v", got)
		}
		if got := msg.Metadata["reference_id"]; got != "ref-17" {
			t.Fatalf("reference_id = %#v", got)
		}
		if got := msg.Metadata["parent_id"]; got != "42" {
			t.Fatalf("parent_id = %#v", got)
		}
		if got := msg.Metadata["markdown"]; got != true {
			t.Fatalf("markdown = %#v", got)
		}
		if got := msg.Metadata["silent"]; got != true {
			t.Fatalf("silent = %#v", got)
		}
		if got := msg.Metadata["expiration_timestamp"]; got != int64(1700003600) {
			t.Fatalf("expiration_timestamp = %#v", got)
		}
		if got := msg.Metadata["last_edit_timestamp"]; got != int64(1700001800) {
			t.Fatalf("last_edit_timestamp = %#v", got)
		}
		reactions, ok := msg.Metadata["reactions"].(map[string]int)
		if !ok {
			t.Fatalf("reactions metadata type = %T", msg.Metadata["reactions"])
		}
		if reactions["👍"] != 2 {
			t.Fatalf("reactions = %#v", reactions)
		}
		reactionsSelf, ok := msg.Metadata["reactions_self"].([]string)
		if !ok {
			t.Fatalf("reactions_self metadata type = %T", msg.Metadata["reactions_self"])
		}
		if len(reactionsSelf) != 1 || reactionsSelf[0] != "👍" {
			t.Fatalf("reactions_self = %#v", reactionsSelf)
		}
		params, ok := msg.Metadata["message_parameters"].(map[string]interface{})
		if !ok {
			t.Fatalf("message_parameters type = %T", msg.Metadata["message_parameters"])
		}
		if _, ok := params["file"]; !ok {
			t.Fatalf("message_parameters = %#v", params)
		}
		parent, ok := msg.Metadata["parent"].(map[string]interface{})
		if !ok {
			t.Fatalf("parent type = %T", msg.Metadata["parent"])
		}
		if got := parent["message"]; got != "root message" {
			t.Fatalf("parent.message = %#v", got)
		}
	default:
		t.Fatal("expected message to be emitted")
	}
}

func TestChannel_processMessage_TextMessageKeepsTextType(t *testing.T) {
	ch := New(Config{}, zap.NewNop())

	ch.processMessage(chatMessage{
		ID:               18,
		Token:            "room-token",
		ActorType:        "users",
		ActorID:          "bob",
		ActorDisplayName: "Bob",
		Timestamp:        1700000100,
		Message:          "hello *world*",
		Markdown:         true,
		MessageParameters: map[string]interface{}{
			"mention": map[string]interface{}{"type": "user", "id": "bob"},
		},
	})

	select {
	case msg := <-ch.Messages():
		if msg.Type != channel.MessageTypeText {
			t.Fatalf("Type = %q, want text", msg.Type)
		}
		if msg.Content != "hello *world*" {
			t.Fatalf("Content = %q", msg.Content)
		}
	default:
		t.Fatal("expected message to be emitted")
	}
}
