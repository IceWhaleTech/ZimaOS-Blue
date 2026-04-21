package uiexec

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a11y"
)

func TestBuildSemanticUI_FiltersLayoutAndNamelessNodes(t *testing.T) {
	sdk := &a11y.SemanticSDK{}
	raw := a11y.RawTree{
		WindowID: "w1",
		Title:    "Example",
		Mode:     "ax",
		Root: &a11y.Node{
			Role: "window",
			Name: "Example",
			Children: []*a11y.Node{
				{Role: "group", Name: "", Children: []*a11y.Node{
					{Role: "button", Name: "OK", Interactive: true, Actions: []string{"AXPress"}},
					{Role: "group", Name: "", Interactive: false},
				}},
				{Role: "layout", Name: "", Interactive: false},
			},
		},
	}

	snapshot, err := sdk.Compile(raw, a11y.CompileOptions{Mode: "ax"})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	ui := BuildSemanticUI(snapshot)

	// We should keep the OK button but drop layout-only / nameless nodes.
	if ui.Finder.FindNode(Query{Role: "button", Name: "OK"}) == nil {
		t.Fatalf("expected to find OK button")
	}
	if ui.Finder.FindNode(Query{Role: "group", NameApprox: "group"}) != nil {
		t.Fatalf("unexpected: layout/group nodes should not be exposed as semantic nodes")
	}
}
