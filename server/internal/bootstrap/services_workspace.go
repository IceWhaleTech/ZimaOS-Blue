package bootstrap

import (
	"path/filepath"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

// ResolveWorkspaceDir returns the shared runtime workspace directory.
// Explicit non-default workspace settings win; otherwise we keep the
// historical data-dir workspace to avoid surprising runtime behavior.
// On ZimaOS, defaults to /media/ZimaOS-HD/AppData/zimaos-blue
func ResolveWorkspaceDir(dataDir string, appCfg *config.Config) string {
	candidates := make([]string, 0, 2)
	if appCfg != nil {
		if v := normalizeExplicitWorkspaceDir(appCfg.AgentCore.WorkspaceDir); v != "" {
			candidates = append(candidates, v)
		}
	}
	// Check ZimaOS first - if detected, use the ZimaOS default
	if IsZimaOS() {
		candidates = append(candidates, zimaOSDefaultWorkspaceDir)
	} else if strings.TrimSpace(dataDir) != "" {
		candidates = append(candidates, filepath.Join(dataDir, "workspace"))
	}
	for _, candidate := range candidates {
		if normalized := normalizeWorkspacePath(candidate); normalized != "" {
			return normalized
		}
	}
	return "."
}

func normalizeExplicitWorkspaceDir(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if filepath.Clean(trimmed) == "." {
		return ""
	}
	return normalizeWorkspacePath(trimmed)
}

func normalizeWorkspacePath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if !filepath.IsAbs(trimmed) {
		if abs, err := filepath.Abs(trimmed); err == nil {
			trimmed = abs
		}
	}
	return filepath.Clean(trimmed)
}

func ResolveBuiltinToolAllowedPaths(appCfg *config.Config, dataDir string) []string {
	return resolveBuiltinToolAllowedPaths(appCfg, dataDir)
}

func ResolveWorkspaceSkillsDir(dataDir string, appCfg *config.Config) string {
	return filepath.Join(ResolveWorkspaceDir(dataDir, appCfg), ".agents", "skills")
}
