package channel

import "testing"

func TestChannelMessageRegexes_InitializeOnDemand(t *testing.T) {
	originalMultiNewline := multiNewlineRe
	originalAITagPatterns := aiTagPatterns

	multiNewlineRe = nil
	aiTagPatterns = nil
	t.Cleanup(func() {
		multiNewlineRe = originalMultiNewline
		aiTagPatterns = originalAITagPatterns
	})

	if multiNewlineRe != nil || aiTagPatterns != nil {
		t.Fatal("expected channel message regexes to start nil")
	}

	cleaned := StripAITags("before<thinking>hidden</thinking>\n\n\n\nafter")
	if cleaned != "before\n\nafter" {
		t.Fatalf("StripAITags() = %q, want AI tags stripped and newlines collapsed", cleaned)
	}
	if multiNewlineRe == nil {
		t.Fatal("expected newline regex to initialize on first strip")
	}
	if len(aiTagPatterns) == 0 {
		t.Fatal("expected AI tag regexes to initialize on first strip")
	}
}

func TestChannelManagerRegexes_InitializeOnDemand(t *testing.T) {
	originalHeading := markdownHeadingLineRe
	originalTable := markdownTableDividerLineRe
	originalFence := markdownFenceLineRe
	originalBullet := markdownBulletLineRe
	originalOrdered := markdownOrderedLineRe
	originalMention := mentionMarkupTagRe

	markdownHeadingLineRe = nil
	markdownTableDividerLineRe = nil
	markdownFenceLineRe = nil
	markdownBulletLineRe = nil
	markdownOrderedLineRe = nil
	mentionMarkupTagRe = nil
	t.Cleanup(func() {
		markdownHeadingLineRe = originalHeading
		markdownTableDividerLineRe = originalTable
		markdownFenceLineRe = originalFence
		markdownBulletLineRe = originalBullet
		markdownOrderedLineRe = originalOrdered
		mentionMarkupTagRe = originalMention
	})

	if markdownHeadingLineRe != nil || markdownTableDividerLineRe != nil || markdownFenceLineRe != nil || markdownBulletLineRe != nil || markdownOrderedLineRe != nil || mentionMarkupTagRe != nil {
		t.Fatal("expected channel manager regexes to start nil")
	}

	if got := normalizeMentionTarget(`<at user_id="u_1">@Orca</at>`); got != "orca" {
		t.Fatalf("normalizeMentionTarget() = %q, want %q", got, "orca")
	}
	if mentionMarkupTagRe == nil {
		t.Fatal("expected mention regex to initialize on first normalization")
	}
	if markdownHeadingLineRe != nil || markdownTableDividerLineRe != nil || markdownFenceLineRe != nil || markdownBulletLineRe != nil || markdownOrderedLineRe != nil {
		t.Fatal("expected markdown regexes to remain cold until markdown detection runs")
	}

	if !looksLikeMarkdown("# Summary\n- item one\n- item two") {
		t.Fatal("expected markdown detection to work after lazy init")
	}
	if markdownHeadingLineRe == nil || markdownBulletLineRe == nil {
		t.Fatal("expected markdown regexes to initialize on demand")
	}
}
