package smallmodel

import (
	"os"
	"path/filepath"
	"testing"
)

func createReadyModelFiles(t *testing.T, m *Manager) {
	t.Helper()
	for _, f := range requiredModelFiles(m.assets) {
		p := filepath.Join(m.ModelDir(), f.Filename)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
		}
		if err := os.WriteFile(p, []byte("ok"), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
}
