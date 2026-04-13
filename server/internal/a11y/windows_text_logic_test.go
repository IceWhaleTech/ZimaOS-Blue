package a11y

import "testing"

func TestWindowsNormalizeRole_NormalizesSeparatorsAndFallback(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: "element"},
		{name: "trim and lower", input: "  Push Button  ", want: "push_button"},
		{name: "hyphen", input: "menu-item", want: "menu_item"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := windowsNormalizeRole(tc.input); got != tc.want {
				t.Fatalf("windowsNormalizeRole(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestWindowsStateTextFromMask_JoinsRecognizedFlagsInBitOrder(t *testing.T) {
	var lookedUp []uint32

	got := windowsStateTextFromMask((1<<0)|(1<<2)|(1<<5), func(flag uint32) string {
		lookedUp = append(lookedUp, flag)
		switch flag {
		case 1 << 0:
			return "focusable"
		case 1 << 2:
			return ""
		case 1 << 5:
			return "expanded"
		default:
			return ""
		}
	})
	if got != "focusable, expanded" {
		t.Fatalf("windowsStateTextFromMask() = %q, want %q", got, "focusable, expanded")
	}
	if len(lookedUp) != 3 || lookedUp[0] != 1<<0 || lookedUp[1] != 1<<2 || lookedUp[2] != 1<<5 {
		t.Fatalf("lookedUp = %#v, want []uint32{1,4,32}", lookedUp)
	}
}

func TestWindowsStateTextFromMask_ReturnsEmptyWhenNoFlagsResolve(t *testing.T) {
	got := windowsStateTextFromMask(0, func(flag uint32) string {
		t.Fatalf("lookup should not run, got flag %d", flag)
		return ""
	})
	if got != "" {
		t.Fatalf("windowsStateTextFromMask() = %q, want empty", got)
	}
}
