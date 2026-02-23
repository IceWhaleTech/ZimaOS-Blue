package memory

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
)

func TestExtractFromConversation_Basic(t *testing.T) {
	turns := []ConversationTurn{
		{Role: "user", Content: "I prefer using Go for backend development and Python for data science."},
		{Role: "assistant", Content: "That's a solid combination. Go is great for performance-critical services."},
	}

	result := ExtractFromConversation(turns, 5)
	if result == nil {
		t.Fatal("expected non-nil result for conversation with preferences")
	}
	if len(result.KeySentences) == 0 {
		t.Fatal("expected at least one key sentence")
	}

	formatted := result.FormatAsMemory()
	if formatted == "" {
		t.Fatal("expected non-empty formatted memory")
	}
	t.Logf("Extracted: %s", formatted)
}

func TestExtractFromConversation_Chinese(t *testing.T) {
	turns := []ConversationTurn{
		{Role: "user", Content: "我喜欢用Go写后端，Python做数据分析。记住我的邮箱是test@example.com"},
		{Role: "assistant", Content: "好的，我记住了。Go确实很适合后端开发。"},
	}

	result := ExtractFromConversation(turns, 5)
	if result == nil {
		t.Fatal("expected non-nil result for Chinese conversation with preferences")
	}
	if len(result.KeySentences) == 0 {
		t.Fatal("expected at least one key sentence")
	}
	t.Logf("Extracted: %s", result.FormatAsMemory())
}

func TestExtractFromConversation_Trivial(t *testing.T) {
	turns := []ConversationTurn{
		{Role: "user", Content: "Hi"},
		{Role: "assistant", Content: "Hello! How can I help you?"},
	}

	result := ExtractFromConversation(turns, 5)
	// Trivial conversation should return nil (nothing worth remembering)
	if result != nil && len(result.KeySentences) > 0 {
		t.Logf("Got result for trivial conversation (may be ok): %s", result.FormatAsMemory())
	}
}

func TestExtractFromConversation_Empty(t *testing.T) {
	result := ExtractFromConversation(nil, 5)
	if result != nil {
		t.Fatal("expected nil for empty conversation")
	}
}

func TestSignalWordScore(t *testing.T) {
	tests := []struct {
		text     string
		wantZero bool
	}{
		{"I prefer Go for backend", false},
		{"Remember to deploy by Friday", false},
		{"The weather is nice today", true},
		{"我喜欢用Python", false},
		{"记住我的密码", false},
	}

	for _, tt := range tests {
		score := pruner.SignalWordScore(tt.text, pruner.DefaultMemorySignals)
		if tt.wantZero && score > 0 {
			t.Errorf("SignalWordScore(%q) = %f, want 0", tt.text, score)
		}
		if !tt.wantZero && score == 0 {
			t.Errorf("SignalWordScore(%q) = 0, want > 0", tt.text)
		}
	}
}

func TestIRTokenize(t *testing.T) {
	tokens := pruner.TextTokenize("I prefer using Go for backend development")
	if len(tokens) == 0 {
		t.Fatal("expected tokens")
	}
	// Should not contain stopwords like "I", "for"
	for _, tok := range tokens {
		if pruner.Stopwords[tok] {
			t.Errorf("token %q should have been filtered as stopword", tok)
		}
	}
	t.Logf("Tokens: %v", tokens)
}
