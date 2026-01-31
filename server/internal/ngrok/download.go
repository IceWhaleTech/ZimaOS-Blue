package ngrok

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// NgrokDownloadInfo contains information about ngrok download.
type NgrokDownloadInfo struct {
	Version     string `json:"version"`
	Size        int64  `json:"size"`
	SizeHuman   string `json:"size_human"`
	DownloadURL string `json:"download_url"`
	Checksum    string `json:"checksum"`
	Platform    string `json:"platform"`
}

// NgrokDownloadProgress represents the current download progress.
type NgrokDownloadProgress struct {
	Downloaded int64     `json:"downloaded"`
	Total      int64     `json:"total"`
	Percentage float64   `json:"percentage"`
	Speed      float64   `json:"speed"`
	SpeedHuman string    `json:"speed_human"`
	ETA        string    `json:"eta"`
	StartedAt  time.Time `json:"started_at"`
	Mirror     string    `json:"mirror,omitempty"` // Current mirror being used
	Phase      string    `json:"phase,omitempty"`  // Current phase: "testing_mirrors", "downloading", "extracting", "complete"
}

// NgrokStatus represents the ngrok installation status.
type NgrokStatus struct {
	Installed       bool   `json:"installed"`
	Version         string `json:"version,omitempty"`
	Path            string `json:"path,omitempty"`
	UpdateAvailable bool   `json:"update_available"`
	LatestVersion   string `json:"latest_version,omitempty"`
}

// MirrorSpeedResult represents the speed test result for a mirror.
type MirrorSpeedResult struct {
	URL       string        `json:"url"`
	Name      string        `json:"name"`
	Latency   time.Duration `json:"latency"`
	Speed     float64       `json:"speed"` // bytes per second (from partial download test)
	Reachable bool          `json:"reachable"`
	Error     string        `json:"error,omitempty"`
}

// DownloadManager manages ngrok binary downloads.
type DownloadManager struct {
	BaseURL  string
	CacheDir string

	mu            sync.Mutex
	downloading   bool
	cancelFunc    context.CancelFunc
	currentBytes  int64
	totalBytes    int64
	startTime     time.Time
	currentMirror string // Track which mirror is being used
	currentPhase  string // Current phase: "testing_mirrors", "downloading", "extracting", "complete"
}

// ngrok download base URL
const NgrokDownloadBase = "https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable"

// MirrorInfo contains mirror URL and display name
type MirrorInfo struct {
	URL  string
	Name string
}

// Mirror URLs for ngrok download (fallback when primary fails)
// These are proxy/mirror services that can help with slow connections
var ngrokMirrors = []MirrorInfo{
	{URL: "https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable", Name: "Official"},
	{URL: "https://ghproxy.com/https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable", Name: "GHProxy"},
	{URL: "https://mirror.ghproxy.com/https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable", Name: "GHProxy Mirror"},
	{URL: "https://gh.ddlc.top/https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable", Name: "DDLC Proxy"},
	{URL: "https://gh-proxy.com/https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable", Name: "GH-Proxy"},
}

// Timeout constants
const (
	// Download timeout for initial connection (dial + TLS handshake)
	downloadConnectTimeout = 10 * time.Second
	// Speed test timeout - must be short to avoid blocking
	speedTestTimeout = 3 * time.Second
	// Speed test download size (download first 32KB to test speed)
	speedTestBytes = 32 * 1024
	// Read timeout for download - if no data received for this duration, fail
	downloadReadTimeout = 30 * time.Second
	// Minimum acceptable speed (bytes/sec) - below this, try next mirror
	minAcceptableSpeed = 10 * 1024 // 10 KB/s
)

// Platform to archive extension mapping
var platformExtensions = map[string]string{
	"darwin-amd64":  ".zip",
	"darwin-arm64":  ".zip",
	"linux-amd64":   ".tgz",
	"linux-arm64":   ".tgz",
	"windows-amd64": ".zip",
}

// createDualStackDialer creates a dialer that prefers IPv4 to avoid IPv6 issues
func createDualStackDialer() *net.Dialer {
	return &net.Dialer{
		Timeout:   downloadConnectTimeout,
		KeepAlive: 30 * time.Second,
		DualStack: true,
		// FallbackDelay controls how long to wait before starting IPv4 if IPv6 is slow
		FallbackDelay: 300 * time.Millisecond,
	}
}

// createHTTPClient creates an HTTP client with proper timeouts and dual-stack support
func createHTTPClient(timeout time.Duration) *http.Client {
	dialer := createDualStackDialer()

	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: timeout,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          10,
		MaxIdleConnsPerHost:   2,
		DisableCompression:    true, // Don't compress, we want accurate progress
	}

	return &http.Client{
		Transport: transport,
		Timeout:   0, // We handle timeouts per-operation
	}
}

// NewDownloadManager creates a new download manager.
func NewDownloadManager(cacheDir string) *DownloadManager {
	if cacheDir == "" {
		home, _ := os.UserHomeDir()
		cacheDir = filepath.Join(home, ".local", "share", "zimaos-echo", "ngrok")
	}

	return &DownloadManager{
		BaseURL:  NgrokDownloadBase,
		CacheDir: cacheDir,
	}
}

// GetPlatform returns the current platform string for ngrok download.
func GetPlatform() string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Map Go arch to ngrok arch naming
	arch := goarch
	if goarch == "amd64" {
		arch = "amd64"
	}

	return fmt.Sprintf("%s-%s", goos, arch)
}

// GetDownloadInfo fetches information about the ngrok download.
func (m *DownloadManager) GetDownloadInfo(ctx context.Context) (*NgrokDownloadInfo, error) {
	platform := GetPlatform()
	ext := platformExtensions[platform]
	if ext == "" {
		ext = ".zip" // default
	}

	// Construct download URL
	downloadURL := fmt.Sprintf("%s-%s%s", m.BaseURL, platform, ext)

	// Try to get file size via HEAD request
	req, err := http.NewRequestWithContext(ctx, "HEAD", downloadURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// Return estimated info if we can't fetch
		return &NgrokDownloadInfo{
			Version:     "v3-stable",
			Size:        25 * 1024 * 1024, // ~25MB estimate
			SizeHuman:   "~25 MB",
			DownloadURL: downloadURL,
			Platform:    platform,
		}, nil
	}
	defer resp.Body.Close()

	size := resp.ContentLength
	if size <= 0 {
		size = 25 * 1024 * 1024 // estimate
	}

	return &NgrokDownloadInfo{
		Version:     "v3-stable",
		Size:        size,
		SizeHuman:   formatBytes(size),
		DownloadURL: downloadURL,
		Platform:    platform,
	}, nil
}

// TestMirrorSpeed tests the speed of a single mirror by downloading a small chunk.
func (m *DownloadManager) TestMirrorSpeed(ctx context.Context, mirror MirrorInfo) MirrorSpeedResult {
	result := MirrorSpeedResult{
		URL:       mirror.URL,
		Name:      mirror.Name,
		Reachable: false,
	}

	platform := GetPlatform()
	ext := platformExtensions[platform]
	if ext == "" {
		ext = ".zip"
	}
	downloadURL := fmt.Sprintf("%s-%s%s", mirror.URL, platform, ext)

	// Create context with strict timeout
	testCtx, cancel := context.WithTimeout(ctx, speedTestTimeout)
	defer cancel()

	// Measure connection latency
	start := time.Now()

	// Use custom HTTP client with dual-stack support
	client := createHTTPClient(speedTestTimeout)

	req, err := http.NewRequestWithContext(testCtx, "GET", downloadURL, nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	// Request only first chunk to test speed
	req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", speedTestBytes-1))

	resp, err := client.Do(req)
	if err != nil {
		result.Error = fmt.Sprintf("connection failed: %v", err)
		return result
	}
	defer resp.Body.Close()

	// Record latency (time to first byte)
	result.Latency = time.Since(start)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return result
	}

	result.Reachable = true

	// Download the chunk and measure speed with timeout
	downloadStart := time.Now()

	// Use limited reader to prevent reading too much
	limitedReader := io.LimitReader(resp.Body, speedTestBytes)
	n, err := io.Copy(io.Discard, limitedReader)
	if err != nil && err != io.EOF {
		result.Error = fmt.Sprintf("read failed: %v", err)
		return result
	}

	elapsed := time.Since(downloadStart).Seconds()
	if elapsed > 0 && n > 0 {
		result.Speed = float64(n) / elapsed
	}

	return result
}

// TestAllMirrors tests all mirrors concurrently and returns results sorted by speed.
// Has an overall timeout to prevent blocking indefinitely.
func (m *DownloadManager) TestAllMirrors(ctx context.Context) []MirrorSpeedResult {
	// Overall timeout for all mirror tests
	testCtx, cancel := context.WithTimeout(ctx, speedTestTimeout*2)
	defer cancel()

	results := make([]MirrorSpeedResult, len(ngrokMirrors))
	var wg sync.WaitGroup

	for i, mirror := range ngrokMirrors {
		wg.Add(1)
		go func(idx int, mir MirrorInfo) {
			defer wg.Done()
			results[idx] = m.TestMirrorSpeed(testCtx, mir)
		}(i, mirror)
	}

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All tests completed
	case <-testCtx.Done():
		// Timeout - use whatever results we have
	}

	// Sort by speed (fastest first), unreachable mirrors go to the end
	sort.Slice(results, func(i, j int) bool {
		if !results[i].Reachable && !results[j].Reachable {
			return false
		}
		if !results[i].Reachable {
			return false
		}
		if !results[j].Reachable {
			return true
		}
		return results[i].Speed > results[j].Speed
	})

	return results
}

// GetFastestMirror returns the fastest reachable mirror.
func (m *DownloadManager) GetFastestMirror(ctx context.Context) (*MirrorInfo, error) {
	results := m.TestAllMirrors(ctx)

	for _, result := range results {
		if result.Reachable {
			// Find the mirror info
			for _, mirror := range ngrokMirrors {
				if mirror.URL == result.URL {
					return &mirror, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no reachable mirrors found")
}

// Download downloads the ngrok binary with progress tracking.
// It first tests all mirrors to find the fastest one, then downloads from it.
func (m *DownloadManager) Download(ctx context.Context) error {
	m.mu.Lock()
	if m.downloading {
		m.mu.Unlock()
		return fmt.Errorf("download already in progress")
	}
	m.downloading = true
	ctx, m.cancelFunc = context.WithCancel(ctx)
	m.startTime = time.Now()
	m.currentBytes = 0
	m.currentPhase = "testing_mirrors"
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.downloading = false
		m.cancelFunc = nil
		m.currentMirror = ""
		m.currentPhase = ""
		m.mu.Unlock()
	}()

	// Ensure cache directory exists
	if err := os.MkdirAll(m.CacheDir, 0755); err != nil {
		return err
	}

	platform := GetPlatform()
	ext := platformExtensions[platform]
	if ext == "" {
		ext = ".zip"
	}

	// Test all mirrors and sort by speed
	mirrorResults := m.TestAllMirrors(ctx)

	// Try mirrors in order of speed (fastest first)
	var lastErr error
	for _, result := range mirrorResults {
		if !result.Reachable {
			continue
		}

		downloadURL := fmt.Sprintf("%s-%s%s", result.URL, platform, ext)

		// Update current mirror and phase
		m.mu.Lock()
		m.currentMirror = result.Name
		m.currentPhase = "downloading"
		m.currentBytes = 0
		m.startTime = time.Now()
		m.mu.Unlock()

		err := m.downloadFromURL(ctx, downloadURL, platform)
		if err == nil {
			return nil // Success
		}

		lastErr = err
		// Continue to next mirror
	}

	if lastErr != nil {
		return fmt.Errorf("all download mirrors failed: %w", lastErr)
	}

	return fmt.Errorf("no reachable mirrors found")
}

// downloadFromURL attempts to download ngrok from a specific URL.
// Supports resume from partial downloads and has read timeout protection.
func (m *DownloadManager) downloadFromURL(ctx context.Context, downloadURL, platform string) error {
	// Create HTTP client with dual-stack support
	client := createHTTPClient(downloadConnectTimeout)

	// Check for partial download first
	partialPath := filepath.Join(m.CacheDir, "ngrok.partial")
	var resumeFrom int64 = 0

	if stat, err := os.Stat(partialPath); err == nil {
		resumeFrom = stat.Size()
		m.mu.Lock()
		m.currentBytes = resumeFrom
		m.mu.Unlock()
	}

	// Try to get file size via HEAD request first
	headCtx, headCancel := context.WithTimeout(ctx, downloadConnectTimeout)
	defer headCancel()

	headReq, err := http.NewRequestWithContext(headCtx, "HEAD", downloadURL, nil)
	if err != nil {
		return err
	}

	headResp, err := client.Do(headReq)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", downloadURL, err)
	}
	headResp.Body.Close()

	// Set total size
	totalSize := headResp.ContentLength
	if totalSize <= 0 {
		totalSize = 25 * 1024 * 1024 // ~25MB estimate
	}
	m.mu.Lock()
	m.totalBytes = totalSize
	m.mu.Unlock()

	// Check if we already have the complete file
	if resumeFrom >= totalSize {
		// File is complete, just extract it
		finalPath := m.GetNgrokPath()
		if err := m.extractArchive(partialPath, finalPath, platform); err != nil {
			os.Remove(partialPath)
			return err
		}
		os.Remove(partialPath)
		if runtime.GOOS != "windows" {
			os.Chmod(finalPath, 0755)
		}
		return nil
	}

	// Create download request
	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return err
	}

	// Set Range header for resume
	if resumeFrom > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", resumeFrom))
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Handle response status
	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		// Server doesn't support range or file changed, start fresh
		resumeFrom = 0
		m.mu.Lock()
		m.currentBytes = 0
		m.mu.Unlock()

		// Retry without Range header
		req, _ = http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
		resp, err = client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Update total size from Content-Length if available
	if resp.ContentLength > 0 {
		m.mu.Lock()
		if resp.StatusCode == http.StatusPartialContent {
			m.totalBytes = resumeFrom + resp.ContentLength
		} else {
			m.totalBytes = resp.ContentLength
			m.currentBytes = 0
			resumeFrom = 0
		}
		m.mu.Unlock()
	}

	// Open file for writing
	flags := os.O_CREATE | os.O_WRONLY
	if resp.StatusCode == http.StatusPartialContent && resumeFrom > 0 {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	file, err := os.OpenFile(partialPath, flags, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create progress reader with timeout protection
	reader := &timeoutReader{
		reader:      resp.Body,
		timeout:     downloadReadTimeout,
		onProgress:  m.updateProgress,
		lastRead:    time.Now(),
		minSpeed:    minAcceptableSpeed,
		speedWindow: make([]int64, 0, 10),
	}

	// Copy data
	_, err = io.Copy(file, reader)
	if err != nil {
		// Save progress for resume
		file.Close()
		return fmt.Errorf("download interrupted: %w", err)
	}

	file.Close()

	// Set extracting phase
	m.mu.Lock()
	m.currentPhase = "extracting"
	m.mu.Unlock()

	// Extract the archive
	finalPath := m.GetNgrokPath()
	if err := m.extractArchive(partialPath, finalPath, platform); err != nil {
		os.Remove(partialPath)
		return err
	}

	// Remove the archive
	os.Remove(partialPath)

	// Make executable
	if runtime.GOOS != "windows" {
		if err := os.Chmod(finalPath, 0755); err != nil {
			return err
		}
	}

	// Set complete phase
	m.mu.Lock()
	m.currentPhase = "complete"
	m.mu.Unlock()

	return nil
}

// extractArchive extracts ngrok from the downloaded archive.
func (m *DownloadManager) extractArchive(archivePath, destPath, platform string) error {
	ext := platformExtensions[platform]

	if ext == ".zip" {
		return m.extractZip(archivePath, destPath)
	} else if ext == ".tgz" {
		return m.extractTarGz(archivePath, destPath)
	}

	return fmt.Errorf("unsupported archive format: %s", ext)
}

// extractZip extracts ngrok from a zip archive.
func (m *DownloadManager) extractZip(archivePath, destPath string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		name := filepath.Base(f.Name)
		if name == "ngrok" || name == "ngrok.exe" {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			out, err := os.Create(destPath)
			if err != nil {
				return err
			}
			defer out.Close()

			_, err = io.Copy(out, rc)
			return err
		}
	}

	return fmt.Errorf("ngrok binary not found in archive")
}

// extractTarGz extracts ngrok from a tar.gz archive.
func (m *DownloadManager) extractTarGz(archivePath, destPath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		name := filepath.Base(header.Name)
		if name == "ngrok" {
			out, err := os.Create(destPath)
			if err != nil {
				return err
			}
			defer out.Close()

			_, err = io.Copy(out, tr)
			return err
		}
	}

	return fmt.Errorf("ngrok binary not found in archive")
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
func (m *DownloadManager) GetProgress() NgrokDownloadProgress {
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

	return NgrokDownloadProgress{
		Downloaded: m.currentBytes,
		Total:      m.totalBytes,
		Percentage: percentage,
		Speed:      speed,
		SpeedHuman: formatBytes(int64(speed)) + "/s",
		ETA:        eta,
		StartedAt:  m.startTime,
		Mirror:     m.currentMirror,
		Phase:      m.currentPhase,
	}
}

// GetStatus returns the ngrok installation status.
func (m *DownloadManager) GetStatus(ctx context.Context) (*NgrokStatus, error) {
	status := &NgrokStatus{
		Installed: false,
	}

	// Check downloaded ngrok
	ngrokPath := m.GetNgrokPath()
	if _, err := os.Stat(ngrokPath); err == nil {
		version, err := m.getNgrokVersion(ctx, ngrokPath)
		if err == nil {
			status.Installed = true
			status.Version = version
			status.Path = ngrokPath
		}
	}

	// Check system ngrok if not found
	if !status.Installed {
		systemPath, err := exec.LookPath("ngrok")
		if err == nil {
			version, err := m.getNgrokVersion(ctx, systemPath)
			if err == nil {
				status.Installed = true
				status.Version = version
				status.Path = systemPath
			}
		}
	}

	return status, nil
}

// getNgrokVersion gets the version of ngrok binary.
func (m *DownloadManager) getNgrokVersion(ctx context.Context, path string) (string, error) {
	cmd := exec.CommandContext(ctx, path, "version")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(string(output))
	return version, nil
}

// GetNgrokPath returns the path to the ngrok binary.
func (m *DownloadManager) GetNgrokPath() string {
	name := "ngrok"
	if runtime.GOOS == "windows" {
		name = "ngrok.exe"
	}
	return filepath.Join(m.CacheDir, name)
}

// ClearCache removes downloaded ngrok files.
func (m *DownloadManager) ClearCache() error {
	files := []string{
		filepath.Join(m.CacheDir, "ngrok"),
		filepath.Join(m.CacheDir, "ngrok.exe"),
		filepath.Join(m.CacheDir, "ngrok.partial"),
	}

	for _, f := range files {
		os.Remove(f)
	}

	return nil
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

// timeoutReader wraps an io.Reader with timeout and speed monitoring.
// It fails if no data is received for too long or if speed is too slow.
type timeoutReader struct {
	reader      io.Reader
	timeout     time.Duration
	onProgress  func(n int64)
	lastRead    time.Time
	minSpeed    float64 // minimum bytes per second
	speedWindow []int64 // recent read sizes for speed calculation
	totalRead   int64
	startTime   time.Time
}

func (r *timeoutReader) Read(p []byte) (int, error) {
	if r.startTime.IsZero() {
		r.startTime = time.Now()
	}

	// Check if we've been stuck for too long
	if time.Since(r.lastRead) > r.timeout {
		return 0, fmt.Errorf("read timeout: no data received for %v", r.timeout)
	}

	n, err := r.reader.Read(p)
	if n > 0 {
		r.lastRead = time.Now()
		r.totalRead += int64(n)

		// Track speed
		r.speedWindow = append(r.speedWindow, int64(n))
		if len(r.speedWindow) > 10 {
			r.speedWindow = r.speedWindow[1:]
		}

		// Check speed after some data has been read
		if r.totalRead > 100*1024 { // After 100KB
			elapsed := time.Since(r.startTime).Seconds()
			if elapsed > 5 { // After 5 seconds
				speed := float64(r.totalRead) / elapsed
				if speed < r.minSpeed {
					return n, fmt.Errorf("download too slow: %.0f B/s (minimum: %.0f B/s)", speed, r.minSpeed)
				}
			}
		}

		if r.onProgress != nil {
			r.onProgress(int64(n))
		}
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

// VersionInfo stores version information.
type VersionInfo struct {
	Version     string    `json:"version"`
	DownloadedAt time.Time `json:"downloaded_at"`
}

// SaveVersionInfo saves version info to disk.
func (m *DownloadManager) SaveVersionInfo(version string) error {
	info := VersionInfo{
		Version:     version,
		DownloadedAt: time.Now(),
	}

	data, err := json.Marshal(info)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(m.CacheDir, "version.json"), data, 0644)
}

// LoadVersionInfo loads version info from disk.
func (m *DownloadManager) LoadVersionInfo() (*VersionInfo, error) {
	data, err := os.ReadFile(filepath.Join(m.CacheDir, "version.json"))
	if err != nil {
		return nil, err
	}

	var info VersionInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	return &info, nil
}
