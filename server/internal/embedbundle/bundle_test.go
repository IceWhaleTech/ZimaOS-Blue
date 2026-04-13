package embedbundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io/fs"
	"testing"
)

func TestLoadTarGzFSRoundTripsFiles(t *testing.T) {
	bundle := buildBundleForTest(t, map[string]string{
		"templates/en/SOUL.md":         "English soul",
		"templates/en/AGENTS_linux.md": "linux agents",
		"templates/fr/SOUL.md":         "French soul",
	})

	fsys, err := LoadTarGzFS(bundle, "templates", "templates/en/SOUL.md")
	if err != nil {
		t.Fatalf("LoadTarGzFS: %v", err)
	}

	entries, err := fs.ReadDir(fsys, "templates")
	if err != nil {
		t.Fatalf("ReadDir templates: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("locale dirs = %d, want 2", len(entries))
	}

	got, err := fs.ReadFile(fsys, "templates/fr/SOUL.md")
	if err != nil {
		t.Fatalf("ReadFile fr/SOUL.md: %v", err)
	}
	if string(got) != "French soul" {
		t.Fatalf("fr/SOUL.md = %q, want %q", string(got), "French soul")
	}
}

func TestLoadTarGzFSRejectsPathTraversal(t *testing.T) {
	bundle := buildBundleForTest(t, map[string]string{
		"../evil.txt": "nope",
	})

	if _, err := LoadTarGzFS(bundle, "templates"); err == nil {
		t.Fatal("expected invalid path error")
	}
}

func buildBundleForTest(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	tw := tar.NewWriter(gz)

	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}); err != nil {
			t.Fatalf("WriteHeader %s: %v", name, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("Write %s: %v", name, err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}

	return compressed.Bytes()
}
