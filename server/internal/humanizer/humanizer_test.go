package humanizer

import "testing"

func TestHumanize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		mode     Mode
		expected string
	}{
		// Empty input
		{"empty", "", ModeIM, ""},
		{"empty voice", "", ModeVoice, ""},

		// Bold
		{"bold IM", "**hello** world", ModeIM, "hello world"},
		{"bold voice", "**hello** world", ModeVoice, "hello world"},
		{"bold underscore", "__hello__ world", ModeIM, "hello world"},

		// Inline code
		{"inline code", "use `fmt.Println()` here", ModeIM, "use fmt.Println() here"},

		// Code fences - IM keeps content
		{"code fence IM", "```go\nfmt.Println()\n```", ModeIM, "fmt.Println()"},
		// Code fences - Voice replaces with indicator
		{"code fence voice", "```go\nfmt.Println()\n```", ModeVoice, "(code omitted)"},

		// Headers
		{"h1", "# Title", ModeIM, "Title"},
		{"h2", "## Section", ModeIM, "Section"},
		{"h3", "### Sub", ModeVoice, "Sub"},

		// Horizontal rules
		{"hr dashes", "above\n\n---\n\nbelow", ModeIM, "above\n\nbelow"},
		{"hr stars", "above\n\n***\n\nbelow", ModeIM, "above\n\nbelow"},

		// Blockquotes
		{"blockquote", "> This is quoted", ModeIM, "This is quoted"},
		{"blockquote multi", "> line1\n> line2", ModeIM, "line1\nline2"},

		// Strikethrough
		{"strikethrough", "~~deleted~~ text", ModeIM, "deleted text"},

		// Links
		{"link IM", "[Google](https://google.com)", ModeIM, "Google (https://google.com)"},
		{"link voice", "[Google](https://google.com)", ModeVoice, "Google"},

		// Images
		{"image IM", "![photo](https://img.com/a.png)", ModeIM, "(image: photo)"},
		{"image voice", "![photo](https://img.com/a.png)", ModeVoice, ""},

		// HTML tags
		{"html br", "line1<br>line2", ModeIM, "line1line2"},
		{"html bold", "<b>bold</b>", ModeIM, "bold"},

		// Bullets
		{"bullet IM", "- item one\n- item two", ModeIM, "• item one\n• item two"},
		{"bullet voice", "- item one\n- item two", ModeVoice, "item one\nitem two"},

		// Emojis (only stripped in voice mode)
		{"emoji IM", "Hello 😊 world", ModeIM, "Hello 😊 world"},
		{"emoji voice", "Hello 😊 world", ModeVoice, "Hello  world"},

		// Whitespace normalization
		{"multi blank lines", "a\n\n\n\n\nb", ModeIM, "a\n\nb"},
		{"trailing spaces", "hello   \nworld  ", ModeIM, "hello\nworld"},

		// Mixed content
		{
			"mixed markdown",
			"## Welcome\n\n**Bold** and `code` here.\n\n- item 1\n- item 2\n\n---\n\n> A quote\n\n```\nsome code\n```",
			ModeIM,
			"Welcome\n\nBold and code here.\n\n• item 1\n• item 2\n\nA quote\n\nsome code",
		},
		{
			"mixed voice",
			"## Welcome\n\n**Bold** and `code` here.\n\n- item 1\n- item 2\n\n---\n\n> A quote\n\n```\nsome code\n```",
			ModeVoice,
			"Welcome\n\nBold and code here.\n\nitem 1\nitem 2\n\nA quote\n\n(code omitted)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Humanize(tt.input, tt.mode)
			if got != tt.expected {
				t.Errorf("Humanize(%q, %v)\n  got:  %q\n  want: %q", tt.input, tt.mode, got, tt.expected)
			}
		})
	}
}

func BenchmarkHumanize(b *testing.B) {
	input := "## Title\n\n**Bold** text with `inline code` and [link](https://example.com).\n\n- item 1\n- item 2\n\n```go\nfmt.Println(\"hello\")\n```\n\n> A blockquote\n\n---\n\nMore text with ~~strikethrough~~ and emojis 😊🎉."

	b.Run("ModeIM", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			Humanize(input, ModeIM)
		}
	})

	b.Run("ModeVoice", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			Humanize(input, ModeVoice)
		}
	})
}
