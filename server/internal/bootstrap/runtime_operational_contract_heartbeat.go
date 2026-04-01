package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/heartbeat"

func cloneRouteRuntimeHeartbeatConfig(cfg *heartbeat.Config) *heartbeat.Config {
	if cfg == nil {
		return &heartbeat.Config{}
	}
	copy := *cfg
	copy.Visibility = cfg.Visibility
	return &copy
}
