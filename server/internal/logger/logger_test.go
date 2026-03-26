package logger

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

func TestInitWithMirrorWritesBothTargets(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	primary := filepath.Join(dir, "primary.log")
	mirror := filepath.Join(dir, "mirror.log")

	cfg := &config.LogConfig{
		Level:  "info",
		Format: "json",
		Output: primary,
	}
	if err := InitWithMirror(cfg, mirror); err != nil {
		t.Fatalf("InitWithMirror() error = %v", err)
	}

	Info().Str("scope", "test").Msg("mirror writes")

	primaryData, err := os.ReadFile(primary)
	if err != nil {
		t.Fatalf("ReadFile(primary) error = %v", err)
	}
	if !bytes.Contains(primaryData, []byte("mirror writes")) {
		t.Fatalf("primary log missing message: %s", string(primaryData))
	}

	mirrorData, err := os.ReadFile(mirror)
	if err != nil {
		t.Fatalf("ReadFile(mirror) error = %v", err)
	}
	if !bytes.Contains(mirrorData, []byte("mirror writes")) {
		t.Fatalf("mirror log missing message: %s", string(mirrorData))
	}
}

func TestInitWithMirrorSkipsDuplicateTarget(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "blue.log")

	cfg := &config.LogConfig{
		Level:  "info",
		Format: "json",
		Output: path,
	}
	if err := InitWithMirror(cfg, path); err != nil {
		t.Fatalf("InitWithMirror() error = %v", err)
	}

	Info().Msg("single write")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if bytes.Count(data, []byte("single write")) != 1 {
		t.Fatalf("expected exactly one copy of log entry, got %d in %s", bytes.Count(data, []byte("single write")), string(data))
	}
}
