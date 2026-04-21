package uiexec

import "testing"

func TestFinder_FindNode_RoleAndExactName(t *testing.T) {
	finder := NewFinder([]Node{
		{ID: "n1", Role: "button", Name: "Send", Actions: []string{"click"}},
		{ID: "n2", Role: "input", Name: "Message", Actions: []string{"type", "focus"}},
	})

	node := finder.FindNode(Query{Role: "button", Name: "Send", RequireCapability: "click"})
	if node == nil || node.ID != "n1" {
		t.Fatalf("FindNode() = %#v, want node n1", node)
	}
}

func TestFinder_FindNode_NameApprox_UsesEmbeddingFallback(t *testing.T) {
	finder := NewFinder([]Node{
		{ID: "n1", Role: "text", Name: "Alice"},
		{ID: "n2", Role: "text", Name: "Bob"},
	})

	node := finder.FindNode(Query{NameApprox: "Alce"})
	if node == nil || node.ID != "n1" {
		t.Fatalf("FindNode(NameApprox) = %#v, want node n1", node)
	}
}

func TestFinder_FindNode_OperableFilter(t *testing.T) {
	finder := NewFinder([]Node{
		{ID: "n1", Role: "button", Name: "Submit", Actions: []string{"click"}},
		{ID: "n2", Role: "button", Name: "Submit"},
	})

	node := finder.FindNode(Query{Name: "Submit", RequireCapability: "click"})
	if node == nil || node.ID != "n1" {
		t.Fatalf("FindNode(RequireCapability) = %#v, want node n1", node)
	}
}
