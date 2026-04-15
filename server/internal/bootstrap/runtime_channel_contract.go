package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

func (binding *runtimeContractBinding) BindChannelRuntime(options routeRuntimeContractChannelOptions) {
	if binding == nil {
		return
	}
	bindRouteRuntimeChannels(options)
}

func bindRouteRuntimeChannels(options routeRuntimeContractChannelOptions) {
	pageMiddleware := routeRuntimePageMiddleware(options.requirePagePermission, permission.PageChannels)
	if options.channelConfigStore == nil {
		registerChannelConfigRoutes(options.api, nil, options.authMiddleware, pageMiddleware, nil, nil)
		registerRouteRuntimeWechatILinkSetupRoutes(options, pageMiddleware, nil, nil)
		return
	}

	channelManager := newRouteRuntimeChannelManager(options)
	channelFactory := serverpkg.NewChannelFactory(options.logger)
	bindRouteRuntimeChannelChatTarget(options.chat, channelManager)
	if options.mgmtTool != nil {
		options.mgmtTool.SetChannels(&mgmtChannelAdapter{mgr: channelManager})
	}
	bindRouteRuntimeChannelWatcherTarget(options.channelTaskWatcher, channelManager, newRouteRuntimeChannelURLResolver(options.ngrokTunnelMgr, options.serverPort))
	bindRouteRuntimeChannelHandler(channelManager, options.chat, options.autoreplyService)
	startRouteRuntimeEnabledChannelsAsync(
		options.channelConfigStore.GetEnabled(),
		options.channelConfigStore,
		channelFactory,
		channelManager,
		options.logger,
	)

	channelConfigHandler := serverpkg.NewChannelConfigHandler(options.channelConfigStore)
	registerChannelConfigRoutes(options.api, channelConfigHandler, options.authMiddleware, pageMiddleware, channelManager, channelFactory)
	registerRouteRuntimeWechatILinkSetupRoutes(options, pageMiddleware, channelManager, channelFactory)
}
