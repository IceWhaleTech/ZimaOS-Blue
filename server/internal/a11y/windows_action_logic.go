package a11y

import "strings"

func windowsActionResultWithPlan(
	hostOS string,
	windowID string,
	actType string,
	value string,
	holdMS int,
	plan windowsActionPlan,
	primary func(windowsActionPlan, string) error,
	verifyPrimary func(windowsActionPlan, string) bool,
	fallback func(string, string, int, bool) error,
) (ActionResult, error) {
	if plan.Unsupported {
		return ActionResult{HostOS: hostOS}, NewError("unsupported_action", plan.UnsupportedReason, map[string]interface{}{"act_type": actType})
	}
	holdMS = NormalizeHoldMS(holdMS)
	primarySucceeded := false
	primaryVerified := false
	if primary != nil && plan.Primary != "" {
		if err := primary(plan, value); err == nil {
			primarySucceeded = true
			primaryVerified = true
			if verifyPrimary != nil {
				primaryVerified = verifyPrimary(plan, value)
			}
			if primaryVerified && !windowsShouldContinueWithFallbackAfterPrimary(actType, plan) {
				return ActionResult{
					HostOS:        hostOS,
					WindowID:      windowID,
					ExecutionMode: plan.ExecutionMode,
					Message:       "Host action completed",
				}, nil
			}
		}
	}
	if plan.Fallback == "" {
		return ActionResult{HostOS: hostOS}, NewError("unsupported_action", "element could not be activated on this host", map[string]interface{}{"act_type": actType})
	}
	if fallback == nil {
		return ActionResult{HostOS: hostOS}, NewError("unsupported_action", "input fallback is unavailable", map[string]interface{}{"act_type": actType})
	}
	fallbackPrimarySucceeded := primarySucceeded && windowsPrimarySuccessShouldSkipFallbackClick(plan, primaryVerified)
	if err := fallback(plan.Fallback, value, holdMS, fallbackPrimarySucceeded); err != nil {
		return ActionResult{HostOS: hostOS}, err
	}
	executionMode := "input"
	if primaryVerified && strings.TrimSpace(plan.ExecutionMode) != "" {
		executionMode = plan.ExecutionMode
	}
	return ActionResult{
		HostOS:        hostOS,
		WindowID:      windowID,
		ExecutionMode: executionMode,
		Message:       "Host action completed",
	}, nil
}

func windowsShouldContinueWithFallbackAfterPrimary(actType string, plan windowsActionPlan) bool {
	return strings.EqualFold(strings.TrimSpace(actType), "type") && strings.TrimSpace(plan.Fallback) == windowsActionInputType
}

func windowsPrimarySuccessShouldSkipFallbackClick(plan windowsActionPlan, primaryVerified bool) bool {
	if !primaryVerified && plan.Primary == windowsActionPutValue {
		return false
	}
	return plan.Primary == windowsActionSelectFocus
}
