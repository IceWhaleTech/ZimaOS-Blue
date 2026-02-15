package pruner

import (
	"testing"
)

// --- 7.2 Code Tokenizer Tests ---

func TestCodeTokenize_CamelCase(t *testing.T) {
	tokens := codeTokenize("authenticateUser")
	expect := map[string]bool{"authenticate": true, "user": true}
	for tok := range expect {
		if !containsToken(tokens, tok) {
			t.Errorf("expected token %q in %v", tok, tokens)
		}
	}
}

func TestCodeTokenize_SnakeCase(t *testing.T) {
	tokens := codeTokenize("get_user_name")
	expect := map[string]bool{"get": true, "user": true, "name": true}
	for tok := range expect {
		if !containsToken(tokens, tok) {
			t.Errorf("expected token %q in %v", tok, tokens)
		}
	}
}

func TestCodeTokenize_MixedCase(t *testing.T) {
	tokens := codeTokenize("parseHTTPResponse_v2")
	// Should split: parse, http, response, v2
	expect := map[string]bool{"parse": true, "http": true, "response": true}
	for tok := range expect {
		if !containsToken(tokens, tok) {
			t.Errorf("expected token %q in %v", tok, tokens)
		}
	}
}

func TestCodeTokenize_Empty(t *testing.T) {
	tokens := codeTokenize("")
	if len(tokens) != 0 {
		t.Errorf("expected empty tokens, got %v", tokens)
	}
}

func TestCodeTokenize_Sentence(t *testing.T) {
	tokens := codeTokenize("func main() { fmt.Println }")
	if !containsToken(tokens, "func") {
		t.Error("expected 'func' in tokens")
	}
	if !containsToken(tokens, "main") {
		t.Error("expected 'main' in tokens")
	}
}

// --- 7.1 BM25 Scorer Tests ---

func TestBM25Scorer_Score_BasicRelevance(t *testing.T) {
	scorer := NewBM25Scorer(1.2, 0.75)
	segments := []Segment{
		{StartLine: 0, EndLine: 5, Kind: SegmentFunction, Name: "func authenticate", Content: "func authenticate(user string) error {\n\tif user == \"\" {\n\t\treturn errors.New(\"empty user\")\n\t}\n\treturn nil\n}", Tokens: codeTokenize("func authenticate user string error empty return nil")},
		{StartLine: 6, EndLine: 10, Kind: SegmentFunction, Name: "func calculateSum", Content: "func calculateSum(a, b int) int {\n\treturn a + b\n}", Tokens: codeTokenize("func calculateSum int return")},
	}

	results := scorer.Score("authenticate user login", segments)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// authenticate segment should score higher than calculateSum
	if results[0].Score <= results[1].Score {
		t.Errorf("authenticate (%f) should score higher than calculateSum (%f)", results[0].Score, results[1].Score)
	}
}

func TestBM25Scorer_Score_EmptyQuery(t *testing.T) {
	scorer := NewBM25Scorer(1.2, 0.75)
	segments := []Segment{
		{StartLine: 0, EndLine: 3, Kind: SegmentFunction, Content: "func foo() {}", Tokens: codeTokenize("func foo")},
	}
	results := scorer.Score("", segments)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// Empty query should give zero score
	if results[0].Score != 0 {
		t.Errorf("expected 0 score for empty query, got %f", results[0].Score)
	}
}

func TestBM25Scorer_Score_NoSegments(t *testing.T) {
	scorer := NewBM25Scorer(1.2, 0.75)
	results := scorer.Score("anything", nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestBM25Scorer_Score_TermFrequencySaturation(t *testing.T) {
	scorer := NewBM25Scorer(1.2, 0.75)
	// Segment with repeated term should not score proportionally higher
	seg1 := Segment{StartLine: 0, EndLine: 1, Kind: SegmentLines, Content: "auth auth auth auth auth", Tokens: codeTokenize("auth auth auth auth auth")}
	seg2 := Segment{StartLine: 2, EndLine: 3, Kind: SegmentLines, Content: "auth", Tokens: codeTokenize("auth")}

	results := scorer.Score("auth", []Segment{seg1, seg2})
	// Due to TF saturation, 5x repetition should NOT give 5x score
	ratio := results[0].Score / results[1].Score
	if ratio > 3.0 {
		t.Errorf("TF saturation not working: ratio %f (expected < 3.0)", ratio)
	}
}

// containsToken checks if a token list contains a specific token.
func containsToken(tokens []string, target string) bool {
	for _, t := range tokens {
		if t == target {
			return true
		}
	}
	return false
}
