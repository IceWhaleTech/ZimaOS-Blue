package server

import (
	"strings"
	"testing"
)

func TestPromptPolicyToolGuidanceConstraints_IncludeComputerUseMessageFlowGuardrails(t *testing.T) {
	got := buildToolGuidanceConstraints()

	required := []string{
		"desktop chat or messaging task",
		"`computer_use`",
		"`message`, `select`, and `type`",
		"`activate`, `mouse_click`, `move_to`, `accessibility_tree`, or `ocr`",
		"`screenshot` only for evidence capture or last-resort debugging",
	}
	for _, want := range required {
		if !strings.Contains(got, want) {
			t.Fatalf("expected tool guidance constraints to include %q, got=%q", want, got)
		}
	}
}

func TestPromptPolicyAutoContinueNudges_CarryComputerUseMessageFlowGuardrails(t *testing.T) {
	toolless := buildToollessAutoContinueNudge(false)
	postTool := buildPostToolAutoContinueNudge(true)

	for _, got := range []string{toolless, postTool} {
		if !strings.Contains(got, "desktop chat or messaging task") {
			t.Fatalf("expected nudge to include desktop chat guardrails, got=%q", got)
		}
		if !strings.Contains(got, "`message`, `select`, and `type`") {
			t.Fatalf("expected nudge to prefer high-level computer_use actions, got=%q", got)
		}
	}
}
