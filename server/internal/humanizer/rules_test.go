package humanizer

import "testing"

func TestStripBold(t *testing.T) {
	tests := []struct{ in, want string }{
		{"**hello**", "hello"},
		{"__world__", "world"},
		{"no bold", "no bold"},
		{"**a** and **b**", "a and b"},
	}
	for _, tt := range tests {
		if got := stripBold(tt.in); got != tt.want {
			t.Errorf("stripBold(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestStripInlineCode(t *testing.T) {
	tests := []struct{ in, want string }{
		{"`code`", "code"},
		{"use `fmt` pkg", "use fmt pkg"},
		{"no code", "no code"},
	}
	for _, tt := range tests {
		if got := stripInlineCode(tt.in); got != tt.want {
			t.Errorf("stripInlineCode(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestStripCodeFences(t *testing.T) {
	tests := []struct {
		in   string
		mode Mode
		want string
	}{
		{"```go\ncode\n```", ModeIM, "code\n"},
		{"```go\ncode\n```", ModeVoice, "(code omitted)"},
		{"```\nplain\n```", ModeIM, "plain\n"},
		{"no fence", ModeIM, "no fence"},
	}
	for _, tt := range tests {
		got := stripCodeFences(tt.in, tt.mode)
		if got != tt.want {
			t.Errorf("stripCodeFences(%q, %v) = %q, want %q", tt.in, tt.mode, got, tt.want)
		}
	}
}

func TestStripHeaders(t *testing.T) {
	tests := []struct{ in, want string }{
		{"# H1", "H1"},
		{"## H2", "H2"},
		{"### H3", "H3"},
		{"###### H6", "H6"},
		{"not a header", "not a header"},
	}
	for _, tt := range tests {
		if got := stripHeaders(tt.in); got != tt.want {
			t.Errorf("stripHeaders(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestStripLinks(t *testing.T) {
	tests := []struct {
		in   string
		mode Mode
		want string
	}{
		{"[text](url)", ModeIM, "text (url)"},
		{"[text](url)", ModeVoice, "text"},
		{"no link", ModeIM, "no link"},
	}
	for _, tt := range tests {
		got := stripLinks(tt.in, tt.mode)
		if got != tt.want {
			t.Errorf("stripLinks(%q, %v) = %q, want %q", tt.in, tt.mode, got, tt.want)
		}
	}
}

func TestStripEmojis(t *testing.T) {
	tests := []struct{ in, want string }{
		{"hello 😊", "hello "},
		{"🎉 party", " party"},
		{"no emoji", "no emoji"},
	}
	for _, tt := range tests {
		if got := stripEmojis(tt.in); got != tt.want {
			t.Errorf("stripEmojis(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeBullets(t *testing.T) {
	tests := []struct {
		in   string
		mode Mode
		want string
	}{
		{"- item", ModeIM, "• item"},
		{"- item", ModeVoice, "item"},
		{"* item", ModeIM, "• item"},
		{"+ item", ModeIM, "• item"},
	}
	for _, tt := range tests {
		got := normalizeBullets(tt.in, tt.mode)
		if got != tt.want {
			t.Errorf("normalizeBullets(%q, %v) = %q, want %q", tt.in, tt.mode, got, tt.want)
		}
	}
}

func TestNormalizeWhitespace(t *testing.T) {
	tests := []struct{ in, want string }{
		{"a\n\n\n\nb", "a\n\nb"},
		{"hello   \n", "hello"},
		{"\t\tindented", "indented"},
	}
	for _, tt := range tests {
		if got := normalizeWhitespace(tt.in); got != tt.want {
			t.Errorf("normalizeWhitespace(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestStripStrikethrough(t *testing.T) {
	tests := []struct{ in, want string }{
		{"~~deleted~~", "deleted"},
		{"keep ~~this~~ text", "keep this text"},
	}
	for _, tt := range tests {
		if got := stripStrikethrough(tt.in); got != tt.want {
			t.Errorf("stripStrikethrough(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestStripHTMLTags(t *testing.T) {
	tests := []struct{ in, want string }{
		{"<b>bold</b>", "bold"},
		{"<br>", ""},
		{"<a href=\"x\">link</a>", "link"},
	}
	for _, tt := range tests {
		if got := stripHTMLTags(tt.in); got != tt.want {
			t.Errorf("stripHTMLTags(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
