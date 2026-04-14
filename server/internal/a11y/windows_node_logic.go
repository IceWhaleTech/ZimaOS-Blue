package a11y

import "strings"

func windowsActionMetadataFromFields(roleText string, stateText string, defaultAction string, hasBounds bool, valueWritable bool) windowsActionMetadata {
	meta := windowsActionMetadata{
		Role:          roleText,
		DefaultAction: defaultAction,
		State:         stateText,
		HasBounds:     hasBounds,
		ValueWritable: valueWritable,
	}
	if expanded, ok := windowsExpandedStateFromText(stateText); ok {
		meta.Expanded = &expanded
	}
	return meta
}

func windowsNodeInteractive(meta windowsActionMetadata) bool {
	return meta.ValueWritable ||
		containsAny(meta.Role, "button", "link", "menu", "list", "tab", "check", "outline", "radio", "tree", "combo box", "switch", "slider") ||
		strings.TrimSpace(meta.DefaultAction) != ""
}

func windowsLikelyValueWritable(roleText string) bool {
	return containsAny(roleText, "editable text", "text", "combo box", "document")
}

func windowsExpandedStateFromText(stateText string) (bool, bool) {
	switch {
	case containsAny(stateText, "expanded"):
		return true, true
	case containsAny(stateText, "collapsed"):
		return false, true
	default:
		return false, false
	}
}
