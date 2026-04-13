package a11y

import "strings"

func windowsVirtualKey(key string) (uint16, bool) {
	switch windowsNormalizeKeyAlias(key) {
	case "ctrl", "control":
		return 0x11, true
	case "shift":
		return 0x10, true
	case "alt":
		return 0x12, true
	case "cmd", "win", "windows":
		return 0x5B, true
	case "enter", "return":
		return 0x0D, true
	case "tab":
		return 0x09, true
	case "escape", "esc":
		return 0x1B, true
	case "space":
		return 0x20, true
	case "left":
		return 0x25, true
	case "up":
		return 0x26, true
	case "right":
		return 0x27, true
	case "down":
		return 0x28, true
	case "delete":
		return 0x2E, true
	case "home":
		return 0x24, true
	case "end":
		return 0x23, true
	case "pageup":
		return 0x21, true
	case "pagedown":
		return 0x22, true
	case "printscreen", "prtsc":
		return 0x2C, true
	}
	runes := []rune(strings.TrimSpace(key))
	if len(runes) != 1 {
		return 0, false
	}
	r := runes[0]
	switch {
	case r >= 'a' && r <= 'z':
		return uint16(r - 'a' + 'A'), true
	case r >= 'A' && r <= 'Z':
		return uint16(r), true
	case r >= '0' && r <= '9':
		return uint16(r), true
	default:
		return 0, false
	}
}

func windowsIsModifierKey(key string) bool {
	switch windowsNormalizeKeyAlias(key) {
	case "ctrl", "control", "shift", "alt", "cmd", "win", "windows":
		return true
	default:
		return false
	}
}

func windowsNormalizeKeyAlias(key string) string {
	normalized := strings.TrimSpace(strings.ToLower(key))
	replacer := strings.NewReplacer(" ", "", "_", "", "-", "")
	return replacer.Replace(normalized)
}
