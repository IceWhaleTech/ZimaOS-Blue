package a11y

import "testing"

func TestWindowsVirtualKey_PrintScreenAliases(t *testing.T) {
	cases := []string{
		"printscreen",
		"print_screen",
		"print screen",
		"prtsc",
		"prt_sc",
	}

	for _, key := range cases {
		vk, ok := windowsVirtualKey(key)
		if !ok {
			t.Fatalf("windowsVirtualKey(%q) reported unsupported", key)
		}
		if vk != 0x2C {
			t.Fatalf("windowsVirtualKey(%q) = 0x%X, want 0x2C", key, vk)
		}
	}
}
