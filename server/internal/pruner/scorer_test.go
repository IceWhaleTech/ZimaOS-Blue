package pruner

import (
	"context"
	"strings"
	"testing"
)

func TestLocalBackend_Prune_Basic(t *testing.T) {
	backend := NewLocalBackend(Config{Threshold: 0.5})

	// Generate a file with structural lines + filler comments
	var sb strings.Builder
	sb.WriteString("package main\n\n")
	sb.WriteString("import \"fmt\"\n\n")
	sb.WriteString("func main() {\n")
	for i := 0; i < 50; i++ {
		sb.WriteString("    // this is a filler comment line\n")
	}
	sb.WriteString("    fmt.Println(\"hello\")\n")
	sb.WriteString("}\n")

	resp, err := backend.Prune(context.Background(), PruneRequest{
		Code:      sb.String(),
		Query:     "main function println",
		Threshold: 0.5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.OriginalLines <= resp.KeptLines {
		t.Logf("original=%d kept=%d pruned=%d", resp.OriginalLines, resp.KeptLines, resp.PrunedLines)
	}
	if resp.PrunedLines == 0 {
		t.Error("expected some lines to be pruned")
	}
	if !strings.Contains(resp.PrunedCode, "package main") {
		t.Error("structural line 'package main' should be kept")
	}
	if !strings.Contains(resp.PrunedCode, "func main()") {
		t.Error("structural line 'func main()' should be kept")
	}
	if !strings.Contains(resp.PrunedCode, "(filtered") {
		t.Error("expected '(filtered N lines)' marker in output")
	}
	if resp.CompressionRate >= 1.0 {
		t.Errorf("expected compression < 1.0, got %f", resp.CompressionRate)
	}
	if resp.LatencyMs <= 0 {
		t.Error("expected positive latency")
	}
}

func TestLocalBackend_Prune_NoQuery(t *testing.T) {
	backend := NewLocalBackend(Config{Threshold: 0.5})

	code := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"
	resp, err := backend.Prune(context.Background(), PruneRequest{
		Code:      code,
		Threshold: 0.5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Short file, most lines are structural — should keep most
	if resp.KeptLines == 0 {
		t.Error("expected some lines to be kept")
	}
}

func TestLocalBackend_Prune_EmptyCode(t *testing.T) {
	backend := NewLocalBackend(Config{Threshold: 0.5})
	resp, err := backend.Prune(context.Background(), PruneRequest{Code: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.PrunedCode != "" {
		t.Errorf("expected empty pruned code, got %q", resp.PrunedCode)
	}
}

func TestLocalBackend_Health(t *testing.T) {
	backend := NewLocalBackend(Config{})
	if err := backend.Health(context.Background()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestScoreLine_StructuralKeywords(t *testing.T) {
	queryTerms := tokenize("")
	tests := []struct {
		line     string
		minScore float64
	}{
		{"package main", 0.9},
		{"func main() {", 0.9},
		{"import \"fmt\"", 0.8},
		{"type Foo struct {", 0.9},
		{"}", 0.8},
		{"    // just a comment", 0.3},
		{"", 0.2},
	}
	for _, tt := range tests {
		score := scoreLine(tt.line, queryTerms, 50, 100)
		if score < tt.minScore {
			t.Errorf("scoreLine(%q) = %f, want >= %f", tt.line, score, tt.minScore)
		}
	}
}

func TestScoreLine_QueryRelevance(t *testing.T) {
	queryTerms := tokenize("authentication login user")
	// Line containing query terms should score higher
	relevant := scoreLine("func authenticateUser(login string) error {", queryTerms, 50, 100)
	irrelevant := scoreLine("    x := calculateSum(a, b)", queryTerms, 50, 100)
	if relevant <= irrelevant {
		t.Errorf("relevant line (%f) should score higher than irrelevant (%f)", relevant, irrelevant)
	}
}

func TestScoreLine_CamelCaseQueryExpansion(t *testing.T) {
	// Query "authenticateUser" should match line containing "authenticate" after expansion
	queryTerms := tokenize("authenticateUser")
	// Non-structural line — only query relevance should boost it
	relevant := scoreLine("    result := authenticate(userInput)", queryTerms, 50, 100)
	irrelevant := scoreLine("    result := calculateTotal(items)", queryTerms, 50, 100)
	if relevant <= irrelevant {
		t.Errorf("camelCase expanded query should boost matching line: relevant=%f irrelevant=%f", relevant, irrelevant)
	}
}

func TestTokenize(t *testing.T) {
	terms := tokenize("func main() { fmt.Println }")
	if _, ok := terms["func"]; !ok {
		t.Error("expected 'func' in tokens")
	}
	if _, ok := terms["main"]; !ok {
		t.Error("expected 'main' in tokens")
	}
	if _, ok := terms["fmt"]; !ok {
		t.Error("expected 'fmt' in tokens")
	}
	// Single-char tokens should be excluded
	if _, ok := terms["{"]; ok {
		t.Error("single-char '{' should be excluded")
	}
}
