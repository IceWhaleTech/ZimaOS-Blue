package bootstrap

func (state *routeRegistrationState) setAuthSurface(result routeRuntimeContractAuthSurfaceResult) {
	if state == nil {
		return
	}
	state.authSurface = result
	state.protected = result.protected
	state.apiProtected = result.apiProtected
	state.runtimeSnapshot.auth = result
}

func (state *routeRegistrationState) setStartupAuthRuntime(result routeRuntimeContractStartupAuthResult) {
	if state == nil {
		return
	}
	state.startupAuthRuntime = result
	state.runtimeSnapshot.startupAuth = result
	state.setAuthSurface(result.auth)
}

func (state *routeRegistrationState) setBootstrapSupport(result routeRuntimeContractBootstrapSupportResult) {
	if state == nil {
		return
	}
	state.bootstrapSupport = result
	state.runtimeSnapshot.bootstrap = result
}

func (state *routeRegistrationState) setBootstrapPhaseRuntime(result routeRuntimeContractBootstrapPhaseResult) {
	if state == nil {
		return
	}
	state.bootstrapPhaseRuntime = result
	state.runtimeSnapshot.bootstrapPhase = result
	state.setMediaDir(result.mediaDir)
	state.setBootstrapSupport(result.bootstrap)
}

func (state *routeRegistrationState) setInfrastructureRuntime(result routeRuntimeContractInfrastructureResult) {
	if state == nil {
		return
	}
	state.infrastructureRuntime = result
	state.runtimeSnapshot.infrastructure = result
	state.setMediaRuntime(result.media)
	state.setGatewayRuntime(result.gateway)
}

func (state *routeRegistrationState) setMediaRuntime(result routeRuntimeContractMediaResult) {
	if state == nil {
		return
	}
	state.mediaRuntime = result
	state.runtimeSnapshot.media = result
}

func (state *routeRegistrationState) setGatewayRuntime(result routeRuntimeContractGatewayResult) {
	if state == nil {
		return
	}
	state.gatewayRuntime = result
	state.runtimeSnapshot.gateway = result
}

func (state *routeRegistrationState) setExperienceRuntime(result routeRuntimeContractExperienceResult) {
	if state == nil {
		return
	}
	state.experienceRuntime = result
	state.runtimeSnapshot.experience = result
	state.setSkillAutoReranker(result.skillAutoReranker)
	state.setChatRuntime(result.chat)
	state.setSkillRuntime(result.skill)
}

func (state *routeRegistrationState) setCoreToolingRuntime(result routeRuntimeContractCoreToolingResult) {
	if state == nil {
		return
	}
	state.coreToolingRuntime = result
	state.runtimeSnapshot.coreTooling = result
	if state.deps != nil {
		state.deps.UIReviewerTool = result.tooling.uiReviewerTool
		state.deps.AnalyzeTool = result.analyzeTool
	}
	state.setProviderRuntime(result.provider)
}

func (state *routeRegistrationState) setCoreSupportRuntime(result routeRuntimeContractCoreSupportResult) {
	if state == nil {
		return
	}
	state.coreSupportRuntime = result
	state.runtimeSnapshot.coreSupport = result
	state.setAskSupport(result.ask)
	state.setQuestionManager(result.ask.QuestionManager)
	state.setExecSupport(result.exec)
	state.setExecApprovals(result.exec.Approvals)
}

func (state *routeRegistrationState) setChatRuntime(result routeRuntimeContractChatBindingResult) {
	if state == nil {
		return
	}
	state.chatRuntime = result
	state.runtimeSnapshot.chat = result
}

func (state *routeRegistrationState) setSkillRuntime(result routeRuntimeContractSkillResult) {
	if state == nil {
		return
	}
	state.skillRuntime = result
	state.runtimeSnapshot.skill = result
}

func (state *routeRegistrationState) setAskSupport(result runtimeAskSupportBundle) {
	if state == nil {
		return
	}
	state.askSupport = result
	state.runtimeSnapshot.ask = result
}

func (state *routeRegistrationState) setExecSupport(result runtimeExecSupportBundle) {
	if state == nil {
		return
	}
	state.execSupport = result
	state.runtimeSnapshot.exec = result
}

func (state *routeRegistrationState) setProviderRuntime(result routeRuntimeContractProviderPoolResult) {
	if state == nil {
		return
	}
	state.providerRuntime = result
	state.runtimeSnapshot.provider = result
}
