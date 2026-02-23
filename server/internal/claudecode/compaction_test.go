package claudecode

import (
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

// generateLargeText creates a string with approximately n whitespace-separated words.
func generateLargeText(n int) string {
	var sb strings.Builder
	sb.Grow(n * 5) // ~5 chars per word
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString("word")
	}
	return sb.String()
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name     string
		msg      llm.Message
		expected int
	}{
		{
			name:     "empty message",
			msg:      llm.Message{Content: ""},
			expected: 0,
		},
		{
			name:     "short message",
			msg:      llm.Message{Content: "Hello"},
			expected: 1, // 1 word
		},
		{
			name:     "longer message",
			msg:      llm.Message{Content: "This is a longer message with more content"},
			expected: 8, // 8 words
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EstimateTokens(tt.msg)
			if result != tt.expected {
				t.Errorf("EstimateTokens() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestEstimateMessagesTokens(t *testing.T) {
	messages := []llm.Message{
		{Content: "Hello"},    // 1 token
		{Content: "World"},    // 1 token
		{Content: "Test1234"}, // 1 token (single word)
	}

	result := EstimateMessagesTokens(messages)
	expected := 3 // 1 + 1 + 1

	if result != expected {
		t.Errorf("EstimateMessagesTokens() = %d, want %d", result, expected)
	}
}

func TestSplitMessagesByTokenShare(t *testing.T) {
	tests := []struct {
		name          string
		messages      []llm.Message
		parts         int
		expectedParts int
	}{
		{
			name:          "empty messages",
			messages:      []llm.Message{},
			parts:         2,
			expectedParts: 0,
		},
		{
			name: "single part",
			messages: []llm.Message{
				{Content: "Hello"},
			},
			parts:         1,
			expectedParts: 1,
		},
		{
			name: "two parts",
			messages: []llm.Message{
				{Content: "Hello World Test"},
				{Content: "Another message here"},
			},
			parts:         2,
			expectedParts: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SplitMessagesByTokenShare(tt.messages, tt.parts)
			if len(result) != tt.expectedParts {
				t.Errorf("SplitMessagesByTokenShare() returned %d parts, want %d", len(result), tt.expectedParts)
			}
		})
	}
}

func TestChunkMessagesByMaxTokens(t *testing.T) {
	messages := []llm.Message{
		{Content: "Short"},                                  // ~1 token
		{Content: "This is a medium length message"},        // ~8 tokens
		{Content: "Another short one"},                      // ~4 tokens
		{Content: "And one more message to test chunking"},  // ~9 tokens
	}

	// With max 10 tokens, should create multiple chunks
	chunks := ChunkMessagesByMaxTokens(messages, 10)

	if len(chunks) < 2 {
		t.Errorf("Expected at least 2 chunks, got %d", len(chunks))
	}

	// Verify all messages are preserved
	totalMessages := 0
	for _, chunk := range chunks {
		totalMessages += len(chunk)
	}
	if totalMessages != len(messages) {
		t.Errorf("Expected %d total messages, got %d", len(messages), totalMessages)
	}
}

func TestComputeAdaptiveChunkRatio(t *testing.T) {
	tests := []struct {
		name          string
		messages      []llm.Message
		contextWindow int
		minExpected   float64
		maxExpected   float64
	}{
		{
			name:          "empty messages",
			messages:      []llm.Message{},
			contextWindow: 100000,
			minExpected:   BaseChunkRatio,
			maxExpected:   BaseChunkRatio,
		},
		{
			name: "small messages",
			messages: []llm.Message{
				{Content: "Hello"},
				{Content: "World"},
			},
			contextWindow: 100000,
			minExpected:   BaseChunkRatio - 0.01,
			maxExpected:   BaseChunkRatio + 0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeAdaptiveChunkRatio(tt.messages, tt.contextWindow)
			if result < tt.minExpected || result > tt.maxExpected {
				t.Errorf("ComputeAdaptiveChunkRatio() = %f, want between %f and %f", result, tt.minExpected, tt.maxExpected)
			}
		})
	}
}

func TestIsOversizedForSummary(t *testing.T) {
	tests := []struct {
		name          string
		msg           llm.Message
		contextWindow int
		expected      bool
	}{
		{
			name:          "small message",
			msg:           llm.Message{Content: "Hello"},
			contextWindow: 100000,
			expected:      false,
		},
		{
			name:          "large message",
			msg:           llm.Message{Content: generateLargeText(200000)}, // ~200k words
			contextWindow: 100000,
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsOversizedForSummary(tt.msg, tt.contextWindow)
			if result != tt.expected {
				t.Errorf("IsOversizedForSummary() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPruneHistoryForContextShare(t *testing.T) {
	messages := []llm.Message{
		{Content: generateLargeText(2500)}, // ~2500 tokens
		{Content: generateLargeText(2500)}, // ~2500 tokens
		{Content: generateLargeText(2500)}, // ~2500 tokens
		{Content: generateLargeText(2500)}, // ~2500 tokens
	}

	// With max 5000 tokens budget (10000 * 0.5), should drop some messages
	result := PruneHistoryForContextShare(messages, 10000, 0.5, 2)

	if result.DroppedCount == 0 {
		t.Error("Expected some messages to be dropped")
	}

	if len(result.Messages)+result.DroppedCount != len(messages) {
		t.Errorf("Total messages mismatch: kept=%d, dropped=%d, original=%d",
			len(result.Messages), result.DroppedCount, len(messages))
	}

	if result.KeptTokens > result.BudgetTokens {
		t.Errorf("Kept tokens (%d) exceeds budget (%d)", result.KeptTokens, result.BudgetTokens)
	}
}

// oldEstimateTokens is the previous len/4 implementation for benchmarking comparison.
func oldEstimateTokens(msg llm.Message) int {
	total := 0
	if msg.Content != "" {
		total += len(msg.Content) / 4
	}
	for _, part := range msg.ContentParts {
		switch part.Type {
		case "text":
			total += len(part.Text) / 4
		case "image":
			total += 765
		}
	}
	for _, tc := range msg.ToolCalls {
		total += len(tc.Name)/4 + len(tc.Arguments)/4 + 10
	}
	if total == 0 && (msg.Role != "" || msg.ToolCallID != "") {
		total = 4
	}
	return total
}

func BenchmarkEstimateTokens(b *testing.B) {
	short := []llm.Message{
		{Role: llm.RoleUser, Content: "Hello, how are you doing today?"},
		{Role: llm.RoleAssistant, Content: "I'm doing well, thanks for asking! How can I help you?"},
		{Role: llm.RoleUser, Content: "This is a medium length message with some code: func main() { fmt.Println(\"hello\") }"},
		{Role: llm.RoleAssistant, Content: "Sure, let me help you with that function. Here's an improved version with error handling."},
	}
	long := []llm.Message{
		{Role: llm.RoleUser, Content: generateLargeText(500)},
		{Role: llm.RoleAssistant, Content: generateLargeText(2000)},
	}

	b.Run("short/old_len_div4", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, msg := range short {
				oldEstimateTokens(msg)
			}
		}
	})
	b.Run("short/new_whitespace_split", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, msg := range short {
				EstimateTokens(msg)
			}
		}
	})
	b.Run("long/old_len_div4", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, msg := range long {
				oldEstimateTokens(msg)
			}
		}
	})
	b.Run("long/new_whitespace_split", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, msg := range long {
				EstimateTokens(msg)
			}
		}
	})
}

// TestEstimateTokensAccuracy compares old (len/4) vs new (whitespace-split)
// against known token counts from real tokenizers.
// Reference token counts obtained from OpenAI tiktoken (cl100k_base).
func TestEstimateTokensAccuracy(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		expectedTokens int // ground truth from tiktoken
	}{
		{
			name:           "english_sentence",
			content:        "The quick brown fox jumps over the lazy dog",
			expectedTokens: 9, // tiktoken: 9
		},
		{
			name:           "code_snippet",
			content:        "func main() {\n\tfmt.Println(\"Hello, World!\")\n}",
			expectedTokens: 15, // tiktoken: ~15
		},
		{
			name:           "chinese_text",
			content:        "今天天气真好，我们一起去公园散步吧",
			expectedTokens: 14, // tiktoken: ~14
		},
		{
			name:           "mixed_en_cn",
			content:        "Hello 你好 World 世界",
			expectedTokens: 6, // tiktoken: ~6
		},
		{
			name:           "json_payload",
			content:        `{"name":"test","value":42,"tags":["a","b","c"]}`,
			expectedTokens: 21, // tiktoken: ~21
		},
		{
			name:           "long_english_paragraph",
			content:        "Large language models are neural networks trained on massive text datasets. They can generate human-like text, answer questions, write code, and perform many other language tasks. The transformer architecture enables these models to process long sequences efficiently.",
			expectedTokens: 44, // tiktoken: ~44
		},
	}

	t.Logf("%-25s %8s %8s %8s %8s %8s", "Case", "Truth", "Old", "OldErr%", "New", "NewErr%")
	t.Logf("%-25s %8s %8s %8s %8s %8s", "----", "-----", "---", "-------", "---", "-------")

	totalOldErr := 0.0
	totalNewErr := 0.0

	for _, tt := range tests {
		msg := llm.Message{Content: tt.content}
		oldResult := oldEstimateTokens(msg)
		newResult := EstimateTokens(msg)

		oldErrPct := float64(oldResult-tt.expectedTokens) / float64(tt.expectedTokens) * 100
		newErrPct := float64(newResult-tt.expectedTokens) / float64(tt.expectedTokens) * 100

		if oldErrPct < 0 {
			totalOldErr += -oldErrPct
		} else {
			totalOldErr += oldErrPct
		}
		if newErrPct < 0 {
			totalNewErr += -newErrPct
		} else {
			totalNewErr += newErrPct
		}

		t.Logf("%-25s %8d %8d %+7.1f%% %8d %+7.1f%%",
			tt.name, tt.expectedTokens, oldResult, oldErrPct, newResult, newErrPct)
	}

	avgOldErr := totalOldErr / float64(len(tests))
	avgNewErr := totalNewErr / float64(len(tests))
	t.Logf("")
	t.Logf("Average absolute error: old=%.1f%%, new=%.1f%%", avgOldErr, avgNewErr)

	// New method should have lower average error than old
	if avgNewErr > avgOldErr {
		t.Errorf("New method (avg err %.1f%%) should be more accurate than old (avg err %.1f%%)", avgNewErr, avgOldErr)
	}
}

func TestNormalizeParts(t *testing.T) {
	tests := []struct {
		name         string
		parts        int
		messageCount int
		expected     int
	}{
		{"zero parts", 0, 10, 1},
		{"negative parts", -1, 10, 1},
		{"one part", 1, 10, 1},
		{"normal case", 3, 10, 3},
		{"parts exceed messages", 20, 5, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeParts(tt.parts, tt.messageCount)
			if result != tt.expected {
				t.Errorf("normalizeParts(%d, %d) = %d, want %d", tt.parts, tt.messageCount, result, tt.expected)
			}
		})
	}
}
