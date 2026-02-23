package reminder

import (
	"context"
	"fmt"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
)

// MemoryStoreInjector implements MessageInjector using the memory.Store.
type MemoryStoreInjector struct {
	store *memory.Store
}

// NewMemoryStoreInjector creates a new injector backed by the memory store.
func NewMemoryStoreInjector(store *memory.Store) *MemoryStoreInjector {
	return &MemoryStoreInjector{store: store}
}

// InjectReminderMessage inserts an assistant message into the user's conversation.
// If sessionID is provided, it's used as the conversation ID directly.
// Otherwise, the most recent conversation for the user is used (or a new one is created).
func (m *MemoryStoreInjector) InjectReminderMessage(ctx context.Context, ownerID, sessionID, content string) (string, error) {
	convID := sessionID

	if convID == "" {
		// Find the most recent conversation for this user
		convs, err := m.store.ListConversations(ctx, 1, 0, ownerID)
		if err != nil {
			return "", fmt.Errorf("list conversations: %w", err)
		}
		if len(convs) > 0 {
			convID = convs[0].ID
		} else {
			// Create a new conversation for reminders
			conv, err := m.store.CreateConversation(ctx, "Reminders", ownerID)
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
