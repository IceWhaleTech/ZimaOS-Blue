package claudecode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNewDownloadManager(t *testing.T) {
	// Test with empty cache dir
	dm := NewDownloadManager("")
	if dm == nil {
		t.Fatal("Expected non-nil DownloadManager")
	}

	if dm.BaseURL == "" {
		t.Error("BaseURL should not be empty")
	}

	if dm.CacheDir == "" {
		t.Error("CacheDir should not be empty")
	}

	// Test with custom cache dir
	customDir := "/tmp/test-cache"
	dm2 := NewDownloadManager(customDir)
	if dm2.CacheDir != customDir {
		t.Errorf("CacheDir = %q, want %q", dm2.CacheDir, customDir)
	}
}

func TestGetPlatform(t *testing.T) {
	platform := GetPlatform()
	if platform == "" {
		t.Error("Platform should not be empty")
	}

	// Should contain OS and arch
	parts := strings.Split(platform, "-")
	if len(parts) != 2 {
		t.Errorf("Platform should be in format 'os-arch', got %q", platform)
	}

	// Verify OS mapping
	switch runtime.GOOS {
	case "darwin":
		if !strings.HasPrefix(platform, "darwin-") {
			t.Errorf("Expected darwin prefix, got %q", platform)
		}
	case "linux":
		if !strings.HasPrefix(platform, "linux-") {
			t.Errorf("Expected linux prefix, got %q", platform)
		}
	case "windows":
		if !strings.HasPrefix(platform, "win32-") {
			t.Errorf("Expected win32 prefix, got %q", platform)
		}
	}

	// Verify arch mapping
	switch runtime.GOARCH {
	case "amd64":
		if !strings.HasSuffix(platform, "-x64") {
			t.Errorf("Expected x64 suffix for amd64, got %q", platform)
		}
	case "arm64":
		if !strings.HasSuffix(platform, "-arm64") {
			t.Errorf("Expected arm64 suffix, got %q", platform)
		}
	}
}

func TestDownloadManager_GetDownloadInfo(t *testing.T) {
	// Create mock server
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "latest.json") {
			info := DownloadInfo{
				Version:     "1.0.0",
				Size:        1024 * 1024,
				DownloadURL: "https://example.com/claude",
				Checksum:    "abc123",
			}
			json.NewEncoder(w).Encode(info)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	dm := NewDownloadManager(t.TempDir())
	dm.BaseURL = server.URL

	ctx := context.Background()
	info, err := dm.GetDownloadInfo(ctx)
	if err != nil {
		t.Fatalf("GetDownloadInfo error: %v", err)
	}

	if info.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", info.Version, "1.0.0")
	}

	if info.Size != 1024*1024 {
		t.Errorf("Size = %d, want %d", info.Size, 1024*1024)
	}

	if info.Platform == "" {
		t.Error("Platform should not be empty")
	}

	if info.SizeHuman == "" {
		t.Error("SizeHuman should not be empty")
	}
}

func TestDownloadManager_GetDownloadInfo_Fallback(t *testing.T) {
	// Create mock server that returns 404
	server := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	dm := NewDownloadManager(t.TempDir())
	dm.BaseURL = server.URL

	ctx := context.Background()
	info, err := dm.GetDownloadInfo(ctx)
	if err != nil {
		t.Fatalf("GetDownloadInfo error: %v", err)
	}

	// Should return estimated info
	if info.Version != "latest" {
		t.Errorf("Version = %q, want %q", info.Version, "latest")
	}

	if info.Size == 0 {
		t.Error("Size should not be 0")
	}
}

func TestDownloadManager_IsDownloading(t *testing.T) {
	dm := NewDownloadManager(t.TempDir())

	if dm.IsDownloading() {
		t.Error("Should not be downloading initially")
	}
}

func TestDownloadManager_GetProgress(t *testing.T) {
	dm := NewDownloadManager(t.TempDir())
	dm.startTime = time.Now().Add(-10 * time.Second)
	dm.currentBytes = 1024 * 1024
	dm.totalBytes = 10 * 1024 * 1024

	progress := dm.GetProgress()

	if progress.Downloaded != 1024*1024 {
		t.Errorf("Downloaded = %d, want %d", progress.Downloaded, 1024*1024)
	}

	if progress.Total != 10*1024*1024 {
		t.Errorf("Total = %d, want %d", progress.Total, 10*1024*1024)
	}

	if progress.Percentage != 10.0 {
		t.Errorf("Percentage = %f, want %f", progress.Percentage, 10.0)
	}

	if progress.Speed <= 0 {
		t.Error("Speed should be positive")
	}

	if progress.SpeedHuman == "" {
		t.Error("SpeedHuman should not be empty")
	}

	if progress.ETA == "" {
		t.Error("ETA should not be empty")
	}
}

func TestDownloadManager_Cancel(t *testing.T) {
	dm := NewDownloadManager(t.TempDir())

	// Cancel should not panic when not downloading
	dm.Cancel()

	// Set up a cancel function
	ctx, cancel := context.WithCancel(context.Background())
	dm.cancelFunc = cancel

	dm.Cancel()

	// Context should be cancelled
	select {
	case <-ctx.Done():
		// Expected
	default:
		t.Error("Context should be cancelled")
	}
}

func TestDownloadManager_GetCLIPath(t *testing.T) {
	cacheDir := "/tmp/test-cache"
	dm := NewDownloadManager(cacheDir)

	path := dm.GetCLIPath()

	if !strings.HasPrefix(path, cacheDir) {
		t.Errorf("Path should start with cache dir, got %q", path)
	}

	if runtime.GOOS == "windows" {
		if !strings.HasSuffix(path, ".exe") {
			t.Error("Path should end with .exe on Windows")
		}
	} else {
		if strings.HasSuffix(path, ".exe") {
			t.Error("Path should not end with .exe on non-Windows")
		}
	}
}

func TestDownloadManager_ClearCache(t *testing.T) {
	cacheDir := t.TempDir()
	dm := NewDownloadManager(cacheDir)

	// Create some test files
	testFiles := []string{
		filepath.Join(cacheDir, "claude"),
		filepath.Join(cacheDir, "claude.partial"),
	}

	for _, f := range testFiles {
		if err := os.WriteFile(f, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	// Clear cache
	if err := dm.ClearCache(); err != nil {
		t.Fatalf("ClearCache error: %v", err)
	}

	// Verify files are removed
	for _, f := range testFiles {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Errorf("File %q should be removed", f)
		}
	}
}

func TestDownloadManager_verifyChecksum(t *testing.T) {
	cacheDir := t.TempDir()
	dm := NewDownloadManager(cacheDir)

	// Create test file
	testContent := []byte("test content for checksum")
	testFile := filepath.Join(cacheDir, "test-file")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Calculate expected checksum
	hash := sha256.Sum256(testContent)
	expectedChecksum := hex.EncodeToString(hash[:])

	// Test valid checksum
	if err := dm.verifyChecksum(testFile, expectedChecksum); err != nil {
		t.Errorf("verifyChecksum should pass with correct checksum: %v", err)
	}

	// Test invalid checksum
	if err := dm.verifyChecksum(testFile, "invalid-checksum"); err == nil {
		t.Error("verifyChecksum should fail with incorrect checksum")
	}

	// Test non-existent file
	if err := dm.verifyChecksum("/non/existent/file", expectedChecksum); err == nil {
		t.Error("verifyChecksum should fail with non-existent file")
	}
}

func TestProgressReader(t *testing.T) {
	content := []byte("test content for progress reader")
	reader := strings.NewReader(string(content))

	var totalRead int64
	pr := &progressReader{
		reader: reader,
		onProgress: func(n int64) {
			totalRead += n
		},
	}

	buf := make([]byte, 10)
	for {
		n, err := pr.Read(buf)
		if n == 0 || err != nil {
			break
		}
	}

	if totalRead != int64(len(content)) {
		t.Errorf("Total read = %d, want %d", totalRead, len(content))
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1024, "1.0 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{1536, "1.5 KB"},
	}

	for _, tt := range tests {
		result := formatBytes(tt.bytes)
		if result != tt.expected {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.bytes, result, tt.expected)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m 30s"},
		{3661 * time.Second, "1h 1m"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.duration)
		if result != tt.expected {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, result, tt.expected)
		}
	}
}

func TestDownloadManager_Download_AlreadyInProgress(t *testing.T) {
	dm := NewDownloadManager(t.TempDir())
	dm.downloading = true

	err := dm.Download(context.Background())
	if err == nil {
		t.Error("Download should fail when already in progress")
	}

	if !strings.Contains(err.Error(), "already in progress") {
		t.Errorf("Error should mention 'already in progress', got %q", err.Error())
	}
}

func TestDownloadManager_GetCLIStatus_NotInstalled(t *testing.T) {
	cacheDir := t.TempDir()
	dm := NewDownloadManager(cacheDir)

	ctx := context.Background()
	status, err := dm.GetCLIStatus(ctx)
	if err != nil {
		t.Fatalf("GetCLIStatus error: %v", err)
	}

	// CLI should not be installed in temp dir
	if status.Installed {
		t.Error("CLI should not be installed in temp dir")
	}
}

func TestDownloadManager_updateProgress(t *testing.T) {
	dm := NewDownloadManager(t.TempDir())
	dm.startTime = time.Now()

	var progressCalled bool
	dm.OnProgress = func(progress DownloadProgress) {
		progressCalled = true
	}

	dm.updateProgress(1024)

	if dm.currentBytes != 1024 {
		t.Errorf("currentBytes = %d, want %d", dm.currentBytes, 1024)
	}

	if !progressCalled {
		t.Error("OnProgress callback should be called")
	}
}

func TestDownloadInfo_Fields(t *testing.T) {
	info := DownloadInfo{
		Version:     "1.0.0",
		Size:        1024,
		SizeHuman:   "1 KB",
		DownloadURL: "https://example.com/cli",
		Checksum:    "abc123",
		Platform:    "darwin-arm64",
	}

	if info.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", info.Version, "1.0.0")
	}

	if info.Size != 1024 {
		t.Errorf("Size = %d, want %d", info.Size, 1024)
	}

	if info.Platform != "darwin-arm64" {
		t.Errorf("Platform = %q, want %q", info.Platform, "darwin-arm64")
	}
}

func TestDownloadProgress_Fields(t *testing.T) {
	now := time.Now()
	progress := DownloadProgress{
		Downloaded: 1024,
		Total:      2048,
		Percentage: 50.0,
		Speed:      100.0,
		SpeedHuman: "100 B/s",
		ETA:        "10s",
		StartedAt:  now,
	}

	if progress.Downloaded != 1024 {
		t.Errorf("Downloaded = %d, want %d", progress.Downloaded, 1024)
	}

	if progress.Percentage != 50.0 {
		t.Errorf("Percentage = %f, want %f", progress.Percentage, 50.0)
	}

	if progress.StartedAt != now {
		t.Error("StartedAt should match")
	}
}

func TestCLIStatus_Fields(t *testing.T) {
	status := CLIStatus{
		Installed:       true,
		Version:         "1.0.0",
		Path:            "/usr/local/bin/claude",
		Source:          "system",
		UpdateAvailable: true,
		LatestVersion:   "1.1.0",
	}

	if !status.Installed {
		t.Error("Installed should be true")
	}

	if status.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", status.Version, "1.0.0")
	}

	if status.Source != "system" {
		t.Errorf("Source = %q, want %q", status.Source, "system")
	}

	if !status.UpdateAvailable {
		t.Error("UpdateAvailable should be true")
	}
}
