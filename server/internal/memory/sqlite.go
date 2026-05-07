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
	z "github.com/IceWhaleTech/zorm"
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
	ID                 string    `json:"id"`
	Title              string    `json:"title"`
	UserID             string    `json:"user_id,omitempty"`
	Pinned             bool      `json:"pinned"`
	AutoTitleFinalized bool      `json:"-"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
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
	TurnID         string              `json:"turn_id,omitempty"`
	CreatedAt      time.Time           `json:"created_at"`
}

// ConversationCommandState stores persisted per-conversation deterministic chat command state.
type ConversationCommandState struct {
	ConversationID            string    `json:"conversation_id"`
	SelectedProviderID        string    `json:"selected_provider_id,omitempty"`
	SelectedModelID           string    `json:"selected_model_id,omitempty"`
	LastGoodProviderID        string    `json:"last_good_provider_id,omitempty"`
	LastGoodModelID           string    `json:"last_good_model_id,omitempty"`
	LastGoodNativeSurfaceMode string    `json:"last_good_native_surface_mode,omitempty"`
	AgentcoreRunnerRef        string    `json:"agentcore_runner_ref,omitempty"`
	Offline                   bool      `json:"offline"`
	WebSearchEnabled          bool      `json:"web_search_enabled"`
	DeepResearchEnabled       bool      `json:"deep_research_enabled"`
	UpdatedAt                 time.Time `json:"updated_at"`
}

// Store provides conversation storage using SQLite.
type Store struct {
	db          *sql.DB
	readDB      *sql.DB
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
	opts = normalizeStoreOptions(dbPath, opts)
	readDB, readErr := openStoreReaderDB(dbPath)
	if readErr != nil || readDB == nil {
		readDB = db
	}

	store := &Store{
		db:      db,
		readDB:  readDB,
		ownsDB:  true,
		dbPath:  dbPath,
		options: opts,
		bgDone:  make(chan struct{}),
	}
	store.startOwnedLoops()
	return store, nil
}

// NewStoreWithDB creates a memory store using an existing shared database connection.
// The caller is responsible for managing the DB lifecycle (pragmas, connection pool, close).
func NewStoreWithDB(db *sql.DB) (*Store, error) {
	return NewStoreWithReadDB(db, db)
}

// NewStoreWithReadDB creates a memory store using separate shared write and read
// database connections. The caller is responsible for managing the DB lifecycle.
func NewStoreWithReadDB(writeDB, readDB *sql.DB) (*Store, error) {
	if writeDB == nil {
		return nil, fmt.Errorf("memory db is required")
	}
	if readDB == nil {
		readDB = writeDB
	}
	store := &Store{db: writeDB, readDB: readDB, ownsDB: false, options: DefaultStoreOptions()}
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
		auto_title_finalized BOOLEAN NOT NULL DEFAULT 0,
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
		turn_id TEXT,
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
		last_good_provider_id TEXT NOT NULL DEFAULT '',
		last_good_model_id TEXT NOT NULL DEFAULT '',
		last_good_native_surface_mode TEXT NOT NULL DEFAULT '',
		agentcore_runner_ref TEXT NOT NULL DEFAULT '',
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
		"ALTER TABLE conversations ADD COLUMN auto_title_finalized BOOLEAN NOT NULL DEFAULT 0",
		"ALTER TABLE conversation_command_state ADD COLUMN last_good_provider_id TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE conversation_command_state ADD COLUMN last_good_model_id TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE conversation_command_state ADD COLUMN last_good_native_surface_mode TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE conversation_command_state ADD COLUMN agentcore_runner_ref TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE messages ADD COLUMN turn_id TEXT",
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
		var firstErr error
		if s.db != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_ = s.checkpoint(ctx, dbutil.CheckpointTruncate)
			cancel()
		}
		if s.readDB != nil && s.readDB != s.db {
			if err := s.readDB.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		if s.db != nil {
			if err := s.db.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	}
	return nil
}

func normalizeConversationScope(userID []string) string {
	if len(userID) == 0 {
		return ""
	}
	return strings.TrimSpace(userID[0])
}

func conversationVisibilityCond(scopedUserID string) interface{} {
	scopedUserID = strings.TrimSpace(scopedUserID)
	if scopedUserID == "" {
		return nil
	}
	return z.Or(
		z.Eq("user_id", scopedUserID),
		z.And(
			z.Eq("user_id", ""),
			z.Like("id", "ch:%"),
		),
	)
}

func (s *Store) ensureConversationAccess(ctx context.Context, conversationID, scopedUserID string) error {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" || scopedUserID == "" {
		return nil
	}

	conds := []interface{}{z.Eq("id", conversationID)}
	if visibility := conversationVisibilityCond(scopedUserID); visibility != nil {
		conds = append(conds, visibility)
	}

	var count int64
	_, err := s.conversationsRead(ctx).Select(&count,
		z.Fields("count(1)"),
		z.Where(conds...),
	)
	if err != nil {
		return fmt.Errorf("failed to verify conversation ownership: %w", err)
	}
	if count == 0 {
		return ErrNotFound
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

	_, err := s.conversations(ctx).Insert(conversationValues(conv))
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

	_, err := s.conversations(ctx).Insert(conversationValues(conv))
	if err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	return conv, nil
}

// GetConversation retrieves a conversation by ID.
func (s *Store) GetConversation(ctx context.Context, id string, userID ...string) (*Conversation, error) {
	scopedUserID := normalizeConversationScope(userID)
	conds := []interface{}{z.Eq("id", id)}
	if visibility := conversationVisibilityCond(scopedUserID); visibility != nil {
		conds = append(conds, visibility)
	}

	var rows []conversationRow
	_, err := s.conversationsRead(ctx).Select(&rows,
		z.Where(conds...),
		z.Limit(1),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}
	if len(rows) == 0 {
		return nil, ErrNotFound
	}

	return rowToConversation(rows[0]), nil
}

// ListConversations lists conversations with pagination, optionally filtered by userID.
func (s *Store) ListConversations(ctx context.Context, limit, offset int, userID ...string) ([]Conversation, error) {
	opts := []z.ZormItem{
		z.OrderBy("pinned DESC", "updated_at DESC", "created_at DESC", "rowid DESC"),
		z.Limit(limit, offset),
	}
	if visibility := conversationVisibilityCond(normalizeConversationScope(userID)); visibility != nil {
		opts = append([]z.ZormItem{
			z.Where(visibility),
		}, opts...)
	}

	var rows []conversationRow
	_, err := s.conversationsRead(ctx).Select(&rows, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to list conversations: %w", err)
	}

	convs := make([]Conversation, 0, len(rows))
	for i := range rows {
		convs = append(convs, *rowToConversation(rows[i]))
	}
	return convs, nil
}

// DeleteConversation deletes a conversation and its messages.
func (s *Store) DeleteConversation(ctx context.Context, id string, userID ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	scopedUserID := normalizeConversationScope(userID)
	conds := []interface{}{z.Eq("id", id)}
	// Messages are deleted via CASCADE
	if scopedUserID != "" {
		conds = append(conds, z.Eq("user_id", scopedUserID))
	}
	affected, err := s.conversations(ctx).Delete(z.Where(conds...))
	if err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}
	if scopedUserID != "" && affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) updateConversationTitle(
	ctx context.Context,
	id,
	title string,
	finalizeAutoTitle bool,
	onlyIfAutoTitlePending bool,
	userID ...string,
) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	scopedUserID := normalizeConversationScope(userID)
	conds := []interface{}{z.Eq("id", id)}
	if scopedUserID != "" {
		conds = append(conds, z.Eq("user_id", scopedUserID))
	}
	if onlyIfAutoTitlePending {
		conds = append(conds, z.Eq("auto_title_finalized", false))
	}

	values := z.V{
		"title":      title,
		"updated_at": formatStoreTime(timeutil.NowTime()),
	}
	fields := []string{"title", "updated_at"}
	if finalizeAutoTitle {
		values["auto_title_finalized"] = true
		fields = append(fields, "auto_title_finalized")
	}

	affected, err := s.conversations(ctx).Update(
		values,
		z.Fields(fields...),
		z.Where(conds...),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to update conversation title: %w", err)
	}
	if scopedUserID != "" && affected == 0 {
		return 0, ErrNotFound
	}
	return affected, nil
}

// UpdateConversationTitle updates a conversation title and prevents future auto-generated title writes.
func (s *Store) UpdateConversationTitle(ctx context.Context, id, title string, userID ...string) error {
	_, err := s.updateConversationTitle(ctx, id, title, true, false, userID...)
	return err
}

// FinalizeAutoConversationTitle updates a title only while auto titling is still pending.
// It atomically locks the conversation so automatic title generation can only succeed once.
func (s *Store) FinalizeAutoConversationTitle(ctx context.Context, id, title string, userID ...string) (bool, error) {
	affected, err := s.updateConversationTitle(ctx, id, title, true, true, userID...)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return affected > 0, nil
}

// PreviewAutoConversationTitle updates a pending auto title without finalizing it.
// It only succeeds while automatic title generation is still pending.
func (s *Store) PreviewAutoConversationTitle(ctx context.Context, id, title string, userID ...string) (bool, error) {
	affected, err := s.updateConversationTitle(ctx, id, title, false, true, userID...)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return affected > 0, nil
}

// PinConversation pins a conversation.
func (s *Store) PinConversation(ctx context.Context, id string, userID ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	scopedUserID := normalizeConversationScope(userID)
	conds := []interface{}{z.Eq("id", id)}
	if scopedUserID != "" {
		conds = append(conds, z.Eq("user_id", scopedUserID))
	}
	affected, err := s.conversations(ctx).Update(
		z.V{"pinned": true, "updated_at": formatStoreTime(timeutil.NowTime())},
		z.Fields("pinned", "updated_at"),
		z.Where(conds...),
	)
	if err != nil {
		return fmt.Errorf("failed to pin conversation: %w", err)
	}
	if scopedUserID != "" && affected == 0 {
		return ErrNotFound
	}
	return nil
}

// UnpinConversation unpins a conversation.
func (s *Store) UnpinConversation(ctx context.Context, id string, userID ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	scopedUserID := normalizeConversationScope(userID)
	conds := []interface{}{z.Eq("id", id)}
	if scopedUserID != "" {
		conds = append(conds, z.Eq("user_id", scopedUserID))
	}
	affected, err := s.conversations(ctx).Update(
		z.V{"pinned": false, "updated_at": formatStoreTime(timeutil.NowTime())},
		z.Fields("pinned", "updated_at"),
		z.Where(conds...),
	)
	if err != nil {
		return fmt.Errorf("failed to unpin conversation: %w", err)
	}
	if scopedUserID != "" && affected == 0 {
		return ErrNotFound
	}
	return nil
}

// SearchConversations searches conversations by title, optionally filtered by userID.
func (s *Store) SearchConversations(ctx context.Context, query string, limit int, userID ...string) ([]Conversation, error) {
	conds := []interface{}{z.Like("title", "%"+query+"%")}
	if visibility := conversationVisibilityCond(normalizeConversationScope(userID)); visibility != nil {
		conds = append(conds, visibility)
	}

	var rows []conversationRow
	_, err := s.conversationsRead(ctx).Select(&rows,
		z.Where(conds...),
		z.OrderBy("pinned DESC", "updated_at DESC", "created_at DESC", "rowid DESC"),
		z.Limit(limit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search conversations: %w", err)
	}

	convs := make([]Conversation, 0, len(rows))
	for i := range rows {
		convs = append(convs, *rowToConversation(rows[i]))
	}
	return convs, nil
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

		messagesTable := z.TableContext(ctx, tx, "messages")
		if _, err := messagesTable.Insert(
			messageValues(persisted, toolCallsJSON, statsJSON, attachmentsJSON, hasAttachments),
		); err != nil {
			return fmt.Errorf("failed to add message: %w", err)
		}

		if hasAttachments && s.shouldExternalizeAttachments() {
			if err := s.persistExternalAttachmentsTx(ctx, tx, persisted.ID, persisted.Attachments, persisted.CreatedAt); err != nil {
				return err
			}
		}

		if _, err := z.TableContext(ctx, tx, "conversations").Update(
			z.V{"updated_at": formatStoreTime(timeutil.NowTime())},
			z.Fields("updated_at"),
			z.Where(z.Eq("id", conversationID)),
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

		values := z.V{"content": content}
		fields := []string{"content"}
		if stats != nil {
			statsJSON, err := json.Marshal(stats)
			if err != nil {
				return fmt.Errorf("failed to marshal stats: %w", err)
			}
			values["stats"] = string(statsJSON)
			fields = append(fields, "stats")
		}
		if provider != "" {
			values["provider"] = provider
			fields = append(fields, "provider")
		}
		if model != "" {
			values["model"] = model
			fields = append(fields, "model")
		}
		_, err := s.messages(ctx).Update(
			values,
			z.Fields(fields...),
			z.Where(z.Eq("id", messageID)),
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

		messagesTable := z.TableContext(ctx, tx, "messages")
		values := z.V{"content": msg.Content}
		fields := []string{"content"}
		if len(statsJSON) > 0 {
			values["stats"] = string(statsJSON)
			fields = append(fields, "stats")
		}
		if msg.Provider != "" {
			values["provider"] = msg.Provider
			fields = append(fields, "provider")
		}
		if msg.Model != "" {
			values["model"] = msg.Model
			fields = append(fields, "model")
		}

		rowsAffected, err := messagesTable.Update(
			values,
			z.Fields(fields...),
			z.Where(z.Eq("id", msg.ID)),
		)
		if err != nil {
			return fmt.Errorf("update message content: %w", err)
		}
		if rowsAffected == 0 {
			var toolCallsJSON []byte
			if len(msg.ToolCalls) > 0 {
				toolCallsJSON, _ = json.Marshal(msg.ToolCalls)
			}
			if _, err := messagesTable.Insert(messageValues(msg, toolCallsJSON, statsJSON, nil, false)); err != nil {
				return fmt.Errorf("insert message content: %w", err)
			}
		}

		if _, err := z.TableContext(ctx, tx, "conversations").Update(
			z.V{"updated_at": formatStoreTime(timeutil.NowTime())},
			z.Fields("updated_at"),
			z.Where(z.Eq("id", msg.ConversationID)),
		); err != nil {
			return fmt.Errorf("update conversation timestamp: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit upsert message tx: %w", err)
		}
		return nil
	})
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

	var count int64
	if _, err := s.messagesRead(ctx).Select(&count,
		z.Fields("count(1)"),
		z.Where(z.Eq("conversation_id", conversationID)),
	); err != nil {
		return 0, fmt.Errorf("failed to count messages: %w", err)
	}
	return int(count), nil
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

	var rows []messageRow
	_, err := s.messagesRead(ctx).Select(&rows,
		z.Fields(
			"id", "conversation_id", "role", "content", "tool_calls", "tool_call_id",
			"tool_name", "provider", "model", "stats", "attachments", "has_attachments", "created_at",
		),
		z.Where(z.Eq("conversation_id", conversationID), z.Eq("role", "assistant")),
		z.OrderBy("created_at DESC", "rowid DESC"),
		z.Limit(1),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan latest assistant message: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}

	scanned, err := rowToScannedMessage(rows[0])
	if err != nil {
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

func normalizeConversationCommandStateToolDefaults(state ConversationCommandState) ConversationCommandState {
	state.WebSearchEnabled = true
	state.DeepResearchEnabled = true
	return state
}

func defaultConversationCommandState(conversationID string) ConversationCommandState {
	return normalizeConversationCommandStateToolDefaults(ConversationCommandState{
		ConversationID: strings.TrimSpace(conversationID),
	})
}

func (s *Store) getConversationCommandStateScope(ctx context.Context, conversationID string) (string, error) {
	var rows []conversationScopeRow
	_, err := s.conversationsRead(ctx).Select(&rows,
		z.Fields("user_id"),
		z.Where(z.Eq("id", conversationID)),
		z.Limit(1),
	)
	if err != nil {
		return "", fmt.Errorf("failed to resolve conversation command state scope: %w", err)
	}
	if len(rows) == 0 {
		return "", ErrNotFound
	}
	if rows[0].UserID == nil {
		return "", nil
	}
	return strings.TrimSpace(*rows[0].UserID), nil
}

func (s *Store) getPersistedConversationCommandState(ctx context.Context, conversationID string) (ConversationCommandState, bool, error) {
	state := defaultConversationCommandState(conversationID)

	var rows []conversationCommandStateRow
	_, err := s.conversationCommandStateRead(ctx).Select(&rows,
		z.Fields("selected_provider_id", "selected_model_id", "last_good_provider_id", "last_good_model_id", "last_good_native_surface_mode", "agentcore_runner_ref", "offline", "web_search_enabled", "deep_research_enabled", "updated_at"),
		z.Where(z.Eq("conversation_id", conversationID)),
		z.Limit(1),
	)
	if err != nil {
		return state, false, fmt.Errorf("failed to get conversation command state: %w", err)
	}
	if len(rows) == 0 {
		return state, false, nil
	}

	state = rowToConversationCommandState(conversationID, rows[0])
	return state, true, nil
}

func (s *Store) getPersistedUserCommandState(ctx context.Context, userID string) (ConversationCommandState, bool, error) {
	state := defaultConversationCommandState("")

	var rows []z.V
	_, err := s.userCommandStateRead(ctx).Select(&rows,
		z.Fields("selected_provider_id", "selected_model_id", "offline", "web_search_enabled", "deep_research_enabled", "updated_at"),
		z.Where(z.Eq("user_id", userID)),
		z.Limit(1),
	)
	if err != nil {
		return state, false, fmt.Errorf("failed to get user command state: %w", err)
	}
	if len(rows) == 0 {
		return state, false, nil
	}

	state.SelectedProviderID = lookupZormString(rows[0], "selected_provider_id")
	state.SelectedModelID = lookupZormString(rows[0], "selected_model_id")
	state.Offline = lookupZormBool(rows[0], "offline")
	state.WebSearchEnabled = lookupZormBool(rows[0], "web_search_enabled")
	state.DeepResearchEnabled = lookupZormBool(rows[0], "deep_research_enabled")
	state.UpdatedAt = parseStoreTime(lookupZormString(rows[0], "updated_at"))
	state = normalizeConversationCommandStateToolDefaults(state)
	return state, true, nil
}

func (s *Store) getLatestLegacyConversationCommandStateForUser(ctx context.Context, userID string) (ConversationCommandState, bool, error) {
	state := defaultConversationCommandState("")

	var rows []z.V
	joined := z.TableContext(ctx, s.reader(), "conversation_command_state")
	_, err := joined.Select(&rows,
		z.Fields(
			"conversation_command_state.selected_provider_id",
			"conversation_command_state.selected_model_id",
			"conversation_command_state.offline",
			"conversation_command_state.web_search_enabled",
			"conversation_command_state.deep_research_enabled",
			"conversation_command_state.updated_at",
		),
		z.InnerJoin("conversations", "conversations.id = conversation_command_state.conversation_id"),
		z.Where(z.Eq("conversations.user_id", userID)),
		z.OrderBy("conversation_command_state.updated_at DESC", "conversation_command_state.conversation_id DESC"),
		z.Limit(1),
	)
	if err != nil {
		return state, false, fmt.Errorf("failed to get legacy conversation command state for user: %w", err)
	}
	if len(rows) == 0 {
		return state, false, nil
	}

	state.SelectedProviderID = lookupZormString(rows[0], "selected_provider_id", "conversation_command_state.selected_provider_id")
	state.SelectedModelID = lookupZormString(rows[0], "selected_model_id", "conversation_command_state.selected_model_id")
	state.Offline = lookupZormBool(rows[0], "offline", "conversation_command_state.offline")
	state.WebSearchEnabled = lookupZormBool(rows[0], "web_search_enabled", "conversation_command_state.web_search_enabled")
	state.DeepResearchEnabled = lookupZormBool(rows[0], "deep_research_enabled", "conversation_command_state.deep_research_enabled")
	state.UpdatedAt = parseStoreTime(lookupZormString(rows[0], "updated_at", "conversation_command_state.updated_at"))
	return normalizeConversationCommandStateToolDefaults(state), true, nil
}

// GetConversationCommandState returns persisted deterministic command state for a conversation.
// For authenticated conversations (with user_id), provider/model/offline stay user-scoped while
// conversation-specific overrides such as agentcore runner ref remain conversation-scoped.
// Missing rows fall back to defaults: provider/model auto, offline=false, web=true, deep=true.
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
		merged := state
		userState, ok, err := s.getPersistedUserCommandState(ctx, userID)
		if err != nil {
			return state, err
		}
		if ok {
			userState.ConversationID = conversationID
			merged = userState
		} else {
			// Compatibility fallback for legacy per-conversation rows before user-scoped state was introduced.
			legacy, ok, err := s.getLatestLegacyConversationCommandStateForUser(ctx, userID)
			if err != nil {
				return state, err
			}
			if ok {
				legacy.ConversationID = conversationID
				merged = legacy
			}
		}

		convState, ok, err := s.getPersistedConversationCommandState(ctx, conversationID)
		if err != nil {
			return state, err
		}
		if ok {
			merged.LastGoodProviderID = strings.TrimSpace(convState.LastGoodProviderID)
			merged.LastGoodModelID = strings.TrimSpace(convState.LastGoodModelID)
			merged.LastGoodNativeSurfaceMode = strings.TrimSpace(convState.LastGoodNativeSurfaceMode)
			merged.AgentcoreRunnerRef = strings.TrimSpace(convState.AgentcoreRunnerRef)
		}
		return merged, nil
	}

	convState, _, err := s.getPersistedConversationCommandState(ctx, conversationID)
	if err != nil {
		return state, err
	}
	return convState, nil
}

// UpsertConversationCommandState stores deterministic command state for a conversation.
// For authenticated conversations (with user_id), provider/model/offline are persisted once per
// user while conversation-specific overrides are kept on the conversation row.
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
	state.LastGoodProviderID = strings.TrimSpace(state.LastGoodProviderID)
	state.LastGoodModelID = strings.TrimSpace(state.LastGoodModelID)
	state.LastGoodNativeSurfaceMode = strings.TrimSpace(state.LastGoodNativeSurfaceMode)
	state.AgentcoreRunnerRef = strings.TrimSpace(state.AgentcoreRunnerRef)
	state = normalizeConversationCommandStateToolDefaults(state)
	state.UpdatedAt = timeutil.NowTime()

	s.mu.Lock()
	defer s.mu.Unlock()

	if userID != "" {
		_, err = s.userCommandState(ctx).Insert(
			userCommandStateValues(userID, state),
			z.OnConflictDoUpdateSet(
				[]string{"user_id"},
				[]string{"selected_provider_id", "selected_model_id", "offline", "web_search_enabled", "deep_research_enabled", "updated_at"},
			),
		)
		if err != nil {
			return fmt.Errorf("failed to upsert user command state: %w", err)
		}

		conversationScopedState := defaultConversationCommandState(conversationID)
		conversationScopedState.LastGoodProviderID = state.LastGoodProviderID
		conversationScopedState.LastGoodModelID = state.LastGoodModelID
		conversationScopedState.LastGoodNativeSurfaceMode = state.LastGoodNativeSurfaceMode
		conversationScopedState.AgentcoreRunnerRef = state.AgentcoreRunnerRef
		conversationScopedState.UpdatedAt = state.UpdatedAt
		_, err = s.conversationCommandState(ctx).Insert(
			conversationCommandStateValues(conversationID, conversationScopedState),
			z.OnConflictDoUpdateSet(
				[]string{"conversation_id"},
				[]string{"selected_provider_id", "selected_model_id", "last_good_provider_id", "last_good_model_id", "last_good_native_surface_mode", "agentcore_runner_ref", "offline", "web_search_enabled", "deep_research_enabled", "updated_at"},
			),
		)
		if err != nil {
			return fmt.Errorf("failed to upsert conversation-scoped command state: %w", err)
		}
		return nil
	}

	_, err = s.conversationCommandState(ctx).Insert(
		conversationCommandStateValues(state.ConversationID, state),
		z.OnConflictDoUpdateSet(
			[]string{"conversation_id"},
			[]string{"selected_provider_id", "selected_model_id", "last_good_provider_id", "last_good_model_id", "last_good_native_surface_mode", "agentcore_runner_ref", "offline", "web_search_enabled", "deep_research_enabled", "updated_at"},
		),
	)
	if err != nil {
		return fmt.Errorf("failed to upsert conversation command state: %w", err)
	}
	return nil
}

// ClearConversationCommandState removes persisted deterministic command state for a conversation.
// For authenticated conversations (with user_id), this clears both the shared user-scoped state
// and the conversation-specific override row.
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
		if _, err := s.userCommandState(ctx).Delete(z.Where(z.Eq("user_id", userID))); err != nil {
			return fmt.Errorf("failed to clear user command state: %w", err)
		}
		if _, err := s.conversationCommandState(ctx).Delete(z.Where(z.Eq("conversation_id", conversationID))); err != nil {
			return fmt.Errorf("failed to clear conversation-scoped command state: %w", err)
		}
		return nil
	}

	if _, err := s.conversationCommandState(ctx).Delete(z.Where(z.Eq("conversation_id", conversationID))); err != nil {
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

	_, err := s.messages(ctx).Delete(
		z.Where(
			z.Eq("conversation_id", conversationID),
			z.In("id", messageIDs),
		),
	)
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
	var rows []previousResponseIDRow
	_, err := s.conversationRuntimeStateRead(ctx).Select(&rows,
		z.Fields("previous_response_id"),
		z.Where(
			z.Eq("conversation_id", conversationID),
			z.Eq("provider_id", ""),
			z.Eq("model_id", ""),
			z.Gte("updated_at", cutoff),
		),
		z.Limit(1),
	)
	if err != nil {
		return "", fmt.Errorf("failed to get conversation previous response id: %w", err)
	}
	if len(rows) == 0 {
		return "", nil
	}
	return strings.TrimSpace(rows[0].PreviousResponseID), nil
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

		_, err := s.conversationRuntimeState(ctx).Insert(
			conversationRuntimeStateValues(conversationID, responseID, now),
			z.OnConflictDoUpdateSet(
				[]string{"conversation_id", "provider_id", "model_id"},
				[]string{"previous_response_id", "updated_at"},
			),
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

		if _, err := s.conversationRuntimeState(ctx).Delete(z.Where(z.Eq("conversation_id", conversationID))); err != nil {
			return fmt.Errorf("failed to clear conversation previous response id: %w", err)
		}
		return nil
	})
}
