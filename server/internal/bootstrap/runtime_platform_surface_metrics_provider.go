package bootstrap

import serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"

func routeRuntimeChatCacheFootprintProvider(target metricsRecorderTarget) serverpkg.ChatCacheFootprintProvider {
	if provider, ok := any(target).(serverpkg.ChatCacheFootprintProvider); ok {
		return provider
	}
	return nil
}
