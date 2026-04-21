package uiexec

import "testing"

func TestPolicyCache_Upsert_ReplacesWithShorterSuccessfulPath(t *testing.T) {
	cache := NewPolicyCache()

	task := "click send"
	state := State{PageHint: "compose", FocusedRole: "input"}

	longer := Policy{
		TaskPattern:  TaskPatternFromTask(task),
		StatePattern: StatePatternFromState(state),
		Actions: []Action{
			{Type: "scroll", Value: "down"},
			{Type: "find", Value: "Send"},
			{Type: "click"},
		},
		Cost: 3,
	}
	shorter := Policy{
		TaskPattern:  TaskPatternFromTask(task),
		StatePattern: StatePatternFromState(state),
		Actions: []Action{
			{Type: "click", Target: "node_send"},
		},
		Cost: 1,
	}

	cache.Upsert(longer, true)
	cache.Upsert(shorter, true)

	got, ok := cache.Match(task, state)
	if !ok {
		t.Fatalf("Match() ok=false, want true")
	}
	if got.Cost != 1 || len(got.Actions) != 1 {
		t.Fatalf("Match() = %#v, want shorter cost=1", got)
	}
}
