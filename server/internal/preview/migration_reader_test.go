package preview

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestMigrationServiceUsesReaderDBForStatus(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "preview.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(write): %v", err)
	}
	defer writeDB.Close()

	if _, err := writeDB.Exec(`
		CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		t.Fatalf("create system_config: %v", err)
	}
	if _, err := writeDB.Exec(`INSERT INTO system_config (key, value) VALUES ('preview_data_migrated', 'true')`); err != nil {
		t.Fatalf("insert status: %v", err)
	}

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatalf("sql.Open(read): %v", err)
	}
	defer readDB.Close()

	svc := NewMigrationServiceWithReadDB(writeDB, readDB)
	if svc.readDB == nil || svc.readDB == svc.db {
		t.Fatal("expected separate reader db")
	}

	migrated, err := svc.GetMigrationStatus(context.Background())
	if err != nil {
		t.Fatalf("GetMigrationStatus: %v", err)
	}
	if !migrated {
		t.Fatal("expected migrated status from reader db")
	}
}
