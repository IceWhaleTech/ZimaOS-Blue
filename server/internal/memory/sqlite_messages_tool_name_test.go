package memory

import (
	"context"
	"path/filepath"
	"testing"
)

func TestStore_MessageToolNameRoundTrip(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "chat.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(ctx, "tool-name")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	in := Message{
		Role:       "tool",
		Content:    `{"ok":true}`,
		ToolCallID: "call_1",
		ToolName:   "web_search",
	}
	added, err := store.AddMessage(ctx, conv.ID, in)
	if err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	msgs, err := store.GetMessages(ctx, conv.ID, 10, 0)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("GetMessages length = %d, want 1", len(msgs))
	}

	got := msgs[0]
	if got.ID != added.ID {
		t.Fatalf("message ID = %q, want %q", got.ID, added.ID)
	}
	if got.ToolCallID != in.ToolCallID {
		t.Fatalf("ToolCallID = %q, want %q", got.ToolCallID, in.ToolCallID)
	}
	if got.ToolName != in.ToolName {
		t.Fatalf("ToolName = %q, want %q", got.ToolName, in.ToolName)
	}
}
