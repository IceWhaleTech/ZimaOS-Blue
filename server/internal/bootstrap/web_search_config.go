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
	providerSettings := make(map[string]tools.WebSearchProviderSetting, len(ws.ProviderSettings))
	for provider, setting := range ws.ProviderSettings {
		providerSettings[provider] = tools.WebSearchProviderSetting{
			APIKey:  strings.TrimSpace(setting.APIKey),
			BaseURL: strings.TrimSpace(setting.BaseURL),
			Enabled: setting.Enabled,
		}
	}
	return tools.WebSearchConfig{
		Provider:        strings.TrimSpace(ws.Provider),
		Providers:       append([]string(nil), ws.Providers...),
		APIKey:          strings.TrimSpace(ws.APIKey),
		BaseURL:         strings.TrimSpace(ws.BaseURL),
		MaxResults:      ws.MaxResults,
		Timeout:         ws.Timeout,
		SafeSearch:      ws.SafeSearch,
		Region:          strings.TrimSpace(ws.Region),
		CacheTTL:        ws.CacheTTL,
		CacheMaxEntries: ws.CacheMaxEntries,
		BrowserFallback: tools.WebSearchBrowserFallbackConfig{
			Enabled:           ws.BrowserFallback.Enabled,
			Engine:            strings.TrimSpace(ws.BrowserFallback.Engine),
			TriggerMode:       strings.TrimSpace(ws.BrowserFallback.TriggerMode),
			QualityThreshold:  ws.BrowserFallback.QualityThreshold,
			MaxBrowserRetries: ws.BrowserFallback.MaxBrowserRetries,
		},
		ProviderSettings: providerSettings,
	}
}
