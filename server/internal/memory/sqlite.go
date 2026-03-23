// Package memory provides conversation history storage using SQLite.
package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// ToolCall represents a tool call made by the LLM.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// MessageStats represents statistics for a message.
type MessageStats struct {
	InputTokens     int     `json:"input_tokens"`
	OutputTokens    int     `json:"output_tokens"`
	TotalTokens     int     `json:"total_tokens"`
	LatencyMs       int64   `json:"latency_ms"`
	TTFTMs          int64   `json:"ttft_ms"`
	TokensPerSecond float64 `json:"tokens_per_second"`
}

// Conversation represents a conversation.
type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	UserID    string    `json:"user_id,omitempty"`
	Pinned    bool      `json:"pinned"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MessageAttachment represents an attachment in a message.
type MessageAttachment struct {
	Type     string  `json:"type"`               // "image", "file", "audio", or "video"
	Name     string  `json:"name"`               // filename
	MimeType string  `json:"mime_type"`          // MIME type
	Data     string  `json:"data"`               // base64 encoded data
	Duration float64 `json:"duration,omitempty"` // audio duration in seconds
}

// Message represents a chat message.
type Message struct {
	ID             string              `json:"id"`
	ConversationID string              `json:"conversation_id"`
	Role           string              `json:"role"`
	Content        string              `json:"content"`
	ToolCalls      []ToolCall          `json:"tool_calls,omitempty"`
	ToolCallID     string              `json:"tool_call_id,omitempty"`
	ToolName       string              `json:"tool_name,omitempty"`
	Provider       string              `json:"provider,omitempty"`
	Model          string              `json:"model,omitempty"`
	Stats          *MessageStats       `json:"stats,omitempty"`
	Attachments    []MessageAttachment `json:"attachments,omitempty"`
	CreatedAt      time.Time           `json:"created_at"`
}

// ConversationCommandState stores persisted per-conversation deterministic chat command state.
type ConversationCommandState struct {
	ConversationID      string    `json:"conversation_id"`
	SelectedProviderID  string    `json:"selected_provider_id,omitempty"`
	SelectedModelID     string    `json:"selected_model_id,omitempty"`
	Offline             bool      `json:"offline"`
	WebSearchEnabled    bool      `json:"web_search_enabled"`
	DeepResearchEnabled bool      `json:"deep_research_enabled"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// Store provides conversation storage using SQLite.
type Store struct {
	db          *sql.DB
	mu          sync.Mutex // Mutex for write operations
	ownsDB      bool       // true if this Store opened the DB and should close it
	dbPath      string
	options     StoreOptions
	bgDone      chan struct{}
	bgWG        sync.WaitGroup
	bgStarted   bool
	bgCloseOnce sync.Once
	recoveryMu  sync.Mutex
}

// responsesPreviousIDTTL limits how long continuation IDs are considered valid.
const responsesPreviousIDTTL = 24 * time.Hour

// NewStore creates a new memory store.
func NewStore(dbPath string) (*Store, error) {
	return NewStoreWithOptions(dbPath, DefaultStoreOptions())
}

// NewStoreWithOptions creates a new owned memory store with custom SQLite settings.
func NewStoreWithOptions(dbPath string, opts StoreOptions) (*Store, error) {
	db, err := openOwnedStoreDB(dbPath, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &Store{
		db:      db,
		ownsDB:  true,
		dbPath:  dbPath,
		options: normalizeStoreOptions(dbPath, opts),
		bgDone:  make(chan struct{}),
	}
	store.startOwnedLoops()
	return store, nil
}

// NewStoreWithDB creates a memory store using an existing shared database connection.
// The caller is responsible for managing the DB lifecycle (pragmas, connection pool, close).
func NewStoreWithDB(db *sql.DB) (*Store, error) {
	store := &Store{db: db, ownsDB: false, options: DefaultStoreOptions()}
	if err := store.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate: %w", err)
	}
	return store, nil
}

// migrate creates the database schema.
func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS conversations (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		pinned BOOLEAN DEFAULT 0,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		conversation_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		tool_calls TEXT,
		tool_call_id TEXT,
		tool_name TEXT,
		provider TEXT,
		model TEXT,
		stats TEXT,
		attachments TEXT,
		has_attachments BOOLEAN NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS message_attachments (
		message_id TEXT NOT NULL,
		attachment_index INTEGER NOT NULL,
		type TEXT NOT NULL DEFAULT '',
		name TEXT NOT NULL DEFAULT '',
		mime_type TEXT NOT NULL DEFAULT '',
		duration REAL NOT NULL DEFAULT 0,
		file_path TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		PRIMARY KEY (message_id, attachment_index),
		FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS conversation_runtime_state (
		conversation_id TEXT NOT NULL,
		provider_id TEXT NOT NULL DEFAULT '',
		model_id TEXT NOT NULL DEFAULT '',
		previous_response_id TEXT NOT NULL DEFAULT '',
		assistant_message_id TEXT NOT NULL DEFAULT '',
		updated_at DATETIME NOT NULL,
		PRIMARY KEY (conversation_id, provider_id, model_id),
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS conversation_command_state (
		conversation_id TEXT PRIMARY KEY,
		selected_provider_id TEXT NOT NULL DEFAULT '',
		selected_model_id TEXT NOT NULL DEFAULT '',
		offline BOOLEAN NOT NULL DEFAULT 0,
		web_search_enabled BOOLEAN NOT NULL DEFAULT 1,
		deep_research_enabled BOOLEAN NOT NULL DEFAULT 0,
		updated_at DATETIME NOT NULL,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS user_command_state (
		user_id TEXT PRIMARY KEY,
		selected_provider_id TEXT NOT NULL DEFAULT '',
		selected_model_id TEXT NOT NULL DEFAULT '',
		offline BOOLEAN NOT NULL DEFAULT 0,
		web_search_enabled BOOLEAN NOT NULL DEFAULT 1,
		deep_research_enabled BOOLEAN NOT NULL DEFAULT 0,
		updated_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_messages_conversation_id ON messages(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_conversations_updated_at ON conversations(updated_at);
	CREATE INDEX IF NOT EXISTS idx_conversation_runtime_state_updated_at ON conversation_runtime_state(updated_at);
	CREATE INDEX IF NOT EXISTS idx_conversation_command_state_updated_at ON conversation_command_state(updated_at);
	CREATE INDEX IF NOT EXISTS idx_user_command_state_updated_at ON user_command_state(updated_at);
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		return err
	}

	// Run migrations for existing databases
	migrations := []string{
		"ALTER TABLE messages ADD COLUMN provider TEXT",
		"ALTER TABLE messages ADD COLUMN model TEXT",
		"ALTER TABLE messages ADD COLUMN stats TEXT",
		"ALTER TABLE messages ADD COLUMN attachments TEXT",
		"ALTER TABLE messages ADD COLUMN has_attachments BOOLEAN NOT NULL DEFAULT 0",
		"ALTER TABLE messages ADD COLUMN tool_name TEXT",
		"ALTER TABLE conversations ADD COLUMN user_id TEXT DEFAULT ''",
		"ALTER TABLE conversations ADD COLUMN pinned BOOLEAN DEFAULT 0",
	}

	for _, migration := range migrations {
		// Ignore errors for columns that already exist
		s.db.Exec(migration)
	}

	// Create index for user_id filtering (ignore if exists)
	s.db.Exec("CREATE INDEX IF NOT EXISTS idx_conversations_user_id ON conversations(user_id)")
	// Create index for pinned conversations
	s.db.Exec("CREATE INDEX IF NOT EXISTS idx_conversations_pinned ON conversations(pinned)")
	s.db.Exec("CREATE INDEX IF NOT EXISTS idx_message_attachments_message_id ON message_attachments(message_id)")

	return nil
}

// Close closes the database connection if this Store owns it.
func (s *Store) Close() error {
	if s.ownsDB {
		s.stopOwnedLoops()
		if s.db != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_ = s.checkpoint(ctx, dbutil.CheckpointTruncate)
			cancel()
			return s.db.Close()
		}
	}
	return nil
}

func normalizeConversationScope(userID []string) string {
	if len(userID) == 0 {
		return ""
	}
	return strings.TrimSpace(userID[0])
}

func (s *Store) ensureConversationAccess(ctx context.Context, conversationID, scopedUserID string) error {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" || scopedUserID == "" {
		return nil
	}

	var exists int
	err := s.db.QueryRowContext(ctx,
		"SELECT 1 FROM conversations WHERE id = ? AND user_id = ? LIMIT 1",
		conversationID, scopedUserID,
	).Scan(&exists)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to verify conversation ownership: %w", err)
	}
	return nil
}

// CreateConversation creates a new conversation.
func (s *Store) CreateConversation(ctx context.Context, title string, userID ...string) (*Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	conv := &Conversation{
		ID:        uuid.New().String(),
		Title:     title,
		CreatedAt: timeutil.NowTime(),
		UpdatedAt: timeutil.NowTime(),
	}
	if len(userID) > 0 {
		conv.UserID = userID[0]
	}

	_, err := s.db.ExecContext(ctx,
		"INSERT INTO conversations (id, title, user_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		conv.ID, conv.Title, conv.UserID, conv.CreatedAt, conv.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	return conv, nil
}

// CreateConversationWithID creates a conversation with a specific ID (for IM channels).
func (s *Store) CreateConversationWithID(ctx context.Context, id, title string) (*Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	conv := &Conversation{
		ID:        id,
		Title:     title,
		CreatedAt: timeutil.NowTime(),
		UpdatedAt: timeutil.NowTime(),
	}

	_, err := s.db.ExecContext(ctx,
		"INSERT INTO conversations (id, title, created_at, updated_at) VALUES (?, ?, ?, ?)",
		conv.ID, conv.Title, conv.CreatedAt, conv.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	return conv, nil
}

// GetConversation retrieves a conversation by ID.
func (s *Store) GetConversation(ctx context.Context, id string, userID ...string) (*Conversation, error) {
	scopedUserID := normalizeConversationScope(userID)
	conv := &Conversation{}
	var err error
	if scopedUserID != "" {
		err = s.db.QueryRowContext(ctx,
			"SELECT id, title, user_id, pinned, created_at, updated_at FROM conversations WHERE id = ? AND user_id = ?",
			id, scopedUserID,
		).Scan(&conv.ID, &conv.Title, &conv.UserID, &conv.Pinned, &conv.CreatedAt, &conv.UpdatedAt)
	} else {
		err = s.db.QueryRowContext(ctx,
			"SELECT id, title, user_id, pinned, created_at, updated_at FROM conversations WHERE id = ?",
			id,
		).Scan(&conv.ID, &conv.Title, &conv.UserID, &conv.Pinned, &conv.CreatedAt, &conv.UpdatedAt)
	}

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	return conv, nil
}

// ListConversations lists conversations with pagination, optionally filtered by userID.
func (s *Store) ListConversations(ctx context.Context, limit, offset int, userID ...string) ([]Conversation, error) {
	var rows *sql.Rows
	var err error

	if len(userID) > 0 && userID[0] != "" {
		rows, err = s.db.QueryContext(ctx,
			"SELECT id, title, user_id, pinned, created_at, updated_at FROM conversations WHERE user_id = ? ORDER BY pinned DESC, updated_at DESC, created_at DESC, rowid DESC LIMIT ? OFFSET ?",
			userID[0], limit, offset,
		)
	} else {
		rows, err = s.db.QueryContext(ctx,
			"SELECT id, title, user_id, pinned, created_at, updated_at FROM conversations ORDER BY pinned DESC, updated_at DESC, created_at DESC, rowid DESC LIMIT ? OFFSET ?",
			limit, offset,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list conversations: %w", err)
	}
	defer rows.Close()

	var convs []Conversation
	for rows.Next() {
		var conv Conversation
		if err := rows.Scan(&conv.ID, &conv.Title, &conv.UserID, &conv.Pinned, &conv.CreatedAt, &conv.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan conversation: %w", err)
		}
		convs = append(convs, conv)
	}

	return convs, rows.Err()
}

// DeleteConversation deletes a conversation and its messages.
func (s *Store) DeleteConversation(ctx context.Context, id string, userID ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	scopedUserID := normalizeConversationScope(userID)
	var (
		res sql.Result
		err error
	)
	// Messages are deleted via CASCADE
	if scopedUserID != "" {
		res, err = s.db.ExecContext(ctx, "DELETE FROM conversations WHERE id = ? AND user_id = ?", id, scopedUserID)
	} else {
		res, err = s.db.ExecContext(ctx, "DELETE FROM conversations WHERE id = ?", id)
	}
	if err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}
	if scopedUserID != "" {
		if affected, rowsErr := res.RowsAffected(); rowsErr == nil && affected == 0 {
			return ErrNotFound
		}
	}
	return nil
}

// UpdateConversationTitle updates a conversation's title.
func (s *Store) UpdateConversationTitle(ctx context.Context, id, title string, userID ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	scopedUserID := normalizeConversationScope(userID)
	var (
		res sql.Result
		err error
	)
	if scopedUserID != "" {
		res, err = s.db.ExecContext(ctx,
			"UPDATE conversations SET title = ?, updated_at = ? WHERE id = ? AND user_id = ?",
			title, timeutil.NowTime(), id, scopedUserID,
		)
	} else {
		res, err = s.db.ExecContext(ctx,
			"UPDATE conversations SET title = ?, updated_at = ? WHERE id = ?",
			title, timeutil.NowTime(), id,
		)
	}
	if err != nil {
		return fmt.Errorf("failed to update conversation title: %w", err)
	}
	if scopedUserID != "" {
		if affected, rowsErr := res.RowsAffected(); rowsErr == nil && affected == 0 {
			return ErrNotFound
		}
	}
	return nil
}

// PinConversation pins a conversation.
func (s *Store) PinConversation(ctx context.Context, id string, userID ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	scopedUserID := normalizeConversationScope(userID)
	var (
		res sql.Result
		err error
	)
	if scopedUserID != "" {
		res, err = s.db.ExecContext(ctx,
			"UPDATE conversations SET pinned = 1, updated_at = ? WHERE id = ? AND user_id = ?",
			timeutil.NowTime(), id, scopedUserID,
		)
	} else {
		res, err = s.db.ExecContext(ctx,
			"UPDATE conversations SET pinned = 1, updated_at = ? WHERE id = ?",
			timeutil.NowTime(), id,
		)
	}
	if err != nil {
		return fmt.Errorf("failed to pin conversation: %w", err)
	}
	if scopedUserID != "" {
		if affected, rowsErr := res.RowsAffected(); rowsErr == nil && affected == 0 {
			return ErrNotFound
		}
	}
	return nil
}

// UnpinConversation unpins a conversation.
func (s *Store) UnpinConversation(ctx context.Context, id string, userID ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	scopedUserID := normalizeConversationScope(userID)
	var (
		res sql.Result
		err error
	)
	if scopedUserID != "" {
		res, err = s.db.ExecContext(ctx,
			"UPDATE conversations SET pinned = 0, updated_at = ? WHERE id = ? AND user_id = ?",
			timeutil.NowTime(), id, scopedUserID,
		)
	} else {
		res, err = s.db.ExecContext(ctx,
			"UPDATE conversations SET pinned = 0, updated_at = ? WHERE id = ?",
			timeutil.NowTime(), id,
		)
	}
	if err != nil {
		return fmt.Errorf("failed to unpin conversation: %w", err)
	}
	if scopedUserID != "" {
		if affected, rowsErr := res.RowsAffected(); rowsErr == nil && affected == 0 {
			return ErrNotFound
		}
	}
	return nil
}

// SearchConversations searches conversations by title, optionally filtered by userID.
func (s *Store) SearchConversations(ctx context.Context, query string, limit int, userID ...string) ([]Conversation, error) {
	var rows *sql.Rows
	var err error

	if len(userID) > 0 && userID[0] != "" {
		rows, err = s.db.QueryContext(ctx,
			"SELECT id, title, user_id, pinned, created_at, updated_at FROM conversations WHERE user_id = ? AND title LIKE ? ORDER BY pinned DESC, updated_at DESC, created_at DESC, rowid DESC LIMIT ?",
			userID[0], "%"+query+"%", limit,
		)
	} else {
		rows, err = s.db.QueryContext(ctx,
			"SELECT id, title, user_id, pinned, created_at, updated_at FROM conversations WHERE title LIKE ? ORDER BY pinned DESC, updated_at DESC, created_at DESC, rowid DESC LIMIT ?",
			"%"+query+"%", limit,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to search conversations: %w", err)
	}
	defer rows.Close()

	var convs []Conversation
	for rows.Next() {
		var conv Conversation
		if err := rows.Scan(&conv.ID, &conv.Title, &conv.UserID, &conv.Pinned, &conv.CreatedAt, &conv.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan conversation: %w", err)
		}
		convs = append(convs, conv)
	}

	return convs, rows.Err()
}

// AddMessage adds a message to a conversation.
func (s *Store) AddMessage(ctx context.Context, conversationID string, msg Message, userID ...string) (*Message, error) {
	return s.addMessage(ctx, conversationID, msg, false, userID...)
}

// AddMessageTrusted adds a message without re-checking conversation ownership.
func (s *Store) AddMessageTrusted(ctx context.Context, conversationID string, msg Message) (*Message, error) {
	return s.addMessage(ctx, conversationID, msg, true)
}

func (s *Store) addMessage(ctx context.Context, conversationID string, msg Message, trusted bool, userID ...string) (*Message, error) {
	if !trusted {
		if err := s.ensureConversationAccess(ctx, conversationID, normalizeConversationScope(userID)); err != nil {
			return nil, err
		}
	}

	var persisted Message
	err := s.retryOnCorruption(func() error {
		s.mu.Lock()
		defer s.mu.Unlock()

		persisted = msg
		if strings.TrimSpace(persisted.ID) == "" {
			persisted.ID = uuid.New().String()
		}
		persisted.ConversationID = conversationID
		if persisted.CreatedAt.IsZero() {
			persisted.CreatedAt = timeutil.NowTime()
		}

		var toolCallsJSON []byte
		if len(persisted.ToolCalls) > 0 {
			var err error
			toolCallsJSON, err = json.Marshal(persisted.ToolCalls)
			if err != nil {
				return fmt.Errorf("failed to marshal tool calls: %w", err)
			}
		}

		var statsJSON []byte
		if persisted.Stats != nil {
			var err error
			statsJSON, err = json.Marshal(persisted.Stats)
			if err != nil {
				return fmt.Errorf("failed to marshal stats: %w", err)
			}
		}

		var attachmentsJSON []byte
		hasAttachments := len(persisted.Attachments) > 0
		if hasAttachments && !s.shouldExternalizeAttachments() {
			var err error
			attachmentsJSON, err = json.Marshal(persisted.Attachments)
			if err != nil {
				return fmt.Errorf("failed to marshal attachments: %w", err)
			}
		}

		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin add message tx: %w", err)
		}
		defer tx.Rollback()

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO messages (
				id, conversation_id, role, content, tool_calls, tool_call_id, tool_name,
				provider, model, stats, attachments, has_attachments, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			persisted.ID,
			persisted.ConversationID,
			persisted.Role,
			persisted.Content,
			toolCallsJSON,
			persisted.ToolCallID,
			persisted.ToolName,
			persisted.Provider,
			persisted.Model,
			statsJSON,
			attachmentsJSON,
			hasAttachments,
			persisted.CreatedAt,
		); err != nil {
			return fmt.Errorf("failed to add message: %w", err)
		}

		if hasAttachments && s.shouldExternalizeAttachments() {
			if err := s.persistExternalAttachmentsTx(ctx, tx, persisted.ID, persisted.Attachments, persisted.CreatedAt); err != nil {
				return err
			}
		}

		if _, err := tx.ExecContext(ctx,
			"UPDATE conversations SET updated_at = ? WHERE id = ?",
			timeutil.NowTime(),
			conversationID,
		); err != nil {
			return fmt.Errorf("failed to update conversation timestamp: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit add message tx: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &persisted, nil
}

// UpdateMessageContent updates the content (and optionally stats) of an existing message.
// Used for incremental persistence during streaming.
func (s *Store) UpdateMessageContent(ctx context.Context, messageID, content string, stats *MessageStats) error {
	return s.UpdateMessageContentFull(ctx, messageID, content, "", "", stats)
}

// UpdateMessageContentFull updates a message's content, stats, and optionally provider/model.
func (s *Store) UpdateMessageContentFull(ctx context.Context, messageID, content, provider, model string, stats *MessageStats) error {
	return s.retryOnCorruption(func() error {
		s.mu.Lock()
		defer s.mu.Unlock()

		if stats != nil {
			statsJSON, err := json.Marshal(stats)
			if err != nil {
				return fmt.Errorf("failed to marshal stats: %w", err)
			}
			if provider != "" || model != "" {
				_, err = s.db.ExecContext(ctx,
					"UPDATE messages SET content = ?, stats = ?, provider = COALESCE(NULLIF(?, ''), provider), model = COALESCE(NULLIF(?, ''), model) WHERE id = ?",
					content, statsJSON, provider, model, messageID,
				)
			} else {
				_, err = s.db.ExecContext(ctx,
					"UPDATE messages SET content = ?, stats = ? WHERE id = ?",
					content, statsJSON, messageID,
				)
			}
			return err
		}
		if provider != "" || model != "" {
			_, err := s.db.ExecContext(ctx,
				"UPDATE messages SET content = ?, provider = COALESCE(NULLIF(?, ''), provider), model = COALESCE(NULLIF(?, ''), model) WHERE id = ?",
				content, provider, model, messageID,
			)
			return err
		}
		_, err := s.db.ExecContext(ctx,
			"UPDATE messages SET content = ? WHERE id = ?",
			content, messageID,
		)
		return err
	})
}

// UpsertMessageContentFullTrusted inserts a message when absent or updates draft/final content in place.
func (s *Store) UpsertMessageContentFullTrusted(ctx context.Context, msg Message) error {
	if strings.TrimSpace(msg.ID) == "" {
		return fmt.Errorf("message id is required")
	}
	if strings.TrimSpace(msg.ConversationID) == "" {
		return fmt.Errorf("conversation id is required")
	}
	if strings.TrimSpace(msg.Role) == "" {
		msg.Role = "assistant"
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = timeutil.NowTime()
	}

	return s.retryOnCorruption(func() error {
		s.mu.Lock()
		defer s.mu.Unlock()

		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin upsert message tx: %w", err)
		}
		defer tx.Rollback()

		var statsJSON []byte
		if msg.Stats != nil {
			statsJSON, err = json.Marshal(msg.Stats)
			if err != nil {
				return fmt.Errorf("failed to marshal stats: %w", err)
			}
		}

		res, err := tx.ExecContext(ctx,
			`UPDATE messages
			SET content = ?,
			    stats = CASE WHEN ? IS NULL THEN stats ELSE ? END,
			    provider = COALESCE(NULLIF(?, ''), provider),
			    model = COALESCE(NULLIF(?, ''), model)
			WHERE id = ?`,
			msg.Content,
			statsJSON,
			statsJSON,
			msg.Provider,
			msg.Model,
			msg.ID,
		)
		if err != nil {
			return fmt.Errorf("update message content: %w", err)
		}

		rowsAffected, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("message upsert rows affected: %w", err)
		}
		if rowsAffected == 0 {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO messages (
					id, conversation_id, role, content, tool_calls, tool_call_id, tool_name,
					provider, model, stats, attachments, has_attachments, created_at
				) VALUES (?, ?, ?, ?, NULL, '', '', ?, ?, ?, NULL, 0, ?)`,
				msg.ID,
				msg.ConversationID,
				msg.Role,
				msg.Content,
				msg.Provider,
				msg.Model,
				statsJSON,
				msg.CreatedAt,
			); err != nil {
				return fmt.Errorf("insert message content: %w", err)
			}
		}

		if _, err := tx.ExecContext(ctx,
			"UPDATE conversations SET updated_at = ? WHERE id = ?",
			timeutil.NowTime(),
			msg.ConversationID,
		); err != nil {
			return fmt.Errorf("update conversation timestamp: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit upsert message tx: %w", err)
		}
		return nil
	})
}

// GetMessages retrieves messages for a conversation.
func (s *Store) GetMessages(ctx context.Context, conversationID string, limit, offset int, userID ...string) ([]Message, error) {
	if err := s.ensureConversationAccess(ctx, conversationID, normalizeConversationScope(userID)); err != nil {
		return nil, err
	}
	messages, err := s.queryMessages(
		ctx,
		`SELECT id, conversation_id, role, content, tool_calls, tool_call_id, tool_name, provider, model, stats, attachments, has_attachments, created_at
		FROM messages
		WHERE conversation_id = ?
		ORDER BY created_at ASC, rowid ASC
		LIMIT ? OFFSET ?`,
		[]any{conversationID, limit, offset},
		true,
	)
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
	messages, err := s.queryMessages(
		ctx,
		`SELECT id, conversation_id, role, content, tool_calls, tool_call_id, tool_name, provider, model, stats, attachments, has_attachments, created_at
		FROM (
			SELECT id, conversation_id, role, content, tool_calls, tool_call_id, tool_name, provider, model, stats, attachments, has_attachments, created_at, rowid
			FROM messages
			WHERE conversation_id = ?
			ORDER BY created_at DESC, rowid DESC
			LIMIT ?
		)
		ORDER BY created_at ASC, rowid ASC`,
		[]any{conversationID, limit},
		true,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent messages: %w", err)
	}
	return messages, nil
}

// CountMessages returns the number of persisted messages in a conversation.
func (s *Store) CountMessages(ctx context.Context, conversationID string, userID ...string) (int, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return 0, nil
	}
	if err := s.ensureConversationAccess(ctx, conversationID, normalizeConversationScope(userID)); err != nil {
		return 0, err
	}

	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(1) FROM messages WHERE conversation_id = ?", conversationID).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count messages: %w", err)
	}
	return count, nil
}

// GetLatestAssistantMessage returns the latest assistant message for a conversation.
func (s *Store) GetLatestAssistantMessage(ctx context.Context, conversationID string, userID ...string) (*Message, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil, nil
	}
	if err := s.ensureConversationAccess(ctx, conversationID, normalizeConversationScope(userID)); err != nil {
		return nil, err
	}

	row := s.db.QueryRowContext(ctx,
		`SELECT id, conversation_id, role, content, tool_calls, tool_call_id, tool_name, provider, model, stats, attachments, has_attachments, created_at
		FROM messages
		WHERE conversation_id = ? AND role = 'assistant'
		ORDER BY created_at DESC, rowid DESC
		LIMIT 1`,
		conversationID,
	)

	scanned, err := s.scanMessageRow(row, true)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to scan latest assistant message: %w", err)
	}
	msg := scanned.message
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
	return &msg, nil
}

func defaultConversationCommandState(conversationID string) ConversationCommandState {
	return ConversationCommandState{
		ConversationID:      strings.TrimSpace(conversationID),
		WebSearchEnabled:    true,
		DeepResearchEnabled: false,
	}
}

func (s *Store) getConversationCommandStateScope(ctx context.Context, conversationID string) (string, error) {
	var userID string
	if err := s.db.QueryRowContext(ctx, "SELECT user_id FROM conversations WHERE id = ?", conversationID).Scan(&userID); err != nil {
		if err == sql.ErrNoRows {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("failed to resolve conversation command state scope: %w", err)
	}
	return strings.TrimSpace(userID), nil
}

func (s *Store) getPersistedConversationCommandState(ctx context.Context, conversationID string) (ConversationCommandState, bool, error) {
	state := defaultConversationCommandState(conversationID)
	var selectedProviderID, selectedModelID string
	var offline, webSearchEnabled, deepResearchEnabled bool
	var updatedAt time.Time
	err := s.db.QueryRowContext(ctx,
		`SELECT selected_provider_id, selected_model_id, offline, web_search_enabled, deep_research_enabled, updated_at
		FROM conversation_command_state
		WHERE conversation_id = ?`,
		conversationID,
	).Scan(&selectedProviderID, &selectedModelID, &offline, &webSearchEnabled, &deepResearchEnabled, &updatedAt)
	if err == sql.ErrNoRows {
		return state, false, nil
	}
	if err != nil {
		return state, false, fmt.Errorf("failed to get conversation command state: %w", err)
	}

	state.SelectedProviderID = strings.TrimSpace(selectedProviderID)
	state.SelectedModelID = strings.TrimSpace(selectedModelID)
	state.Offline = offline
	state.WebSearchEnabled = webSearchEnabled
	state.DeepResearchEnabled = deepResearchEnabled
	state.UpdatedAt = updatedAt
	return state, true, nil
}

func (s *Store) getPersistedUserCommandState(ctx context.Context, userID string) (ConversationCommandState, bool, error) {
	state := defaultConversationCommandState("")
	var selectedProviderID, selectedModelID string
	var offline, webSearchEnabled, deepResearchEnabled bool
	var updatedAt time.Time
	err := s.db.QueryRowContext(ctx,
		`SELECT selected_provider_id, selected_model_id, offline, web_search_enabled, deep_research_enabled, updated_at
		FROM user_command_state
		WHERE user_id = ?`,
		userID,
	).Scan(&selectedProviderID, &selectedModelID, &offline, &webSearchEnabled, &deepResearchEnabled, &updatedAt)
	if err == sql.ErrNoRows {
		return state, false, nil
	}
	if err != nil {
		return state, false, fmt.Errorf("failed to get user command state: %w", err)
	}

	state.SelectedProviderID = strings.TrimSpace(selectedProviderID)
	state.SelectedModelID = strings.TrimSpace(selectedModelID)
	state.Offline = offline
	state.WebSearchEnabled = webSearchEnabled
	state.DeepResearchEnabled = deepResearchEnabled
	state.UpdatedAt = updatedAt
	return state, true, nil
}

func (s *Store) getLatestLegacyConversationCommandStateForUser(ctx context.Context, userID string) (ConversationCommandState, bool, error) {
	state := defaultConversationCommandState("")
	var selectedProviderID, selectedModelID string
	var offline, webSearchEnabled, deepResearchEnabled bool
	var updatedAt time.Time
	err := s.db.QueryRowContext(ctx,
		`SELECT cs.selected_provider_id, cs.selected_model_id, cs.offline, cs.web_search_enabled, cs.deep_research_enabled, cs.updated_at
		FROM conversation_command_state cs
		INNER JOIN conversations c ON c.id = cs.conversation_id
		WHERE c.user_id = ?
		ORDER BY cs.updated_at DESC, cs.conversation_id DESC
		LIMIT 1`,
		userID,
	).Scan(&selectedProviderID, &selectedModelID, &offline, &webSearchEnabled, &deepResearchEnabled, &updatedAt)
	if err == sql.ErrNoRows {
		return state, false, nil
	}
	if err != nil {
		return state, false, fmt.Errorf("failed to get legacy conversation command state for user: %w", err)
	}

	state.SelectedProviderID = strings.TrimSpace(selectedProviderID)
	state.SelectedModelID = strings.TrimSpace(selectedModelID)
	state.Offline = offline
	state.WebSearchEnabled = webSearchEnabled
	state.DeepResearchEnabled = deepResearchEnabled
	state.UpdatedAt = updatedAt
	return state, true, nil
}

// GetConversationCommandState returns persisted deterministic command state for a conversation.
// For authenticated conversations (with user_id), state is user-scoped and shared across sessions.
// Missing rows fall back to defaults: provider/model auto, offline=false, web=true, deep=false.
func (s *Store) GetConversationCommandState(ctx context.Context, conversationID string) (ConversationCommandState, error) {
	conversationID = strings.TrimSpace(conversationID)
	state := defaultConversationCommandState(conversationID)
	if conversationID == "" {
		return state, nil
	}

	userID, err := s.getConversationCommandStateScope(ctx, conversationID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return state, nil
		}
		return state, err
	}
	if userID != "" {
		userState, ok, err := s.getPersistedUserCommandState(ctx, userID)
		if err != nil {
			return state, err
		}
		if ok {
			userState.ConversationID = conversationID
			return userState, nil
		}
		// Compatibility fallback for legacy per-conversation rows before user-scoped state was introduced.
		legacy, ok, err := s.getLatestLegacyConversationCommandStateForUser(ctx, userID)
		if err != nil {
			return state, err
		}
		if ok {
			legacy.ConversationID = conversationID
			return legacy, nil
		}
		return state, nil
	}

	convState, _, err := s.getPersistedConversationCommandState(ctx, conversationID)
	if err != nil {
		return state, err
	}
	return convState, nil
}

// UpsertConversationCommandState stores deterministic command state for a conversation.
// For authenticated conversations (with user_id), state is persisted once per user.
func (s *Store) UpsertConversationCommandState(ctx context.Context, state ConversationCommandState) error {
	conversationID := strings.TrimSpace(state.ConversationID)
	if conversationID == "" {
		return nil
	}

	userID, err := s.getConversationCommandStateScope(ctx, conversationID)
	if err != nil {
		return err
	}

	state.ConversationID = conversationID
	state.SelectedProviderID = strings.TrimSpace(state.SelectedProviderID)
	state.SelectedModelID = strings.TrimSpace(state.SelectedModelID)
	state.UpdatedAt = timeutil.NowTime()

	s.mu.Lock()
	defer s.mu.Unlock()

	if userID != "" {
		_, err = s.db.ExecContext(ctx,
			`INSERT INTO user_command_state (user_id, selected_provider_id, selected_model_id, offline, web_search_enabled, deep_research_enabled, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(user_id)
			DO UPDATE SET
				selected_provider_id = excluded.selected_provider_id,
				selected_model_id = excluded.selected_model_id,
				offline = excluded.offline,
				web_search_enabled = excluded.web_search_enabled,
				deep_research_enabled = excluded.deep_research_enabled,
				updated_at = excluded.updated_at`,
			userID,
			state.SelectedProviderID,
			state.SelectedModelID,
			state.Offline,
			state.WebSearchEnabled,
			state.DeepResearchEnabled,
			state.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to upsert user command state: %w", err)
		}
		return nil
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO conversation_command_state (conversation_id, selected_provider_id, selected_model_id, offline, web_search_enabled, deep_research_enabled, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(conversation_id)
		DO UPDATE SET
			selected_provider_id = excluded.selected_provider_id,
			selected_model_id = excluded.selected_model_id,
			offline = excluded.offline,
			web_search_enabled = excluded.web_search_enabled,
			deep_research_enabled = excluded.deep_research_enabled,
			updated_at = excluded.updated_at`,
		state.ConversationID,
		state.SelectedProviderID,
		state.SelectedModelID,
		state.Offline,
		state.WebSearchEnabled,
		state.DeepResearchEnabled,
		state.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert conversation command state: %w", err)
	}
	return nil
}

// ClearConversationCommandState removes persisted deterministic command state for a conversation.
// For authenticated conversations (with user_id), this clears the shared user-scoped state.
func (s *Store) ClearConversationCommandState(ctx context.Context, conversationID string) error {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil
	}

	userID, err := s.getConversationCommandStateScope(ctx, conversationID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if userID != "" {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM user_command_state WHERE user_id = ?`, userID); err != nil {
			return fmt.Errorf("failed to clear user command state: %w", err)
		}
		return nil
	}

	if _, err := s.db.ExecContext(ctx, `DELETE FROM conversation_command_state WHERE conversation_id = ?`, conversationID); err != nil {
		return fmt.Errorf("failed to clear conversation command state: %w", err)
	}
	return nil
}

// DeleteMessages deletes multiple messages by their IDs.
func (s *Store) DeleteMessages(ctx context.Context, conversationID string, messageIDs []string) error {
	if len(messageIDs) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Build placeholders for IN clause
	placeholders := make([]string, len(messageIDs))
	args := make([]interface{}, len(messageIDs)+1)
	args[0] = conversationID
	for i, id := range messageIDs {
		placeholders[i] = "?"
		args[i+1] = id
	}

	query := fmt.Sprintf(
		"DELETE FROM messages WHERE conversation_id = ? AND id IN (%s)",
		strings.Join(placeholders, ","),
	)

	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete messages: %w", err)
	}

	return nil
}

// GetConversationPreviousResponseID returns the latest persisted Responses continuation ID.
// The ID is conversation-scoped and expires automatically after TTL.
func (s *Store) GetConversationPreviousResponseID(ctx context.Context, conversationID string) (string, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return "", nil
	}

	cutoff := timeutil.NowTime().Add(-responsesPreviousIDTTL)
	var prevID string
	err := s.db.QueryRowContext(
		ctx,
		`SELECT previous_response_id
		FROM conversation_runtime_state
		WHERE conversation_id = ? AND provider_id = '' AND model_id = '' AND updated_at >= ?
		LIMIT 1`,
		conversationID,
		cutoff,
	).Scan(&prevID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get conversation previous response id: %w", err)
	}
	return strings.TrimSpace(prevID), nil
}

// SetConversationPreviousResponseID stores the latest Responses continuation ID for a conversation.
func (s *Store) SetConversationPreviousResponseID(ctx context.Context, conversationID, responseID string) error {
	conversationID = strings.TrimSpace(conversationID)
	responseID = strings.TrimSpace(responseID)
	if conversationID == "" || responseID == "" {
		return nil
	}

	now := timeutil.NowTime()
	return s.retryOnCorruption(func() error {
		s.mu.Lock()
		defer s.mu.Unlock()

		_, err := s.db.ExecContext(
			ctx,
			`INSERT INTO conversation_runtime_state (conversation_id, provider_id, model_id, previous_response_id, assistant_message_id, updated_at)
			VALUES (?, '', '', ?, '', ?)
			ON CONFLICT(conversation_id, provider_id, model_id)
			DO UPDATE SET previous_response_id = excluded.previous_response_id, updated_at = excluded.updated_at`,
			conversationID,
			responseID,
			now,
		)
		if err != nil {
			return fmt.Errorf("failed to set conversation previous response id: %w", err)
		}
		return nil
	})
}

// ClearConversationPreviousResponseID removes persisted continuation IDs for a conversation.
func (s *Store) ClearConversationPreviousResponseID(ctx context.Context, conversationID string) error {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil
	}

	return s.retryOnCorruption(func() error {
		s.mu.Lock()
		defer s.mu.Unlock()

		if _, err := s.db.ExecContext(
			ctx,
			`DELETE FROM conversation_runtime_state WHERE conversation_id = ?`,
			conversationID,
		); err != nil {
			return fmt.Errorf("failed to clear conversation previous response id: %w", err)
		}
		return nil
	})
}
