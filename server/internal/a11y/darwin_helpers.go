//go:build darwin

package a11y

import "strings"

const (
	darwinEventFlagShift     uint64 = 1 << 17
	darwinEventFlagControl   uint64 = 1 << 18
	darwinEventFlagAlternate uint64 = 1 << 19
	darwinEventFlagCommand   uint64 = 1 << 20
)

var darwinKeyCodeMap = map[string]uint16{
	"a":         0x00,
	"s":         0x01,
	"d":         0x02,
	"f":         0x03,
	"h":         0x04,
	"g":         0x05,
	"z":         0x06,
	"x":         0x07,
	"c":         0x08,
	"v":         0x09,
	"b":         0x0B,
	"q":         0x0C,
	"w":         0x0D,
	"e":         0x0E,
	"r":         0x0F,
	"y":         0x10,
	"t":         0x11,
	"1":         0x12,
	"2":         0x13,
	"3":         0x14,
	"4":         0x15,
	"6":         0x16,
	"5":         0x17,
	"=":         0x18,
	"9":         0x19,
	"7":         0x1A,
	"-":         0x1B,
	"8":         0x1C,
	"0":         0x1D,
	"]":         0x1E,
	"o":         0x1F,
	"u":         0x20,
	"[":         0x21,
	"i":         0x22,
	"p":         0x23,
	"return":    0x24,
	"enter":     0x4C,
	"l":         0x25,
	"j":         0x26,
	"'":         0x27,
	"k":         0x28,
	";":         0x29,
	"\\":        0x2A,
	",":         0x2B,
	"/":         0x2C,
	"n":         0x2D,
	"m":         0x2E,
	".":         0x2F,
	"tab":       0x30,
	"space":     0x31,
	"`":         0x32,
	"delete":    0x33,
	"backspace": 0x33,
	"escape":    0x35,
	"esc":       0x35,
	"command":   0x37,
	"cmd":       0x37,
	"shift":     0x38,
	"option":    0x3A,
	"alt":       0x3A,
	"control":   0x3B,
	"ctrl":      0x3B,
	"function":  0x3F,
	"fn":        0x3F,
	"pageup":    0x74,
	"page_up":   0x74,
	"pagedown":  0x79,
	"page_down": 0x79,
	"home":      0x73,
	"end":       0x77,
	"left":      0x7B,
	"right":     0x7C,
	"down":      0x7D,
	"up":        0x7E,
}

func normalizeDarwinRole(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "element"
	}
	value = strings.TrimPrefix(value, "AX")
	value = strings.TrimPrefix(value, "ax")
	if value == "" {
		return "element"
	}
	var out []rune
	for idx, ch := range value {
		if idx > 0 && ch >= 'A' && ch <= 'Z' {
			out = append(out, '_')
		}
		if ch >= 'A' && ch <= 'Z' {
			ch = ch - 'A' + 'a'
		}
		out = append(out, ch)
	}
	return strings.ToLower(string(out))
}

func darwinKeyCodeForName(raw string) (uint16, bool) {
	code, ok := darwinKeyCodeMap[strings.TrimSpace(strings.ToLower(raw))]
	return code, ok
}

func darwinModifierFlags(keys []string) uint64 {
	var flags uint64
	for _, key := range keys {
		switch strings.TrimSpace(strings.ToLower(key)) {
		case "command", "cmd":
			flags |= darwinEventFlagCommand
		case "shift":
			flags |= darwinEventFlagShift
		case "option", "alt":
			flags |= darwinEventFlagAlternate
		case "control", "ctrl":
			flags |= darwinEventFlagControl
		}
	}
	return flags
}

func darwinIsModifierKey(raw string) bool {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "command", "cmd", "shift", "option", "alt", "control", "ctrl", "fn", "function":
		return true
	default:
		return false
	}
}

func darwinIncludeWindowRecord(record darwinWindowRecord) bool {
	if strings.TrimSpace(record.ID) == "" || record.PID <= 0 {
		return false
	}
	if record.Layer != 0 {
		return false
	}
	if record.Bounds.Size.Width <= 1 || record.Bounds.Size.Height <= 1 {
		return false
	}
	switch strings.TrimSpace(strings.ToLower(record.AppName)) {
	case "window server", "controlcenter", "控制中心", "dock", "程序坞", "notificationcenter", "通知中心", "systemuiserver", "wallpaperagent", "loginwindow", "universalaccessauthwarn":
		return false
	default:
		return true
	}
}

func darwinMarkFocusedRecord(records []darwinWindowRecord, frontPID int, frontTitle string, frontBounds darwinRect) {
	if len(records) == 0 {
		return
	}
	for idx := range records {
		records[idx].Focused = false
	}
	if frontPID > 0 {
		for idx := range records {
			if records[idx].PID != frontPID {
				continue
			}
			titleMatch := strings.TrimSpace(frontTitle) == "" || strings.EqualFold(strings.TrimSpace(records[idx].Title), strings.TrimSpace(frontTitle))
			boundsMatch := darwinRectDefined(frontBounds) && darwinRectsClose(records[idx].Bounds, frontBounds)
			if titleMatch || boundsMatch {
				records[idx].Focused = true
				return
			}
		}
	}
	records[0].Focused = true
}
