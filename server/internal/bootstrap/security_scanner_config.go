package bootstrap

import (
	"crypto/tls"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ratelimit"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/security"
)

var securityScannerEnvironmentVars = []string{
	"BLUE_ENV",
	"BLUE_ENVIRONMENT",
	"APP_ENV",
	"ENVIRONMENT",
	"NODE_ENV",
	"GO_ENV",
	"GIN_MODE",
}

func buildSecurityScannerConfig(cfg *config.Config) *security.ScannerConfig {
	scannerCfg := security.DefaultScannerConfig()
	if cfg == nil {
		return scannerCfg
	}

	scannerCfg.Environment = inferSecurityScannerEnvironment(cfg)
	scannerCfg.TLSEnabled = cfg.Server.TLS.Enabled
	scannerCfg.TLSCertPath = strings.TrimSpace(cfg.Server.TLS.CertFile)
	scannerCfg.TLSKeyPath = strings.TrimSpace(cfg.Server.TLS.KeyFile)
	scannerCfg.TLSMinVersion = tls.VersionTLS12
	scannerCfg.RateLimitEnabled = true
	scannerCfg.RateLimitRPS = ratelimit.DefaultConfig().Rate
	scannerCfg.SandboxEnabled = cfg.Security.Sandbox.Enabled
	scannerCfg.SandboxMemoryLimitMB = parseScannerMemoryLimitMB(cfg.Security.Sandbox.MemoryLimit)
	scannerCfg.SandboxCPULimitCores = cfg.Security.Sandbox.CPULimit
	scannerCfg.SandboxTimeoutSeconds = scannerDurationSeconds(cfg.Security.Sandbox.DefaultTimeout)
	scannerCfg.SandboxNetworkEnabled = cfg.Security.Sandbox.NetworkEnabled
	scannerCfg.DebugMode = isSecurityScannerDebugLogLevel(cfg.Log.Level)
	scannerCfg.JWTSecretLength = len(strings.TrimSpace(cfg.Security.JWT.Secret))
	scannerCfg.JWTExpirySecs = scannerDurationSeconds(cfg.Security.JWT.Expiration)

	return scannerCfg
}

func inferSecurityScannerEnvironment(cfg *config.Config) string {
	for _, key := range securityScannerEnvironmentVars {
		if env := normalizeSecurityScannerEnvironment(os.Getenv(key)); env != "" {
			return env
		}
	}

	if isSecurityScannerTruthyEnv("BLUE_DEV") {
		return "development"
	}

	if cfg != nil {
		if provider := strings.ToLower(strings.TrimSpace(cfg.Server.TLS.ACMEProvider)); strings.Contains(provider, "staging") {
			return "staging"
		}
		if isSecurityScannerLoopbackHost(cfg.Server.Host) {
			return "development"
		}
		if cfg.Server.TLS.Enabled && !cfg.Server.TLS.SelfSigned {
			return "production"
		}

		host := strings.TrimSpace(cfg.Server.Host)
		if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
			return "production"
		}
		if isSecurityScannerDebugLogLevel(cfg.Log.Level) {
			return "development"
		}
	}

	// Security checks should prefer stricter production assumptions when unclear.
	return "production"
}

func normalizeSecurityScannerEnvironment(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "prod", "production", "release", "live":
		return "production"
	case "stage", "staging", "preprod", "pre-prod", "qa", "uat":
		return "staging"
	case "dev", "development", "debug", "local", "test", "testing":
		return "development"
	default:
		return ""
	}
}

func isSecurityScannerTruthyEnv(key string) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return false
	}

	parsed, err := strconv.ParseBool(value)
	if err == nil {
		return parsed
	}

	switch strings.ToLower(value) {
	case "1", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func isSecurityScannerLoopbackHost(host string) bool {
	switch strings.TrimSpace(strings.ToLower(host)) {
	case "127.0.0.1", "localhost", "::1", "[::1]":
		return true
	default:
		return false
	}
}

func isSecurityScannerDebugLogLevel(level string) bool {
	return strings.EqualFold(strings.TrimSpace(level), "debug")
}

func scannerDurationSeconds(value time.Duration) int {
	if value <= 0 {
		return 0
	}
	return int(value / time.Second)
}

func parseScannerMemoryLimitMB(raw string) int {
	value := strings.TrimSpace(strings.ToUpper(raw))
	if value == "" {
		return 0
	}

	type unit struct {
		suffix string
		scale  float64
	}

	units := []unit{
		{suffix: "GIB", scale: 1024},
		{suffix: "GB", scale: 1000},
		{suffix: "GI", scale: 1024},
		{suffix: "G", scale: 1024},
		{suffix: "MIB", scale: 1},
		{suffix: "MB", scale: 1},
		{suffix: "MI", scale: 1},
		{suffix: "M", scale: 1},
		{suffix: "KIB", scale: 1.0 / 1024},
		{suffix: "KB", scale: 1.0 / 1000},
		{suffix: "KI", scale: 1.0 / 1024},
		{suffix: "K", scale: 1.0 / 1024},
		{suffix: "B", scale: 1.0 / (1024 * 1024)},
	}

	for _, current := range units {
		if !strings.HasSuffix(value, current.suffix) {
			continue
		}
		number := strings.TrimSpace(strings.TrimSuffix(value, current.suffix))
		if number == "" {
			return 0
		}
		parsed, err := strconv.ParseFloat(number, 64)
		if err != nil {
			return 0
		}
		return int(math.Round(parsed * current.scale))
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return int(math.Round(parsed))
}
