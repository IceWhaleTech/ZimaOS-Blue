package bootstrap

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

func TestOpenPrimaryDatabaseRepairsRecoverableCorruptionWithoutLeavingBak(t *testing.T) {
	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "blue.db")
	writePrimarySQLiteEntries(t, dbPath, "hello")
	requirePrimarySQLiteRepairableSample(t, dbPath, 8192, []byte("garbagegarbagegarbagegarbage"))
	corruptPrimarySQLiteBytes(t, dbPath, 8192, []byte("garbagegarbagegarbagegarbage"))

	conn, err := openPrimaryDatabase(&ServerConfig{DataDir: dataDir}, zap.NewNop())
	if err != nil {
		t.Fatalf("openPrimaryDatabase() error = %v", err)
	}
	defer conn.Close()

	var count int
	if err := conn.Writer.QueryRow(`SELECT COUNT(*) FROM entries`).Scan(&count); err != nil {
		t.Fatalf("count repaired rows: %v", err)
	}
	if count == 0 {
		t.Fatal("expected repaired primary database to retain at least one row")
	}
	if err := dbutil.CheckDatabaseIntegrity(dbPath); err != nil {
		t.Fatalf("repaired primary database failed integrity check: %v", err)
	}
	if matches, err := filepath.Glob(dbPath + ".bak.*"); err != nil {
		t.Fatalf("glob backup files: %v", err)
	} else if len(matches) != 0 {
		t.Fatalf("expected successful primary repair to clean temporary .bak files, got %v", matches)
	}
}

func requirePrimarySQLiteRepairableSample(t *testing.T, path string, offset int64, payload []byte) {
	t.Helper()

	probeDir := t.TempDir()
	probePath := filepath.Join(probeDir, filepath.Base(path))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sqlite sample %s: %v", path, err)
	}
	if err := os.WriteFile(probePath, data, 0o644); err != nil {
		t.Fatalf("write sqlite repair probe %s: %v", probePath, err)
	}
	corruptPrimarySQLiteBytes(t, probePath, offset, payload)

	result, err := dbutil.RepairSQLiteDatabase(probePath)
	if err != nil {
		t.Skipf("sqlite3 CLI cannot repair this corruption sample in current environment: %v", err)
	}
	if result == nil || !result.Repaired {
		t.Skip("sqlite3 CLI did not report a successful repair for this corruption sample")
	}
}

func writePrimarySQLiteEntries(t *testing.T, path, value string) {
	t.Helper()

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open sqlite db %s: %v", path, err)
	}
	defer db.Close()

	if _, err := db.Exec(`DROP TABLE IF EXISTS entries`); err != nil {
		t.Fatalf("drop entries table in %s: %v", path, err)
	}
	if _, err := db.Exec(`CREATE TABLE entries (id INTEGER PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		t.Fatalf("create entries table in %s: %v", path, err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin insert transaction for %s: %v", path, err)
	}
	stmt, err := tx.Prepare(`INSERT INTO entries(id, value) VALUES (?, ?)`)
	if err != nil {
		t.Fatalf("prepare insert statement for %s: %v", path, err)
	}
	defer stmt.Close()

	for i := 1; i <= 2000; i++ {
		entryValue := value
		if i > 1 {
			entryValue = strings.Repeat("x", 200)
		}
		if _, err := stmt.Exec(i, entryValue); err != nil {
			_ = tx.Rollback()
			t.Fatalf("insert entry %d into %s: %v", i, path, err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit inserts for %s: %v", path, err)
	}
}

func corruptPrimarySQLiteBytes(t *testing.T, path string, offset int64, payload []byte) {
	t.Helper()

	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open sqlite db for corruption %s: %v", path, err)
	}
	defer f.Close()

	if _, err := f.WriteAt(payload, offset); err != nil {
		t.Fatalf("corrupt sqlite db %s: %v", path, err)
	}
}
