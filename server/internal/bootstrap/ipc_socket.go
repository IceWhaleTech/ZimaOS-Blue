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

func ResolveAuditIPCSocketPath(dataDir string) string {
	if sockPath := strings.TrimSpace(os.Getenv("BLUE_AUDIT_IPC_SOCKET")); sockPath != "" {
		return sockPath
	}
	if dataDir = strings.TrimSpace(dataDir); dataDir != "" {
		return filepath.Join(dataDir, "session_audit.sock")
	}
	return "/tmp/session_audit.sock"
}
