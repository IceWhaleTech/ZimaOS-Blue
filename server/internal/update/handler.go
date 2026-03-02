package update

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/downloader"
	"github.com/labstack/echo/v4"
)

// Handler handles update API endpoints
type Handler struct {
	mu              sync.RWMutex
	currentVersion  string
	config          *Config
	github          *GitHubClient
	downloader      *Downloader
	applier         *Applier
	status          *UpdateStatus
	latestInfo      *UpdateInfo
	history         []UpdateHistory
	otaChecker      *OTAChecker
	lastResume      *ResumeRecovery
	resumeRecoverer ResumeRecoverer
}

// Config holds update configuration
type Config struct {
	Enabled        bool          `yaml:"enabled"`
	CheckInterval  time.Duration `yaml:"check_interval"`
	AutoDownload   bool          `yaml:"auto_download"`
	AutoApply      bool          `yaml:"auto_apply"`
	ReleaseChannel string        `yaml:"release_channel"`
	BackupCount    int           `yaml:"backup_count"`
	StoragePath    string        `yaml:"storage_path"`
}

const (
	resumeTaskFile       = "update_resume_task.json"
	defaultResumeDelay   = 2 * time.Second
	minResumeDelayOnBoot = 500 * time.Millisecond
)

type pendingResumeTask struct {
	ID            string                 `json:"id"`
	CreatedAt     time.Time              `json:"created_at"`
	RunAt         time.Time              `json:"run_at"`
	FromVersion   string                 `json:"from_version"`
	TargetVersion string                 `json:"target_version,omitempty"`
	Context       map[string]interface{} `json:"context,omitempty"`
}

// NewHandler creates a new update handler
func NewHandler(version string, cfg *Config) *Handler {
	binaryPath, _ := os.Executable()
	h := &Handler{
		currentVersion: version,
		config:         cfg,
		github:         NewGitHubClient(),
		downloader:     NewDownloader(cfg.StoragePath),
		applier:        NewApplier(binaryPath, cfg.StoragePath, cfg.BackupCount),
		status:         &UpdateStatus{State: StateIdle},
	}
	h.schedulePendingResumeTask()
	return h
}

// RegisterRoutes registers update routes
func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.GET("/system/update/check", h.Check)
	g.GET("/system/update/info", h.Info)
	g.POST("/system/update/download", h.Download)
	g.POST("/system/update/download-ota", h.DownloadOTA)
	g.POST("/system/update/apply", h.Apply)
	g.POST("/system/update/rollback", h.Rollback)
	g.GET("/system/update/history", h.History)
	g.GET("/system/update/ota", h.OTAStatus)
	g.GET("/system/update/release-notes", h.ReleaseNotes)
}

// SetOTAChecker attaches the background OTA checker to the handler.
func (h *Handler) SetOTAChecker(ota *OTAChecker) {
	h.otaChecker = ota
}

// SetResumeRecoverer sets the callback used to restore workloads after OTA restart.
func (h *Handler) SetResumeRecoverer(recoverer ResumeRecoverer) {
	h.mu.Lock()
	h.resumeRecoverer = recoverer
	h.mu.Unlock()
}

// Check checks for available updates
func (h *Handler) Check(c echo.Context) error {
	h.mu.Lock()
	h.status.State = StateChecking
	h.mu.Unlock()

	ctx := c.Request().Context()
	latest, err := h.github.GetLatestVersion(ctx, h.config.ReleaseChannel)

	h.mu.Lock()
	h.status.State = StateIdle
	h.status.LastChecked = time.Now()

	if err != nil {
		// Network error or repo not found - return "no update available" instead of error
		h.latestInfo = &UpdateInfo{
			CurrentVersion:  h.currentVersion,
			LatestVersion:   h.currentVersion,
			UpdateAvailable: false,
			ReleaseChannel:  h.config.ReleaseChannel,
		}
		h.mu.Unlock()
		return c.JSON(http.StatusOK, h.latestInfo)
	}

	current, _ := ParseVersion(h.currentVersion)
	updateAvailable := latest.IsNewerThan(current)

	h.latestInfo = &UpdateInfo{
		CurrentVersion:  h.currentVersion,
		LatestVersion:   latest.String(),
		UpdateAvailable: updateAvailable,
		ReleaseChannel:  h.config.ReleaseChannel,
		DownloadURL:     h.github.GetDownloadURL(latest.String()),
	}
	h.mu.Unlock()

	return c.JSON(http.StatusOK, h.latestInfo)
}

// Info returns current update info
func (h *Handler) Info(c echo.Context) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"current_version": h.currentVersion,
		"status":          h.status,
		"latest":          h.latestInfo,
		"uptime":          GetUptime().String(),
		"resume_recovery": h.lastResume,
	})
}

// Download downloads the update
func (h *Handler) Download(c echo.Context) error {
	h.mu.Lock()
	if h.status.State != StateIdle {
		h.mu.Unlock()
		return c.JSON(http.StatusConflict, map[string]string{"error": "update in progress"})
	}
	h.status.State = StateDownloading
	h.mu.Unlock()

	go h.downloadAsync(c.Request().Context())
	return c.JSON(http.StatusAccepted, map[string]string{"status": "downloading"})
}

func (h *Handler) downloadAsync(ctx context.Context) {
	h.downloader.SetProgressCallback(func(p float64) {
		h.mu.Lock()
		h.status.Progress = p
		h.mu.Unlock()
	})

	url := h.latestInfo.DownloadURL
	path, err := h.downloader.Download(ctx, url, h.latestInfo.Checksum)

	h.mu.Lock()
	if err != nil {
		h.status.State = StateFailed
		h.status.Error = err.Error()
	} else {
		h.status.State = StateIdle
		h.status.DownloadedPath = path
	}
	h.mu.Unlock()
}

// DownloadOTA downloads the update from OTA package URLs (with mirror fallback).
// POST /api/v1/system/update/download-ota
func (h *Handler) DownloadOTA(c echo.Context) error {
	h.mu.Lock()
	if h.status.State != StateIdle {
		h.mu.Unlock()
		return c.JSON(http.StatusConflict, map[string]string{"error": "update in progress"})
	}
	h.status.State = StateDownloading
	h.status.Progress = 0
	h.status.Error = ""
	h.mu.Unlock()

	go h.downloadOTAAsync()
	return c.JSON(http.StatusAccepted, map[string]string{"status": "downloading"})
}

func (h *Handler) downloadOTAAsync() {
	if h.otaChecker == nil {
		h.setError("OTA checker not initialized")
		return
	}
	latest := h.otaChecker.GetLatest()
	if latest == nil || len(latest.Packages) == 0 {
		h.setError("no OTA packages available")
		return
	}

	h.downloader.SetProgressCallback(func(p float64) {
		h.mu.Lock()
		h.status.Progress = p
		h.mu.Unlock()
	})

	// Try each package URL (mirrors)
	var lastErr error
	for _, url := range latest.Packages {
		path, err := h.downloader.Download(context.Background(), url, "")
		if err != nil {
			lastErr = err
			log.Printf("[update] download mirror failed (%s): %v", url, err)
			continue
		}
		h.mu.Lock()
		h.status.State = StateIdle
		h.status.DownloadedPath = path
		h.status.Progress = 100
		h.mu.Unlock()
		log.Printf("[update] download complete: %s", path)
		return
	}

	h.setError(fmt.Sprintf("all download mirrors failed: %v", lastErr))
}

// The response is sent BEFORE the process is replaced, so the client
// should start polling /api/v1/health to detect when the new version is up.
func (h *Handler) Apply(c echo.Context) error {
	var req ApplyRequest
	if c.Request().ContentLength > 0 {
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		}
	}

	h.mu.Lock()
	if h.status.DownloadedPath == "" {
		h.mu.Unlock()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "no update downloaded"})
	}
	if h.status.State == StateApplying || h.status.State == StateRestarting {
		h.mu.Unlock()
		return c.JSON(http.StatusConflict, map[string]string{"error": "update already in progress"})
	}
	h.status.State = StateApplying
	path := h.status.DownloadedPath
	h.mu.Unlock()

	// Persist resume context before replacing process image.
	targetVersion := ""
	if h.latestInfo != nil {
		targetVersion = h.latestInfo.LatestVersion
	}
	resumeTask, err := h.persistPendingResumeTask(path, targetVersion, req.ResumeContext, req.ResumeDelaySeconds)
	if err != nil {
		h.setError(err.Error())
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Phase 1: backup + replace binary (synchronous — can still return error)
	if err := h.applier.PrepareAndReplace(path); err != nil {
		_ = h.clearPendingResumeTask()
		h.setError(err.Error())
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	h.mu.Lock()
	h.status.State = StateRestarting
	h.status.DownloadedPath = ""
	h.mu.Unlock()

	// Phase 2: send response, then restart in background
	c.Response().Header().Set("Connection", "close")
	if err := c.JSON(http.StatusOK, map[string]string{
		"status":         "restarting",
		"version":        targetVersion,
		"resume_task_id": resumeTask.ID,
	}); err != nil {
		return err
	}
	c.Response().Flush()

	// Give the response time to reach the client, then exec
	go func() {
		time.Sleep(200 * time.Millisecond)
		if err := h.applier.Restart(); err != nil {
			log.Printf("[update] restart failed: %v", err)
			h.setError(err.Error())
		}
	}()

	return nil
}

// Rollback rolls back to previous version
func (h *Handler) Rollback(c echo.Context) error {
	if err := h.applier.Rollback(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "rolled_back"})
}

// History returns update history
func (h *Handler) History(c echo.Context) error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return c.JSON(http.StatusOK, h.history)
}

func (h *Handler) setError(msg string) {
	h.mu.Lock()
	h.status.State = StateFailed
	h.status.Error = msg
	h.mu.Unlock()
}

func (h *Handler) schedulePendingResumeTask() {
	task, err := h.loadPendingResumeTask()
	if err != nil {
		log.Printf("[update] load pending resume task failed: %v", err)
		return
	}
	if task == nil {
		return
	}

	delay := time.Until(task.RunAt)
	if delay < 0 {
		delay = 0
	}
	// Give bootstrap a short window to inject runtime recoverers after handler construction.
	if delay < minResumeDelayOnBoot {
		delay = minResumeDelayOnBoot
	}

	go func(t *pendingResumeTask, d time.Duration) {
		time.Sleep(d)
		h.executePendingResumeTask(t)
	}(task, delay)
}

func (h *Handler) executePendingResumeTask(task *pendingResumeTask) {
	if task == nil {
		return
	}

	status := StatusSuccess
	errMsg := ""
	if task.TargetVersion != "" && task.TargetVersion != h.currentVersion {
		status = StatusFailed
		errMsg = fmt.Sprintf("resume task target version %s does not match current version %s", task.TargetVersion, h.currentVersion)
	}

	if status == StatusSuccess {
		h.mu.RLock()
		recoverer := h.resumeRecoverer
		h.mu.RUnlock()
		if recoverer != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			err := recoverer(ctx, ResumeRecoverInput{
				TaskID:         task.ID,
				FromVersion:    task.FromVersion,
				TargetVersion:  task.TargetVersion,
				CurrentVersion: h.currentVersion,
				Context:        cloneResumeContext(task.Context),
			})
			cancel()
			if err != nil {
				status = StatusFailed
				errMsg = err.Error()
			}
		}
	}

	now := time.Now().UTC()
	h.mu.Lock()
	h.lastResume = &ResumeRecovery{
		TaskID:         task.ID,
		Status:         status,
		FromVersion:    task.FromVersion,
		TargetVersion:  task.TargetVersion,
		CurrentVersion: h.currentVersion,
		RecoveredAt:    now,
		Context:        task.Context,
		Error:          errMsg,
	}
	h.history = append(h.history, UpdateHistory{
		ID:          task.ID,
		FromVersion: task.FromVersion,
		ToVersion:   h.currentVersion,
		Status:      status,
		AppliedAt:   now,
	})
	if status == StatusSuccess {
		h.status.State = StateIdle
		h.status.Error = ""
	} else {
		h.status.State = StateFailed
		h.status.Error = errMsg
	}
	h.mu.Unlock()

	if err := h.clearPendingResumeTask(); err != nil {
		log.Printf("[update] clear pending resume task failed: %v", err)
	}
}

func cloneResumeContext(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (h *Handler) pendingResumeTaskPath() string {
	base := "."
	if h.config != nil && h.config.StoragePath != "" {
		base = h.config.StoragePath
	}
	return filepath.Join(base, resumeTaskFile)
}

func (h *Handler) persistPendingResumeTask(downloadedPath, targetVersion string, extra map[string]interface{}, delaySeconds int) (*pendingResumeTask, error) {
	delay := defaultResumeDelay
	if delaySeconds > 0 {
		delay = time.Duration(delaySeconds) * time.Second
	}

	now := time.Now().UTC()
	ctx := map[string]interface{}{
		"downloaded_path": downloadedPath,
		"reason":          "ota-apply-restart",
	}
	for k, v := range extra {
		ctx[k] = v
	}

	task := &pendingResumeTask{
		ID:            fmt.Sprintf("resume_%d", now.UnixNano()),
		CreatedAt:     now,
		RunAt:         now.Add(delay),
		FromVersion:   h.currentVersion,
		TargetVersion: targetVersion,
		Context:       ctx,
	}

	path := h.pendingResumeTaskPath()
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create resume dir: %w", err)
		}
	}
	data, err := json.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("marshal resume task: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return nil, fmt.Errorf("write resume task: %w", err)
	}
	return task, nil
}

func (h *Handler) loadPendingResumeTask() (*pendingResumeTask, error) {
	path := h.pendingResumeTaskPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var task pendingResumeTask
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, err
	}
	if task.ID == "" {
		return nil, fmt.Errorf("invalid pending resume task: empty id")
	}
	return &task, nil
}

func (h *Handler) clearPendingResumeTask() error {
	path := h.pendingResumeTaskPath()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// LoadHistory loads history from file
func (h *Handler) LoadHistory(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &h.history)
}

// GetCurrentVersion returns the current version string.
func (h *Handler) GetCurrentVersion() string {
	return h.currentVersion
}

// GetStatus returns the current update status.
func (h *Handler) GetStatus() *UpdateStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	s := *h.status // copy
	return &s
}

// CheckForUpdate checks for available updates and returns the info.
func (h *Handler) CheckForUpdate(ctx context.Context) (*UpdateInfo, error) {
	latest, err := h.github.GetLatestVersion(ctx, h.config.ReleaseChannel)
	if err != nil {
		return nil, err
	}
	current, _ := ParseVersion(h.currentVersion)
	updateAvailable := latest.IsNewerThan(current)
	return &UpdateInfo{
		CurrentVersion:  h.currentVersion,
		LatestVersion:   latest.String(),
		UpdateAvailable: updateAvailable,
		ReleaseChannel:  h.config.ReleaseChannel,
		DownloadURL:     h.github.GetDownloadURL(latest.String()),
	}, nil
}

// StartDownload starts downloading the update in the background.
func (h *Handler) StartDownload(ctx context.Context) error {
	h.mu.Lock()
	if h.status.State != StateIdle {
		h.mu.Unlock()
		return fmt.Errorf("update already in progress (state: %s)", h.status.State)
	}
	h.status.State = StateDownloading
	h.status.Progress = 0
	h.mu.Unlock()

	go h.downloadAsync(ctx)
	return nil
}

// ApplyUpdate applies the downloaded update.
func (h *Handler) ApplyUpdate() error {
	h.mu.Lock()
	path := h.status.DownloadedPath
	if path == "" {
		h.mu.Unlock()
		return fmt.Errorf("no update downloaded")
	}
	h.status.State = StateApplying
	h.mu.Unlock()

	if err := h.applier.Apply(path); err != nil {
		h.setError(err.Error())
		return err
	}

	h.mu.Lock()
	h.status.State = StateRestarting
	h.mu.Unlock()
	return nil
}

// OTAStatus returns the latest OTA check result.
// GET /api/v1/system/update/ota?desktop=1
func (h *Handler) OTAStatus(c echo.Context) error {
	if h.otaChecker == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"current_version":  h.currentVersion,
			"update_available": false,
		})
	}

	// Desktop clients (Tauri) pass ?desktop=1 to get platform-specific package URL
	desktop := c.QueryParam("desktop") == "1"
	var latest *OTAResponse
	if desktop {
		// Fetch with -desktop suffix on-demand (not cached by background checker)
		if resp, err := h.otaChecker.FetchForDesktop(c.Request().Context()); err == nil {
			latest = resp
		}
	} else {
		latest = h.otaChecker.GetLatest()
	}

	available := latest != nil && latest.Version != "" && IsNewerVersionString(h.currentVersion, latest.Version)
	resp := map[string]interface{}{
		"current_version":  h.currentVersion,
		"update_available": available,
	}
	if latest != nil {
		resp["latest_version"] = latest.Version
		resp["download_urls"] = latest.Packages
		resp["release_note_url"] = latest.ReleaseNoteURL
		if latest.Delay > 0 {
			resp["delay"] = latest.Delay
		}
	}
	return c.JSON(http.StatusOK, resp)
}

const defaultReleaseNotes = `### New
- Please visit [here](https://github.com/IceWhaleTech/ZimaOS-Blue/releases) to find latest notes.

### Tips
- If you find any software problems, welcome to join the Discord and get support from 43,000 Zima community members
- [https://zimaboard.com/discord](https://zimaboard.com/discord)`

// ReleaseNotes fetches release notes content via the downloader (GitHub URL expansion + IPv4 fallback).
// GET /api/v1/system/update/release-notes
func (h *Handler) ReleaseNotes(c echo.Context) error {
	url := c.QueryParam("url")
	if url == "" && h.otaChecker != nil {
		if latest := h.otaChecker.GetLatest(); latest != nil {
			url = latest.ReleaseNoteURL
		}
	}
	if url == "" {
		return c.String(http.StatusOK, defaultReleaseNotes)
	}

	dl := &downloader.Downloader{URL: url, Timeout: 8 * time.Second}
	content, _, err := dl.Fetch()
	if err != nil || content == "" {
		return c.String(http.StatusOK, defaultReleaseNotes)
	}
	return c.String(http.StatusOK, content)
}
