package pruner

import (
	"sync"
	"testing"
)

func resetPrunerRegexGlobalsForTest(t *testing.T) {
	t.Helper()

	segmentPatterns = nil
	sentenceSplitPattern = nil
	mdImageRe = nil
	mdLinkRe = nil
	mdAutoLinkRe = nil
	mdUnorderedListRe = nil
	mdOrderedListRe = nil
	mdTaskListRe = nil
	mdHeadingRe = nil
	mdBlockQuoteRe = nil
	mdRuleRe = nil
	mdInlineCleaner = nil
	logTimestampRe = nil
	markdownHeadingRe = nil

	segmentPatternsOnce = sync.Once{}
	sentenceSplitPatternOnce = sync.Once{}
	markdownRegexOnce = sync.Once{}
	detectorRegexOnce = sync.Once{}

	t.Cleanup(func() {
		segmentPatterns = nil
		sentenceSplitPattern = nil
		mdImageRe = nil
		mdLinkRe = nil
		mdAutoLinkRe = nil
		mdUnorderedListRe = nil
		mdOrderedListRe = nil
		mdTaskListRe = nil
		mdHeadingRe = nil
		mdBlockQuoteRe = nil
		mdRuleRe = nil
		mdInlineCleaner = nil
		logTimestampRe = nil
		markdownHeadingRe = nil

		segmentPatternsOnce = sync.Once{}
		sentenceSplitPatternOnce = sync.Once{}
		markdownRegexOnce = sync.Once{}
		detectorRegexOnce = sync.Once{}

		ensureSegmentPatterns()
		ensureSentenceSplitPattern()
		ensureMarkdownRegexes()
		ensureDetectorRegexes()
	})
}

func TestPrunerRegexes_InitializeOnDemand(t *testing.T) {
	t.Run("segmenter", func(t *testing.T) {
		resetPrunerRegexGlobalsForTest(t)

		code := "func main() {\n\tprintln(\"x\")\n}\n"
		segs := Segmentize(code)
		if len(segs) != 1 || segs[0].Kind != SegmentFunction {
			t.Fatalf("expected lazy segment regexes to preserve function detection, got %+v", segs)
		}
		if len(segmentPatterns) == 0 {
			t.Fatal("expected segment regexes to initialize on first access")
		}
	})

	t.Run("sentence split", func(t *testing.T) {
		resetPrunerRegexGlobalsForTest(t)

		got := SplitSentences("Hello world. This is a test.")
		if len(got) != 2 || got[0] != "Hello world" || got[1] != "This is a test." {
			t.Fatalf("expected lazy sentence regex to preserve splitting, got %#v", got)
		}
		if sentenceSplitPattern == nil {
			t.Fatal("expected sentence regex to initialize on first access")
		}
	})

	t.Run("markdown", func(t *testing.T) {
		resetPrunerRegexGlobalsForTest(t)

		input := "# Title\n\n- [x] done\n[link](https://example.com)\n"
		got := MarkdownToText(input)
		want := "Title\n\ndone\nlink"
		if got != want {
			t.Fatalf("expected lazy markdown regexes to preserve output:\ngot:  %q\nwant: %q", got, want)
		}
		if mdImageRe == nil || mdLinkRe == nil || mdAutoLinkRe == nil || mdUnorderedListRe == nil ||
			mdOrderedListRe == nil || mdTaskListRe == nil || mdHeadingRe == nil || mdBlockQuoteRe == nil ||
			mdRuleRe == nil || mdInlineCleaner == nil {
			t.Fatal("expected markdown cleanup helpers to initialize on first access")
		}
	})

	t.Run("detector and paragraphs", func(t *testing.T) {
		resetPrunerRegexGlobalsForTest(t)

		logs := "2026-02-15T10:30:00Z INFO server started\n2026-02-15T10:30:01Z ERROR failed\n2026-02-15T10:30:02Z INFO retrying\n2026-02-15T10:30:03Z INFO ready\n2026-02-15T10:30:04Z INFO healthy\n"
		if got := DetectContentType(logs, 5); got != ContentLog {
			t.Fatalf("expected lazy detector regexes to preserve log detection, got %v", got)
		}
		doc := "# Title\n\nParagraph one.\n\n## Section\n\nParagraph two."
		segs := SegmentizeParagraphs(doc)
		if len(segs) == 0 || segs[0].Kind != SegmentHeading {
			t.Fatalf("expected paragraph segmentation to keep markdown heading detection, got %+v", segs)
		}
		if logTimestampRe == nil || markdownHeadingRe == nil {
			t.Fatal("expected detector regexes to initialize on first access")
		}
	})
}
