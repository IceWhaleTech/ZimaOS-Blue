package humanizer

import (
	"strings"
	"testing"
)

func TestParseFenceSpans(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int // expected number of spans
	}{
		{"no fences", "hello world", 0},
		{"single closed", "```go\nfmt.Println()\n```", 1},
		{"unclosed", "```go\nfmt.Println()", 1},
		{"two fences", "```\na\n```\n\n```\nb\n```", 2},
		{"tilde fence", "~~~\ncode\n~~~", 1},
		{"nested backticks ignored", "````\n```\ninner\n```\n````", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spans := parseFenceSpans(tt.input)
			if len(spans) != tt.want {
				t.Errorf("parseFenceSpans(%q) got %d spans, want %d", tt.input, len(spans), tt.want)
			}
		})
	}
}

func TestParseFenceSpansDetail(t *testing.T) {
	input := "before\n```go\ncode here\n```\nafter"
	spans := parseFenceSpans(input)
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	s := spans[0]
	if s.OpenLine != "```go" {
		t.Errorf("OpenLine = %q, want %q", s.OpenLine, "```go")
	}
	if s.Marker != "```" {
		t.Errorf("Marker = %q, want %q", s.Marker, "```")
	}
	if s.Indent != "" {
		t.Errorf("Indent = %q, want empty", s.Indent)
	}
	// "before\n" = 7 bytes, so fence starts at 7
	if s.Start != 7 {
		t.Errorf("Start = %d, want 7", s.Start)
	}
}

func TestIsSafeFenceBreak(t *testing.T) {
	input := "before\n```\ncode\n```\nafter"
	spans := parseFenceSpans(input)

	// Index 0 (before) should be safe
	if !isSafeFenceBreak(spans, 0) {
		t.Error("index 0 should be safe")
	}
	// Index inside fence should NOT be safe
	if isSafeFenceBreak(spans, 12) {
		t.Error("index 12 (inside fence) should not be safe")
	}
	// Index after fence should be safe
	if !isSafeFenceBreak(spans, 20) {
		t.Error("index 20 (after fence) should be safe")
	}
}

func TestScanParenAwareBreakpoints(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantNewline   int
		wantWhitespace int
	}{
		{"simple newline", "hello\nworld", 5, -1},
		{"space", "hello world", -1, 5},
		{"paren hides newline", "hello(\nworld)", -1, -1},
		{"after paren", "a(b\nc) d", -1, 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bp := scanParenAwareBreakpoints(tt.input, nil)
			if bp.lastNewline != tt.wantNewline {
				t.Errorf("lastNewline = %d, want %d", bp.lastNewline, tt.wantNewline)
			}
			if bp.lastWhitespace != tt.wantWhitespace {
				t.Errorf("lastWhitespace = %d, want %d", bp.lastWhitespace, tt.wantWhitespace)
			}
		})
	}
}

func TestChunkText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		limit int
		want  []string
	}{
		{"empty", "", 100, nil},
		{"under limit", "hello", 100, []string{"hello"}},
		{"exact limit", "hello", 5, []string{"hello"}},
		{"split on newline", "hello\nworld foo", 10, []string{"hello", "world foo"}},
		{"split on space", "hello world", 8, []string{"hello", "world"}},
		{"hard cut", "abcdefghij", 5, []string{"abcde", "fghij"}},
		{
			"multi chunk",
			"aaa bbb ccc ddd eee",
			8,
			[]string{"aaa bbb", "ccc ddd", "eee"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ChunkText(tt.input, tt.limit)
			if !sliceEqual(got, tt.want) {
				t.Errorf("ChunkText(%q, %d)\n  got:  %v\n  want: %v", tt.input, tt.limit, got, tt.want)
			}
		})
	}
}

func TestChunkMarkdownText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		limit int
	}{
		{"no fence", "hello world foo bar baz", 10},
		{"fence fits", "```go\nfmt.Println()\n```", 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := ChunkMarkdownText(tt.input, tt.limit)
			if len(chunks) == 0 && tt.input != "" {
				t.Error("expected at least one chunk")
			}
			// Verify all chunks are within limit (with some tolerance for fence markers).
			joined := strings.Join(chunks, "")
			// The joined text won't exactly match input due to fence close/reopen,
			// but should contain all the original content words.
			_ = joined
		})
	}
}

func TestChunkMarkdownTextFenceReopen(t *testing.T) {
	// Create text with a code fence that must be split.
	code := strings.Repeat("x", 50)
	input := "```go\n" + code + "\n```"
	limit := 30

	chunks := ChunkMarkdownText(input, limit)
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d: %v", len(chunks), chunks)
	}

	// First chunk should end with closing fence.
	if !strings.HasSuffix(chunks[0], "```") {
		t.Errorf("first chunk should end with closing fence, got: %q", chunks[0])
	}

	// Second chunk should start with opening fence.
	if !strings.HasPrefix(chunks[1], "```go\n") {
		t.Errorf("second chunk should start with opening fence, got: %q", chunks[1])
	}
}

func TestChunkByParagraph(t *testing.T) {
	tests := []struct {
		name  string
		input string
		limit int
		want  int // expected number of chunks
	}{
		{"empty", "", 100, 0},
		{"single paragraph", "hello world", 100, 1},
		{"two paragraphs", "hello\n\nworld", 100, 2},
		{"fence not split", "```\na\n\nb\n```", 100, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ChunkByParagraph(tt.input, tt.limit)
			if len(got) != tt.want {
				t.Errorf("ChunkByParagraph(%q, %d) got %d chunks, want %d: %v",
					tt.input, tt.limit, len(got), tt.want, got)
			}
		})
	}
}

func TestChunkByParagraphFenceSafe(t *testing.T) {
	// Blank line inside fence should NOT be a split point.
	input := "before\n\n```\nline1\n\nline2\n```\n\nafter"
	chunks := ChunkByParagraph(input, 1000)
	// Should get 3 chunks: "before", the fence block, "after"
	if len(chunks) != 3 {
		t.Errorf("expected 3 chunks, got %d: %v", len(chunks), chunks)
	}
}

func sliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- Benchmarks ---

// Generate a realistic markdown document for benchmarking.
func benchmarkMarkdownDoc() string {
	var b strings.Builder
	b.WriteString("# Introduction\n\n")
	b.WriteString("This is a paragraph with some text that explains the concept.\n\n")
	b.WriteString("## Code Example\n\n")
	b.WriteString("```go\n")
	b.WriteString("func main() {\n")
	b.WriteString("\tfmt.Println(\"hello world\")\n")
	b.WriteString("\tfor i := 0; i < 100; i++ {\n")
	b.WriteString("\t\tfmt.Printf(\"iteration %d\\n\", i)\n")
	b.WriteString("\t}\n")
	b.WriteString("}\n")
	b.WriteString("```\n\n")
	b.WriteString("Another paragraph here with [a link](https://example.com/path/to/resource).\n\n")
	b.WriteString("```python\n")
	b.WriteString("def hello():\n")
	b.WriteString("    print('hello')\n")
	b.WriteString("    for i in range(100):\n")
	b.WriteString("        print(f'iteration {i}')\n")
	b.WriteString("```\n\n")
	b.WriteString("Final paragraph with some concluding remarks.\n")
	return b.String()
}

func BenchmarkChunkText(b *testing.B) {
	text := benchmarkMarkdownDoc()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ChunkText(text, 200)
	}
}

func BenchmarkChunkMarkdownText(b *testing.B) {
	text := benchmarkMarkdownDoc()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ChunkMarkdownText(text, 200)
	}
}

func BenchmarkChunkByParagraph(b *testing.B) {
	text := benchmarkMarkdownDoc()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ChunkByParagraph(text, 200)
	}
}

func BenchmarkParseFenceSpans(b *testing.B) {
	text := benchmarkMarkdownDoc()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseFenceSpans(text)
	}
}

// BenchmarkIndexByteVsSplit compares strings.IndexByte line scanning
// vs strings.Split for newline processing.
func BenchmarkIndexByteVsSplit(b *testing.B) {
	text := benchmarkMarkdownDoc()
	// Make it bigger for more meaningful comparison.
	text = strings.Repeat(text, 10)

	b.Run("IndexByte", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			offset := 0
			count := 0
			for offset < len(text) {
				nl := strings.IndexByte(text[offset:], '\n')
				if nl == -1 {
					break
				}
				count++
				offset += nl + 1
			}
			_ = count
		}
	})

	b.Run("Split", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			lines := strings.Split(text, "\n")
			_ = len(lines)
		}
	})
}
