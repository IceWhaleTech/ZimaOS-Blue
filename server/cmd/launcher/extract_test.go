package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractBluecli_ExtractsGzipPayload(t *testing.T) {
	t.Helper()

	original := embeddedBluecli
	t.Cleanup(func() {
		embeddedBluecli = original
	})

	want := []byte("#!/bin/sh\necho blue\n")
	embeddedBluecli = buildTestGzipPayload(t, want)

	dst := filepath.Join(t.TempDir(), ".bluecli")
	if err := extractBluecli(dst); err != nil {
		t.Fatalf("extractBluecli(gzip) error = %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read extracted bluecli: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("extracted bluecli = %q, want %q", string(got), string(want))
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat extracted bluecli: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("extracted bluecli perms = %o, want 755", info.Mode().Perm())
	}
}

func TestEmbeddedVersion_HashesExtractedBluecliBytes(t *testing.T) {
	t.Helper()

	original := embeddedBluecli
	t.Cleanup(func() {
		embeddedBluecli = original
	})

	payload := []byte("#!/bin/sh\necho versioned\n")
	embeddedBluecli = buildTestGzipPayload(t, payload)

	sum := sha256.Sum256(payload)
	want := hex.EncodeToString(sum[:8])
	if got := embeddedVersion(); got != want {
		t.Fatalf("embeddedVersion() = %q, want %q", got, want)
	}
}

func TestExtractDist_ExtractsTarGzPayload(t *testing.T) {
	t.Helper()

	original := embeddedDist
	t.Cleanup(func() {
		embeddedDist = original
	})

	embeddedDist = buildTestTarArchive(t, true, map[string]string{
		"index.html":      "<html>ok</html>",
		"assets/app.js":   "console.log('ok')",
		"assets/app.css":  "body{color:black}",
		"nested/info.txt": "hello",
	})

	dst := t.TempDir()
	if err := extractDist(dst); err != nil {
		t.Fatalf("extractDist(tar.gz) error = %v", err)
	}

	assertExtractedFile(t, dst, "index.html", "<html>ok</html>")
	assertExtractedFile(t, dst, filepath.Join("assets", "app.js"), "console.log('ok')")
	assertExtractedFile(t, dst, filepath.Join("nested", "info.txt"), "hello")
}

func TestExtractDist_ExtractsTarPayload(t *testing.T) {
	t.Helper()

	original := embeddedDist
	t.Cleanup(func() {
		embeddedDist = original
	})

	embeddedDist = buildTestTarArchive(t, false, map[string]string{
		"index.html":    "<html>tar</html>",
		"assets/app.js": "console.log('tar')",
	})

	dst := t.TempDir()
	if err := extractDist(dst); err != nil {
		t.Fatalf("extractDist(tar) error = %v", err)
	}

	assertExtractedFile(t, dst, "index.html", "<html>tar</html>")
	assertExtractedFile(t, dst, filepath.Join("assets", "app.js"), "console.log('tar')")
}

func buildTestTarArchive(t *testing.T, gzipEnabled bool, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	var tarWriter *tar.Writer
	var gzipWriter *gzip.Writer

	if gzipEnabled {
		gzipWriter = gzip.NewWriter(&buf)
		tarWriter = tar.NewWriter(gzipWriter)
	} else {
		tarWriter = tar.NewWriter(&buf)
	}

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := tarWriter.WriteHeader(hdr); err != nil {
			t.Fatalf("write tar header %s: %v", name, err)
		}
		if _, err := tarWriter.Write([]byte(content)); err != nil {
			t.Fatalf("write tar file %s: %v", name, err)
		}
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if gzipWriter != nil {
		if err := gzipWriter.Close(); err != nil {
			t.Fatalf("close gzip writer: %v", err)
		}
	}

	return buf.Bytes()
}

func buildTestGzipPayload(t *testing.T, payload []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(payload); err != nil {
		t.Fatalf("write gzip payload: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close gzip payload: %v", err)
	}
	return buf.Bytes()
}

func assertExtractedFile(t *testing.T, dst, relativePath, want string) {
	t.Helper()

	got, err := os.ReadFile(filepath.Join(dst, relativePath))
	if err != nil {
		t.Fatalf("read extracted file %s: %v", relativePath, err)
	}
	if string(got) != want {
		t.Fatalf("extracted %s = %q, want %q", relativePath, string(got), want)
	}
}
