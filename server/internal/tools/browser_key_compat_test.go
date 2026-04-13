package tools

import "testing"

func TestNormalizeBrowserPressKeyAliases(t *testing.T) {
	cases := []string{
		"enter",
		"return",
		"tab",
		"escape",
		"space",
		"backspace",
		"delete",
		"left",
		"up",
		"right",
		"down",
		"printscreen",
		"prtsc",
		"a",
		"7",
	}

	for _, key := range cases {
		if normalized, err := normalizeBrowserPressKey(key); err != nil || normalized == "" {
			t.Fatalf("normalizeBrowserPressKey(%q) = %q, %v; want non-empty normalized key", key, normalized, err)
		}
	}
}
