package bootstrap

func activateRuntimeProxyEntry(options runtimeProxyEntryOptions) runtimeProxyEntryResult {
	masking := newRuntimeProxyEntryMaskingBinding(options)
	result := masking.result

	laneOptions := newRuntimeProxyLaneOptions(options, result.dataMasker)
	if laneOptions == nil {
		return result
	}

	lane := activateRuntimeProxyLane(*laneOptions)
	if lane == nil {
		if options.logger != nil {
			options.logger.Warn("Proxy runtime surface could not be initialized")
		}
		return result
	}

	options.proxyBridgeSurface.bind(lane.handler, options.runtimeLLM, options.auxiliaryLLM)
	result.lane = lane
	masking.setMaskingHook(lane.saveToggle)
	result.maskingOnToggle = lane.saveToggle
	if options.runtimeLLM != nil {
		result.agentLLMCaller = options.runtimeLLM
	}
	return result
}
