package pruner

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	z "github.com/IceWhaleTech/zorm"
	_ "github.com/mattn/go-sqlite3"
)

// DiskCache provides SQLite-backed cache persistence with TTL expiration.
// Reuses the same SQLite driver (mattn/go-sqlite3) as the proxy's CCCache.
type DiskCache struct {
	db     *sql.DB
	readDB *sql.DB
	ttl    time.Duration
	mu     sync.RWMutex
}

// NewDiskCache creates a SQLite-backed disk cache at the given path.
// If dbPath is empty or the database cannot be opened, returns a no-op cache.
func NewDiskCache(dbPath string, ttl time.Duration) *DiskCache {
	if dbPath == "" {
		return &DiskCache{ttl: ttl}
	}

	db, err := dbutil.OpenSQLiteWithRecoveryAndRecreate(dbPath, dbPath, func(db *sql.DB) error {
		db.SetMaxOpenConns(2)
		db.SetMaxIdleConns(1)
		if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
			return err
		}
		if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
			return err
		}
		if _, err := db.Exec("PRAGMA synchronous=FULL"); err != nil {
			return err
		}
		if _, err := db.Exec("PRAGMA wal_autocheckpoint=1000"); err != nil {
			return err
		}

		dc := &DiskCache{db: db}
		return dc.initSchema()
	})
	if err != nil {
		return &DiskCache{ttl: ttl}
	}

	readDB, readErr := openPrunerCacheReaderDB(dbPath)
	if readErr != nil || readDB == nil {
		readDB = db
	}

	return &DiskCache{db: db, readDB: readDB, ttl: ttl}
}

func (d *DiskCache) initSchema() error {
	_, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS pruner_cache (
			key TEXT PRIMARY KEY,
			data BLOB NOT NULL,
			created_at DATETIME NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_pruner_cache_created ON pruner_cache(created_at);
	`)
	return err
}

func (d *DiskCache) table(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, d.db, "pruner_cache")
}

func (d *DiskCache) readTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, d.reader(), "pruner_cache")
}

func (d *DiskCache) reader() *sql.DB {
	if d != nil && d.readDB != nil {
		return d.readDB
	}
	if d == nil {
		return nil
	}
	return d.db
}

func openPrunerCacheReaderDB(dbPath string) (*sql.DB, error) {
	if dbPath == "" || dbPath == ":memory:" {
		return nil, nil
	}
	dsn := fmt.Sprintf("file:%s?mode=ro", dbPath)
	db, err := dbutil.OpenSQLiteWithRecovery(dsn, dbPath, func(db *sql.DB) error {
		db.SetMaxOpenConns(4)
		db.SetMaxIdleConns(2)
		if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
			return fmt.Errorf("set pruner cache reader busy timeout: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}

type prunerCacheRow struct {
	Key       string `json:"key" zorm:"key"`
	Data      string `json:"data" zorm:"data"`
	CreatedAt string `json:"created_at" zorm:"created_at"`
}

// Get retrieves scored segments from disk cache.
func (d *DiskCache) Get(key string) ([]ScoredSegment, bool) {
	if d.db == nil {
		return nil, false
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	ctx := context.Background()
	var rows []prunerCacheRow
	_, err := d.readTable(ctx).Select(&rows,
		z.Where(z.Eq("key", key)),
		z.Limit(1),
	)
	if err != nil || len(rows) == 0 {
		return nil, false
	}

	// Check TTL
	ca := parsePrunerCacheTime(rows[0].CreatedAt)
	if ca.IsZero() || timeutil.SinceTime(ca) > d.ttl {
		go func() {
			d.mu.Lock()
			defer d.mu.Unlock()
			deleteCtx := context.Background()
			_, _ = d.table(deleteCtx).Delete(z.Where(z.Eq("key", key)))
		}()
		return nil, false
	}

	var segments []ScoredSegment
	if err := json.Unmarshal([]byte(rows[0].Data), &segments); err != nil {
		return nil, false
	}

	return segments, true
}

// Put stores scored segments to disk cache.
func (d *DiskCache) Put(key string, segments []ScoredSegment) {
	if d.db == nil {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	data, err := json.Marshal(segments)
	if err != nil {
		return
	}

	ctx := context.Background()
	_, _ = d.table(ctx).Insert(
		map[string]interface{}{
			"key":        key,
			"data":       string(data),
			"created_at": timeutil.NowTime().UTC().Format(time.RFC3339),
		},
		z.OnConflictDoUpdateSet(
			[]string{"key"},
			[]string{"data", "created_at"},
		),
	)
}

// Cleanup removes expired entries.
func (d *DiskCache) Cleanup() int64 {
	if d.db == nil {
		return 0
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	cutoff := timeutil.NowTime().Add(-d.ttl).UTC().Format(time.RFC3339)
	ctx := context.Background()
	result, err := d.table(ctx).Delete(z.Where(z.Expr("created_at <= ?", cutoff)))
	if err != nil {
		return 0
	}
	return int64(result)
}

// Close closes the database connection.
func (d *DiskCache) Close() error {
	if d.db == nil {
		return nil
	}
	if d.readDB != nil && d.readDB != d.db {
		_ = d.readDB.Close()
	}
	return d.db.Close()
}

func parsePrunerCacheTime(raw string) time.Time {
	parsed, err := time.Parse(time.RFC3339, raw)
	if err == nil {
		return parsed
	}
	parsed, err = time.Parse(time.RFC3339Nano, raw)
	if err == nil {
		return parsed
	}
	parsed, _ = time.Parse("2006-01-02 15:04:05", raw)
	return parsed
}
