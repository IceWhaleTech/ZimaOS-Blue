package a11y

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildStructuredSnapshot_ProjectsFullAndInteractiveTrees(t *testing.T) {
	root := &Node{
		Role: "window",
		Name: "Feishu",
		Children: []*Node{
			{
				Token:       "token-search",
				Role:        "search_field",
				Name:        "Search",
				Interactive: true,
			},
			{
				Token:       "token-message",
				Role:        "text_field",
				Name:        "Type a message",
				Description: "focused editable",
				Interactive: true,
			},
			{
				Token:       "token-send",
				Role:        "button",
				Name:        "Send",
				Interactive: true,
			},
		},
	}

	snapshot := BuildStructuredSnapshot(BuildStructuredSnapshotOptions{
		WindowID: "win-1",
		Title:    "Feishu",
	}, root)
	if snapshot == nil {
		t.Fatal("BuildStructuredSnapshot() = nil")
	}
	if snapshot.WindowID != "win-1" {
		t.Fatalf("WindowID = %q, want win-1", snapshot.WindowID)
	}
	if len(snapshot.Nodes) != 4 {
		t.Fatalf("len(Nodes) = %d, want 4", len(snapshot.Nodes))
	}
	if got := snapshot.NameIndex["type_a_message"]; len(got) != 1 {
		t.Fatalf("NameIndex[type_a_message] = %#v, want single node id", got)
	}
	if got := snapshot.RoleIndex["text_field"]; len(got) != 1 {
		t.Fatalf("RoleIndex[text_field] = %#v, want single node id", got)
	}

	full := snapshot.Projection(SnapshotProjectionFull)
	if full.Tree == "" {
		t.Fatal("full projection tree is empty")
	}
	if got := full.RefMap[2]; got != "token-message" {
		t.Fatalf("full RefMap[2] = %q, want token-message", got)
	}

	interactive := snapshot.Projection(SnapshotProjectionInteractive)
	if interactive.Tree == "" {
		t.Fatal("interactive projection tree is empty")
	}
	if got := interactive.NodeToRef[snapshot.LookupToken("token-message")]; got == 0 {
		t.Fatalf("interactive NodeToRef missing token-message node: %#v", interactive.NodeToRef)
	}
}

func TestBuildStructuredSnapshot_ProjectsNodeBoundsWhenPresent(t *testing.T) {
	root := &Node{
		Role: "window",
		Name: "Feishu",
		Children: []*Node{
			{
				Token:       "token-orca",
				Role:        "list_item",
				Name:        "Orca",
				Interactive: true,
			},
		},
	}
	setNodeBoundsForTest(t, root, NormalizedRect{X: 0, Y: 0, Width: 1, Height: 1})
	setNodeBoundsForTest(t, root.Children[0], NormalizedRect{X: 0.10, Y: 0.20, Width: 0.30, Height: 0.10})

	snapshot := BuildStructuredSnapshot(BuildStructuredSnapshotOptions{
		WindowID: "win-1",
		Title:    "Feishu",
	}, root)
	if snapshot == nil {
		t.Fatal("BuildStructuredSnapshot() = nil")
	}
	nodeID := snapshot.LookupToken("token-orca")
	if nodeID == 0 {
		t.Fatal("LookupToken(token-orca) = 0, want node id")
	}
	node, ok := snapshotNodeByID(snapshot, nodeID)
	if !ok {
		t.Fatalf("snapshotNodeByID(%d) = missing, want node", nodeID)
	}
	want := (NormalizedRect{X: 0.10, Y: 0.20, Width: 0.30, Height: 0.10})
	if node.Bounds != want {
		t.Fatalf("node.Bounds = %#v, want %#v", node.Bounds, want)
	}
}

func TestBuildStructuredSnapshot_AssignsStableIDIndependentOfFocusState(t *testing.T) {
	buildSnapshot := func(description string) *Snapshot {
		root := &Node{
			Role: "window",
			Name: "Feishu",
			Children: []*Node{
				{
					Token:       "token-message",
					Role:        "text_field",
					Name:        "Type a message",
					Description: description,
					Interactive: true,
				},
			},
		}
		setNodeBoundsForTest(t, root, NormalizedRect{X: 0, Y: 0, Width: 1, Height: 1})
		setNodeBoundsForTest(t, root.Children[0], NormalizedRect{X: 0.10, Y: 0.70, Width: 0.80, Height: 0.12})
		return BuildStructuredSnapshot(BuildStructuredSnapshotOptions{
			WindowID: "win-1",
			Title:    "Feishu",
		}, root)
	}

	unfocused := buildSnapshot("")
	focused := buildSnapshot("focused editable")
	if unfocused == nil || focused == nil {
		t.Fatal("BuildStructuredSnapshot() = nil, want snapshots")
	}

	unfocusedID := unfocused.LookupToken("token-message")
	focusedID := focused.LookupToken("token-message")
	if unfocusedID == 0 || focusedID == 0 {
		t.Fatalf("LookupToken(token-message) = (%d, %d), want non-zero ids", unfocusedID, focusedID)
	}

	unfocusedNode, ok := snapshotNodeByID(unfocused, unfocusedID)
	if !ok {
		t.Fatalf("snapshotNodeByID(%d) missing in unfocused snapshot", unfocusedID)
	}
	focusedNode, ok := snapshotNodeByID(focused, focusedID)
	if !ok {
		t.Fatalf("snapshotNodeByID(%d) missing in focused snapshot", focusedID)
	}
	if strings.TrimSpace(unfocusedNode.StableID) == "" {
		t.Fatal("unfocusedNode.StableID = empty, want stable id")
	}
	if unfocusedNode.StableID != focusedNode.StableID {
		t.Fatalf("StableID changed across focus-state drift: %q vs %q", unfocusedNode.StableID, focusedNode.StableID)
	}
}

func TestBuildStructuredSnapshot_AssignsStableIDIndependentOfComposerNameDrift(t *testing.T) {
	buildSnapshot := func(name string) *Snapshot {
		root := &Node{
			Role: "window",
			Name: "Feishu",
			Children: []*Node{
				{
					Token:       "token-message",
					Role:        "document",
					Name:        name,
					Description: "focused editable",
					Interactive: true,
				},
			},
		}
		setNodeBoundsForTest(t, root, NormalizedRect{X: 0, Y: 0, Width: 1, Height: 1})
		setNodeBoundsForTest(t, root.Children[0], NormalizedRect{X: 0.10, Y: 0.70, Width: 0.80, Height: 0.12})
		return BuildStructuredSnapshot(BuildStructuredSnapshotOptions{
			WindowID: "win-1",
			Title:    "Feishu",
		}, root)
	}

	preSubmit := buildSnapshot("Message")
	postSubmit := buildSnapshot("Type a message")
	if preSubmit == nil || postSubmit == nil {
		t.Fatal("BuildStructuredSnapshot() = nil, want snapshots")
	}

	preSubmitID := preSubmit.LookupToken("token-message")
	postSubmitID := postSubmit.LookupToken("token-message")
	if preSubmitID == 0 || postSubmitID == 0 {
		t.Fatalf("LookupToken(token-message) = (%d, %d), want non-zero ids", preSubmitID, postSubmitID)
	}

	preSubmitNode, ok := snapshotNodeByID(preSubmit, preSubmitID)
	if !ok {
		t.Fatalf("snapshotNodeByID(%d) missing in pre-submit snapshot", preSubmitID)
	}
	postSubmitNode, ok := snapshotNodeByID(postSubmit, postSubmitID)
	if !ok {
		t.Fatalf("snapshotNodeByID(%d) missing in post-submit snapshot", postSubmitID)
	}
	if strings.TrimSpace(preSubmitNode.StableID) == "" {
		t.Fatal("preSubmitNode.StableID = empty, want stable id")
	}
	if preSubmitNode.StableID != postSubmitNode.StableID {
		t.Fatalf("StableID changed across composer name drift: %q vs %q", preSubmitNode.StableID, postSubmitNode.StableID)
	}
}

func setNodeBoundsForTest(t *testing.T, node *Node, bounds NormalizedRect) {
	t.Helper()
	if node == nil {
		t.Fatal("node = nil, want non-nil")
	}
	value := reflect.ValueOf(node).Elem().FieldByName("Bounds")
	if !value.IsValid() {
		t.Fatal("Node.Bounds field is missing")
	}
	if !value.CanSet() {
		t.Fatal("Node.Bounds field is not settable")
	}
	if value.Type() != reflect.TypeOf(NormalizedRect{}) {
		t.Fatalf("Node.Bounds type = %v, want %v", value.Type(), reflect.TypeOf(NormalizedRect{}))
	}
	value.Set(reflect.ValueOf(bounds))
}

func TestSnapshotStore_SwapMarkDirtyAndPatch(t *testing.T) {
	store := NewSnapshotStore()
	base := &Snapshot{
		WindowID: "win-1",
		Nodes: []FlatNode{
			{NodeID: 1, Role: "window", Name: "Feishu"},
			{NodeID: 2, ParentID: 1, Role: "text_field", Name: "Message", BackendToken: "token-message", Interactive: true},
		},
	}

	current := store.Swap(base)
	if current == nil {
		t.Fatal("Swap() = nil")
	}
	if current.Revision == 0 {
		t.Fatalf("Revision = %d, want > 0", current.Revision)
	}

	dirty, ok := store.MarkDirty("win-1", "structure_changed")
	if !ok {
		t.Fatal("MarkDirty() = false, want true")
	}
	if !dirty.Dirty {
		t.Fatal("Dirty = false, want true")
	}
	if dirty.DirtyReason != "structure_changed" {
		t.Fatalf("DirtyReason = %q, want structure_changed", dirty.DirtyReason)
	}
	if dirty.Revision <= current.Revision {
		t.Fatalf("dirty revision = %d, want > %d", dirty.Revision, current.Revision)
	}

	patched, ok := store.ApplyPatch("win-1", "value_changed", func(existing *Snapshot) *Snapshot {
		out := existing.Clone()
		out.Nodes[1].Value = "hello"
		return out
	})
	if !ok {
		t.Fatal("ApplyPatch() = false, want true")
	}
	if got := patched.Nodes[1].Value; got != "hello" {
		t.Fatalf("patched value = %q, want hello", got)
	}
	if patched.Revision <= dirty.Revision {
		t.Fatalf("patched revision = %d, want > %d", patched.Revision, dirty.Revision)
	}
}
