//go:build windows
// +build windows

package workspace

import "testing"

func TestFirstWindowsMultiString(t *testing.T) {
	raw := []uint16{'z', 'h', '-', 'C', 'N', 0, 'e', 'n', '-', 'U', 'S', 0, 0}
	if got := firstWindowsMultiString(raw); got != "zh-CN" {
		t.Fatalf("firstWindowsMultiString() = %q, want %q", got, "zh-CN")
	}
}

func TestNormalizeWindowsLocale(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "already normalized", input: "en-US", want: "en-US"},
		{name: "underscores", input: "zh_CN", want: "zh-CN"},
		{name: "trim spaces", input: "  ja-JP  ", want: "ja-JP"},
		{name: "empty", input: "   ", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeWindowsLocale(tt.input); got != tt.want {
				t.Fatalf("normalizeWindowsLocale(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestWindowsLocaleProbeOrder(t *testing.T) {
	origUserPreferred := readUserPreferredUILanguage
	origUserDefault := readUserDefaultLocaleName
	origSystemDefault := readSystemDefaultLocaleName
	t.Cleanup(func() {
		readUserPreferredUILanguage = origUserPreferred
		readUserDefaultLocaleName = origUserDefault
		readSystemDefaultLocaleName = origSystemDefault
	})

	callOrder := make([]string, 0, 3)
	readUserPreferredUILanguage = func() string {
		callOrder = append(callOrder, "userPreferred")
		return ""
	}
	readUserDefaultLocaleName = func() string {
		callOrder = append(callOrder, "userDefault")
		return "zh_CN"
	}
	readSystemDefaultLocaleName = func() string {
		callOrder = append(callOrder, "systemDefault")
		return "en_US"
	}

	if got := windowsLocale(); got != "zh-CN" {
		t.Fatalf("windowsLocale() = %q, want %q", got, "zh-CN")
	}
	if len(callOrder) != 2 {
		t.Fatalf("probe order length = %d, want 2", len(callOrder))
	}
	if callOrder[0] != "userPreferred" || callOrder[1] != "userDefault" {
		t.Fatalf("probe order = %v, want [userPreferred userDefault]", callOrder)
	}
}

func TestWindowsLocaleFallsBackToSystemDefault(t *testing.T) {
	origUserPreferred := readUserPreferredUILanguage
	origUserDefault := readUserDefaultLocaleName
	origSystemDefault := readSystemDefaultLocaleName
	t.Cleanup(func() {
		readUserPreferredUILanguage = origUserPreferred
		readUserDefaultLocaleName = origUserDefault
		readSystemDefaultLocaleName = origSystemDefault
	})

	readUserPreferredUILanguage = func() string { return "" }
	readUserDefaultLocaleName = func() string { return "" }
	readSystemDefaultLocaleName = func() string { return "de_DE" }

	if got := windowsLocale(); got != "de-DE" {
		t.Fatalf("windowsLocale() = %q, want %q", got, "de-DE")
	}
}
