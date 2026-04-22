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

func TestNormalizeA11yAction_MapsFuzzyIntentPhrases(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "send a message phrase", in: "send a message", want: "message"},
		{name: "interactive controls phrase", in: "show interactive controls", want: a11yruntime.ActionSnapshotInteractive},
		{name: "take screenshot phrase", in: "take a screenshot", want: a11yruntime.ActionScreenshot},
		{name: "switch conversation phrase", in: "switch to conversation", want: "select"},
		{name: "turn on setting phrase", in: "turn on setting", want: "toggle"},
		{name: "chinese send message phrase", in: "发送消息", want: "message"},
		{name: "chinese interactive controls phrase", in: "查看交互控件", want: a11yruntime.ActionSnapshotInteractive},
		{name: "chinese screenshot phrase", in: "截个图", want: a11yruntime.ActionScreenshot},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeA11yAction(tt.in); got != tt.want {
				t.Fatalf("normalizeA11yAction(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
