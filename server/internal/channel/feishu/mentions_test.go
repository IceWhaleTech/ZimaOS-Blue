package feishu

import "testing"

func TestFeishuMentionsMetadata_UsesInboundMentions(t *testing.T) {
	mentions, mentionIDs := feishuMentionsMetadata("hello @_bot_1", []incomingMention{{
		Key:  "@_bot_1",
		Name: "Blue",
		ID: struct {
			OpenID string `json:"open_id"`
			UserID string `json:"user_id"`
		}{
			OpenID: "ou_bot",
		},
	}})

	if len(mentions) != 1 {
		t.Fatalf("mentions len = %d, want 1", len(mentions))
	}
	if mentions[0]["id"] != "ou_bot" {
		t.Fatalf("mention id = %#v, want %q", mentions[0]["id"], "ou_bot")
	}
	if mentions[0]["name"] != "Blue" {
		t.Fatalf("mention name = %#v, want %q", mentions[0]["name"], "Blue")
	}
	if len(mentionIDs) != 1 || mentionIDs[0] != "ou_bot" {
		t.Fatalf("mentionIDs = %#v", mentionIDs)
	}
}

func TestFeishuMentionsMetadata_FallsBackToAtTags(t *testing.T) {
	mentions, mentionIDs := feishuMentionsMetadata(`<at user_id="ou_bot">Blue</at> hello`, nil)

	if len(mentions) != 1 {
		t.Fatalf("mentions len = %d, want 1", len(mentions))
	}
	if mentions[0]["id"] != "ou_bot" {
		t.Fatalf("mention id = %#v, want %q", mentions[0]["id"], "ou_bot")
	}
	if mentions[0]["name"] != "Blue" {
		t.Fatalf("mention name = %#v, want %q", mentions[0]["name"], "Blue")
	}
	if len(mentionIDs) != 1 || mentionIDs[0] != "ou_bot" {
		t.Fatalf("mentionIDs = %#v", mentionIDs)
	}
}
