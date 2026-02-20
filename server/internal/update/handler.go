package update

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/downloader"
	"github.com/labstack/echo/v4"
)

// Handler handles update API endpoints
type Handler struct {
	mu             sync.RWMutex
	currentVersion string
	config         *Config
	github         *GitHubClient
	downloader     *Downloader
	applier        *Applier
	status         *UpdateStatus
	latestInfo     *UpdateInfo
	history        []UpdateHistory
	otaChecker     *OTAChecker
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

// NewHandler creates a new update handler
func NewHandler(version string, cfg *Config) *Handler {
	binaryPath, _ := os.Executable()
	return &Handler{
		currentVersion: version,
		config:         cfg,
		github:         NewGitHubClient(),
		downloader:     NewDownloader(cfg.StoragePath),
		applier:        NewApplier(binaryPath, cfg.StoragePath, cfg.BackupCount),
		status:         &UpdateStatus{State: StateIdle},
	}
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

	// Phase 1: backup + replace binary (synchronous — can still return error)
	if err := h.applier.PrepareAndReplace(path); err != nil {
		h.setError(err.Error())
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	h.mu.Lock()
	h.status.State = StateRestarting
	h.status.DownloadedPath = ""
	h.mu.Unlock()

	// Phase 2: send response, then restart in background
	targetVersion := ""
	if h.latestInfo != nil {
		targetVersion = h.latestInfo.LatestVersion
	}

	c.Response().Header().Set("Connection", "close")
	if err := c.JSON(http.StatusOK, map[string]string{
		"status":  "restarting",
		"version": targetVersion,
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
