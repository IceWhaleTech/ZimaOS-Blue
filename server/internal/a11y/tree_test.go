package a11y

import (
	"strings"
	"testing"
)

func TestBuildSnapshotTree_AssignsRefsToInteractiveNodes(t *testing.T) {
	root := &Node{
		Role: "window",
		Name: "Preferences",
		Children: []*Node{
			{
				Token:       "token-button",
				Role:        "button",
				Name:        "Continue",
				Interactive: true,
			},
			{
				Token: "token-field",
				Role:  "text_field",
				Value: "search term",
			},
		},
	}

	tree, refMap := BuildSnapshotTree(root, false)
	if !strings.Contains(tree, `@1 [button] "Continue"`) {
		t.Fatalf("tree = %q, want button ref", tree)
	}
	if !strings.Contains(tree, `@2 [text_field] "search term"`) {
		t.Fatalf("tree = %q, want value fallback label", tree)
	}
	if got := refMap[1]; got != "token-button" {
		t.Fatalf("refMap[1] = %q, want token-button", got)
	}
	if got := refMap[2]; got != "token-field" {
		t.Fatalf("refMap[2] = %q, want token-field", got)
	}
}

func TestBuildSnapshotTree_InteractiveOnlySkipsPlainContainers(t *testing.T) {
	root := &Node{
		Role: "window",
		Name: "Main",
		Children: []*Node{
			{
				Role: "group",
				Name: "Container",
				Children: []*Node{
					{
						Token:       "token-link",
						Role:        "link",
						Name:        "Open",
						Interactive: true,
					},
				},
			},
		},
	}

	tree, refMap := BuildSnapshotTree(root, true)
	if strings.Contains(tree, `[group] "Container"`) {
		t.Fatalf("tree = %q, did not expect non-interactive container line", tree)
	}
	if !strings.Contains(tree, `@1 [link] "Open"`) {
		t.Fatalf("tree = %q, want interactive descendant", tree)
	}
	if len(refMap) != 1 || refMap[1] != "token-link" {
		t.Fatalf("refMap = %#v, want only token-link", refMap)
	}
}
