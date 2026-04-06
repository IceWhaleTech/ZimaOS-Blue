package memory

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	z "github.com/IceWhaleTech/zorm"
)

const (
	defaultStoreBusyTimeout                 = 5000
	defaultStoreCacheSizePages              = 500
	defaultStoreRuntimeStateCleanupInterval = 10 * time.Minute
)

// DefaultChatAttachmentDir returns the canonical external attachment directory
// under the runtime data dir.
func DefaultChatAttachmentDir(dataDir string) string {
	trimmed := strings.TrimSpace(dataDir)
	if trimmed == "" {
		return filepath.Join("media", "message_attachments")
	}
	return filepath.Join(trimmed, "media", "message_attachments")
}

func defaultChatAttachmentDirFromDBPath(dbPath string) string {
	trimmed := strings.TrimSpace(dbPath)
	if trimmed == "" || trimmed == ":memory:" {
		return ""
	}
	return DefaultChatAttachmentDir(filepath.Dir(trimmed))
}

// StoreOptions controls owned SQLite chat-store behavior.
type StoreOptions struct {
	Durability                  string
	WALAutoCheckpoint           int
	CheckpointInterval          time.Duration
	RuntimeStateCleanupInterval time.Duration
	AttachmentExternalStore     bool
	AttachmentDir               string
	MaxOpenConns                int
	MaxIdleConns                int
}

// DefaultStoreOptions returns conservative defaults for generic callers.
func DefaultStoreOptions() StoreOptions {
	return StoreOptions{
		Durability:        "full",
		WALAutoCheckpoint: 1000,
		MaxOpenConns:      1,
		MaxIdleConns:      1,
	}
}

// DefaultChatStoreOptions returns chat-optimized defaults for a dedicated handle.
func DefaultChatStoreOptions(dbPath string) StoreOptions {
	opts := StoreOptions{
		Durability:                  "normal",
		WALAutoCheckpoint:           4000,
		CheckpointInterval:          60 * time.Second,
		RuntimeStateCleanupInterval: defaultStoreRuntimeStateCleanupInterval,
		MaxOpenConns:                4,
		MaxIdleConns:                4,
	}
	if strings.TrimSpace(dbPath) != "" && strings.TrimSpace(dbPath) != ":memory:" {
		opts.AttachmentDir = defaultChatAttachmentDirFromDBPath(dbPath)
	}
	return opts
}

func normalizeStoreOptions(dbPath string, opts StoreOptions) StoreOptions {
	if strings.TrimSpace(opts.Durability) == "" {
		opts.Durability = "full"
	}
	opts.Durability = strings.ToLower(strings.TrimSpace(opts.Durability))
	switch opts.Durability {
	case "normal", "full":
	default:
		opts.Durability = "full"
	}
	if opts.WALAutoCheckpoint <= 0 {
		opts.WALAutoCheckpoint = 1000
	}
	if opts.MaxOpenConns <= 0 {
		opts.MaxOpenConns = 1
	}
	if opts.MaxIdleConns <= 0 {
		opts.MaxIdleConns = opts.MaxOpenConns
	}
	if opts.MaxIdleConns > opts.MaxOpenConns {
		opts.MaxIdleConns = opts.MaxOpenConns
	}
	if opts.RuntimeStateCleanupInterval < 0 {
		opts.RuntimeStateCleanupInterval = 0
	}
	if opts.AttachmentExternalStore && strings.TrimSpace(opts.AttachmentDir) == "" {
		if trimmed := strings.TrimSpace(dbPath); trimmed != "" && trimmed != ":memory:" {
			opts.AttachmentDir = defaultChatAttachmentDirFromDBPath(trimmed)
		}
	}
	if strings.TrimSpace(dbPath) == "" || strings.TrimSpace(dbPath) == ":memory:" {
		opts.CheckpointInterval = 0
		opts.AttachmentExternalStore = opts.AttachmentExternalStore && strings.TrimSpace(opts.AttachmentDir) != ""
	}
	return opts
}

func openOwnedStoreDB(dbPath string, opts StoreOptions) (*sql.DB, error) {
	opts = normalizeStoreOptions(dbPath, opts)
	return dbutil.OpenSQLiteWithRecoveryAndRecreate(dbPath, dbPath, func(db *sql.DB) error {
		db.SetMaxOpenConns(opts.MaxOpenConns)
		db.SetMaxIdleConns(opts.MaxIdleConns)

		pragmas := []string{
			"PRAGMA journal_mode=WAL",
			"PRAGMA foreign_keys=ON",
			fmt.Sprintf("PRAGMA busy_timeout=%d", defaultStoreBusyTimeout),
			fmt.Sprintf("PRAGMA wal_autocheckpoint=%d", opts.WALAutoCheckpoint),
			fmt.Sprintf("PRAGMA cache_size=-%d", defaultStoreCacheSizePages),
		}
		switch opts.Durability {
		case "normal":
			pragmas = append(pragmas, "PRAGMA synchronous=NORMAL")
		default:
			pragmas = append(pragmas, "PRAGMA synchronous=FULL")
		}
		for _, pragma := range pragmas {
			if _, err := db.Exec(pragma); err != nil {
				return fmt.Errorf("exec %q: %w", pragma, err)
			}
		}

		store := &Store{db: db, options: opts}
		if err := store.migrate(); err != nil {
			return fmt.Errorf("failed to migrate: %w", err)
		}
		db.Exec("PRAGMA shrink_memory")
		return nil
	})
}

func (s *Store) startOwnedLoops() {
	if s == nil || !s.ownsDB || s.bgStarted || s.bgDone == nil {
		return
	}
	s.bgStarted = true
	s.bgWG.Add(1)
	go s.runOwnedLoops()
}

func (s *Store) runOwnedLoops() {
	defer s.bgWG.Done()

	var (
		checkpointTicker *time.Ticker
		cleanupTicker    *time.Ticker
	)
	if s.options.CheckpointInterval > 0 {
		checkpointTicker = time.NewTicker(s.options.CheckpointInterval)
		defer checkpointTicker.Stop()
	}
	if s.options.RuntimeStateCleanupInterval > 0 {
		cleanupTicker = time.NewTicker(s.options.RuntimeStateCleanupInterval)
		defer cleanupTicker.Stop()
	}

	for {
		select {
		case <-s.bgDone:
			return
		case <-tickerChan(checkpointTicker):
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_ = s.checkpoint(ctx, dbutil.CheckpointPassive)
			cancel()
		case <-tickerChan(cleanupTicker):
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_ = s.CleanupExpiredRuntimeState(ctx)
			cancel()
		}
	}
}

func tickerChan(t *time.Ticker) <-chan time.Time {
	if t == nil {
		return nil
	}
	return t.C
}

func (s *Store) stopOwnedLoops() {
	if s == nil || !s.bgStarted || s.bgDone == nil {
		return
	}
	s.bgCloseOnce.Do(func() {
		close(s.bgDone)
	})
	s.bgWG.Wait()
}

func (s *Store) checkpoint(ctx context.Context, mode dbutil.CheckpointMode) error {
	if s == nil || s.db == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return dbutil.CheckpointWAL(ctx, s.db, mode)
}

// CleanupExpiredRuntimeState prunes stale previous_response_id rows.
func (s *Store) CleanupExpiredRuntimeState(ctx context.Context) error {
	if s == nil || s.db == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cutoff := time.Now().UTC().Add(-responsesPreviousIDTTL)
	_, err := s.conversationRuntimeState(ctx).Delete(z.Where(z.Lt("updated_at", cutoff)))
	if err != nil {
		return fmt.Errorf("cleanup expired conversation runtime state: %w", err)
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

// Recover attempts to reopen an owned SQLite store using the shared recovery flow.
func (s *Store) Recover() error {
	if s == nil || !s.ownsDB || strings.TrimSpace(s.dbPath) == "" || strings.TrimSpace(s.dbPath) == ":memory:" {
		return fmt.Errorf("store recovery is unavailable")
	}

	s.recoveryMu.Lock()
	defer s.recoveryMu.Unlock()

	if s.db != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = s.checkpoint(ctx, dbutil.CheckpointTruncate)
		cancel()
		_ = s.db.Close()
	}
	if s.readDB != nil && s.readDB != s.db {
		_ = s.readDB.Close()
	}

	db, err := openOwnedStoreDB(s.dbPath, s.options)
	if err != nil {
		return err
	}
	s.db = db
	readDB, readErr := openStoreReaderDB(s.dbPath)
	if readErr != nil || readDB == nil {
		readDB = db
	}
	s.readDB = readDB
	return nil
}
