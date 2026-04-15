package bootstrap

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// zimaOSDefaultWorkspaceDir is the default workspace directory when running on ZimaOS.
const zimaOSDefaultWorkspaceDir = "/media/ZimaOS-HD/AppData/zimaos-blue"

// zimaOSAppDataMount is the mount point for ZimaOS AppData.
const zimaOSAppDataMount = "/media/ZimaOS-HD"

// Injectable dependencies for testing
var (
	workspaceRuntimeGOOS   = runtime.GOOS
	workspaceReadOSRelease = func() ([]byte, error) {
		return os.ReadFile("/etc/os-release")
	}
	workspaceStatPath = os.Stat
)

// IsZimaOS returns true if running on ZimaOS (a Linux-based NAS OS).
// It checks for the presence of ZimaOS-specific paths or environment variables.
func IsZimaOS() bool {
	// Check environment variable first (set by ZimaOS runtime or module wrapper)
	if strings.TrimSpace(os.Getenv("ZIMAOS_BLUE_MODE")) == "1" {
		return true
	}
	return isZimaOSWorkspaceHost()
}

// isZimaOSWorkspaceHost checks if the host appears to be a ZimaOS system.
// Exported as a variable for testing injection.
func isZimaOSWorkspaceHost() bool {
	// Must be on Linux
	if workspaceRuntimeGOOS != "linux" {
		return false
	}
	// Check for ZimaOS-specific mount point
	if info, err := workspaceStatPath(zimaOSAppDataMount); err == nil && info.IsDir() {
		return true
	}
	// Check for /etc/os-release containing ZimaOS
	if content, err := workspaceReadOSRelease(); err == nil {
		if strings.Contains(string(content), "ZimaOS") {
			return true
		}
	}
	return false
}

// ResolveZimaOSDataDir returns the ZimaOS-specific data directory.
// On ZimaOS: /media/ZimaOS-HD/AppData/zimaos-blue
// Otherwise falls back to the standard dataDir.
func ResolveZimaOSDataDir(dataDir string) string {
	if IsZimaOS() {
		return zimaOSDefaultWorkspaceDir
	}
	return dataDir
}

// ResolveZimaOSWorkspaceDir returns the workspace directory for ZimaOS.
// On ZimaOS, this is the AppData directory itself (not a subdirectory).
func ResolveZimaOSWorkspaceDir(dataDir string) string {
	if IsZimaOS() {
		return zimaOSDefaultWorkspaceDir
	}
	// For non-ZimaOS, use the traditional dataDir/workspace
	return filepath.Join(dataDir, "workspace")
}
