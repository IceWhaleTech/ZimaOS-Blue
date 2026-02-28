package server

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
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

		// Short conversation
		{"short_no_ref", "What is Go?", 4, false, false, TierNoHistory},
		{"short_with_ref", "这个怎么用？", 4, false, false, TierCompressedMemory},

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
		{"long_chinese_ref_elliptical", "ZIMAOS上呢", 10, false, false, TierCompressedMemory},

		// Long conversation, with English references
		{"long_english_ref_this", "Can you explain this further?", 10, false, false, TierCompressedMemory},
		{"long_english_ref_that", "That doesn't work", 10, false, false, TierCompressedMemory},
		{"long_english_ref_it", "Why did it fail?", 10, false, false, TierCompressedMemory},
		{"long_english_ref_before", "As I mentioned before", 10, false, false, TierCompressedMemory},
		{"long_english_ref_continue", "continue", 10, false, false, TierCompressedMemory},
		{"long_english_ref_previous", "Go back to the previous approach", 10, false, false, TierCompressedMemory},

		// Agent mode
		{"agent_short", "Run the tests", 4, true, false, TierCompressedMemory},
		{"agent_long", "Run the tests", 10, true, false, TierCompressedMemory},

		// Regenerate
		{"regenerate", "Regenerate", 10, false, true, TierCompressedMemory},
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
		"ZIMAOS上呢", "Docker里呢？",
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

func TestShouldRecallMemories(t *testing.T) {
	tests := []struct {
		name       string
		msg        string
		tier       ContextTier
		agentMode  bool
		regenerate bool
		wantRecall bool
	}{
		{
			name:       "compressed_tier_recalls",
			msg:        "继续这个方案",
			tier:       TierCompressedMemory,
			wantRecall: true,
		},
		{
			name:       "no_history_without_cue_skips",
			msg:        "今天天气怎么样",
			tier:       TierNoHistory,
			wantRecall: false,
		},
		{
			name:       "no_history_with_memory_cue_recalls",
			msg:        "你还记得我的偏好吗",
			tier:       TierNoHistory,
			wantRecall: true,
		},
		{
			name:       "no_history_with_session_query_capability_cue_recalls",
			msg:        "确保会话查询能力会被记录到记忆里",
			tier:       TierNoHistory,
			wantRecall: true,
		},
		{
			name:       "agent_mode_always_recalls",
			msg:        "run tests",
			tier:       TierNoHistory,
			agentMode:  true,
			wantRecall: true,
		},
		{
			name:       "regenerate_skips_recall",
			msg:        "你还记得我的偏好吗",
			tier:       TierCompressedMemory,
			regenerate: true,
			wantRecall: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldRecallMemories(tt.msg, tt.tier, tt.agentMode, tt.regenerate, MemoryRecallModeBalanced)
			if got != tt.wantRecall {
				t.Fatalf("shouldRecallMemories(%q, %v, %v, %v) = %v, want %v",
					tt.msg, tt.tier, tt.agentMode, tt.regenerate, got, tt.wantRecall)
			}
		})
	}
}

func TestMemoryRecallDecisionReason(t *testing.T) {
	tests := []struct {
		name       string
		msg        string
		tier       ContextTier
		agentMode  bool
		regenerate bool
		wantRecall bool
		wantReason MemoryRecallReason
	}{
		{
			name:       "regenerate",
			msg:        "remember this",
			tier:       TierCompressedMemory,
			regenerate: true,
			wantRecall: false,
			wantReason: MemoryRecallReasonRegenerateSkip,
		},
		{
			name:       "agent_mode",
			msg:        "hello",
			tier:       TierNoHistory,
			agentMode:  true,
			wantRecall: true,
			wantReason: MemoryRecallReasonAgentMode,
		},
		{
			name:       "compressed_tier",
			msg:        "继续",
			tier:       TierCompressedMemory,
			wantRecall: true,
			wantReason: MemoryRecallReasonCompressedTier,
		},
		{
			name:       "memory_cue",
			msg:        "你还记得我的偏好吗",
			tier:       TierNoHistory,
			wantRecall: true,
			wantReason: MemoryRecallReasonMemoryCue,
		},
		{
			name:       "memory_cue_session_query_capability",
			msg:        "请把 session query capability 记录到记忆里",
			tier:       TierNoHistory,
			wantRecall: true,
			wantReason: MemoryRecallReasonMemoryCue,
		},
		{
			name:       "default_skip",
			msg:        "今天天气怎么样",
			tier:       TierNoHistory,
			wantRecall: false,
			wantReason: MemoryRecallReasonDefaultSkip,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRecall, gotReason := memoryRecallDecision(tt.msg, tt.tier, tt.agentMode, tt.regenerate, MemoryRecallModeBalanced)
			if gotRecall != tt.wantRecall || gotReason != tt.wantReason {
				t.Fatalf("memoryRecallDecision()=(%v,%s), want (%v,%s)",
					gotRecall, gotReason, tt.wantRecall, tt.wantReason)
			}
		})
	}
}

func TestShouldRecallMemories_Mode(t *testing.T) {
	msg := "继续这个方案"
	if shouldRecallMemories(msg, TierCompressedMemory, false, false, MemoryRecallModeAggressive) {
		t.Fatalf("aggressive mode should skip compressed-tier recall without explicit memory cue")
	}
	if !shouldRecallMemories(msg, TierCompressedMemory, false, false, MemoryRecallModeBalanced) {
		t.Fatalf("balanced mode should recall on compressed tier")
	}
	if !shouldRecallMemories("hello", TierCompressedMemory, false, false, MemoryRecallModeQuality) {
		t.Fatalf("quality mode should recall on compressed tier")
	}
}

func TestRecallLimitsForMode(t *testing.T) {
	aggressive := recallLimitsForMode(MemoryRecallModeAggressive)
	if aggressive.MaxResults >= 5 || aggressive.TotalRunes >= 800 {
		t.Fatalf("aggressive limits too large: %+v", aggressive)
	}

	balanced := recallLimitsForMode(MemoryRecallModeBalanced)
	if balanced.MaxResults != 5 || balanced.ChunkRunes != 200 || balanced.TotalRunes != 800 {
		t.Fatalf("balanced limits unexpected: %+v", balanced)
	}

	quality := recallLimitsForMode(MemoryRecallModeQuality)
	if quality.MaxResults <= balanced.MaxResults || quality.TotalRunes <= balanced.TotalRunes {
		t.Fatalf("quality limits should be larger than balanced: %+v vs %+v", quality, balanced)
	}
}

func TestRecallMinScoreForMode(t *testing.T) {
	if got := recallMinScoreForMode(MemoryRecallModeAggressive); got <= 0.5 {
		t.Fatalf("aggressive min score = %v, want > 0.5", got)
	}
	if got := recallMinScoreForMode(MemoryRecallModeBalanced); got != 0.5 {
		t.Fatalf("balanced min score = %v, want 0.5", got)
	}
	if got := recallMinScoreForMode(MemoryRecallModeQuality); got >= 0.5 {
		t.Fatalf("quality min score = %v, want < 0.5", got)
	}
}

func TestMemoryRecallStats(t *testing.T) {
	var stats MemoryRecallStats
	stats.RecordWithSource(true, MemoryRecallReasonCompressedTier, MemoryRecallSourceSend)
	stats.RecordWithSource(true, MemoryRecallReasonMemoryCue, MemoryRecallSourceSend)
	stats.RecordWithSource(false, MemoryRecallReasonDefaultSkip, MemoryRecallSourceStream)
	stats.RecordInjectionWithSource(120, MemoryRecallSourceSend)
	stats.RecordInjectionWithSource(80, MemoryRecallSourceSend)
	stats.RecordWithSource(false, MemoryRecallReasonDefaultSkip, MemoryRecallSourceIM)

	s := stats.Snapshot()
	if s.Total != 4 || s.Recalled != 2 || s.Skipped != 2 {
		t.Fatalf("snapshot totals %+v", s)
	}
	if s.InjectedContexts != 2 || s.InjectedTokens != 200 {
		t.Fatalf("injected contexts/tokens = %d/%d, want 2/200", s.InjectedContexts, s.InjectedTokens)
	}
	if s.AvgInjectedTokens != 100 {
		t.Fatalf("avg injected tokens = %d, want 100", s.AvgInjectedTokens)
	}
	if s.EstimatedSavedTokens != 200 {
		t.Fatalf("estimated saved tokens = %d, want 200", s.EstimatedSavedTokens)
	}
	if s.ReasonCounts[string(MemoryRecallReasonCompressedTier)] != 1 {
		t.Fatalf("compressed count = %d, want 1", s.ReasonCounts[string(MemoryRecallReasonCompressedTier)])
	}
	if s.ReasonCounts[string(MemoryRecallReasonMemoryCue)] != 1 {
		t.Fatalf("memory cue count = %d, want 1", s.ReasonCounts[string(MemoryRecallReasonMemoryCue)])
	}
	if s.ReasonCounts[string(MemoryRecallReasonDefaultSkip)] != 2 {
		t.Fatalf("default skip count = %d, want 2", s.ReasonCounts[string(MemoryRecallReasonDefaultSkip)])
	}

	send := s.BySource[string(MemoryRecallSourceSend)]
	if send.Total != 2 || send.Recalled != 2 || send.Skipped != 0 {
		t.Fatalf("send source totals %+v", send)
	}
	if send.AvgInjectedTokens != 100 || send.EstimatedSavedTokens != 0 {
		t.Fatalf("send source token stats %+v", send)
	}
	if send.ReasonCounts[string(MemoryRecallReasonCompressedTier)] != 1 ||
		send.ReasonCounts[string(MemoryRecallReasonMemoryCue)] != 1 {
		t.Fatalf("send source reason counts %+v", send.ReasonCounts)
	}
	stream := s.BySource[string(MemoryRecallSourceStream)]
	if stream.Total != 1 || stream.Skipped != 1 || stream.EstimatedSavedTokens != 0 {
		t.Fatalf("stream source stats %+v", stream)
	}
	if stream.ReasonCounts[string(MemoryRecallReasonDefaultSkip)] != 1 {
		t.Fatalf("stream source reason counts %+v", stream.ReasonCounts)
	}
	im := s.BySource[string(MemoryRecallSourceIM)]
	if im.Total != 1 || im.Skipped != 1 {
		t.Fatalf("im source stats %+v", im)
	}
	if im.ReasonCounts[string(MemoryRecallReasonDefaultSkip)] != 1 {
		t.Fatalf("im source reason counts %+v", im.ReasonCounts)
	}
}

func TestMemoryRecallStatsReset(t *testing.T) {
	var stats MemoryRecallStats
	stats.RecordWithSource(true, MemoryRecallReasonMemoryCue, MemoryRecallSourceSend)
	stats.RecordInjectionWithSource(42, MemoryRecallSourceSend)
	stats.Reset()

	s := stats.Snapshot()
	if s.Total != 0 || s.Recalled != 0 || s.InjectedTokens != 0 {
		t.Fatalf("snapshot after reset %+v", s)
	}
	send := s.BySource[string(MemoryRecallSourceSend)]
	if send.Total != 0 || send.InjectedTokens != 0 {
		t.Fatalf("send snapshot after reset %+v", send)
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

func TestBuildSmartContextTierNoHistoryUsesLatestTurnOnly(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "no-history")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	h := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())

	seed := []memory.Message{
		{Role: "user", Content: "Q1"},
		{Role: "assistant", Content: "A1"},
		{Role: "user", Content: "Q2"},
		{Role: "assistant", Content: "", ToolCalls: []memory.ToolCall{
			{ID: "tc1", Name: "exec", Arguments: `{"cmd":"pwd"}`},
		}},
		{Role: "tool", Content: `{"stdout":"/tmp"}`, ToolCallID: "tc1"},
		{Role: "assistant", Content: "done"},
		{Role: "user", Content: "What is Rust?"},
	}
	for _, m := range seed {
		if _, err := store.AddMessage(context.Background(), conv.ID, m); err != nil {
			t.Fatalf("AddMessage: %v", err)
		}
	}

	got := h.buildSmartContext(context.Background(), smartContextParams{
		ConvID:      conv.ID,
		UserMessage: "What is Rust?",
	})

	if got.Tier != TierNoHistory {
		t.Fatalf("tier = %v, want %v", got.Tier, TierNoHistory)
	}
	if len(got.Messages) != 1 {
		t.Fatalf("messages len = %d, want 1", len(got.Messages))
	}
	if got.Messages[0].Role != llm.RoleUser || got.Messages[0].Content != "What is Rust?" {
		t.Fatalf("latest message = %+v, want user/What is Rust?", got.Messages[0])
	}
}

func TestContextTierString(t *testing.T) {
	if TierNoHistory.String() != "no_history" {
		t.Errorf("TierNoHistory.String() = %q", TierNoHistory.String())
	}
	if TierCompressedMemory.String() != "compressed_memory" {
		t.Errorf("TierCompressedMemory.String() = %q", TierCompressedMemory.String())
	}
}
