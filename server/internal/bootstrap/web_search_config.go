package bootstrap

import (
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func buildWebSearchConfig(cfg *config.Config) tools.WebSearchConfig {
	if cfg == nil {
		return tools.WebSearchConfig{}
	}
	ws := cfg.ToolCalling.WebSearch
	return tools.WebSearchConfig{
		Provider:   strings.TrimSpace(ws.Provider),
		Providers:  ws.Providers,
		APIKey:     strings.TrimSpace(ws.APIKey),
		BaseURL:    strings.TrimSpace(ws.BaseURL),
		MaxResults: ws.MaxResults,
		Timeout:    ws.Timeout,
		SafeSearch: ws.SafeSearch,
		Region:     strings.TrimSpace(ws.Region),
	}
}
