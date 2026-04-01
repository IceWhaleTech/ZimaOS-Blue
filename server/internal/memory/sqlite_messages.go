package memory

import (
	"context"
	"fmt"

	z "github.com/IceWhaleTech/zorm"
)

func messageSelectFields(includeHeavy bool) []string {
	fields := []string{
		"id", "conversation_id", "role", "content", "tool_calls", "tool_call_id",
		"tool_name", "provider", "model", "created_at",
	}
	if includeHeavy {
		fields = []string{
			"id", "conversation_id", "role", "content", "tool_calls", "tool_call_id",
			"tool_name", "provider", "model", "stats", "attachments", "has_attachments", "created_at",
		}
	}
	return fields
}

func reverseMessageRows(rows []messageRow) {
	for left, right := 0, len(rows)-1; left < right; left, right = left+1, right-1 {
		rows[left], rows[right] = rows[right], rows[left]
	}
}

func (s *Store) queryMessages(ctx context.Context, conversationID string, limit, offset int, includeHeavy, recent bool) ([]Message, error) {
	opts := []z.ZormItem{
		z.Fields(messageSelectFields(includeHeavy)...),
		z.Where(z.Eq("conversation_id", conversationID)),
	}
	if recent {
		opts = append(opts,
			z.OrderBy("created_at DESC", "rowid DESC"),
			z.Limit(limit),
		)
	} else {
		opts = append(opts,
			z.OrderBy("created_at ASC", "rowid ASC"),
			z.Limit(limit, offset),
		)
	}

	var rows []messageRow
	_, err := s.messagesRead(ctx).Select(&rows, opts...)
	if err != nil {
		return nil, err
	}
	if recent {
		reverseMessageRows(rows)
	}

	scannedRows := make([]scannedMessage, 0, len(rows))
	for i := range rows {
		msg, err := rowToScannedMessage(rows[i])
		if err != nil {
			return nil, err
		}
		if !includeHeavy {
			msg.message.Stats = nil
			msg.message.Attachments = nil
			msg.legacyAttachments = nil
			msg.hasAttachments = false
		}
		scannedRows = append(scannedRows, msg)
	}

	messages := make([]Message, 0, len(scannedRows))
	for _, scanned := range scannedRows {
		msg := scanned.message
		if includeHeavy {
			if scanned.hasAttachments {
				attachments, err := s.loadExternalAttachments(ctx, msg.ID)
				if err != nil {
					return nil, err
				}
				if len(attachments) > 0 {
					msg.Attachments = attachments
				} else {
					attachments, err := parseLegacyAttachments(scanned.legacyAttachments)
					if err != nil {
						return nil, err
					}
					msg.Attachments = attachments
				}
			} else {
				attachments, err := parseLegacyAttachments(scanned.legacyAttachments)
				if err != nil {
					return nil, err
				}
				msg.Attachments = attachments
			}
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

// GetMessagesLite retrieves messages without stats/attachments JSON payloads.
func (s *Store) GetMessagesLite(ctx context.Context, conversationID string, limit, offset int, userID ...string) ([]Message, error) {
	if err := s.ensureConversationAccess(ctx, conversationID, normalizeConversationScope(userID)); err != nil {
		return nil, err
	}
	return s.queryMessages(ctx, conversationID, limit, offset, false, false)
}

// GetRecentMessagesLite retrieves recent messages without stats/attachments JSON payloads.
func (s *Store) GetRecentMessagesLite(ctx context.Context, conversationID string, limit int, userID ...string) ([]Message, error) {
	if err := s.ensureConversationAccess(ctx, conversationID, normalizeConversationScope(userID)); err != nil {
		return nil, err
	}
	if limit <= 0 {
		return nil, nil
	}
	return s.queryMessages(ctx, conversationID, limit, 0, false, true)
}

// GetMessages retrieves messages for a conversation.
func (s *Store) GetMessages(ctx context.Context, conversationID string, limit, offset int, userID ...string) ([]Message, error) {
	if err := s.ensureConversationAccess(ctx, conversationID, normalizeConversationScope(userID)); err != nil {
		return nil, err
	}
	messages, err := s.queryMessages(ctx, conversationID, limit, offset, true, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	return messages, nil
}

// GetRecentMessages retrieves the latest messages for a conversation and returns
// them in chronological order.
func (s *Store) GetRecentMessages(ctx context.Context, conversationID string, limit int, userID ...string) ([]Message, error) {
	if err := s.ensureConversationAccess(ctx, conversationID, normalizeConversationScope(userID)); err != nil {
		return nil, err
	}
	if limit <= 0 {
		return nil, nil
	}
	messages, err := s.queryMessages(ctx, conversationID, limit, 0, true, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent messages: %w", err)
	}
	return messages, nil
}
