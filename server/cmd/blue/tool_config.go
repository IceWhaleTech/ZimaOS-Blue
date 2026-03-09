package main

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func buildBuiltinToolConfigs(cfg *config.Config) (tools.WebSearchConfig, tools.WebFetchConfig) {
	if cfg == nil {
		return tools.WebSearchConfig{}, tools.WebFetchConfig{}
	}
	return tools.WebSearchConfig{
			Provider:   cfg.ToolCalling.WebSearch.Provider,
			Providers:  cfg.ToolCalling.WebSearch.Providers,
			APIKey:     cfg.ToolCalling.WebSearch.APIKey,
			BaseURL:    cfg.ToolCalling.WebSearch.BaseURL,
			MaxResults: cfg.ToolCalling.WebSearch.MaxResults,
			Timeout:    cfg.ToolCalling.WebSearch.Timeout,
			SafeSearch: cfg.ToolCalling.WebSearch.SafeSearch,
			Region:     cfg.ToolCalling.WebSearch.Region,
		}, tools.WebFetchConfig{
			AllowPrivateHosts: cfg.ToolCalling.WebFetch.AllowPrivateHosts,
			Timeout:           cfg.ToolCalling.WebFetch.Timeout,
		}
}
