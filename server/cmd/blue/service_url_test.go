package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveServerPortFromConfigFile(t *testing.T) {
	t.Setenv("BLUE_SERVER_PORT", "")
	oldCfg, oldDev, oldProfile := cfgFile, devMode, profile
	defer func() {
		cfgFile, devMode, profile = oldCfg, oldDev, oldProfile
	}()

	cfgDir := t.TempDir()
	cfgPath := filepath.Join(cfgDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("server:\n  port: 19091\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfgFile = cfgPath
	devMode = false
	profile = ""

	if got := resolveServerPort(); got != 19091 {
		t.Fatalf("resolveServerPort()=%d, want 19091", got)
	}
}

func TestGetServiceBaseURLPrefersEnvHostAndPort(t *testing.T) {
	t.Setenv("BLUE_SERVER_HOST", "127.0.0.1")
	t.Setenv("BLUE_SERVER_PORT", "19092")

	if got := getServiceBaseURL(); got != "http://127.0.0.1:19092" {
		t.Fatalf("getServiceBaseURL()=%q, want %q", got, "http://127.0.0.1:19092")
	}
	if got := getServiceAPIBaseURL("api/v1/skills"); got != "http://127.0.0.1:19092/api/v1/skills" {
		t.Fatalf("getServiceAPIBaseURL()=%q", got)
	}
}
