package update

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"

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
}

// Config holds update configuration
type Config struct {
	Enabled        bool          `mapstructure:"enabled"`
	CheckInterval  time.Duration `mapstructure:"check_interval"`
	AutoDownload   bool          `mapstructure:"auto_download"`
	AutoApply      bool          `mapstructure:"auto_apply"`
	ReleaseChannel string        `mapstructure:"release_channel"`
	BackupCount    int           `mapstructure:"backup_count"`
	StoragePath    string        `mapstructure:"storage_path"`
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
	g.POST("/system/update/apply", h.Apply)
	g.POST("/system/update/rollback", h.Rollback)
	g.GET("/system/update/history", h.History)
}

// Check checks for available updates
func (h *Handler) Check(c echo.Context) error {
	h.mu.Lock()
	h.status.State = StateChecking
	h.mu.Unlock()

	ctx := c.Request().Context()
	latest, err := h.github.GetLatestVersion(ctx, h.config.ReleaseChannel)
	if err != nil {
		h.setError(err.Error())
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	current, _ := ParseVersion(h.currentVersion)
	updateAvailable := latest.IsNewerThan(current)

	h.mu.Lock()
	h.status.State = StateIdle
	h.status.LastChecked = time.Now()
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

// Apply applies the downloaded update
func (h *Handler) Apply(c echo.Context) error {
	h.mu.Lock()
	if h.status.DownloadedPath == "" {
		h.mu.Unlock()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "no update downloaded"})
	}
	h.status.State = StateApplying
	path := h.status.DownloadedPath
	h.mu.Unlock()

	if err := h.applier.Apply(path); err != nil {
		h.setError(err.Error())
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "applied"})
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
