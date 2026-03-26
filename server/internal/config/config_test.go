package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
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
	if got := cfg.Browser.ResolvedStrategy(); got != browser.BrowserStrategySingle {
		t.Errorf("Browser.ResolvedStrategy() = %q, want %q", got, browser.BrowserStrategySingle)
	}
	if cfg.Browser.Lightpanda.Enabled {
		t.Errorf("Browser.Lightpanda.Enabled = %v, want false", cfg.Browser.Lightpanda.Enabled)
	}
	if !cfg.Browser.LightpandaAutoDownload() {
		t.Errorf("Browser.LightpandaAutoDownload() = %v, want true", cfg.Browser.LightpandaAutoDownload())
	}
	if got := cfg.Browser.LightpandaDownloadTimeout(); got != 20*time.Minute {
		t.Errorf("Browser.LightpandaDownloadTimeout() = %v, want %v", got, 20*time.Minute)
	}
	if !cfg.Browser.PreferLocalChrome() {
		t.Errorf("Browser.PreferLocalChrome() = %v, want true", cfg.Browser.PreferLocalChrome())
	}
	if !cfg.Browser.CapabilityEscalateOnFailure() {
		t.Errorf("Browser.CapabilityEscalateOnFailure() = %v, want true", cfg.Browser.CapabilityEscalateOnFailure())
	}
	if cfg.Browser.RelayPreferredFallback() != "managed" {
		t.Errorf("Browser.RelayPreferredFallback() = %q, want managed", cfg.Browser.RelayPreferredFallback())
	}
	if len(cfg.Browser.TrustedSitePresets) != 0 {
		t.Errorf("Browser.TrustedSitePresets = %v, want empty", cfg.Browser.TrustedSitePresets)
	}
	if cfg.ToolCalling.WebFetch.Timeout != 5*time.Minute {
		t.Errorf("ToolCalling.WebFetch.Timeout = %v, want %v", cfg.ToolCalling.WebFetch.Timeout, 5*time.Minute)
	}
	if !cfg.ToolCalling.WebFetch.HTTPNativeEnabled {
		t.Errorf("ToolCalling.WebFetch.HTTPNativeEnabled = %v, want true", cfg.ToolCalling.WebFetch.HTTPNativeEnabled)
	}
	if cfg.ToolCalling.WebFetch.HTTPNativeLibrary != "" {
		t.Errorf("ToolCalling.WebFetch.HTTPNativeLibrary = %q, want empty", cfg.ToolCalling.WebFetch.HTTPNativeLibrary)
	}
	if len(cfg.ToolCalling.WebFetch.HTTPNativePreferHosts) != 0 {
		t.Errorf("ToolCalling.WebFetch.HTTPNativePreferHosts = %v, want empty", cfg.ToolCalling.WebFetch.HTTPNativePreferHosts)
	}
	if cfg.ToolCalling.WebFetch.FirecrawlTimeout != 5*time.Minute {
		t.Errorf("ToolCalling.WebFetch.FirecrawlTimeout = %v, want %v", cfg.ToolCalling.WebFetch.FirecrawlTimeout, 5*time.Minute)
	}
	if cfg.ToolCalling.WebFetch.JinaReaderEnabled {
		t.Errorf("ToolCalling.WebFetch.JinaReaderEnabled = %v, want false", cfg.ToolCalling.WebFetch.JinaReaderEnabled)
	}
	if cfg.ToolCalling.WebFetch.JinaReaderTimeout != 5*time.Minute {
		t.Errorf("ToolCalling.WebFetch.JinaReaderTimeout = %v, want %v", cfg.ToolCalling.WebFetch.JinaReaderTimeout, 5*time.Minute)
	}
	if len(cfg.ToolCalling.WebFetch.ProxyFetcherProviders) != 2 {
		t.Errorf("ToolCalling.WebFetch.ProxyFetcherProviders = %v, want default pair", cfg.ToolCalling.WebFetch.ProxyFetcherProviders)
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
	if cfg.AgentCore.WorkspaceDir != "." {
		t.Errorf("AgentCore.WorkspaceDir = %q, want %q", cfg.AgentCore.WorkspaceDir, ".")
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
	configContent := strings.TrimSpace(`
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
  strategy: "hybrid_capability"
  driver: "relay"
  relay_enabled: true
  relay_port: 18792
  relay_token: "relay-test-token"
  headless: false
  pool_size: 1
  cdp_url: "http://127.0.0.1:18792?token=test-token"
  trusted_sites: ["https://www.zhihu.com"]
  trusted_site_presets: ["browser_common"]
  relay_preferred_sites: ["zhihu.com"]
  relay_preferred_site_presets: ["browser_common"]
  relay_preferred_fallback_driver: "managed"
  lightpanda:
    enabled: true
    binary_path: "/tmp/lightpanda"
    auto_download: false
    download_timeout: "7m"
    args: ["--headless"]
  chromium:
    prefer_local_chrome: false
  capability_router:
    escalate_on_failure: false
performance:
  database:
    conn_max_idle_time: "2m"
  resource_reclaim:
    stt_idle_after: "3m"
`)
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
	if got := cfg.Browser.ResolvedStrategy(); got != browser.BrowserStrategyHybridCapability {
		t.Errorf("Browser.ResolvedStrategy() = %v, want %v", got, browser.BrowserStrategyHybridCapability)
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
	if len(cfg.Browser.TrustedSites) != 1 || cfg.Browser.TrustedSites[0] != "https://www.zhihu.com" {
		t.Errorf("Browser.TrustedSites = %v, want zhihu seed", cfg.Browser.TrustedSites)
	}
	if len(cfg.Browser.TrustedSitePresets) != 1 || cfg.Browser.TrustedSitePresets[0] != "browser_common" {
		t.Errorf("Browser.TrustedSitePresets = %v, want browser_common", cfg.Browser.TrustedSitePresets)
	}
	if len(cfg.Browser.RelayPreferredSites) != 1 || cfg.Browser.RelayPreferredSites[0] != "zhihu.com" {
		t.Errorf("Browser.RelayPreferredSites = %v, want zhihu.com", cfg.Browser.RelayPreferredSites)
	}
	if len(cfg.Browser.RelayPreferredSitePresets) != 1 || cfg.Browser.RelayPreferredSitePresets[0] != "browser_common" {
		t.Errorf("Browser.RelayPreferredSitePresets = %v, want browser_common", cfg.Browser.RelayPreferredSitePresets)
	}
	if got := cfg.Browser.RelayPreferredFallback(); got != "managed" {
		t.Errorf("Browser.RelayPreferredFallback() = %q, want managed", got)
	}
	if !cfg.Browser.Lightpanda.Enabled {
		t.Fatalf("Browser.Lightpanda.Enabled = false, want true")
	}
	if cfg.Browser.Lightpanda.BinaryPath != "/tmp/lightpanda" {
		t.Fatalf("Browser.Lightpanda.BinaryPath = %q, want /tmp/lightpanda", cfg.Browser.Lightpanda.BinaryPath)
	}
	if cfg.Browser.LightpandaAutoDownload() {
		t.Fatalf("Browser.LightpandaAutoDownload() = true, want false")
	}
	if got := cfg.Browser.LightpandaDownloadTimeout(); got != 7*time.Minute {
		t.Fatalf("Browser.LightpandaDownloadTimeout() = %v, want %v", got, 7*time.Minute)
	}
	if len(cfg.Browser.Lightpanda.Args) != 1 || cfg.Browser.Lightpanda.Args[0] != "--headless" {
		t.Fatalf("Browser.Lightpanda.Args = %#v, want [--headless]", cfg.Browser.Lightpanda.Args)
	}
	if cfg.Browser.PreferLocalChrome() {
		t.Fatalf("Browser.PreferLocalChrome() = true, want false")
	}
	if cfg.Browser.CapabilityEscalateOnFailure() {
		t.Fatalf("Browser.CapabilityEscalateOnFailure() = true, want false")
	}
	if cfg.Performance.Database.ConnMaxIdleTime != 2*time.Minute {
		t.Errorf("Performance.Database.ConnMaxIdleTime = %v, want %v", cfg.Performance.Database.ConnMaxIdleTime, 2*time.Minute)
	}
	if cfg.Performance.ResourceReclaim.STTIdleAfter != 3*time.Minute {
		t.Errorf("Performance.ResourceReclaim.STTIdleAfter = %v, want %v", cfg.Performance.ResourceReclaim.STTIdleAfter, 3*time.Minute)
	}
}

func TestLoad_LegacyWorkspaceDirMigratesToAgentCore(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name          string
		configContent string
		want          string
	}{
		{
			name: "agentcore key wins",
			configContent: `
agentcore:
  workspace_dir: "/tmp/agentcore-workspace"
claude_code_cli:
  backend:
    workspace_dir: "/tmp/legacy-cli-workspace"
claudecode:
  workspace_dir: "/tmp/legacy-claudecode-workspace"
`,
			want: "/tmp/agentcore-workspace",
		},
		{
			name: "legacy claude_code_cli key migrates",
			configContent: `
claude_code_cli:
  backend:
    workspace_dir: "/tmp/legacy-cli-workspace"
`,
			want: "/tmp/legacy-cli-workspace",
		},
		{
			name: "legacy claudecode key migrates",
			configContent: `
claudecode:
  workspace_dir: "/tmp/legacy-claudecode-workspace"
`,
			want: "/tmp/legacy-claudecode-workspace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(tmpDir, strings.ReplaceAll(tt.name, " ", "_")+".yaml")
			if err := os.WriteFile(configPath, []byte(strings.TrimSpace(tt.configContent)), 0644); err != nil {
				t.Fatalf("write config file: %v", err)
			}

			cfg, err := Load(configPath)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.AgentCore.WorkspaceDir != tt.want {
				t.Fatalf("AgentCore.WorkspaceDir = %q, want %q", cfg.AgentCore.WorkspaceDir, tt.want)
			}
		})
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
	os.Setenv("BLUE_WEB_FETCH_HTTP_NATIVE_ENABLED", "true")
	os.Setenv("BLUE_WEB_FETCH_HTTP_NATIVE_LIBRARY", "/usr/lib/libcurl.4.dylib")
	os.Setenv("BLUE_WEB_FETCH_HTTP_NATIVE_PREFER_HOSTS", "thepaper.cn,36kr.com")
	os.Setenv("BLUE_WEB_FETCH_FIRECRAWL_TIMEOUT", "18s")
	os.Setenv("BLUE_WEB_FETCH_JINA_READER_ENABLED", "true")
	os.Setenv("BLUE_WEB_FETCH_JINA_READER_TIMEOUT", "21s")
	os.Setenv("BLUE_WEB_FETCH_PROXY_FETCHER_PROVIDERS", "jina_reader,firecrawl")
	defer func() {
		os.Unsetenv("BLUE_SERVER_HOST")
		os.Unsetenv("BLUE_SERVER_PORT")
		os.Unsetenv("BLUE_LOG_LEVEL")
		os.Unsetenv("BLUE_WEB_FETCH_ALLOW_PRIVATE_HOSTS")
		os.Unsetenv("BLUE_WEB_FETCH_TIMEOUT")
		os.Unsetenv("BLUE_WEB_FETCH_HTTP_NATIVE_ENABLED")
		os.Unsetenv("BLUE_WEB_FETCH_HTTP_NATIVE_LIBRARY")
		os.Unsetenv("BLUE_WEB_FETCH_HTTP_NATIVE_PREFER_HOSTS")
		os.Unsetenv("BLUE_WEB_FETCH_FIRECRAWL_TIMEOUT")
		os.Unsetenv("BLUE_WEB_FETCH_JINA_READER_ENABLED")
		os.Unsetenv("BLUE_WEB_FETCH_JINA_READER_TIMEOUT")
		os.Unsetenv("BLUE_WEB_FETCH_PROXY_FETCHER_PROVIDERS")
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
	if !cfg.ToolCalling.WebFetch.HTTPNativeEnabled {
		t.Fatalf("ToolCalling.WebFetch.HTTPNativeEnabled = %v, want true", cfg.ToolCalling.WebFetch.HTTPNativeEnabled)
	}
	if cfg.ToolCalling.WebFetch.HTTPNativeLibrary != "/usr/lib/libcurl.4.dylib" {
		t.Fatalf("ToolCalling.WebFetch.HTTPNativeLibrary = %q, want %q", cfg.ToolCalling.WebFetch.HTTPNativeLibrary, "/usr/lib/libcurl.4.dylib")
	}
	if got := strings.Join(cfg.ToolCalling.WebFetch.HTTPNativePreferHosts, ","); got != "thepaper.cn,36kr.com" {
		t.Fatalf("ToolCalling.WebFetch.HTTPNativePreferHosts = %q, want %q", got, "thepaper.cn,36kr.com")
	}
	if cfg.ToolCalling.WebFetch.FirecrawlTimeout != 18*time.Second {
		t.Fatalf("ToolCalling.WebFetch.FirecrawlTimeout = %v, want %v", cfg.ToolCalling.WebFetch.FirecrawlTimeout, 18*time.Second)
	}
	if !cfg.ToolCalling.WebFetch.JinaReaderEnabled {
		t.Fatalf("ToolCalling.WebFetch.JinaReaderEnabled = %v, want true", cfg.ToolCalling.WebFetch.JinaReaderEnabled)
	}
	if cfg.ToolCalling.WebFetch.JinaReaderTimeout != 21*time.Second {
		t.Fatalf("ToolCalling.WebFetch.JinaReaderTimeout = %v, want %v", cfg.ToolCalling.WebFetch.JinaReaderTimeout, 21*time.Second)
	}
	if got := strings.Join(cfg.ToolCalling.WebFetch.ProxyFetcherProviders, ","); got != "jina_reader,firecrawl" {
		t.Fatalf("ToolCalling.WebFetch.ProxyFetcherProviders = %q, want %q", got, "jina_reader,firecrawl")
	}
}
