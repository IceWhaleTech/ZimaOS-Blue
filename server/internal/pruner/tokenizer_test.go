package pruner

import (
	"testing"
)

func TestTextTokenize_Basic(t *testing.T) {
	tokens := textTokenize("The quick brown fox jumps over the lazy dog")
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
	tokens := textTokenize("")
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens for empty input, got %d", len(tokens))
	}
}

func TestTextTokenize_StopwordRemoval(t *testing.T) {
	tokens := textTokenize("this is a test of the system")
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
