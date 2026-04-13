package a11y

import "strings"

func windowsStateTextFromMask(mask uint32, lookup func(uint32) string) string {
	parts := make([]string, 0, 4)
	for bit := uint32(0); bit < 32; bit++ {
		flag := uint32(1) << bit
		if mask&flag == 0 {
			continue
		}
		if lookup == nil {
			continue
		}
		if text := strings.TrimSpace(lookup(flag)); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, ", ")
}

func windowsNormalizeRole(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "element"
	}
	value = strings.ReplaceAll(value, " ", "_")
	value = strings.ReplaceAll(value, "-", "_")
	return value
}
