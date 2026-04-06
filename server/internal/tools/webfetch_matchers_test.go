package tools

import "testing"

func TestWebFetchCueMatcher_InitializesOnDemand(t *testing.T) {
	matcher := newWebFetchCueMatcher([]string{"login", "password"})
	if matcher.inner != nil {
		t.Fatal("expected webfetch cue matcher inner to start nil")
	}

	if !containsAnyWebFetchCue(matcher, "please log in with password") {
		t.Fatal("expected matcher to initialize on first access")
	}
	if matcher.inner == nil {
		t.Fatal("expected webfetch cue matcher inner to initialize lazily")
	}

	if got := countDistinctWebFetchCues(matcher, "login", "password"); got != 2 {
		t.Fatalf("countDistinctWebFetchCues = %d, want 2", got)
	}
}
