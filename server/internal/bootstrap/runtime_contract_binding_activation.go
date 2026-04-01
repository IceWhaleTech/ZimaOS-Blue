package bootstrap

func (binding *runtimeContractBinding) approvalDetailTarget() approvalRuntimeDetailTarget {
	if binding == nil {
		return nil
	}
	return binding.runtime.ApprovalDetailTarget()
}

func (binding *runtimeContractBinding) ActivateRouteRuntime(options routeRuntimeContractActivationOptions) runtimeActivationResult {
	if binding == nil {
		return runtimeActivationResult{}
	}
	return activateRouteRuntimeActivation(routeRuntimeActivationOptions{
		e:                     options.e,
		v1:                    options.v1,
		protected:             options.protected,
		deps:                  options.deps,
		logger:                options.logger,
		runtimeLLM:            options.runtimeLLM,
		oauthManager:          options.oauthManager,
		harnessRuntime:        binding.HarnessRuntime(),
		reflectService:        binding.runtime.ReflectService(),
		requirePagePermission: options.requirePagePermission,
		authPageV1Group:       options.authPageV1Group,
	})
}

func (binding *runtimeContractBinding) RegisterActivationSupportRoutes(
	activation runtimeActivationResult,
	options routeRuntimeContractSupportOptions,
) {
	if binding == nil {
		return
	}
	registerRuntimeActivationSupportRoutes(activation, runtimeActivationRouteSupportOptions{
		v1:             options.v1,
		authMiddleware: options.authMiddleware,
		pageMiddleware: options.pageMiddleware,
		memoryHandler:  options.memoryHandler,
		chat:           options.chat,
		reflector:      binding.runtime.ReflectService(),
	})
}

func (binding *runtimeContractBinding) BindDeferredSupport(
	activation runtimeActivationResult,
	options routeRuntimeContractDeferredSupportOptions,
) {
	if binding == nil {
		return
	}
	bindRuntimeActivationSupport(activation, runtimeActivationDeferredSupportOptions{
		ctx:                    options.ctx,
		services:               options.services,
		deps:                   options.deps,
		settings:               options.settings,
		smallModelManager:      options.smallModelManager,
		chatSmallModel:         options.chatSmallModel,
		analyzeSmallModel:      options.analyzeSmallModel,
		smallModelStats:        options.smallModelStats,
		harnessRuntime:         binding.HarnessRuntime(),
		reflectionTarget:       binding.runtime.ReflectService(),
		reflectionProposalGate: options.reflectionProposalGate,
		compactorChat:          options.compactorChat,
		memoryHandler:          options.memoryHandler,
		providerSettings:       options.providerSettings,
		chatSettings:           options.chatSettings,
		researchSettings:       binding.runtime.ResearchService(),
		skillReranker:          options.skillReranker,
		skillRerankerDefaults:  options.skillRerankerDefaults,
		promptSettings:         options.promptSettings,
		pushLocale:             options.pushLocale,
		mgmtSettings:           options.mgmtSettings,
		questionMgr:            options.questionMgr,
		approvalHandler:        options.approvalHandler,
		execApprovals:          options.execApprovals,
		detailTarget:           binding.approvalDetailTarget(),
		workflowTarget:         options.workflowTarget,
		metrics:                options.metrics,
		chatApprover:           options.chatApprover,
	})
}
