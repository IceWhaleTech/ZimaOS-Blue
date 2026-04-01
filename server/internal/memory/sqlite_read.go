package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	z "github.com/IceWhaleTech/zorm"
)

func (s *Store) reader() *sql.DB {
	if s != nil && s.readDB != nil {
		return s.readDB
	}
	if s == nil {
		return nil
	}
	return s.db
}

func (s *Store) conversations(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "conversations")
}

func (s *Store) conversationsRead(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "conversations")
}

func (s *Store) messages(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "messages")
}

func (s *Store) messagesRead(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "messages")
}

func (s *Store) attachmentsRead(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "message_attachments")
}

func (s *Store) conversationCommandState(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "conversation_command_state")
}

func (s *Store) conversationCommandStateRead(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "conversation_command_state")
}

func (s *Store) userCommandState(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "user_command_state")
}

func (s *Store) userCommandStateRead(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "user_command_state")
}

func (s *Store) conversationRuntimeState(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "conversation_runtime_state")
}

func (s *Store) conversationRuntimeStateRead(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "conversation_runtime_state")
}

func openStoreReaderDB(dbPath string) (*sql.DB, error) {
	if dbPath == "" || dbPath == ":memory:" {
		return nil, nil
	}
	dsn := fmt.Sprintf("file:%s?mode=ro", dbPath)
	db, err := dbutil.OpenSQLiteWithRecovery(dsn, dbPath, func(db *sql.DB) error {
		db.SetMaxOpenConns(4)
		db.SetMaxIdleConns(2)
		if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
			return fmt.Errorf("failed to set memory store reader busy timeout: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}

type conversationRow struct {
	ID        string  `json:"id" zorm:"id"`
	Title     string  `json:"title" zorm:"title"`
	UserID    *string `json:"user_id" zorm:"user_id"`
	Pinned    bool    `json:"pinned" zorm:"pinned"`
	CreatedAt string  `json:"created_at" zorm:"created_at"`
	UpdatedAt string  `json:"updated_at" zorm:"updated_at"`
}

type messageRow struct {
	ID                string  `json:"id" zorm:"id"`
	ConversationID    string  `json:"conversation_id" zorm:"conversation_id"`
	Role              string  `json:"role" zorm:"role"`
	Content           string  `json:"content" zorm:"content"`
	ToolCalls         *string `json:"tool_calls" zorm:"tool_calls"`
	ToolCallID        *string `json:"tool_call_id" zorm:"tool_call_id"`
	ToolName          *string `json:"tool_name" zorm:"tool_name"`
	Provider          *string `json:"provider" zorm:"provider"`
	Model             *string `json:"model" zorm:"model"`
	Stats             *string `json:"stats" zorm:"stats"`
	LegacyAttachments *string `json:"attachments" zorm:"attachments"`
	HasAttachments    bool    `json:"has_attachments" zorm:"has_attachments"`
	CreatedAt         string  `json:"created_at" zorm:"created_at"`
}

type messageAttachmentFileRow struct {
	FilePath string `json:"file_path" zorm:"file_path"`
}

type conversationCommandStateRow struct {
	SelectedProviderID  string `json:"selected_provider_id" zorm:"selected_provider_id"`
	SelectedModelID     string `json:"selected_model_id" zorm:"selected_model_id"`
	Offline             bool   `json:"offline" zorm:"offline"`
	WebSearchEnabled    bool   `json:"web_search_enabled" zorm:"web_search_enabled"`
	DeepResearchEnabled bool   `json:"deep_research_enabled" zorm:"deep_research_enabled"`
	UpdatedAt           string `json:"updated_at" zorm:"updated_at"`
}

type conversationScopeRow struct {
	UserID *string `json:"user_id" zorm:"user_id"`
}

type previousResponseIDRow struct {
	PreviousResponseID string `json:"previous_response_id" zorm:"previous_response_id"`
}

type scannedMessage struct {
	message           Message
	legacyAttachments *string
	hasAttachments    bool
}

func formatStoreTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339Nano)
}

func conversationValues(conv *Conversation) z.V {
	return z.V{
		"id":         conv.ID,
		"title":      conv.Title,
		"user_id":    conv.UserID,
		"pinned":     conv.Pinned,
		"created_at": formatStoreTime(conv.CreatedAt),
		"updated_at": formatStoreTime(conv.UpdatedAt),
	}
}

func conversationCommandStateValues(conversationID string, state ConversationCommandState) z.V {
	return z.V{
		"conversation_id":       conversationID,
		"selected_provider_id":  state.SelectedProviderID,
		"selected_model_id":     state.SelectedModelID,
		"offline":               state.Offline,
		"web_search_enabled":    state.WebSearchEnabled,
		"deep_research_enabled": state.DeepResearchEnabled,
		"updated_at":            formatStoreTime(state.UpdatedAt),
	}
}

func userCommandStateValues(userID string, state ConversationCommandState) z.V {
	return z.V{
		"user_id":               userID,
		"selected_provider_id":  state.SelectedProviderID,
		"selected_model_id":     state.SelectedModelID,
		"offline":               state.Offline,
		"web_search_enabled":    state.WebSearchEnabled,
		"deep_research_enabled": state.DeepResearchEnabled,
		"updated_at":            formatStoreTime(state.UpdatedAt),
	}
}

func conversationRuntimeStateValues(conversationID, previousResponseID string, updatedAt time.Time) z.V {
	return z.V{
		"conversation_id":      conversationID,
		"provider_id":          "",
		"model_id":             "",
		"previous_response_id": previousResponseID,
		"assistant_message_id": "",
		"updated_at":           formatStoreTime(updatedAt),
	}
}

func jsonTextValue(payload []byte) interface{} {
	if len(payload) == 0 {
		return nil
	}
	return string(payload)
}

func messageValues(msg Message, toolCallsJSON, statsJSON, attachmentsJSON []byte, hasAttachments bool) z.V {
	return z.V{
		"id":              msg.ID,
		"conversation_id": msg.ConversationID,
		"role":            msg.Role,
		"content":         msg.Content,
		"tool_calls":      jsonTextValue(toolCallsJSON),
		"tool_call_id":    msg.ToolCallID,
		"tool_name":       msg.ToolName,
		"provider":        msg.Provider,
		"model":           msg.Model,
		"stats":           jsonTextValue(statsJSON),
		"attachments":     jsonTextValue(attachmentsJSON),
		"has_attachments": hasAttachments,
		"created_at":      formatStoreTime(msg.CreatedAt),
	}
}

func rowToConversation(row conversationRow) *Conversation {
	conv := &Conversation{
		ID:        row.ID,
		Title:     row.Title,
		Pinned:    row.Pinned,
		CreatedAt: parseStoreTime(row.CreatedAt),
		UpdatedAt: parseStoreTime(row.UpdatedAt),
	}
	if row.UserID != nil {
		conv.UserID = strings.TrimSpace(*row.UserID)
	}
	return conv
}

func rowToScannedMessage(row messageRow) (scannedMessage, error) {
	scanned := scannedMessage{
		message: Message{
			ID:             row.ID,
			ConversationID: row.ConversationID,
			Role:           row.Role,
			Content:        row.Content,
			CreatedAt:      parseStoreTime(row.CreatedAt),
		},
		legacyAttachments: row.LegacyAttachments,
		hasAttachments:    row.HasAttachments,
	}
	msg := &scanned.message

	if row.ToolCalls != nil && strings.TrimSpace(*row.ToolCalls) != "" {
		if err := json.Unmarshal([]byte(*row.ToolCalls), &msg.ToolCalls); err != nil {
			return scanned, fmt.Errorf("failed to unmarshal tool calls: %w", err)
		}
	}
	if row.ToolCallID != nil {
		msg.ToolCallID = *row.ToolCallID
	}
	if row.ToolName != nil {
		msg.ToolName = *row.ToolName
	}
	if row.Provider != nil {
		msg.Provider = *row.Provider
	}
	if row.Model != nil {
		msg.Model = *row.Model
	}
	if row.Stats != nil && strings.TrimSpace(*row.Stats) != "" {
		var stats MessageStats
		if err := json.Unmarshal([]byte(*row.Stats), &stats); err != nil {
			return scanned, fmt.Errorf("failed to unmarshal stats: %w", err)
		}
		msg.Stats = &stats
	}
	return scanned, nil
}

func rowToConversationCommandState(conversationID string, row conversationCommandStateRow) ConversationCommandState {
	state := defaultConversationCommandState(conversationID)
	state.SelectedProviderID = strings.TrimSpace(row.SelectedProviderID)
	state.SelectedModelID = strings.TrimSpace(row.SelectedModelID)
	state.Offline = row.Offline
	state.WebSearchEnabled = row.WebSearchEnabled
	state.DeepResearchEnabled = row.DeepResearchEnabled
	state.UpdatedAt = parseStoreTime(row.UpdatedAt)
	return state
}

func parseStoreTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02T15:04:05.999999999-07:00",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func lookupZormValue(row z.V, keys ...string) (interface{}, bool) {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			return value, true
		}
	}
	return nil, false
}

func lookupZormString(row z.V, keys ...string) string {
	value, ok := lookupZormValue(row, keys...)
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []byte:
		return strings.TrimSpace(string(typed))
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func lookupZormBool(row z.V, keys ...string) bool {
	value, ok := lookupZormValue(row, keys...)
	if !ok || value == nil {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case int:
		return typed != 0
	case int32:
		return typed != 0
	case int64:
		return typed != 0
	case float64:
		return typed != 0
	case string:
		typed = strings.TrimSpace(strings.ToLower(typed))
		return typed != "" && typed != "0" && typed != "false"
	case []byte:
		s := strings.TrimSpace(strings.ToLower(string(typed)))
		return s != "" && s != "0" && s != "false"
	default:
		s := strings.TrimSpace(strings.ToLower(fmt.Sprint(typed)))
		return s != "" && s != "0" && s != "false"
	}
}
