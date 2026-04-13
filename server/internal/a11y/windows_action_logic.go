package a11y

func windowsActionResultWithPlan(
	hostOS string,
	windowID string,
	actType string,
	value string,
	holdMS int,
	plan windowsActionPlan,
	primary func(windowsActionPlan, string) error,
	fallback func(string, string, int) error,
) (ActionResult, error) {
	if plan.Unsupported {
		return ActionResult{HostOS: hostOS}, NewError("unsupported_action", plan.UnsupportedReason, map[string]interface{}{"act_type": actType})
	}
	holdMS = NormalizeHoldMS(holdMS)
	if primary != nil && plan.Primary != "" {
		if err := primary(plan, value); err == nil {
			return ActionResult{
				HostOS:        hostOS,
				WindowID:      windowID,
				ExecutionMode: plan.ExecutionMode,
				Message:       "Host action completed",
			}, nil
		}
	}
	if plan.Fallback == "" {
		return ActionResult{HostOS: hostOS}, NewError("unsupported_action", "element could not be activated on this host", map[string]interface{}{"act_type": actType})
	}
	if fallback == nil {
		return ActionResult{HostOS: hostOS}, NewError("unsupported_action", "input fallback is unavailable", map[string]interface{}{"act_type": actType})
	}
	if err := fallback(plan.Fallback, value, holdMS); err != nil {
		return ActionResult{HostOS: hostOS}, err
	}
	return ActionResult{
		HostOS:        hostOS,
		WindowID:      windowID,
		ExecutionMode: "input",
		Message:       "Host action completed",
	}, nil
}
