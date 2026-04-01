package bootstrap

import "context"

func registerRuntimeActivationSupportRoutes(
	activation runtimeActivationResult,
	options runtimeActivationRouteSupportOptions,
) {
	var runner layeredMemoryAgentTarget
	if activation.agentRunner != nil {
		runner = activation.agentRunner
	}
	registerMemoryRoutes(
		options.v1,
		options.memoryHandler,
		options.authMiddleware,
		options.pageMiddleware,
		options.chat,
		runner,
		options.reflector,
	)
}

func bindRuntimeActivationSupport(
	activation runtimeActivationResult,
	options runtimeActivationDeferredSupportOptions,
) {
	ctx := options.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	var runner agentRuntimePolicyTarget
	if activation.agentRunner != nil {
		runner = activation.agentRunner
	}
	bindRuntimeActivationDeferred(ctx, activation, options, runner)
	bindRuntimeActivationApproval(activation, options)
}
