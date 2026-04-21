package uiexec

import "testing"

func TestPolicyCache_Match_FuzzyTaskPattern(t *testing.T) {
	cache := NewPolicyCache()
	p := Policy{
		TaskPattern:  "click send",
		StatePattern: "compose|input|send,message",
		Actions:      []Action{{Type: "click", Target: "send"}},
		Cost:         1,
	}
	cache.Upsert(p, true)

	state := State{PageHint: "Compose", FocusedRole: "input", VisibleEntities: []string{"Message", "Send"}}
	got, ok := cache.Match("click the send button", state)
	if !ok {
		t.Fatalf("Match() ok=false, want true")
	}
	if got.Cost != 1 {
		t.Fatalf("Match() = %#v, want cost=1", got)
	}
}
