package claudecode

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/llm"
)

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
			expected: 1, // 5 chars / 4 = 1
		},
		{
			name:     "longer message",
			msg:      llm.Message{Content: "This is a longer message with more content"},
			expected: 10, // 43 chars / 4 = 10
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
		{Content: "Test1234"}, // 2 tokens
	}

	result := EstimateMessagesTokens(messages)
	expected := 4 // 1 + 1 + 2

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
			msg:           llm.Message{Content: string(make([]byte, 200000))}, // 200k chars
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
		{Content: string(make([]byte, 10000))}, // ~2500 tokens
		{Content: string(make([]byte, 10000))}, // ~2500 tokens
		{Content: string(make([]byte, 10000))}, // ~2500 tokens
		{Content: string(make([]byte, 10000))}, // ~2500 tokens
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
