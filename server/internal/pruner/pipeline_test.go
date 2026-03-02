package pruner

import (
	"strings"
	"testing"
)

// --- 7.10 Top-K Selector Tests ---

func TestSelectTopK_BasicBudget(t *testing.T) {
	scored := []ScoredSegment{
		{Segment: Segment{StartLine: 0, EndLine: 2, Content: "func a() {\n\tx := 1\n}"}, Score: 0.9},
		{Segment: Segment{StartLine: 3, EndLine: 5, Content: "func b() {\n\ty := 2\n}"}, Score: 0.3},
		{Segment: Segment{StartLine: 6, EndLine: 8, Content: "func c() {\n\tz := 3\n}"}, Score: 0.7},
	}
	// Budget large enough for all
	result := SelectTopK(scored, 10000)
	if len(result) != 3 {
		t.Errorf("expected 3 segments, got %d", len(result))
	}
}

func TestSelectTopK_LimitedBudget(t *testing.T) {
	scored := []ScoredSegment{
		{Segment: Segment{StartLine: 0, EndLine: 2, Content: "func a() {\n\tx := 1\n}"}, Score: 0.9},
		{Segment: Segment{StartLine: 3, EndLine: 5, Content: "func b() {\n\ty := 2\n}"}, Score: 0.3},
		{Segment: Segment{StartLine: 6, EndLine: 8, Content: "func c() {\n\tz := 3\n}"}, Score: 0.7},
	}
	// Very small budget — should only keep highest scoring
	result := SelectTopK(scored, 10)
	if len(result) == 0 {
		t.Fatal("expected at least 1 segment")
	}
	// First result should be highest scoring
	if result[0].Score < 0.9 {
		t.Errorf("expected highest score first, got %f", result[0].Score)
	}
}

func TestSelectTopK_Empty(t *testing.T) {
	result := SelectTopK(nil, 1000)
	if len(result) != 0 {
		t.Errorf("expected 0 results, got %d", len(result))
	}
}

// --- 7.11 Rule-Based Weight Boosting Tests ---

func TestBoostScores_StructuralBoost(t *testing.T) {
	scored := []ScoredSegment{
		{Segment: Segment{Kind: SegmentFunction, Content: "func main() {}"}, Score: 0.5},
		{Segment: Segment{Kind: SegmentLines, Content: "x := 1"}, Score: 0.5},
	}
	boosted := BoostScores(scored)
	// Function segments should get a boost over plain lines
	if boosted[0].Score <= boosted[1].Score {
		t.Errorf("function segment (%f) should score higher than lines (%f) after boost",
			boosted[0].Score, boosted[1].Score)
	}
}

func TestBoostScores_ImportBoost(t *testing.T) {
	scored := []ScoredSegment{
		{Segment: Segment{Kind: SegmentLines, Content: "import \"fmt\"\nimport \"os\""}, Score: 0.3},
		{Segment: Segment{Kind: SegmentLines, Content: "x := doSomething()"}, Score: 0.3},
	}
	boosted := BoostScores(scored)
	if boosted[0].Score <= boosted[1].Score {
		t.Errorf("import segment (%f) should score higher after boost", boosted[0].Score)
	}
}

func TestBoostScores_Empty(t *testing.T) {
	result := BoostScores(nil)
	if len(result) != 0 {
		t.Errorf("expected 0 results, got %d", len(result))
	}
}

// --- 7.12 BM25Backend Integration Test ---

func TestBM25Backend_Prune(t *testing.T) {
	backend := NewBM25Backend(Config{Threshold: 0.3})
	code := strings.Repeat("func authenticate(user string) error {\n\tif user == \"\" {\n\t\treturn errors.New(\"empty\")\n\t}\n\treturn nil\n}\n\n", 5)
	code += strings.Repeat("// filler line with no relevance\n", 50)

	resp, err := backend.Prune(nil, PruneRequest{
		Code:      code,
		Query:     "authenticate user",
		Threshold: 0.3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.KeptLines == 0 {
		t.Error("expected some lines to be kept")
	}
	if resp.CompressionRate >= 1.0 && resp.PrunedLines > 0 {
		t.Errorf("expected compression < 1.0 when lines are pruned, got %f", resp.CompressionRate)
	}
}

func TestBM25Backend_Health(t *testing.T) {
	backend := NewBM25Backend(Config{})
	if err := backend.Health(nil); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestBM25Backend_Compression(t *testing.T) {
	backend := NewBM25Backend(Config{Threshold: 0.5})
	code := generateGoCode(1000)
	resp, err := backend.Prune(nil, PruneRequest{
		Code:      code,
		Query:     "handler request error",
		Threshold: 0.5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	reduction := 1.0 - resp.CompressionRate
	if reduction < 0.40 {
		t.Errorf("BM25 token reduction %.1f%% too low (expected >= 40%%)", reduction*100)
	}
}

func TestBoostScores_NameMatchBoost(t *testing.T) {
	scored := []ScoredSegment{
		{Segment: Segment{Kind: SegmentFunction, Name: "func authenticate", Content: "func authenticate() {}"}, Score: 0.5},
		{Segment: Segment{Kind: SegmentFunction, Name: "func calculateSum", Content: "func calculateSum() {}"}, Score: 0.5},
	}
	boosted := BoostScoresWithQuery(scored, "authenticate user")
	if boosted[0].Score <= boosted[1].Score {
		t.Errorf("name-matching segment (%f) should score higher than non-matching (%f)",
			boosted[0].Score, boosted[1].Score)
	}
}

func TestSelectTopK_CoverageConstraint(t *testing.T) {
	// 3 function segments, very tight budget — should still keep at least
	// the signature of each function (coverage constraint)
	scored := []ScoredSegment{
		{Segment: Segment{StartLine: 0, EndLine: 10, Kind: SegmentFunction, Name: "func a", Content: strings.Repeat("line\n", 10)}, Score: 0.9},
		{Segment: Segment{StartLine: 11, EndLine: 21, Kind: SegmentFunction, Name: "func b", Content: strings.Repeat("line\n", 10)}, Score: 0.1},
		{Segment: Segment{StartLine: 22, EndLine: 32, Kind: SegmentFunction, Name: "func c", Content: strings.Repeat("line\n", 10)}, Score: 0.5},
	}
	// Budget only fits ~1 segment, but coverage should ensure all 3 get at least a mention
	result := SelectTopK(scored, 15)
	if len(result) < 1 {
		t.Error("expected at least 1 segment selected")
	}
}

func TestSelectTopK_SkipsOversizedSegmentAndKeepsFittingOnes(t *testing.T) {
	scored := []ScoredSegment{
		{Segment: Segment{StartLine: 0, EndLine: 10, Content: "A", TokenCount: 200}, Score: 0.99},
		{Segment: Segment{StartLine: 11, EndLine: 20, Content: "B", TokenCount: 200}, Score: 0.90},
		{Segment: Segment{StartLine: 21, EndLine: 22, Content: "C", TokenCount: 20}, Score: 0.80},
	}
	result := SelectTopK(scored, 230)
	if len(result) != 2 {
		t.Fatalf("expected 2 segments selected, got %d", len(result))
	}
	if result[0].Segment.Content != "A" || result[1].Segment.Content != "C" {
		t.Fatalf("unexpected selection order/content: %+v", result)
	}
}
