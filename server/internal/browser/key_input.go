package browser

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
)

type browserKeyTarget interface {
	Press(key input.Key) error
	Release(key input.Key) error
	InsertText(text string) error
}

type rodBrowserKeyTarget struct {
	page *rod.Page
}

func (t rodBrowserKeyTarget) Press(key input.Key) error {
	return t.page.Keyboard.Press(key)
}

func (t rodBrowserKeyTarget) Release(key input.Key) error {
	return t.page.Keyboard.Release(key)
}

func (t rodBrowserKeyTarget) InsertText(text string) error {
	return t.page.InsertText(text)
}

var browserKeySleep = time.Sleep

func (s *RodService) PressKeys(ctx context.Context, targetID string, keys []string, holdMS int) error {
	tab, err := s.getTab(ctx, targetID)
	if err != nil {
		return err
	}
	return safeRodPageCall("press keys", func() error {
		return executeBrowserKeyInput(rodBrowserKeyTarget{page: tab.page}, keys, holdMS)
	})
}

func executeBrowserKeyInput(target browserKeyTarget, rawKeys []string, holdMS int) error {
	steps, err := planBrowserKeySteps(rawKeys)
	if err != nil {
		return err
	}
	for _, step := range steps {
		if err := executeBrowserKeyStep(target, step, holdMS); err != nil {
			return err
		}
	}
	return nil
}

func planBrowserKeySteps(rawKeys []string) ([][]string, error) {
	cleaned := make([]string, 0, len(rawKeys))
	parsed := make([][]string, 0, len(rawKeys))
	hasChordEntry := false
	nonModifierCount := 0

	for _, raw := range rawKeys {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		parts, err := splitBrowserKeyChord(trimmed)
		if err != nil {
			return nil, err
		}
		if len(parts) > 1 {
			hasChordEntry = true
		}
		if len(parts) == 1 && !browserIsModifierToken(parts[0]) {
			nonModifierCount++
		}
		cleaned = append(cleaned, trimmed)
		parsed = append(parsed, parts)
	}

	if len(cleaned) == 0 {
		return nil, fmt.Errorf("unsupported browser key: empty input")
	}
	if len(parsed) == 1 {
		return parsed, nil
	}
	if !hasChordEntry && nonModifierCount <= 1 {
		chord := make([]string, 0, len(parsed))
		for _, part := range parsed {
			chord = append(chord, part[0])
		}
		return [][]string{chord}, nil
	}
	return parsed, nil
}

func splitBrowserKeyChord(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("unsupported browser key: empty input")
	}
	if !strings.Contains(raw, "+") || raw == "+" {
		return []string{raw}, nil
	}
	parts := make([]string, 0, 4)
	var current strings.Builder
	for _, r := range raw {
		if r == '+' {
			if current.Len() == 0 {
				parts = append(parts, "+")
				continue
			}
			part := strings.TrimSpace(current.String())
			if part == "" {
				return nil, fmt.Errorf("unsupported browser key chord: %s", raw)
			}
			parts = append(parts, part)
			current.Reset()
			continue
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 {
		part := strings.TrimSpace(current.String())
		if part == "" {
			return nil, fmt.Errorf("unsupported browser key chord: %s", raw)
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("unsupported browser key chord: %s", raw)
	}
	return parts, nil
}

func executeBrowserKeyStep(target browserKeyTarget, step []string, holdMS int) error {
	modifiers := make([]input.Key, 0, len(step))
	var primary input.Key
	primarySet := false
	primaryText := ""

	for _, token := range step {
		key, ok := browserInputKey(token)
		if ok {
			if browserInputKeyIsModifier(key) {
				modifiers = append(modifiers, key)
				continue
			}
			if primarySet || primaryText != "" {
				return fmt.Errorf("unsupported browser key chord: %s", strings.Join(step, "+"))
			}
			primary = key
			primarySet = true
			continue
		}
		if len(step) > 1 {
			return fmt.Errorf("unsupported browser key chord: %s", strings.Join(step, "+"))
		}
		primaryText = token
	}

	for _, modifier := range modifiers {
		if err := target.Press(modifier); err != nil {
			return err
		}
	}

	releaseModifiers := func() error {
		for idx := len(modifiers) - 1; idx >= 0; idx-- {
			if err := target.Release(modifiers[idx]); err != nil {
				return err
			}
		}
		return nil
	}

	if !primarySet && primaryText != "" {
		if len(modifiers) != 0 {
			if err := releaseModifiers(); err != nil {
				return err
			}
			return fmt.Errorf("unsupported browser key chord: %s", strings.Join(step, "+"))
		}
		return target.InsertText(primaryText)
	}

	if primarySet {
		if err := target.Press(primary); err != nil {
			_ = releaseModifiers()
			return err
		}
		if holdMS > 0 {
			browserKeySleep(time.Duration(holdMS) * time.Millisecond)
		}
		if err := target.Release(primary); err != nil {
			_ = releaseModifiers()
			return err
		}
	} else if holdMS > 0 {
		browserKeySleep(time.Duration(holdMS) * time.Millisecond)
	}

	return releaseModifiers()
}

func browserInputKey(token string) (input.Key, bool) {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return 0, false
	}
	runes := []rune(trimmed)
	if len(runes) == 1 {
		r := runes[0]
		if unicode.IsLetter(r) {
			r = unicode.ToLower(r)
		}
		return input.Key(r), true
	}
	switch browserNormalizeKeyToken(trimmed) {
	case "ctrl", "control":
		return input.ControlLeft, true
	case "shift":
		return input.ShiftLeft, true
	case "alt", "option":
		return input.AltLeft, true
	case "cmd", "command", "meta", "win", "windows":
		return input.MetaLeft, true
	case "enter", "return":
		return input.Enter, true
	case "tab":
		return input.Tab, true
	case "escape", "esc":
		return input.Escape, true
	case "space":
		return input.Space, true
	case "backspace":
		return input.Backspace, true
	case "delete":
		return input.Delete, true
	case "left":
		return input.ArrowLeft, true
	case "up":
		return input.ArrowUp, true
	case "right":
		return input.ArrowRight, true
	case "down":
		return input.ArrowDown, true
	case "home":
		return input.Home, true
	case "end":
		return input.End, true
	case "pageup":
		return input.PageUp, true
	case "pagedown":
		return input.PageDown, true
	case "insert":
		return input.Insert, true
	case "printscreen", "prtsc":
		return input.PrintScreen, true
	case "f1":
		return input.F1, true
	case "f2":
		return input.F2, true
	case "f3":
		return input.F3, true
	case "f4":
		return input.F4, true
	case "f5":
		return input.F5, true
	case "f6":
		return input.F6, true
	case "f7":
		return input.F7, true
	case "f8":
		return input.F8, true
	case "f9":
		return input.F9, true
	case "f10":
		return input.F10, true
	case "f11":
		return input.F11, true
	case "f12":
		return input.F12, true
	default:
		return 0, false
	}
}

func browserIsModifierToken(token string) bool {
	key, ok := browserInputKey(token)
	return ok && browserInputKeyIsModifier(key)
}

func browserInputKeyIsModifier(key input.Key) bool {
	return key.Modifier() != 0
}

func browserNormalizeKeyToken(token string) string {
	replacer := strings.NewReplacer(" ", "", "_", "", "-", "")
	return replacer.Replace(strings.ToLower(strings.TrimSpace(token)))
}
