package ngrok

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"
)

func TestGetPlatform(t *testing.T) {
	platform := GetPlatform()

	// Should return a valid platform string
	if platform == "" {
		t.Error("GetPlatform() returned empty string")
	}

	// Should contain OS and arch
	expectedOS := runtime.GOOS
	if expectedOS == "windows" {
		expectedOS = "windows"
	}

	// Platform should be in format "os-arch"
	validPlatforms := []string{
		"darwin-amd64",
		"darwin-arm64",
		"linux-amd64",
		"linux-arm64",
		"windows-amd64",
	}

	found := false
	for _, p := range validPlatforms {
		if platform == p {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("GetPlatform() returned unexpected platform: %s", platform)
	}
}

func TestDownloadManager_NewDownloadManager(t *testing.T) {
	// Test with empty cache dir (should use default)
	dm := NewDownloadManager("")
	if dm == nil {
		t.Fatal("NewDownloadManager() returned nil")
	}
	if dm.CacheDir == "" {
		t.Error("CacheDir should not be empty")
	}

	// Test with custom cache dir
	customDir := filepath.Join(os.TempDir(), "ngrok-test")
	dm2 := NewDownloadManager(customDir)
	if dm2.CacheDir != customDir {
		t.Errorf("CacheDir = %s, want %s", dm2.CacheDir, customDir)
	}
}

func TestDownloadManager_GetDownloadInfo(t *testing.T) {
	dm := NewDownloadManager(filepath.Join(os.TempDir(), "ngrok-test"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	info, err := dm.GetDownloadInfo(ctx)
	if err != nil {
		// Network errors are acceptable in tests
		t.Skipf("Skipping due to network error: %v", err)
	}

	if info == nil {
		t.Fatal("GetDownloadInfo() returned nil")
	}

	if info.Platform == "" {
		t.Error("Platform should not be empty")
	}

	if info.Size <= 0 {
		t.Error("Size should be positive")
	}

	if info.SizeHuman == "" {
		t.Error("SizeHuman should not be empty")
	}
}

func TestDownloadManager_GetStatus_NotInstalled(t *testing.T) {
	// Use a temp directory that doesn't have ngrok
	tempDir := filepath.Join(os.TempDir(), "ngrok-test-empty")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	dm := NewDownloadManager(tempDir)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status, err := dm.GetStatus(ctx)
	if err != nil {
		t.Fatalf("GetStatus() error: %v", err)
	}

	if status.Installed {
		t.Error("Status.Installed should be false for empty directory")
	}
}

func TestDownloadManager_GetNgrokPath(t *testing.T) {
	cacheDir := filepath.Join(os.TempDir(), "ngrok-test")
	dm := NewDownloadManager(cacheDir)

	path := dm.GetNgrokPath()

	expectedName := "ngrok"
	if runtime.GOOS == "windows" {
		expectedName = "ngrok.exe"
	}

	if filepath.Base(path) != expectedName {
		t.Errorf("GetNgrokPath() base = %s, want %s", filepath.Base(path), expectedName)
	}

	if filepath.Dir(path) != cacheDir {
		t.Errorf("GetNgrokPath() dir = %s, want %s", filepath.Dir(path), cacheDir)
	}
}

func TestDownloadManager_IsDownloading(t *testing.T) {
	dm := NewDownloadManager(filepath.Join(os.TempDir(), "ngrok-test"))

	// Initially should not be downloading
	if dm.IsDownloading() {
		t.Error("IsDownloading() should be false initially")
	}
}

func TestDownloadManager_GetProgress_Initial(t *testing.T) {
	dm := NewDownloadManager(filepath.Join(os.TempDir(), "ngrok-test"))

	progress := dm.GetProgress()

	if progress.Downloaded != 0 {
		t.Errorf("Initial Downloaded = %d, want 0", progress.Downloaded)
	}

	if progress.Percentage != 0 {
		t.Errorf("Initial Percentage = %f, want 0", progress.Percentage)
	}
}

func TestDownloadManager_ClearCache(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "ngrok-test-clear")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	// Create some test files
	testFile := filepath.Join(tempDir, "ngrok")
	os.WriteFile(testFile, []byte("test"), 0755)

	dm := NewDownloadManager(tempDir)

	err := dm.ClearCache()
	if err != nil {
		t.Fatalf("ClearCache() error: %v", err)
	}

	// File should be removed
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("ClearCache() should remove ngrok file")
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
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1572864, "1.5 MB"},
		{26738688, "25.5 MB"},
	}

	for _, tt := range tests {
		result := formatBytes(tt.bytes)
		if result != tt.expected {
			t.Errorf("formatBytes(%d) = %s, want %s", tt.bytes, result, tt.expected)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{5 * time.Second, "5s"},
		{30 * time.Second, "30s"},
		{65 * time.Second, "1m 5s"},
		{3600 * time.Second, "1h 0m"},
		{3665 * time.Second, "1h 1m"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.duration)
		if result != tt.expected {
			t.Errorf("formatDuration(%v) = %s, want %s", tt.duration, result, tt.expected)
		}
	}
}

func TestCreateDualStackDialer(t *testing.T) {
	dialer := createDualStackDialer()

	if dialer == nil {
		t.Fatal("createDualStackDialer() returned nil")
	}

	if dialer.Timeout != downloadConnectTimeout {
		t.Errorf("Timeout = %v, want %v", dialer.Timeout, downloadConnectTimeout)
	}

	if !dialer.DualStack {
		t.Error("DualStack should be true")
	}

	if dialer.FallbackDelay != 300*time.Millisecond {
		t.Errorf("FallbackDelay = %v, want 300ms", dialer.FallbackDelay)
	}
}

func TestCreateHTTPClient(t *testing.T) {
	client := createHTTPClient(5 * time.Second)

	if client == nil {
		t.Fatal("createHTTPClient() returned nil")
	}

	// Client timeout should be 0 (we handle timeouts per-operation)
	if client.Timeout != 0 {
		t.Errorf("Client.Timeout = %v, want 0", client.Timeout)
	}

	// Transport should be configured
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Transport is not *http.Transport")
	}

	if transport.TLSHandshakeTimeout != 10*time.Second {
		t.Errorf("TLSHandshakeTimeout = %v, want 10s", transport.TLSHandshakeTimeout)
	}

	if !transport.DisableCompression {
		t.Error("DisableCompression should be true")
	}
}

func TestTimeoutReader_BasicRead(t *testing.T) {
	data := []byte("hello world test data for reading")
	reader := &timeoutReader{
		reader:      bytes.NewReader(data),
		timeout:     5 * time.Second,
		onProgress:  nil,
		lastRead:    time.Now(),
		minSpeed:    0, // Disable speed check for this test
		speedWindow: make([]int64, 0, 10),
	}

	buf := make([]byte, 10)
	n, err := reader.Read(buf)

	if err != nil {
		t.Errorf("Read() error = %v", err)
	}

	if n != 10 {
		t.Errorf("Read() n = %d, want 10", n)
	}

	if string(buf) != "hello worl" {
		t.Errorf("Read() data = %s, want 'hello worl'", string(buf))
	}
}

func TestTimeoutReader_ProgressCallback(t *testing.T) {
	data := []byte("test data")
	var progressCalled int64

	reader := &timeoutReader{
		reader:  bytes.NewReader(data),
		timeout: 5 * time.Second,
		onProgress: func(n int64) {
			progressCalled += n
		},
		lastRead:    time.Now(),
		minSpeed:    0,
		speedWindow: make([]int64, 0, 10),
	}

	buf := make([]byte, 100)
	n, _ := reader.Read(buf)

	if progressCalled != int64(n) {
		t.Errorf("Progress callback received %d bytes, want %d", progressCalled, n)
	}
}

func TestTimeoutReader_SpeedTracking(t *testing.T) {
	// Create a large enough data set to trigger speed checking
	data := make([]byte, 200*1024) // 200KB
	for i := range data {
		data[i] = byte(i % 256)
	}

	reader := &timeoutReader{
		reader:      bytes.NewReader(data),
		timeout:     5 * time.Second,
		onProgress:  nil,
		lastRead:    time.Now(),
		minSpeed:    1, // Very low minimum speed (1 B/s)
		speedWindow: make([]int64, 0, 10),
	}

	// Read all data
	buf := make([]byte, 32*1024)
	totalRead := int64(0)
	for {
		n, err := reader.Read(buf)
		totalRead += int64(n)
		if err != nil {
			break
		}
	}

	if totalRead != int64(len(data)) {
		t.Errorf("Total read = %d, want %d", totalRead, len(data))
	}

	if reader.totalRead != totalRead {
		t.Errorf("reader.totalRead = %d, want %d", reader.totalRead, totalRead)
	}
}

func TestProgressReader_BasicRead(t *testing.T) {
	data := []byte("test data for progress reader")
	var progressTotal int64

	reader := &progressReader{
		reader: bytes.NewReader(data),
		onProgress: func(n int64) {
			progressTotal += n
		},
	}

	buf := make([]byte, 100)
	n, _ := reader.Read(buf)

	if progressTotal != int64(n) {
		t.Errorf("Progress total = %d, want %d", progressTotal, n)
	}
}

func TestDownloadManager_UpdateProgress(t *testing.T) {
	dm := NewDownloadManager(filepath.Join(os.TempDir(), "ngrok-test"))

	// Set initial state
	dm.mu.Lock()
	dm.currentBytes = 0
	dm.totalBytes = 1000
	dm.startTime = time.Now()
	dm.mu.Unlock()

	// Update progress
	dm.updateProgress(100)
	dm.updateProgress(200)

	dm.mu.Lock()
	current := dm.currentBytes
	dm.mu.Unlock()

	if current != 300 {
		t.Errorf("currentBytes = %d, want 300", current)
	}
}

func TestDownloadManager_GetProgress_WithData(t *testing.T) {
	dm := NewDownloadManager(filepath.Join(os.TempDir(), "ngrok-test"))

	// Set up progress state
	dm.mu.Lock()
	dm.currentBytes = 500
	dm.totalBytes = 1000
	dm.startTime = time.Now().Add(-1 * time.Second) // Started 1 second ago
	dm.currentMirror = "TestMirror"
	dm.mu.Unlock()

	progress := dm.GetProgress()

	if progress.Downloaded != 500 {
		t.Errorf("Downloaded = %d, want 500", progress.Downloaded)
	}

	if progress.Total != 1000 {
		t.Errorf("Total = %d, want 1000", progress.Total)
	}

	if progress.Percentage != 50.0 {
		t.Errorf("Percentage = %f, want 50.0", progress.Percentage)
	}

	if progress.Mirror != "TestMirror" {
		t.Errorf("Mirror = %s, want TestMirror", progress.Mirror)
	}

	// Speed should be approximately 500 B/s (500 bytes in ~1 second)
	if progress.Speed < 400 || progress.Speed > 600 {
		t.Errorf("Speed = %f, expected around 500", progress.Speed)
	}
}

func TestDownloadManager_Cancel(t *testing.T) {
	dm := NewDownloadManager(filepath.Join(os.TempDir(), "ngrok-test"))

	// Set up a cancel function
	ctx, cancel := context.WithCancel(context.Background())
	dm.mu.Lock()
	dm.cancelFunc = cancel
	dm.downloading = true
	dm.mu.Unlock()

	// Cancel should not panic
	dm.Cancel()

	// Context should be cancelled
	select {
	case <-ctx.Done():
		// Expected
	default:
		t.Error("Context should be cancelled after Cancel()")
	}
}

func TestMirrorSpeedResult_Sorting(t *testing.T) {
	results := []MirrorSpeedResult{
		{URL: "mirror1", Reachable: true, Speed: 100},
		{URL: "mirror2", Reachable: false, Speed: 0},
		{URL: "mirror3", Reachable: true, Speed: 500},
		{URL: "mirror4", Reachable: true, Speed: 200},
	}

	// Sort by speed (fastest first), unreachable last
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

	// Verify order
	if results[0].URL != "mirror3" {
		t.Errorf("First mirror should be mirror3 (fastest), got %s", results[0].URL)
	}
	if results[1].URL != "mirror4" {
		t.Errorf("Second mirror should be mirror4, got %s", results[1].URL)
	}
	if results[2].URL != "mirror1" {
		t.Errorf("Third mirror should be mirror1, got %s", results[2].URL)
	}
	if results[3].URL != "mirror2" {
		t.Errorf("Last mirror should be mirror2 (unreachable), got %s", results[3].URL)
	}
}

func TestDownloadManager_VersionInfo(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "ngrok-test-version")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	dm := NewDownloadManager(tempDir)

	// Save version info
	err := dm.SaveVersionInfo("v3.5.0")
	if err != nil {
		t.Fatalf("SaveVersionInfo() error: %v", err)
	}

	// Load version info
	info, err := dm.LoadVersionInfo()
	if err != nil {
		t.Fatalf("LoadVersionInfo() error: %v", err)
	}

	if info.Version != "v3.5.0" {
		t.Errorf("Version = %s, want v3.5.0", info.Version)
	}

	if info.DownloadedAt.IsZero() {
		t.Error("DownloadedAt should not be zero")
	}
}

func TestDownloadManager_LoadVersionInfo_NotExists(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "ngrok-test-no-version")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	dm := NewDownloadManager(tempDir)

	// Should return error when file doesn't exist
	_, err := dm.LoadVersionInfo()
	if err == nil {
		t.Error("LoadVersionInfo() should return error when file doesn't exist")
	}
}

func TestNgrokMirrors_Configuration(t *testing.T) {
	// Verify mirrors are configured
	if len(ngrokMirrors) == 0 {
		t.Error("ngrokMirrors should not be empty")
	}

	// First mirror should be official
	if ngrokMirrors[0].Name != "Official" {
		t.Errorf("First mirror should be Official, got %s", ngrokMirrors[0].Name)
	}

	// All mirrors should have URL and Name
	for i, mirror := range ngrokMirrors {
		if mirror.URL == "" {
			t.Errorf("Mirror %d has empty URL", i)
		}
		if mirror.Name == "" {
			t.Errorf("Mirror %d has empty Name", i)
		}
	}
}

func TestPlatformExtensions(t *testing.T) {
	// Verify all expected platforms have extensions
	expectedPlatforms := []string{
		"darwin-amd64",
		"darwin-arm64",
		"linux-amd64",
		"linux-arm64",
		"windows-amd64",
	}

	for _, platform := range expectedPlatforms {
		ext, ok := platformExtensions[platform]
		if !ok {
			t.Errorf("Platform %s not found in platformExtensions", platform)
		}
		if ext != ".zip" && ext != ".tgz" {
			t.Errorf("Platform %s has unexpected extension: %s", platform, ext)
		}
	}

	// Darwin and Windows should use .zip
	if platformExtensions["darwin-amd64"] != ".zip" {
		t.Error("darwin-amd64 should use .zip")
	}
	if platformExtensions["windows-amd64"] != ".zip" {
		t.Error("windows-amd64 should use .zip")
	}

	// Linux should use .tgz
	if platformExtensions["linux-amd64"] != ".tgz" {
		t.Error("linux-amd64 should use .tgz")
	}
}

func TestTimeoutConstants(t *testing.T) {
	// Verify timeout constants are reasonable
	if downloadConnectTimeout < 5*time.Second {
		t.Error("downloadConnectTimeout should be at least 5 seconds")
	}

	if speedTestTimeout < 1*time.Second {
		t.Error("speedTestTimeout should be at least 1 second")
	}

	if downloadReadTimeout < 10*time.Second {
		t.Error("downloadReadTimeout should be at least 10 seconds")
	}

	if minAcceptableSpeed < 1024 {
		t.Error("minAcceptableSpeed should be at least 1 KB/s")
	}

	if speedTestBytes < 1024 {
		t.Error("speedTestBytes should be at least 1 KB")
	}
}

func TestDownloadManager_IsDownloading_Concurrent(t *testing.T) {
	dm := NewDownloadManager(filepath.Join(os.TempDir(), "ngrok-test"))

	// Test concurrent access
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_ = dm.IsDownloading()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestDownloadManager_GetProgress_Concurrent(t *testing.T) {
	dm := NewDownloadManager(filepath.Join(os.TempDir(), "ngrok-test"))

	// Set up some state
	dm.mu.Lock()
	dm.currentBytes = 100
	dm.totalBytes = 1000
	dm.startTime = time.Now()
	dm.mu.Unlock()

	// Test concurrent access
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_ = dm.GetProgress()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
