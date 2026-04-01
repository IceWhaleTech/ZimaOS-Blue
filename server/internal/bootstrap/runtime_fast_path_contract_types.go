package bootstrap

type routeRuntimeFastPathBinding interface {
	BindStartupSurfaceRuntime(options routeRuntimeContractStartupSurfaceOptions)
	BindAuthSurfaceRuntime(options routeRuntimeContractAuthSurfaceOptions) routeRuntimeContractAuthSurfaceResult
	BindAccountSurfaceRuntime(options routeRuntimeContractAccountSurfaceOptions)
	BindShellSurfaceRuntime(options routeRuntimeContractShellSurfaceOptions)
	BindBootstrapSupportRuntime(options routeRuntimeContractBootstrapSupportOptions) routeRuntimeContractBootstrapSupportResult
}

var _ routeRuntimeFastPathBinding = (*runtimeContractBinding)(nil)

type routeRuntimeContractStartupAuthOptions struct {
	startup routeRuntimeContractStartupSurfaceOptions
	auth    routeRuntimeContractAuthSurfaceOptions
	account routeRuntimeContractAccountSurfaceOptions
}

type routeRuntimeContractStartupAuthResult struct {
	startupBound bool
	auth         routeRuntimeContractAuthSurfaceResult
	accountBound bool
}

type routeRuntimeContractBootstrapPhaseOptions struct {
	dataDir      string
	shell        routeRuntimeContractShellSurfaceOptions
	bootstrap    routeRuntimeContractBootstrapSupportOptions
	onEarlyReady func()
}

type routeRuntimeContractBootstrapPhaseResult struct {
	mediaDir            string
	shellBound          bool
	bootstrap           routeRuntimeContractBootstrapSupportResult
	earlyReadyTriggered bool
}
