package mediagen

import (
	"sort"
	"testing"
)

func TestScanKeywordCueMatcher_RespectsShortASCIIWordBoundaries(t *testing.T) {
	matcher := newScanKeywordCueMatcher([]string{"move"})

	if matcher.Contains("please move this image") != true {
		t.Fatal("expected matcher to detect standalone short ASCII cue")
	}
	if matcher.Contains("please remove this image") {
		t.Fatal("expected matcher to avoid matching move inside remove")
	}
}

func TestScanKeywordCueMatcher_CountDistinctAndFindMatches(t *testing.T) {
	matcher := newScanKeywordCueMatcher([]string{"生成", "图片", "生成"})

	if got := matcher.CountDistinct("请生成图片，然后再次生成"); got != 2 {
		t.Fatalf("CountDistinct = %d, want 2", got)
	}

	matches := matcher.FindMatches("请生成图片")
	if len(matches) != 2 {
		t.Fatalf("FindMatches len = %d, want 2", len(matches))
	}
	if matches[0].start != 1 || matches[0].end != 3 {
		t.Fatalf("first match = %+v, want start=1 end=3", matches[0])
	}
	if matches[1].start != 3 || matches[1].end != 5 {
		t.Fatalf("second match = %+v, want start=3 end=5", matches[1])
	}
}

func TestAhoKeywordCueMatcher_MatchesScanBehavior(t *testing.T) {
	tests := []struct {
		name          string
		cues          []string
		text          string
		wantContains  bool
		wantDistinct  int
		wantMatchLens int
	}{
		{
			name:          "short ascii respects boundaries",
			cues:          []string{"move"},
			text:          "Please MOVE this image, not remove it.",
			wantContains:  true,
			wantDistinct:  1,
			wantMatchLens: 1,
		},
		{
			name:          "overlapping unicode patterns",
			cues:          []string{"生成", "图片", "生成图片"},
			text:          "请生成图片",
			wantContains:  true,
			wantDistinct:  3,
			wantMatchLens: 3,
		},
		{
			name:          "duplicate cues dedupe",
			cues:          []string{"image", "image", "picture"},
			text:          "Generate an IMAGE, not just a caption.",
			wantContains:  true,
			wantDistinct:  1,
			wantMatchLens: 1,
		},
		{
			name:          "no match",
			cues:          []string{"video", "animate"},
			text:          "Write a summary about sunsets.",
			wantContains:  false,
			wantDistinct:  0,
			wantMatchLens: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scan := newScanKeywordCueMatcher(tt.cues)
			aho := newAhoKeywordCueMatcher(tt.cues)

			if got := scan.Contains(tt.text); got != tt.wantContains {
				t.Fatalf("scan Contains = %v, want %v", got, tt.wantContains)
			}
			if got := aho.Contains(tt.text); got != tt.wantContains {
				t.Fatalf("aho Contains = %v, want %v", got, tt.wantContains)
			}

			if got := scan.CountDistinct(tt.text); got != tt.wantDistinct {
				t.Fatalf("scan CountDistinct = %d, want %d", got, tt.wantDistinct)
			}
			if got := aho.CountDistinct(tt.text); got != tt.wantDistinct {
				t.Fatalf("aho CountDistinct = %d, want %d", got, tt.wantDistinct)
			}

			scanMatches := scan.FindMatches(tt.text)
			ahoMatches := aho.FindMatches(tt.text)
			if len(scanMatches) != tt.wantMatchLens {
				t.Fatalf("scan FindMatches len = %d, want %d", len(scanMatches), tt.wantMatchLens)
			}
			if len(ahoMatches) != tt.wantMatchLens {
				t.Fatalf("aho FindMatches len = %d, want %d", len(ahoMatches), tt.wantMatchLens)
			}
			if len(scanMatches) != len(ahoMatches) {
				t.Fatalf("match len mismatch: scan=%d aho=%d", len(scanMatches), len(ahoMatches))
			}
			sortCueMatches(scanMatches)
			sortCueMatches(ahoMatches)
			for i := range scanMatches {
				if scanMatches[i] != ahoMatches[i] {
					t.Fatalf("match[%d] mismatch: scan=%+v aho=%+v", i, scanMatches[i], ahoMatches[i])
				}
			}
		})
	}
}

func TestAhoKeywordCueMatcher_InitializesMatcherOnDemand(t *testing.T) {
	matcher, ok := newAhoKeywordCueMatcher([]string{"image", "photo"}).(*ahoKeywordCueMatcher)
	if !ok {
		t.Fatal("expected aho keyword cue matcher implementation")
	}
	if matcher.matcher != nil {
		t.Fatal("expected aho matcher to start nil")
	}

	if !matcher.Contains("Please generate an image") {
		t.Fatal("expected matcher to initialize on first access")
	}
	if matcher.matcher == nil {
		t.Fatal("expected aho matcher inner to initialize lazily")
	}

	if got := matcher.CountDistinct("photo image"); got != 2 {
		t.Fatalf("CountDistinct = %d, want 2", got)
	}
}

func sortCueMatches(matches []cueMatch) {
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].start == matches[j].start {
			return matches[i].end < matches[j].end
		}
		return matches[i].start < matches[j].start
	})
}
