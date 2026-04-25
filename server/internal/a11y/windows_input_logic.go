package a11y

import "strings"

const windowsScrollDeltaUnit = 120

func windowsScrollResultWithInput(
	hostOS string,
	windowID string,
	direction string,
	lines int,
	bringFront func(),
	send func(horizontal bool, delta int32) error,
) (ActionResult, error) {
	horizontal, delta := windowsScrollDelta(direction, lines)
	if bringFront != nil {
		bringFront()
	}
	if send != nil {
		if err := send(horizontal, delta); err != nil {
			return ActionResult{HostOS: hostOS}, err
		}
	}
	return ActionResult{
		HostOS:        hostOS,
		WindowID:      windowID,
		ExecutionMode: "input",
		Message:       "Scroll completed",
	}, nil
}

func windowsScrollDelta(direction string, lines int) (bool, int32) {
	lines = normalizeScrollLines(lines)
	delta := int32(lines * windowsScrollDeltaUnit)
	switch normalizeDirection(direction) {
	case "up":
		return false, delta
	case "left":
		return true, -delta
	case "right":
		return true, delta
	default:
		return false, -delta
	}
}

func windowsPointerMoveResult(hostOS string, x int, y int, move func(int, int) error) (ActionResult, error) {
	if move != nil {
		if err := move(x, y); err != nil {
			return ActionResult{HostOS: hostOS}, err
		}
	}
	return ActionResult{
		HostOS:        hostOS,
		ExecutionMode: "input",
		Message:       "Pointer moved",
	}, nil
}

func windowsKeyResultWithInput(
	hostOS string,
	windowID string,
	keys []string,
	holdMS int,
	bringFront func(),
	send func([]string, int) error,
) (ActionResult, error) {
	holdMS = NormalizeHoldMS(holdMS)
	if bringFront != nil {
		bringFront()
	}
	if send != nil {
		if err := send(keys, holdMS); err != nil {
			return ActionResult{HostOS: hostOS}, err
		}
	}
	return ActionResult{
		HostOS:        hostOS,
		WindowID:      windowID,
		ExecutionMode: "input",
		Message:       "Keys sent",
	}, nil
}

func windowsFocusedTextResultWithInput(
	hostOS string,
	windowID string,
	value string,
	sendText func(string) (string, error),
) (ActionResult, error) {
	inputMethod := "input_type"
	if sendText != nil {
		method, err := sendText(value)
		if err != nil {
			return ActionResult{HostOS: hostOS}, err
		}
		if method != "" {
			inputMethod = method
		}
	}
	return ActionResult{
		HostOS:             hostOS,
		WindowID:           windowID,
		ExecutionMode:      "input",
		TargetHit:          true,
		VerificationPassed: true,
		VerificationMethod: "focused_text",
		InputMethod:        inputMethod,
		Message:            "Host action completed",
	}, nil
}

func normalizeDirection(direction string) string {
	switch normalizeCompactToken(direction) {
	case "up":
		return "up"
	case "left":
		return "left"
	case "right":
		return "right"
	default:
		return "down"
	}
}

func normalizeCompactToken(value string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	replacer := strings.NewReplacer(" ", "", "_", "", "-", "")
	return replacer.Replace(normalized)
}
