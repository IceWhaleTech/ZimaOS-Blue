package server

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
)

func TestClassifyContext(t *testing.T) {
	tests := []struct {
		name         string
		message      string
		messageCount int
		agentMode    bool
		regenerate   bool
		want         ContextTier
	}{
		// First message / empty conversation
		{"first_message", "你好", 0, false, false, TierNoHistory},
		{"single_message", "Hello", 1, false, false, TierNoHistory},

		// Short conversation (≤6 messages = ≤3 rounds)
		{"short_no_ref", "What is Go?", 4, false, false, TierRecentOnly},
		{"short_with_ref", "这个怎么用？", 4, false, false, TierRecentOnly},

		// Long conversation, no references → fresh question
		{"long_no_ref", "What is the weather today?", 10, false, false, TierNoHistory},
		{"long_no_ref_en", "How do I install Docker?", 20, false, false, TierNoHistory},

		// Long conversation, with Chinese references
		{"long_chinese_ref_this", "这个方案可以吗？", 10, false, false, TierCompressedMemory},
		{"long_chinese_ref_before", "之前说的那个", 10, false, false, TierCompressedMemory},
		{"long_chinese_ref_continue", "继续", 10, false, false, TierCompressedMemory},
		{"long_chinese_ref_then", "然后呢", 10, false, false, TierCompressedMemory},
		{"long_chinese_ref_also", "还有一个问题", 10, false, false, TierCompressedMemory},
		{"long_chinese_ref_why", "为什么会这样", 10, false, false, TierCompressedMemory},

		// Long conversation, with English references
		{"long_english_ref_this", "Can you explain this further?", 10, false, false, TierCompressedMemory},
		{"long_english_ref_that", "That doesn't work", 10, false, false, TierCompressedMemory},
		{"long_english_ref_it", "Why did it fail?", 10, false, false, TierCompressedMemory},
		{"long_english_ref_before", "As I mentioned before", 10, false, false, TierCompressedMemory},
		{"long_english_ref_continue", "continue", 10, false, false, TierCompressedMemory},
		{"long_english_ref_previous", "Go back to the previous approach", 10, false, false, TierCompressedMemory},

		// Agent mode
		{"agent_short", "Run the tests", 4, true, false, TierRecentOnly},
		{"agent_long", "Run the tests", 10, true, false, TierCompressedMemory},

		// Regenerate
		{"regenerate", "Regenerate", 10, false, true, TierRecentOnly},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyContext(tt.message, tt.messageCount, tt.agentMode, tt.regenerate)
			if got != tt.want {
				t.Errorf("classifyContext(%q, %d, %v, %v) = %v, want %v",
					tt.message, tt.messageCount, tt.agentMode, tt.regenerate, got, tt.want)
			}
		})
	}
}

func TestHasReference(t *testing.T) {
	// Should detect references
	refs := []string{
		"这个怎么用", "那个方案", "刚才说的", "之前提到的",
		"继续", "然后呢", "接着说", "还有一个",
		"What about this?", "That is wrong", "Can you explain it?",
		"They should work", "As mentioned earlier",
		"Go back to the previous one", "Continue please",
	}
	for _, msg := range refs {
		if !hasReference(msg) {
			t.Errorf("hasReference(%q) = false, want true", msg)
		}
	}

	// Should NOT detect references (standalone questions)
	noRefs := []string{
		"你好", "Hello", "What is Go?",
		"How do I install Docker?",
		"今天天气怎么样",
		"Calculate 2+2",
	}
	for _, msg := range noRefs {
		if hasReference(msg) {
			t.Errorf("hasReference(%q) = true, want false", msg)
		}
	}
}

func TestExtractRecentRounds(t *testing.T) {
	messages := []memory.Message{
		{Role: "user", Content: "Q1"},
		{Role: "assistant", Content: "A1"},
		{Role: "user", Content: "Q2"},
		{Role: "assistant", Content: "A2"},
		{Role: "user", Content: "Q3"},
		{Role: "assistant", Content: "A3"},
	}

	// Extract last 2 rounds
	result := extractRecentRounds(messages, 2)
	if len(result) != 4 {
		t.Fatalf("extractRecentRounds(6 msgs, 2 rounds) = %d messages, want 4", len(result))
	}
	if result[0].Content != "Q2" {
		t.Errorf("first message = %q, want Q2", result[0].Content)
	}
	if result[3].Content != "A3" {
		t.Errorf("last message = %q, want A3", result[3].Content)
	}

	// Extract last 1 round
	result = extractRecentRounds(messages, 1)
	if len(result) != 2 {
		t.Fatalf("extractRecentRounds(6 msgs, 1 round) = %d messages, want 2", len(result))
	}
	if result[0].Content != "Q3" {
		t.Errorf("first message = %q, want Q3", result[0].Content)
	}

	// Extract more rounds than available
	result = extractRecentRounds(messages, 10)
	if len(result) != 6 {
		t.Fatalf("extractRecentRounds(6 msgs, 10 rounds) = %d messages, want 6", len(result))
	}

	// Empty messages
	result = extractRecentRounds(nil, 2)
	if result != nil {
		t.Errorf("extractRecentRounds(nil, 2) = %v, want nil", result)
	}
}

func TestExtractRecentRoundsWithToolCalls(t *testing.T) {
	messages := []memory.Message{
		{Role: "user", Content: "Q1"},
		{Role: "assistant", Content: "A1"},
		{Role: "user", Content: "Search for X"},
		{Role: "assistant", Content: "", ToolCalls: []memory.ToolCall{
			{ID: "tc1", Name: "search", Arguments: `{"q":"X"}`},
		}},
		{Role: "tool", Content: "Result for X", ToolCallID: "tc1"},
		{Role: "assistant", Content: "Here is X"},
		{Role: "user", Content: "Q3"},
		{Role: "assistant", Content: "A3"},
	}

	// Extract last 2 rounds — should include the tool round
	result := extractRecentRounds(messages, 2)
	if len(result) != 6 {
		t.Fatalf("extractRecentRounds with tools = %d messages, want 6", len(result))
	}
	if result[0].Content != "Search for X" {
		t.Errorf("first message = %q, want 'Search for X'", result[0].Content)
	}
	// Verify tool call is preserved
	if len(result[1].ToolCalls) != 1 || result[1].ToolCalls[0].ID != "tc1" {
		t.Errorf("tool call not preserved in extracted messages")
	}
}

func TestConvertToLLMMessages(t *testing.T) {
	messages := []memory.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi", ToolCalls: []memory.ToolCall{
			{ID: "t1", Name: "calc", Arguments: `{"x":1}`},
		}},
		{Role: "tool", Content: "1", ToolCallID: "t1"},
	}

	result := convertToLLMMessages(messages)
	if len(result) != 3 {
		t.Fatalf("convertToLLMMessages = %d, want 3", len(result))
	}
	if string(result[0].Role) != "user" || result[0].Content != "Hello" {
		t.Errorf("msg[0] = %v, want user/Hello", result[0])
	}
	if len(result[1].ToolCalls) != 1 || result[1].ToolCalls[0].Name != "calc" {
		t.Errorf("msg[1] tool calls not preserved")
	}
	if result[2].ToolCallID != "t1" {
		t.Errorf("msg[2] ToolCallID = %q, want t1", result[2].ToolCallID)
	}
}

func TestContextTierString(t *testing.T) {
	if TierNoHistory.String() != "no_history" {
		t.Errorf("TierNoHistory.String() = %q", TierNoHistory.String())
	}
	if TierRecentOnly.String() != "recent_only" {
		t.Errorf("TierRecentOnly.String() = %q", TierRecentOnly.String())
	}
	if TierCompressedMemory.String() != "compressed_memory" {
		t.Errorf("TierCompressedMemory.String() = %q", TierCompressedMemory.String())
	}
}
