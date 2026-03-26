package bootstrap

import (
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

func TestInferSecurityScannerEnvironment(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		cfg     *config.Config
		wantEnv string
	}{
		{
			name: "prefers explicit production environment variable",
			env: map[string]string{
				"NODE_ENV": "production",
			},
			cfg:     &config.Config{},
			wantEnv: "production",
		},
		{
			name: "maps staging aliases",
			env: map[string]string{
				"APP_ENV": "qa",
			},
			cfg:     &config.Config{},
			wantEnv: "staging",
		},
		{
			name: "uses blue dev flag",
			env: map[string]string{
				"BLUE_DEV": "1",
			},
			cfg: &config.Config{
				Server: config.ServerConfig{Host: "0.0.0.0"},
			},
			wantEnv: "development",
		},
		{
			name: "treats loopback host as development",
			cfg: &config.Config{
				Server: config.ServerConfig{Host: "127.0.0.1"},
			},
			wantEnv: "development",
		},
		{
			name: "defaults to production for non-loopback deployments",
			cfg: &config.Config{
				Server: config.ServerConfig{Host: "0.0.0.0"},
				Log:    config.LogConfig{Level: "debug"},
			},
			wantEnv: "production",
		},
		{
			name: "uses debug level as development fallback on custom local hosts",
			cfg: &config.Config{
				Server: config.ServerConfig{Host: "devbox.internal"},
				Log:    config.LogConfig{Level: "info"},
			},
			wantEnv: "production",
		},
		{
			name: "uses debug level as development fallback when no stronger signal exists",
			cfg: &config.Config{
				Server: config.ServerConfig{Host: "dev-machine"},
				Log:    config.LogConfig{Level: "debug"},
			},
			wantEnv: "development",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range securityScannerEnvironmentVars {
				t.Setenv(key, "")
			}
			t.Setenv("BLUE_DEV", "")
			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			if got := inferSecurityScannerEnvironment(tt.cfg); got != tt.wantEnv {
				t.Fatalf("inferSecurityScannerEnvironment() = %q, want %q", got, tt.wantEnv)
			}
		})
	}
}

func TestBuildSecurityScannerConfig(t *testing.T) {
	t.Setenv("BLUE_ENV", "")
	t.Setenv("BLUE_DEV", "")

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "0.0.0.0",
			TLS: config.TLSConfig{
				Enabled:  true,
				CertFile: "/tmp/fullchain.pem",
				KeyFile:  "/tmp/privkey.pem",
			},
		},
		Log: config.LogConfig{
			Level: "debug",
		},
		Security: config.SecurityConfig{
			JWT: config.JWTConfig{
				Secret:     "0123456789abcdef0123456789abcdef",
				Expiration: 2 * time.Hour,
			},
			Sandbox: config.SandboxConfig{
				Enabled:        true,
				DefaultTimeout: 45 * time.Second,
				MemoryLimit:    "1.5GiB",
				CPULimit:       1.5,
				NetworkEnabled: true,
			},
		},
	}

	scannerCfg := buildSecurityScannerConfig(cfg, true)

	if scannerCfg.Environment != "production" {
		t.Fatalf("Environment = %q, want %q", scannerCfg.Environment, "production")
	}
	if scannerCfg.SandboxMemoryLimitMB != 1536 {
		t.Fatalf("SandboxMemoryLimitMB = %d, want %d", scannerCfg.SandboxMemoryLimitMB, 1536)
	}
	if scannerCfg.SandboxTimeoutSeconds != 45 {
		t.Fatalf("SandboxTimeoutSeconds = %d, want %d", scannerCfg.SandboxTimeoutSeconds, 45)
	}
	if !scannerCfg.TLSEnabled {
		t.Fatalf("TLSEnabled = %v, want true", scannerCfg.TLSEnabled)
	}
	if scannerCfg.TLSCertPath != "/tmp/fullchain.pem" {
		t.Fatalf("TLSCertPath = %q, want %q", scannerCfg.TLSCertPath, "/tmp/fullchain.pem")
	}
	if scannerCfg.JWTExpirySecs != 7200 {
		t.Fatalf("JWTExpirySecs = %d, want %d", scannerCfg.JWTExpirySecs, 7200)
	}
	if !scannerCfg.DebugMode {
		t.Fatalf("DebugMode = %v, want true", scannerCfg.DebugMode)
	}
}

func TestBuildSecurityScannerConfig_DisablesSandboxWhenRuntimeUnavailable(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "0.0.0.0",
		},
		Security: config.SecurityConfig{
			Sandbox: config.SandboxConfig{
				Enabled:        true,
				DefaultTimeout: 30 * time.Second,
				MemoryLimit:    "256MB",
			},
		},
	}

	scannerCfg := buildSecurityScannerConfig(cfg, false)
	if scannerCfg.SandboxEnabled {
		t.Fatal("SandboxEnabled = true, want false when runtime sandbox is unavailable")
	}
}
