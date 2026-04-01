package bootstrap

func applyRouteRuntimeOperationalSupport(binding routeRuntimeOperationalBinding, activation runtimeActivationResult, options routeRuntimeContractSupportOptions) bool {
	if binding == nil || options.v1 == nil {
		return false
	}
	binding.RegisterActivationSupportRoutes(activation, options)
	return true
}

func applyRouteRuntimeOperationalDeferredSupport(binding routeRuntimeOperationalBinding, activation runtimeActivationResult, options routeRuntimeContractDeferredSupportOptions) bool {
	if binding == nil || !hasRouteRuntimeOperationalDeferredSupport(options) {
		return false
	}
	binding.BindDeferredSupport(activation, options)
	return true
}

func hasRouteRuntimeOperationalDeferredSupport(options routeRuntimeContractDeferredSupportOptions) bool {
	return options.services != nil || options.deps != nil || options.settings != nil || options.smallModelManager != nil ||
		routeRuntimeHasValue(options.chatSmallModel) ||
		routeRuntimeHasValue(options.analyzeSmallModel) ||
		routeRuntimeHasValue(options.smallModelStats) ||
		options.reflectionProposalGate != nil ||
		routeRuntimeHasValue(options.compactorChat) ||
		options.memoryHandler != nil ||
		routeRuntimeHasValue(options.providerSettings) ||
		routeRuntimeHasValue(options.chatSettings) ||
		routeRuntimeHasValue(options.skillReranker) ||
		routeRuntimeHasValue(options.promptSettings) ||
		routeRuntimeHasValue(options.pushLocale) || routeRuntimeHasValue(options.mgmtSettings) || routeRuntimeHasValue(options.questionMgr) ||
		options.approvalHandler != nil ||
		options.execApprovals != nil ||
		routeRuntimeHasValue(options.workflowTarget) ||
		routeRuntimeHasValue(options.metrics) ||
		routeRuntimeHasValue(options.chatApprover)
}
