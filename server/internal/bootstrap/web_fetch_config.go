package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func buildWebFetchConfig(cfg *config.Config) tools.WebFetchConfig {
	if cfg == nil {
		return tools.WebFetchConfig{}
	}
	wf := cfg.ToolCalling.WebFetch
	return tools.WebFetchConfig{
		Timeout:               wf.Timeout,
		LayeredFetchEnabled:   wf.LayeredFetchEnabled,
		SessionMemoryEnabled:  wf.SessionMemoryEnabled,
		DomainStrategyEnabled: wf.DomainStrategyEnabled,
		AdapterMemoryEnabled:  wf.AdapterMemoryEnabled,
		NetworkObserveEnabled: cfg.Browser.NetworkObserveEnabled,
		MaxExploreAttempts:    wf.MaxExploreAttempts,
		AutoFallbackHosts:     append([]string(nil), wf.AutoFallbackHosts...),
		ChallengePolicy:       wf.ChallengePolicy,
		HTTPNativeEnabled:     wf.HTTPNativeEnabled,
		HTTPNativeLibrary:     wf.HTTPNativeLibrary,
		HTTPNativePreferHosts: append([]string(nil), wf.HTTPNativePreferHosts...),
		FirecrawlTimeout:      wf.FirecrawlTimeout,
		JinaReaderEnabled:     wf.JinaReaderEnabled,
		JinaReaderTimeout:     wf.JinaReaderTimeout,
		ProxyFetcherProviders: append([]string(nil), wf.ProxyFetcherProviders...),
		AllowPrivateHosts:     wf.AllowPrivateHosts,
	}
}
