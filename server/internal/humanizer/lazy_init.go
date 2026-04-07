package humanizer

import (
	"regexp"
	"sync"
)

var (
	codeFenceRe    *regexp.Regexp
	headerRe       *regexp.Regexp
	horizontalRe   *regexp.Regexp
	blockquoteRe   *regexp.Regexp
	bulletDashRe   *regexp.Regexp
	numberedListRe *regexp.Regexp

	boldRe          *regexp.Regexp
	boldUnderRe     *regexp.Regexp
	italicRe        *regexp.Regexp
	italicUnderRe   *regexp.Regexp
	strikethroughRe *regexp.Regexp
	inlineCodeRe    *regexp.Regexp
	linkRe          *regexp.Regexp
	imageRe         *regexp.Regexp
	htmlTagRe       *regexp.Regexp

	emojiRe *regexp.Regexp

	mathBlockRe  *regexp.Regexp
	mathInlineRe *regexp.Regexp

	bareURLRe *regexp.Regexp

	multiBlankLineRe *regexp.Regexp
	trailingSpaceRe  *regexp.Regexp

	typelessCardRe *regexp.Regexp

	functionCallsRe *regexp.Regexp
	invokeRe        *regexp.Regexp
	paramRe         *regexp.Regexp

	processCommentBlockRe *regexp.Regexp
	processFenceBlockRe   *regexp.Regexp

	italicStarStripRe  *regexp.Regexp
	italicUnderStripRe *regexp.Regexp

	fenceLineRe      *regexp.Regexp
	paragraphBreakRe *regexp.Regexp
	imageTextRe      *regexp.Regexp

	humanizerRegexOnce sync.Once
)

func ensureHumanizerRegexes() {
	humanizerRegexOnce.Do(func() {
		codeFenceRe = regexp.MustCompile("(?s)```[\\w]*\\n?(.*?)```")
		headerRe = regexp.MustCompile(`(?m)^#{1,6}\s+`)
		horizontalRe = regexp.MustCompile(`(?m)^[\s]*([-*_]){3,}\s*$`)
		blockquoteRe = regexp.MustCompile(`(?m)^>\s?`)
		bulletDashRe = regexp.MustCompile(`(?m)^(\s*)[-*+]\s`)
		numberedListRe = regexp.MustCompile(`(?m)^(\s*)\d+\.\s`)

		boldRe = regexp.MustCompile(`\*\*(.+?)\*\*`)
		boldUnderRe = regexp.MustCompile(`__(.+?)__`)
		italicRe = regexp.MustCompile(`(?:^|[^*])\*([^*]+?)\*(?:[^*]|$)`)
		italicUnderRe = regexp.MustCompile(`(?:^|[^_])_([^_]+?)_(?:[^_]|$)`)
		strikethroughRe = regexp.MustCompile(`~~(.+?)~~`)
		inlineCodeRe = regexp.MustCompile("`([^`]+)`")
		linkRe = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
		imageRe = regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`)
		htmlTagRe = regexp.MustCompile(`<[^>]+>`)

		emojiRe = regexp.MustCompile(`[\x{1F600}-\x{1F64F}]|[\x{1F300}-\x{1F5FF}]|[\x{1F680}-\x{1F6FF}]|[\x{1F1E0}-\x{1F1FF}]|[\x{2600}-\x{26FF}]|[\x{2700}-\x{27BF}]|[\x{FE00}-\x{FE0F}]|[\x{1F900}-\x{1F9FF}]|[\x{1FA00}-\x{1FA6F}]|[\x{1FA70}-\x{1FAFF}]|[\x{200D}]|[\x{20E3}]|[\x{FE0F}]|[\x{2300}-\x{23FF}]|[\x{2B05}-\x{2B07}]|[\x{2B1B}-\x{2B1C}]|[\x{2B50}]|[\x{2B55}]|[\x{3030}]|[\x{303D}]|[\x{3297}]|[\x{3299}]|[\x{1F3FB}-\x{1F3FF}]|[\x{E0020}-\x{E007F}]|[\x{200B}-\x{200F}]|[\x{2028}-\x{202F}]|[\x{2060}-\x{206F}]`)

		mathBlockRe = regexp.MustCompile(`(?s)\$\$(.+?)\$\$`)
		mathInlineRe = regexp.MustCompile(`\$([^\$\n]+?)\$`)

		bareURLRe = regexp.MustCompile(`https?://[^\s\)>\]]+`)

		multiBlankLineRe = regexp.MustCompile(`\n{3,}`)
		trailingSpaceRe = regexp.MustCompile(`(?m)[ \t]+$`)

		typelessCardRe = regexp.MustCompile("(?s)```typeless\\n?(.*?)```")

		functionCallsRe = regexp.MustCompile(`(?s)<(?:antml:)?function_calls>(.*?)</(?:antml:)?function_calls>`)
		invokeRe = regexp.MustCompile(`(?s)<(?:antml:)?invoke\s+name="([^"]+)">(.*?)</(?:antml:)?invoke>`)
		paramRe = regexp.MustCompile(`(?s)<(?:antml:)?parameter\s+name="([^"]+)">(.*?)</(?:antml:)?parameter>`)

		processCommentBlockRe = regexp.MustCompile(`(?s)<!--\s*process-start\s*-->.*?<!--\s*process-end\s*-->`)
		processFenceBlockRe = regexp.MustCompile("(?s)```process\\n?(.*?)```")

		italicStarStripRe = regexp.MustCompile(`(?:^|\s)\*([^*\n]+?)\*(?:\s|$|[.,!?;:])`)
		italicUnderStripRe = regexp.MustCompile(`(?:^|\s)_([^_\n]+?)_(?:\s|$|[.,!?;:])`)

		fenceLineRe = regexp.MustCompile(`^( {0,3})(` + "`{3,}" + `|~{3,})(.*)$`)
		paragraphBreakRe = regexp.MustCompile(`\n[\t ]*\n+`)
		imageTextRe = regexp.MustCompile(`\(image: [^)]*\)`)
	})
}
