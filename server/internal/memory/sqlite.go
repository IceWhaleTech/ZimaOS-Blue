// Package memory provides conversation history storage using SQLite.
package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// Common errors
var (
	ErrNotFound = errors.New("not found")
)

// ToolCall represents a tool call made by the LLM.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Conversation represents a conversation.
type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Message represents a chat message.
type Message struct {
	ID             string     `json:"id"`
	ConversationID string     `json:"conversation_id"`
	Role           string     `json:"role"`
	Content        string     `json:"content"`
	ToolCalls      []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID     string     `json:"tool_call_id,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// Store provides conversation storage using SQLite.
type Store struct {
	db *sql.DB
	mu sync.Mutex // Mutex for write operations
}

// NewStore creates a new memory store.
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
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

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
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
		created_at DATETIME NOT NULL,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_messages_conversation_id ON messages(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_conversations_updated_at ON conversations(updated_at);
	`

	_, err := s.db.Exec(schema)
	return err
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// CreateConversation creates a new conversation.
func (s *Store) CreateConversation(ctx context.Context, title string) (*Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	conv := &Conversation{
		ID:        uuid.New().String(),
		Title:     title,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
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
		"SELECT id, title, created_at, updated_at FROM conversations WHERE id = ?",
		id,
	).Scan(&conv.ID, &conv.Title, &conv.CreatedAt, &conv.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	return conv, nil
}

// ListConversations lists conversations with pagination.
func (s *Store) ListConversations(ctx context.Context, limit, offset int) ([]Conversation, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, title, created_at, updated_at FROM conversations ORDER BY updated_at DESC LIMIT ? OFFSET ?",
		limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list conversations: %w", err)
	}
	defer rows.Close()

	var convs []Conversation
	for rows.Next() {
		var conv Conversation
		if err := rows.Scan(&conv.ID, &conv.Title, &conv.CreatedAt, &conv.UpdatedAt); err != nil {
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
		title, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("failed to update conversation title: %w", err)
	}
	return nil
}

// SearchConversations searches conversations by title.
func (s *Store) SearchConversations(ctx context.Context, query string, limit int) ([]Conversation, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, title, created_at, updated_at FROM conversations WHERE title LIKE ? ORDER BY updated_at DESC LIMIT ?",
		"%"+query+"%", limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search conversations: %w", err)
	}
	defer rows.Close()

	var convs []Conversation
	for rows.Next() {
		var conv Conversation
		if err := rows.Scan(&conv.ID, &conv.Title, &conv.CreatedAt, &conv.UpdatedAt); err != nil {
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
	msg.CreatedAt = time.Now()

	var toolCallsJSON []byte
	if len(msg.ToolCalls) > 0 {
		var err error
		toolCallsJSON, err = json.Marshal(msg.ToolCalls)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal tool calls: %w", err)
		}
	}

	_, err := s.db.ExecContext(ctx,
		"INSERT INTO messages (id, conversation_id, role, content, tool_calls, tool_call_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		msg.ID, msg.ConversationID, msg.Role, msg.Content, toolCallsJSON, msg.ToolCallID, msg.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to add message: %w", err)
	}

	// Update conversation's updated_at
	s.db.ExecContext(ctx, "UPDATE conversations SET updated_at = ? WHERE id = ?", time.Now(), conversationID)

	return &msg, nil
}

// GetMessages retrieves messages for a conversation.
func (s *Store) GetMessages(ctx context.Context, conversationID string, limit, offset int) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, conversation_id, role, content, tool_calls, tool_call_id, created_at FROM messages WHERE conversation_id = ? ORDER BY created_at ASC LIMIT ? OFFSET ?",
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

		if err := rows.Scan(&msg.ID, &msg.ConversationID, &msg.Role, &msg.Content, &toolCallsJSON, &toolCallID, &msg.CreatedAt); err != nil {
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

		messages = append(messages, msg)
	}

	return messages, rows.Err()
}
