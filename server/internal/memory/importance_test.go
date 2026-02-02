package memory

import (
	"testing"
	"time"
)

func TestImportanceScorer(t *testing.T) {
	scorer := NewImportanceScorer(DefaultImportanceConfig())

	t.Run("RecentMemoryHighScore", func(t *testing.T) {
		chunk := &MemoryChunk{
			Content:   "This is an important meeting note about the critical deadline.",
			CreatedAt: time.Now(),
		}
		score := scorer.Score(chunk)
		if score < 0.5 {
			t.Errorf("expected high score for recent important content, got %f", score)
		}
	})

	t.Run("OldMemoryLowerScore", func(t *testing.T) {
		chunk := &MemoryChunk{
			Content:   "Some random content without keywords.",
			CreatedAt: time.Now().AddDate(0, -3, 0), // 3 months ago
		}
		score := scorer.Score(chunk)
		if score > 0.5 {
			t.Errorf("expected lower score for old content without keywords, got %f", score)
		}
	})

	t.Run("KeywordBoost", func(t *testing.T) {
		chunkWithKeywords := &MemoryChunk{
			Content:   "Remember this critical password for the API token.",
			CreatedAt: time.Now().AddDate(0, 0, -10),
		}
		chunkWithoutKeywords := &MemoryChunk{
			Content:   "The weather is nice today.",
			CreatedAt: time.Now().AddDate(0, 0, -10),
		}

		scoreWith := scorer.Score(chunkWithKeywords)
		scoreWithout := scorer.Score(chunkWithoutKeywords)

		if scoreWith <= scoreWithout {
			t.Errorf("expected keyword content to score higher: %f vs %f", scoreWith, scoreWithout)
		}
	})
}

func TestImportanceScorer_ScoreWithDetails(t *testing.T) {
	scorer := NewImportanceScorer(DefaultImportanceConfig())

	chunk := &MemoryChunk{
		Content:   "This is a critical bug fix that needs immediate attention.",
		CreatedAt: time.Now(),
		Metadata:  map[string]string{"access_count": "5"},
	}

	details := scorer.ScoreWithDetails(chunk)

	if details.CombinedScore <= 0 {
		t.Error("expected positive combined score")
	}
	if details.RecencyScore < 0.9 {
		t.Errorf("expected high recency score for recent content, got %f", details.RecencyScore)
	}
	if len(details.MatchedKeywords) == 0 {
		t.Error("expected matched keywords")
	}

	// Check that "critical" and "bug" are matched
	found := make(map[string]bool)
	for _, kw := range details.MatchedKeywords {
		found[kw] = true
	}
	if !found["critical"] {
		t.Error("expected 'critical' in matched keywords")
	}
	if !found["bug"] {
		t.Error("expected 'bug' in matched keywords")
	}
}

func TestExtractImportantContent(t *testing.T) {
	content := `This is a normal sentence. Remember this important deadline for the project.
	The weather is nice. This critical bug needs to be fixed immediately.
	Some filler text here. The API token should be kept secret.`

	extracted := ExtractImportantContent(content, 2)

	if extracted == "" {
		t.Error("expected non-empty extracted content")
	}

	// Should contain important sentences
	if !containsAny(extracted, []string{"critical", "important", "secret"}) {
		t.Errorf("expected important keywords in extracted content: %s", extracted)
	}
}

func containsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
