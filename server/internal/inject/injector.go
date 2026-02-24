// Package inject provides message injection into conversations.
// This is a shared utility used by push notifications, auto-reply, and other
// features that need to insert messages into user conversations.
package inject

import (
	"context"
	"fmt"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
)

// MessageInjector injects messages into user conversations.
type MessageInjector interface {
	// InjectMessage inserts a message into the user's conversation.
	// If sessionID is provided, it's used as the conversation ID directly.
	// Otherwise, the most recent conversation for the user is used (or a new one is created).
	InjectMessage(ctx context.Context, ownerID, sessionID, content string) (conversationID string, err error)
}

// MemoryStoreInjector implements MessageInjector using the memory.Store.
type MemoryStoreInjector struct {
	store *memory.Store
}

// NewMemoryStoreInjector creates a new injector backed by the memory store.
func NewMemoryStoreInjector(store *memory.Store) *MemoryStoreInjector {
	return &MemoryStoreInjector{store: store}
}

// InjectMessage inserts an assistant message into the user's conversation.
func (m *MemoryStoreInjector) InjectMessage(ctx context.Context, ownerID, sessionID, content string) (string, error) {
	convID := sessionID

	if convID == "" {
		convs, err := m.store.ListConversations(ctx, 1, 0, ownerID)
		if err != nil {
			return "", fmt.Errorf("list conversations: %w", err)
		}
		if len(convs) > 0 {
			convID = convs[0].ID
		} else {
			conv, err := m.store.CreateConversation(ctx, "Notifications", ownerID)
			if err != nil {
				return "", fmt.Errorf("create conversation: %w", err)
			}
			convID = conv.ID
		}
	}

	msg := memory.Message{
		Role:    "assistant",
		Content: content,
	}
	if _, err := m.store.AddMessage(ctx, convID, msg); err != nil {
		return "", fmt.Errorf("add message: %w", err)
	}

	return convID, nil
}
