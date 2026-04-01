package bootstrap

import "path/filepath"

func (binding *runtimeContractBinding) BindStartupAuthRuntime(
	options routeRuntimeContractStartupAuthOptions,
) routeRuntimeContractStartupAuthResult {
	if binding == nil {
		return routeRuntimeContractStartupAuthResult{}
	}
	return bindRouteRuntimeStartupAuth(binding, options)
}

func bindRouteRuntimeStartupAuth(binding routeRuntimeFastPathBinding, options routeRuntimeContractStartupAuthOptions) routeRuntimeContractStartupAuthResult {
	binding.BindStartupSurfaceRuntime(options.startup)
	result := routeRuntimeContractStartupAuthResult{
		startupBound: options.startup.v1 != nil,
		auth:         binding.BindAuthSurfaceRuntime(options.auth),
	}
	options.account.protected = result.auth.protected
	options.account.authPageV1Group = result.auth.authPageV1Group
	options.account.requirePagePermission = result.auth.requirePagePermission
	options.account.permissionHandler = result.auth.permissionHandler
	binding.BindAccountSurfaceRuntime(options.account)
	result.accountBound = options.account.v1 != nil
	return result
}

func (binding *runtimeContractBinding) BindBootstrapPhaseRuntime(
	options routeRuntimeContractBootstrapPhaseOptions,
) routeRuntimeContractBootstrapPhaseResult {
	if binding == nil {
		return routeRuntimeContractBootstrapPhaseResult{}
	}
	return bindRouteRuntimeBootstrapPhase(binding, options)
}

func bindRouteRuntimeBootstrapPhase(binding routeRuntimeFastPathBinding, options routeRuntimeContractBootstrapPhaseOptions) routeRuntimeContractBootstrapPhaseResult {
	result := routeRuntimeContractBootstrapPhaseResult{
		mediaDir: filepath.Join(options.dataDir, "media"),
	}
	options.shell.mediaDir = result.mediaDir
	binding.BindShellSurfaceRuntime(options.shell)
	result.shellBound = options.shell.v1 != nil
	result.bootstrap = binding.BindBootstrapSupportRuntime(options.bootstrap)
	if options.onEarlyReady != nil {
		options.onEarlyReady()
		result.earlyReadyTriggered = true
	}
	return result
}
