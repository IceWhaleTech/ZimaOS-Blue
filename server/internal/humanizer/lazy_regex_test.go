package humanizer

import (
	"sync"
	"testing"
)

func resetHumanizerRegexesForTest(t *testing.T) {
	t.Helper()

	codeFenceRe = nil
	headerRe = nil
	horizontalRe = nil
	blockquoteRe = nil
	bulletDashRe = nil
	numberedListRe = nil
	boldRe = nil
	boldUnderRe = nil
	italicRe = nil
	italicUnderRe = nil
	strikethroughRe = nil
	inlineCodeRe = nil
	linkRe = nil
	imageRe = nil
	htmlTagRe = nil
	emojiRe = nil
	mathBlockRe = nil
	mathInlineRe = nil
	bareURLRe = nil
	multiBlankLineRe = nil
	trailingSpaceRe = nil
	typelessCardRe = nil
	functionCallsRe = nil
	invokeRe = nil
	paramRe = nil
	processCommentBlockRe = nil
	processFenceBlockRe = nil
	italicStarStripRe = nil
	italicUnderStripRe = nil
	fenceLineRe = nil
	paragraphBreakRe = nil
	imageTextRe = nil
	humanizerRegexOnce = sync.Once{}

	t.Cleanup(func() {
		codeFenceRe = nil
		headerRe = nil
		horizontalRe = nil
		blockquoteRe = nil
		bulletDashRe = nil
		numberedListRe = nil
		boldRe = nil
		boldUnderRe = nil
		italicRe = nil
		italicUnderRe = nil
		strikethroughRe = nil
		inlineCodeRe = nil
		linkRe = nil
		imageRe = nil
		htmlTagRe = nil
		emojiRe = nil
		mathBlockRe = nil
		mathInlineRe = nil
		bareURLRe = nil
		multiBlankLineRe = nil
		trailingSpaceRe = nil
		typelessCardRe = nil
		functionCallsRe = nil
		invokeRe = nil
		paramRe = nil
		processCommentBlockRe = nil
		processFenceBlockRe = nil
		italicStarStripRe = nil
		italicUnderStripRe = nil
		fenceLineRe = nil
		paragraphBreakRe = nil
		imageTextRe = nil
		humanizerRegexOnce = sync.Once{}
		ensureHumanizerRegexes()
	})
}

func TestHumanizerRegexes_InitializeOnDemand(t *testing.T) {
	resetHumanizerRegexesForTest(t)

	if codeFenceRe != nil || paragraphBreakRe != nil || imageTextRe != nil {
		t.Fatal("expected humanizer regexes to start nil")
	}

	if got := Humanize("```go\nfmt.Println()\n```", ModeIM); got != "fmt.Println()" {
		t.Fatalf("Humanize() = %q, want %q", got, "fmt.Println()")
	}
	if codeFenceRe == nil || htmlTagRe == nil || functionCallsRe == nil {
		t.Fatal("expected humanizer rules regexes to initialize on first humanize call")
	}

	chunks := ChunkByParagraph("first paragraph\n\nsecond paragraph", 200)
	if len(chunks) != 2 || chunks[0] != "first paragraph" || chunks[1] != "second paragraph" {
		t.Fatalf("ChunkByParagraph() = %#v, want two paragraph chunks", chunks)
	}
	if fenceLineRe == nil || paragraphBreakRe == nil {
		t.Fatal("expected humanizer chunk regexes to initialize on first chunking call")
	}

	if got := RenderVoice(IR{Text: "(image: chart) Hello https://example.com"}); got != "Hello" {
		t.Fatalf("RenderVoice() = %q, want %q", got, "Hello")
	}
	if imageTextRe == nil || emojiRe == nil || bareURLRe == nil {
		t.Fatal("expected render regexes to initialize on first voice render")
	}
}
