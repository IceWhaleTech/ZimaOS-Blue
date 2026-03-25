package browser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLightpandaBinaryManagerUsesThreeNonJsdelivrSources(t *testing.T) {
	manager := NewLightpandaBinaryManager(DefaultConfig())
	sources := manager.downloadSources()
	if len(sources) != 3 {
		t.Fatalf("downloadSources() = %d sources, want 3", len(sources))
	}
	for _, source := range sources {
		if strings.Contains(strings.ToLower(source), "jsdelivr") {
			t.Fatalf("download source %q should not use jsdelivr", source)
		}
	}
}

func TestLightpandaBinaryManagerReadyPathUsesExplicitBinary(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Lightpanda.Enabled = true
	binaryPath := filepath.Join(t.TempDir(), "lightpanda")
	if err := os.WriteFile(binaryPath, []byte("stub"), 0o755); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", binaryPath, err)
	}
	cfg.Lightpanda.BinaryPath = binaryPath

	manager := NewLightpandaBinaryManager(cfg)
	got, ok, err := manager.ReadyPath()
	if err != nil {
		t.Fatalf("ReadyPath() error = %v", err)
	}
	if !ok {
		t.Fatal("ReadyPath() reported binary unavailable, want ready")
	}
	if got != binaryPath {
		t.Fatalf("ReadyPath() = %q, want %q", got, binaryPath)
	}
}
