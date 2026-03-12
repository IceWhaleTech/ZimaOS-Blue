package memory

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestStoreConversationScopeEnforcesUserIsolation(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "multi-user.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	ownerConv, err := store.CreateConversation(ctx, "Owner", "user-a")
	if err != nil {
		t.Fatalf("CreateConversation(owner): %v", err)
	}
	otherConv, err := store.CreateConversation(ctx, "Other", "user-b")
	if err != nil {
		t.Fatalf("CreateConversation(other): %v", err)
	}

	if _, err := store.GetConversation(ctx, ownerConv.ID, "user-b"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetConversation(wrong user) err = %v, want ErrNotFound", err)
	}
	if err := store.PinConversation(ctx, ownerConv.ID, "user-b"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("PinConversation(wrong user) err = %v, want ErrNotFound", err)
	}
	if _, err := store.AddMessage(ctx, ownerConv.ID, Message{Role: "user", Content: "hello"}, "user-b"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("AddMessage(wrong user) err = %v, want ErrNotFound", err)
	}
	if _, err := store.GetMessages(ctx, ownerConv.ID, 10, 0, "user-b"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetMessages(wrong user) err = %v, want ErrNotFound", err)
	}
	if _, err := store.CountMessages(ctx, ownerConv.ID, "user-b"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("CountMessages(wrong user) err = %v, want ErrNotFound", err)
	}
	if err := store.DeleteConversation(ctx, ownerConv.ID, "user-b"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteConversation(wrong user) err = %v, want ErrNotFound", err)
	}

	if _, err := store.AddMessage(ctx, ownerConv.ID, Message{Role: "user", Content: "owner message"}, "user-a"); err != nil {
		t.Fatalf("AddMessage(owner): %v", err)
	}
	messages, err := store.GetMessages(ctx, ownerConv.ID, 10, 0, "user-a")
	if err != nil {
		t.Fatalf("GetMessages(owner): %v", err)
	}
	if len(messages) != 1 || messages[0].Content != "owner message" {
		t.Fatalf("owner messages = %#v", messages)
	}

	if _, err := store.GetConversation(ctx, otherConv.ID, "user-b"); err != nil {
		t.Fatalf("GetConversation(other owner): %v", err)
	}
	if _, err := store.GetConversation(ctx, ownerConv.ID); err != nil {
		t.Fatalf("GetConversation(unscoped): %v", err)
	}
}
