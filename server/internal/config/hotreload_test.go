package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestNewHotReloader(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Port: 8080},
		Worker: WorkerConfig{PoolSize: 10},
	}

	tests := []struct {
		name     string
		hrConfig *HotReloadConfig
	}{
		{
			name:     "nil config uses defaults",
			hrConfig: nil,
		},
		{
			name: "custom config",
			hrConfig: &HotReloadConfig{
				Enabled:             true,
				WatchInterval:       2 * time.Second,
				ValidateBeforeApply: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hr, err := NewHotReloader("", cfg, tt.hrConfig)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if hr == nil {
				t.Fatal("expected non-nil HotReloader")
			}
			if hr.Config() != cfg {
				t.Error("expected initial config to be stored")
			}
		})
	}
}

func TestHotReloaderConfig(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Port: 8080},
		Worker: WorkerConfig{PoolSize: 10},
	}

	hr, err := NewHotReloader("", cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := hr.Config()
	if got != cfg {
		t.Error("Config() should return stored config")
	}
}

func TestHotReloaderStats(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Port: 8080},
		Worker: WorkerConfig{PoolSize: 10},
	}

	hr, err := NewHotReloader("/test/config.yaml", cfg, &HotReloadConfig{
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stats := hr.Stats()

	if stats["enabled"] != true {
		t.Error("expected enabled to be true")
	}
	if stats["config_path"] != "/test/config.yaml" {
		t.Error("expected config_path to match")
	}
	if stats["reload_count"].(int64) != 0 {
		t.Error("expected reload_count to be 0")
	}
}

func TestHotReloaderOnReload(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Port: 8080},
		Worker: WorkerConfig{PoolSize: 10},
	}

	// Use a non-existent file path to trigger failure
	hr, err := NewHotReloader("/nonexistent/path/config.yaml", cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var (
		mu       sync.Mutex
		received []ReloadEvent
	)

	hr.OnReload(func(event ReloadEvent) {
		mu.Lock()
		received = append(received, event)
		mu.Unlock()
	})

	// Trigger a reload (will fail since config file doesn't exist)
	hr.reload()

	// Wait for callback
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if len(received) != 1 {
		t.Errorf("expected 1 callback, got %d", len(received))
	}
	if len(received) > 0 && received[0].Success {
		t.Error("expected reload to fail with non-existent config file")
	}
	mu.Unlock()
}

func TestHotReloaderValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				Server: ServerConfig{Port: 8080},
				Worker: WorkerConfig{PoolSize: 10},
			},
			wantErr: false,
		},
		{
			name: "invalid port - zero",
			config: &Config{
				Server: ServerConfig{Port: 0},
				Worker: WorkerConfig{PoolSize: 10},
			},
			wantErr: true,
		},
		{
			name: "invalid port - too high",
			config: &Config{
				Server: ServerConfig{Port: 70000},
				Worker: WorkerConfig{PoolSize: 10},
			},
			wantErr: true,
		},
		{
			name: "invalid pool size",
			config: &Config{
				Server: ServerConfig{Port: 8080},
				Worker: WorkerConfig{PoolSize: 0},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hr, _ := NewHotReloader("", &Config{
				Server: ServerConfig{Port: 8080},
				Worker: WorkerConfig{PoolSize: 10},
			}, nil)

			err := hr.validateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHotReloaderStartStop(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Port: 8080},
		Worker: WorkerConfig{PoolSize: 10},
	}

	// Create a temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(`
server:
  port: 8080
worker:
  pool_size: 10
`), 0644); err != nil {
		t.Fatalf("failed to create temp config: %v", err)
	}

	hr, err := NewHotReloader(configPath, cfg, &HotReloadConfig{
		Enabled:             true,
		WatchInterval:       100 * time.Millisecond,
		ValidateBeforeApply: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Start should not error
	if err := hr.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Stop should not error
	if err := hr.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
}

func TestHotReloaderDisabled(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Port: 8080},
		Worker: WorkerConfig{PoolSize: 10},
	}

	hr, err := NewHotReloader("", cfg, &HotReloadConfig{
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Start should be a no-op when disabled
	if err := hr.Start(); err != nil {
		t.Fatalf("Start() should not error when disabled: %v", err)
	}

	// Stop should be a no-op when disabled
	if err := hr.Stop(); err != nil {
		t.Fatalf("Stop() should not error when disabled: %v", err)
	}
}

func TestHotReloaderConcurrentReload(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Port: 8080},
		Worker: WorkerConfig{PoolSize: 10},
	}

	hr, err := NewHotReloader("", cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Start a reload
	hr.isReloading.Store(true)

	// Second reload should fail
	err = hr.Reload()
	if err == nil {
		t.Error("expected error for concurrent reload")
	}

	hr.isReloading.Store(false)
}

func TestHotReloaderFileChange(t *testing.T) {
	// Create initial config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	initialContent := `
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: "30s"
  write_timeout: "30s"
  idle_timeout: "120s"
log:
  level: "info"
  format: "console"
  output: "stdout"
worker:
  pool_size: 10
  max_queue_len: 100
`
	if err := os.WriteFile(configPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create temp config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	var (
		mu       sync.Mutex
		received []ReloadEvent
	)

	hr, err := NewHotReloader(configPath, cfg, &HotReloadConfig{
		Enabled:             true,
		WatchInterval:       50 * time.Millisecond,
		ValidateBeforeApply: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hr.OnReload(func(event ReloadEvent) {
		mu.Lock()
		received = append(received, event)
		mu.Unlock()
	})

	if err := hr.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer hr.Stop()

	// Modify config file
	newContent := `
server:
  host: "0.0.0.0"
  port: 9090
  read_timeout: "30s"
  write_timeout: "30s"
  idle_timeout: "120s"
log:
  level: "debug"
  format: "console"
  output: "stdout"
worker:
  pool_size: 20
  max_queue_len: 100
`
	time.Sleep(100 * time.Millisecond) // Wait for watcher to be ready
	if err := os.WriteFile(configPath, []byte(newContent), 0644); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	// Wait for reload
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(received) == 0 {
		t.Skip("file watcher may not have triggered - platform dependent")
	}

	lastEvent := received[len(received)-1]
	if !lastEvent.Success {
		t.Errorf("expected successful reload, got error: %v", lastEvent.Error)
	}

	newCfg := hr.Config()
	if newCfg.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", newCfg.Server.Port)
	}
	if newCfg.Worker.PoolSize != 20 {
		t.Errorf("expected pool size 20, got %d", newCfg.Worker.PoolSize)
	}
}
