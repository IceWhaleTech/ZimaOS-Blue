package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"

func withRuntimeToolingSkillFallbacks(options routeRuntimeContractToolingOptions) routeRuntimeContractToolingOptions {
	if options.browserSkill == nil {
		options.browserSkill = runtimeSkillAsBrowserTarget(options.skillRegistry, "browser")
	}
	if options.uiReviewerSkill == nil {
		options.uiReviewerSkill = runtimeSkillAsUIReviewerTarget(options.skillRegistry, "ui_reviewer")
	}
	if options.reminderSkill == nil {
		options.reminderSkill = runtimeSkillAsReminderTarget(options.skillRegistry, "reminder")
	}
	return options
}

func bindRuntimeToolingBrowserRuntime(options routeRuntimeContractToolingOptions) {
	// Browser tool registration removed - browser now fully migrated to skill/exec routing
	// The browser backend is still wired into browserSkill for internal use
	_ = options.browserBackend
	if options.sttService != nil {
		tools.AttachSTTServiceToWebTools(options.registry, options.sttService)
	}
	bindRuntimeBrowserTargets(
		options.mediaDir,
		options.browserBackend,
		options.lightpanda,
		runtimeToolingBrowserMediaDirTarget(options.registry),
		options.browserSkill,
		options.uiReviewerSkill,
		newRuntimeBrowserSkillService(options.acquireBrowser, options.lazyBrowser),
		runtimeToolingBrowserAccessTargets(options.registry)...,
	)
}

func runtimeToolingBrowserMediaDirTarget(registry *tools.Registry) runtimeBrowserMediaDirTarget {
	if registry == nil {
		return nil
	}
	// Browser tool no longer registered as native tool - now skill-based only
	// This function returns nil which is handled gracefully by callers
	_ = tools.GetBrowserTool(registry) // kept for import compatibility
	return nil
}

func runtimeToolingBrowserAccessTargets(registry *tools.Registry) []runtimeBrowserAccessTarget {
	if registry == nil {
		return nil
	}
	accessTargets := make([]runtimeBrowserAccessTarget, 0, 4)
	if tool := tools.GetWebTool(registry); tool != nil {
		accessTargets = append(accessTargets, tool)
	}
	if tool := tools.GetWebFetchTool(registry); tool != nil {
		accessTargets = append(accessTargets, tool)
	}
	if tool := tools.GetWebReadTool(registry); tool != nil {
		accessTargets = append(accessTargets, tool)
	}
	if tool := tools.GetWebExtractTool(registry); tool != nil {
		accessTargets = append(accessTargets, tool)
	}
	return accessTargets
}

func bindRuntimeToolingReminderRuntime(options routeRuntimeContractToolingOptions) {
	bindRuntimeReminderServices(
		options.registry,
		options.pushService,
		runtimeToolingReminderCalendarTarget(options.registry),
		options.reminderSkill,
	)
}

func runtimeToolingReminderCalendarTarget(registry *tools.Registry) runtimeReminderCalendarTarget {
	if registry == nil {
		return nil
	}
	return tools.GetCalendarTool(registry)
}

func bindRuntimeToolingImageRuntime(options routeRuntimeContractToolingOptions, uiReviewer *tools.UIReviewerTool) {
	bindRuntimeImageTools(
		options.registry,
		nil,
		uiReviewer,
		options.workspaceAllowedPaths,
		options.mediaManager,
		options.mediaStorage,
		options.llmRegistry,
		options.ocr,
		options.providerPool,
	)
}
