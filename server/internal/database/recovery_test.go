package database_test

import (
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

type errString string

func (e errString) Error() string { return string(e) }
