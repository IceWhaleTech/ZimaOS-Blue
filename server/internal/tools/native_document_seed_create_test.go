package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaybeSeedNativeDocumentCreateFromSingleInputPath_ReadsSeedAndDerivesOutput(t *testing.T) {
	tmpDir := t.TempDir()
	scope := newFSToolScope([]string{tmpDir})

	if err := os.MkdirAll(filepath.Join(tmpDir, "reports"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "reports", "seed.md"), []byte("# Seed\n\nBody\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	args := map[string]interface{}{
		"path": "reports/seed.md",
	}
	if err := maybeSeedNativeDocumentCreateFromSingleInputPath(context.Background(), scope, "docx", args); err != nil {
		t.Fatalf("seed error = %v", err)
	}
	if got := strings.TrimSpace(asString(args["path"])); got != "reports/seed.docx" {
		t.Fatalf("path = %q, want reports/seed.docx", got)
	}
	if got := strings.TrimSpace(asString(args["markdown"])); !strings.Contains(got, "Seed") {
		t.Fatalf("markdown = %q, want seeded content", got)
	}
}

