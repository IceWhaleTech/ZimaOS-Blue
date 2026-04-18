package tools

import (
	"testing"

	a11yruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a11y"
)

func TestNormalizeA11yAction_MapsCommonDesktopAutomationAliases(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "activate", in: "activate", want: a11yruntime.ActionFocus},
		{name: "accessibility tree", in: "accessibility_tree", want: a11yruntime.ActionSnapshotInteractive},
		{name: "mouse click", in: "mouse_click", want: "click"},
		{name: "move to", in: "move_to", want: a11yruntime.ActionPointerMove},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeA11yAction(tt.in); got != tt.want {
				t.Fatalf("normalizeA11yAction(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
