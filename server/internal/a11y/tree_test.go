package a11y

import (
	"fmt"
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

func TestBuildSnapshotTree_InteractiveOnlyCompactsLargeTreesToPriorityNodes(t *testing.T) {
	root := &Node{
		Role: "window",
		Name: "Feishu",
	}
	for idx := 0; idx < 80; idx++ {
		root.Children = append(root.Children, &Node{
			Token:       fmt.Sprintf("token-conversation-%02d", idx),
			Role:        "list_item",
			Name:        fmt.Sprintf("Conversation %02d", idx),
			Interactive: true,
		})
	}
	root.Children = append(root.Children,
		&Node{Token: "token-message", Role: "text_field", Name: "Message", Interactive: true},
		&Node{Token: "token-send", Role: "button", Name: "Send", Interactive: true},
		&Node{Token: "token-search", Role: "search_field", Name: "Search", Interactive: true},
	)

	tree, refMap := BuildSnapshotTree(root, true)
	if len(refMap) > interactiveSnapshotMaxRefs {
		t.Fatalf("len(refMap) = %d, want <= %d", len(refMap), interactiveSnapshotMaxRefs)
	}
	for _, expected := range []string{`[text_field] "Message"`, `[button] "Send"`, `[search_field] "Search"`} {
		if !strings.Contains(tree, expected) {
			t.Fatalf("tree = %q, want %s", tree, expected)
		}
	}
	if strings.Index(tree, `[text_field] "Message"`) > strings.Index(tree, `[list_item] "Conversation 00"`) {
		t.Fatalf("tree = %q, want key input before generic conversation items", tree)
	}
	if strings.Contains(tree, `Conversation 79`) {
		t.Fatalf("tree = %q, did not expect lowest-priority overflow item", tree)
	}
	for _, token := range []string{"token-message", "token-send", "token-search"} {
		found := false
		for _, value := range refMap {
			if value == token {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("refMap = %#v, want token %q", refMap, token)
		}
	}
}

func TestBuildSnapshotTree_InteractiveOnlyCompactsLargeTreesDeprioritizesWindowChrome(t *testing.T) {
	root := &Node{
		Role: "window",
		Name: "Feishu",
	}
	root.Children = append(root.Children,
		&Node{Token: "token-close", Role: "button", Name: "Close", Interactive: true},
		&Node{Token: "token-minimize", Role: "button", Name: "Minimize", Interactive: true},
		&Node{Token: "token-maximize", Role: "button", Name: "Maximize", Interactive: true},
	)
	for idx := 0; idx < 80; idx++ {
		root.Children = append(root.Children, &Node{
			Token:       fmt.Sprintf("token-conversation-%02d", idx),
			Role:        "list_item",
			Name:        fmt.Sprintf("Conversation %02d", idx),
			Interactive: true,
		})
	}
	root.Children = append(root.Children,
		&Node{Token: "token-message", Role: "text_field", Name: "Type a message", Interactive: true, Description: "focused editable"},
		&Node{Token: "token-send", Role: "button", Name: "Send", Interactive: true},
	)

	tree, refMap := BuildSnapshotTree(root, true)
	if strings.Contains(tree, `[button] "Close"`) {
		t.Fatalf("tree = %q, did not expect Close in compact priority snapshot", tree)
	}
	if strings.Contains(tree, `[button] "Minimize"`) {
		t.Fatalf("tree = %q, did not expect Minimize in compact priority snapshot", tree)
	}
	if strings.Contains(tree, `[button] "Maximize"`) {
		t.Fatalf("tree = %q, did not expect Maximize in compact priority snapshot", tree)
	}
	for _, token := range []string{"token-message", "token-send"} {
		found := false
		for _, value := range refMap {
			if value == token {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("refMap = %#v, want token %q", refMap, token)
		}
	}
}

func TestBuildSnapshotTree_InteractiveOnlyCompactsLargeTreesDeprioritizesShellNavigation(t *testing.T) {
	root := &Node{
		Role: "window",
		Name: "Feishu",
	}
	root.Children = append(root.Children,
		&Node{Token: "token-back", Role: "button", Name: "Back", Interactive: true},
		&Node{Token: "token-forward", Role: "button", Name: "Forward", Interactive: true},
		&Node{Token: "token-reload", Role: "button", Name: "Reload", Interactive: true},
		&Node{Token: "token-settings", Role: "button", Name: "Settings", Interactive: true},
	)
	for idx := 0; idx < 80; idx++ {
		root.Children = append(root.Children, &Node{
			Token:       fmt.Sprintf("token-workspace-%02d", idx),
			Role:        "list_item",
			Name:        fmt.Sprintf("Workspace %02d", idx),
			Interactive: true,
		})
	}
	root.Children = append(root.Children,
		&Node{Token: "token-message", Role: "text_field", Name: "Type a message", Interactive: true, Description: "focused editable"},
		&Node{Token: "token-send", Role: "button", Name: "Send", Interactive: true},
	)

	tree, refMap := BuildSnapshotTree(root, true)
	for _, unexpected := range []string{`[button] "Back"`, `[button] "Forward"`, `[button] "Reload"`, `[button] "Settings"`} {
		if strings.Contains(tree, unexpected) {
			t.Fatalf("tree = %q, did not expect shell navigation item %s in compact priority snapshot", tree, unexpected)
		}
	}
	for _, token := range []string{"token-message", "token-send"} {
		found := false
		for _, value := range refMap {
			if value == token {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("refMap = %#v, want token %q", refMap, token)
		}
	}
}

func TestInteractiveSnapshotPriority_DeprioritizesWindowChromeBelowContentItems(t *testing.T) {
	closeButton := &Node{Role: "button", Name: "Close", Interactive: true}
	workspaceItem := &Node{Role: "list_item", Name: "Workspace 01", Interactive: true}
	messageField := &Node{Role: "text_field", Name: "Type a message", Interactive: true, Description: "focused editable"}

	if gotClose, gotWorkspace := interactiveSnapshotPriority(closeButton), interactiveSnapshotPriority(workspaceItem); gotClose >= gotWorkspace {
		t.Fatalf("close priority = %d, workspace priority = %d, want close lower than workspace content", gotClose, gotWorkspace)
	}
	if gotMessage, gotWorkspace := interactiveSnapshotPriority(messageField), interactiveSnapshotPriority(workspaceItem); gotMessage <= gotWorkspace {
		t.Fatalf("message priority = %d, workspace priority = %d, want message higher than workspace", gotMessage, gotWorkspace)
	}
}

func TestInteractiveSnapshotPriority_DeprioritizesShellNavigationBelowContentItems(t *testing.T) {
	backButton := &Node{Role: "button", Name: "Back", Interactive: true}
	addressBar := &Node{Role: "search_field", Name: "Address and search bar", Interactive: true}
	workspaceItem := &Node{Role: "list_item", Name: "Workspace 01", Interactive: true}
	messageField := &Node{Role: "text_field", Name: "Type a message", Interactive: true, Description: "focused editable"}

	if gotBack, gotWorkspace := interactiveSnapshotPriority(backButton), interactiveSnapshotPriority(workspaceItem); gotBack >= gotWorkspace {
		t.Fatalf("back priority = %d, workspace priority = %d, want shell navigation lower than workspace content", gotBack, gotWorkspace)
	}
	if gotAddressBar, gotWorkspace := interactiveSnapshotPriority(addressBar), interactiveSnapshotPriority(workspaceItem); gotAddressBar >= gotWorkspace {
		t.Fatalf("address bar priority = %d, workspace priority = %d, want browser chrome field lower than workspace content", gotAddressBar, gotWorkspace)
	}
	if gotMessage, gotBack := interactiveSnapshotPriority(messageField), interactiveSnapshotPriority(backButton); gotMessage <= gotBack {
		t.Fatalf("message priority = %d, back priority = %d, want message higher than shell navigation", gotMessage, gotBack)
	}
}
