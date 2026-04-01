package bootstrap

func routeRuntimeServerPort(cfg *ServerConfig) int {
	if cfg != nil && cfg.Port != 0 {
		return cfg.Port
	}
	return DefaultConfig().Port
}

func routeRuntimeServerVersion(cfg *ServerConfig) string {
	if cfg != nil && cfg.Version != "" {
		return cfg.Version
	}
	return DefaultConfig().Version
}

func routeRuntimeServerMode(cfg *ServerConfig) string {
	if cfg != nil && cfg.Mode != "" {
		return cfg.Mode
	}
	return DefaultConfig().Mode
}

func routeRuntimeServerBuildTime(cfg *ServerConfig) string {
	if cfg != nil && cfg.BuildTime != "" {
		return cfg.BuildTime
	}
	return DefaultConfig().BuildTime
}

func routeRuntimeServerGitCommit(cfg *ServerConfig) string {
	if cfg != nil && cfg.GitCommit != "" {
		return cfg.GitCommit
	}
	return DefaultConfig().GitCommit
}

func routeRuntimeServerDataDir(cfg *ServerConfig) string {
	if cfg != nil && cfg.DataDir != "" {
		return cfg.DataDir
	}
	return DefaultConfig().DataDir
}
