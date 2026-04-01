package bootstrap

func logTaskSurfaceRegistration(options runtimeTaskSurfaceOptions, registration runtimeTaskSurfaceRegistration) {
	if options.logger == nil {
		return
	}
	if registration.deepResearchRegistered {
		options.logger.Info("Deep research compatibility routes registered")
	}
	if registration.harnessResearchRegistered {
		options.logger.Info("Harness research capability routes registered")
	}
	if registration.harnessRoutesRegistered {
		options.logger.Info("Harness control-plane routes registered")
	}
	if registration.selfReflectRoutesApplied {
		options.logger.Info("Self-reflect proposal routes registered")
	}
}
