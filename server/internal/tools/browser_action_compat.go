package tools

import "strings"

// NormalizeBrowserActionAlias maps browser CLI and skill aliases to the
// canonical action family used by browser tooling.
func NormalizeBrowserActionAlias(action string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "navigate", "open", "goto", "go", "visit":
		return "navigate", true
	case "snapshot", "inspect", "tree":
		return "snapshot", true
	case "snapshot_interactive", "interactive", "elements":
		return "snapshot_interactive", true
	case "snapshot_auto", "read", "page":
		return "snapshot_auto", true
	case "act", "click", "type", "focus", "hover", "scroll", "select":
		return strings.ToLower(strings.TrimSpace(action)), true
	case "screenshot", "shot", "capture", "screen":
		return "screenshot", true
	case "tabs", "list", "ls", "tab", "status":
		return "tabs", true
	case "close", "remove", "rm", "delete":
		return "close", true
	case "recipe", "run_recipe", "run-recipe":
		return "recipe", true
	case "recipes", "list_recipes", "list-recipes":
		return "recipes", true
	default:
		return "", false
	}
}

// LooksLikeBrowserURL reports whether the provided string is probably a URL
// acceptable to browser navigation and screenshot entry points.
func LooksLikeBrowserURL(raw string) bool {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.HasPrefix(value, "http://"),
		strings.HasPrefix(value, "https://"),
		strings.HasPrefix(value, "file://"),
		strings.HasPrefix(value, "ftp://"),
		strings.HasPrefix(value, "www."),
		strings.HasPrefix(value, "localhost:"),
		strings.HasPrefix(value, "127.0.0.1:"),
		strings.HasPrefix(value, "[::1]:"):
		return true
	default:
		return false
	}
}

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
