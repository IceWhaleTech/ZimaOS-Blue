package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Server defaults
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %v, want %v", cfg.Server.Host, "0.0.0.0")
	}
	if cfg.Server.Port != 23456 {
		t.Errorf("Server.Port = %v, want %v", cfg.Server.Port, 23456)
	}
	if cfg.Server.ReadTimeout != 30*time.Second {
		t.Errorf("Server.ReadTimeout = %v, want %v", cfg.Server.ReadTimeout, 30*time.Second)
	}

	// Log defaults
	if cfg.Log.Level != "info" {
		t.Errorf("Log.Level = %v, want %v", cfg.Log.Level, "info")
	}
	if cfg.Log.Format != "console" {
		t.Errorf("Log.Format = %v, want %v", cfg.Log.Format, "console")
	}

	// Worker defaults
	if cfg.Worker.PoolSize != 10 {
		t.Errorf("Worker.PoolSize = %v, want %v", cfg.Worker.PoolSize, 10)
	}
}

func TestLoad_FromFile(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  host: "127.0.0.1"
  port: 9090
  read_timeout: "60s"
log:
  level: "debug"
  format: "json"
worker:
  pool_size: 20
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Server.Host = %v, want %v", cfg.Server.Host, "127.0.0.1")
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %v, want %v", cfg.Server.Port, 9090)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("Log.Level = %v, want %v", cfg.Log.Level, "debug")
	}
	if cfg.Worker.PoolSize != 20 {
		t.Errorf("Worker.PoolSize = %v, want %v", cfg.Worker.PoolSize, 20)
	}
}

func TestLoad_FromEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("ECHO_SERVER_PORT", "7070")
	os.Setenv("ECHO_LOG_LEVEL", "warn")
	defer func() {
		os.Unsetenv("ECHO_SERVER_PORT")
		os.Unsetenv("ECHO_LOG_LEVEL")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 7070 {
		t.Errorf("Server.Port = %v, want %v", cfg.Server.Port, 7070)
	}
	if cfg.Log.Level != "warn" {
		t.Errorf("Log.Level = %v, want %v", cfg.Log.Level, "warn")
	}
}
