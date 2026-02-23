package humanizer

import (
	"testing"
)

func TestRenderDiscord(t *testing.T) {
	tests := []struct {
		name string
		ir   IR
		want string
	}{
		{
			name: "plain text",
			ir:   IR{Text: "hello world"},
			want: "hello world",
		},
		{
			name: "bold",
			ir: IR{
				Text:   "hello world",
				Styles: []StyleSpan{{Start: 0, End: 5, Style: StyleBold}},
			},
			want: "**hello** world",
		},
		{
			name: "italic",
			ir: IR{
				Text:   "hello world",
				Styles: []StyleSpan{{Start: 0, End: 5, Style: StyleItalic}},
			},
			want: "*hello* world",
		},
		{
			name: "strikethrough",
			ir: IR{
				Text:   "deleted text",
				Styles: []StyleSpan{{Start: 0, End: 7, Style: StyleStrikethrough}},
			},
			want: "~~deleted~~ text",
		},
		{
			name: "inline code",
			ir: IR{
				Text:   "use fmt.Println() here",
				Styles: []StyleSpan{{Start: 4, End: 17, Style: StyleCode}},
			},
			want: "use `fmt.Println()` here",
		},
		{
			name: "code block",
			ir: IR{
				Text:   "some code\n",
				Styles: []StyleSpan{{Start: 0, End: 10, Style: StyleCodeBlock}},
			},
			want: "```\nsome code\n```",
		},
		{
			name: "spoiler",
			ir: IR{
				Text:   "this is hidden text",
				Styles: []StyleSpan{{Start: 8, End: 14, Style: StyleSpoiler}},
			},
			want: "this is ||hidden|| text",
		},
		{
			name: "link",
			ir: IR{
				Text:  "click here for info",
				Links: []LinkSpan{{Start: 6, End: 10, Href: "https://example.com"}},
			},
			want: "click [here](" + "https://example.com) for info",
		},
		{
			name: "link same as label",
			ir: IR{
				Text:  "https://example.com",
				Links: []LinkSpan{{Start: 0, End: 19, Href: "https://example.com"}},
			},
			want: "https://example.com", // no markup for bare URL
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderDiscord(tt.ir)
			if got != tt.want {
				t.Errorf("RenderDiscord() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderTelegram(t *testing.T) {
	tests := []struct {
		name string
		ir   IR
		want string
	}{
		{
			name: "bold",
			ir: IR{
				Text:   "hello world",
				Styles: []StyleSpan{{Start: 0, End: 5, Style: StyleBold}},
			},
			want: "<b>hello</b> world",
		},
		{
			name: "html escaping",
			ir:   IR{Text: "a < b & c > d"},
			want: "a &lt; b &amp; c &gt; d",
		},
		{
			name: "link",
			ir: IR{
				Text:  "click here",
				Links: []LinkSpan{{Start: 0, End: 5, Href: "https://example.com"}},
			},
			want: `<a href="https://example.com">click</a> here`,
		},
		{
			name: "link with special chars in href",
			ir: IR{
				Text:  "click",
				Links: []LinkSpan{{Start: 0, End: 5, Href: "https://example.com/a&b"}},
			},
			want: `<a href="https://example.com/a&amp;b">click</a>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderTelegram(tt.ir)
			if got != tt.want {
				t.Errorf("RenderTelegram() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderSlack(t *testing.T) {
	tests := []struct {
		name string
		ir   IR
		want string
	}{
		{
			name: "bold",
			ir: IR{
				Text:   "hello world",
				Styles: []StyleSpan{{Start: 0, End: 5, Style: StyleBold}},
			},
			want: "*hello* world",
		},
		{
			name: "italic",
			ir: IR{
				Text:   "hello world",
				Styles: []StyleSpan{{Start: 0, End: 5, Style: StyleItalic}},
			},
			want: "_hello_ world",
		},
		{
			name: "mrkdwn escaping",
			ir:   IR{Text: "a < b & c > d"},
			want: "a &lt; b &amp; c &gt; d",
		},
		{
			name: "link",
			ir: IR{
				Text:  "click here",
				Links: []LinkSpan{{Start: 0, End: 5, Href: "https://example.com"}},
			},
			want: "<https://example.com|click> here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderSlack(tt.ir)
			if got != tt.want {
				t.Errorf("RenderSlack() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderPlain(t *testing.T) {
	tests := []struct {
		name string
		ir   IR
		want string
	}{
		{
			name: "no links",
			ir:   IR{Text: "hello world"},
			want: "hello world",
		},
		{
			name: "with link appends url",
			ir: IR{
				Text:  "Google",
				Links: []LinkSpan{{Start: 0, End: 6, Href: "https://google.com"}},
			},
			want: "Google (https://google.com)",
		},
		{
			name: "link same as label",
			ir: IR{
				Text:  "https://google.com",
				Links: []LinkSpan{{Start: 0, End: 18, Href: "https://google.com"}},
			},
			want: "https://google.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderPlain(tt.ir)
			if got != tt.want {
				t.Errorf("RenderPlain() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderVoice(t *testing.T) {
	tests := []struct {
		name string
		ir   IR
		want string
	}{
		{
			name: "plain text",
			ir:   IR{Text: "hello world"},
			want: "hello world",
		},
		{
			name: "code block replaced",
			ir: IR{
				Text:   "before\nsome code\nafter",
				Styles: []StyleSpan{{Start: 7, End: 16, Style: StyleCodeBlock}},
			},
			want: "before\n(code omitted)\nafter",
		},
		{
			name: "image stripped",
			ir:   IR{Text: "(image: photo)"},
			want: "",
		},
		{
			name: "bullets stripped",
			ir:   IR{Text: "• item one\n• item two"},
			want: "item one\nitem two",
		},
		{
			name: "emoji stripped",
			ir:   IR{Text: "Hello 😊 world"},
			want: "Hello  world",
		},
		{
			name: "empty",
			ir:   IR{Text: ""},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderVoice(tt.ir)
			if got != tt.want {
				t.Errorf("RenderVoice() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderMatrix(t *testing.T) {
	ir := IR{
		Text:   "hello world",
		Styles: []StyleSpan{{Start: 0, End: 5, Style: StyleBold}},
	}
	got := RenderMatrix(ir)
	want := "<strong>hello</strong> world"
	if got != want {
		t.Errorf("RenderMatrix() = %q, want %q", got, want)
	}
}

func BenchmarkRenderDiscord(b *testing.B) {
	ir := Parse("## Title\n\n**Bold** text with `inline code` and [link](https://example.com).\n\n- item 1\n- item 2", ParseOptions{HeadingStyle: "bold"})
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		RenderDiscord(ir)
	}
}

func BenchmarkRenderTelegram(b *testing.B) {
	ir := Parse("## Title\n\n**Bold** text with `inline code` and [link](https://example.com).\n\n- item 1\n- item 2", ParseOptions{HeadingStyle: "bold"})
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		RenderTelegram(ir)
	}
}

func BenchmarkRenderPlain(b *testing.B) {
	ir := Parse("## Title\n\n**Bold** text with `inline code` and [link](https://example.com).\n\n- item 1\n- item 2", ParseOptions{})
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		RenderPlain(ir)
	}
}

func BenchmarkRenderVoice(b *testing.B) {
	ir := Parse("## Title\n\n**Bold** text with `inline code` and [link](https://example.com).\n\n```go\nfmt.Println()\n```\n\n- item 1\n- item 2", ParseOptions{})
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		RenderVoice(ir)
	}
}
