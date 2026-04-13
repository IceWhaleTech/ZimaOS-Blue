package database

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestOpenSQLiteRunsIntegrityCheckOnOpenByDefault(t *testing.T) {
	orig := runIntegrityCheckOpenDatabase
	t.Cleanup(func() {
		runIntegrityCheckOpenDatabase = orig
	})

	var calls int
	runIntegrityCheckOpenDatabase = func(db *sql.DB) error {
		calls++
		return nil
	}

	conn, err := OpenSQLite(filepath.Join(t.TempDir(), "test.db"), &SQLiteOpenOpts{ReadOnly: true})
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})

	if calls != 1 {
		t.Fatalf("integrity check calls = %d, want 1", calls)
	}
}

func TestOpenSQLiteSkipsIntegrityCheckOnOpenWhenRequested(t *testing.T) {
	orig := runIntegrityCheckOpenDatabase
	t.Cleanup(func() {
		runIntegrityCheckOpenDatabase = orig
	})

	var calls int
	runIntegrityCheckOpenDatabase = func(db *sql.DB) error {
		calls++
		return nil
	}

	conn, err := OpenSQLite(filepath.Join(t.TempDir(), "test.db"), &SQLiteOpenOpts{
		ReadOnly:                 true,
		SkipIntegrityCheckOnOpen: true,
	})
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})

	if calls != 0 {
		t.Fatalf("integrity check calls = %d, want 0", calls)
	}
}
