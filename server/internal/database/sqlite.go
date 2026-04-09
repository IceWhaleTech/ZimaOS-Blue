package database

import (
	"database/sql"
	"fmt"
	"runtime"
	"time"
)

// SQLiteConn holds a read-write connection pair for SQLite.
// Writer has MaxOpenConns=1 (SQLite single-writer constraint).
// Reader allows concurrent reads via WAL mode.
type SQLiteConn struct {
	Writer *sql.DB // Single-writer connection (MaxOpenConns=1)
	Reader *sql.DB // Multi-reader connection pool
}

// DB returns the writer connection for backward compatibility.
func (c *SQLiteConn) DB() *sql.DB { return c.Writer }

// Close closes both connections.
func (c *SQLiteConn) Close() error {
	var errs []error
	if c.Reader != nil && c.Reader != c.Writer {
		if err := c.Reader.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if c.Writer != nil {
		if err := c.Writer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// SQLiteOpenOpts configures how a SQLite database is opened.
type SQLiteOpenOpts struct {
	// ReadOnly opens a single connection (no read-write split).
	ReadOnly bool
	// MaxReaders sets the reader pool size (default: 4).
	MaxReaders int
	// BusyTimeout in milliseconds (default: 5000).
	BusyTimeout int
	// CacheSize in pages (negative = KB). Default: -2000 (2MB).
	CacheSize int
	// ForeignKeys enables foreign key constraints (default: true).
	ForeignKeys bool
}

var defaultOpts = SQLiteOpenOpts{
	MaxReaders:  4,
	BusyTimeout: 5000,
	CacheSize:   -2000,
	ForeignKeys: true,
}

// OpenSQLite opens a SQLite database with read-write separation and optimal defaults.
// The writer connection is limited to 1 (SQLite constraint), while readers can be concurrent.
func OpenSQLite(path string, opts *SQLiteOpenOpts) (*SQLiteConn, error) {
	if opts == nil {
		opts = &defaultOpts
	}
	if opts.MaxReaders <= 0 {
		opts.MaxReaders = defaultOpts.MaxReaders
	}
	if opts.BusyTimeout <= 0 {
		opts.BusyTimeout = defaultOpts.BusyTimeout
	}
	if opts.CacheSize == 0 {
		opts.CacheSize = defaultOpts.CacheSize
	}

	// Open writer connection
	writer, err := openAndConfigure(path, path, false, opts)
	if err != nil {
		return nil, fmt.Errorf("open writer: %w", err)
	}
	writer.SetMaxOpenConns(1)
	writer.SetMaxIdleConns(1)
	writer.SetConnMaxLifetime(time.Hour)
	writer.SetConnMaxIdleTime(30 * time.Minute)

	conn := &SQLiteConn{Writer: writer}

	if opts.ReadOnly {
		// Single connection mode — reader is the same as writer
		conn.Reader = writer
		return conn, nil
	}

	// Open separate reader pool (file: URI required for mode=ro)
	readerDSN := fmt.Sprintf("file:%s?mode=ro", path)
	reader, err := openAndConfigure(readerDSN, path, true, opts)
	if err != nil {
		writer.Close()
		return nil, fmt.Errorf("open reader: %w", err)
	}
	reader.SetMaxOpenConns(opts.MaxReaders)
	reader.SetMaxIdleConns(opts.MaxReaders)
	reader.SetConnMaxLifetime(time.Hour)
	reader.SetConnMaxIdleTime(10 * time.Minute)

	conn.Reader = reader
	return conn, nil
}

// openAndConfigure opens a SQLite connection and applies standard PRAGMAs.
func openAndConfigure(dsn, dbPath string, readOnly bool, opts *SQLiteOpenOpts) (*sql.DB, error) {
	// Apply PRAGMAs — order matters
	pragmas := []string{
		fmt.Sprintf("PRAGMA busy_timeout=%d", opts.BusyTimeout),
		"PRAGMA journal_mode=WAL",
		fmt.Sprintf("PRAGMA cache_size=%d", opts.CacheSize),
		"PRAGMA synchronous=FULL",
	}
	if opts.ForeignKeys {
		pragmas = append(pragmas, "PRAGMA foreign_keys=ON")
	}
	if !readOnly {
		// Writer-only optimizations
		pragmas = append(pragmas, "PRAGMA wal_autocheckpoint=1000")
		if runtime.GOOS == "darwin" {
			pragmas = append(pragmas,
				"PRAGMA fullfsync=ON",
				"PRAGMA checkpoint_fullfsync=ON",
			)
		}
	}

	return OpenSQLiteWithRecovery(dsn, dbPath, func(db *sql.DB) error {
		for _, p := range pragmas {
			if _, err := db.Exec(p); err != nil {
				return fmt.Errorf("exec %q: %w", p, err)
			}
		}
		return integrityCheckOpenDatabase(db)
	})
}

// OpenSQLiteSimple opens a SQLite database with a single connection pool (no read-write split).
// Use this for auxiliary databases that don't need high concurrency.
func OpenSQLiteSimple(path string) (*sql.DB, error) {
	db, err := OpenSQLiteWithRecovery(path, path, func(db *sql.DB) error {
		db.SetMaxOpenConns(2)
		db.SetMaxIdleConns(2)
		db.SetConnMaxLifetime(time.Hour)
		db.SetConnMaxIdleTime(30 * time.Minute)

		pragmas := []string{
			"PRAGMA busy_timeout=5000",
			"PRAGMA journal_mode=WAL",
			"PRAGMA cache_size=-2000",
			"PRAGMA synchronous=FULL",
			"PRAGMA foreign_keys=ON",
			"PRAGMA wal_autocheckpoint=1000",
		}
		if runtime.GOOS == "darwin" {
			pragmas = append(pragmas,
				"PRAGMA fullfsync=ON",
				"PRAGMA checkpoint_fullfsync=ON",
			)
		}
		for _, p := range pragmas {
			if _, err := db.Exec(p); err != nil {
				return fmt.Errorf("exec %q: %w", p, err)
			}
		}
		return integrityCheckOpenDatabase(db)
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}
