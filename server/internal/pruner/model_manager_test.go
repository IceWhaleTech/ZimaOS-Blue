package pruner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func skipOnnxModelManagerTests(t *testing.T) {
	t.Helper()
	t.Skip("ONNX cross-encoder has been removed; model manager tests temporarily disabled")
}

func TestModelManagerIsReady_Empty(t *testing.T) {
	skipOnnxModelManagerTests(t)

	dir := t.TempDir()
	mgr := NewPrunerModelManager(dir)
	if mgr.IsReady() {
		t.Error("empty dir should not be ready")
	}
}

func TestModelManagerIsReady_AllFiles(t *testing.T) {
	skipOnnxModelManagerTests(t)

	dir := t.TempDir()
	for _, f := range prunerModelFiles {
		os.WriteFile(filepath.Join(dir, f.Filename), []byte("data"), 0644)
	}
	mgr := NewPrunerModelManager(dir)
	if !mgr.IsReady() {
		t.Error("all files present should be ready")
	}
}

func TestModelManagerGetStatus(t *testing.T) {
	skipOnnxModelManagerTests(t)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "vocab.json"), []byte("{}"), 0644)
	mgr := NewPrunerModelManager(dir)

	status := mgr.GetStatus()
	if status.Ready {
		t.Error("should not be ready with only vocab.json")
	}
	if status.Downloading {
		t.Error("should not be downloading")
	}
	found := false
	for _, f := range status.Files {
		if f.Filename == "vocab.json" && f.Downloaded {
			found = true
		}
	}
	if !found {
		t.Error("vocab.json should show as downloaded")
	}
}

func TestModelManagerDownload(t *testing.T) {
	skipOnnxModelManagerTests(t)

	// Mock server serving fake model files
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "5")
		w.Write([]byte("hello"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	mgr := NewPrunerModelManager(dir)

	// Override URLs to point to mock server
	origFiles := make([]PrunerModelInfo, len(prunerModelFiles))
	copy(origFiles, prunerModelFiles)
	for i := range prunerModelFiles {
		prunerModelFiles[i].URL = srv.URL + "/" + prunerModelFiles[i].Filename
	}
	defer func() { copy(prunerModelFiles, origFiles) }()

	err := mgr.Download(context.Background())
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	if !mgr.IsReady() {
		t.Error("should be ready after download")
	}

	// Verify files exist with correct content
	for _, f := range prunerModelFiles {
		data, err := os.ReadFile(filepath.Join(dir, f.Filename))
		if err != nil {
			t.Errorf("file %s not found: %v", f.Filename, err)
			continue
		}
		if string(data) != "hello" {
			t.Errorf("file %s has wrong content: %q", f.Filename, data)
		}
	}

	// No .tmp files should remain
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("temp file should not remain: %s", e.Name())
		}
	}
}

func TestModelManagerDownloadSkipsExisting(t *testing.T) {
	skipOnnxModelManagerTests(t)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Write([]byte("data"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	// Pre-create all but one file
	origFiles := make([]PrunerModelInfo, len(prunerModelFiles))
	copy(origFiles, prunerModelFiles)
	for i := range prunerModelFiles {
		prunerModelFiles[i].URL = srv.URL + "/" + prunerModelFiles[i].Filename
	}
	defer func() { copy(prunerModelFiles, origFiles) }()

	for _, f := range prunerModelFiles[:len(prunerModelFiles)-1] {
		os.WriteFile(filepath.Join(dir, f.Filename), []byte("existing"), 0644)
	}

	mgr := NewPrunerModelManager(dir)
	err := mgr.Download(context.Background())
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 download (skip existing), got %d", callCount)
	}
}

func TestModelManagerCancelDownload(t *testing.T) {
	skipOnnxModelManagerTests(t)

	// Slow server that blocks
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000000")
		w.Write([]byte("start"))
		w.(http.Flusher).Flush()
		time.Sleep(10 * time.Second) // block
	}))
	defer srv.Close()

	dir := t.TempDir()
	mgr := NewPrunerModelManager(dir)

	origFiles := make([]PrunerModelInfo, len(prunerModelFiles))
	copy(origFiles, prunerModelFiles)
	for i := range prunerModelFiles {
		prunerModelFiles[i].URL = srv.URL + "/" + prunerModelFiles[i].Filename
	}
	defer func() { copy(prunerModelFiles, origFiles) }()

	done := make(chan error, 1)
	go func() {
		done <- mgr.Download(context.Background())
	}()

	// Wait for download to start
	time.Sleep(100 * time.Millisecond)
	mgr.CancelDownload()

	select {
	case err := <-done:
		if err == nil {
			t.Error("expected error after cancel")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancel did not stop download within 5s")
	}
}

func TestModelManagerDoubleDownload(t *testing.T) {
	skipOnnxModelManagerTests(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Write([]byte("data"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	mgr := NewPrunerModelManager(dir)

	origFiles := make([]PrunerModelInfo, len(prunerModelFiles))
	copy(origFiles, prunerModelFiles)
	for i := range prunerModelFiles {
		prunerModelFiles[i].URL = srv.URL + "/" + prunerModelFiles[i].Filename
	}
	defer func() { copy(prunerModelFiles, origFiles) }()

	go mgr.Download(context.Background())
	time.Sleep(50 * time.Millisecond)

	err := mgr.Download(context.Background())
	if err == nil {
		t.Error("expected error for concurrent download")
	}
}

func TestModelManagerProgress(t *testing.T) {
	skipOnnxModelManagerTests(t)

	dir := t.TempDir()
	mgr := NewPrunerModelManager(dir)

	status := mgr.GetStatus()
	if status.Progress != nil {
		t.Error("progress should be nil when not downloading")
	}
}
