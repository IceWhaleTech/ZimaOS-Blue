package sessionaudit

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/logger"
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

type SearchOptions struct {
	Query                string `json:"query"`
	ConversationID       string `json:"conversation_id,omitempty"`
	UserID               string `json:"user_id,omitempty"`
	Limit                int    `json:"limit,omitempty"`
	PerConversationLimit int    `json:"per_conversation_limit,omitempty"`
	SnippetLength        int    `json:"snippet_length,omitempty"`
}

type SearchSnippet struct {
	AuditEntryID string    `json:"audit_entry_id"`
	Role         string    `json:"role"`
	EventType    string    `json:"event_type"`
	ToolName     string    `json:"tool_name,omitempty"`
	Source       string    `json:"source,omitempty"`
	Snippet      string    `json:"snippet"`
	CreatedAt    time.Time `json:"created_at"`
}

type SearchConversationResult struct {
	ConversationID  string          `json:"conversation_id"`
	SessionID       string          `json:"session_id,omitempty"`
	UserID          string          `json:"user_id,omitempty"`
	HitCount        int             `json:"hit_count"`
	LatestCreatedAt time.Time       `json:"latest_created_at"`
	Snippets        []SearchSnippet `json:"snippets"`
}

type rgJSONEvent struct {
	Type string `json:"type"`
	Data struct {
		Lines struct {
			Text string `json:"text"`
		} `json:"lines"`
	} `json:"data"`
}

// Store persists tool payload audit events and searchable session recall
// projections in SQLite plus a JSONL sidecar for stream-friendly raw reads.
type Store struct {
	db         *sql.DB
	readDB     *sql.DB
	cfg        StoreConfig
	done       chan struct{}
	ownsDB     bool
	dbPath     string
	rawLogDir  string
	ftsEnabled bool
	closeMu    sync.Mutex
	isClosed   bool
	recoveryMu sync.Mutex
	subMu      sync.RWMutex
	subs       map[uint64]*auditSubscriber
	nextSubID  uint64
}

type auditSubscriber struct {
	conversationID string
	ch             chan Entry
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

CREATE TABLE IF NOT EXISTS session_search_projections (
	audit_entry_id  TEXT PRIMARY KEY,
	conversation_id TEXT NOT NULL,
	session_id      TEXT NOT NULL DEFAULT '',
	user_id         TEXT NOT NULL DEFAULT '',
	source          TEXT NOT NULL DEFAULT '',
	event_type      TEXT NOT NULL,
	role            TEXT NOT NULL,
	tool_name       TEXT NOT NULL DEFAULT '',
	search_text     TEXT NOT NULL,
	display_text    TEXT NOT NULL,
	created_at      TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_session_search_projection_conv_created
	ON session_search_projections(conversation_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_session_search_projection_created
	ON session_search_projections(created_at DESC);
`

const sessionSearchFTSSchema = `
CREATE VIRTUAL TABLE IF NOT EXISTS session_search_fts USING fts5(
	audit_entry_id UNINDEXED,
	conversation_id UNINDEXED,
	search_text
);
`

const (
	searchProjectionMaxBytes       = 16 * 1024
	defaultSearchConversationLimit = 10
	defaultSearchSnippetLimit      = 3
	defaultSearchSnippetLength     = 160
	rawLogDirName                  = "session_audit_logs"
	rawLogScannerMaxBytes          = 16 * 1024 * 1024
	auditSubscriberBuffer          = 256
)

const DefaultDBFilename = "session_audit.db"

// ResolveRawLogDir returns the default JSONL sidecar directory for session audit logs.
func ResolveRawLogDir(baseDir string) string {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		return rawLogDirName
	}
	return filepath.Join(baseDir, rawLogDirName)
}

// ResolveDBPath returns the effective SQLite audit DB path for DB-backed mode.
// Empty paths default to a dedicated session_audit.db under dataDir.
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
	ftsEnabled, err := initStoreSchema(writeDB)
	if err != nil {
		return nil, err
	}

	s := &Store{
		db:         writeDB,
		readDB:     readDB,
		cfg:        cfg,
		done:       make(chan struct{}),
		ownsDB:     ownsDB,
		dbPath:     dbPath,
		rawLogDir:  resolveRawLogDir(writeDB, dbPath),
		ftsEnabled: ftsEnabled,
		subs:       make(map[uint64]*auditSubscriber),
	}
	if s.rawLogDir != "" {
		if err := os.MkdirAll(s.rawLogDir, 0o755); err != nil {
			logger.Warn().Err(err).Str("path", s.rawLogDir).Msg("[sessionaudit] failed to initialize raw audit log dir")
			s.rawLogDir = ""
		}
	}

	if s.cfg.RetentionDays > 0 || s.cfg.CheckpointInterval > 0 {
		_ = s.PruneExpired(context.Background())
		go s.cleanupLoop()
	}
	return s, nil
}

// NewJSONLStore creates a JSONL-only audit store under baseDir/session_audit_logs.
func NewJSONLStore(baseDir string, cfg StoreConfig) (*Store, error) {
	rawLogDir := ResolveRawLogDir(baseDir)
	if strings.TrimSpace(rawLogDir) == "" {
		return nil, fmt.Errorf("session audit raw log dir is empty")
	}
	cfg = normalizeStoreConfig(cfg)
	cfg.CheckpointInterval = 0

	s := &Store{
		cfg:       cfg,
		done:      make(chan struct{}),
		ownsDB:    false,
		dbPath:    "",
		rawLogDir: rawLogDir,
		subs:      make(map[uint64]*auditSubscriber),
	}
	if err := os.MkdirAll(s.rawLogDir, 0o755); err != nil {
		return nil, fmt.Errorf("create session audit raw log dir: %w", err)
	}
	if s.cfg.RetentionDays > 0 {
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
	ID             string `json:"id" zorm:"id"`
	ConversationID string `json:"conversation_id" zorm:"conversation_id"`
}

type searchProjection struct {
	AuditEntryID   string
	ConversationID string
	SessionID      string
	UserID         string
	Source         string
	EventType      string
	Role           string
	ToolName       string
	SearchText     string
	DisplayText    string
	CreatedAt      time.Time
}

type searchProjectionRow struct {
	AuditEntryID   string
	ConversationID string
	SessionID      string
	UserID         string
	Source         string
	EventType      string
	Role           string
	ToolName       string
	DisplayText    string
	CreatedAt      string
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

func initStoreSchema(db *sql.DB) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("session audit db is nil")
	}
	if _, err := db.Exec(storeSchema); err != nil {
		return false, fmt.Errorf("create session audit schema: %w", err)
	}
	ftsEnabled := supportsFTS5(db)
	if ftsEnabled {
		if _, err := db.Exec(sessionSearchFTSSchema); err != nil {
			return false, fmt.Errorf("create session audit fts schema: %w", err)
		}
	}
	return ftsEnabled, nil
}

func supportsFTS5(db *sql.DB) bool {
	if db == nil {
		return false
	}
	rows, err := db.Query(`SELECT name FROM pragma_module_list WHERE name = 'fts5'`)
	if err == nil {
		defer rows.Close()
		if rows.Next() {
			return true
		}
	}
	if _, err := db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS temp.session_search_fts5_probe USING fts5(content)`); err != nil {
		return false
	}
	_, _ = db.Exec(`DROP TABLE IF EXISTS temp.session_search_fts5_probe`)
	return true
}

func buildSearchProjection(entry Entry) (*searchProjection, bool) {
	switch strings.TrimSpace(entry.EventType) {
	case "user_message", "assistant_message", "tool_result", "context_pack":
	default:
		return nil, false
	}
	displayText := compactSearchText(entry.Payload)
	if displayText == "" || len(displayText) > searchProjectionMaxBytes {
		return nil, false
	}
	return &searchProjection{
		AuditEntryID:   strings.TrimSpace(entry.ID),
		ConversationID: strings.TrimSpace(entry.ConversationID),
		SessionID:      strings.TrimSpace(entry.SessionID),
		UserID:         strings.TrimSpace(entry.UserID),
		Source:         strings.TrimSpace(entry.Source),
		EventType:      strings.TrimSpace(entry.EventType),
		Role:           strings.TrimSpace(entry.Role),
		ToolName:       strings.TrimSpace(entry.ToolName),
		SearchText:     displayText,
		DisplayText:    displayText,
		CreatedAt:      entry.CreatedAt.UTC(),
	}, true
}

func resolveRawLogDir(db *sql.DB, dbPath string) string {
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" {
		dbPath = detectMainDBPath(db)
	}
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" || dbPath == ":memory:" {
		return ""
	}
	if strings.HasPrefix(dbPath, "file:") {
		trimmed := strings.TrimPrefix(dbPath, "file:")
		if idx := strings.Index(trimmed, "?"); idx >= 0 {
			trimmed = trimmed[:idx]
		}
		dbPath = strings.TrimSpace(trimmed)
		if dbPath == "" || dbPath == ":memory:" {
			return ""
		}
	}
	return filepath.Join(filepath.Dir(dbPath), rawLogDirName)
}

func detectMainDBPath(db *sql.DB) string {
	if db == nil {
		return ""
	}
	rows, err := db.Query(`PRAGMA database_list`)
	if err != nil {
		return ""
	}
	defer rows.Close()

	for rows.Next() {
		var (
			seq  int
			name string
			file string
		)
		if err := rows.Scan(&seq, &name, &file); err != nil {
			return ""
		}
		if name == "main" {
			return strings.TrimSpace(file)
		}
	}
	return ""
}

func sanitizeRawLogFilePrefix(conversationID string) string {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return "conversation"
	}
	var b strings.Builder
	for _, r := range conversationID {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
		if b.Len() >= 48 {
			break
		}
	}
	out := strings.Trim(b.String(), "._")
	if out == "" {
		return "conversation"
	}
	return out
}

func (s *Store) rawLogPath(conversationID string) string {
	if s == nil || s.rawLogDir == "" {
		return ""
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(conversationID))
	return filepath.Join(
		s.rawLogDir,
		fmt.Sprintf("%s-%s.jsonl", sanitizeRawLogFilePrefix(conversationID), hex.EncodeToString(sum[:8])),
	)
}

func compactSearchText(payload string) string {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return ""
	}
	return strings.Join(strings.Fields(payload), " ")
}

func normalizeSearchOptions(opts SearchOptions) SearchOptions {
	opts.Query = strings.TrimSpace(opts.Query)
	opts.ConversationID = strings.TrimSpace(opts.ConversationID)
	opts.UserID = strings.TrimSpace(opts.UserID)
	if opts.Limit <= 0 {
		opts.Limit = defaultSearchConversationLimit
	}
	if opts.PerConversationLimit <= 0 {
		opts.PerConversationLimit = defaultSearchSnippetLimit
	}
	if opts.SnippetLength <= 0 {
		opts.SnippetLength = defaultSearchSnippetLength
	}
	return opts
}

func searchTerms(query string) []string {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil
	}
	parts := strings.Fields(query)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func escapeFTS5Query(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}
	words := strings.Fields(query)
	for i, word := range words {
		word = strings.ReplaceAll(word, `"`, `""`)
		words[i] = `"` + word + `"*`
	}
	return strings.Join(words, " OR ")
}

func buildSnippet(text, query string, maxLen int) string {
	text = compactSearchText(text)
	if text == "" {
		return ""
	}
	runes := []rune(text)
	if maxLen <= 0 || len(runes) <= maxLen {
		return text
	}
	lowerText := strings.ToLower(text)
	matchAt := -1
	for _, term := range searchTerms(query) {
		if idx := strings.Index(lowerText, term); idx >= 0 && (matchAt == -1 || idx < matchAt) {
			matchAt = idx
		}
	}
	if matchAt < 0 {
		return string(runes[:maxLen]) + "..."
	}
	matchRune := len([]rune(text[:matchAt]))
	start := matchRune - maxLen/3
	if start < 0 {
		start = 0
	}
	end := start + maxLen
	if end > len(runes) {
		end = len(runes)
		start = end - maxLen
		if start < 0 {
			start = 0
		}
	}
	snippet := string(runes[start:end])
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(runes) {
		snippet += "..."
	}
	return snippet
}

func parseSearchTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	ts, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, false
	}
	return ts, true
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
	if s == nil || (s.db == nil && strings.TrimSpace(s.rawLogDir) == "") {
		return fmt.Errorf("session audit store is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if len(entries) == 0 {
		return nil
	}
	if s.db == nil {
		normalizedEntries := make([]Entry, 0, len(entries))
		for _, entry := range entries {
			if strings.TrimSpace(entry.ID) == "" {
				entry.ID = uuid.NewString()
			}
			if entry.CreatedAt.IsZero() {
				entry.CreatedAt = timeutil.NowTime()
			}
			entry.PayloadBytes = len(entry.Payload)
			if strings.TrimSpace(entry.PayloadSHA256) == "" {
				sum := sha256.Sum256([]byte(entry.Payload))
				entry.PayloadSHA256 = hex.EncodeToString(sum[:])
			}
			if _, err := entryToAuditValues(entry); err != nil {
				return err
			}
			normalizedEntries = append(normalizedEntries, entry)
		}
		s.appendRawLogBatch(normalizedEntries)
		s.publishBatch(normalizedEntries)
		return nil
	}

	return s.retryOnCorruption(func() error {
		normalizedEntries := make([]Entry, 0, len(entries))
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin audit batch tx: %w", err)
		}
		defer tx.Rollback()
		table := z.TableContext(ctx, tx, "session_tool_audit_logs")

		for _, entry := range entries {
			if strings.TrimSpace(entry.ID) == "" {
				entry.ID = uuid.NewString()
			}
			if entry.CreatedAt.IsZero() {
				entry.CreatedAt = timeutil.NowTime()
			}
			entry.PayloadBytes = len(entry.Payload)
			if strings.TrimSpace(entry.PayloadSHA256) == "" {
				sum := sha256.Sum256([]byte(entry.Payload))
				entry.PayloadSHA256 = hex.EncodeToString(sum[:])
			}
			normalizedEntries = append(normalizedEntries, entry)
			values, err := entryToAuditValues(entry)
			if err != nil {
				return err
			}
			if _, err := table.Insert(values); err != nil {
				return fmt.Errorf("insert session audit entry: %w", err)
			}
			if projection, ok := buildSearchProjection(entry); ok {
				if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO session_search_projections (
					audit_entry_id, conversation_id, session_id, user_id, source, event_type, role, tool_name, search_text, display_text, created_at
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					projection.AuditEntryID, projection.ConversationID, projection.SessionID, projection.UserID, projection.Source,
					projection.EventType, projection.Role, projection.ToolName, projection.SearchText, projection.DisplayText,
					projection.CreatedAt.Format(time.RFC3339Nano),
				); err != nil {
					return fmt.Errorf("insert session search projection: %w", err)
				}
				if s.ftsEnabled {
					if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO session_search_fts (
						audit_entry_id, conversation_id, search_text
					) VALUES (?, ?, ?)`, projection.AuditEntryID, projection.ConversationID, projection.SearchText); err != nil {
						return fmt.Errorf("insert session search fts entry: %w", err)
					}
				}
			}
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit audit batch tx: %w", err)
		}
		s.appendRawLogBatch(normalizedEntries)
		s.publishBatch(normalizedEntries)
		return nil
	})
}

// Subscribe registers a non-blocking live audit subscriber.
// If conversationID is non-empty, only matching conversation entries are sent.
func (s *Store) Subscribe(conversationID string) (<-chan Entry, func()) {
	ch := make(chan Entry, auditSubscriberBuffer)
	if s == nil {
		close(ch)
		return ch, func() {}
	}

	conversationID = strings.TrimSpace(conversationID)

	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.isClosed {
		close(ch)
		return ch, func() {}
	}

	s.subMu.Lock()
	s.nextSubID++
	id := s.nextSubID
	if s.subs == nil {
		s.subs = make(map[uint64]*auditSubscriber)
	}
	s.subs[id] = &auditSubscriber{
		conversationID: conversationID,
		ch:             ch,
	}
	s.subMu.Unlock()

	var once sync.Once
	cleanup := func() {
		once.Do(func() {
			s.subMu.Lock()
			sub, ok := s.subs[id]
			if ok {
				delete(s.subs, id)
				close(sub.ch)
			}
			s.subMu.Unlock()
		})
	}
	return ch, cleanup
}

func (s *Store) publishBatch(entries []Entry) {
	if s == nil || len(entries) == 0 {
		return
	}
	s.subMu.RLock()
	defer s.subMu.RUnlock()

	if len(s.subs) == 0 {
		return
	}
	for _, entry := range entries {
		for _, sub := range s.subs {
			if sub == nil {
				continue
			}
			if sub.conversationID != "" && sub.conversationID != strings.TrimSpace(entry.ConversationID) {
				continue
			}
			select {
			case sub.ch <- entry:
			default:
			}
		}
	}
}

// SearchConversations returns grouped searchable snippets by conversation.
func (s *Store) SearchConversations(ctx context.Context, opts SearchOptions) ([]SearchConversationResult, error) {
	if s == nil || (s.db == nil && strings.TrimSpace(s.rawLogDir) == "") {
		return nil, fmt.Errorf("session audit store is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	opts = normalizeSearchOptions(opts)
	if opts.Query == "" {
		return nil, nil
	}

	var (
		rows []searchProjectionRow
		err  error
	)
	if s.db != nil {
		rows, err = s.searchProjectionRows(ctx, s.reader(), opts)
		if err != nil {
			logger.Warn().Err(err).Str("query", opts.Query).Msg("[sessionaudit] projection search failed; attempting raw log fallback")
		}
	}
	results := buildProjectionSearchResults(rows, opts)

	if err == nil && len(results) >= opts.Limit {
		return results, nil
	}

	rawResults, rawErr := s.searchRawConversations(ctx, opts, results)
	if rawErr != nil {
		logger.Warn().Err(rawErr).Str("query", opts.Query).Msg("[sessionaudit] raw log recall fallback failed")
		if err != nil && len(results) == 0 {
			return nil, err
		}
		return results, nil
	}
	results = mergeSearchConversationResults(results, rawResults, opts.Limit)
	if err != nil && len(results) == 0 {
		return nil, err
	}
	return results, nil
}

func buildProjectionSearchResults(rows []searchProjectionRow, opts SearchOptions) []SearchConversationResult {
	results := make([]SearchConversationResult, 0, opts.Limit)
	indexByConversation := make(map[string]int, opts.Limit)
	for _, row := range rows {
		idx, ok := indexByConversation[row.ConversationID]
		if !ok {
			if len(results) >= opts.Limit {
				continue
			}
			idx = len(results)
			indexByConversation[row.ConversationID] = idx
			results = append(results, SearchConversationResult{
				ConversationID: row.ConversationID,
				SessionID:      row.SessionID,
				UserID:         row.UserID,
			})
		}

		result := &results[idx]
		result.HitCount++
		if createdAt, ok := parseSearchTime(row.CreatedAt); ok {
			if createdAt.After(result.LatestCreatedAt) {
				result.LatestCreatedAt = createdAt
			}
			if len(result.Snippets) < opts.PerConversationLimit {
				result.Snippets = append(result.Snippets, SearchSnippet{
					AuditEntryID: row.AuditEntryID,
					Role:         row.Role,
					EventType:    row.EventType,
					ToolName:     row.ToolName,
					Source:       row.Source,
					Snippet:      buildSnippet(row.DisplayText, opts.Query, opts.SnippetLength),
					CreatedAt:    createdAt,
				})
			}
			continue
		}
		if len(result.Snippets) < opts.PerConversationLimit {
			result.Snippets = append(result.Snippets, SearchSnippet{
				AuditEntryID: row.AuditEntryID,
				Role:         row.Role,
				EventType:    row.EventType,
				ToolName:     row.ToolName,
				Source:       row.Source,
				Snippet:      buildSnippet(row.DisplayText, opts.Query, opts.SnippetLength),
			})
		}
	}
	return results
}

func mergeSearchConversationResults(existing, extra []SearchConversationResult, limit int) []SearchConversationResult {
	if len(extra) == 0 {
		return existing
	}
	seen := make(map[string]struct{}, len(existing)+len(extra))
	merged := make([]SearchConversationResult, 0, len(existing)+len(extra))
	for _, result := range existing {
		if strings.TrimSpace(result.ConversationID) == "" {
			continue
		}
		if _, ok := seen[result.ConversationID]; ok {
			continue
		}
		seen[result.ConversationID] = struct{}{}
		merged = append(merged, result)
	}
	for _, result := range extra {
		if strings.TrimSpace(result.ConversationID) == "" {
			continue
		}
		if _, ok := seen[result.ConversationID]; ok {
			continue
		}
		seen[result.ConversationID] = struct{}{}
		merged = append(merged, result)
	}
	sort.SliceStable(merged, func(i, j int) bool {
		if !merged[i].LatestCreatedAt.Equal(merged[j].LatestCreatedAt) {
			return merged[i].LatestCreatedAt.After(merged[j].LatestCreatedAt)
		}
		if merged[i].HitCount != merged[j].HitCount {
			return merged[i].HitCount > merged[j].HitCount
		}
		return strings.Compare(merged[i].ConversationID, merged[j].ConversationID) > 0
	})
	if limit > 0 && len(merged) > limit {
		merged = merged[:limit]
	}
	return merged
}

func (s *Store) searchProjectionRows(ctx context.Context, db *sql.DB, opts SearchOptions) ([]searchProjectionRow, error) {
	if db == nil {
		return nil, fmt.Errorf("session audit reader is not initialized")
	}
	maxRows := opts.Limit * opts.PerConversationLimit * 4
	if maxRows < opts.Limit {
		maxRows = opts.Limit
	}

	var (
		args    []interface{}
		sqlText string
	)
	if s.ftsEnabled {
		filters := []string{`session_search_fts MATCH ?`}
		args = append(args, escapeFTS5Query(opts.Query))
		if opts.ConversationID != "" {
			filters = append(filters, "p.conversation_id = ?")
			args = append(args, opts.ConversationID)
		}
		if opts.UserID != "" {
			filters = append(filters, "p.user_id = ?")
			args = append(args, opts.UserID)
		}
		sqlText = `SELECT
			p.audit_entry_id, p.conversation_id, p.session_id, p.user_id, p.source, p.event_type, p.role, p.tool_name, p.display_text, p.created_at
		FROM session_search_fts
		JOIN session_search_projections p ON p.audit_entry_id = session_search_fts.audit_entry_id
		WHERE ` + strings.Join(filters, " AND ") + `
		ORDER BY p.created_at DESC
		LIMIT ?`
		args = append(args, maxRows)
	} else {
		termFilters := make([]string, 0, len(searchTerms(opts.Query)))
		for _, term := range searchTerms(opts.Query) {
			termFilters = append(termFilters, "LOWER(search_text) LIKE ?")
			args = append(args, "%"+term+"%")
		}
		if len(termFilters) == 0 {
			return nil, nil
		}
		filters := []string{"(" + strings.Join(termFilters, " OR ") + ")"}
		if opts.ConversationID != "" {
			filters = append(filters, "conversation_id = ?")
			args = append(args, opts.ConversationID)
		}
		if opts.UserID != "" {
			filters = append(filters, "user_id = ?")
			args = append(args, opts.UserID)
		}
		sqlText = `SELECT
			audit_entry_id, conversation_id, session_id, user_id, source, event_type, role, tool_name, display_text, created_at
		FROM session_search_projections
		WHERE ` + strings.Join(filters, " AND ") + `
		ORDER BY created_at DESC
		LIMIT ?`
		args = append(args, maxRows)
	}
	return querySearchProjectionRows(ctx, db, sqlText, args...)
}

func querySearchProjectionRows(ctx context.Context, db *sql.DB, query string, args ...interface{}) ([]searchProjectionRow, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]searchProjectionRow, 0)
	for rows.Next() {
		var row searchProjectionRow
		if err := rows.Scan(
			&row.AuditEntryID,
			&row.ConversationID,
			&row.SessionID,
			&row.UserID,
			&row.Source,
			&row.EventType,
			&row.Role,
			&row.ToolName,
			&row.DisplayText,
			&row.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// Recent returns recent entries, optionally scoped to one conversation.
func (s *Store) Recent(ctx context.Context, conversationID string, limit int) ([]Entry, error) {
	if s == nil || (s.db == nil && strings.TrimSpace(s.rawLogDir) == "") {
		return nil, fmt.Errorf("session audit store is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 {
		limit = 50
	}

	conversationID = strings.TrimSpace(conversationID)
	if conversationID != "" {
		if recent, found, err := s.readRecentRawLog(conversationID, limit); err == nil && found {
			return recent, nil
		} else if err != nil {
			logger.Warn().Err(err).Str("conversation_id", conversationID).Msg("[sessionaudit] failed to read raw jsonl sidecar")
		}
	}
	if s.db == nil {
		return s.readRecentAllRawLogs(limit)
	}
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
	if s == nil || (s.db == nil && strings.TrimSpace(s.rawLogDir) == "") {
		return nil
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if s.db == nil {
		s.removeRawLogFile(conversationID)
		return nil
	}
	if s.ftsEnabled {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM session_search_fts WHERE conversation_id = ?`, conversationID); err != nil {
			return fmt.Errorf("delete conversation session search fts: %w", err)
		}
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM session_search_projections WHERE conversation_id = ?`, conversationID); err != nil {
		return fmt.Errorf("delete conversation session search projections: %w", err)
	}
	if _, err := s.table(ctx).Delete(z.Where(z.Eq("conversation_id", conversationID))); err != nil {
		return fmt.Errorf("delete conversation audit logs: %w", err)
	}
	s.removeRawLogFile(conversationID)
	return nil
}

// PruneExpired deletes entries older than retention days.
func (s *Store) PruneExpired(ctx context.Context) error {
	if s == nil || (s.db == nil && strings.TrimSpace(s.rawLogDir) == "") {
		return nil
	}
	if s.cfg.RetentionDays <= 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cutoffTime := timeutil.NowTime().AddDate(0, 0, -s.cfg.RetentionDays).UTC()
	if s.db == nil {
		return s.pruneRawLogFilesBefore(cutoffTime)
	}
	cutoff := cutoffTime.Format(time.RFC3339Nano)

	for {
		var rows []sessionAuditIDRow
		if _, err := s.readTable(ctx).Select(&rows,
			z.Fields("id", "conversation_id"),
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
		if err := s.deleteSearchProjectionIDs(ctx, ids); err != nil {
			return err
		}
		affected, err := s.table(ctx).Delete(z.Where(z.In("id", ids)))
		if err != nil {
			return fmt.Errorf("prune expired session audit logs: %w", err)
		}
		if affected == 0 {
			return nil
		}
		s.pruneRawLogEntries(rows)
	}
}

func (s *Store) appendRawLogBatch(entries []Entry) {
	if s == nil || len(entries) == 0 {
		return
	}
	grouped := make(map[string][]Entry)
	for _, entry := range entries {
		path := s.rawLogPath(entry.ConversationID)
		if path == "" {
			continue
		}
		grouped[path] = append(grouped[path], entry)
	}
	for path, pathEntries := range grouped {
		if err := appendRawLogFile(path, pathEntries); err != nil {
			logger.Warn().Err(err).Str("path", path).Msg("[sessionaudit] failed to append raw jsonl sidecar")
		}
	}
}

func (s *Store) searchRawConversations(ctx context.Context, opts SearchOptions, existing []SearchConversationResult) ([]SearchConversationResult, error) {
	paths, err := s.rawLogPathsForSearch(opts.ConversationID)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, nil
	}

	entries, err := searchRawEntriesWithRG(ctx, opts, paths)
	if err != nil {
		logger.Warn().Err(err).Str("query", opts.Query).Msg("[sessionaudit] rg raw recall unavailable; scanning raw logs directly")
		entries, err = searchRawEntriesByScan(ctx, opts, paths)
		if err != nil {
			return nil, err
		}
	}
	return buildRawSearchResults(entries, opts, existing), nil
}

func (s *Store) rawLogPathsForSearch(conversationID string) ([]string, error) {
	if s == nil || strings.TrimSpace(s.rawLogDir) == "" {
		return nil, nil
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID != "" {
		path := s.rawLogPath(conversationID)
		if path == "" {
			return nil, nil
		}
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		return []string{path}, nil
	}

	entries, err := os.ReadDir(s.rawLogDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".jsonl") {
			continue
		}
		paths = append(paths, filepath.Join(s.rawLogDir, entry.Name()))
	}
	sort.Strings(paths)
	return paths, nil
}

func searchRawEntriesWithRG(ctx context.Context, opts SearchOptions, paths []string) ([]Entry, error) {
	rgPath, err := exec.LookPath("rg")
	if err != nil {
		return nil, err
	}
	terms := searchTerms(opts.Query)
	if len(terms) == 0 {
		terms = []string{strings.TrimSpace(opts.Query)}
	}
	args := []string{"--json", "-i", "-F"}
	for _, term := range terms {
		if strings.TrimSpace(term) == "" {
			continue
		}
		args = append(args, "-e", term)
	}
	args = append(args, "--")
	args = append(args, paths...)

	cmd := exec.CommandContext(ctx, rgPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), rawLogScannerMaxBytes)
	matches := make([]Entry, 0)
	seenIDs := make(map[string]struct{})
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event rgJSONEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil || event.Type != "match" {
			continue
		}
		entry, ok := parseRawLogEntryLine(event.Data.Lines.Text)
		if !ok || !rawEntryMatchesOptions(entry, opts) {
			continue
		}
		if id := strings.TrimSpace(entry.ID); id != "" {
			if _, exists := seenIDs[id]; exists {
				continue
			}
			seenIDs[id] = struct{}{}
		}
		matches = append(matches, entry)
	}
	scanErr := scanner.Err()
	waitErr := cmd.Wait()
	if exitErr, ok := waitErr.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		waitErr = nil
	}
	if scanErr != nil {
		return nil, scanErr
	}
	if waitErr != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("%w: %s", waitErr, msg)
		}
		return nil, waitErr
	}
	return matches, nil
}

func searchRawEntriesByScan(ctx context.Context, opts SearchOptions, paths []string) ([]Entry, error) {
	matches := make([]Entry, 0)
	seenIDs := make(map[string]struct{})
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64*1024), rawLogScannerMaxBytes)
		for scanner.Scan() {
			entry, ok := parseRawLogEntryLine(scanner.Text())
			if !ok || !rawEntryMatchesOptions(entry, opts) {
				continue
			}
			if id := strings.TrimSpace(entry.ID); id != "" {
				if _, exists := seenIDs[id]; exists {
					continue
				}
				seenIDs[id] = struct{}{}
			}
			matches = append(matches, entry)
		}
		scanErr := scanner.Err()
		closeErr := file.Close()
		if scanErr != nil {
			return nil, scanErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
	}
	return matches, nil
}

func parseRawLogEntryLine(line string) (Entry, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return Entry{}, false
	}
	var entry Entry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return Entry{}, false
	}
	return entry, true
}

func rawEntryMatchesOptions(entry Entry, opts SearchOptions) bool {
	if opts.ConversationID != "" && strings.TrimSpace(entry.ConversationID) != opts.ConversationID {
		return false
	}
	if opts.UserID != "" && strings.TrimSpace(entry.UserID) != opts.UserID {
		return false
	}
	return rawEntryMatchesQuery(entry, opts.Query)
}

func rawEntryMatchesQuery(entry Entry, query string) bool {
	text := strings.ToLower(rawEntrySearchText(entry))
	if text == "" {
		return false
	}
	terms := searchTerms(query)
	if len(terms) == 0 {
		terms = []string{strings.ToLower(strings.TrimSpace(query))}
	}
	for _, term := range terms {
		if term == "" {
			continue
		}
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func rawEntrySearchText(entry Entry) string {
	return compactSearchText(strings.Join([]string{
		entry.Payload,
		entry.ToolName,
		entry.Source,
		entry.EventType,
		entry.Role,
	}, " "))
}

func buildRawSearchResults(entries []Entry, opts SearchOptions, existing []SearchConversationResult) []SearchConversationResult {
	if len(entries) == 0 {
		return nil
	}
	skip := make(map[string]struct{}, len(existing))
	for _, result := range existing {
		if strings.TrimSpace(result.ConversationID) == "" {
			continue
		}
		skip[result.ConversationID] = struct{}{}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if !entries[i].CreatedAt.Equal(entries[j].CreatedAt) {
			return entries[i].CreatedAt.After(entries[j].CreatedAt)
		}
		return strings.Compare(entries[i].ID, entries[j].ID) > 0
	})

	results := make([]SearchConversationResult, 0, opts.Limit)
	indexByConversation := make(map[string]int, opts.Limit)
	for _, entry := range entries {
		conversationID := strings.TrimSpace(entry.ConversationID)
		if conversationID == "" {
			continue
		}
		if _, exists := skip[conversationID]; exists {
			continue
		}
		idx, ok := indexByConversation[conversationID]
		if !ok {
			if len(results) >= opts.Limit {
				continue
			}
			idx = len(results)
			indexByConversation[conversationID] = idx
			results = append(results, SearchConversationResult{
				ConversationID: conversationID,
				SessionID:      strings.TrimSpace(entry.SessionID),
				UserID:         strings.TrimSpace(entry.UserID),
			})
		}

		result := &results[idx]
		result.HitCount++
		if entry.CreatedAt.After(result.LatestCreatedAt) {
			result.LatestCreatedAt = entry.CreatedAt
		}
		if len(result.Snippets) < opts.PerConversationLimit {
			result.Snippets = append(result.Snippets, SearchSnippet{
				AuditEntryID: strings.TrimSpace(entry.ID),
				Role:         strings.TrimSpace(entry.Role),
				EventType:    strings.TrimSpace(entry.EventType),
				ToolName:     strings.TrimSpace(entry.ToolName),
				Source:       strings.TrimSpace(entry.Source),
				Snippet:      buildSnippet(rawEntrySearchText(entry), opts.Query, opts.SnippetLength),
				CreatedAt:    entry.CreatedAt,
			})
		}
	}
	return results
}

func appendRawLogFile(path string, entries []Entry) error {
	if strings.TrimSpace(path) == "" || len(entries) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, entry := range entries {
		line, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		if _, err := writer.Write(line); err != nil {
			return err
		}
		if err := writer.WriteByte('\n'); err != nil {
			return err
		}
	}
	return writer.Flush()
}

func (s *Store) readRecentRawLog(conversationID string, limit int) ([]Entry, bool, error) {
	path := s.rawLogPath(conversationID)
	entries, found, err := readRawLogEntries(path, limit)
	if err != nil || !found {
		return nil, found, err
	}
	return entries, true, nil
}

func (s *Store) readRecentAllRawLogs(limit int) ([]Entry, error) {
	paths, err := s.rawLogPathsForSearch("")
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return []Entry{}, nil
	}
	all := make([]Entry, 0)
	for _, path := range paths {
		entries, found, err := readRawLogEntries(path, -1)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		all = append(all, entries...)
	}
	sort.SliceStable(all, func(i, j int) bool {
		if !all[i].CreatedAt.Equal(all[j].CreatedAt) {
			return all[i].CreatedAt.After(all[j].CreatedAt)
		}
		return strings.Compare(all[i].ID, all[j].ID) > 0
	})
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

func readRawLogEntries(path string, limit int) ([]Entry, bool, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, false, nil
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	defer file.Close()

	if limit == 0 {
		limit = -1
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), rawLogScannerMaxBytes)
	entries := make([]Entry, 0, maxInt(1, limit))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry Entry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// Tolerate a partially written tail line so readers can keep scanning while writers append.
			continue
		}
		entries = append(entries, entry)
		if limit > 0 && len(entries) > limit {
			copy(entries, entries[1:])
			entries = entries[:limit]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, true, err
	}
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	return entries, true, nil
}

func (s *Store) removeRawLogFile(conversationID string) {
	path := s.rawLogPath(conversationID)
	if path == "" {
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		logger.Warn().Err(err).Str("path", path).Msg("[sessionaudit] failed to remove raw jsonl sidecar")
		return
	}
	s.cleanupRawLogDirIfEmpty()
}

func (s *Store) pruneRawLogEntries(rows []sessionAuditIDRow) {
	if s == nil || len(rows) == 0 {
		return
	}
	grouped := make(map[string]map[string]struct{})
	for _, row := range rows {
		conversationID := strings.TrimSpace(row.ConversationID)
		entryID := strings.TrimSpace(row.ID)
		if conversationID == "" || entryID == "" {
			continue
		}
		if grouped[conversationID] == nil {
			grouped[conversationID] = make(map[string]struct{})
		}
		grouped[conversationID][entryID] = struct{}{}
	}
	for conversationID, removeIDs := range grouped {
		path := s.rawLogPath(conversationID)
		if path == "" {
			continue
		}
		entries, found, err := readRawLogEntries(path, -1)
		if err != nil {
			logger.Warn().Err(err).Str("path", path).Msg("[sessionaudit] failed to read raw jsonl sidecar during prune")
			continue
		}
		if !found {
			continue
		}
		kept := make([]Entry, 0, len(entries))
		for _, entry := range entries {
			if _, drop := removeIDs[strings.TrimSpace(entry.ID)]; drop {
				continue
			}
			kept = append(kept, entry)
		}
		if err := rewriteRawLogFile(path, kept); err != nil {
			logger.Warn().Err(err).Str("path", path).Msg("[sessionaudit] failed to rewrite raw jsonl sidecar during prune")
		}
	}
	s.cleanupRawLogDirIfEmpty()
}

func (s *Store) pruneRawLogFilesBefore(cutoff time.Time) error {
	paths, err := s.rawLogPathsForSearch("")
	if err != nil {
		return err
	}
	for _, path := range paths {
		entries, found, err := readRawLogEntries(path, -1)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		kept := make([]Entry, 0, len(entries))
		for _, entry := range entries {
			if !entry.CreatedAt.IsZero() && entry.CreatedAt.Before(cutoff) {
				continue
			}
			kept = append(kept, entry)
		}
		if err := rewriteRawLogFile(path, kept); err != nil {
			return err
		}
	}
	s.cleanupRawLogDirIfEmpty()
	return nil
}

func rewriteRawLogFile(path string, entries []Entry) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	if len(entries) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	writer := bufio.NewWriter(file)
	for _, entry := range entries {
		line, err := json.Marshal(entry)
		if err != nil {
			file.Close()
			return err
		}
		if _, err := writer.Write(line); err != nil {
			file.Close()
			return err
		}
		if err := writer.WriteByte('\n'); err != nil {
			file.Close()
			return err
		}
	}
	if err := writer.Flush(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func (s *Store) cleanupRawLogDirIfEmpty() {
	if s == nil || strings.TrimSpace(s.rawLogDir) == "" {
		return
	}
	if err := os.Remove(strings.TrimSpace(s.rawLogDir)); err != nil && !os.IsNotExist(err) && !os.IsExist(err) {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) && pathErr.Err == syscall.ENOTEMPTY {
			return
		}
		logger.Warn().Err(err).Str("path", s.rawLogDir).Msg("[sessionaudit] failed to remove empty raw audit dir")
	}
}

func (s *Store) deleteSearchProjectionIDs(ctx context.Context, ids []string) error {
	if s == nil || s.db == nil || len(ids) == 0 {
		return nil
	}
	holders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := make([]interface{}, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	if s.ftsEnabled {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM session_search_fts WHERE audit_entry_id IN (`+holders+`)`, args...); err != nil {
			return fmt.Errorf("delete pruned session search fts: %w", err)
		}
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM session_search_projections WHERE audit_entry_id IN (`+holders+`)`, args...); err != nil {
		return fmt.Errorf("delete pruned session search projections: %w", err)
	}
	return nil
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
			if s.db != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				_ = dbutil.CheckpointWAL(ctx, s.db, dbutil.CheckpointPassive)
				cancel()
			}
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

	s.subMu.Lock()
	for id, sub := range s.subs {
		if sub != nil {
			close(sub.ch)
		}
		delete(s.subs, id)
	}
	s.subMu.Unlock()

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
	ftsEnabled, err := initStoreSchema(s.db)
	if err != nil {
		return err
	}
	s.ftsEnabled = ftsEnabled
	return nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
