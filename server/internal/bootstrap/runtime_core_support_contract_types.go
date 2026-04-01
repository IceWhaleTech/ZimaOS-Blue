package bootstrap

type routeRuntimeCoreSupportBinding interface {
	NewAskSupportBundle(options routeRuntimeContractAskSupportOptions) runtimeAskSupportBundle
	NewExecSupportBundle(options routeRuntimeContractExecSupportOptions) runtimeExecSupportBundle
	BindCapabilitySupportRuntime(options routeRuntimeContractCapabilitySupportOptions) routeRuntimeContractCapabilitySupportResult
}

var _ routeRuntimeCoreSupportBinding = (*runtimeContractBinding)(nil)

type routeRuntimeContractCoreSupportOptions struct {
	ask        routeRuntimeContractAskSupportOptions
	exec       routeRuntimeContractExecSupportOptions
	capability routeRuntimeContractCapabilitySupportOptions
}

type routeRuntimeContractCoreSupportResult struct {
	ask        runtimeAskSupportBundle
	exec       runtimeExecSupportBundle
	capability routeRuntimeContractCapabilitySupportResult
}
