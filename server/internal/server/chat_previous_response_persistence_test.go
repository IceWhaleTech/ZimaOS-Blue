package server

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestChatHandler_PreviousResponseID_Persistence(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "chat.db")

	store, err := memory.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(ctx, "test")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	handler1 := NewChatHandler(store, nil, tools.NewRegistry())
	handler1.setPreviousResponseID(conv.ID, "resp_persisted_1")

	persisted, err := store.GetConversationPreviousResponseID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetConversationPreviousResponseID: %v", err)
	}
	if persisted != "resp_persisted_1" {
		t.Fatalf("persisted previous_response_id = %q, want %q", persisted, "resp_persisted_1")
	}

	// Simulate process restart by creating a fresh handler with empty in-memory cache.
	handler2 := NewChatHandler(store, nil, tools.NewRegistry())
	if got := handler2.getPreviousResponseID(conv.ID); got != "resp_persisted_1" {
		t.Fatalf("getPreviousResponseID after restart = %q, want %q", got, "resp_persisted_1")
	}

	handler2.clearPreviousResponseID(conv.ID)
	afterClear, err := store.GetConversationPreviousResponseID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetConversationPreviousResponseID after clear: %v", err)
	}
	if afterClear != "" {
		t.Fatalf("GetConversationPreviousResponseID after clear = %q, want empty", afterClear)
	}
}
