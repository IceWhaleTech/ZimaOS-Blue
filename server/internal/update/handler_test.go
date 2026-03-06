package update

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

func newTCP4Server(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("skip test server setup (tcp4 unavailable): %v", err)
	}
	srv := httptest.NewUnstartedServer(handler)
	srv.Listener = ln
	srv.Start()
	return srv
}

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
	srv := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestHandler_DownloadOTA_ThenApply_EndToEnd(t *testing.T) {
	h, dir := newTestHandler(t)

	// Prevent real process replacement during Apply.
	mock := &mockExecutor{}
	h.applier.SetExecutor(mock)
	h.latestInfo = &UpdateInfo{LatestVersion: "0.2.0"}

	// Mock OTA binary server.
	srv := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("new-binary-content"))
	}))
	defer srv.Close()

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

	// Step 1: download OTA package.
	downloadReq := httptest.NewRequest(http.MethodPost, "/system/update/download-ota", nil)
	downloadRec := httptest.NewRecorder()
	downloadCtx := e.NewContext(downloadReq, downloadRec)
	if err := h.DownloadOTA(downloadCtx); err != nil {
		t.Fatalf("DownloadOTA returned error: %v", err)
	}
	if downloadRec.Code != http.StatusAccepted {
		t.Fatalf("download status = %d, want %d", downloadRec.Code, http.StatusAccepted)
	}

	// Wait for async download completion.
	for i := 0; i < 60; i++ {
		h.mu.RLock()
		state := h.status.State
		errMsg := h.status.Error
		downloaded := h.status.DownloadedPath
		h.mu.RUnlock()

		if state == StateFailed {
			t.Fatalf("download failed: %s", errMsg)
		}
		if state == StateIdle && downloaded != "" {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	h.mu.RLock()
	downloadedPath := h.status.DownloadedPath
	h.mu.RUnlock()
	if downloadedPath == "" {
		t.Fatal("expected downloaded path before apply")
	}

	// Step 2: apply update and trigger restart path.
	applyReq := httptest.NewRequest(http.MethodPost, "/system/update/apply", nil)
	applyRec := httptest.NewRecorder()
	applyCtx := e.NewContext(applyReq, applyRec)
	if err := h.Apply(applyCtx); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if applyRec.Code != http.StatusOK {
		t.Fatalf("apply status = %d, want %d", applyRec.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.Unmarshal(applyRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal apply response: %v", err)
	}
	if resp["status"] != "restarting" {
		t.Fatalf("apply status body = %q, want %q", resp["status"], "restarting")
	}
	if resp["version"] != "0.2.0" {
		t.Fatalf("apply version = %q, want %q", resp["version"], "0.2.0")
	}

	// Restart happens in background after a short delay.
	for i := 0; i < 30; i++ {
		if mock.called {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if !mock.called {
		t.Fatal("expected restart executor to be called")
	}

	h.mu.RLock()
	state := h.status.State
	postApplyPath := h.status.DownloadedPath
	h.mu.RUnlock()
	if state != StateRestarting {
		t.Fatalf("state after apply = %q, want %q", state, StateRestarting)
	}
	if postApplyPath != "" {
		t.Fatalf("downloaded path should be cleared after apply, got %q", postApplyPath)
	}

	// Binary should be replaced with downloaded content.
	binaryData, err := os.ReadFile(filepath.Join(dir, "blue"))
	if err != nil {
		t.Fatalf("read replaced binary: %v", err)
	}
	if string(binaryData) != "new-binary-content" {
		t.Fatalf("binary content = %q, want %q", string(binaryData), "new-binary-content")
	}
}

func TestHandler_Apply_PersistsResumeTask(t *testing.T) {
	h, dir := newTestHandler(t)

	mock := &mockExecutor{}
	h.applier.SetExecutor(mock)

	updatePath := filepath.Join(dir, "update.bin")
	if err := os.WriteFile(updatePath, []byte("new-binary"), 0755); err != nil {
		t.Fatal(err)
	}
	h.status.DownloadedPath = updatePath
	h.latestInfo = &UpdateInfo{LatestVersion: "0.2.0"}

	body := []byte(`{"resume_context":{"job_id":"job-123","kind":"once-cron"},"resume_delay_seconds":1}`)
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/system/update/apply", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Apply(c); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	resumePath := filepath.Join(dir, resumeTaskFile)
	data, err := os.ReadFile(resumePath)
	if err != nil {
		t.Fatalf("read resume task: %v", err)
	}
	var task pendingResumeTask
	if err := json.Unmarshal(data, &task); err != nil {
		t.Fatalf("unmarshal resume task: %v", err)
	}
	if task.ID == "" {
		t.Fatal("resume task id should not be empty")
	}
	if task.TargetVersion != "0.2.0" {
		t.Fatalf("target version = %q, want %q", task.TargetVersion, "0.2.0")
	}
	if task.Context["job_id"] != "job-123" {
		t.Fatalf("resume context job_id = %v, want %q", task.Context["job_id"], "job-123")
	}
}

func TestNewHandler_RecoversPendingResumeTask(t *testing.T) {
	dir := t.TempDir()
	task := pendingResumeTask{
		ID:            "resume_test_1",
		CreatedAt:     time.Now().Add(-1 * time.Minute).UTC(),
		RunAt:         time.Now().Add(-10 * time.Millisecond).UTC(),
		FromVersion:   "0.1.0",
		TargetVersion: "0.2.0",
		Context: map[string]interface{}{
			"job_id": "job-999",
		},
	}
	data, _ := json.Marshal(task)
	if err := os.WriteFile(filepath.Join(dir, resumeTaskFile), data, 0644); err != nil {
		t.Fatalf("write pending resume task: %v", err)
	}

	cfg := &Config{
		StoragePath:    dir,
		BackupCount:    3,
		ReleaseChannel: ChannelStable,
	}
	h := NewHandler("0.2.0", cfg)

	for i := 0; i < 80; i++ {
		h.mu.RLock()
		done := h.lastResume != nil && len(h.history) > 0
		h.mu.RUnlock()
		if done {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	h.mu.RLock()
	last := h.lastResume
	historyLen := len(h.history)
	state := h.status.State
	h.mu.RUnlock()

	if last == nil {
		t.Fatal("expected pending resume task to be recovered")
	}
	if last.Status != StatusSuccess {
		t.Fatalf("resume status = %q, want %q", last.Status, StatusSuccess)
	}
	if last.Context["job_id"] != "job-999" {
		t.Fatalf("recovered context job_id = %v, want %q", last.Context["job_id"], "job-999")
	}
	if historyLen == 0 {
		t.Fatal("expected update history entry after recovery")
	}
	if state != StateIdle {
		t.Fatalf("status state = %q, want %q", state, StateIdle)
	}

	resumePath := filepath.Join(dir, resumeTaskFile)
	for i := 0; i < 40; i++ {
		if _, err := os.Stat(resumePath); os.IsNotExist(err) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	if _, err := os.Stat(resumePath); !os.IsNotExist(err) {
		t.Fatalf("resume task file should be removed, stat err=%v", err)
	}
}

func TestNewHandler_RecoversPendingResumeTask_WithRecoverer(t *testing.T) {
	dir := t.TempDir()
	task := pendingResumeTask{
		ID:            "resume_test_with_recoverer",
		CreatedAt:     time.Now().Add(-1 * time.Minute).UTC(),
		RunAt:         time.Now().Add(-10 * time.Millisecond).UTC(),
		FromVersion:   "0.1.0",
		TargetVersion: "0.2.0",
		Context: map[string]interface{}{
			"kind":   "once-cron",
			"job_id": "job-abc",
		},
	}
	data, _ := json.Marshal(task)
	if err := os.WriteFile(filepath.Join(dir, resumeTaskFile), data, 0644); err != nil {
		t.Fatalf("write pending resume task: %v", err)
	}

	cfg := &Config{
		StoragePath:    dir,
		BackupCount:    3,
		ReleaseChannel: ChannelStable,
	}
	h := NewHandler("0.2.0", cfg)

	calls := make(chan ResumeRecoverInput, 1)
	h.SetResumeRecoverer(func(_ context.Context, input ResumeRecoverInput) error {
		calls <- input
		return nil
	})

	for i := 0; i < 100; i++ {
		h.mu.RLock()
		done := h.lastResume != nil && len(h.history) > 0
		h.mu.RUnlock()
		if done {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	var got ResumeRecoverInput
	select {
	case got = <-calls:
	default:
		t.Fatal("expected resume recoverer to be invoked")
	}
	if got.TaskID != task.ID {
		t.Fatalf("recoverer task id = %q, want %q", got.TaskID, task.ID)
	}
	if got.Context["job_id"] != "job-abc" {
		t.Fatalf("recoverer context job_id = %v, want %q", got.Context["job_id"], "job-abc")
	}

	h.mu.RLock()
	last := h.lastResume
	state := h.status.State
	h.mu.RUnlock()
	if last == nil {
		t.Fatal("expected last resume recovery record")
	}
	if last.Status != StatusSuccess {
		t.Fatalf("resume status = %q, want %q", last.Status, StatusSuccess)
	}
	if state != StateIdle {
		t.Fatalf("state = %q, want %q", state, StateIdle)
	}
}

func TestNewHandler_RecoversPendingResumeTask_RecovererFailure(t *testing.T) {
	dir := t.TempDir()
	task := pendingResumeTask{
		ID:            "resume_test_recoverer_failed",
		CreatedAt:     time.Now().Add(-1 * time.Minute).UTC(),
		RunAt:         time.Now().Add(-10 * time.Millisecond).UTC(),
		FromVersion:   "0.1.0",
		TargetVersion: "0.2.0",
		Context: map[string]interface{}{
			"kind":   "once-cron",
			"job_id": "job-fail",
		},
	}
	data, _ := json.Marshal(task)
	if err := os.WriteFile(filepath.Join(dir, resumeTaskFile), data, 0644); err != nil {
		t.Fatalf("write pending resume task: %v", err)
	}

	cfg := &Config{
		StoragePath:    dir,
		BackupCount:    3,
		ReleaseChannel: ChannelStable,
	}
	h := NewHandler("0.2.0", cfg)
	h.SetResumeRecoverer(func(_ context.Context, _ ResumeRecoverInput) error {
		return context.DeadlineExceeded
	})

	for i := 0; i < 100; i++ {
		h.mu.RLock()
		done := h.lastResume != nil && len(h.history) > 0
		h.mu.RUnlock()
		if done {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	h.mu.RLock()
	last := h.lastResume
	state := h.status.State
	errMsg := h.status.Error
	h.mu.RUnlock()

	if last == nil {
		t.Fatal("expected last resume recovery record")
	}
	if last.Status != StatusFailed {
		t.Fatalf("resume status = %q, want %q", last.Status, StatusFailed)
	}
	if !strings.Contains(last.Error, context.DeadlineExceeded.Error()) {
		t.Fatalf("resume error = %q, want contains %q", last.Error, context.DeadlineExceeded.Error())
	}
	if state != StateFailed {
		t.Fatalf("state = %q, want %q", state, StateFailed)
	}
	if !strings.Contains(errMsg, context.DeadlineExceeded.Error()) {
		t.Fatalf("status error = %q, want contains %q", errMsg, context.DeadlineExceeded.Error())
	}
}

func TestHandler_RunAutoUpdateCycle_AutoDownloadOnly(t *testing.T) {
	h, dir := newTestHandler(t)
	h.config.Enabled = true
	h.config.AutoDownload = true
	h.config.AutoApply = false

	mock := &mockExecutor{}
	h.applier.SetExecutor(mock)

	srv := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("new-binary-content"))
	}))
	defer srv.Close()

	ota := NewOTAChecker("0.1.0", dir, "en_US")
	ota.mu.Lock()
	ota.latest = &OTAResponse{
		Version:        "0.2.0",
		Packages:       []string{srv.URL + "/blue.tar.gz"},
		ReleaseNoteURL: "https://example.com/blue/notes.md",
	}
	ota.mu.Unlock()
	h.otaChecker = ota

	if err := h.runAutoUpdateCycle(context.Background()); err != nil {
		t.Fatalf("runAutoUpdateCycle failed: %v", err)
	}

	h.mu.RLock()
	state := h.status.State
	downloaded := h.status.DownloadedPath
	h.mu.RUnlock()

	if state != StateIdle {
		t.Fatalf("state = %q, want %q", state, StateIdle)
	}
	if downloaded == "" {
		t.Fatal("expected downloaded path to be set")
	}
	if mock.called {
		t.Fatal("executor should not be called when auto_apply is disabled")
	}
}

func TestHandler_RunAutoUpdateCycle_AutoApply(t *testing.T) {
	h, dir := newTestHandler(t)
	h.config.Enabled = true
	h.config.AutoApply = true
	h.config.AutoDownload = true

	mock := &mockExecutor{}
	h.applier.SetExecutor(mock)

	srv := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("new-binary-content"))
	}))
	defer srv.Close()

	ota := NewOTAChecker("0.1.0", dir, "en_US")
	ota.mu.Lock()
	ota.latest = &OTAResponse{
		Version:        "0.2.0",
		Packages:       []string{srv.URL + "/blue.tar.gz"},
		ReleaseNoteURL: "https://example.com/blue/notes.md",
	}
	ota.mu.Unlock()
	h.otaChecker = ota

	if err := h.runAutoUpdateCycle(context.Background()); err != nil {
		t.Fatalf("runAutoUpdateCycle failed: %v", err)
	}

	if !mock.called {
		t.Fatal("expected executor to be called for auto apply")
	}

	h.mu.RLock()
	state := h.status.State
	downloaded := h.status.DownloadedPath
	h.mu.RUnlock()

	if state != StateRestarting {
		t.Fatalf("state = %q, want %q", state, StateRestarting)
	}
	if downloaded != "" {
		t.Fatalf("downloaded path should be cleared after auto apply, got %q", downloaded)
	}

	binaryData, err := os.ReadFile(filepath.Join(dir, "blue"))
	if err != nil {
		t.Fatalf("read replaced binary: %v", err)
	}
	if string(binaryData) != "new-binary-content" {
		t.Fatalf("binary content = %q, want %q", string(binaryData), "new-binary-content")
	}
}

func TestHandler_RunAutoUpdateCycle_NoUpdateAvailable(t *testing.T) {
	h, dir := newTestHandler(t)
	h.config.Enabled = true
	h.config.AutoApply = true
	h.config.AutoDownload = true

	mock := &mockExecutor{}
	h.applier.SetExecutor(mock)

	ota := NewOTAChecker("0.1.0", dir, "en_US")
	ota.mu.Lock()
	ota.latest = &OTAResponse{
		Version:        "0.1.0",
		Packages:       []string{"https://example.com/blue.tar.gz"},
		ReleaseNoteURL: "https://example.com/blue/notes.md",
	}
	ota.mu.Unlock()
	h.otaChecker = ota

	if err := h.runAutoUpdateCycle(context.Background()); err != nil {
		t.Fatalf("runAutoUpdateCycle failed: %v", err)
	}

	h.mu.RLock()
	state := h.status.State
	downloaded := h.status.DownloadedPath
	h.mu.RUnlock()

	if state != StateIdle {
		t.Fatalf("state = %q, want %q", state, StateIdle)
	}
	if downloaded != "" {
		t.Fatalf("downloaded path = %q, want empty", downloaded)
	}
	if mock.called {
		t.Fatal("executor should not be called when no update is available")
	}
}
