package tools

import (
	"fmt"
	"strings"

	"github.com/go-rod/rod/lib/input"
)

func normalizeBrowserPressKey(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "enter", "return":
		return string(input.Enter), nil
	case "tab":
		return string(input.Tab), nil
	case "escape", "esc":
		return string(input.Escape), nil
	case "space":
		return string(input.Space), nil
	case "backspace":
		return string(input.Backspace), nil
	case "delete":
		return string(input.Delete), nil
	case "left":
		return string(input.ArrowLeft), nil
	case "up":
		return string(input.ArrowUp), nil
	case "right":
		return string(input.ArrowRight), nil
	case "down":
		return string(input.ArrowDown), nil
	case "home":
		return string(input.Home), nil
	case "end":
		return string(input.End), nil
	case "pageup", "page_up":
		return string(input.PageUp), nil
	case "pagedown", "page_down":
		return string(input.PageDown), nil
	case "insert":
		return string(input.Insert), nil
	case "printscreen", "print_screen", "print screen", "prtsc", "prt_sc":
		return string(input.PrintScreen), nil
	}
	runes := []rune(strings.TrimSpace(raw))
	if len(runes) == 1 {
		return string(runes[0]), nil
	}
	return "", fmt.Errorf("unsupported browser key: %s", raw)
}
