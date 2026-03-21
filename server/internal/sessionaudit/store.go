package sessionaudit

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// StoreConfig controls tool payload audit storage and retention.
type StoreConfig struct {
	RetentionDays    int
	CleanupInterval  time.Duration
	CleanupBatchSize int
}

// DefaultStoreConfig returns the default tool payload audit config.
func DefaultStoreConfig() StoreConfig {
	return StoreConfig{
		RetentionDays:    30,
		CleanupInterval:  6 * time.Hour,
		CleanupBatchSize: 500,
	}
}

// Entry is one persisted audit event for chat tool execution.
type Entry struct {
	ID             string                 `json:"id"`
	ConversationID string                 `json:"conversation_id"`
	SessionID      string                 `json:"session_id,omitempty"`
	UserID         string                 `json:"user_id,omitempty"`
	Source         string                 `json:"source,omitempty"`
	EventType      string                 `json:"event_type"` // assistant_tool_call | tool_result
	Role           string                 `json:"role"`       // assistant | tool
	ToolCallID     string                 `json:"tool_call_id,omitempty"`
	ToolName       string                 `json:"tool_name,omitempty"`
	Payload        string                 `json:"payload"`
	PayloadSHA256  string                 `json:"payload_sha256"`
	PayloadBytes   int                    `json:"payload_bytes"`
	IsError        bool                   `json:"is_error"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
}

// Store persists tool payload audit events in a dedicated SQLite database.
type Store struct {
	db       *sql.DB
	cfg      StoreConfig
	done     chan struct{}
	ownsDB   bool
	closeMu  sync.Mutex
	isClosed bool
}

const storeSchema = `
CREATE TABLE IF NOT EXISTS session_tool_audit_logs (
	id              TEXT PRIMARY KEY,
	conversation_id TEXT NOT NULL,
	session_id      TEXT NOT NULL DEFAULT '',
	user_id         TEXT NOT NULL DEFAULT '',
	source          TEXT NOT NULL DEFAULT '',
	event_type      TEXT NOT NULL,
	role            TEXT NOT NULL,
	tool_call_id    TEXT NOT NULL DEFAULT '',
	tool_name       TEXT NOT NULL DEFAULT '',
	payload         TEXT NOT NULL,
	payload_sha256  TEXT NOT NULL,
	payload_bytes   INTEGER NOT NULL DEFAULT 0,
	is_error        INTEGER NOT NULL DEFAULT 0,
	metadata        TEXT NOT NULL DEFAULT '{}',
	created_at      TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_session_tool_audit_conv_created
	ON session_tool_audit_logs(conversation_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_session_tool_audit_created
	ON session_tool_audit_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_session_tool_audit_tool_call
	ON session_tool_audit_logs(tool_call_id);
`

const DefaultDBFilename = "session_audit.db"

// ResolveDBPath returns the effective audit DB path.
// Empty paths default to a dedicated session_audit.db under dataDir so tool
// payload logging does not contend with the main blue.db conversation store.
func ResolveDBPath(dataDir, configuredPath string) string {
	configuredPath = strings.TrimSpace(configuredPath)
	if configuredPath == "" {
		configuredPath = DefaultDBFilename
	}
	if filepath.IsAbs(configuredPath) || strings.TrimSpace(dataDir) == "" {
		return configuredPath
	}
	// Keep relative audit DB paths under dataDir for predictable deployment paths.
	return filepath.Join(dataDir, filepath.Base(configuredPath))
}

func normalizeStoreConfig(cfg StoreConfig) StoreConfig {
	if cfg.CleanupBatchSize <= 0 {
		cfg.CleanupBatchSize = DefaultStoreConfig().CleanupBatchSize
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = DefaultStoreConfig().CleanupInterval
	}
	return cfg
}

func newStoreWithDB(db *sql.DB, cfg StoreConfig, ownsDB bool) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("session audit db is nil")
	}
	cfg = normalizeStoreConfig(cfg)
	if _, err := db.Exec(storeSchema); err != nil {
		return nil, fmt.Errorf("create session audit schema: %w", err)
	}

	s := &Store{
		db:     db,
		cfg:    cfg,
		done:   make(chan struct{}),
		ownsDB: ownsDB,
	}

	if s.cfg.RetentionDays > 0 {
		_ = s.PruneExpired(context.Background())
		go s.cleanupLoop()
	}
	return s, nil
}

// NewSQLiteStore opens/creates an isolated SQLite database for tool payload audit logs.
func NewSQLiteStore(dbPath string, cfg StoreConfig) (*Store, error) {
	if strings.TrimSpace(dbPath) == "" {
		return nil, fmt.Errorf("session audit db path is empty")
	}

	db, err := dbutil.OpenSQLiteWithRecoveryAndRecreate(dbPath, dbPath, func(db *sql.DB) error {
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)

		if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
			return fmt.Errorf("enable WAL mode: %w", err)
		}
		if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
			return fmt.Errorf("set busy timeout: %w", err)
		}
		if _, err := db.Exec("PRAGMA synchronous=FULL"); err != nil {
			return fmt.Errorf("set synchronous mode: %w", err)
		}
		if _, err := db.Exec("PRAGMA wal_autocheckpoint=1000"); err != nil {
			return fmt.Errorf("set wal autocheckpoint: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("open session audit db: %w", err)
	}

	s, err := newStoreWithDB(db, cfg, true)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// NewSQLiteStoreWithDB reuses an existing SQLite connection for audit storage.
func NewSQLiteStoreWithDB(db *sql.DB, cfg StoreConfig) (*Store, error) {
	return newStoreWithDB(db, cfg, false)
}

// Record appends one audit entry.
func (s *Store) Record(ctx context.Context, entry Entry) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("session audit store is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if strings.TrimSpace(entry.ID) == "" {
		entry.ID = uuid.NewString()
	}
	entry.ConversationID = strings.TrimSpace(entry.ConversationID)
	if entry.ConversationID == "" {
		return fmt.Errorf("conversation_id is required")
	}
	entry.SessionID = strings.TrimSpace(entry.SessionID)
	entry.UserID = strings.TrimSpace(entry.UserID)
	entry.Source = strings.TrimSpace(entry.Source)
	entry.EventType = strings.TrimSpace(entry.EventType)
	entry.Role = strings.TrimSpace(entry.Role)
	entry.ToolCallID = strings.TrimSpace(entry.ToolCallID)
	entry.ToolName = strings.TrimSpace(entry.ToolName)
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = timeutil.NowTime()
	}
	entry.PayloadBytes = len(entry.Payload)
	if entry.PayloadSHA256 == "" {
		sum := sha256.Sum256([]byte(entry.Payload))
		entry.PayloadSHA256 = hex.EncodeToString(sum[:])
	}

	metadataJSON := "{}"
	if len(entry.Metadata) > 0 {
		if b, err := json.Marshal(entry.Metadata); err == nil {
			metadataJSON = string(b)
		}
	}

	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO session_tool_audit_logs (
			id, conversation_id, session_id, user_id, source, event_type, role,
			tool_call_id, tool_name, payload, payload_sha256, payload_bytes,
			is_error, metadata, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ID,
		entry.ConversationID,
		entry.SessionID,
		entry.UserID,
		entry.Source,
		entry.EventType,
		entry.Role,
		entry.ToolCallID,
		entry.ToolName,
		entry.Payload,
		entry.PayloadSHA256,
		entry.PayloadBytes,
		boolToInt(entry.IsError),
		metadataJSON,
		entry.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("insert session audit entry: %w", err)
	}
	return nil
}

// Recent returns recent entries, optionally scoped to one conversation.
func (s *Store) Recent(ctx context.Context, conversationID string, limit int) ([]Entry, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("session audit store is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 {
		limit = 50
	}

	conversationID = strings.TrimSpace(conversationID)
	var (
		rows *sql.Rows
		err  error
	)
	if conversationID != "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, conversation_id, session_id, user_id, source, event_type, role, tool_call_id, tool_name,
			        payload, payload_sha256, payload_bytes, is_error, metadata, created_at
			   FROM session_tool_audit_logs
			  WHERE conversation_id = ?
			  ORDER BY created_at DESC
			  LIMIT ?`,
			conversationID, limit,
		)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, conversation_id, session_id, user_id, source, event_type, role, tool_call_id, tool_name,
			        payload, payload_sha256, payload_bytes, is_error, metadata, created_at
			   FROM session_tool_audit_logs
			  ORDER BY created_at DESC
			  LIMIT ?`,
			limit,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("query recent session audit entries: %w", err)
	}
	defer rows.Close()

	entries := make([]Entry, 0, limit)
	for rows.Next() {
		var e Entry
		var isError int
		var metadataRaw string
		var createdAt string
		if err := rows.Scan(
			&e.ID, &e.ConversationID, &e.SessionID, &e.UserID, &e.Source, &e.EventType, &e.Role,
			&e.ToolCallID, &e.ToolName, &e.Payload, &e.PayloadSHA256, &e.PayloadBytes, &isError, &metadataRaw, &createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan session audit entry: %w", err)
		}
		e.IsError = isError != 0
		if metadataRaw != "" && metadataRaw != "{}" {
			_ = json.Unmarshal([]byte(metadataRaw), &e.Metadata)
		}
		if ts, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
			e.CreatedAt = ts
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// DeleteConversation removes all audit logs for one conversation.
func (s *Store) DeleteConversation(ctx context.Context, conversationID string) error {
	if s == nil || s.db == nil {
		return nil
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM session_tool_audit_logs WHERE conversation_id = ?`, conversationID); err != nil {
		return fmt.Errorf("delete conversation audit logs: %w", err)
	}
	return nil
}

// PruneExpired deletes entries older than retention days.
func (s *Store) PruneExpired(ctx context.Context) error {
	if s == nil || s.db == nil {
		return nil
	}
	if s.cfg.RetentionDays <= 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cutoff := timeutil.NowTime().AddDate(0, 0, -s.cfg.RetentionDays).UTC().Format(time.RFC3339Nano)

	for {
		result, err := s.db.ExecContext(ctx,
			`DELETE FROM session_tool_audit_logs
			  WHERE id IN (
			    SELECT id
			      FROM session_tool_audit_logs
			     WHERE created_at < ?
			     ORDER BY created_at ASC
			     LIMIT ?
			  )`,
			cutoff,
			s.cfg.CleanupBatchSize,
		)
		if err != nil {
			return fmt.Errorf("prune expired session audit logs: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil || affected == 0 {
			return nil
		}
	}
}

func (s *Store) cleanupLoop() {
	ticker := time.NewTicker(s.cfg.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_ = s.PruneExpired(context.Background())
		case <-s.done:
			return
		}
	}
}

// Close stops background cleanup and closes the DB.
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	s.closeMu.Lock()
	if s.isClosed {
		s.closeMu.Unlock()
		return nil
	}
	s.isClosed = true
	close(s.done)
	db := s.db
	s.db = nil
	s.closeMu.Unlock()

	if db != nil && s.ownsDB {
		return db.Close()
	}
	return nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
