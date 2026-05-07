package memory

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	z "github.com/IceWhaleTech/zorm"
	"github.com/google/uuid"
)

// ForkResult contains the result of a conversation fork operation.
type ForkResult struct {
	NewConversationID string `json:"new_conversation_id"`
	Title             string `json:"title"`
	MessageCount      int    `json:"message_count"`
}

// ForkConversation copies all messages up to and including forkMessageID
// into a new conversation. The new conversation is titled "{original} (fork)".
func (s *Store) ForkConversation(ctx context.Context, conversationID, forkMessageID string) (*ForkResult, error) {
	// Validate source conversation exists
	srcConv, err := s.GetConversation(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("source conversation: %w", err)
	}

	// Get all messages in order, up to and including the fork point
	allMessages, err := s.GetMessages(ctx, conversationID, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("fetch messages: %w", err)
	}

	// Find the fork point index
	forkIdx := -1
	for i, msg := range allMessages {
		if msg.ID == forkMessageID {
			forkIdx = i
			break
		}
	}
	if forkIdx < 0 {
		return nil, fmt.Errorf("message %s not found in conversation %s", forkMessageID, conversationID)
	}

	// Messages to copy: everything up to and including the fork point
	toCopy := allMessages[:forkIdx+1]

	s.mu.Lock()
	defer s.mu.Unlock()

	// Create new conversation
	newConv := &Conversation{
		ID:        uuid.New().String(),
		Title:     srcConv.Title + " (fork)",
		CreatedAt: timeutil.NowTime(),
		UpdatedAt: timeutil.NowTime(),
	}

	_, err = s.conversations(ctx).Insert(conversationValues(newConv))
	if err != nil {
		return nil, fmt.Errorf("create fork conversation: %w", err)
	}

	// Copy messages in a transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin fork tx: %w", err)
	}
	defer tx.Rollback()

	messagesTable := z.TableContext(ctx, tx, "messages")
	for _, msg := range toCopy {
		copied := msg
		copied.ID = uuid.New().String()
		copied.ConversationID = newConv.ID
		copied.CreatedAt = msg.CreatedAt // Preserve original timestamp

		var toolCallsJSON []byte
		if len(copied.ToolCalls) > 0 {
			toolCallsJSON, _ = json.Marshal(copied.ToolCalls)
		}
		var statsJSON []byte
		if copied.Stats != nil {
			statsJSON, _ = json.Marshal(copied.Stats)
		}
		var attachmentsJSON []byte
		hasAttachments := len(copied.Attachments) > 0
		if hasAttachments && !s.shouldExternalizeAttachments() {
			attachmentsJSON, _ = json.Marshal(copied.Attachments)
		}

		if _, err := messagesTable.Insert(
			messageValues(copied, toolCallsJSON, statsJSON, attachmentsJSON, hasAttachments),
		); err != nil {
			return nil, fmt.Errorf("copy message: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit fork tx: %w", err)
	}

	return &ForkResult{
		NewConversationID: newConv.ID,
		Title:             newConv.Title,
		MessageCount:      len(toCopy),
	}, nil
}

// RewindResult contains the result of a conversation rewind operation.
type RewindResult struct {
	RemainingMessages int `json:"remaining_messages"`
	DeletedMessages   int `json:"deleted_messages"`
}

// RewindConversation deletes all messages after rewindMessageID (exclusive).
// The message at rewindMessageID is kept.
func (s *Store) RewindConversation(ctx context.Context, conversationID, rewindMessageID string) (*RewindResult, error) {
	// Validate conversation exists
	if _, err := s.GetConversation(ctx, conversationID); err != nil {
		return nil, fmt.Errorf("conversation: %w", err)
	}

	// Get all messages in order
	allMessages, err := s.GetMessages(ctx, conversationID, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("fetch messages: %w", err)
	}

	// Find the rewind point index
	rewindIdx := -1
	for i, msg := range allMessages {
		if msg.ID == rewindMessageID {
			rewindIdx = i
			break
		}
	}
	if rewindIdx < 0 {
		return nil, fmt.Errorf("message %s not found in conversation %s", rewindMessageID, conversationID)
	}

	// Messages to delete: everything after the rewind point
	toDelete := make([]string, 0)
	for i := rewindIdx + 1; i < len(allMessages); i++ {
		toDelete = append(toDelete, allMessages[i].ID)
	}

	if len(toDelete) > 0 {
		s.mu.Lock()
		_, err := s.messages(ctx).Delete(
			z.Where(
				z.Eq("conversation_id", conversationID),
				z.In("id", toDelete),
			),
		)
		s.mu.Unlock()
		if err != nil {
			return nil, fmt.Errorf("delete messages: %w", err)
		}

		// Update conversation timestamp
		s.mu.Lock()
		_, err = s.conversations(ctx).Update(
			z.V{"updated_at": formatStoreTime(timeutil.NowTime())},
			z.Fields("updated_at"),
			z.Where(z.Eq("id", conversationID)),
		)
		s.mu.Unlock()
		if err != nil {
			return nil, fmt.Errorf("update conversation: %w", err)
		}
	}

	return &RewindResult{
		RemainingMessages: rewindIdx + 1,
		DeletedMessages:   len(toDelete),
	}, nil
}

