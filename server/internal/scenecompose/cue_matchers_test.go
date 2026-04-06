package scenecompose

import "testing"

func TestFoldedCueMatcher_InitializesOnDemand(t *testing.T) {
	matcher := newFoldedCueMatcher([]string{"forest", "woods"})
	if matcher.inner != nil {
		t.Fatal("expected folded cue matcher inner to start nil")
	}

	if !matcher.Contains("forest trail") {
		t.Fatal("expected matcher to initialize on first access")
	}
	if matcher.inner == nil {
		t.Fatal("expected folded cue matcher inner to initialize lazily")
	}

	if got := matcher.Count("forest woods forest"); got != 2 {
		t.Fatalf("Count = %d, want 2", got)
	}
}
