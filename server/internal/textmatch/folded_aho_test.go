package textmatch

import "testing"

func TestFoldedAhoMatcherContainsAndPrefix(t *testing.T) {
	matcher := NewFoldedAhoMatcher([]string{"summary", "i will", "总结", "要約"})

	if !matcher.ContainsAnyFold("Please write a SUMMARY of the changes.") {
		t.Fatal("expected folded matcher to detect case-insensitive English pattern")
	}
	if !matcher.ContainsAnyFold("请给我一个总结") {
		t.Fatal("expected folded matcher to detect Chinese pattern")
	}
	if matcher.ContainsAnyFold("Nothing relevant here.") {
		t.Fatal("expected no match for unrelated text")
	}
	if !matcher.HasAnyPrefixFold(" I will check the logs next.") {
		t.Fatal("expected folded matcher to match prefix after trimming")
	}
	if matcher.HasAnyPrefixFold("We will check the logs next.") {
		t.Fatal("expected prefix matcher to stay exact")
	}
}

func TestFoldedAhoMatcherFindAllFold(t *testing.T) {
	matcher := NewFoldedAhoMatcher([]string{"生成", "图片", "生成图片"})
	matches := matcher.FindAllFold("请生成图片")

	if len(matches) != 3 {
		t.Fatalf("FindAllFold len = %d, want 3", len(matches))
	}
	if matches[0].PatternIndex != 0 || matches[0].Start != 1 || matches[0].End != 3 {
		t.Fatalf("first match = %+v, want pattern 0 start=1 end=3", matches[0])
	}
	if matches[1].PatternIndex != 2 || matches[1].Start != 1 || matches[1].End != 5 {
		t.Fatalf("second match = %+v, want pattern 2 start=1 end=5", matches[1])
	}
	if matches[2].PatternIndex != 1 || matches[2].Start != 3 || matches[2].End != 5 {
		t.Fatalf("third match = %+v, want pattern 1 start=3 end=5", matches[2])
	}
}
