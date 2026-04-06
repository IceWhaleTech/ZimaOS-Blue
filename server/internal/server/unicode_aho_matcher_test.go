package server

import "testing"

func TestUnicodeAhoMatcher_InitializesOnDemand(t *testing.T) {
	matcher := newUnicodeAhoMatcher([]string{"latest", "today"})

	if matcher.inner != nil {
		t.Fatal("expected unicode aho matcher inner to start nil")
	}

	if !matcher.ContainsAnyFold("latest news") {
		t.Fatal("expected matcher to work after first access")
	}
	if matcher.inner == nil {
		t.Fatal("expected matcher inner to initialize on first access")
	}

	if !matcher.HasAnyPrefixFold("today only") {
		t.Fatal("expected prefix matcher to keep working after lazy init")
	}
}
