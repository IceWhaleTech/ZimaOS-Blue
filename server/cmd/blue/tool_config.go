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
	providerSettings := make(map[string]tools.WebSearchProviderSetting, len(cfg.ToolCalling.WebSearch.ProviderSettings))
	for provider, setting := range cfg.ToolCalling.WebSearch.ProviderSettings {
		providerSettings[provider] = tools.WebSearchProviderSetting{
			APIKey:  strings.TrimSpace(setting.APIKey),
			BaseURL: strings.TrimSpace(setting.BaseURL),
			Enabled: setting.Enabled,
		}
	}
	return tools.WebSearchConfig{
			Provider:         cfg.ToolCalling.WebSearch.Provider,
			Providers:        cfg.ToolCalling.WebSearch.Providers,
			APIKey:           cfg.ToolCalling.WebSearch.APIKey,
			BaseURL:          cfg.ToolCalling.WebSearch.BaseURL,
			MaxResults:       cfg.ToolCalling.WebSearch.MaxResults,
			Timeout:          cfg.ToolCalling.WebSearch.Timeout,
			SafeSearch:       cfg.ToolCalling.WebSearch.SafeSearch,
			Region:           cfg.ToolCalling.WebSearch.Region,
			CacheTTL:         cfg.ToolCalling.WebSearch.CacheTTL,
			CacheMaxEntries:  cfg.ToolCalling.WebSearch.CacheMaxEntries,
			ProviderSettings: providerSettings,
			BrowserFallback: tools.WebSearchBrowserFallbackConfig{
				Enabled:           cfg.ToolCalling.WebSearch.BrowserFallback.Enabled,
				Engine:            cfg.ToolCalling.WebSearch.BrowserFallback.Engine,
				TriggerMode:       cfg.ToolCalling.WebSearch.BrowserFallback.TriggerMode,
				QualityThreshold:  cfg.ToolCalling.WebSearch.BrowserFallback.QualityThreshold,
				MaxBrowserRetries: cfg.ToolCalling.WebSearch.BrowserFallback.MaxBrowserRetries,
			},
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
	execConfig.ServiceHost = "127.0.0.1"
	if cfg != nil && cfg.Server.Port > 0 {
		execConfig.ServicePort = cfg.Server.Port
	}
	tools.RegisterExecTools(registry, execConfig, nil, nil, nil)

	tools.RegisterFactoryToolDefinitions(registry)
}
