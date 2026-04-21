package uiexec

import "testing"

func TestStateFromUI_BasicFields(t *testing.T) {
	ui := &SemanticUI{
		Nodes: []Node{
			{ID: "n1", Role: "input", Name: "Message", Actions: []string{"focus", "type"}, Focused: true, Visible: true, Enabled: true},
			{ID: "n2", Role: "button", Name: "Send", Actions: []string{"click"}, Visible: true, Enabled: true},
			{ID: "n3", Role: "text", Name: "Hidden", Visible: false},
		},
	}
	state := StateFromUI(ui, "Compose Page")
	if state.PageHint == "" {
		t.Fatalf("PageHint empty, want non-empty")
	}
	if state.FocusedRole != "input" {
		t.Fatalf("FocusedRole = %q, want input", state.FocusedRole)
	}
	if len(state.VisibleEntities) == 0 {
		t.Fatalf("VisibleEntities empty, want some")
	}
}
