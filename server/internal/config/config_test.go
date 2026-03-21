package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func isolateConfigDiscovery(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(%q) error = %v", tmpDir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})
	t.Setenv("HOME", tmpDir)
}

func TestLoad_Defaults(t *testing.T) {
	isolateConfigDiscovery(t)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Server defaults
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %v, want %v", cfg.Server.Host, "0.0.0.0")
	}
	if cfg.Server.Port != 80 {
		t.Errorf("Server.Port = %v, want %v", cfg.Server.Port, 80)
	}
	if cfg.Server.ReadTimeout != 5*time.Minute {
		t.Errorf("Server.ReadTimeout = %v, want %v", cfg.Server.ReadTimeout, 5*time.Minute)
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
	if !cfg.Browser.Headless {
		t.Errorf("Browser.Headless = %v, want true", cfg.Browser.Headless)
	}
	if cfg.Browser.RelayEnabled {
		t.Errorf("Browser.RelayEnabled = %v, want false", cfg.Browser.RelayEnabled)
	}
	if cfg.Browser.RelayHost != "127.0.0.1" {
		t.Errorf("Browser.RelayHost = %v, want %v", cfg.Browser.RelayHost, "127.0.0.1")
	}
	if cfg.Browser.RelayPort != 18792 {
		t.Errorf("Browser.RelayPort = %v, want %v", cfg.Browser.RelayPort, 18792)
	}
	if cfg.Browser.DefaultTimeout != 60000 {
		t.Errorf("Browser.DefaultTimeout = %v, want %v", cfg.Browser.DefaultTimeout, 60000)
	}
	if cfg.Browser.MaxTimeout != 300000 {
		t.Errorf("Browser.MaxTimeout = %v, want %v", cfg.Browser.MaxTimeout, 300000)
	}
	if cfg.ToolCalling.WebFetch.Timeout != 5*time.Minute {
		t.Errorf("ToolCalling.WebFetch.Timeout = %v, want %v", cfg.ToolCalling.WebFetch.Timeout, 5*time.Minute)
	}
	if cfg.ToolCalling.WebFetch.FirecrawlTimeout != 5*time.Minute {
		t.Errorf("ToolCalling.WebFetch.FirecrawlTimeout = %v, want %v", cfg.ToolCalling.WebFetch.FirecrawlTimeout, 5*time.Minute)
	}
	if cfg.Performance.Database.ConnMaxIdleTime != 10*time.Minute {
		t.Errorf("Performance.Database.ConnMaxIdleTime = %v, want %v", cfg.Performance.Database.ConnMaxIdleTime, 10*time.Minute)
	}
	if !cfg.Performance.ResourceReclaim.Enabled {
		t.Errorf("Performance.ResourceReclaim.Enabled = %v, want true", cfg.Performance.ResourceReclaim.Enabled)
	}
	if cfg.Performance.ResourceReclaim.BrowserIdleAfter != 10*time.Minute {
		t.Errorf("Performance.ResourceReclaim.BrowserIdleAfter = %v, want %v", cfg.Performance.ResourceReclaim.BrowserIdleAfter, 10*time.Minute)
	}
	if cfg.Performance.ResourceReclaim.WorkflowIdleAfter != 15*time.Minute {
		t.Errorf("Performance.ResourceReclaim.WorkflowIdleAfter = %v, want %v", cfg.Performance.ResourceReclaim.WorkflowIdleAfter, 15*time.Minute)
	}
	if cfg.Performance.ResourceReclaim.CronIdleAfter != 15*time.Minute {
		t.Errorf("Performance.ResourceReclaim.CronIdleAfter = %v, want %v", cfg.Performance.ResourceReclaim.CronIdleAfter, 15*time.Minute)
	}
	if cfg.Performance.ResourceReclaim.STTIdleAfter != 10*time.Minute {
		t.Errorf("Performance.ResourceReclaim.STTIdleAfter = %v, want %v", cfg.Performance.ResourceReclaim.STTIdleAfter, 10*time.Minute)
	}
	if !cfg.Performance.ResourceReclaim.SpeechStatusDoesNotPrewarmSTT {
		t.Errorf("Performance.ResourceReclaim.SpeechStatusDoesNotPrewarmSTT = %v, want true", cfg.Performance.ResourceReclaim.SpeechStatusDoesNotPrewarmSTT)
	}
	if !cfg.Session.Audit.Enabled {
		t.Errorf("Session.Audit.Enabled = %v, want true", cfg.Session.Audit.Enabled)
	}
	if cfg.Session.Audit.RetentionDays != 30 {
		t.Errorf("Session.Audit.RetentionDays = %v, want %v", cfg.Session.Audit.RetentionDays, 30)
	}
	if cfg.Session.Persistence.Path != "./data/blue.db" {
		t.Errorf("Session.Persistence.Path = %v, want %v", cfg.Session.Persistence.Path, "./data/blue.db")
	}
	if cfg.Security.Password.MinLength != 8 {
		t.Errorf("Security.Password.MinLength = %v, want %v", cfg.Security.Password.MinLength, 8)
	}
}

func TestLoad_DefaultPasswordMinLength(t *testing.T) {
	isolateConfigDiscovery(t)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Security.Password.MinLength != 8 {
		t.Fatalf("Security.Password.MinLength = %v, want %v", cfg.Security.Password.MinLength, 8)
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
browser:
  driver: "relay"
  relay_enabled: true
  relay_port: 18792
  relay_token: "relay-test-token"
  headless: false
  pool_size: 1
  cdp_url: "http://127.0.0.1:18792?token=test-token"
performance:
  database:
    conn_max_idle_time: "2m"
  resource_reclaim:
    stt_idle_after: "3m"
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
	if cfg.Browser.Headless {
		t.Errorf("Browser.Headless = %v, want false", cfg.Browser.Headless)
	}
	if cfg.Browser.PoolSize != 1 {
		t.Errorf("Browser.PoolSize = %v, want %v", cfg.Browser.PoolSize, 1)
	}
	if got := cfg.Browser.ResolvedDriver(); got != "relay" {
		t.Errorf("Browser.ResolvedDriver() = %v, want %v", got, "relay")
	}
	if !cfg.Browser.RelayEnabled {
		t.Fatalf("Browser.RelayEnabled = false, want true")
	}
	if cfg.Browser.RelayPort != 18792 {
		t.Fatalf("Browser.RelayPort = %v, want %v", cfg.Browser.RelayPort, 18792)
	}
	if cfg.Browser.RelayToken != "relay-test-token" {
		t.Fatalf("Browser.RelayToken = %v, want %v", cfg.Browser.RelayToken, "relay-test-token")
	}
	if cfg.Browser.CDPURL != "http://127.0.0.1:18792?token=test-token" {
		t.Errorf("Browser.CDPURL = %v, want %v", cfg.Browser.CDPURL, "http://127.0.0.1:18792?token=test-token")
	}
	if cfg.Performance.Database.ConnMaxIdleTime != 2*time.Minute {
		t.Errorf("Performance.Database.ConnMaxIdleTime = %v, want %v", cfg.Performance.Database.ConnMaxIdleTime, 2*time.Minute)
	}
	if cfg.Performance.ResourceReclaim.STTIdleAfter != 3*time.Minute {
		t.Errorf("Performance.ResourceReclaim.STTIdleAfter = %v, want %v", cfg.Performance.ResourceReclaim.STTIdleAfter, 3*time.Minute)
	}
}

func TestLoad_FromEnv(t *testing.T) {
	isolateConfigDiscovery(t)

	// Set environment variables
	os.Setenv("BLUE_SERVER_HOST", "127.0.0.1")
	os.Setenv("BLUE_SERVER_PORT", "7070")
	os.Setenv("BLUE_LOG_LEVEL", "warn")
	os.Setenv("BLUE_WEB_FETCH_ALLOW_PRIVATE_HOSTS", "true")
	os.Setenv("BLUE_WEB_FETCH_TIMEOUT", "12s")
	os.Setenv("BLUE_WEB_FETCH_FIRECRAWL_TIMEOUT", "18s")
	defer func() {
		os.Unsetenv("BLUE_SERVER_HOST")
		os.Unsetenv("BLUE_SERVER_PORT")
		os.Unsetenv("BLUE_LOG_LEVEL")
		os.Unsetenv("BLUE_WEB_FETCH_ALLOW_PRIVATE_HOSTS")
		os.Unsetenv("BLUE_WEB_FETCH_TIMEOUT")
		os.Unsetenv("BLUE_WEB_FETCH_FIRECRAWL_TIMEOUT")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 7070 {
		t.Errorf("Server.Port = %v, want %v", cfg.Server.Port, 7070)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Server.Host = %v, want %v", cfg.Server.Host, "127.0.0.1")
	}
	if cfg.Log.Level != "warn" {
		t.Errorf("Log.Level = %v, want %v", cfg.Log.Level, "warn")
	}
	if !cfg.ToolCalling.WebFetch.AllowPrivateHosts {
		t.Fatalf("ToolCalling.WebFetch.AllowPrivateHosts = %v, want true", cfg.ToolCalling.WebFetch.AllowPrivateHosts)
	}
	if cfg.ToolCalling.WebFetch.Timeout != 12*time.Second {
		t.Fatalf("ToolCalling.WebFetch.Timeout = %v, want %v", cfg.ToolCalling.WebFetch.Timeout, 12*time.Second)
	}
	if cfg.ToolCalling.WebFetch.FirecrawlTimeout != 18*time.Second {
		t.Fatalf("ToolCalling.WebFetch.FirecrawlTimeout = %v, want %v", cfg.ToolCalling.WebFetch.FirecrawlTimeout, 18*time.Second)
	}
}
