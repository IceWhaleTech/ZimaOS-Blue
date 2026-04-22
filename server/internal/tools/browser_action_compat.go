package tools

import "strings"

const browserLegacyPageScrollStep = 640

var browserActionEnum = []string{
	"navigate",
	"snapshot",
	"snapshot_interactive",
	"snapshot_auto",
	"act",
	"screenshot",
	"tabs",
	"close",
	"recipe",
	"recipes",
}

func normalizeBrowserActionIntentPhrase(action string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(action))
	switch {
	case strings.Contains(lower, "interactive"),
		strings.Contains(lower, "actionable"),
		strings.Contains(lower, "clickable"),
		strings.Contains(lower, "elements"),
		strings.Contains(lower, "element"),
		strings.Contains(lower, "交互"),
		strings.Contains(lower, "元素"):
		return "snapshot_interactive", true
	default:
		return "", false
	}
}

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
	case "act", "click", "type", "focus", "hover", "scroll", "select", "scroll_down", "scroll_up", "scroll_page":
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
	rawAction := strings.TrimSpace(action)
	canonicalAction := strings.ToLower(rawAction)
	canonicalActType := strings.ToLower(strings.TrimSpace(actType))
	if mappedAction, ok := NormalizeBrowserActionAlias(canonicalAction); ok {
		canonicalAction = mappedAction
	} else if intentAction, ok := normalizeBrowserActionIntentPhrase(rawAction); ok {
		canonicalAction = intentAction
	} else if fuzzyAction, ok := resolveFuzzySchemaEnumValue(rawAction, browserActionEnum); ok {
		canonicalAction = fuzzyAction
	}

	switch canonicalAction {
	case "click", "type", "focus", "hover", "scroll", "select":
		if canonicalActType == "" {
			canonicalActType = canonicalAction
		}
		canonicalAction = "act"
	case "scroll_down":
		if canonicalActType == "" {
			canonicalActType = "down"
		}
		canonicalAction = "scroll_page"
	case "scroll_up":
		if canonicalActType == "" {
			canonicalActType = "up"
		}
		canonicalAction = "scroll_page"
	case "read":
		canonicalAction = "snapshot_auto"
	}

	if canonicalAction == "act" {
		switch canonicalActType {
		case "scroll_down":
			canonicalAction = "scroll_page"
			canonicalActType = "down"
		case "scroll_up":
			canonicalAction = "scroll_page"
			canonicalActType = "up"
		}
	}

	return canonicalAction, canonicalActType
}

// BrowserLegacyPageScrollDelta maps compatibility page-scroll directions to a
// relative browser scroll delta.
func BrowserLegacyPageScrollDelta(action string, actType string) (string, int, int, bool) {
	canonicalAction, canonicalActType := CanonicalizeBrowserAction(action, actType)
	if canonicalAction != "scroll_page" {
		return "", 0, 0, false
	}

	switch canonicalActType {
	case "down":
		return "down", 0, browserLegacyPageScrollStep, true
	case "up":
		return "up", 0, -browserLegacyPageScrollStep, true
	default:
		return "", 0, 0, false
	}
}
