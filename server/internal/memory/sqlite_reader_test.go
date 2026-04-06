package memory

import (
	"context"
	"path/filepath"
	"testing"
)

func TestStore_UsesReaderDBForReads(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "chat.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	if store.readDB == nil {
		t.Fatal("expected read db to be initialized")
	}
	if store.readDB == store.db {
		t.Fatal("expected file-backed memory store to use a separate read db")
	}

	ctx := context.Background()
	conv, err := store.CreateConversation(ctx, "reader", "user-a")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if _, err := store.AddMessage(ctx, conv.ID, Message{
		Role:    "user",
		Content: "hello reader",
	}, "user-a"); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	if err := store.db.Close(); err != nil {
		t.Fatalf("close writer db: %v", err)
	}

	gotConv, err := store.GetConversation(ctx, conv.ID, "user-a")
	if err != nil {
		t.Fatalf("GetConversation via reader: %v", err)
	}
	if gotConv == nil || gotConv.ID != conv.ID {
		t.Fatalf("unexpected conversation via reader: %+v", gotConv)
	}

	gotMsgs, err := store.GetRecentMessages(ctx, conv.ID, 1, "user-a")
	if err != nil {
		t.Fatalf("GetRecentMessages via reader: %v", err)
	}
	if len(gotMsgs) != 1 || gotMsgs[0].Content != "hello reader" {
		t.Fatalf("unexpected messages via reader: %+v", gotMsgs)
	}

	count, err := store.CountMessages(ctx, conv.ID, "user-a")
	if err != nil {
		t.Fatalf("CountMessages via reader: %v", err)
	}
	if count != 1 {
		t.Fatalf("CountMessages = %d, want 1", count)
	}
}

func TestStore_UsesReaderDBForCommandAndResponseStateReads(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	if store.readDB == nil {
		t.Fatal("expected read db to be initialized")
	}
	if store.readDB == store.db {
		t.Fatal("expected file-backed memory store to use a separate read db")
	}

	ctx := context.Background()
	conv, err := store.CreateConversation(ctx, "reader-state", "user-a")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	if err := store.UpsertConversationCommandState(ctx, ConversationCommandState{
		ConversationID:      conv.ID,
		SelectedProviderID:  "openai",
		SelectedModelID:     "gpt-5",
		Offline:             true,
		WebSearchEnabled:    false,
		DeepResearchEnabled: true,
	}); err != nil {
		t.Fatalf("UpsertConversationCommandState: %v", err)
	}
	if err := store.SetConversationPreviousResponseID(ctx, conv.ID, "resp_reader"); err != nil {
		t.Fatalf("SetConversationPreviousResponseID: %v", err)
	}

	if err := store.db.Close(); err != nil {
		t.Fatalf("close writer db: %v", err)
	}

	state, err := store.GetConversationCommandState(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetConversationCommandState via reader: %v", err)
	}
	if state.SelectedProviderID != "openai" || state.SelectedModelID != "gpt-5" || !state.Offline || !state.WebSearchEnabled || !state.DeepResearchEnabled {
		t.Fatalf("unexpected command state via reader: %+v", state)
	}

	prevID, err := store.GetConversationPreviousResponseID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetConversationPreviousResponseID via reader: %v", err)
	}
	if prevID != "resp_reader" {
		t.Fatalf("GetConversationPreviousResponseID = %q, want %q", prevID, "resp_reader")
	}
}
