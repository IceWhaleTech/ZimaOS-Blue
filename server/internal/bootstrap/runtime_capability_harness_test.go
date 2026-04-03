package bootstrap

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

func TestNewHarnessRuntimeBundle_WiresRunTracer(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "runtime-harness.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{}
	cfg.Harness.Enabled = true
	cfg.Harness.StorePath = filepath.Join(tmp, "blue.db")
	cfg.Harness.ArtifactRoot = filepath.Join(tmp, "artifacts")

	bundle, err := newHarnessRuntimeBundle(db, cfg, nil)
	if err != nil {
		t.Fatalf("newHarnessRuntimeBundle failed: %v", err)
	}
	if bundle == nil || bundle.RunTracer == nil {
		t.Fatalf("expected run tracer wiring, got %#v", bundle)
	}
	if middlewares := bundle.Controller.ExecutionMiddlewares(); len(middlewares) == 0 {
		t.Fatalf("expected run tracer middleware to be registered, got %#v", middlewares)
	}
	if _, err := bundle.Controller.RunTraceSnapshot(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "run_id is required") {
		t.Fatalf("expected run trace provider to be wired, got err=%v", err)
	}
}

func TestNewHarnessRuntimeBundle_WiresSkillCandidateMiddleware(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "runtime-harness-skill.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{}
	cfg.Harness.Enabled = true
	cfg.Harness.StorePath = filepath.Join(tmp, "blue.db")
	cfg.Harness.ArtifactRoot = filepath.Join(tmp, "artifacts")

	bundle, err := newHarnessRuntimeBundle(db, cfg, nil)
	if err != nil {
		t.Fatalf("newHarnessRuntimeBundle failed: %v", err)
	}
	if bundle == nil || bundle.Controller == nil {
		t.Fatalf("expected controller wiring, got %#v", bundle)
	}
	if middlewares := bundle.Controller.ExecutionMiddlewares(); len(middlewares) < 2 {
		t.Fatalf("expected run tracer plus skill candidate middleware, got %#v", middlewares)
	}
}
