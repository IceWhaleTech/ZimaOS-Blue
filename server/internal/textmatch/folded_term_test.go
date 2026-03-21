package textmatch

import "testing"

func TestFoldedTermMatcher_RespectsASCIIWordBoundaries(t *testing.T) {
	matcher := NewFoldedTermMatcher([]string{"mode", "selector", "工具"})

	if !matcher.ContainsAnyFold("How do I use selector mode?") {
		t.Fatal("expected boundary-aware matcher to detect standalone ASCII terms")
	}
	if matcher.ContainsAnyFold("This model should not trigger the term matcher.") {
		t.Fatal("expected ASCII word boundary to block partial word match")
	}
	if !matcher.ContainsAnyFold("这个工具应该被识别") {
		t.Fatal("expected non-ASCII term to match by substring")
	}
}

func TestFoldedTermMatcher_AllowsPunctuationTermsInsideURLs(t *testing.T) {
	matcher := NewFoldedTermMatcher([]string{"https://", "www.", ".md"})

	if !matcher.ContainsAnyFold("Open https://example.com/docs now.") {
		t.Fatal("expected URL prefix to match inside full URL")
	}
	if !matcher.ContainsAnyFold("Please review report.md in the workspace.") {
		t.Fatal("expected file extension term to match inside filename")
	}
}

func TestFoldedTermMatcher_FirstMatchFold(t *testing.T) {
	matcher := NewFoldedTermMatcher([]string{"不要", "stop", "cancel"})
	match, ok := matcher.FirstMatchFold("请不要继续执行")
	if !ok {
		t.Fatal("expected first match")
	}
	if match.Start != 1 || match.End != 3 {
		t.Fatalf("match = %+v, want start=1 end=3", match)
	}
}
