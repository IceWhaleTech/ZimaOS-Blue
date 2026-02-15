package database_test

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
)

func TestOpenSQLite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	conn, err := database.OpenSQLite(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Writer should work
	_, err = conn.Writer.Exec("CREATE TABLE t(id INTEGER PRIMARY KEY, val TEXT)")
	if err != nil {
		t.Fatal("writer create:", err)
	}
	_, err = conn.Writer.Exec("INSERT INTO t(val) VALUES('hello')")
	if err != nil {
		t.Fatal("writer insert:", err)
	}

	// Reader should see the data
	var val string
	err = conn.Reader.QueryRow("SELECT val FROM t WHERE id=1").Scan(&val)
	if err != nil {
		t.Fatal("reader query:", err)
	}
	if val != "hello" {
		t.Fatalf("expected 'hello', got %q", val)
	}

	// Reader should NOT be able to write
	_, err = conn.Reader.Exec("INSERT INTO t(val) VALUES('nope')")
	if err == nil {
		t.Fatal("reader should not be able to write")
	}

	// DB() backward compat
	if conn.DB() != conn.Writer {
		t.Fatal("DB() should return Writer")
	}
}

func TestOpenSQLiteSimple(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "simple.db")

	db, err := database.OpenSQLiteSimple(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE t(id INTEGER PRIMARY KEY)")
	if err != nil {
		t.Fatal(err)
	}

	stats := db.Stats()
	if stats.MaxOpenConnections != 2 {
		t.Fatalf("expected MaxOpenConns=2, got %d", stats.MaxOpenConnections)
	}
}

var _ = os.TempDir // suppress unused import
