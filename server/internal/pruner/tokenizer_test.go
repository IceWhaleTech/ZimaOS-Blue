package pruner

import (
	"strings"
	"testing"
)

func TestTextTokenize_Basic(t *testing.T) {
	tokens := TextTokenize("The quick brown fox jumps over the lazy dog")
	// Stopwords "the", "over" should be removed
	if containsToken(tokens, "the") {
		t.Error("stopword 'the' should be removed")
	}
	if !containsToken(tokens, "quick") {
		t.Error("expected 'quick' in tokens")
	}
	if !containsToken(tokens, "fox") {
		t.Error("expected 'fox' in tokens")
	}
}

func TestTextTokenize_Empty(t *testing.T) {
	tokens := TextTokenize("")
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens for empty input, got %d", len(tokens))
	}
}

func TestTextTokenize_StopwordRemoval(t *testing.T) {
	tokens := TextTokenize("this is a test of the system")
	// "this", "is", "a", "of", "the" are stopwords
	for _, sw := range []string{"this", "is", "of", "the"} {
		if containsToken(tokens, sw) {
			t.Errorf("stopword '%s' should be removed", sw)
		}
	}
	if !containsToken(tokens, "test") {
		t.Error("expected 'test' in tokens")
	}
	if !containsToken(tokens, "system") {
		t.Error("expected 'system' in tokens")
	}
}

func TestUnifiedTokenize_Code(t *testing.T) {
	tokens := UnifiedTokenize("func authenticateUser(login string)", ContentCode)
	if !containsToken(tokens, "authenticate") {
		t.Error("expected 'authenticate' from camelCase split")
	}
	if !containsToken(tokens, "user") {
		t.Error("expected 'user' from camelCase split")
	}
}

func TestUnifiedTokenize_Doc(t *testing.T) {
	tokens := UnifiedTokenize("The authentication system handles user login", ContentDoc)
	if containsToken(tokens, "the") {
		t.Error("stopword 'the' should be removed for doc content")
	}
	if !containsToken(tokens, "authentication") {
		t.Error("expected 'authentication' in tokens")
	}
}

func TestSplitSentences_Basic(t *testing.T) {
	sentences := SplitSentences("Hello world. This is a test. And another one.")
	if len(sentences) < 2 {
		t.Errorf("expected at least 2 sentences, got %d: %v", len(sentences), sentences)
	}
}

func TestSplitSentences_PreservesEmails(t *testing.T) {
	sentences := SplitSentences("My email is test@example.com and I like Go")
	// Should NOT split on the dot in the email
	found := false
	for _, s := range sentences {
		if containsSubstring(s, "test@example.com") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("email should be preserved intact, got sentences: %v", sentences)
	}
}

func TestSplitSentences_Chinese(t *testing.T) {
	sentences := SplitSentences("你好世界。这是一个测试。再来一个。")
	if len(sentences) < 2 {
		t.Errorf("expected at least 2 Chinese sentences, got %d: %v", len(sentences), sentences)
	}
}

func TestSplitSentences_Newlines(t *testing.T) {
	sentences := SplitSentences("Line one\nLine two\nLine three")
	if len(sentences) != 3 {
		t.Errorf("expected 3 sentences from newlines, got %d: %v", len(sentences), sentences)
	}
}

func TestSignalWordScore_Matches(t *testing.T) {
	score := SignalWordScore("I prefer Go for backend", DefaultMemorySignals)
	if score == 0 {
		t.Error("expected non-zero score for text with signal word 'prefer'")
	}
}

func TestSignalWordScore_NoMatch(t *testing.T) {
	score := SignalWordScore("The weather is nice today", DefaultMemorySignals)
	if score != 0 {
		t.Errorf("expected zero score for text without signal words, got %f", score)
	}
}

func TestSignalWordScore_Chinese(t *testing.T) {
	score := SignalWordScore("记住我的密码是abc123", DefaultMemorySignals)
	if score == 0 {
		t.Error("expected non-zero score for Chinese text with signal words")
	}
}

func TestSignalWordScore_DiminishingReturns(t *testing.T) {
	// More signal words should give higher score but with diminishing returns
	score1 := SignalWordScore("I prefer this", DefaultMemorySignals)
	score2 := SignalWordScore("I prefer and love and always remember this", DefaultMemorySignals)
	if score2 <= score1 {
		t.Errorf("more signal words should give higher score: 1-word=%f, multi-word=%f", score1, score2)
	}
	if score2 >= 1.0 {
		t.Errorf("score should be < 1.0 due to diminishing returns, got %f", score2)
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && strings.Contains(s, sub)
}

