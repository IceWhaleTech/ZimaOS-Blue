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
	if !cfg.Browser.Headless {
		t.Errorf("Browser.Headless = %v, want true", cfg.Browser.Headless)
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
	if cfg.Research.Router.DefaultMode != "web" {
		t.Errorf("Research.Router.DefaultMode = %v, want %v", cfg.Research.Router.DefaultMode, "web")
	}
	if cfg.Research.Autoresearch.Enabled {
		t.Errorf("Research.Autoresearch.Enabled = %v, want false", cfg.Research.Autoresearch.Enabled)
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
  headless: false
  pool_size: 1
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
}

func TestLoad_FromEnv(t *testing.T) {
	isolateConfigDiscovery(t)

	// Set environment variables
	os.Setenv("BLUE_SERVER_PORT", "7070")
	os.Setenv("BLUE_LOG_LEVEL", "warn")
	os.Setenv("BLUE_WEB_FETCH_ALLOW_PRIVATE_HOSTS", "true")
	os.Setenv("BLUE_WEB_FETCH_TIMEOUT", "12s")
	os.Setenv("BLUE_RESEARCH_DEFAULT_ROUTE_MODE", "hybrid")
	os.Setenv("BLUE_RESEARCH_ALLOW_EXPERIMENT", "false")
	os.Setenv("BLUE_RESEARCH_ALLOW_HYBRID", "false")
	os.Setenv("BLUE_AUTORESEARCH_ENABLED", "true")
	os.Setenv("BLUE_AUTORESEARCH_COMMAND", "uv")
	os.Setenv("BLUE_AUTORESEARCH_WORKING_DIR", "/tmp/autoresearch")
	os.Setenv("BLUE_AUTORESEARCH_ARGS", "[\"run\",\"train.py\",\"--seed\",\"1337\"]")
	os.Setenv("BLUE_AUTORESEARCH_ARTIFACT_DIR", "/tmp/autoresearch-artifacts")
	os.Setenv("BLUE_AUTORESEARCH_ENV_JSON", "{\"WANDB_MODE\":\"disabled\",\"CUDA_VISIBLE_DEVICES\":\"0\"}")
	os.Setenv("BLUE_AUTORESEARCH_TIMEOUT", "30m")
	defer func() {
		os.Unsetenv("BLUE_SERVER_PORT")
		os.Unsetenv("BLUE_LOG_LEVEL")
		os.Unsetenv("BLUE_WEB_FETCH_ALLOW_PRIVATE_HOSTS")
		os.Unsetenv("BLUE_WEB_FETCH_TIMEOUT")
		os.Unsetenv("BLUE_RESEARCH_DEFAULT_ROUTE_MODE")
		os.Unsetenv("BLUE_RESEARCH_ALLOW_EXPERIMENT")
		os.Unsetenv("BLUE_RESEARCH_ALLOW_HYBRID")
		os.Unsetenv("BLUE_AUTORESEARCH_ENABLED")
		os.Unsetenv("BLUE_AUTORESEARCH_COMMAND")
		os.Unsetenv("BLUE_AUTORESEARCH_WORKING_DIR")
		os.Unsetenv("BLUE_AUTORESEARCH_ARGS")
		os.Unsetenv("BLUE_AUTORESEARCH_ARTIFACT_DIR")
		os.Unsetenv("BLUE_AUTORESEARCH_ENV_JSON")
		os.Unsetenv("BLUE_AUTORESEARCH_TIMEOUT")
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
	if !cfg.ToolCalling.WebFetch.AllowPrivateHosts {
		t.Fatalf("ToolCalling.WebFetch.AllowPrivateHosts = %v, want true", cfg.ToolCalling.WebFetch.AllowPrivateHosts)
	}
	if cfg.ToolCalling.WebFetch.Timeout != 12*time.Second {
		t.Fatalf("ToolCalling.WebFetch.Timeout = %v, want %v", cfg.ToolCalling.WebFetch.Timeout, 12*time.Second)
	}
	if cfg.Research.Router.DefaultMode != "hybrid" {
		t.Fatalf("Research.Router.DefaultMode = %v, want %v", cfg.Research.Router.DefaultMode, "hybrid")
	}
	if cfg.Research.Router.AllowExperiment {
		t.Fatal("Research.Router.AllowExperiment = true, want false")
	}
	if cfg.Research.Router.AllowHybrid {
		t.Fatal("Research.Router.AllowHybrid = true, want false")
	}
	if !cfg.Research.Autoresearch.Enabled {
		t.Fatal("Research.Autoresearch.Enabled = false, want true")
	}
	if cfg.Research.Autoresearch.Command != "uv" {
		t.Fatalf("Research.Autoresearch.Command = %v, want %v", cfg.Research.Autoresearch.Command, "uv")
	}
	if cfg.Research.Autoresearch.WorkingDir != "/tmp/autoresearch" {
		t.Fatalf("Research.Autoresearch.WorkingDir = %v, want %v", cfg.Research.Autoresearch.WorkingDir, "/tmp/autoresearch")
	}
	if len(cfg.Research.Autoresearch.Args) != 4 {
		t.Fatalf("len(Research.Autoresearch.Args) = %d, want 4", len(cfg.Research.Autoresearch.Args))
	}
	if cfg.Research.Autoresearch.Args[0] != "run" || cfg.Research.Autoresearch.Args[1] != "train.py" {
		t.Fatalf("Research.Autoresearch.Args = %#v, want [run train.py --seed 1337]", cfg.Research.Autoresearch.Args)
	}
	if cfg.Research.Autoresearch.ArtifactDir != "/tmp/autoresearch-artifacts" {
		t.Fatalf("Research.Autoresearch.ArtifactDir = %v, want %v", cfg.Research.Autoresearch.ArtifactDir, "/tmp/autoresearch-artifacts")
	}
	if got := cfg.Research.Autoresearch.Env["WANDB_MODE"]; got != "disabled" {
		t.Fatalf("Research.Autoresearch.Env[WANDB_MODE] = %q, want %q", got, "disabled")
	}
	if got := cfg.Research.Autoresearch.Env["CUDA_VISIBLE_DEVICES"]; got != "0" {
		t.Fatalf("Research.Autoresearch.Env[CUDA_VISIBLE_DEVICES] = %q, want %q", got, "0")
	}
	if cfg.Research.Autoresearch.Timeout != 30*time.Minute {
		t.Fatalf("Research.Autoresearch.Timeout = %v, want %v", cfg.Research.Autoresearch.Timeout, 30*time.Minute)
	}
}
