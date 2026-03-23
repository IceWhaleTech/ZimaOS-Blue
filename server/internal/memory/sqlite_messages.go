package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

type scannedMessage struct {
	message           Message
	legacyAttachments sql.NullString
	hasAttachments    bool
}

func (s *Store) scanMessageRow(scanner interface{ Scan(dest ...any) error }, includeHeavy bool) (scannedMessage, error) {
	var scanned scannedMessage
	msg := &scanned.message
	var toolCallsJSON sql.NullString
	var toolCallID sql.NullString
	var toolName sql.NullString
	var provider sql.NullString
	var model sql.NullString

	if includeHeavy {
		var statsJSON sql.NullString
		if err := scanner.Scan(
			&msg.ID, &msg.ConversationID, &msg.Role, &msg.Content,
			&toolCallsJSON, &toolCallID, &toolName, &provider, &model,
			&statsJSON, &scanned.legacyAttachments, &scanned.hasAttachments, &msg.CreatedAt,
		); err != nil {
			return scanned, err
		}
		if statsJSON.Valid && strings.TrimSpace(statsJSON.String) != "" {
			var stats MessageStats
			if err := json.Unmarshal([]byte(statsJSON.String), &stats); err != nil {
				return scanned, fmt.Errorf("failed to unmarshal stats: %w", err)
			}
			msg.Stats = &stats
		}
	} else {
		if err := scanner.Scan(
			&msg.ID, &msg.ConversationID, &msg.Role, &msg.Content,
			&toolCallsJSON, &toolCallID, &toolName, &provider, &model,
			&msg.CreatedAt,
		); err != nil {
			return scanned, err
		}
	}

	if toolCallsJSON.Valid && strings.TrimSpace(toolCallsJSON.String) != "" {
		if err := json.Unmarshal([]byte(toolCallsJSON.String), &msg.ToolCalls); err != nil {
			return scanned, fmt.Errorf("failed to unmarshal tool calls: %w", err)
		}
	}
	if toolCallID.Valid {
		msg.ToolCallID = toolCallID.String
	}
	if toolName.Valid {
		msg.ToolName = toolName.String
	}
	if provider.Valid {
		msg.Provider = provider.String
	}
	if model.Valid {
		msg.Model = model.String
	}
	return scanned, nil
}

func (s *Store) queryMessages(ctx context.Context, query string, args []any, includeHeavy bool) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	scannedRows := make([]scannedMessage, 0)
	for rows.Next() {
		msg, err := s.scanMessageRow(rows, includeHeavy)
		if err != nil {
			return nil, err
		}
		scannedRows = append(scannedRows, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
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
	return s.queryMessages(
		ctx,
		`SELECT id, conversation_id, role, content, tool_calls, tool_call_id, tool_name, provider, model, created_at
		FROM messages
		WHERE conversation_id = ?
		ORDER BY created_at ASC, rowid ASC
		LIMIT ? OFFSET ?`,
		[]any{conversationID, limit, offset},
		false,
	)
}

// GetRecentMessagesLite retrieves recent messages without stats/attachments JSON payloads.
func (s *Store) GetRecentMessagesLite(ctx context.Context, conversationID string, limit int, userID ...string) ([]Message, error) {
	if err := s.ensureConversationAccess(ctx, conversationID, normalizeConversationScope(userID)); err != nil {
		return nil, err
	}
	if limit <= 0 {
		return nil, nil
	}
	return s.queryMessages(
		ctx,
		`SELECT id, conversation_id, role, content, tool_calls, tool_call_id, tool_name, provider, model, created_at
		FROM (
			SELECT id, conversation_id, role, content, tool_calls, tool_call_id, tool_name, provider, model, created_at, rowid
			FROM messages
			WHERE conversation_id = ?
			ORDER BY created_at DESC, rowid DESC
			LIMIT ?
		)
		ORDER BY created_at ASC, rowid ASC`,
		[]any{conversationID, limit},
		false,
	)
}
