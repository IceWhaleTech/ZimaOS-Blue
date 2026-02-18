package session

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	z "github.com/IceWhaleTech/zorm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/context"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
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

// persistenceRow represents a session row for zorm scanning.
type persistenceRow struct {
	ID           string  `json:"id"`
	AgentID      string  `json:"agent_id"`
	ChannelID    string  `json:"channel_id"`
	PeerID       string  `json:"peer_id"`
	ThreadID     *string `json:"thread_id"`
	State        int     `json:"state"`
	Metadata     string  `json:"metadata"`
	Messages     string  `json:"messages"`
	SystemPrompt *string `json:"system_prompt"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
	LastActiveAt string  `json:"last_active_at"`
	CompactedAt  *string `json:"compacted_at"`
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

	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)

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
	if _, err := db.Exec(sessionSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	return &SQLiteSessionStore{db: db, maxTokens: maxTokens}, nil
}

func (s *SQLiteSessionStore) table() *z.ZormTable {
	return z.Table(s.db, "sessions")
}

// Save saves a session to the database.
func (s *SQLiteSessionStore) Save(session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	metadataJSON, err := json.Marshal(session.Metadata)
	if err != nil {
		return fmt.Errorf("failed to serialize metadata: %w", err)
	}

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
				ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments,
			})
		}
		msgData.Messages = append(msgData.Messages, md)
	}

	messagesJSON, err := json.Marshal(msgData)
	if err != nil {
		return fmt.Errorf("failed to serialize messages: %w", err)
	}

	var threadID *string
	if session.ID.ThreadID != "" {
		threadID = &session.ID.ThreadID
	}
	var compactedAt *time.Time
	if session.CompactedAt != nil {
		compactedAt = session.CompactedAt
	}
	var systemPromptPtr *string
	if systemPrompt != "" {
		systemPromptPtr = &systemPrompt
	}

	data := map[string]interface{}{
		"id":             session.ID.String(),
		"agent_id":       session.ID.AgentID,
		"channel_id":     session.ID.ChannelID,
		"peer_id":        session.ID.PeerID,
		"thread_id":      threadID,
		"state":          int(session.State),
		"metadata":       string(metadataJSON),
		"messages":       string(messagesJSON),
		"system_prompt":  systemPromptPtr,
		"created_at":     session.CreatedAt,
		"updated_at":     session.UpdatedAt,
		"last_active_at": session.LastActiveAt,
		"compacted_at":   compactedAt,
	}

	_, err = s.table().Insert(data,
		z.OnConflictDoUpdateSet(
			[]string{"id"},
			[]string{"state", "metadata", "messages", "system_prompt", "updated_at", "last_active_at", "compacted_at"},
		),
	)
	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}
	return nil
}

func parseSessionTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	if t.IsZero() {
		t, _ = time.Parse("2006-01-02 15:04:05", s)
	}
	if t.IsZero() {
		t, _ = time.Parse("2006-01-02T15:04:05Z", s)
	}
	return t
}

// Load loads a session from the database.
func (s *SQLiteSessionStore) Load(id SessionID) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var rows []persistenceRow
	_, err := s.table().Select(&rows,
		z.Where(z.Eq("id", id.String())),
		z.Limit(1),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}

	return s.rowToSession(rows[0])
}

// Delete deletes a session from the database.
func (s *SQLiteSessionStore) Delete(id SessionID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.table().Delete(z.Where(z.Eq("id", id.String())))
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

// List lists sessions matching the filter.
func (s *SQLiteSessionStore) List(filter SessionFilter) ([]*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	conds := make([]interface{}, 0)
	if filter.AgentID != "" {
		conds = append(conds, z.Eq("agent_id", filter.AgentID))
	}
	if filter.ChannelID != "" {
		conds = append(conds, z.Eq("channel_id", filter.ChannelID))
	}
	if filter.PeerID != "" {
		conds = append(conds, z.Eq("peer_id", filter.PeerID))
	}
	if filter.State != nil {
		conds = append(conds, z.Eq("state", int(*filter.State)))
	}
	if filter.CreatedAfter != nil {
		conds = append(conds, z.Gt("created_at", *filter.CreatedAfter))
	}
	if filter.CreatedBefore != nil {
		conds = append(conds, z.Lt("created_at", *filter.CreatedBefore))
	}

	opts := []z.ZormItem{z.OrderBy("last_active_at DESC")}
	if len(conds) > 0 {
		opts = append([]z.ZormItem{z.Where(conds...)}, opts...)
	}
	if filter.Limit > 0 {
		if filter.Offset > 0 {
			opts = append(opts, z.Limit(filter.Limit, filter.Offset))
		} else {
			opts = append(opts, z.Limit(filter.Limit))
		}
	} else if filter.Offset > 0 {
		opts = append(opts, z.Limit(-1, filter.Offset))
	}

	var rows []persistenceRow
	_, err := s.table().Select(&rows, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	sessions := make([]*Session, 0, len(rows))
	for _, row := range rows {
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

	_, err := s.table().Update(
		map[string]interface{}{
			"state":      int(SessionStateArchived),
			"updated_at": timeutil.NowTime(),
		},
		z.Where(z.Eq("id", id.String())),
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

// rowToSession converts a persistenceRow to a Session.
func (s *SQLiteSessionStore) rowToSession(row persistenceRow) (*Session, error) {
	id := SessionID{
		AgentID:   row.AgentID,
		ChannelID: row.ChannelID,
		PeerID:    row.PeerID,
	}
	if row.ThreadID != nil {
		id.ThreadID = *row.ThreadID
	}

	session := &Session{
		ID:           id,
		Context:      context.NewConversationContext(s.maxTokens),
		State:        SessionState(row.State),
		CreatedAt:    parseSessionTime(row.CreatedAt),
		UpdatedAt:    parseSessionTime(row.UpdatedAt),
		LastActiveAt: parseSessionTime(row.LastActiveAt),
	}

	if row.CompactedAt != nil {
		t := parseSessionTime(*row.CompactedAt)
		if !t.IsZero() {
			session.CompactedAt = &t
		}
	}

	if row.Metadata != "" {
		if err := json.Unmarshal([]byte(row.Metadata), &session.Metadata); err != nil {
			return nil, fmt.Errorf("failed to parse metadata: %w", err)
		}
	}

	if row.SystemPrompt != nil && *row.SystemPrompt != "" {
		session.Context.SetSystemPrompt(*row.SystemPrompt)
	}

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
					ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments,
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

func (s *InMemorySessionStore) Save(session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID.String()] = session
	return nil
}

func (s *InMemorySessionStore) Load(id SessionID) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[id.String()]
	if !ok {
		return nil, nil
	}
	return session, nil
}

func (s *InMemorySessionStore) Delete(id SessionID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id.String())
	return nil
}

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

	if filter.Offset > 0 && filter.Offset < len(result) {
		result = result[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}

	return result, nil
}

func (s *InMemorySessionStore) Archive(id SessionID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session, ok := s.sessions[id.String()]; ok {
		session.SetState(SessionStateArchived)
	}
	return nil
}

func (s *InMemorySessionStore) Close() error {
	return nil
}
