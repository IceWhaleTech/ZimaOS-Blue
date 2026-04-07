package pruner

import (
	"regexp"
	"strings"
	"sync"
)

var (
	segmentPatterns     []segmentPattern
	segmentPatternsOnce sync.Once

	sentenceSplitPattern     *regexp.Regexp
	sentenceSplitPatternOnce sync.Once

	mdImageRe         *regexp.Regexp
	mdLinkRe          *regexp.Regexp
	mdAutoLinkRe      *regexp.Regexp
	mdUnorderedListRe *regexp.Regexp
	mdOrderedListRe   *regexp.Regexp
	mdTaskListRe      *regexp.Regexp
	mdHeadingRe       *regexp.Regexp
	mdBlockQuoteRe    *regexp.Regexp
	mdRuleRe          *regexp.Regexp
	mdInlineCleaner   *strings.Replacer
	markdownRegexOnce sync.Once

	logTimestampRe    *regexp.Regexp
	markdownHeadingRe *regexp.Regexp
	detectorRegexOnce sync.Once
)

func ensureSegmentPatterns() {
	segmentPatternsOnce.Do(func() {
		segmentPatterns = []segmentPattern{
			// Go: func, type struct/interface
			{re: regexp.MustCompile(`^\s*func\s+`), kind: SegmentFunction},
			{re: regexp.MustCompile(`^\s*type\s+\w+\s+(struct|interface)\s*\{`), kind: SegmentClass},
			// Python: def, class
			{re: regexp.MustCompile(`^\s*def\s+\w+\s*\(`), kind: SegmentFunction},
			{re: regexp.MustCompile(`^\s*class\s+\w+`), kind: SegmentClass},
			// JS/TS: function, class, arrow (const x = (...) =>)
			{re: regexp.MustCompile(`^\s*function\s+\w+`), kind: SegmentFunction},
			{re: regexp.MustCompile(`^\s*class\s+\w+`), kind: SegmentClass},
			// Rust: fn, impl, struct, enum
			{re: regexp.MustCompile(`^\s*(pub\s+)?fn\s+\w+`), kind: SegmentFunction},
			{re: regexp.MustCompile(`^\s*impl\s+`), kind: SegmentClass},
			// Java: public/private/protected methods and classes
			{re: regexp.MustCompile(`^\s*(public|private|protected)\s+class\s+`), kind: SegmentClass},
		}
	})
}

func ensureSentenceSplitPattern() {
	sentenceSplitPatternOnce.Do(func() {
		sentenceSplitPattern = regexp.MustCompile(`(?:[.!?]+\s+)|[。！？]+|\n+`)
	})
}

func ensureMarkdownRegexes() {
	markdownRegexOnce.Do(func() {
		mdImageRe = regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`)
		mdLinkRe = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
		mdAutoLinkRe = regexp.MustCompile(`<((?:https?|mailto):[^>]+)>`)
		mdUnorderedListRe = regexp.MustCompile(`^\s*[-*+]\s+`)
		mdOrderedListRe = regexp.MustCompile(`^\s*\d+[.)]\s+`)
		mdTaskListRe = regexp.MustCompile(`^\[(?: |x|X)\]\s+`)
		mdHeadingRe = regexp.MustCompile(`^\s{0,3}#{1,6}\s+`)
		mdBlockQuoteRe = regexp.MustCompile(`^\s*>\s*`)
		mdRuleRe = regexp.MustCompile(`^\s*([-*_]\s*){3,}\s*$`)
		mdInlineCleaner = strings.NewReplacer("**", "", "__", "", "*", "", "_", "", "~~", "", "`", "")
	})
}

func ensureDetectorRegexes() {
	detectorRegexOnce.Do(func() {
		logTimestampRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}`)
		markdownHeadingRe = regexp.MustCompile(`^#{1,6}\s+\S`)
	})
}
