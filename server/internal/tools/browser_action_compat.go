package tools

import "strings"

// CanonicalizeBrowserAction normalizes legacy top-level browser act aliases.
// Older callers may send action=scroll/click/type/etc. instead of
// action=act + act_type=<verb>. This keeps those calls working.
func CanonicalizeBrowserAction(action string, actType string) (string, string) {
	canonicalAction := strings.ToLower(strings.TrimSpace(action))
	canonicalActType := strings.ToLower(strings.TrimSpace(actType))

	switch canonicalAction {
	case "click", "type", "focus", "hover", "scroll", "select":
		if canonicalActType == "" {
			canonicalActType = canonicalAction
		}
		canonicalAction = "act"
	case "read":
		canonicalAction = "snapshot_auto"
	}

	return canonicalAction, canonicalActType
}
