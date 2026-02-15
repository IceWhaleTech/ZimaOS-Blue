package session

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/context"
	_ "github.com/mattn/go-sqlite3"
)

// SessionStore interface for session persistence.
type SessionStore interface {
	Save(session *Session) error
	Load(id SessionID) (*Session, error)
	Delete(id SessionID) error
	List(filter SessionFilter) ([]*Session, error)
	Archive(id SessionID) error
	Close() error
}

// SQLiteSessionStore implements SessionStore using SQLite.
type SQLiteSessionStore struct {
	db        *sql.DB
	mu        sync.Mutex
	maxTokens int
}

// sessionRow represents a session row in the database.
type sessionRow struct {
	ID           string
	AgentID      string
	ChannelID    string
	PeerID       string
	ThreadID     sql.NullString
	State        int
	Metadata     string
	Messages     string
	SystemPrompt sql.NullString
	CreatedAt    time.Time
	UpdatedAt    time.Time
	LastActiveAt time.Time
	CompactedAt  sql.NullTime
}

// messagesData represents serialized messages.
type messagesData struct {
	Messages []messageData `json:"messages"`
}

// messageData represents a serialized message.
type messageData struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// toolCall represents a serialized tool call.
type toolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

const sessionSchema = `
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    agent_id TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    peer_id TEXT NOT NULL,
    thread_id TEXT,
    state INTEGER NOT NULL DEFAULT 0,
    metadata TEXT,
    messages TEXT,
    system_prompt TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    last_active_at DATETIME NOT NULL,
    compacted_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_sessions_agent ON sessions(agent_id);
CREATE INDEX IF NOT EXISTS idx_sessions_channel ON sessions(channel_id);
CREATE INDEX IF NOT EXISTS idx_sessions_peer ON sessions(peer_id);
CREATE INDEX IF NOT EXISTS idx_sessions_state ON sessions(state);
CREATE INDEX IF NOT EXISTS idx_sessions_last_active ON sessions(last_active_at);
`

// NewSQLiteSessionStore creates a new SQLiteSessionStore.
func NewSQLiteSessionStore(dbPath string, maxTokens int) (*SQLiteSessionStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Connection pool limits
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)

	// Enable WAL mode and foreign keys
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
	}

	// Create schema
	if _, err := db.Exec(sessionSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	return &SQLiteSessionStore{
		db:        db,
		maxTokens: maxTokens,
	}, nil
}

// Save saves a session to the database.
func (s *SQLiteSessionStore) Save(session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Serialize metadata
	metadataJSON, err := json.Marshal(session.Metadata)
	if err != nil {
		return fmt.Errorf("failed to serialize metadata: %w", err)
	}

	// Serialize messages
	messages := session.GetMessages()
	msgData := messagesData{Messages: make([]messageData, 0, len(messages))}
	var systemPrompt string

	for _, msg := range messages {
		if msg.Role == context.RoleSystem {
			systemPrompt = msg.Content
			continue
		}
		md := messageData{
			Role:       string(msg.Role),
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}
		for _, tc := range msg.ToolCalls {
			md.ToolCalls = append(md.ToolCalls, toolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			})
		}
		msgData.Messages = append(msgData.Messages, md)
	}

	messagesJSON, err := json.Marshal(msgData)
	if err != nil {
		return fmt.Errorf("failed to serialize messages: %w", err)
	}

	// Upsert session
	query := `
		INSERT INTO sessions (
			id, agent_id, channel_id, peer_id, thread_id, state, metadata, messages, system_prompt,
			created_at, updated_at, last_active_at, compacted_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			state = excluded.state,
			metadata = excluded.metadata,
			messages = excluded.messages,
			system_prompt = excluded.system_prompt,
			updated_at = excluded.updated_at,
			last_active_at = excluded.last_active_at,
			compacted_at = excluded.compacted_at
	`

	var threadID sql.NullString
	if session.ID.ThreadID != "" {
		threadID = sql.NullString{String: session.ID.ThreadID, Valid: true}
	}

	var compactedAt sql.NullTime
	if session.CompactedAt != nil {
		compactedAt = sql.NullTime{Time: *session.CompactedAt, Valid: true}
	}

	var systemPromptNull sql.NullString
	if systemPrompt != "" {
		systemPromptNull = sql.NullString{String: systemPrompt, Valid: true}
	}

	_, err = s.db.Exec(query,
		session.ID.String(),
		session.ID.AgentID,
		session.ID.ChannelID,
		session.ID.PeerID,
		threadID,
		int(session.State),
		string(metadataJSON),
		string(messagesJSON),
		systemPromptNull,
		session.CreatedAt,
		session.UpdatedAt,
		session.LastActiveAt,
		compactedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	return nil
}

// Load loads a session from the database.
func (s *SQLiteSessionStore) Load(id SessionID) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
		SELECT id, agent_id, channel_id, peer_id, thread_id, state, metadata, messages, system_prompt,
			   created_at, updated_at, last_active_at, compacted_at
		FROM sessions WHERE id = ?
	`

	var row sessionRow
	err := s.db.QueryRow(query, id.String()).Scan(
		&row.ID,
		&row.AgentID,
		&row.ChannelID,
		&row.PeerID,
		&row.ThreadID,
		&row.State,
		&row.Metadata,
		&row.Messages,
		&row.SystemPrompt,
		&row.CreatedAt,
		&row.UpdatedAt,
		&row.LastActiveAt,
		&row.CompactedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	return s.rowToSession(row)
}

// Delete deletes a session from the database.
func (s *SQLiteSessionStore) Delete(id SessionID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("DELETE FROM sessions WHERE id = ?", id.String())
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

// List lists sessions matching the filter.
func (s *SQLiteSessionStore) List(filter SessionFilter) ([]*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
		SELECT id, agent_id, channel_id, peer_id, thread_id, state, metadata, messages, system_prompt,
			   created_at, updated_at, last_active_at, compacted_at
		FROM sessions WHERE 1=1
	`
	args := make([]interface{}, 0)

	if filter.AgentID != "" {
		query += " AND agent_id = ?"
		args = append(args, filter.AgentID)
	}
	if filter.ChannelID != "" {
		query += " AND channel_id = ?"
		args = append(args, filter.ChannelID)
	}
	if filter.PeerID != "" {
		query += " AND peer_id = ?"
		args = append(args, filter.PeerID)
	}
	if filter.State != nil {
		query += " AND state = ?"
		args = append(args, int(*filter.State))
	}
	if filter.CreatedAfter != nil {
		query += " AND created_at > ?"
		args = append(args, *filter.CreatedAfter)
	}
	if filter.CreatedBefore != nil {
		query += " AND created_at < ?"
		args = append(args, *filter.CreatedBefore)
	}

	query += " ORDER BY last_active_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	sessions := make([]*Session, 0)
	for rows.Next() {
		var row sessionRow
		err := rows.Scan(
			&row.ID,
			&row.AgentID,
			&row.ChannelID,
			&row.PeerID,
			&row.ThreadID,
			&row.State,
			&row.Metadata,
			&row.Messages,
			&row.SystemPrompt,
			&row.CreatedAt,
			&row.UpdatedAt,
			&row.LastActiveAt,
			&row.CompactedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		session, err := s.rowToSession(row)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// Archive archives a session.
func (s *SQLiteSessionStore) Archive(id SessionID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		"UPDATE sessions SET state = ?, updated_at = ? WHERE id = ?",
		int(SessionStateArchived),
		time.Now(),
		id.String(),
	)
	if err != nil {
		return fmt.Errorf("failed to archive session: %w", err)
	}
	return nil
}

// Close closes the database connection.
func (s *SQLiteSessionStore) Close() error {
	return s.db.Close()
}

// rowToSession converts a database row to a Session.
func (s *SQLiteSessionStore) rowToSession(row sessionRow) (*Session, error) {
	// Parse session ID
	id := SessionID{
		AgentID:   row.AgentID,
		ChannelID: row.ChannelID,
		PeerID:    row.PeerID,
	}
	if row.ThreadID.Valid {
		id.ThreadID = row.ThreadID.String
	}

	// Create session
	session := &Session{
		ID:           id,
		Context:      context.NewConversationContext(s.maxTokens),
		State:        SessionState(row.State),
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
		LastActiveAt: row.LastActiveAt,
	}

	if row.CompactedAt.Valid {
		session.CompactedAt = &row.CompactedAt.Time
	}

	// Parse metadata
	if row.Metadata != "" {
		if err := json.Unmarshal([]byte(row.Metadata), &session.Metadata); err != nil {
			return nil, fmt.Errorf("failed to parse metadata: %w", err)
		}
	}

	// Set system prompt
	if row.SystemPrompt.Valid && row.SystemPrompt.String != "" {
		session.Context.SetSystemPrompt(row.SystemPrompt.String)
	}

	// Parse and add messages
	if row.Messages != "" {
		var msgData messagesData
		if err := json.Unmarshal([]byte(row.Messages), &msgData); err != nil {
			return nil, fmt.Errorf("failed to parse messages: %w", err)
		}

		for _, md := range msgData.Messages {
			msg := context.Message{
				Role:       context.Role(md.Role),
				Content:    md.Content,
				ToolCallID: md.ToolCallID,
			}
			for _, tc := range md.ToolCalls {
				msg.ToolCalls = append(msg.ToolCalls, context.ToolCall{
					ID:        tc.ID,
					Name:      tc.Name,
					Arguments: tc.Arguments,
				})
			}
			session.Context.AddMessage(msg)
		}
	}

	return session, nil
}

// InMemorySessionStore implements SessionStore using in-memory storage.
type InMemorySessionStore struct {
	sessions  map[string]*Session
	mu        sync.RWMutex
	maxTokens int
}

// NewInMemorySessionStore creates a new InMemorySessionStore.
func NewInMemorySessionStore(maxTokens int) *InMemorySessionStore {
	return &InMemorySessionStore{
		sessions:  make(map[string]*Session),
		maxTokens: maxTokens,
	}
}

// Save saves a session.
func (s *InMemorySessionStore) Save(session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID.String()] = session
	return nil
}

// Load loads a session.
func (s *InMemorySessionStore) Load(id SessionID) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[id.String()]
	if !ok {
		return nil, nil
	}
	return session, nil
}

// Delete deletes a session.
func (s *InMemorySessionStore) Delete(id SessionID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id.String())
	return nil
}

// List lists sessions.
func (s *InMemorySessionStore) List(filter SessionFilter) ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Session, 0)
	for _, session := range s.sessions {
		if filter.AgentID != "" && session.ID.AgentID != filter.AgentID {
			continue
		}
		if filter.ChannelID != "" && session.ID.ChannelID != filter.ChannelID {
			continue
		}
		if filter.PeerID != "" && session.ID.PeerID != filter.PeerID {
			continue
		}
		if filter.State != nil && session.State != *filter.State {
			continue
		}
		result = append(result, session)
	}

	// Apply limit and offset
	if filter.Offset > 0 && filter.Offset < len(result) {
		result = result[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}

	return result, nil
}

// Archive archives a session.
func (s *InMemorySessionStore) Archive(id SessionID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session, ok := s.sessions[id.String()]; ok {
		session.SetState(SessionStateArchived)
	}
	return nil
}

// Close closes the store.
func (s *InMemorySessionStore) Close() error {
	return nil
}
