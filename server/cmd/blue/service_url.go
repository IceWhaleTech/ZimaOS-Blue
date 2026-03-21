package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

func getServiceBaseURL() string {
	return fmt.Sprintf("http://%s:%d", resolveServerHost(), resolveServerPort())
}

func getServiceAPIBaseURL(path string) string {
	if path == "" {
		return getServiceBaseURL()
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return getServiceBaseURL() + path
}

func resolveServerPort() int {
	if port := configuredGatewayPort(); port > 0 {
		return port
	}

	if envPort := os.Getenv("BLUE_SERVER_PORT"); envPort != "" {
		if parsed, err := strconv.Atoi(envPort); err == nil && parsed > 0 {
			return parsed
		}
	}

	if cfg, err := loadCLIConfig(); err == nil && cfg != nil && cfg.Server.Port > 0 {
		return cfg.Server.Port
	}

	if devMode {
		return 8081
	}
	return 8080
}

func resolveServerHost() string {
	if host := normalizeServiceHost(configuredGatewayBind()); host != "" {
		return host
	}

	if host := normalizeServiceHost(os.Getenv("BLUE_SERVER_HOST")); host != "" {
		return host
	}

	if cfg, err := loadCLIConfig(); err == nil && cfg != nil {
		if host := normalizeServiceHost(cfg.Server.Host); host != "" {
			return host
		}
	}

	return "localhost"
}

func applyServerRuntimeOverrides(cfg *config.ServerConfig) {
	if cfg == nil {
		return
	}

	if host := configuredGatewayBind(); host != "" {
		cfg.Host = host
	} else if envHost := strings.TrimSpace(os.Getenv("BLUE_SERVER_HOST")); envHost != "" {
		cfg.Host = envHost
	}

	if port := configuredGatewayPort(); port > 0 {
		cfg.Port = port
	} else if envPort := strings.TrimSpace(os.Getenv("BLUE_SERVER_PORT")); envPort != "" {
		if parsed, err := strconv.Atoi(envPort); err == nil && parsed > 0 {
			cfg.Port = parsed
		}
	}
}

func normalizeServiceHost(raw string) string {
	host := strings.TrimSpace(raw)
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		return ""
	default:
		return host
	}
}

func loadCLIConfig() (*config.Config, error) {
	if cfgFile != "" {
		if _, err := os.Stat(cfgFile); err == nil {
			return config.Load(cfgFile)
		}
	}

	if devMode || profile != "" {
		path := filepath.Join(getConfigDir(), "config.yaml")
		if _, err := os.Stat(path); err == nil {
			return config.Load(path)
		}
	}

	return config.Load("")
}
