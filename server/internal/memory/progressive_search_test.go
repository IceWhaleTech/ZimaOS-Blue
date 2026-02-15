package memory

import (
	"testing"
)

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		content string
		want    string
	}{
		{"First line\nSecond line", "First line"},
		{"Short", "Short"},
		{
			"This is a very long line that exceeds eighty characters and should be truncated at the boundary properly",
			"This is a very long line that exceeds eighty characters and should be truncated..." ,
		},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractTitle(tt.content)
		if got != tt.want {
			t.Errorf("extractTitle(%q) = %q, want %q", tt.content[:min(20, len(tt.content))], got, tt.want)
		}
	}
}

func TestExtractSnippet(t *testing.T) {
	content := "The quick brown fox jumps over the lazy dog. This is a test of snippet extraction around keyword matches."

	// With matching query
	snippet := extractSnippet(content, "fox", 40)
	if len(snippet) == 0 {
		t.Fatal("expected non-empty snippet")
	}

	// Without query
	snippet = extractSnippet(content, "", 20)
	if len(snippet) > 25 { // 20 + "..."
		t.Errorf("expected snippet <= 25 chars, got %d", len(snippet))
	}

	// Empty content
	snippet = extractSnippet("", "query", 100)
	if snippet != "" {
		t.Errorf("expected empty snippet for empty content, got %q", snippet)
	}
}

func TestExtractContextLines(t *testing.T) {
	content := "line1\nline2\nline3 match\nline4\nline5\nline6"

	lines := extractContextLines(content, "match", 1)
	if len(lines) == 0 {
		t.Fatal("expected context lines")
	}
	// Should include line before and after match
	found := false
	for _, l := range lines {
		if l == "line3 match" {
			found = true
		}
	}
	if !found {
		t.Error("expected to find matching line in context")
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		content string
		want    int
	}{
		{"", 0},
		{"hi", 1},
		{"hello world this is a test", 6}, // 26/4 = 6
	}
	for _, tt := range tests {
		got := estimateTokens(tt.content)
		if got != tt.want {
			t.Errorf("estimateTokens(%q) = %d, want %d", tt.content, got, tt.want)
		}
	}
}

func TestInferType(t *testing.T) {
	if got := inferType(map[string]string{"type": "decision"}); got != "decision" {
		t.Errorf("expected 'decision', got %q", got)
	}
	if got := inferType(map[string]string{"layer": "daily"}); got != "daily" {
		t.Errorf("expected 'daily', got %q", got)
	}
	if got := inferType(map[string]string{}); got != "memory" {
		t.Errorf("expected 'memory', got %q", got)
	}
}

func TestExtractTags(t *testing.T) {
	meta := map[string]string{
		"tag_0": "golang",
		"tag_1": "memory",
		"other": "ignored",
	}
	tags := extractTags(meta)
	if len(tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(tags))
	}
}

func TestIndexResultTokenEstimate(t *testing.T) {
	results := []IndexResult{
		{ID: "1", Title: "Short title", Score: 0.9, TokenEst: 100},
		{ID: "2", Title: "Another title", Score: 0.8, TokenEst: 200},
	}
	tokens := estimateIndexTokens(results)
	// Each entry ~30 + title/4 tokens
	if tokens < 60 || tokens > 200 {
		t.Errorf("unexpected token estimate: %d", tokens)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
