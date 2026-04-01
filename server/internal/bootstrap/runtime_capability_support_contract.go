package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
)

func (binding *runtimeContractBinding) BindCapabilitySupportRuntime(
	options routeRuntimeContractCapabilitySupportOptions,
) routeRuntimeContractCapabilitySupportResult {
	if binding == nil {
		return routeRuntimeContractCapabilitySupportResult{}
	}
	return bindRouteRuntimeCapabilitySupport(options)
}

func bindRouteRuntimeCapabilitySupport(
	options routeRuntimeContractCapabilitySupportOptions,
) routeRuntimeContractCapabilitySupportResult {
	chatPermission := routeRuntimePageMiddleware(options.requirePagePermission, permission.PageChat)
	settingsPermission := routeRuntimePageMiddleware(options.requirePagePermission, permission.PageSettings)
	toolsPermission := routeRuntimePageMiddleware(options.requirePagePermission, permission.PageTools)
	automationPermission := routeRuntimePageMiddleware(options.requirePagePermission, permission.PageAutomation)

	registerRuntimeExecSupportRoutes(options.v1, options.authMiddleware, chatPermission, options.execSupport)
	registerRuntimeAskSupportRoutes(options.v1, options.authMiddleware, chatPermission, options.askSupport)

	if options.deps != nil {
		registerBackupRoutes(options.v1, options.deps.BackupHandler, options.authRouteMiddleware, settingsPermission)
		registerSecurityRoutes(
			options.protected,
			options.requirePagePermission,
			options.deps.SecurityHandler,
			options.deps.Config,
			options.deps.ConfigKV,
			options.dataDir,
			options.deps.SandboxManager != nil && options.deps.SandboxManager.IsSupported(),
		)
		registerSandboxRoutes(options.protected, options.requirePagePermission, options.deps)
		registerCronRoutes(options.apiProtected, options.deps.CronHandler, automationPermission)
		registerBrowserAutomationRoutes(options.apiProtected, options.deps.BrowserHandler, toolsPermission)
		registerWorkflowRoutes(
			options.e,
			options.v1,
			options.deps.WorkflowHandler,
			options.authRouteMiddleware,
			automationPermission,
			options.flagEvaluator,
		)
		registerVoiceRoutes(
			options.v1,
			options.protected,
			options.deps.VoiceHandler,
			options.deps.VoiceWSHandler,
			options.authRouteMiddleware,
			chatPermission,
		)
		registerSpeechRoutes(options.v1, options.deps.SpeechHandler, options.authRouteMiddleware, settingsPermission)
	}

	result := routeRuntimeContractCapabilitySupportResult{
		convertRegistered: options.execSupport.registerConvertRoutes(options.protected, chatPermission),
	}
	if result.convertRegistered {
		if options.logger != nil {
			options.logger.Info("Convert routes registered")
		}
		if options.trace != nil {
			options.trace.Mark("convert_routes_registered")
		}
	}

	return result
}
