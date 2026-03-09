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
	host := os.Getenv("BLUE_SERVER_HOST")
	if host == "" {
		host = "localhost"
	}
	return fmt.Sprintf("http://%s:%d", host, resolveServerPort())
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
