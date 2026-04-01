package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"

func (binding *runtimeContractBinding) NewAskSupportBundle(options routeRuntimeContractAskSupportOptions) runtimeAskSupportBundle {
	if binding == nil {
		return runtimeAskSupportBundle{}
	}
	return newRuntimeAskSupportBundle(runtimeAskSupportOptions{
		writeDB:        options.writeDB,
		readDB:         options.readDB,
		appConfig:      options.appConfig,
		toolRegistry:   options.toolRegistry,
		skillRegistry:  skillRegistryFromRuntimeSource(options.skillRegistry),
		broker:         options.broker,
		harnessRuntime: binding.HarnessRuntime(),
		chatTarget:     options.chatTarget,
		mediaDir:       options.mediaDir,
		timeout:        options.timeout,
	})
}

func (binding *runtimeContractBinding) NewExecSupportBundle(options routeRuntimeContractExecSupportOptions) runtimeExecSupportBundle {
	if binding == nil {
		return runtimeExecSupportBundle{}
	}
	return newRuntimeExecSupportBundle(runtimeExecSupportOptions{
		writeDB:              options.writeDB,
		readDB:               options.readDB,
		dataDir:              options.dataDir,
		workspaceDir:         options.workspaceDir,
		ripgrep:              options.ripgrep,
		workspaceAllowedPath: options.workspaceAllowedPath,
		memoryStore:          options.memoryStore,
		toolRegistry:         options.toolRegistry,
		skillRegistry:        options.skillRegistry,
		selectorSource:       options.selectorSource,
		broker:               options.broker,
		harnessRuntime:       binding.HarnessRuntime(),
		sandboxManager:       options.sandboxManager,
		chatHandler:          options.chatHandler,
		logger:               options.logger,
		closers:              options.closers,
		profileRoutes:        options.profileRoutes,
		sessionRoutes:        options.sessionRoutes,
		oauthSource:          options.oauthSource,
		lookupAPIKey:         options.lookupAPIKey,
	})
}

func (binding *runtimeContractBinding) RegisterMgmtTool(options routeRuntimeContractMgmtOptions) *tools.MgmtTool {
	if binding == nil {
		return nil
	}
	mgmtTool := tools.RegisterMgmtTool(options.registry)
	bindRuntimeMgmtTool(
		mgmtTool,
		options.providerPool,
		skillRegistryFromRuntimeSource(options.skillRegistry),
		options.workspaceDir,
		options.registry,
		options.version,
		options.userService,
		options.apiKeyService,
	)
	return mgmtTool
}

func (binding *runtimeContractBinding) BindMgmtUpgrade(target runtimeMgmtToolTarget, options routeRuntimeContractMgmtUpgradeOptions) {
	if binding == nil {
		return
	}
	bindRuntimeMgmtUpgrade(target, options.handler, options.otaChecker, options.version)
}
