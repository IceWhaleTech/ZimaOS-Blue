package a11y

import "strings"

func NormalizeHoldMS(raw int) int {
	return normalizeHoldMS(raw)
}

func normalizeHoldMS(raw int) int {
	if raw <= 0 {
		return DefaultHoldMS
	}
	return raw
}

type darwinActionMetadata struct {
	Role             string
	DefaultAction    string
	AvailableActions []string
	ValueSettable    bool
	Expanded         *bool
}

type darwinActionPlan struct {
	ExecutionMode     string
	SemanticAction    string
	SetValue          bool
	InputFallback     string
	Unsupported       bool
	UnsupportedReason string
}

const (
	darwinInputFallbackClick       = "click"
	darwinInputFallbackDoubleClick = "double_click"
	darwinInputFallbackRightClick  = "right_click"
	darwinInputFallbackClickHold   = "click_hold"
	darwinInputFallbackType        = "type"
)

func planDarwinAction(actType string, meta darwinActionMetadata) darwinActionPlan {
	actType = strings.TrimSpace(strings.ToLower(actType))
	role := strings.TrimSpace(strings.ToLower(meta.Role))
	defaultAction := strings.TrimSpace(strings.ToLower(meta.DefaultAction))

	switch actType {
	case "click", "submit":
		if hasAnyDarwinAction(meta.AvailableActions, "AXPress", "AXConfirm") || containsAny(defaultAction, "press", "click", "confirm") {
			return darwinActionPlan{ExecutionMode: "semantic", SemanticAction: "AXPress"}
		}
		return darwinActionPlan{ExecutionMode: "input", InputFallback: darwinInputFallbackClick}
	case "double_click":
		return darwinActionPlan{ExecutionMode: "input", InputFallback: darwinInputFallbackDoubleClick}
	case "right_click":
		return darwinActionPlan{ExecutionMode: "input", InputFallback: darwinInputFallbackRightClick}
	case "long_press":
		return darwinActionPlan{ExecutionMode: "input", InputFallback: darwinInputFallbackClickHold}
	case "focus":
		if hasAnyDarwinAction(meta.AvailableActions, "AXRaise") {
			return darwinActionPlan{ExecutionMode: "semantic", SemanticAction: "AXRaise"}
		}
		return darwinActionPlan{ExecutionMode: "input", InputFallback: darwinInputFallbackClick}
	case "type":
		if meta.ValueSettable {
			return darwinActionPlan{ExecutionMode: "semantic", SetValue: true, InputFallback: darwinInputFallbackType}
		}
		return darwinActionPlan{ExecutionMode: "input", InputFallback: darwinInputFallbackType}
	case "select":
		if supportsDarwinSelection(role, defaultAction) && (hasAnyDarwinAction(meta.AvailableActions, "AXPress") || containsAny(defaultAction, "select", "press", "pick")) {
			return darwinActionPlan{ExecutionMode: "semantic", SemanticAction: "AXPress"}
		}
		return darwinActionPlan{Unsupported: true, UnsupportedReason: "element does not expose a selectable semantic action"}
	case "toggle":
		if supportsDarwinToggle(role, defaultAction) && (hasAnyDarwinAction(meta.AvailableActions, "AXPress") || containsAny(defaultAction, "press", "toggle")) {
			return darwinActionPlan{ExecutionMode: "semantic", SemanticAction: "AXPress"}
		}
		return darwinActionPlan{Unsupported: true, UnsupportedReason: "element does not expose a toggle semantic action"}
	case "expand":
		if meta.Expanded == nil {
			return darwinActionPlan{Unsupported: true, UnsupportedReason: "expanded state is unavailable"}
		}
		if *meta.Expanded {
			return darwinActionPlan{Unsupported: true, UnsupportedReason: "element is already expanded"}
		}
		if hasAnyDarwinAction(meta.AvailableActions, "AXPress") || containsAny(defaultAction, "press", "expand", "show") {
			return darwinActionPlan{ExecutionMode: "semantic", SemanticAction: "AXPress"}
		}
		return darwinActionPlan{Unsupported: true, UnsupportedReason: "element does not expose an expand semantic action"}
	case "collapse":
		if meta.Expanded == nil {
			return darwinActionPlan{Unsupported: true, UnsupportedReason: "expanded state is unavailable"}
		}
		if !*meta.Expanded {
			return darwinActionPlan{Unsupported: true, UnsupportedReason: "element is already collapsed"}
		}
		if hasAnyDarwinAction(meta.AvailableActions, "AXPress") || containsAny(defaultAction, "press", "collapse", "hide") {
			return darwinActionPlan{ExecutionMode: "semantic", SemanticAction: "AXPress"}
		}
		return darwinActionPlan{Unsupported: true, UnsupportedReason: "element does not expose a collapse semantic action"}
	default:
		return darwinActionPlan{Unsupported: true, UnsupportedReason: "unsupported action"}
	}
}

func hasAnyDarwinAction(actions []string, expected ...string) bool {
	for _, action := range actions {
		trimmed := strings.TrimSpace(action)
		for _, candidate := range expected {
			if strings.EqualFold(trimmed, candidate) {
				return true
			}
		}
	}
	return false
}

func supportsDarwinSelection(role string, defaultAction string) bool {
	return containsAny(role, "button", "radio", "tab", "menu", "list", "row", "cell", "link") || containsAny(defaultAction, "select", "pick")
}

func supportsDarwinToggle(role string, defaultAction string) bool {
	return containsAny(role, "checkbox", "switch", "toggle", "disclosure") || containsAny(defaultAction, "toggle", "press")
}

type windowsActionMetadata struct {
	Role          string
	DefaultAction string
	State         string
	HasBounds     bool
	ValueWritable bool
	Expanded      *bool
}

type windowsActionPlan struct {
	ExecutionMode     string
	Primary           string
	Fallback          string
	SelectFlags       uint32
	Unsupported       bool
	UnsupportedReason string
}

const (
	windowsActionDefaultAction    = "default_action"
	windowsActionSelectFocus      = "select_focus"
	windowsActionSelectSelection  = "select_selection"
	windowsActionPutValue         = "put_value"
	windowsActionInputClick       = "input_click"
	windowsActionInputFocusClick  = "input_focus_click"
	windowsActionInputDoubleClick = "input_double_click"
	windowsActionInputRightClick  = "input_right_click"
	windowsActionInputLongPress   = "input_long_press"
	windowsActionInputType        = "input_type"
	windowsActionInputScroll      = "input_scroll"
	windowsActionInputPointer     = "input_pointer"
	windowsActionInputKey         = "input_key"

	windowsSELFLAGTakeFocus     uint32 = 0x1
	windowsSELFLAGTakeSelection uint32 = 0x2
)

func planWindowsAction(actType string, meta windowsActionMetadata) windowsActionPlan {
	actType = strings.TrimSpace(strings.ToLower(actType))
	role := strings.TrimSpace(strings.ToLower(meta.Role))
	defaultAction := strings.TrimSpace(strings.ToLower(meta.DefaultAction))
	state := strings.TrimSpace(strings.ToLower(meta.State))

	switch actType {
	case "double_click":
		return windowsActionPlan{
			ExecutionMode: "input",
			Fallback:      fallbackIf(meta.HasBounds, windowsActionInputDoubleClick),
		}
	case "right_click":
		return windowsActionPlan{
			ExecutionMode: "input",
			Fallback:      fallbackIf(meta.HasBounds, windowsActionInputRightClick),
		}
	case "long_press":
		return windowsActionPlan{
			ExecutionMode: "input",
			Fallback:      fallbackIf(meta.HasBounds, windowsActionInputLongPress),
		}
	case "click", "submit":
		return windowsActionPlan{
			ExecutionMode: "semantic",
			Primary:       windowsActionDefaultAction,
			Fallback:      fallbackIf(meta.HasBounds, windowsActionInputClick),
		}
	case "focus":
		return windowsActionPlan{
			ExecutionMode: "semantic",
			Primary:       windowsActionSelectFocus,
			SelectFlags:   windowsSELFLAGTakeFocus,
			Fallback:      fallbackIf(meta.HasBounds, windowsActionInputFocusClick),
		}
	case "select":
		if supportsWindowsSelection(role, defaultAction, state) {
			return windowsActionPlan{
				ExecutionMode: "semantic",
				Primary:       windowsActionSelectSelection,
				SelectFlags:   windowsSELFLAGTakeSelection,
			}
		}
		return windowsActionPlan{Unsupported: true, UnsupportedReason: "element does not support selection"}
	case "type":
		if meta.ValueWritable {
			return windowsActionPlan{
				ExecutionMode: "semantic",
				Primary:       windowsActionPutValue,
				Fallback:      windowsActionInputType,
			}
		}
		return windowsActionPlan{
			ExecutionMode: "input",
			Primary:       windowsActionSelectFocus,
			SelectFlags:   windowsSELFLAGTakeFocus,
			Fallback:      windowsActionInputType,
		}
	case "toggle":
		if supportsWindowsToggle(role, defaultAction, state) {
			return windowsActionPlan{
				ExecutionMode: "semantic",
				Primary:       windowsActionDefaultAction,
				Fallback:      fallbackIf(meta.HasBounds, windowsActionInputClick),
			}
		}
		return windowsActionPlan{Unsupported: true, UnsupportedReason: "element does not support toggle"}
	case "expand":
		if meta.Expanded == nil {
			return windowsActionPlan{Unsupported: true, UnsupportedReason: "expanded state is unavailable"}
		}
		if *meta.Expanded {
			return windowsActionPlan{Unsupported: true, UnsupportedReason: "element is already expanded"}
		}
		if supportsWindowsExpandCollapse(role, defaultAction) {
			return windowsActionPlan{
				ExecutionMode: "semantic",
				Primary:       windowsActionDefaultAction,
				Fallback:      fallbackIf(meta.HasBounds, windowsActionInputClick),
			}
		}
		return windowsActionPlan{Unsupported: true, UnsupportedReason: "element does not support expand"}
	case "collapse":
		if meta.Expanded == nil {
			return windowsActionPlan{Unsupported: true, UnsupportedReason: "expanded state is unavailable"}
		}
		if !*meta.Expanded {
			return windowsActionPlan{Unsupported: true, UnsupportedReason: "element is already collapsed"}
		}
		if supportsWindowsExpandCollapse(role, defaultAction) {
			return windowsActionPlan{
				ExecutionMode: "semantic",
				Primary:       windowsActionDefaultAction,
				Fallback:      fallbackIf(meta.HasBounds, windowsActionInputClick),
			}
		}
		return windowsActionPlan{Unsupported: true, UnsupportedReason: "element does not support collapse"}
	default:
		return windowsActionPlan{Unsupported: true, UnsupportedReason: "unsupported action"}
	}
}

func supportsWindowsSelection(role string, defaultAction string, state string) bool {
	return containsAny(role, "list", "list item", "outline", "outline item", "menu item", "page tab", "radio button") ||
		containsAny(defaultAction, "select") ||
		containsAny(state, "selectable", "selected")
}

func supportsWindowsToggle(role string, defaultAction string, state string) bool {
	return containsAny(role, "check box", "checkbox", "push button", "button", "outline button") ||
		containsAny(defaultAction, "press", "toggle") ||
		containsAny(state, "checked", "pressed", "mixed")
}

func supportsWindowsExpandCollapse(role string, defaultAction string) bool {
	return containsAny(role, "outline", "outline button", "tree item", "button") ||
		containsAny(defaultAction, "press", "open", "close", "expand", "collapse")
}

func containsAny(value string, parts ...string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return false
	}
	for _, part := range parts {
		if strings.Contains(value, strings.TrimSpace(strings.ToLower(part))) {
			return true
		}
	}
	return false
}

func fallbackIf(enabled bool, value string) string {
	if enabled {
		return value
	}
	return ""
}

func normalizeScrollLines(lines int) int {
	if lines <= 0 {
		return 6
	}
	return lines
}
