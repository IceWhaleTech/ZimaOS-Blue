package memory

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

func TestStore_ConversationPreviousResponseID_SetGetClear(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "chat.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(ctx, "test")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	prevID, err := store.GetConversationPreviousResponseID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetConversationPreviousResponseID (empty): %v", err)
	}
	if prevID != "" {
		t.Fatalf("GetConversationPreviousResponseID (empty) = %q, want empty", prevID)
	}

	if err := store.SetConversationPreviousResponseID(ctx, conv.ID, "resp_1"); err != nil {
		t.Fatalf("SetConversationPreviousResponseID: %v", err)
	}

	prevID, err = store.GetConversationPreviousResponseID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetConversationPreviousResponseID: %v", err)
	}
	if prevID != "resp_1" {
		t.Fatalf("GetConversationPreviousResponseID = %q, want %q", prevID, "resp_1")
	}

	if err := store.ClearConversationPreviousResponseID(ctx, conv.ID); err != nil {
		t.Fatalf("ClearConversationPreviousResponseID: %v", err)
	}

	prevID, err = store.GetConversationPreviousResponseID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetConversationPreviousResponseID (after clear): %v", err)
	}
	if prevID != "" {
		t.Fatalf("GetConversationPreviousResponseID (after clear) = %q, want empty", prevID)
	}
}

func TestStore_ConversationPreviousResponseID_PersistsAcrossRestart(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "chat.db")

	store1, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore (first): %v", err)
	}
	conv, err := store1.CreateConversation(ctx, "test")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if err := store1.SetConversationPreviousResponseID(ctx, conv.ID, "resp_persist_1"); err != nil {
		t.Fatalf("SetConversationPreviousResponseID: %v", err)
	}
	if err := store1.Close(); err != nil {
		t.Fatalf("Close (first): %v", err)
	}

	store2, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore (second): %v", err)
	}
	defer store2.Close()

	prevID, err := store2.GetConversationPreviousResponseID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetConversationPreviousResponseID: %v", err)
	}
	if prevID != "resp_persist_1" {
		t.Fatalf("GetConversationPreviousResponseID = %q, want %q", prevID, "resp_persist_1")
	}
}

func TestStore_ConversationPreviousResponseID_TTL(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "chat.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(ctx, "test")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if err := store.SetConversationPreviousResponseID(ctx, conv.ID, "resp_old_1"); err != nil {
		t.Fatalf("SetConversationPreviousResponseID: %v", err)
	}

	oldTs := timeutil.NowTime().Add(-responsesPreviousIDTTL - time.Hour)
	if _, err := store.db.ExecContext(
		ctx,
		`UPDATE conversation_runtime_state SET updated_at = ? WHERE conversation_id = ?`,
		oldTs,
		conv.ID,
	); err != nil {
		t.Fatalf("force old timestamp: %v", err)
	}

	prevID, err := store.GetConversationPreviousResponseID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetConversationPreviousResponseID: %v", err)
	}
	if prevID != "" {
		t.Fatalf("GetConversationPreviousResponseID (expired) = %q, want empty", prevID)
	}
}
