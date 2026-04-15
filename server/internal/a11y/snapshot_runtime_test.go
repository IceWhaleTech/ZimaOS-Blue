package a11y

import "testing"

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
