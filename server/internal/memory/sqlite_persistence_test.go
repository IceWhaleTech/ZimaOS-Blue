package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestStoreAddMessageTransactionRollbackPreservesConversationTimestamp(t *testing.T) {
	ctx := context.Background()
	store, err := NewStore(filepath.Join(t.TempDir(), "chat.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(ctx, "tx")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	messageID := uuid.NewString()
	if _, err := store.AddMessageTrusted(ctx, conv.ID, Message{
		ID:      messageID,
		Role:    "assistant",
		Content: "first",
	}); err != nil {
		t.Fatalf("AddMessageTrusted(first): %v", err)
	}

	afterFirst, err := store.GetConversation(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetConversation(after first): %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	if _, err := store.AddMessageTrusted(ctx, conv.ID, Message{
		ID:      messageID,
		Role:    "assistant",
		Content: "duplicate",
	}); err == nil {
		t.Fatalf("AddMessageTrusted(duplicate) error = nil, want duplicate failure")
	}

	afterFailure, err := store.GetConversation(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetConversation(after failure): %v", err)
	}
	if !afterFailure.UpdatedAt.Equal(afterFirst.UpdatedAt) {
		t.Fatalf("conversation updated_at changed on failed insert: got %v want %v", afterFailure.UpdatedAt, afterFirst.UpdatedAt)
	}
}

func TestStoreExternalAttachmentsRoundTripAndLegacyFallback(t *testing.T) {
	ctx := context.Background()
	dbDir := t.TempDir()
	opts := DefaultChatStoreOptions(filepath.Join(dbDir, "chat.db"))
	opts.AttachmentExternalStore = true
	store, err := NewStoreWithOptions(filepath.Join(dbDir, "chat.db"), opts)
	if err != nil {
		t.Fatalf("NewStoreWithOptions: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(ctx, "attachments")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	msg, err := store.AddMessageTrusted(ctx, conv.ID, Message{
		Role:    "user",
		Content: "with attachment",
		Attachments: []MessageAttachment{
			{Type: "file", Name: "note.txt", MimeType: "text/plain", Data: "Zm9v"},
		},
	})
	if err != nil {
		t.Fatalf("AddMessageTrusted(external attachments): %v", err)
	}

	var hasAttachments bool
	var legacy sql.NullString
	if err := store.db.QueryRowContext(ctx,
		`SELECT has_attachments, attachments FROM messages WHERE id = ?`,
		msg.ID,
	).Scan(&hasAttachments, &legacy); err != nil {
		t.Fatalf("scan attachment columns: %v", err)
	}
	if !hasAttachments {
		t.Fatalf("has_attachments = false, want true")
	}
	if legacy.Valid && legacy.String != "" {
		t.Fatalf("legacy attachments column = %q, want empty when externalized", legacy.String)
	}
	attachmentPath := filepath.Join(dbDir, "media", "message_attachments", msg.ID, "0000.json")
	if _, err := os.Stat(attachmentPath); err != nil {
		t.Fatalf("expected external attachment at %q: %v", attachmentPath, err)
	}

	got, err := store.GetMessages(ctx, conv.ID, 10, 0)
	if err != nil {
		t.Fatalf("GetMessages(external attachments): %v", err)
	}
	if len(got) != 1 || len(got[0].Attachments) != 1 || got[0].Attachments[0].Name != "note.txt" {
		t.Fatalf("external attachments round-trip mismatch: %+v", got)
	}

	legacyAttachments, _ := json.Marshal([]MessageAttachment{
		{Type: "image", Name: "legacy.png", MimeType: "image/png", Data: "YmFy"},
	})
	legacyID := uuid.NewString()
	if _, err := store.db.ExecContext(ctx,
		`INSERT INTO messages (id, conversation_id, role, content, attachments, has_attachments, created_at)
		 VALUES (?, ?, 'assistant', 'legacy', ?, 1, ?)`,
		legacyID,
		conv.ID,
		string(legacyAttachments),
		time.Now().UTC(),
	); err != nil {
		t.Fatalf("insert legacy attachment row: %v", err)
	}

	got, err = store.GetMessages(ctx, conv.ID, 10, 0)
	if err != nil {
		t.Fatalf("GetMessages(legacy fallback): %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("GetMessages length = %d, want 2", len(got))
	}
	var legacyFound bool
	for _, item := range got {
		if item.ID != legacyID {
			continue
		}
		legacyFound = true
		if len(item.Attachments) != 1 || item.Attachments[0].Name != "legacy.png" {
			t.Fatalf("legacy attachment fallback mismatch: %+v", item.Attachments)
		}
	}
	if !legacyFound {
		t.Fatalf("legacy message %q not returned", legacyID)
	}
}

func TestStoreGetRecentMessagesLiteOmitsHeavyFields(t *testing.T) {
	ctx := context.Background()
	store, err := NewStore(filepath.Join(t.TempDir(), "chat.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(ctx, "lite")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	if _, err := store.AddMessageTrusted(ctx, conv.ID, Message{
		Role:     "assistant",
		Content:  "payload",
		Provider: "openai",
		Model:    "gpt-test",
		Stats: &MessageStats{
			InputTokens: 1,
		},
		Attachments: []MessageAttachment{
			{Type: "file", Name: "doc.txt", MimeType: "text/plain", Data: "Zm9v"},
		},
	}); err != nil {
		t.Fatalf("AddMessageTrusted: %v", err)
	}

	full, err := store.GetRecentMessages(ctx, conv.ID, 1)
	if err != nil {
		t.Fatalf("GetRecentMessages: %v", err)
	}
	lite, err := store.GetRecentMessagesLite(ctx, conv.ID, 1)
	if err != nil {
		t.Fatalf("GetRecentMessagesLite: %v", err)
	}
	if len(full) != 1 || len(lite) != 1 {
		t.Fatalf("unexpected message counts full=%d lite=%d", len(full), len(lite))
	}
	if lite[0].Content != full[0].Content || lite[0].Provider != "openai" || lite[0].Model != "gpt-test" {
		t.Fatalf("lite message lost core fields: full=%+v lite=%+v", full[0], lite[0])
	}
	if lite[0].Stats != nil {
		t.Fatalf("lite stats = %+v, want nil", lite[0].Stats)
	}
	if len(lite[0].Attachments) != 0 {
		t.Fatalf("lite attachments = %+v, want empty", lite[0].Attachments)
	}
}

func TestStoreUpdateMessageContentFullPreservesExistingProviderModelAndStats(t *testing.T) {
	ctx := context.Background()
	store, err := NewStore(filepath.Join(t.TempDir(), "chat.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(ctx, "update-content")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	msg, err := store.AddMessageTrusted(ctx, conv.ID, Message{
		Role:     "assistant",
		Content:  "draft",
		Provider: "openai",
		Model:    "gpt-5",
		Stats: &MessageStats{
			InputTokens: 3,
		},
	})
	if err != nil {
		t.Fatalf("AddMessageTrusted: %v", err)
	}

	if err := store.UpdateMessageContentFull(ctx, msg.ID, "final", "", "", nil); err != nil {
		t.Fatalf("UpdateMessageContentFull: %v", err)
	}

	got, err := store.GetMessages(ctx, conv.ID, 10, 0)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("GetMessages len = %d, want 1", len(got))
	}
	if got[0].Content != "final" {
		t.Fatalf("content = %q, want %q", got[0].Content, "final")
	}
	if got[0].Provider != "openai" || got[0].Model != "gpt-5" {
		t.Fatalf("provider/model changed unexpectedly: %+v", got[0])
	}
	if got[0].Stats == nil || got[0].Stats.InputTokens != 3 {
		t.Fatalf("stats changed unexpectedly: %+v", got[0].Stats)
	}
}

func TestStoreUpsertMessageContentFullTrustedInsertsAndPreservesExistingFields(t *testing.T) {
	ctx := context.Background()
	store, err := NewStore(filepath.Join(t.TempDir(), "chat.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(ctx, "upsert-content")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	msgID := uuid.NewString()
	if err := store.UpsertMessageContentFullTrusted(ctx, Message{
		ID:             msgID,
		ConversationID: conv.ID,
		Role:           "assistant",
		Content:        "draft",
		Provider:       "openai",
		Model:          "gpt-5",
		Stats: &MessageStats{
			InputTokens: 7,
		},
	}); err != nil {
		t.Fatalf("UpsertMessageContentFullTrusted(insert): %v", err)
	}

	if err := store.UpsertMessageContentFullTrusted(ctx, Message{
		ID:             msgID,
		ConversationID: conv.ID,
		Role:           "assistant",
		Content:        "final",
	}); err != nil {
		t.Fatalf("UpsertMessageContentFullTrusted(update): %v", err)
	}

	got, err := store.GetMessages(ctx, conv.ID, 10, 0)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("GetMessages len = %d, want 1", len(got))
	}
	if got[0].ID != msgID {
		t.Fatalf("message id = %q, want %q", got[0].ID, msgID)
	}
	if got[0].Content != "final" {
		t.Fatalf("content = %q, want %q", got[0].Content, "final")
	}
	if got[0].Provider != "openai" || got[0].Model != "gpt-5" {
		t.Fatalf("provider/model changed unexpectedly: %+v", got[0])
	}
	if got[0].Stats == nil || got[0].Stats.InputTokens != 7 {
		t.Fatalf("stats changed unexpectedly: %+v", got[0].Stats)
	}
}
