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
		Timeout:           wf.Timeout,
		AllowPrivateHosts: wf.AllowPrivateHosts,
	}
}
