package update

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

func newTestHandler(t *testing.T) (*Handler, string) {
	t.Helper()
	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "blue")
	os.WriteFile(binaryPath, []byte("binary"), 0755)

	cfg := &Config{
		StoragePath:    dir,
		BackupCount:    3,
		ReleaseChannel: ChannelStable,
	}
	h := &Handler{
		currentVersion: "0.1.0",
		config:         cfg,
		github:         NewGitHubClient(),
		downloader:     NewDownloader(dir),
		applier:        NewApplier(binaryPath, dir, 3),
		status:         &UpdateStatus{State: StateIdle},
	}
	return h, dir
}

func TestHandler_Apply_RespondsBeforeRestart(t *testing.T) {
	h, dir := newTestHandler(t)

	// Mock executor so we don't actually exec
	mock := &mockExecutor{}
	h.applier.SetExecutor(mock)

	// Simulate a downloaded update
	updatePath := filepath.Join(dir, "update.bin")
	os.WriteFile(updatePath, []byte("new-binary"), 0755)
	h.status.DownloadedPath = updatePath
	h.latestInfo = &UpdateInfo{LatestVersion: "0.2.0"}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/system/update/apply", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Apply(c)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	// Response should be 200 with "restarting"
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["status"] != "restarting" {
		t.Errorf("status = %q, want %q", resp["status"], "restarting")
	}
	if resp["version"] != "0.2.0" {
		t.Errorf("version = %q, want %q", resp["version"], "0.2.0")
	}

	// State should be restarting
	h.mu.RLock()
	state := h.status.State
	h.mu.RUnlock()
	if state != StateRestarting {
		t.Errorf("state = %q, want %q", state, StateRestarting)
	}
}

func TestHandler_Apply_NoDownload(t *testing.T) {
	h, _ := newTestHandler(t)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/system/update/apply", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Apply(c)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandler_Apply_AlreadyApplying(t *testing.T) {
	h, _ := newTestHandler(t)
	h.status.State = StateApplying
	h.status.DownloadedPath = "/some/path"

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/system/update/apply", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Apply(c)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestHandler_DownloadOTA_NoChecker(t *testing.T) {
	h, _ := newTestHandler(t)
	h.otaChecker = nil

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/system/update/download-ota", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.DownloadOTA(c)
	if err != nil {
		t.Fatalf("DownloadOTA returned error: %v", err)
	}

	// Should accept the request (download starts async)
	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}

	// But the async goroutine will set error since otaChecker is nil
	// Give it a moment
	// (In real tests we'd use a channel, but for now just verify the response)
}

func TestHandler_DownloadOTA_InProgress(t *testing.T) {
	h, _ := newTestHandler(t)
	h.status.State = StateDownloading

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/system/update/download-ota", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.DownloadOTA(c)
	if err != nil {
		t.Fatalf("DownloadOTA returned error: %v", err)
	}

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestHandler_Info(t *testing.T) {
	h, _ := newTestHandler(t)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/system/update/info", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Info(c)
	if err != nil {
		t.Fatalf("Info returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["current_version"] != "0.1.0" {
		t.Errorf("current_version = %v, want 0.1.0", resp["current_version"])
	}
}

func TestHandler_DownloadOTA_WithMockServer(t *testing.T) {
	h, dir := newTestHandler(t)

	// Create a mock OTA server that serves a binary
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("new-binary-content"))
	}))
	defer srv.Close()

	// Set up OTA checker with a cached response pointing to our mock server
	ota := NewOTAChecker("0.1.0", dir, "en_US")
	ota.mu.Lock()
	ota.latest = &OTAResponse{
		Version:        "0.2.0",
		Packages:       []string{srv.URL + "/blue.tar.gz"},
		ReleaseNoteURL: "https://example.com/blue/notes.md",
	}
	ota.mu.Unlock()
	h.otaChecker = ota

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/system/update/download-ota", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.DownloadOTA(c)
	if err != nil {
		t.Fatalf("DownloadOTA returned error: %v", err)
	}

	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}

	// Wait for async download to complete
	for i := 0; i < 50; i++ {
		h.mu.RLock()
		state := h.status.State
		errMsg := h.status.Error
		h.mu.RUnlock()
		if state == StateIdle {
			break
		}
		if state == StateFailed {
			t.Fatalf("download failed: %s", errMsg)
		}
		time.Sleep(50 * time.Millisecond)
	}

	h.mu.RLock()
	downloaded := h.status.DownloadedPath
	h.mu.RUnlock()

	if downloaded == "" {
		t.Error("expected downloaded path to be set")
	}
}
