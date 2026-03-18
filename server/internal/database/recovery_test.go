package database_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	_ "github.com/mattn/go-sqlite3"
)

func TestIsSQLiteCorruptionError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "malformed", err: os.ErrInvalid, want: false},
		{name: "disk image malformed", err: errString("database disk image is malformed"), want: true},
		{name: "file is not database", err: errString("file is not a database"), want: true},
		{name: "sqlite corrupt", err: errString("SQLITE_CORRUPT: page checksum mismatch"), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := database.IsSQLiteCorruptionError(tt.err)
			if got != tt.want {
				t.Fatalf("IsSQLiteCorruptionError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpenSQLiteSimpleWrapsCorruptionError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.db")
	if err := os.WriteFile(path, []byte("not-a-sqlite-database"), 0o600); err != nil {
		t.Fatalf("write corrupt db: %v", err)
	}

	db, err := database.OpenSQLiteSimple(path)
	if db != nil {
		_ = db.Close()
		t.Fatal("expected open to fail for corrupt sqlite file")
	}
	if err == nil {
		t.Fatal("expected corruption error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "appears corrupted") {
		t.Fatalf("expected wrapped corruption error, got %v", err)
	}
}

func TestOpenSQLiteSimpleRepairsRecoverableCorruption(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "recoverable.db")
	writeSQLiteEntry(t, path, "hello")
	corruptSQLiteBytes(t, path, 8192, []byte("garbagegarbagegarbagegarbage"))

	db, err := database.OpenSQLiteSimple(path)
	if err != nil {
		t.Fatalf("expected recoverable db to open after repair, got %v", err)
	}
	defer db.Close()

	var got string
	if err := db.QueryRow("SELECT value FROM entries WHERE id = 1").Scan(&got); err != nil {
		t.Fatalf("failed to read repaired db: %v", err)
	}
	if got != "hello" {
		t.Fatalf("unexpected repaired value %q", got)
	}
	if err := database.CheckDatabaseIntegrity(path); err != nil {
		t.Fatalf("repaired db failed integrity check: %v", err)
	}
	if matches, err := filepath.Glob(path + ".corrupt.*"); err != nil {
		t.Fatalf("glob repair backup: %v", err)
	} else if len(matches) != 1 {
		t.Fatalf("expected one preserved corrupt backup, got %v", matches)
	}
}

func writeSQLiteEntry(t *testing.T, path, value string) {
	t.Helper()

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open sqlite db %s: %v", path, err)
	}
	defer db.Close()

	if _, err := db.Exec("DROP TABLE IF EXISTS entries"); err != nil {
		t.Fatalf("drop entries table in %s: %v", path, err)
	}
	if _, err := db.Exec("CREATE TABLE entries (id INTEGER PRIMARY KEY, value TEXT NOT NULL)"); err != nil {
		t.Fatalf("create entries table in %s: %v", path, err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin insert transaction for %s: %v", path, err)
	}
	stmt, err := tx.Prepare("INSERT INTO entries(id, value) VALUES (?, ?)")
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

func corruptSQLiteBytes(t *testing.T, path string, offset int64, payload []byte) {
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

type errString string

func (e errString) Error() string { return string(e) }
