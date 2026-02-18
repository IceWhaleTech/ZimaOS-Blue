package zimaos

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNewIntegration(t *testing.T) {
	cfg := Config{
		Enabled:     true,
		APIEndpoint: "http://localhost",
		AppID:       "zimaos-blue",
	}

	i := NewIntegration(cfg)
	if i == nil {
		t.Fatal("Expected integration, got nil")
	}

	if i.config.APIEndpoint != "http://localhost" {
		t.Errorf("Expected API endpoint http://localhost, got %s", i.config.APIEndpoint)
	}
}

func TestIntegration_DefaultConfig(t *testing.T) {
	cfg := Config{}
	i := NewIntegration(cfg)

	if i.config.APIEndpoint != "http://localhost" {
		t.Errorf("Expected default API endpoint, got %s", i.config.APIEndpoint)
	}

	if i.config.DataPath != "/DATA/AppData/zimaos-blue" {
		t.Errorf("Expected default data path, got %s", i.config.DataPath)
	}
}

func TestIntegration_IsRunningOnZimaOS(t *testing.T) {
	cfg := Config{}
	i := NewIntegration(cfg)

	// Without ZimaOS environment, should return false
	os.Unsetenv("ZIMAOS_VERSION")
	if i.IsRunningOnZimaOS() {
		t.Error("Expected not running on ZimaOS")
	}

	// With ZimaOS environment variable
	os.Setenv("ZIMAOS_VERSION", "1.0.0")
	defer os.Unsetenv("ZIMAOS_VERSION")

	if !i.IsRunningOnZimaOS() {
		t.Error("Expected running on ZimaOS with env var set")
	}
}

func TestIntegration_GetSystemInfo_NotOnZimaOS(t *testing.T) {
	os.Unsetenv("ZIMAOS_VERSION")

	cfg := Config{}
	i := NewIntegration(cfg)

	ctx := context.Background()
	info, err := i.GetSystemInfo(ctx)
	if err != nil {
		t.Fatalf("GetSystemInfo failed: %v", err)
	}

	if info.Version != "unknown" {
		t.Errorf("Expected version 'unknown', got %s", info.Version)
	}

	if info.Hostname == "" {
		t.Error("Expected hostname to be set")
	}
}

func TestIntegration_GetSystemInfo_OnZimaOS(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/info" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"version": "1.0.0",
				"hostname": "zimaos",
				"architecture": "amd64",
				"total_memory": 8589934592,
				"total_storage": 1099511627776,
				"timezone": "UTC",
				"language": "en"
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	os.Setenv("ZIMAOS_VERSION", "1.0.0")
	defer os.Unsetenv("ZIMAOS_VERSION")

	cfg := Config{
		APIEndpoint: server.URL,
	}
	i := NewIntegration(cfg)

	ctx := context.Background()
	info, err := i.GetSystemInfo(ctx)
	if err != nil {
		t.Fatalf("GetSystemInfo failed: %v", err)
	}

	if info.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %s", info.Version)
	}

	if info.Hostname != "zimaos" {
		t.Errorf("Expected hostname 'zimaos', got %s", info.Hostname)
	}
}

func TestIntegration_ListApps(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/apps" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[
				{"id": "app1", "name": "App 1", "version": "1.0.0", "status": "running"},
				{"id": "app2", "name": "App 2", "version": "2.0.0", "status": "stopped"}
			]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	os.Setenv("ZIMAOS_VERSION", "1.0.0")
	defer os.Unsetenv("ZIMAOS_VERSION")

	cfg := Config{
		APIEndpoint: server.URL,
	}
	i := NewIntegration(cfg)

	ctx := context.Background()
	apps, err := i.ListApps(ctx)
	if err != nil {
		t.Fatalf("ListApps failed: %v", err)
	}

	if len(apps) != 2 {
		t.Errorf("Expected 2 apps, got %d", len(apps))
	}

	if apps[0].ID != "app1" {
		t.Errorf("Expected first app ID 'app1', got %s", apps[0].ID)
	}
}

func TestIntegration_ListApps_NotOnZimaOS(t *testing.T) {
	os.Unsetenv("ZIMAOS_VERSION")

	cfg := Config{}
	i := NewIntegration(cfg)

	ctx := context.Background()
	apps, err := i.ListApps(ctx)
	if err != nil {
		t.Fatalf("ListApps failed: %v", err)
	}

	if len(apps) != 0 {
		t.Errorf("Expected 0 apps when not on ZimaOS, got %d", len(apps))
	}
}

func TestIntegration_GetApp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/apps/app1" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"id": "app1",
				"name": "App 1",
				"version": "1.0.0",
				"status": "running",
				"port": 80,
				"description": "Test app"
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	os.Setenv("ZIMAOS_VERSION", "1.0.0")
	defer os.Unsetenv("ZIMAOS_VERSION")

	cfg := Config{
		APIEndpoint: server.URL,
	}
	i := NewIntegration(cfg)

	ctx := context.Background()
	app, err := i.GetApp(ctx, "app1")
	if err != nil {
		t.Fatalf("GetApp failed: %v", err)
	}

	if app.ID != "app1" {
		t.Errorf("Expected app ID 'app1', got %s", app.ID)
	}

	if app.Port != 80 {
		t.Errorf("Expected port 80, got %d", app.Port)
	}
}

func TestIntegration_ListFiles_Local(t *testing.T) {
	os.Unsetenv("ZIMAOS_VERSION")

	tempDir := t.TempDir()

	// Create test files
	os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tempDir, "file2.txt"), []byte("test2"), 0644)
	os.Mkdir(filepath.Join(tempDir, "subdir"), 0755)

	cfg := Config{}
	i := NewIntegration(cfg)

	ctx := context.Background()
	files, err := i.ListFiles(ctx, tempDir)
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	if len(files) != 3 {
		t.Errorf("Expected 3 files, got %d", len(files))
	}

	// Check for directory
	hasDir := false
	for _, f := range files {
		if f.Name == "subdir" && f.IsDir {
			hasDir = true
			break
		}
	}
	if !hasDir {
		t.Error("Expected to find subdir directory")
	}
}

func TestIntegration_GetDataPath(t *testing.T) {
	cfg := Config{
		DataPath: "/custom/data/path",
	}
	i := NewIntegration(cfg)

	if i.GetDataPath() != "/custom/data/path" {
		t.Errorf("Expected data path '/custom/data/path', got %s", i.GetDataPath())
	}
}

func TestIntegration_GetConfigPath(t *testing.T) {
	cfg := Config{
		DataPath: "/custom/data/path",
	}
	i := NewIntegration(cfg)

	expected := "/custom/data/path/config"
	if i.GetConfigPath() != expected {
		t.Errorf("Expected config path '%s', got %s", expected, i.GetConfigPath())
	}
}

func TestIntegration_SendNotification_NotOnZimaOS(t *testing.T) {
	os.Unsetenv("ZIMAOS_VERSION")

	cfg := Config{}
	i := NewIntegration(cfg)

	ctx := context.Background()
	err := i.SendNotification(ctx, "Test", "Test message", "info")
	if err != nil {
		t.Fatalf("SendNotification failed: %v", err)
	}
}

func TestIntegration_SystemInfoCaching(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/info" {
			callCount++
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"version": "1.0.0",
				"hostname": "zimaos"
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	os.Setenv("ZIMAOS_VERSION", "1.0.0")
	defer os.Unsetenv("ZIMAOS_VERSION")

	cfg := Config{
		APIEndpoint: server.URL,
	}
	i := NewIntegration(cfg)

	ctx := context.Background()

	// First call
	_, err := i.GetSystemInfo(ctx)
	if err != nil {
		t.Fatalf("First GetSystemInfo failed: %v", err)
	}

	// Second call should use cache
	_, err = i.GetSystemInfo(ctx)
	if err != nil {
		t.Fatalf("Second GetSystemInfo failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected 1 API call (cached), got %d", callCount)
	}
}
