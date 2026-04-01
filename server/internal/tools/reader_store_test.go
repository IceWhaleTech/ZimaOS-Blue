package tools

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestBrowserSiteAllowlistStore_UsesReaderDBForReads(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "browser-sites.db")
	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(write): %v", err)
	}

	if _, err := NewBrowserSiteAllowlistStore(writeDB); err != nil {
		_ = writeDB.Close()
		t.Fatalf("NewBrowserSiteAllowlistStore(bootstrap): %v", err)
	}

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("sql.Open(read): %v", err)
	}
	defer readDB.Close()

	store, err := NewBrowserSiteAllowlistStoreWithReadDB(writeDB, readDB)
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("NewBrowserSiteAllowlistStoreWithReadDB: %v", err)
	}
	if store.readDB == nil || store.readDB == store.db {
		_ = writeDB.Close()
		t.Fatal("expected separate reader db")
	}

	if err := store.Add("https://example.com/path", "user-1"); err != nil {
		_ = writeDB.Close()
		t.Fatalf("Add: %v", err)
	}

	if err := writeDB.Close(); err != nil {
		t.Fatalf("close writer db: %v", err)
	}

	if got := store.Match("https://example.com/settings", "user-1"); got == nil || got.Origin != "https://example.com" {
		t.Fatalf("unexpected browser site via reader db: %+v", got)
	}

	entries, err := store.List()
	if err != nil {
		t.Fatalf("List via reader db: %v", err)
	}
	if len(entries) != 1 || entries[0].Origin != "https://example.com" {
		t.Fatalf("unexpected browser site list via reader db: %+v", entries)
	}
}

func TestDirAllowlistStore_UsesReaderDBForReads(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "exec-dirs.db")
	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(write): %v", err)
	}

	if _, err := NewDirAllowlistStore(writeDB); err != nil {
		_ = writeDB.Close()
		t.Fatalf("NewDirAllowlistStore(bootstrap): %v", err)
	}

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("sql.Open(read): %v", err)
	}
	defer readDB.Close()

	store, err := NewDirAllowlistStoreWithReadDB(writeDB, readDB)
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("NewDirAllowlistStoreWithReadDB: %v", err)
	}
	if store.readDB == nil || store.readDB == store.db {
		_ = writeDB.Close()
		t.Fatal("expected separate reader db")
	}

	if err := store.Add("/tmp/reader", "user-1"); err != nil {
		_ = writeDB.Close()
		t.Fatalf("Add: %v", err)
	}

	if err := writeDB.Close(); err != nil {
		t.Fatalf("close writer db: %v", err)
	}

	if got := store.Match("/tmp/reader/subdir"); got == nil || got.Path != "/tmp/reader" {
		t.Fatalf("unexpected dir allowlist match via reader db: %+v", got)
	}

	entries, err := store.List()
	if err != nil {
		t.Fatalf("List via reader db: %v", err)
	}
	if len(entries) != 1 || entries[0].Path != "/tmp/reader" {
		t.Fatalf("unexpected dir allowlist entries via reader db: %+v", entries)
	}
}

func TestExecAuditStore_UsesReaderDBForReads(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "exec-audit.db")
	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(write): %v", err)
	}

	if _, err := NewExecAuditStore(writeDB); err != nil {
		_ = writeDB.Close()
		t.Fatalf("NewExecAuditStore(bootstrap): %v", err)
	}

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("sql.Open(read): %v", err)
	}
	defer readDB.Close()

	store, err := NewExecAuditStoreWithReadDB(writeDB, readDB)
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("NewExecAuditStoreWithReadDB: %v", err)
	}
	if store.readDB == nil || store.readDB == store.db {
		_ = writeDB.Close()
		t.Fatal("expected separate reader db")
	}

	exitCode := 0
	if err := store.Record(ExecAuditEntry{
		ID:         "reader-audit",
		Timestamp:  time.Now(),
		UserID:     "user-1",
		Command:    "echo reader",
		PolicyMode: "full",
		Decision:   "allowed",
		RiskLevel:  RiskLevelLow,
		ExitCode:   &exitCode,
	}); err != nil {
		_ = writeDB.Close()
		t.Fatalf("Record: %v", err)
	}

	if err := writeDB.Close(); err != nil {
		t.Fatalf("close writer db: %v", err)
	}

	entries, err := store.Recent(10)
	if err != nil {
		t.Fatalf("Recent via reader db: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != "reader-audit" {
		t.Fatalf("unexpected exec audit entries via reader db: %+v", entries)
	}
}
