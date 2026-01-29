package claudecode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// DownloadInfo contains information about a CLI download.
type DownloadInfo struct {
	Version     string `json:"version"`
	Size        int64  `json:"size"`
	SizeHuman   string `json:"size_human"`
	DownloadURL string `json:"download_url"`
	Checksum    string `json:"checksum"`
	Platform    string `json:"platform"`
}

// DownloadProgress represents the current download progress.
type DownloadProgress struct {
	Downloaded int64     `json:"downloaded"`
	Total      int64     `json:"total"`
	Percentage float64   `json:"percentage"`
	Speed      float64   `json:"speed"`       // bytes per second
	SpeedHuman string    `json:"speed_human"` // human readable speed
	ETA        string    `json:"eta"`         // estimated time remaining
	StartedAt  time.Time `json:"started_at"`
}

// CLIStatus represents the current CLI installation status.
type CLIStatus struct {
	Installed       bool   `json:"installed"`
	Version         string `json:"version,omitempty"`
	Path            string `json:"path,omitempty"`
	Source          string `json:"source,omitempty"` // "embedded", "downloaded", "system"
	UpdateAvailable bool   `json:"update_available"`
	LatestVersion   string `json:"latest_version,omitempty"`
}

// DownloadManager manages Claude Code CLI downloads.
type DownloadManager struct {
	BaseURL    string
	CacheDir   string
	OnProgress func(progress DownloadProgress)

	mu           sync.Mutex
	downloading  bool
	cancelFunc   context.CancelFunc
	currentBytes int64
	totalBytes   int64
	startTime    time.Time
}

// NewDownloadManager creates a new download manager.
func NewDownloadManager(cacheDir string) *DownloadManager {
	if cacheDir == "" {
		home, _ := os.UserHomeDir()
		cacheDir = filepath.Join(home, ".local", "share", "zimaos-echo", "claude-code")
	}

	return &DownloadManager{
		BaseURL:  "https://storage.googleapis.com/anthropic-public/claude-code",
		CacheDir: cacheDir,
	}
}

// GetPlatform returns the current platform string.
func GetPlatform() string {
	os := runtime.GOOS
	arch := runtime.GOARCH

	// Map Go arch to platform arch
	switch arch {
	case "amd64":
		arch = "x64"
	case "arm64":
		// Keep as arm64
	}

	// Map Go OS to platform OS
	switch os {
	case "darwin":
		// Keep as darwin
	case "linux":
		// Keep as linux
	case "windows":
		os = "win32"
	}

	return fmt.Sprintf("%s-%s", os, arch)
}

// GetDownloadInfo fetches information about the latest CLI version.
func (m *DownloadManager) GetDownloadInfo(ctx context.Context) (*DownloadInfo, error) {
	platform := GetPlatform()

	// Try to fetch latest.json
	latestURL := fmt.Sprintf("%s/%s/latest.json", m.BaseURL, platform)
	req, err := http.NewRequestWithContext(ctx, "GET", latestURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Return estimated info if we can't fetch latest
		return &DownloadInfo{
			Version:   "latest",
			Size:      120 * 1024 * 1024, // ~120MB estimate
			SizeHuman: "~120 MB",
			Platform:  platform,
		}, nil
	}

	var info DownloadInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	info.Platform = platform
	info.SizeHuman = formatBytes(info.Size)

	return &info, nil
}

// Download downloads the CLI with progress tracking.
func (m *DownloadManager) Download(ctx context.Context) error {
	m.mu.Lock()
	if m.downloading {
		m.mu.Unlock()
		return fmt.Errorf("download already in progress")
	}
	m.downloading = true
	ctx, m.cancelFunc = context.WithCancel(ctx)
	m.startTime = time.Now()
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.downloading = false
		m.cancelFunc = nil
		m.mu.Unlock()
	}()

	// Ensure cache directory exists
	if err := os.MkdirAll(m.CacheDir, 0755); err != nil {
		return err
	}

	// Get download info
	info, err := m.GetDownloadInfo(ctx)
	if err != nil {
		return err
	}

	m.totalBytes = info.Size

	// Determine download URL
	downloadURL := info.DownloadURL
	if downloadURL == "" {
		downloadURL = fmt.Sprintf("%s/%s/claude", m.BaseURL, info.Platform)
	}

	// Create download request
	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return err
	}

	// Check for partial download
	partialPath := filepath.Join(m.CacheDir, "claude.partial")
	if stat, err := os.Stat(partialPath); err == nil {
		m.currentBytes = stat.Size()
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", stat.Size()))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Update total size from Content-Length if available
	if resp.ContentLength > 0 {
		if resp.StatusCode == http.StatusPartialContent {
			m.totalBytes = m.currentBytes + resp.ContentLength
		} else {
			m.totalBytes = resp.ContentLength
			m.currentBytes = 0
		}
	}

	// Open file for writing
	flags := os.O_CREATE | os.O_WRONLY
	if resp.StatusCode == http.StatusPartialContent {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	file, err := os.OpenFile(partialPath, flags, 0755)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create progress reader
	reader := &progressReader{
		reader:     resp.Body,
		onProgress: m.updateProgress,
	}

	// Copy data
	_, err = io.Copy(file, reader)
	if err != nil {
		return err
	}

	// Verify checksum if provided
	if info.Checksum != "" {
		if err := m.verifyChecksum(partialPath, info.Checksum); err != nil {
			os.Remove(partialPath)
			return err
		}
	}

	// Move to final location
	finalPath := filepath.Join(m.CacheDir, "claude")
	if runtime.GOOS == "windows" {
		finalPath += ".exe"
	}

	if err := os.Rename(partialPath, finalPath); err != nil {
		return err
	}

	// Make executable
	if runtime.GOOS != "windows" {
		if err := os.Chmod(finalPath, 0755); err != nil {
			return err
		}
	}

	return nil
}

// Cancel cancels the current download.
func (m *DownloadManager) Cancel() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancelFunc != nil {
		m.cancelFunc()
	}
}

// IsDownloading returns true if a download is in progress.
func (m *DownloadManager) IsDownloading() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.downloading
}

// GetProgress returns the current download progress.
func (m *DownloadManager) GetProgress() DownloadProgress {
	m.mu.Lock()
	defer m.mu.Unlock()

	elapsed := time.Since(m.startTime).Seconds()
	speed := float64(0)
	if elapsed > 0 {
		speed = float64(m.currentBytes) / elapsed
	}

	eta := ""
	if speed > 0 && m.totalBytes > m.currentBytes {
		remaining := float64(m.totalBytes-m.currentBytes) / speed
		eta = formatDuration(time.Duration(remaining) * time.Second)
	}

	percentage := float64(0)
	if m.totalBytes > 0 {
		percentage = float64(m.currentBytes) / float64(m.totalBytes) * 100
	}

	return DownloadProgress{
		Downloaded: m.currentBytes,
		Total:      m.totalBytes,
		Percentage: percentage,
		Speed:      speed,
		SpeedHuman: formatBytes(int64(speed)) + "/s",
		ETA:        eta,
		StartedAt:  m.startTime,
	}
}

// GetCLIStatus returns the current CLI installation status.
func (m *DownloadManager) GetCLIStatus(ctx context.Context) (*CLIStatus, error) {
	status := &CLIStatus{
		Installed: false,
	}

	// Check downloaded CLI
	downloadedPath := filepath.Join(m.CacheDir, "claude")
	if runtime.GOOS == "windows" {
		downloadedPath += ".exe"
	}

	if _, err := os.Stat(downloadedPath); err == nil {
		version, err := m.getCLIVersion(ctx, downloadedPath)
		if err == nil {
			status.Installed = true
			status.Version = version
			status.Path = downloadedPath
			status.Source = "downloaded"
		}
	}

	// Check system CLI if not found
	if !status.Installed {
		systemPath, err := exec.LookPath("claude")
		if err == nil {
			version, err := m.getCLIVersion(ctx, systemPath)
			if err == nil {
				status.Installed = true
				status.Version = version
				status.Path = systemPath
				status.Source = "system"
			}
		}
	}

	// Check for updates
	if status.Installed {
		info, err := m.GetDownloadInfo(ctx)
		if err == nil && info.Version != "" && info.Version != "latest" {
			status.LatestVersion = info.Version
			status.UpdateAvailable = status.Version != info.Version
		}
	}

	return status, nil
}

// getCLIVersion gets the version of a CLI binary.
func (m *DownloadManager) getCLIVersion(ctx context.Context, path string) (string, error) {
	cmd := exec.CommandContext(ctx, path, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// verifyChecksum verifies the SHA256 checksum of a file.
func (m *DownloadManager) verifyChecksum(path, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != expected {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
	}

	return nil
}

// updateProgress updates the current progress.
func (m *DownloadManager) updateProgress(n int64) {
	m.mu.Lock()
	m.currentBytes += n
	m.mu.Unlock()

	if m.OnProgress != nil {
		m.OnProgress(m.GetProgress())
	}
}

// progressReader wraps an io.Reader to track progress.
type progressReader struct {
	reader     io.Reader
	onProgress func(n int64)
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 && r.onProgress != nil {
		r.onProgress(int64(n))
	}
	return n, err
}

// formatBytes formats bytes to human readable string.
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatDuration formats a duration to human readable string.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

// GetCLIPath returns the path to the CLI binary.
func (m *DownloadManager) GetCLIPath() string {
	path := filepath.Join(m.CacheDir, "claude")
	if runtime.GOOS == "windows" {
		path += ".exe"
	}
	return path
}

// ClearCache removes downloaded CLI files.
func (m *DownloadManager) ClearCache() error {
	files := []string{
		filepath.Join(m.CacheDir, "claude"),
		filepath.Join(m.CacheDir, "claude.exe"),
		filepath.Join(m.CacheDir, "claude.partial"),
	}

	for _, f := range files {
		os.Remove(f)
	}

	return nil
}
