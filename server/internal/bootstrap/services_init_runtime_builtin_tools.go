package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func registerBuiltinRuntimeTools(registry *tools.Registry, cfg *ServerConfig, appCfg *config.Config) {
	tools.RegisterBuiltinToolsWithRuntimeConfig(
		registry,
		buildWebSearchConfig(appCfg),
		buildWebFetchConfig(appCfg),
		resolveBuiltinToolAllowedPaths(appCfg, cfg.DataDir),
		0,
		tools.BuiltinRuntimeConfig{
			DataDir:      cfg.DataDir,
			WorkspaceDir: ResolveWorkspaceDir(cfg.DataDir, appCfg),
			Ripgrep:      appCfg.ToolCalling.Ripgrep,
			SkillDynamicExposureEnabled: func() bool {
				return true
			},
		},
	)
}
