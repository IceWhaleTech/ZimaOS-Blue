package mediagen

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestSQLiteConfigStore_UsesReaderDBForReads(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mediagen-config.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(write): %v", err)
	}

	bootstrapStore, err := NewSQLiteConfigStore(writeDB)
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("NewSQLiteConfigStore(bootstrap): %v", err)
	}

	if err := bootstrapStore.Save(map[string]*MediaProviderConfig{
		"reader-provider": {
			ID:       "reader-provider",
			Enabled:  true,
			BaseURL:  "https://reader.example.com",
			APIKey:   "reader-secret",
			Priority: 7,
		},
	}); err != nil {
		_ = writeDB.Close()
		t.Fatalf("Save: %v", err)
	}

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("sql.Open(read): %v", err)
	}
	defer readDB.Close()

	store, err := NewSQLiteConfigStoreWithReadDB(writeDB, readDB)
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("NewSQLiteConfigStoreWithReadDB: %v", err)
	}
	if store.readDB == nil || store.readDB == store.db {
		_ = writeDB.Close()
		t.Fatal("expected separate reader db")
	}

	if err := writeDB.Close(); err != nil {
		t.Fatalf("close writer db: %v", err)
	}

	configs, err := store.Load()
	if err != nil {
		t.Fatalf("Load via reader db: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("Load len = %d, want 1", len(configs))
	}

	cfg := configs["reader-provider"]
	if cfg == nil {
		t.Fatal("expected reader-provider config")
	}
	if !cfg.Enabled || cfg.BaseURL != "https://reader.example.com" || cfg.APIKey != "reader-secret" {
		t.Fatalf("unexpected config via reader db: %+v", cfg)
	}
}
