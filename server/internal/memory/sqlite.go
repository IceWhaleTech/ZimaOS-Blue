// Package memory provides conversation history storage using SQLite.
package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

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
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MessageAttachment represents an attachment in a message.
type MessageAttachment struct {
	Type     string `json:"type"`      // "image" or "file"
	Name     string `json:"name"`      // filename
	MimeType string `json:"mime_type"` // MIME type
	Data     string `json:"data"`      // base64 encoded data
}

// Message represents a chat message.
type Message struct {
	ID             string              `json:"id"`
	ConversationID string              `json:"conversation_id"`
	Role           string              `json:"role"`
	Content        string              `json:"content"`
	ToolCalls      []ToolCall          `json:"tool_calls,omitempty"`
	ToolCallID     string              `json:"tool_call_id,omitempty"`
	Provider       string              `json:"provider,omitempty"`
	Model          string              `json:"model,omitempty"`
	Stats          *MessageStats       `json:"stats,omitempty"`
	Attachments    []MessageAttachment `json:"attachments,omitempty"`
	CreatedAt      time.Time           `json:"created_at"`
}

// Store provides conversation storage using SQLite.
type Store struct {
	db     *sql.DB
	mu     sync.Mutex // Mutex for write operations
	ownsDB bool       // true if this Store opened the DB and should close it
}

// NewStore creates a new memory store.
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings for better concurrency
	db.SetMaxOpenConns(1) // SQLite only supports one writer at a time
	db.SetMaxIdleConns(1)

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Set busy timeout for concurrent access
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
	}

	// Reduce page cache for lower idle memory (~512KB instead of default ~2MB)
	db.Exec("PRAGMA cache_size=-500")

	store := &Store{db: db, ownsDB: true}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate: %w", err)
	}

	// Release unused memory after schema init
	db.Exec("PRAGMA shrink_memory")

	return store, nil
}

// NewStoreWithDB creates a memory store using an existing shared database connection.
// The caller is responsible for managing the DB lifecycle (pragmas, connection pool, close).
func NewStoreWithDB(db *sql.DB) (*Store, error) {
	store := &Store{db: db, ownsDB: false}
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
		provider TEXT,
		model TEXT,
		stats TEXT,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_messages_conversation_id ON messages(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_conversations_updated_at ON conversations(updated_at);
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
		"ALTER TABLE conversations ADD COLUMN user_id TEXT DEFAULT ''",
	}

	for _, migration := range migrations {
		// Ignore errors for columns that already exist
		s.db.Exec(migration)
	}

	// Create index for user_id filtering (ignore if exists)
	s.db.Exec("CREATE INDEX IF NOT EXISTS idx_conversations_user_id ON conversations(user_id)")

	return nil
}

// Close closes the database connection if this Store owns it.
func (s *Store) Close() error {
	if s.ownsDB {
		return s.db.Close()
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
func (s *Store) GetConversation(ctx context.Context, id string) (*Conversation, error) {
	conv := &Conversation{}
	err := s.db.QueryRowContext(ctx,
		"SELECT id, title, user_id, created_at, updated_at FROM conversations WHERE id = ?",
		id,
	).Scan(&conv.ID, &conv.Title, &conv.UserID, &conv.CreatedAt, &conv.UpdatedAt)

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
			"SELECT id, title, user_id, created_at, updated_at FROM conversations WHERE user_id = ? ORDER BY updated_at DESC LIMIT ? OFFSET ?",
			userID[0], limit, offset,
		)
	} else {
		rows, err = s.db.QueryContext(ctx,
			"SELECT id, title, user_id, created_at, updated_at FROM conversations ORDER BY updated_at DESC LIMIT ? OFFSET ?",
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
		if err := rows.Scan(&conv.ID, &conv.Title, &conv.UserID, &conv.CreatedAt, &conv.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan conversation: %w", err)
		}
		convs = append(convs, conv)
	}

	return convs, rows.Err()
}

// DeleteConversation deletes a conversation and its messages.
func (s *Store) DeleteConversation(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Messages are deleted via CASCADE
	_, err := s.db.ExecContext(ctx, "DELETE FROM conversations WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}
	return nil
}

// UpdateConversationTitle updates a conversation's title.
func (s *Store) UpdateConversationTitle(ctx context.Context, id, title string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx,
		"UPDATE conversations SET title = ?, updated_at = ? WHERE id = ?",
		title, timeutil.NowTime(), id,
	)
	if err != nil {
		return fmt.Errorf("failed to update conversation title: %w", err)
	}
	return nil
}

// SearchConversations searches conversations by title, optionally filtered by userID.
func (s *Store) SearchConversations(ctx context.Context, query string, limit int, userID ...string) ([]Conversation, error) {
	var rows *sql.Rows
	var err error

	if len(userID) > 0 && userID[0] != "" {
		rows, err = s.db.QueryContext(ctx,
			"SELECT id, title, user_id, created_at, updated_at FROM conversations WHERE user_id = ? AND title LIKE ? ORDER BY updated_at DESC LIMIT ?",
			userID[0], "%"+query+"%", limit,
		)
	} else {
		rows, err = s.db.QueryContext(ctx,
			"SELECT id, title, user_id, created_at, updated_at FROM conversations WHERE title LIKE ? ORDER BY updated_at DESC LIMIT ?",
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
		if err := rows.Scan(&conv.ID, &conv.Title, &conv.UserID, &conv.CreatedAt, &conv.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan conversation: %w", err)
		}
		convs = append(convs, conv)
	}

	return convs, rows.Err()
}

// AddMessage adds a message to a conversation.
func (s *Store) AddMessage(ctx context.Context, conversationID string, msg Message) (*Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	msg.ID = uuid.New().String()
	msg.ConversationID = conversationID
	msg.CreatedAt = timeutil.NowTime()

	var toolCallsJSON []byte
	if len(msg.ToolCalls) > 0 {
		var err error
		toolCallsJSON, err = json.Marshal(msg.ToolCalls)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal tool calls: %w", err)
		}
	}

	var statsJSON []byte
	if msg.Stats != nil {
		var err error
		statsJSON, err = json.Marshal(msg.Stats)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal stats: %w", err)
		}
	}

	var attachmentsJSON []byte
	if len(msg.Attachments) > 0 {
		var err error
		attachmentsJSON, err = json.Marshal(msg.Attachments)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal attachments: %w", err)
		}
	}

	_, err := s.db.ExecContext(ctx,
		"INSERT INTO messages (id, conversation_id, role, content, tool_calls, tool_call_id, provider, model, stats, attachments, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		msg.ID, msg.ConversationID, msg.Role, msg.Content, toolCallsJSON, msg.ToolCallID, msg.Provider, msg.Model, statsJSON, attachmentsJSON, msg.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to add message: %w", err)
	}

	// Update conversation's updated_at
	s.db.ExecContext(ctx, "UPDATE conversations SET updated_at = ? WHERE id = ?", timeutil.NowTime(), conversationID)

	return &msg, nil
}

// UpdateMessageContent updates the content (and optionally stats) of an existing message.
// Used for incremental persistence during streaming.
func (s *Store) UpdateMessageContent(ctx context.Context, messageID, content string, stats *MessageStats) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if stats != nil {
		statsJSON, err := json.Marshal(stats)
		if err != nil {
			return fmt.Errorf("failed to marshal stats: %w", err)
		}
		_, err = s.db.ExecContext(ctx,
			"UPDATE messages SET content = ?, stats = ? WHERE id = ?",
			content, statsJSON, messageID,
		)
		return err
	}
	_, err := s.db.ExecContext(ctx,
		"UPDATE messages SET content = ? WHERE id = ?",
		content, messageID,
	)
	return err
}

// GetMessages retrieves messages for a conversation.
func (s *Store) GetMessages(ctx context.Context, conversationID string, limit, offset int) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, conversation_id, role, content, tool_calls, tool_call_id, provider, model, stats, attachments, created_at FROM messages WHERE conversation_id = ? ORDER BY created_at ASC LIMIT ? OFFSET ?",
		conversationID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		var toolCallsJSON sql.NullString
		var toolCallID sql.NullString
		var provider sql.NullString
		var model sql.NullString
		var statsJSON sql.NullString
		var attachmentsJSON sql.NullString

		if err := rows.Scan(&msg.ID, &msg.ConversationID, &msg.Role, &msg.Content, &toolCallsJSON, &toolCallID, &provider, &model, &statsJSON, &attachmentsJSON, &msg.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		if toolCallsJSON.Valid && toolCallsJSON.String != "" {
			if err := json.Unmarshal([]byte(toolCallsJSON.String), &msg.ToolCalls); err != nil {
				return nil, fmt.Errorf("failed to unmarshal tool calls: %w", err)
			}
		}

		if toolCallID.Valid {
			msg.ToolCallID = toolCallID.String
		}

		if provider.Valid {
			msg.Provider = provider.String
		}

		if model.Valid {
			msg.Model = model.String
		}

		if statsJSON.Valid && statsJSON.String != "" {
			msg.Stats = &MessageStats{}
			if err := json.Unmarshal([]byte(statsJSON.String), msg.Stats); err != nil {
				return nil, fmt.Errorf("failed to unmarshal stats: %w", err)
			}
		}

		if attachmentsJSON.Valid && attachmentsJSON.String != "" {
			if err := json.Unmarshal([]byte(attachmentsJSON.String), &msg.Attachments); err != nil {
				return nil, fmt.Errorf("failed to unmarshal attachments: %w", err)
			}
		}

		messages = append(messages, msg)
	}

	return messages, rows.Err()
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
