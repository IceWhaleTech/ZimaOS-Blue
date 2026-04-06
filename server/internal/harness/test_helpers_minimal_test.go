package harness

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

func newMinimalTestController(t *testing.T) *Controller {
	t.Helper()
	tmpDir := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmpDir, "harness-test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store, err := NewSQLiteStore(db)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	harnessCfg := *config.DefaultHarnessConfig()
	harnessCfg.StorePath = filepath.Join(tmpDir, "blue.db")
	harnessCfg.ArtifactRoot = filepath.Join(tmpDir, "artifacts")
	return NewController(store, NewPolicyResolver(harnessCfg, nil))
}
