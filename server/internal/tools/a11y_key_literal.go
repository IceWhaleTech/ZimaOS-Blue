package tools

import (
	"strings"
	"unicode"
)

func normalizeA11yShortcutLiteralKeys(keys []string) []string {
	if len(keys) != 1 {
		return keys
	}
	if normalized, ok := parseA11yKeyLiteral(keys[0]); ok {
		return normalized
	}
	return keys
}

func parseA11yKeyLiteral(raw string) ([]string, bool) {
	if keys, ok := parseA11yShortcutLiteral(raw); ok {
		return keys, true
	}
	if token, _, ok := normalizeA11yShortcutToken(raw); ok {
		return []string{token}, true
	}
	return nil, false
}

func parseA11yShortcutLiteral(raw string) ([]string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.Contains(raw, "+") {
		return nil, false
	}
	parts := strings.Split(raw, "+")
	keys := make([]string, 0, len(parts))
	modifierCount := 0
	primaryCount := 0
	for _, part := range parts {
		token, isModifier, ok := normalizeA11yShortcutToken(part)
		if !ok {
			return nil, false
		}
		keys = append(keys, token)
		if isModifier {
			modifierCount++
		} else {
			primaryCount++
		}
	}
	if modifierCount == 0 || primaryCount != 1 {
		return nil, false
	}
	return keys, true
}

func normalizeA11yShortcutToken(raw string) (string, bool, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false, false
	}
	normalized := strings.NewReplacer(" ", "", "_", "", "-", "").Replace(strings.ToLower(trimmed))
	switch normalized {
	case "meta", "command", "cmd", "win", "windows":
		return "cmd", true, true
	case "control", "ctrl":
		return "ctrl", true, true
	case "option", "alt":
		return "alt", true, true
	case "shift":
		return "shift", true, true
	case "function", "fn":
		return "fn", true, true
	case "enter", "return":
		return "enter", false, true
	case "tab":
		return "tab", false, true
	case "escape", "esc":
		return "escape", false, true
	case "space":
		return "space", false, true
	case "delete", "backspace":
		return normalized, false, true
	case "home", "end", "left", "right", "up", "down":
		return normalized, false, true
	case "pageup":
		return "pageup", false, true
	case "pagedown":
		return "pagedown", false, true
	case "printscreen", "prtsc":
		return "printscreen", false, true
	}
	runes := []rune(trimmed)
	if len(runes) != 1 {
		return "", false, false
	}
	if unicode.IsLetter(runes[0]) || unicode.IsDigit(runes[0]) {
		return strings.ToLower(trimmed), false, true
	}
	return "", false, false
}

func a11yTypeAliasShortcutLiteralEligible(args map[string]interface{}) bool {
	if _, ok := firstCompatValueDeep(args, "ref"); ok {
		return false
	}
	if firstCompatString(args, "target_name", "targetName", "target_role", "targetRole") != "" {
		return false
	}
	if normalizeA11yActIntent(firstCompatString(args, "intent", "scene", "scenario", "goal")) != "" {
		return false
	}
	if firstCompatString(args, "conversation", "thread", "chat", "contact") != "" {
		return false
	}
	if _, ok := compatStringSlice(args, "keys"); ok {
		return false
	}
	if _, ok := compatStringSlice(args, "submit_keys", "submitKeys"); ok {
		return false
	}
	if submit, provided := compatBoolArg(args, "submit"); provided && submit {
		return false
	}
	return true
}

func resolveA11yKeySequenceArgs(args map[string]interface{}) ([]string, bool) {
	if keys, ok := compatStringSlice(args, "keys", "key"); ok && len(keys) > 0 {
		return normalizeA11yShortcutLiteralKeys(keys), true
	}
	if submitKeys, ok := compatStringSlice(args, "submit_keys", "submitKeys"); ok && len(submitKeys) > 0 {
		return normalizeA11yShortcutLiteralKeys(submitKeys), true
	}
	if shortcutKeys, ok := parseA11yKeyLiteral(firstCompatString(args, "value", "text")); ok {
		return shortcutKeys, true
	}
	return nil, false
}
