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
	verifyPrimary func(windowsActionPlan, string) (bool, string),
	verifyFallback func() (bool, string),
	fallback func(string, string, int, bool) (string, error),
) (ActionResult, error) {
	if plan.Unsupported {
		return ActionResult{HostOS: hostOS}, NewError("unsupported_action", plan.UnsupportedReason, map[string]interface{}{"act_type": actType})
	}
	holdMS = NormalizeHoldMS(holdMS)
	primarySucceeded := false
	primaryVerified := false
	verificationMethod := ""
	if primary != nil && plan.Primary != "" {
		if err := primary(plan, value); err == nil {
			primarySucceeded = true
			primaryVerified = true
			if verifyPrimary != nil {
				primaryVerified, verificationMethod = verifyPrimary(plan, value)
			}
			if primaryVerified && !windowsShouldContinueWithFallbackAfterPrimary(actType, plan) {
				return ActionResult{
					HostOS:             hostOS,
					WindowID:           windowID,
					ExecutionMode:      plan.ExecutionMode,
					TargetHit:          true,
					VerificationPassed: true,
					VerificationMethod: valueOrFallback(verificationMethod, "semantic_action"),
					InputMethod:        windowsPrimaryInputMethod(actType, plan),
					Message:            "Host action completed",
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
	fallbackMethod, err := fallback(plan.Fallback, value, holdMS, fallbackPrimarySucceeded)
	if err != nil {
		return ActionResult{HostOS: hostOS}, err
	}
	fallbackVerified := false
	fallbackVerificationMethod := ""
	if strings.EqualFold(strings.TrimSpace(actType), "type") && verifyFallback != nil {
		fallbackVerified, fallbackVerificationMethod = verifyFallback()
	}
	executionMode := "input"
	if primaryVerified && strings.TrimSpace(plan.ExecutionMode) != "" {
		executionMode = plan.ExecutionMode
	}
	verificationPassed := !strings.EqualFold(strings.TrimSpace(actType), "type") || fallbackVerified
	verificationMethod = windowsFallbackVerificationMethod(actType, plan, verificationMethod, fallbackVerificationMethod)
	return ActionResult{
		HostOS:             hostOS,
		WindowID:           windowID,
		ExecutionMode:      executionMode,
		TargetHit:          true,
		VerificationPassed: verificationPassed,
		VerificationMethod: verificationMethod,
		InputMethod:        valueOrFallback(fallbackMethod, plan.Fallback),
		Fallbacks:          windowsFallbackStages(plan, verificationMethod, fallbackMethod, fallbackVerified),
		Message:            "Host action completed",
	}, nil
}

func windowsPrimaryInputMethod(actType string, plan windowsActionPlan) string {
	if strings.EqualFold(strings.TrimSpace(actType), "type") && plan.Primary == windowsActionPutValue {
		return "set_value"
	}
	if plan.ExecutionMode == "semantic" && plan.Primary != "" {
		return "semantic_action"
	}
	return ""
}

func windowsFallbackVerificationMethod(actType string, plan windowsActionPlan, primaryVerification string, fallbackVerification string) string {
	if strings.EqualFold(strings.TrimSpace(actType), "type") {
		if strings.TrimSpace(fallbackVerification) != "" {
			return strings.TrimSpace(fallbackVerification)
		}
		return strings.TrimSpace(primaryVerification)
	}
	if plan.Fallback != "" {
		return "input_action"
	}
	return strings.TrimSpace(primaryVerification)
}

func windowsFallbackStages(plan windowsActionPlan, primaryVerification string, fallbackMethod string, fallbackVerified bool) []string {
	var stages []string
	if plan.Primary == windowsActionPutValue {
		stages = append(stages, "set_value")
		if strings.TrimSpace(primaryVerification) == "" {
			stages = append(stages, "verify_failed")
		}
	}
	if plan.Fallback != "" {
		stages = append(stages, plan.Fallback)
	}
	if strings.TrimSpace(fallbackMethod) != "" {
		stages = append(stages, fallbackMethod)
	}
	if plan.Fallback == windowsActionInputType && !fallbackVerified {
		stages = append(stages, "verify_failed")
	}
	return stages
}

func valueOrFallback(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return strings.TrimSpace(fallback)
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
