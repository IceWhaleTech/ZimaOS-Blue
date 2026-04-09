package main

import (
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/bootstrap"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func buildBuiltinToolConfigs(cfg *config.Config) (tools.WebSearchConfig, tools.WebFetchConfig) {
	if cfg == nil {
		return tools.WebSearchConfig{}, tools.WebFetchConfig{}
	}
	return tools.WebSearchConfig{
			Provider:        cfg.ToolCalling.WebSearch.Provider,
			Providers:       cfg.ToolCalling.WebSearch.Providers,
			APIKey:          cfg.ToolCalling.WebSearch.APIKey,
			BaseURL:         cfg.ToolCalling.WebSearch.BaseURL,
			MaxResults:      cfg.ToolCalling.WebSearch.MaxResults,
			Timeout:         cfg.ToolCalling.WebSearch.Timeout,
			SafeSearch:      cfg.ToolCalling.WebSearch.SafeSearch,
			Region:          cfg.ToolCalling.WebSearch.Region,
			CacheTTL:        cfg.ToolCalling.WebSearch.CacheTTL,
			CacheMaxEntries: cfg.ToolCalling.WebSearch.CacheMaxEntries,
		}, tools.WebFetchConfig{
			AllowPrivateHosts:     cfg.ToolCalling.WebFetch.AllowPrivateHosts,
			Timeout:               cfg.ToolCalling.WebFetch.Timeout,
			FirecrawlTimeout:      cfg.ToolCalling.WebFetch.FirecrawlTimeout,
			JinaReaderEnabled:     cfg.ToolCalling.WebFetch.JinaReaderEnabled,
			JinaReaderTimeout:     cfg.ToolCalling.WebFetch.JinaReaderTimeout,
			ProxyFetcherProviders: append([]string(nil), cfg.ToolCalling.WebFetch.ProxyFetcherProviders...),
		}
}

func registerServerToolRegistry(registry *tools.Registry, cfg *config.Config, dataDir string) {
	if registry == nil {
		return
	}

	webSearchConfig, webFetchConfig := buildBuiltinToolConfigs(cfg)
	workspaceAllowedPaths := bootstrap.ResolveBuiltinToolAllowedPaths(cfg, dataDir)
	runtimeCfg := tools.BuiltinRuntimeConfig{
		DataDir: strings.TrimSpace(dataDir),
	}
	if cfg != nil {
		runtimeCfg.Ripgrep = cfg.ToolCalling.Ripgrep
	}
	tools.RegisterBuiltinToolsWithRuntimeConfig(registry, webSearchConfig, webFetchConfig, workspaceAllowedPaths, 0, runtimeCfg)

	execConfig := bootstrap.NewRuntimeExecConfig(dataDir, workspaceAllowedPaths)
	tools.RegisterExecTools(registry, execConfig, nil, nil, nil)

	tools.RegisterFactoryToolDefinitions(registry)
}
