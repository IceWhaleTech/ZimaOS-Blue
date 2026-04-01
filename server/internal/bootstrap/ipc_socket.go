package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
)

func resolveIPCSocketPath(dataDir string) string {
	if sockPath := strings.TrimSpace(os.Getenv("BLUE_IPC_SOCKET")); sockPath != "" {
		return sockPath
	}
	if dataDir = strings.TrimSpace(dataDir); dataDir != "" {
		return filepath.Join(dataDir, "blue.sock")
	}
	return "/tmp/blue.sock"
}
