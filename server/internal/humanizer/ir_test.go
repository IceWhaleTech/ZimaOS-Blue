package humanizer

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		opts       ParseOptions
		wantText   string
		wantStyles []StyleSpan
		wantLinks  []LinkSpan
	}{
		{
			name:     "empty",
			input:    "",
			wantText: "",
		},
		{
			name:     "plain text",
			input:    "hello world",
			wantText: "hello world",
		},
		{
			name:     "bold",
			input:    "**hello** world",
			wantText: "hello world",
			wantStyles: []StyleSpan{
				{Start: 0, End: 5, Style: StyleBold},
			},
		},
		{
			name:     "bold underscore",
			input:    "__hello__ world",
			wantText: "hello world",
			wantStyles: []StyleSpan{
				{Start: 0, End: 5, Style: StyleBold},
			},
		},
		{
			name:     "italic star",
			input:    "*hello* world",
			wantText: "hello world",
			wantStyles: []StyleSpan{
				{Start: 0, End: 5, Style: StyleItalic},
			},
		},
		{
			name:     "italic underscore",
			input:    "_hello_ world",
			wantText: "hello world",
			wantStyles: []StyleSpan{
				{Start: 0, End: 5, Style: StyleItalic},
			},
		},
		{
			name:     "strikethrough",
			input:    "~~deleted~~ text",
			wantText: "deleted text",
			wantStyles: []StyleSpan{
				{Start: 0, End: 7, Style: StyleStrikethrough},
			},
		},
		{
			name:     "inline code",
			input:    "use `fmt.Println()` here",
			wantText: "use fmt.Println() here",
			wantStyles: []StyleSpan{
				{Start: 4, End: 17, Style: StyleCode},
			},
		},
		{
			name:     "double backtick inline code",
			input:    "use ``code with ` backtick`` here",
			wantText: "use code with ` backtick here",
			wantStyles: []StyleSpan{
				{Start: 4, End: 24, Style: StyleCode},
			},
		},
		{
			name:     "spoiler",
			input:    "this is ||hidden|| text",
			wantText: "this is hidden text",
			wantStyles: []StyleSpan{
				{Start: 8, End: 14, Style: StyleSpoiler},
			},
		},
		{
			name:     "link",
			input:    "[Google](https://google.com)",
			wantText: "Google",
			wantLinks: []LinkSpan{
				{Start: 0, End: 6, Href: "https://google.com"},
			},
		},
		{
			name:     "link with bold label",
			input:    "[**Bold Link**](https://example.com)",
			wantText: "Bold Link",
			wantStyles: []StyleSpan{
				{Start: 0, End: 9, Style: StyleBold},
			},
			wantLinks: []LinkSpan{
				{Start: 0, End: 9, Href: "https://example.com"},
			},
		},
		{
			name:     "image",
			input:    "![photo](https://img.com/a.png)",
			wantText: "(image: photo)",
		},
		{
			name:     "image empty alt",
			input:    "![](https://img.com/a.png)",
			wantText: "",
		},
		{
			name:     "heading h1",
			input:    "# Title",
			wantText: "Title",
		},
		{
			name:  "heading bold mode",
			input: "# Title",
			opts:  ParseOptions{HeadingStyle: "bold"},
			wantText: "Title",
			wantStyles: []StyleSpan{
				{Start: 0, End: 5, Style: StyleBold},
			},
		},
		{
			name:     "heading h3",
			input:    "### Sub Section",
			wantText: "Sub Section",
		},
		{
			name:     "blockquote",
			input:    "> This is quoted",
			wantText: "This is quoted",
		},
		{
			name:  "blockquote with prefix",
			input: "> This is quoted",
			opts:  ParseOptions{BlockquotePrefix: "> "},
			wantText: "> This is quoted",
		},
		{
			name:     "unordered list",
			input:    "- item one\n- item two",
			wantText: "• item one\n• item two",
		},
		{
			name:     "ordered list",
			input:    "1. first\n2. second",
			wantText: "1. first\n2. second",
		},
		{
			name:     "horizontal rule dashes",
			input:    "above\n\n---\n\nbelow",
			wantText: "above\n\nbelow",
		},
		{
			name:     "horizontal rule stars",
			input:    "above\n\n***\n\nbelow",
			wantText: "above\n\nbelow",
		},
		{
			name:     "code fence",
			input:    "```go\nfmt.Println()\n```",
			wantText: "fmt.Println()",
			wantStyles: []StyleSpan{
				{Start: 0, End: 13, Style: StyleCodeBlock},
			},
		},
		{
			name:     "code fence tilde",
			input:    "~~~\ncode here\n~~~",
			wantText: "code here",
			wantStyles: []StyleSpan{
				{Start: 0, End: 9, Style: StyleCodeBlock},
			},
		},
		{
			name:     "unclosed fence",
			input:    "```\ncode here",
			wantText: "code here",
			wantStyles: []StyleSpan{
				{Start: 0, End: 9, Style: StyleCodeBlock},
			},
		},
		{
			name:     "multi paragraph",
			input:    "first paragraph\n\nsecond paragraph",
			wantText: "first paragraph\n\nsecond paragraph",
		},
		{
			name:     "trailing spaces trimmed",
			input:    "hello   \nworld  ",
			wantText: "hello\nworld",
		},
		{
			name:     "multiple blank lines collapsed",
			input:    "a\n\n\n\n\nb",
			wantText: "a\n\nb",
		},
		{
			name:     "triple star not fully parsed",
			input:    "***bold italic***",
			wantText: "*bold italic*",
			// Current parser: ** matches bold, inner *...* has no closing match
			// This is a known limitation — triple *** as bold+italic is not supported
			wantStyles: []StyleSpan{
				{Start: 0, End: 12, Style: StyleBold},
			},
		},
		{
			name:     "bold with inline code",
			input:    "**bold** and `code`",
			wantText: "bold and code",
			wantStyles: []StyleSpan{
				{Start: 0, End: 4, Style: StyleBold},
				{Start: 9, End: 13, Style: StyleCode},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ir := Parse(tt.input, tt.opts)
			if ir.Text != tt.wantText {
				t.Errorf("Parse(%q).Text = %q, want %q", tt.input, ir.Text, tt.wantText)
			}
			if tt.wantStyles != nil {
				if len(ir.Styles) != len(tt.wantStyles) {
					t.Errorf("Parse(%q).Styles count = %d, want %d\n  got:  %+v\n  want: %+v",
						tt.input, len(ir.Styles), len(tt.wantStyles), ir.Styles, tt.wantStyles)
				} else {
					for i, want := range tt.wantStyles {
						got := ir.Styles[i]
						if got.Start != want.Start || got.End != want.End || got.Style != want.Style {
							t.Errorf("Parse(%q).Styles[%d] = %+v, want %+v", tt.input, i, got, want)
						}
					}
				}
			}
			if tt.wantLinks != nil {
				if len(ir.Links) != len(tt.wantLinks) {
					t.Errorf("Parse(%q).Links count = %d, want %d", tt.input, len(ir.Links), len(tt.wantLinks))
				} else {
					for i, want := range tt.wantLinks {
						got := ir.Links[i]
						if got.Start != want.Start || got.End != want.End || got.Href != want.Href {
							t.Errorf("Parse(%q).Links[%d] = %+v, want %+v", tt.input, i, got, want)
						}
					}
				}
			}
		})
	}
}

func TestParseTable(t *testing.T) {
	input := "| Name | Age |\n|------|-----|\n| Alice | 30 |\n| Bob | 25 |"

	t.Run("off", func(t *testing.T) {
		ir := Parse(input, ParseOptions{TableMode: "off"})
		if ir.Text != "Name | Age\nAlice | 30\nBob | 25" {
			t.Errorf("TableMode=off: got %q", ir.Text)
		}
	})

	t.Run("bullets", func(t *testing.T) {
		ir := Parse(input, ParseOptions{TableMode: "bullets"})
		// First cell bold, remaining cells as "Header: Value"
		if len(ir.Styles) == 0 {
			t.Error("TableMode=bullets: expected bold styles for first column")
		}
		// Should contain "• Age: 30"
		if !contains(ir.Text, "Age: 30") {
			t.Errorf("TableMode=bullets: expected 'Age: 30' in %q", ir.Text)
		}
	})

	t.Run("code", func(t *testing.T) {
		ir := Parse(input, ParseOptions{TableMode: "code"})
		// Should have a CodeBlock style
		hasCodeBlock := false
		for _, s := range ir.Styles {
			if s.Style == StyleCodeBlock {
				hasCodeBlock = true
			}
		}
		if !hasCodeBlock {
			t.Error("TableMode=code: expected CodeBlock style")
		}
	})
}

func TestSliceStyleSpans(t *testing.T) {
	spans := []StyleSpan{
		{Start: 5, End: 10, Style: StyleBold},
		{Start: 15, End: 20, Style: StyleItalic},
		{Start: 25, End: 30, Style: StyleCode},
	}

	// Slice [0, 12) — should include first span shifted
	result := SliceStyleSpans(spans, 0, 12)
	if len(result) != 1 {
		t.Fatalf("expected 1 span, got %d", len(result))
	}
	if result[0].Start != 5 || result[0].End != 10 || result[0].Style != StyleBold {
		t.Errorf("got %+v", result[0])
	}

	// Slice [10, 22) — should include second span shifted
	result = SliceStyleSpans(spans, 10, 22)
	if len(result) != 1 {
		t.Fatalf("expected 1 span, got %d", len(result))
	}
	if result[0].Start != 5 || result[0].End != 10 || result[0].Style != StyleItalic {
		t.Errorf("got %+v", result[0])
	}

	// Slice [7, 17) — should clip first span and include partial second
	result = SliceStyleSpans(spans, 7, 17)
	if len(result) != 2 {
		t.Fatalf("expected 2 spans, got %d: %+v", len(result), result)
	}
	if result[0].Start != 0 || result[0].End != 3 { // clipped bold
		t.Errorf("span[0] = %+v, want [0:3]", result[0])
	}
	if result[1].Start != 8 || result[1].End != 10 { // clipped italic
		t.Errorf("span[1] = %+v, want [8:10]", result[1])
	}
}

func TestSliceLinkSpans(t *testing.T) {
	spans := []LinkSpan{
		{Start: 5, End: 10, Href: "https://a.com"},
		{Start: 20, End: 25, Href: "https://b.com"},
	}

	result := SliceLinkSpans(spans, 0, 15)
	if len(result) != 1 {
		t.Fatalf("expected 1 link, got %d", len(result))
	}
	if result[0].Href != "https://a.com" {
		t.Errorf("got href %q", result[0].Href)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && searchSubstring(s, substr)))
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func BenchmarkParse(b *testing.B) {
	input := "## Title\n\n**Bold** text with `inline code` and [link](https://example.com).\n\n- item 1\n- item 2\n\n```go\nfmt.Println(\"hello\")\n```\n\n> A blockquote\n\n---\n\nMore text with ~~strikethrough~~ and emojis 😊🎉."

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Parse(input, ParseOptions{})
	}
}

func BenchmarkParseMinimal(b *testing.B) {
	input := "Hello world, this is a simple message."
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Parse(input, ParseOptions{})
	}
}
