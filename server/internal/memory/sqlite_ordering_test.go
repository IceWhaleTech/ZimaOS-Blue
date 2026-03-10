package memory

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestStore_GetMessages_PaginatesFromStart(t *testing.T) {
	ctx := context.Background()
	store, err := NewStore(filepath.Join(t.TempDir(), "chat.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(ctx, "pagination-order")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	for i := 1; i <= 5; i++ {
		if _, err := store.AddMessage(ctx, conv.ID, Message{
			Role:    "user",
			Content: fmt.Sprintf("m%d", i),
		}); err != nil {
			t.Fatalf("AddMessage(%d): %v", i, err)
		}
	}

	page0, err := store.GetMessages(ctx, conv.ID, 2, 0)
	if err != nil {
		t.Fatalf("GetMessages page0: %v", err)
	}
	if len(page0) != 2 || page0[0].Content != "m1" || page0[1].Content != "m2" {
		t.Fatalf("page0 = %#v, want [m1 m2]", contentsOf(page0))
	}

	page1, err := store.GetMessages(ctx, conv.ID, 2, 2)
	if err != nil {
		t.Fatalf("GetMessages page1: %v", err)
	}
	if len(page1) != 2 || page1[0].Content != "m3" || page1[1].Content != "m4" {
		t.Fatalf("page1 = %#v, want [m3 m4]", contentsOf(page1))
	}

	page2, err := store.GetMessages(ctx, conv.ID, 2, 4)
	if err != nil {
		t.Fatalf("GetMessages page2: %v", err)
	}
	if len(page2) != 1 || page2[0].Content != "m5" {
		t.Fatalf("page2 = %#v, want [m5]", contentsOf(page2))
	}
}

func TestStore_GetMessages_StableWhenCreatedAtEqual(t *testing.T) {
	ctx := context.Background()
	store, err := NewStore(filepath.Join(t.TempDir(), "chat.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(ctx, "same-created-at")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	for i := 1; i <= 4; i++ {
		if _, err := store.AddMessage(ctx, conv.ID, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("m%d", i),
		}); err != nil {
			t.Fatalf("AddMessage(%d): %v", i, err)
		}
	}

	sameTime := time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)
	if _, err := store.db.ExecContext(ctx, "UPDATE messages SET created_at = ? WHERE conversation_id = ?", sameTime, conv.ID); err != nil {
		t.Fatalf("force same created_at: %v", err)
	}

	msgs, err := store.GetMessages(ctx, conv.ID, 10, 0)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(msgs) != 4 {
		t.Fatalf("GetMessages len = %d, want 4", len(msgs))
	}
	if msgs[0].Content != "m1" || msgs[1].Content != "m2" || msgs[2].Content != "m3" || msgs[3].Content != "m4" {
		t.Fatalf("messages = %#v, want [m1 m2 m3 m4]", contentsOf(msgs))
	}
}

func TestStore_ListConversations_StableWhenUpdatedAtEqual(t *testing.T) {
	ctx := context.Background()
	store, err := NewStore(filepath.Join(t.TempDir(), "chat.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	conv1, err := store.CreateConversation(ctx, "c1")
	if err != nil {
		t.Fatalf("CreateConversation c1: %v", err)
	}
	conv2, err := store.CreateConversation(ctx, "c2")
	if err != nil {
		t.Fatalf("CreateConversation c2: %v", err)
	}
	conv3, err := store.CreateConversation(ctx, "c3")
	if err != nil {
		t.Fatalf("CreateConversation c3: %v", err)
	}

	sameTime := time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)
	if _, err := store.db.ExecContext(ctx, "UPDATE conversations SET updated_at = ?", sameTime); err != nil {
		t.Fatalf("force same updated_at: %v", err)
	}

	convs, err := store.ListConversations(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(convs) != 3 {
		t.Fatalf("ListConversations len = %d, want 3", len(convs))
	}
	if convs[0].ID != conv3.ID || convs[1].ID != conv2.ID || convs[2].ID != conv1.ID {
		t.Fatalf("conversation IDs = [%s %s %s], want [%s %s %s]",
			convs[0].ID, convs[1].ID, convs[2].ID,
			conv3.ID, conv2.ID, conv1.ID,
		)
	}
}

func contentsOf(messages []Message) []string {
	out := make([]string, 0, len(messages))
	for _, msg := range messages {
		out = append(out, msg.Content)
	}
	return out
}
