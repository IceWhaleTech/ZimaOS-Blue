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
	z "github.com/IceWhaleTech/zorm"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// StoreConfig controls tool payload audit storage and retention.
type StoreConfig struct {
	RetentionDays      int
	CleanupInterval    time.Duration
	CleanupBatchSize   int
	Durability         string
	WALAutoCheckpoint  int
	CheckpointInterval time.Duration
}

// DefaultStoreConfig returns the default tool payload audit config.
func DefaultStoreConfig() StoreConfig {
	return StoreConfig{
		RetentionDays:      30,
		CleanupInterval:    6 * time.Hour,
		CleanupBatchSize:   500,
		Durability:         "normal",
		WALAutoCheckpoint:  4000,
		CheckpointInterval: 60 * time.Second,
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
	db         *sql.DB
	readDB     *sql.DB
	cfg        StoreConfig
	done       chan struct{}
	ownsDB     bool
	dbPath     string
	closeMu    sync.Mutex
	isClosed   bool
	recoveryMu sync.Mutex
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
	if strings.TrimSpace(cfg.Durability) == "" {
		cfg.Durability = DefaultStoreConfig().Durability
	}
	cfg.Durability = strings.ToLower(strings.TrimSpace(cfg.Durability))
	switch cfg.Durability {
	case "normal", "full":
	default:
		cfg.Durability = DefaultStoreConfig().Durability
	}
	if cfg.WALAutoCheckpoint <= 0 {
		cfg.WALAutoCheckpoint = DefaultStoreConfig().WALAutoCheckpoint
	}
	if cfg.CheckpointInterval < 0 {
		cfg.CheckpointInterval = 0
	}
	return cfg
}

func newStoreWithDB(writeDB, readDB *sql.DB, cfg StoreConfig, ownsDB bool, dbPath string) (*Store, error) {
	if writeDB == nil {
		return nil, fmt.Errorf("session audit db is nil")
	}
	if readDB == nil {
		readDB = writeDB
	}
	cfg = normalizeStoreConfig(cfg)
	if _, err := writeDB.Exec(storeSchema); err != nil {
		return nil, fmt.Errorf("create session audit schema: %w", err)
	}

	s := &Store{
		db:     writeDB,
		readDB: readDB,
		cfg:    cfg,
		done:   make(chan struct{}),
		ownsDB: ownsDB,
		dbPath: dbPath,
	}

	if s.cfg.RetentionDays > 0 || s.cfg.CheckpointInterval > 0 {
		_ = s.PruneExpired(context.Background())
		go s.cleanupLoop()
	}
	return s, nil
}

func (s *Store) reader() *sql.DB {
	if s != nil && s.readDB != nil {
		return s.readDB
	}
	if s == nil {
		return nil
	}
	return s.db
}

func (s *Store) table(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "session_tool_audit_logs")
}

func (s *Store) readTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "session_tool_audit_logs")
}

type sessionAuditRow struct {
	ID             string `json:"id" zorm:"id"`
	ConversationID string `json:"conversation_id" zorm:"conversation_id"`
	SessionID      string `json:"session_id" zorm:"session_id"`
	UserID         string `json:"user_id" zorm:"user_id"`
	Source         string `json:"source" zorm:"source"`
	EventType      string `json:"event_type" zorm:"event_type"`
	Role           string `json:"role" zorm:"role"`
	ToolCallID     string `json:"tool_call_id" zorm:"tool_call_id"`
	ToolName       string `json:"tool_name" zorm:"tool_name"`
	Payload        string `json:"payload" zorm:"payload"`
	PayloadSHA256  string `json:"payload_sha256" zorm:"payload_sha256"`
	PayloadBytes   int    `json:"payload_bytes" zorm:"payload_bytes"`
	IsError        int    `json:"is_error" zorm:"is_error"`
	Metadata       string `json:"metadata" zorm:"metadata"`
	CreatedAt      string `json:"created_at" zorm:"created_at"`
}

type sessionAuditIDRow struct {
	ID string `json:"id" zorm:"id"`
}

func rowToEntry(row sessionAuditRow) Entry {
	entry := Entry{
		ID:             row.ID,
		ConversationID: row.ConversationID,
		SessionID:      row.SessionID,
		UserID:         row.UserID,
		Source:         row.Source,
		EventType:      row.EventType,
		Role:           row.Role,
		ToolCallID:     row.ToolCallID,
		ToolName:       row.ToolName,
		Payload:        row.Payload,
		PayloadSHA256:  row.PayloadSHA256,
		PayloadBytes:   row.PayloadBytes,
		IsError:        row.IsError != 0,
	}
	if row.Metadata != "" && row.Metadata != "{}" {
		_ = json.Unmarshal([]byte(row.Metadata), &entry.Metadata)
	}
	if ts, err := time.Parse(time.RFC3339Nano, row.CreatedAt); err == nil {
		entry.CreatedAt = ts
	}
	return entry
}

func rowsToEntries(rows []sessionAuditRow) []Entry {
	entries := make([]Entry, 0, len(rows))
	for i := range rows {
		entries = append(entries, rowToEntry(rows[i]))
	}
	return entries
}

func entryToAuditValues(entry Entry) (z.V, error) {
	if strings.TrimSpace(entry.ID) == "" {
		entry.ID = uuid.NewString()
	}
	entry.ConversationID = strings.TrimSpace(entry.ConversationID)
	if entry.ConversationID == "" {
		return nil, fmt.Errorf("conversation_id is required")
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

	return z.V{
		"id":              entry.ID,
		"conversation_id": entry.ConversationID,
		"session_id":      entry.SessionID,
		"user_id":         entry.UserID,
		"source":          entry.Source,
		"event_type":      entry.EventType,
		"role":            entry.Role,
		"tool_call_id":    entry.ToolCallID,
		"tool_name":       entry.ToolName,
		"payload":         entry.Payload,
		"payload_sha256":  entry.PayloadSHA256,
		"payload_bytes":   entry.PayloadBytes,
		"is_error":        boolToInt(entry.IsError),
		"metadata":        metadataJSON,
		"created_at":      entry.CreatedAt.UTC().Format(time.RFC3339Nano),
	}, nil
}

func openSessionAuditReaderDB(dbPath string) (*sql.DB, error) {
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" || dbPath == ":memory:" {
		return nil, nil
	}
	dsn := fmt.Sprintf("file:%s?mode=ro", dbPath)
	db, err := dbutil.OpenSQLiteWithRecovery(dsn, dbPath, func(db *sql.DB) error {
		db.SetMaxOpenConns(4)
		db.SetMaxIdleConns(2)
		if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
			return fmt.Errorf("set reader busy timeout: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}

// NewSQLiteStore opens/creates an isolated SQLite database for tool payload audit logs.
func NewSQLiteStore(dbPath string, cfg StoreConfig) (*Store, error) {
	if strings.TrimSpace(dbPath) == "" {
		return nil, fmt.Errorf("session audit db path is empty")
	}
	cfg = normalizeStoreConfig(cfg)

	db, err := dbutil.OpenSQLiteWithRecoveryAndRecreate(dbPath, dbPath, func(db *sql.DB) error {
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)

		if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
			return fmt.Errorf("enable WAL mode: %w", err)
		}
		if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
			return fmt.Errorf("set busy timeout: %w", err)
		}
		if _, err := db.Exec("PRAGMA synchronous=" + strings.ToUpper(cfg.Durability)); err != nil {
			return fmt.Errorf("set synchronous mode: %w", err)
		}
		if _, err := db.Exec(fmt.Sprintf("PRAGMA wal_autocheckpoint=%d", cfg.WALAutoCheckpoint)); err != nil {
			return fmt.Errorf("set wal autocheckpoint: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("open session audit db: %w", err)
	}

	readDB, readErr := openSessionAuditReaderDB(dbPath)
	if readErr != nil {
		readDB = db
	}

	s, err := newStoreWithDB(db, readDB, cfg, true, dbPath)
	if err != nil {
		if readDB != nil && readDB != db {
			_ = readDB.Close()
		}
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// NewSQLiteStoreWithDB reuses an existing SQLite connection for audit storage.
func NewSQLiteStoreWithDB(db *sql.DB, cfg StoreConfig) (*Store, error) {
	return NewSQLiteStoreWithReadDB(db, db, cfg)
}

// NewSQLiteStoreWithReadDB reuses existing SQLite write/read connections for
// audit storage.
func NewSQLiteStoreWithReadDB(writeDB, readDB *sql.DB, cfg StoreConfig) (*Store, error) {
	return newStoreWithDB(writeDB, readDB, cfg, false, "")
}

// Record appends one audit entry.
func (s *Store) Record(ctx context.Context, entry Entry) error {
	return s.RecordBatch(ctx, []Entry{entry})
}

// RecordBatch appends audit entries in a single transaction.
func (s *Store) RecordBatch(ctx context.Context, entries []Entry) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("session audit store is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if len(entries) == 0 {
		return nil
	}

	return s.retryOnCorruption(func() error {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin audit batch tx: %w", err)
		}
		defer tx.Rollback()
		table := z.TableContext(ctx, tx, "session_tool_audit_logs")

		for _, entry := range entries {
			values, err := entryToAuditValues(entry)
			if err != nil {
				return err
			}
			if _, err := table.Insert(values); err != nil {
				return fmt.Errorf("insert session audit entry: %w", err)
			}
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit audit batch tx: %w", err)
		}
		return nil
	})
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
	opts := []z.ZormItem{
		z.OrderBy("created_at DESC"),
		z.Limit(limit),
	}
	if conversationID != "" {
		opts = append([]z.ZormItem{z.Where(z.Eq("conversation_id", conversationID))}, opts...)
	}
	var rows []sessionAuditRow
	if _, err := s.readTable(ctx).Select(&rows, opts...); err != nil {
		return nil, fmt.Errorf("query recent session audit entries: %w", err)
	}
	return rowsToEntries(rows), nil
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
	if _, err := s.table(ctx).Delete(z.Where(z.Eq("conversation_id", conversationID))); err != nil {
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
		var rows []sessionAuditIDRow
		if _, err := s.readTable(ctx).Select(&rows,
			z.Fields("id"),
			z.Where(z.Lt("created_at", cutoff)),
			z.OrderBy("created_at ASC"),
			z.Limit(s.cfg.CleanupBatchSize),
		); err != nil {
			return fmt.Errorf("select expired session audit logs: %w", err)
		}
		if len(rows) == 0 {
			return nil
		}
		ids := make([]string, 0, len(rows))
		for i := range rows {
			ids = append(ids, rows[i].ID)
		}
		affected, err := s.table(ctx).Delete(z.Where(z.In("id", ids)))
		if err != nil {
			return fmt.Errorf("prune expired session audit logs: %w", err)
		}
		if affected == 0 {
			return nil
		}
	}
}

func (s *Store) cleanupLoop() {
	var cleanupTicker *time.Ticker
	var checkpointTicker *time.Ticker
	if s.cfg.CleanupInterval > 0 {
		cleanupTicker = time.NewTicker(s.cfg.CleanupInterval)
		defer cleanupTicker.Stop()
	}
	if s.cfg.CheckpointInterval > 0 {
		checkpointTicker = time.NewTicker(s.cfg.CheckpointInterval)
		defer checkpointTicker.Stop()
	}

	for {
		select {
		case <-tickerChan(cleanupTicker):
			_ = s.PruneExpired(context.Background())
		case <-tickerChan(checkpointTicker):
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_ = dbutil.CheckpointWAL(ctx, s.db, dbutil.CheckpointPassive)
			cancel()
		case <-s.done:
			return
		}
	}
}

func tickerChan(t *time.Ticker) <-chan time.Time {
	if t == nil {
		return nil
	}
	return t.C
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
	readDB := s.readDB
	s.db = nil
	s.readDB = nil
	s.closeMu.Unlock()

	if db != nil && s.ownsDB {
		if readDB != nil && readDB != db {
			_ = readDB.Close()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = dbutil.CheckpointWAL(ctx, db, dbutil.CheckpointTruncate)
		cancel()
		return db.Close()
	}
	return nil
}

func (s *Store) retryOnCorruption(op func() error) error {
	if op == nil {
		return nil
	}
	err := op()
	if err == nil || !dbutil.IsSQLiteCorruptionError(err) {
		return err
	}
	if recoverErr := s.Recover(); recoverErr != nil {
		return fmt.Errorf("%w (recover failed: %v)", err, recoverErr)
	}
	return op()
}

// Recover attempts to reopen the dedicated audit DB with the shared recovery flow.
func (s *Store) Recover() error {
	if s == nil || !s.ownsDB || strings.TrimSpace(s.dbPath) == "" || strings.TrimSpace(s.dbPath) == ":memory:" {
		return fmt.Errorf("session audit recovery is unavailable")
	}

	s.recoveryMu.Lock()
	defer s.recoveryMu.Unlock()

	if s.db != nil {
		if s.readDB != nil && s.readDB != s.db {
			_ = s.readDB.Close()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = dbutil.CheckpointWAL(ctx, s.db, dbutil.CheckpointTruncate)
		cancel()
		_ = s.db.Close()
	}

	db, err := dbutil.OpenSQLiteWithRecoveryAndRecreate(s.dbPath, s.dbPath, func(db *sql.DB) error {
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
			return fmt.Errorf("enable WAL mode: %w", err)
		}
		if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
			return fmt.Errorf("set busy timeout: %w", err)
		}
		if _, err := db.Exec("PRAGMA synchronous=" + strings.ToUpper(s.cfg.Durability)); err != nil {
			return fmt.Errorf("set synchronous mode: %w", err)
		}
		if _, err := db.Exec(fmt.Sprintf("PRAGMA wal_autocheckpoint=%d", s.cfg.WALAutoCheckpoint)); err != nil {
			return fmt.Errorf("set wal autocheckpoint: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.db = db
	readDB, readErr := openSessionAuditReaderDB(s.dbPath)
	if readErr != nil {
		s.readDB = db
	} else if readDB != nil {
		s.readDB = readDB
	} else {
		s.readDB = db
	}
	if _, err := s.db.Exec(storeSchema); err != nil {
		return fmt.Errorf("create session audit schema: %w", err)
	}
	return nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
