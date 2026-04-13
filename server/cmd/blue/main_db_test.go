package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
)

func TestApplyPrimaryDatabasePoolConfig(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "pool.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer db.Close()

	applyPrimaryDatabasePoolConfig(&dbutil.SQLiteConn{Writer: db, Reader: db}, config.DatabasePerfConfig{
		PoolSize:        2,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Minute,
		ConnMaxIdleTime: 50 * time.Millisecond,
	})

	if got := db.Stats().MaxOpenConnections; got != 2 {
		t.Fatalf("MaxOpenConnections = %d, want 2", got)
	}

	conn1, err := db.Conn(context.Background())
	if err != nil {
		t.Fatalf("db.Conn() first error = %v", err)
	}
	conn2, err := db.Conn(context.Background())
	if err != nil {
		t.Fatalf("db.Conn() second error = %v", err)
	}
	conn1.Close()
	conn2.Close()

	if got := db.Stats().Idle; got < 1 {
		t.Fatalf("Idle after returning conns = %d, want >= 1", got)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := db.Ping(); err != nil {
			t.Fatalf("db.Ping() error = %v", err)
		}
		if got := db.Stats().Idle; got < 2 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("idle connections did not shrink before deadline; idle=%d", db.Stats().Idle)
}

func TestOpenPrimaryDatabaseWithStartupRecoveryDetectsCorruptionAfterCleanShutdown(t *testing.T) {
	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "blue.db")
	if err := os.WriteFile(dbPath, []byte("not-a-sqlite-database"), 0o600); err != nil {
		t.Fatalf("write corrupt db: %v", err)
	}

	dbutil.SetStartupQuickCheckEnabled(false)
	t.Cleanup(func() {
		dbutil.SetStartupQuickCheckEnabled(true)
	})

	conn, err := openPrimaryDatabaseWithStartupRecovery(dataDir, config.DatabasePerfConfig{
		WALMode:      true,
		PoolSize:     2,
		MaxIdleConns: 1,
	})
	if err != nil {
		t.Fatalf("openPrimaryDatabaseWithStartupRecovery() error = %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})

	if err := dbutil.QuickCheckDatabase(dbPath); err != nil {
		t.Fatalf("QuickCheckDatabase(%s) error = %v", dbPath, err)
	}

	backups, err := filepath.Glob(dbPath + ".bak.*")
	if err != nil {
		t.Fatalf("glob corrupt backups: %v", err)
	}
	if len(backups) == 0 {
		t.Fatalf("expected corrupt db backup for %s", dbPath)
	}
}

func TestOpenPrimaryDatabaseWithStartupRecoverySkipsOpenIntegrityChecksForWAL(t *testing.T) {
	dataDir := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(dataDir, "capture.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}

	orig := openPrimarySQLite
	t.Cleanup(func() {
		openPrimarySQLite = orig
	})
	openPrimarySQLite = func(path string, opts *dbutil.SQLiteOpenOpts) (*dbutil.SQLiteConn, error) {
		if got, want := path, filepath.Join(dataDir, "blue.db"); got != want {
			t.Fatalf("open path = %q, want %q", got, want)
		}
		if opts == nil {
			t.Fatal("expected sqlite open opts")
		}
		if !opts.SkipIntegrityCheckOnOpen {
			t.Fatal("expected primary WAL open to skip duplicate integrity checks on open")
		}
		if got, want := opts.MaxReaders, 7; got != want {
			t.Fatalf("MaxReaders = %d, want %d", got, want)
		}
		if got, want := opts.BusyTimeout, 5000; got != want {
			t.Fatalf("BusyTimeout = %d, want %d", got, want)
		}
		if got, want := opts.CacheSize, -4096; got != want {
			t.Fatalf("CacheSize = %d, want %d", got, want)
		}
		if !opts.ForeignKeys {
			t.Fatal("expected ForeignKeys to be enabled")
		}
		return &dbutil.SQLiteConn{Writer: db, Reader: db}, nil
	}

	dbutil.SetStartupQuickCheckEnabled(true)
	t.Cleanup(func() {
		dbutil.SetStartupQuickCheckEnabled(true)
	})

	conn, err := openPrimaryDatabaseWithStartupRecovery(dataDir, config.DatabasePerfConfig{
		WALMode:      true,
		PoolSize:     7,
		MaxIdleConns: 2,
		CacheSize:    4096,
	})
	if err != nil {
		t.Fatalf("openPrimaryDatabaseWithStartupRecovery() error = %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})
}
