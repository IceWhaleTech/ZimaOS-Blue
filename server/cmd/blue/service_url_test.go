package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

func TestResolveServerPortFromConfigFile(t *testing.T) {
	t.Setenv("BLUE_SERVER_PORT", "")
	oldCfg, oldDev, oldProfile := cfgFile, devMode, profile
	oldGatewayPort, oldGatewayBind := gatewayPort, gatewayBind
	defer func() {
		cfgFile, devMode, profile = oldCfg, oldDev, oldProfile
		gatewayPort, gatewayBind = oldGatewayPort, oldGatewayBind
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

func TestResolveServerPortPrefersGatewayFlag(t *testing.T) {
	t.Setenv("BLUE_SERVER_PORT", "19092")
	oldGatewayPort := gatewayPort
	defer func() {
		gatewayPort = oldGatewayPort
	}()

	gatewayPort = 19093

	if got := resolveServerPort(); got != 19093 {
		t.Fatalf("resolveServerPort()=%d, want 19093", got)
	}
}

func TestGetServiceBaseURLPrefersEnvHostAndPort(t *testing.T) {
	oldGatewayPort, oldGatewayBind := gatewayPort, gatewayBind
	defer func() {
		gatewayPort, gatewayBind = oldGatewayPort, oldGatewayBind
	}()

	t.Setenv("BLUE_SERVER_HOST", "127.0.0.1")
	t.Setenv("BLUE_SERVER_PORT", "19092")

	if got := getServiceBaseURL(); got != "http://127.0.0.1:19092" {
		t.Fatalf("getServiceBaseURL()=%q, want %q", got, "http://127.0.0.1:19092")
	}
	if got := getServiceAPIBaseURL("api/v1/skills"); got != "http://127.0.0.1:19092/api/v1/skills" {
		t.Fatalf("getServiceAPIBaseURL()=%q", got)
	}
}

func TestGetServiceBaseURLNormalizesWildcardBindToLocalhost(t *testing.T) {
	oldGatewayPort, oldGatewayBind := gatewayPort, gatewayBind
	defer func() {
		gatewayPort, gatewayBind = oldGatewayPort, oldGatewayBind
	}()

	t.Setenv("BLUE_SERVER_HOST", "")
	t.Setenv("BLUE_SERVER_PORT", "")
	gatewayBind = "0.0.0.0"
	gatewayPort = 18080

	if got := getServiceBaseURL(); got != "http://localhost:18080" {
		t.Fatalf("getServiceBaseURL()=%q, want %q", got, "http://localhost:18080")
	}
}

func TestApplyServerRuntimeOverridesPrefersGatewayFlags(t *testing.T) {
	oldGatewayPort, oldGatewayBind := gatewayPort, gatewayBind
	defer func() {
		gatewayPort, gatewayBind = oldGatewayPort, oldGatewayBind
	}()

	t.Setenv("BLUE_SERVER_HOST", "env.example")
	t.Setenv("BLUE_SERVER_PORT", "19100")

	cfg := config.ServerConfig{
		Host: "config.example",
		Port: 80,
	}
	gatewayBind = "127.0.0.1"
	gatewayPort = 19101

	applyServerRuntimeOverrides(&cfg)

	if cfg.Host != "127.0.0.1" {
		t.Fatalf("cfg.Host=%q, want %q", cfg.Host, "127.0.0.1")
	}
	if cfg.Port != 19101 {
		t.Fatalf("cfg.Port=%d, want %d", cfg.Port, 19101)
	}
}
