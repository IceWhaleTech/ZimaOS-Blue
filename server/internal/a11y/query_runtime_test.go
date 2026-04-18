package a11y

import "testing"

func TestResolveSnapshotTarget_PrefersComposerInputOverSearchField(t *testing.T) {
	snapshot := BuildStructuredSnapshot(BuildStructuredSnapshotOptions{
		WindowID: "win-1",
		Title:    "Feishu",
	}, &Node{
		Role: "window",
		Name: "Feishu",
		Children: []*Node{
			{Token: "token-search", Role: "search_field", Name: "Search", Interactive: true},
			{Token: "token-message", Role: "text_field", Name: "Type a message", Description: "focused editable", Interactive: true},
			{Token: "token-send", Role: "button", Name: "Send", Interactive: true},
		},
	})

	result, err := ResolveSnapshotTarget(snapshot, TargetSelector{Role: "input"})
	if err != nil {
		t.Fatalf("ResolveSnapshotTarget() error = %v", err)
	}
	if got := result.Token; got != "token-message" {
		t.Fatalf("Token = %q, want token-message", got)
	}
	if result.CandidateCount < 2 {
		t.Fatalf("CandidateCount = %d, want >= 2", result.CandidateCount)
	}
}

func TestResolveSnapshotTarget_PrefersExactNameBeforeFuzzyMatches(t *testing.T) {
	snapshot := BuildStructuredSnapshot(BuildStructuredSnapshotOptions{
		WindowID: "win-2",
		Title:    "Settings",
	}, &Node{
		Role: "window",
		Name: "Settings",
		Children: []*Node{
			{Token: "token-network", Role: "button", Name: "Open Network", Interactive: true},
			{Token: "token-network-and-internet", Role: "button", Name: "Open Network and Internet", Interactive: true},
		},
	})

	result, err := ResolveSnapshotTarget(snapshot, TargetSelector{Name: "Open Network", Role: "control"})
	if err != nil {
		t.Fatalf("ResolveSnapshotTarget() error = %v", err)
	}
	if got := result.Token; got != "token-network" {
		t.Fatalf("Token = %q, want token-network", got)
	}
}

func TestResolveSnapshotTarget_FallsBackToLabelAnchorForSetting(t *testing.T) {
	snapshot := BuildStructuredSnapshot(BuildStructuredSnapshotOptions{
		WindowID: "win-3",
		Title:    "Settings",
	}, &Node{
		Role: "window",
		Name: "Settings",
		Children: []*Node{
			{Role: "text", Name: "Notifications"},
			{Token: "token-toggle", Role: "switch", Name: "", Interactive: true},
		},
	})

	result, err := ResolveSnapshotTarget(snapshot, TargetSelector{Name: "Notifications", Role: "setting"})
	if err != nil {
		t.Fatalf("ResolveSnapshotTarget() error = %v", err)
	}
	if got := result.Token; got != "token-toggle" {
		t.Fatalf("Token = %q, want token-toggle", got)
	}
	if len(result.Fallbacks) == 0 {
		t.Fatal("Fallbacks = nil, want label anchor fallback marker")
	}
}

func TestResolveSnapshotTarget_ConversationRoleIncludesStableIDAndBounds(t *testing.T) {
	snapshot := BuildStructuredSnapshot(BuildStructuredSnapshotOptions{
		WindowID: "win-4",
		Title:    "Feishu",
	}, &Node{
		Role: "window",
		Name: "Feishu",
		Children: []*Node{
			{
				Token:       "token-orca",
				Role:        "list_item",
				Name:        "Orca Team",
				Interactive: true,
				Bounds:      NormalizedRect{X: 0.08, Y: 0.14, Width: 0.30, Height: 0.09},
			},
			{
				Token:       "token-send",
				Role:        "button",
				Name:        "Send",
				Interactive: true,
			},
		},
	})

	result, err := ResolveSnapshotTarget(snapshot, TargetSelector{Name: "Orca Team", Role: "conversation"})
	if err != nil {
		t.Fatalf("ResolveSnapshotTarget() error = %v", err)
	}
	if got := result.Token; got != "token-orca" {
		t.Fatalf("Token = %q, want token-orca", got)
	}
	if got := result.Role; got != "list_item" {
		t.Fatalf("Role = %q, want list_item", got)
	}
	if got := result.Label; got != "Orca Team" {
		t.Fatalf("Label = %q, want Orca Team", got)
	}
	if got := result.Bounds; got != (NormalizedRect{X: 0.08, Y: 0.14, Width: 0.30, Height: 0.09}) {
		t.Fatalf("Bounds = %#v, want structured bounds", got)
	}
	if result.StableID == "" {
		t.Fatal("StableID = empty, want stable id")
	}
}
