package uiexec

import "testing"

func TestTryDirectAction_FindsAndExecutes(t *testing.T) {
	finder := NewFinder([]Node{
		{ID: "send", Role: "button", Name: "Send", Actions: []string{"click"}},
		{ID: "search", Role: "input", Name: "Search", Actions: []string{"focus", "type"}},
	})
	var got Action
	exec := func(a Action) error {
		got = a
		return nil
	}

	ok := TryDirectAction(finder, exec, "Send", "click")
	if !ok {
		t.Fatalf("TryDirectAction() = false, want true")
	}
	if got.Type != "click" || got.Target != "send" {
		t.Fatalf("executed = %#v, want click(send)", got)
	}
}

func TestTryDirectAction_MissReturnsFalse(t *testing.T) {
	finder := NewFinder([]Node{{ID: "send", Role: "button", Name: "Send", Actions: []string{"click"}}})
	exec := func(a Action) error { return nil }

	ok := TryDirectAction(finder, exec, "Alice", "click")
	if ok {
		t.Fatalf("TryDirectAction() = true, want false")
	}
}
