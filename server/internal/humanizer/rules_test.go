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

func TestStripTypelessCards(t *testing.T) {
	searchCard := "```typeless\n{\"type\":\"search\",\"query\":\"golang generics\",\"results\":[{\"title\":\"Go Generics Tutorial\",\"url\":\"https://go.dev/doc/tutorial/generics\"},{\"title\":\"Generics in Go\",\"url\":\"https://example.com/go-generics\",\"description\":\"A deep dive\"}],\"total_count\":2}\n```"

	tests := []struct {
		name string
		in   string
		mode Mode
		want string
	}{
		{
			"search card IM",
			searchCard,
			ModeIM,
			"🔍 搜索「golang generics」找到 2 条结果：\n\n1. Go Generics Tutorial\n   https://go.dev/doc/tutorial/generics\n2. Generics in Go\n   https://example.com/go-generics",
		},
		{
			"search card voice",
			searchCard,
			ModeVoice,
			"(搜索结果已省略)",
		},
		{
			"unknown card type passthrough",
			"```typeless\n{\"type\":\"info\",\"content\":\"hello\"}\n```",
			ModeIM,
			"```typeless\n{\"type\":\"info\",\"content\":\"hello\"}\n```",
		},
		{
			"invalid JSON passthrough",
			"```typeless\nnot json\n```",
			ModeIM,
			"```typeless\nnot json\n```",
		},
		{
			"no typeless blocks",
			"plain text",
			ModeIM,
			"plain text",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripTypelessCards(tt.in, tt.mode)
			if got != tt.want {
				t.Errorf("stripTypelessCards() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestStripFunctionCalls(t *testing.T) {
	webSearchCall := `<function_calls>
<invoke name="web_search">
<parameter name="query">browser4 architecture diagram</parameter>
<parameter name="region">wt-wt</parameter>
<parameter name="max_results">10</parameter>
</invoke>
</function_calls>`

	calcCall := `<function_calls>
<invoke name="calculator">
<parameter name="expression">2+2</parameter>
</invoke>
</function_calls>`

	multiCall := `<function_calls>
<invoke name="web_search">
<parameter name="query">golang generics</parameter>
</invoke>
<invoke name="memory_search">
<parameter name="keyword">golang</parameter>
<parameter name="limit">5</parameter>
</invoke>
</function_calls>`

	tests := []struct {
		name string
		in   string
		mode Mode
		want string
	}{
		{
			"web search IM",
			webSearchCall,
			ModeIM,
			"🔧 网页搜索（browser4 architecture diagram，区域: wt-wt，结果条数: 10）",
		},
		{
			"web search voice",
			webSearchCall,
			ModeVoice,
			"(工具调用已省略)",
		},
		{
			"calculator IM",
			calcCall,
			ModeIM,
			"🔧 计算器（2+2）",
		},
		{
			"multi tool IM",
			multiCall,
			ModeIM,
			"🔧 网页搜索（golang generics）\n🔧 记忆搜索（golang，数量限制: 5）",
		},
		{
			"no function calls",
			"plain text",
			ModeIM,
			"plain text",
		},
		{
			"mixed content",
			"before\n" + calcCall + "\nafter",
			ModeIM,
			"before\n🔧 计算器（2+2）\nafter",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripFunctionCalls(tt.in, tt.mode)
			if got != tt.want {
				t.Errorf("stripFunctionCalls() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestCompactForIM(t *testing.T) {
	in := "Answer first.\n\n<!-- process-start -->\n```process\n[{\"tool\":\"exec\",\"cmd\":\"pwd\"}]\n```\n<!-- process-end -->\n\nDone."
	got := CompactForIM(in)
	want := "Answer first.\n\nDone."
	if got != want {
		t.Fatalf("CompactForIM() = %q, want %q", got, want)
	}
}
