package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
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

		// Once we already have history, cue-based auto switching is disabled.
		{"short_no_ref", "What is Go?", 4, false, false, TierFullHistory},
		{"short_with_ref", "这个怎么用？", 4, false, false, TierFullHistory},
		{"long_no_ref", "What is the weather today?", 10, false, false, TierFullHistory},
		{"long_no_ref_en", "How do I install Docker?", 20, false, false, TierFullHistory},
		{"long_topic_switch_cn", "换个话题，聊聊 Docker 网络", 10, false, false, TierFullHistory},
		{"topic_switch_overrides_reference_cues", "对了，换个话题，忽略之前那段", 10, false, false, TierFullHistory},
		{"topic_switch_overrides_agent_mode", "切换话题，解释一下 HTTP/3", 10, true, false, TierFullHistory},
		{"long_chinese_ref_this", "这个方案可以吗？", 10, false, false, TierFullHistory},
		{"long_chinese_ref_before", "之前说的那个", 10, false, false, TierFullHistory},
		{"long_chinese_ref_continue", "继续", 10, false, false, TierFullHistory},
		{"long_chinese_ref_then", "然后呢", 10, false, false, TierFullHistory},
		{"long_chinese_ref_also", "还有一个问题", 10, false, false, TierFullHistory},
		{"long_chinese_ref_why", "为什么会这样", 10, false, false, TierFullHistory},
		{"long_chinese_ref_elliptical", "ZIMAOS上呢", 10, false, false, TierFullHistory},
		{"long_english_ref_this", "Can you explain this further?", 10, false, false, TierFullHistory},
		{"long_english_ref_that", "That doesn't work", 10, false, false, TierFullHistory},
		{"long_english_ref_it", "Why did it fail?", 10, false, false, TierFullHistory},
		{"long_english_ref_before", "As I mentioned before", 10, false, false, TierFullHistory},
		{"long_english_ref_continue", "continue", 10, false, false, TierFullHistory},
		{"long_english_ref_previous", "Go back to the previous approach", 10, false, false, TierFullHistory},
		{"agent_short", "Run the tests", 4, true, false, TierFullHistory},
		{"agent_long", "Run the tests", 10, true, false, TierFullHistory},
		{"regenerate", "Regenerate", 10, false, true, TierFullHistory},
		{"regenerate_keeps_context_even_with_switch_phrase", "换个话题", 10, false, true, TierFullHistory},
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

func TestHasTopicSwitchCue(t *testing.T) {
	switchCues := []string{
		"换个话题，我们聊点别的",
		"切换话题，忽略之前内容",
		"We should change the topic now",
		"Let's talk about something else",
		"ignore the previous discussion",
	}
	for _, msg := range switchCues {
		if !hasTopicSwitchCue(msg) {
			t.Errorf("hasTopicSwitchCue(%q) = false, want true", msg)
		}
	}

	nonSwitch := []string{
		"这个问题怎么解决",
		"继续上一个方案",
		"What about this approach?",
	}
	for _, msg := range nonSwitch {
		if hasTopicSwitchCue(msg) {
			t.Errorf("hasTopicSwitchCue(%q) = true, want false", msg)
		}
	}
}

func TestShouldUseFreshStandaloneIMContext(t *testing.T) {
	for _, tc := range []struct {
		msg       string
		agentMode bool
	}{
		{msg: "换个话题，解释 Kubernetes", agentMode: true},
		{msg: "继续上一个任务", agentMode: true},
		{msg: "换个话题", agentMode: false},
	} {
		if shouldUseFreshStandaloneIMContext(tc.msg, tc.agentMode) {
			t.Fatalf("expected auto fresh-standalone IM switching to stay disabled for %q", tc.msg)
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
			name:       "automatic_turn_recalls",
			msg:        "继续这个方案",
			tier:       TierNoHistory,
			wantRecall: true,
		},
		{
			name:       "no_history_without_cue_still_recalls",
			msg:        "今天天气怎么样",
			tier:       TierNoHistory,
			wantRecall: true,
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
			name:       "agent_mode_still_recalls",
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
		name         string
		msg          string
		continuation bool
		regenerate   bool
		wantRecall   bool
		wantReason   MemoryRecallReason
	}{
		{
			name:       "regenerate",
			msg:        "remember this",
			regenerate: true,
			wantRecall: false,
			wantReason: MemoryRecallReasonRegenerateSkip,
		},
		{
			name:         "continuation_skip",
			msg:          "hello",
			continuation: true,
			wantRecall:   false,
			wantReason:   MemoryRecallReasonContinuationSkip,
		},
		{
			name:       "automatic_turn",
			msg:        "继续",
			wantRecall: true,
			wantReason: MemoryRecallReasonAutomaticTurn,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRecall, gotReason := memoryRecallDecision(tt.msg, tt.continuation, tt.regenerate, MemoryRecallModeBalanced)
			if gotRecall != tt.wantRecall || gotReason != tt.wantReason {
				t.Fatalf("memoryRecallDecision()=(%v,%s), want (%v,%s)",
					gotRecall, gotReason, tt.wantRecall, tt.wantReason)
			}
		})
	}
}

func TestBuildSmartContext_RefetchesAfterConversationCacheBudgetEviction(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	defer handler.Close()
	handler.conversationCache.maxBytes = 120
	handler.conversationCache.ttl = time.Minute

	convA, err := store.CreateConversation(context.Background(), "cache-a")
	if err != nil {
		t.Fatalf("CreateConversation(cache-a): %v", err)
	}
	convB, err := store.CreateConversation(context.Background(), "cache-b")
	if err != nil {
		t.Fatalf("CreateConversation(cache-b): %v", err)
	}

	for _, msg := range []memory.Message{
		{Role: "user", Content: "Explain the first cache entry"},
		{Role: "assistant", Content: "Here is the first cached answer."},
	} {
		if _, err := store.AddMessage(context.Background(), convA.ID, msg); err != nil {
			t.Fatalf("AddMessage(convA): %v", err)
		}
	}
	if _, err := store.AddMessage(context.Background(), convB.ID, memory.Message{
		Role:    "user",
		Content: strings.Repeat("x", 256),
	}); err != nil {
		t.Fatalf("AddMessage(convB): %v", err)
	}

	first := handler.buildSmartContext(context.Background(), smartContextParams{
		ConvID:      convA.ID,
		UserMessage: "继续",
		Model:       "gpt-4o-mini",
	})
	if first.MessageCountBefore != 2 {
		t.Fatalf("first MessageCountBefore = %d, want 2", first.MessageCountBefore)
	}

	largeMessages, err := store.GetMessages(context.Background(), convB.ID, 16, 0)
	if err != nil {
		t.Fatalf("GetMessages(convB): %v", err)
	}
	handler.conversationCache.Set(convB.ID, largeMessages)
	if _, hit := handler.conversationCache.Get(convA.ID); hit {
		t.Fatal("expected conversation A cache entry to be evicted by byte budget")
	}

	second := handler.buildSmartContext(context.Background(), smartContextParams{
		ConvID:      convA.ID,
		UserMessage: "继续",
		Model:       "gpt-4o-mini",
	})
	if second.MessageCountBefore != first.MessageCountBefore {
		t.Fatalf("second MessageCountBefore = %d, want %d after refetch", second.MessageCountBefore, first.MessageCountBefore)
	}
	if len(second.Messages) != len(first.Messages) {
		t.Fatalf("second Messages len = %d, want %d", len(second.Messages), len(first.Messages))
	}
}

func TestShouldRecallMemories_Mode(t *testing.T) {
	msg := "继续这个方案"
	if !shouldRecallMemories(msg, TierCompressedMemory, false, false, MemoryRecallModeAggressive) {
		t.Fatalf("aggressive mode should still recall normal turns")
	}
	if !shouldRecallMemories(msg, TierCompressedMemory, false, false, MemoryRecallModeBalanced) {
		t.Fatalf("balanced mode should recall normal turns")
	}
	if !shouldRecallMemories("hello", TierCompressedMemory, false, false, MemoryRecallModeQuality) {
		t.Fatalf("quality mode should recall normal turns")
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
	stats.RecordWithSource(true, MemoryRecallReasonAutomaticTurn, MemoryRecallSourceSend)
	stats.RecordWithSource(true, MemoryRecallReasonAutomaticTurn, MemoryRecallSourceSend)
	stats.RecordWithSource(false, MemoryRecallReasonContinuationSkip, MemoryRecallSourceStream)
	stats.RecordInjectionWithSource(120, MemoryRecallSourceSend)
	stats.RecordInjectionWithSource(80, MemoryRecallSourceSend)
	stats.RecordWithSource(false, MemoryRecallReasonRegenerateSkip, MemoryRecallSourceIM)

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
	if s.ReasonCounts[string(MemoryRecallReasonAutomaticTurn)] != 2 {
		t.Fatalf("automatic_turn count = %d, want 2", s.ReasonCounts[string(MemoryRecallReasonAutomaticTurn)])
	}
	if s.ReasonCounts[string(MemoryRecallReasonContinuationSkip)] != 1 {
		t.Fatalf("continuation_skip count = %d, want 1", s.ReasonCounts[string(MemoryRecallReasonContinuationSkip)])
	}
	if s.ReasonCounts[string(MemoryRecallReasonRegenerateSkip)] != 1 {
		t.Fatalf("regenerate skip count = %d, want 1", s.ReasonCounts[string(MemoryRecallReasonRegenerateSkip)])
	}

	send := s.BySource[string(MemoryRecallSourceSend)]
	if send.Total != 2 || send.Recalled != 2 || send.Skipped != 0 {
		t.Fatalf("send source totals %+v", send)
	}
	if send.AvgInjectedTokens != 100 || send.EstimatedSavedTokens != 0 {
		t.Fatalf("send source token stats %+v", send)
	}
	if send.ReasonCounts[string(MemoryRecallReasonAutomaticTurn)] != 2 {
		t.Fatalf("send source reason counts %+v", send.ReasonCounts)
	}
	stream := s.BySource[string(MemoryRecallSourceStream)]
	if stream.Total != 1 || stream.Skipped != 1 || stream.EstimatedSavedTokens != 0 {
		t.Fatalf("stream source stats %+v", stream)
	}
	if stream.ReasonCounts[string(MemoryRecallReasonContinuationSkip)] != 1 {
		t.Fatalf("stream source reason counts %+v", stream.ReasonCounts)
	}
	im := s.BySource[string(MemoryRecallSourceIM)]
	if im.Total != 1 || im.Skipped != 1 {
		t.Fatalf("im source stats %+v", im)
	}
	if im.ReasonCounts[string(MemoryRecallReasonRegenerateSkip)] != 1 {
		t.Fatalf("im source reason counts %+v", im.ReasonCounts)
	}
}

func TestMemoryRecallStatsReset(t *testing.T) {
	var stats MemoryRecallStats
	stats.RecordWithSource(true, MemoryRecallReasonAutomaticTurn, MemoryRecallSourceSend)
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

func TestTrimPolicyPruneContextMessages_SoftTrim(t *testing.T) {
	longTool := strings.Repeat("x", 7000)
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: "u1"},
		{Role: llm.RoleAssistant, Content: "a1"},
		{Role: llm.RoleTool, Content: longTool, ToolCallID: "tc1"},
		{Role: llm.RoleAssistant, Content: "a1 followup"},
		{Role: llm.RoleUser, Content: "u2"},
		{Role: llm.RoleAssistant, Content: "a2"},
		{Role: llm.RoleUser, Content: "u3"},
		{Role: llm.RoleAssistant, Content: "a3"},
		{Role: llm.RoleUser, Content: "u4"},
		{Role: llm.RoleAssistant, Content: "a4"},
	}

	out := trimPolicyPruneContextMessages(msgs, 500)
	if len(out) != len(msgs) {
		t.Fatalf("len(out) = %d, want %d", len(out), len(msgs))
	}
	if out[2].Content == longTool {
		t.Fatal("expected old tool result to be soft-trimmed")
	}
	if !strings.Contains(out[2].Content, "[Tool result trimmed: kept first") {
		t.Fatalf("trim note missing: %q", out[2].Content)
	}
	if len([]rune(out[2].Content)) >= len([]rune(longTool)) {
		t.Fatalf("trimmed content length = %d, want < %d", len([]rune(out[2].Content)), len([]rune(longTool)))
	}
}

func TestTrimPolicyPruneContextMessages_ProtectedTailNotPruned(t *testing.T) {
	longTool := strings.Repeat("y", 7000)
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: "u1"},
		{Role: llm.RoleAssistant, Content: "a1"},
		{Role: llm.RoleUser, Content: "u2"},
		{Role: llm.RoleAssistant, Content: "a2"},
		{Role: llm.RoleUser, Content: "u3"},
		{Role: llm.RoleAssistant, Content: "a3"},
		{Role: llm.RoleTool, Content: longTool, ToolCallID: "tc9"},
		{Role: llm.RoleUser, Content: "u4"},
		{Role: llm.RoleAssistant, Content: "a4"},
	}

	out := trimPolicyPruneContextMessages(msgs, 500)
	if out[6].Content != longTool {
		t.Fatalf("tool result in protected tail should stay unchanged")
	}
}

func TestTrimPolicyPruneContextMessages_InsufficientAssistantsSkips(t *testing.T) {
	longTool := strings.Repeat("z", 7000)
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: "u1"},
		{Role: llm.RoleAssistant, Content: "a1"},
		{Role: llm.RoleTool, Content: longTool, ToolCallID: "tc1"},
		{Role: llm.RoleUser, Content: "u2"},
		{Role: llm.RoleAssistant, Content: "a2"},
	}
	out := trimPolicyPruneContextMessages(msgs, 500)
	if out[2].Content != longTool {
		t.Fatalf("expected no pruning when assistant count < keepLastAssistants")
	}
}

func TestTrimPolicyPruneContextMessages_ToolAllowDeny(t *testing.T) {
	longTool := strings.Repeat("w", 7000)
	buildMsgs := func() []llm.Message {
		return []llm.Message{
			{Role: llm.RoleUser, Content: "u1"},
			{
				Role: llm.RoleAssistant,
				ToolCalls: []llm.ToolCall{
					{ID: "tc1", Name: "web_search", Arguments: `{"query":"x"}`},
				},
			},
			{Role: llm.RoleTool, Content: longTool, ToolCallID: "tc1"},
			{Role: llm.RoleUser, Content: "u2"},
			{Role: llm.RoleAssistant, Content: "a2"},
			{Role: llm.RoleUser, Content: "u3"},
			{Role: llm.RoleAssistant, Content: "a3"},
			{Role: llm.RoleUser, Content: "u4"},
			{Role: llm.RoleAssistant, Content: "a4"},
		}
	}

	defaultOut := trimPolicyPruneContextMessages(buildMsgs(), 500)
	if defaultOut[2].Content == longTool {
		t.Fatal("expected default policy to prune tool result")
	}

	denyWeb := trimPolicyDefaultPruneSettings
	denyWeb.Tools.Deny = []string{"web_*"}
	denyOut := trimPolicyPruneContextMessagesWithSettings(buildMsgs(), 500, denyWeb)
	if denyOut[2].Content != longTool {
		t.Fatal("deny rule should skip pruning for web_search")
	}

	allowExecOnly := trimPolicyDefaultPruneSettings
	allowExecOnly.Tools.Allow = []string{"exec"}
	allowOut := trimPolicyPruneContextMessagesWithSettings(buildMsgs(), 500, allowExecOnly)
	if allowOut[2].Content != longTool {
		t.Fatal("allow rule should skip pruning for unmatched tool")
	}

	allowAndDeny := trimPolicyDefaultPruneSettings
	allowAndDeny.Tools.Allow = []string{"web_*"}
	allowAndDeny.Tools.Deny = []string{"web_search"}
	allowAndDenyOut := trimPolicyPruneContextMessagesWithSettings(buildMsgs(), 500, allowAndDeny)
	if allowAndDenyOut[2].Content != longTool {
		t.Fatal("deny rule should take precedence over allow rule")
	}
}

func TestTrimPolicyPruneContextMessages_ToolAllowDeny_ExplicitToolName(t *testing.T) {
	longTool := strings.Repeat("w", 7000)
	buildMsgs := func() []llm.Message {
		return []llm.Message{
			{Role: llm.RoleUser, Content: "u1"},
			{Role: llm.RoleAssistant, Content: "a1"},
			{Role: llm.RoleTool, Content: longTool, ToolName: "web_search"},
			{Role: llm.RoleUser, Content: "u2"},
			{Role: llm.RoleAssistant, Content: "a2"},
			{Role: llm.RoleUser, Content: "u3"},
			{Role: llm.RoleAssistant, Content: "a3"},
			{Role: llm.RoleUser, Content: "u4"},
			{Role: llm.RoleAssistant, Content: "a4"},
		}
	}

	defaultOut := trimPolicyPruneContextMessages(buildMsgs(), 500)
	if defaultOut[2].Content == longTool {
		t.Fatal("expected default policy to prune tool result")
	}

	allowExecOnly := trimPolicyDefaultPruneSettings
	allowExecOnly.Tools.Allow = []string{"exec"}
	allowOut := trimPolicyPruneContextMessagesWithSettings(buildMsgs(), 500, allowExecOnly)
	if allowOut[2].Content != longTool {
		t.Fatal("allow rule should skip pruning for unmatched explicit tool name")
	}

	allowWeb := trimPolicyDefaultPruneSettings
	allowWeb.Tools.Allow = []string{"web_*"}
	allowWebOut := trimPolicyPruneContextMessagesWithSettings(buildMsgs(), 500, allowWeb)
	if allowWebOut[2].Content == longTool {
		t.Fatal("allow rule should permit pruning for matched explicit tool name")
	}
}

func TestTrimPolicyPruneContextMessages_HardClearWithSettings(t *testing.T) {
	longTool := strings.Repeat("k", 9000)
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: "u1"},
		{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{ID: "tc1", Name: "exec", Arguments: `{"command":"echo 1"}`},
			},
		},
		{Role: llm.RoleTool, Content: longTool, ToolCallID: "tc1"},
		{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{ID: "tc2", Name: "exec", Arguments: `{"command":"echo 2"}`},
			},
		},
		{Role: llm.RoleTool, Content: longTool, ToolCallID: "tc2"},
		{Role: llm.RoleUser, Content: "u2"},
		{Role: llm.RoleAssistant, Content: "a2"},
		{Role: llm.RoleUser, Content: "u3"},
		{Role: llm.RoleAssistant, Content: "a3"},
		{Role: llm.RoleUser, Content: "u4"},
		{Role: llm.RoleAssistant, Content: "a4"},
	}

	settings := trimPolicyDefaultPruneSettings
	settings.MinPrunableToolChars = 3000
	settings.HardClearRatio = 0.4
	settings.HardClear.Placeholder = "[cleared-by-policy]"

	out := trimPolicyPruneContextMessagesWithSettings(msgs, 500, settings)
	if out[2].Content != settings.HardClear.Placeholder {
		t.Fatalf("tool[2] not hard-cleared, got=%q", out[2].Content)
	}
	if out[4].Content != settings.HardClear.Placeholder {
		t.Fatalf("tool[4] not hard-cleared, got=%q", out[4].Content)
	}
}

func TestTrimPolicyPruneContextMessagesWithReport_SkippedByToolPolicy(t *testing.T) {
	longTool := strings.Repeat("p", 7000)
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: "u1"},
		{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{ID: "tc1", Name: "web_search", Arguments: `{"query":"x"}`},
			},
		},
		{Role: llm.RoleTool, Content: longTool, ToolCallID: "tc1"},
		{Role: llm.RoleUser, Content: "u2"},
		{Role: llm.RoleAssistant, Content: "a2"},
		{Role: llm.RoleUser, Content: "u3"},
		{Role: llm.RoleAssistant, Content: "a3"},
		{Role: llm.RoleUser, Content: "u4"},
		{Role: llm.RoleAssistant, Content: "a4"},
	}

	settings := trimPolicyDefaultPruneSettings
	settings.Tools.Allow = []string{"exec"}
	out, report := trimPolicyPruneContextMessagesWithReport(msgs, 500, settings)
	if out[2].Content != longTool {
		t.Fatal("tool result should remain unchanged when disallowed by tool policy")
	}
	if report.ExaminedToolResults != 1 {
		t.Fatalf("examined=%d, want 1", report.ExaminedToolResults)
	}
	if report.SkippedByToolPolicy != 1 {
		t.Fatalf("skipped_by_tool_policy=%d, want 1", report.SkippedByToolPolicy)
	}
	if report.EligibleToolResults != 0 || report.SoftTrimmed != 0 || report.HardCleared != 0 {
		t.Fatalf("unexpected report counts: %+v", report)
	}
}

func TestTrimPolicyPruneContextMessagesWithReport_HardClearByTool(t *testing.T) {
	longTool := strings.Repeat("h", 9000)
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: "u1"},
		{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{ID: "tc1", Name: "exec", Arguments: `{"command":"echo 1"}`},
			},
		},
		{Role: llm.RoleTool, Content: longTool, ToolCallID: "tc1"},
		{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{ID: "tc2", Name: "exec", Arguments: `{"command":"echo 2"}`},
			},
		},
		{Role: llm.RoleTool, Content: longTool, ToolCallID: "tc2"},
		{Role: llm.RoleUser, Content: "u2"},
		{Role: llm.RoleAssistant, Content: "a2"},
		{Role: llm.RoleUser, Content: "u3"},
		{Role: llm.RoleAssistant, Content: "a3"},
		{Role: llm.RoleUser, Content: "u4"},
		{Role: llm.RoleAssistant, Content: "a4"},
	}

	settings := trimPolicyDefaultPruneSettings
	settings.MinPrunableToolChars = 3000
	settings.HardClearRatio = 0.4
	out, report := trimPolicyPruneContextMessagesWithReport(msgs, 500, settings)
	if out[2].Content != settings.HardClear.Placeholder || out[4].Content != settings.HardClear.Placeholder {
		t.Fatalf("expected both tool results hard-cleared, got tool2=%q tool4=%q", out[2].Content, out[4].Content)
	}
	if report.HardCleared != 2 {
		t.Fatalf("hard_cleared=%d, want 2", report.HardCleared)
	}
	if report.HardClearByTool["exec"] != 2 {
		t.Fatalf("hard_clear_by_tool=%v, want exec:2", report.HardClearByTool)
	}
}

func TestTrimPolicyPruneContextMessagesWithReport_WebQueryHardClearUsesSemanticSummary(t *testing.T) {
	raw := buildWebQueryToolPayloadForLLMTests(strings.Repeat("On April 4, 2026, version 1.2.3 shipped 12 improvements, 4 fixes, and 2 migrations. ", 180))
	contentBytes, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal web_query payload: %v", err)
	}
	longTool := string(contentBytes) + strings.Repeat("\n", 64)
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: "u1"},
		{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{ID: "tc1", Name: "web_query", Arguments: `{"input":"blue release notes"}`},
			},
		},
		{Role: llm.RoleTool, Content: longTool, ToolCallID: "tc1"},
		{Role: llm.RoleUser, Content: "u2"},
		{Role: llm.RoleAssistant, Content: "a2"},
		{Role: llm.RoleUser, Content: "u3"},
		{Role: llm.RoleAssistant, Content: "a3"},
		{Role: llm.RoleUser, Content: "u4"},
		{Role: llm.RoleAssistant, Content: "a4"},
	}

	settings := trimPolicyDefaultPruneSettings
	settings.MinPrunableToolChars = 1500
	settings.HardClearRatio = 0.4

	out, report := trimPolicyPruneContextMessagesWithReport(msgs, 500, settings)
	if out[2].Content == settings.HardClear.Placeholder {
		t.Fatalf("web_query tool result fell back to generic placeholder: %q", out[2].Content)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(out[2].Content), &payload); err != nil {
		t.Fatalf("semantic hard-clear payload must stay valid JSON: %v payload=%q", err, out[2].Content)
	}
	if got, ok := payload["has_results"].(bool); !ok || !got {
		t.Fatalf("has_results = %#v, want true", payload["has_results"])
	}
	if _, ok := payload["selected_result"].(map[string]interface{}); !ok {
		t.Fatalf("selected_result = %#v, want object", payload["selected_result"])
	}
	if report.HardCleared != 0 {
		t.Fatalf("expected semantic downgrade instead of hard clear, report=%+v", report)
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
		{Role: "tool", Content: "1", ToolCallID: "t1", ToolName: "calc"},
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
	if result[2].ToolName != "calc" {
		t.Errorf("msg[2] ToolName = %q, want calc", result[2].ToolName)
	}
}

func TestBuildSmartContextTierNoHistoryUsesLatestTurnOnly(t *testing.T) {
	h := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	got := h.buildSmartContext(context.Background(), smartContextParams{
		ConvID:            "first-turn",
		UserMessage:       "What is Rust?",
		PreloadedMessages: []memory.Message{{Role: "user", Content: "What is Rust?"}},
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

func TestBuildSmartContextTierNoHistoryKeepsRecentRoundsWhenConfigured(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	h := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	h.SetProviderPool(newProviderPoolWithContextWindowModels(t, []contextWindowModelSpec{{
		ProviderID:    "p-context",
		ModelID:       "tiny-history-model",
		ContextWindow: 40,
	}}))
	preloaded := []memory.Message{
		{Role: "user", Content: "Q1"},
		{Role: "assistant", Content: "A1"},
		{Role: "user", Content: "Q2"},
		{Role: "assistant", Content: "A2"},
		{Role: "user", Content: "What is Rust?"},
	}

	got := h.buildSmartContext(context.Background(), smartContextParams{
		ConvID:                "conv-no-history-recent",
		UserMessage:           "What is Rust?",
		NoHistoryRecentRounds: 2,
		PreloadedMessages:     preloaded,
	})

	if got.Tier != TierFullHistory {
		t.Fatalf("tier = %v, want %v", got.Tier, TierFullHistory)
	}
	if len(got.Messages) != len(preloaded) {
		t.Fatalf("messages len = %d, want %d", len(got.Messages), len(preloaded))
	}
	if got.Messages[0].Role != llm.RoleUser || got.Messages[0].Content != "Q1" {
		t.Fatalf("messages[0] = %+v, want user/Q1", got.Messages[0])
	}
	if got.Messages[len(got.Messages)-1].Role != llm.RoleUser || got.Messages[len(got.Messages)-1].Content != "What is Rust?" {
		t.Fatalf("last message = %+v, want user/What is Rust?", got.Messages[len(got.Messages)-1])
	}
}

func TestBuildSmartContext_UsesPruneToolRulesFromSettings(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	h := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	h.compactionConfig.MaxContextTokens = 500

	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	settings.settings.SmallModelContextPruneToolAllow = []string{"exec"}
	h.SetSettingsHandler(settings)

	longTool := strings.Repeat("r", 7000)
	preloaded := []memory.Message{
		{Role: "user", Content: "u1"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "u2"},
		{
			Role: "assistant",
			ToolCalls: []memory.ToolCall{
				{ID: "tc1", Name: "web_search", Arguments: `{"query":"x"}`},
			},
		},
		{Role: "tool", Content: longTool, ToolCallID: "tc1"},
		{Role: "assistant", Content: "a2 done"},
		{Role: "user", Content: "u3"},
		{Role: "assistant", Content: "a3"},
		{Role: "user", Content: "u4"},
		{Role: "assistant", Content: "a4"},
	}

	got := h.buildSmartContext(context.Background(), smartContextParams{
		ConvID:            "conv-prune-rules",
		UserMessage:       "继续",
		PreloadedMessages: preloaded,
	})

	var toolContent string
	for _, m := range got.Messages {
		if m.Role == llm.RoleTool && m.ToolCallID == "tc1" {
			toolContent = m.Content
			break
		}
	}
	if toolContent == "" {
		t.Fatal("expected tool message tc1 to be present in smart context")
	}
	if toolContent != longTool {
		t.Fatalf("tool content should be unchanged by allow-list policy, got=%q", toolContent)
	}
}

func TestBuildSmartContextCompressedMemoryTracksComparableMessageCounts(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	h := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	h.SetProviderPool(newProviderPoolWithContextWindowModels(t, []contextWindowModelSpec{{
		ProviderID:    "p-context",
		ModelID:       "tiny-history-model",
		ContextWindow: 128,
	}}))
	preloaded := []memory.Message{
		{Role: "user", Content: strings.Repeat("Q1 background ", 12)},
		{Role: "assistant", Content: strings.Repeat("A1 details ", 12)},
		{Role: "user", Content: strings.Repeat("Q2 background ", 12)},
		{Role: "assistant", Content: strings.Repeat("A2 details ", 12)},
		{Role: "user", Content: strings.Repeat("Q3 background ", 12)},
		{Role: "assistant", Content: strings.Repeat("A3 details ", 12)},
		{Role: "user", Content: strings.Repeat("继续前面的方案 ", 6)},
	}
	h.summaryCache.Put("conv-compaction-counts", &ConversationSummary{
		Text:         "Older context summary",
		MessageCount: len(preloaded),
	})

	got := h.buildSmartContext(context.Background(), smartContextParams{
		ConvID:            "conv-compaction-counts",
		UserMessage:       "继续",
		Model:             "tiny-history-model",
		MaxTokens:         16,
		PreloadedMessages: preloaded,
	})

	if got.Tier != TierCompressedMemory {
		t.Fatalf("tier = %v, want %v", got.Tier, TierCompressedMemory)
	}
	if got.Summary == "" {
		t.Fatal("expected cached summary to be used")
	}
	if got.MessageCountBefore != len(preloaded) {
		t.Fatalf("MessageCountBefore = %d, want %d", got.MessageCountBefore, len(preloaded))
	}
	if got.MessageCountAfter != len(got.Messages) {
		t.Fatalf("MessageCountAfter = %d, want %d", got.MessageCountAfter, len(got.Messages))
	}
	if got.MessageCountAfter != 7 {
		t.Fatalf("MessageCountAfter = %d, want 7", got.MessageCountAfter)
	}
	if got.Messages[0].Role != llm.RoleSystem || !strings.Contains(got.Messages[0].Content, "Current-turn anchor") {
		t.Fatalf("messages[0] = %+v, want current-turn anchor message", got.Messages[0])
	}
	if got.Messages[1].Role != llm.RoleSystem || !strings.Contains(got.Messages[1].Content, historicalContextBackgroundPrefix) || !strings.Contains(got.Messages[1].Content, "Older context summary") {
		t.Fatalf("messages[1] = %+v, want wrapped historical summary message", got.Messages[1])
	}
	if cached, ok := h.summaryCache.Get("conv-compaction-counts"); !ok {
		t.Fatal("expected cached summary to remain available")
	} else if summary := cached.(*ConversationSummary); strings.Contains(summary.Text, "Current-turn anchor") || strings.Contains(summary.Text, historicalContextBackgroundPrefix) {
		t.Fatalf("cached summary should remain raw, got %q", summary.Text)
	}
}

func TestBuildSmartContextContinuationKeepsFullHistoryUnderSoftThreshold(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	h := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	h.SetProviderPool(newProviderPoolWithContextWindowModels(t, []contextWindowModelSpec{{
		ProviderID:    "p-context",
		ModelID:       "large-history-model",
		ContextWindow: 32000,
	}}))

	preloaded := []memory.Message{
		{Role: "user", Content: "Q1"},
		{Role: "assistant", Content: "A1"},
		{Role: "user", Content: "Q2"},
		{Role: "assistant", Content: "A2"},
		{Role: "user", Content: "Q3"},
		{Role: "assistant", Content: "A3"},
		{Role: "user", Content: "继续"},
	}

	got := h.buildSmartContext(context.Background(), smartContextParams{
		ConvID:            "conv-full-history",
		UserMessage:       "继续",
		Model:             "large-history-model",
		MaxTokens:         256,
		PreloadedMessages: preloaded,
	})

	if got.Tier != TierFullHistory {
		t.Fatalf("tier = %v, want %v", got.Tier, TierFullHistory)
	}
	if got.Summary != "" {
		t.Fatalf("summary = %q, want empty when full history fits", got.Summary)
	}
	if got.MessageCountAfter != len(got.Messages) {
		t.Fatalf("MessageCountAfter = %d, want %d", got.MessageCountAfter, len(got.Messages))
	}
	if got.MessageCountAfter != len(preloaded) {
		t.Fatalf("MessageCountAfter = %d, want %d", got.MessageCountAfter, len(preloaded))
	}
}

func TestCompressedHistoryContextMessages_KeepsSafetyWrapperCompact(t *testing.T) {
	latestUser := "What was the root cause and which file changed?"
	summary := "Goal\n- Debug the login retry regression\n\nDiscoveries\n- API key rotation on 2026-03-18 broke refresh handling in auth/middleware.go.\n- request-id req_9F82B must remain exact."

	msgs := compressedHistoryContextMessages(latestUser, summary)
	if len(msgs) != 2 {
		t.Fatalf("compressed history messages = %d, want 2", len(msgs))
	}
	if msgs[0].Role != llm.RoleSystem || !strings.Contains(msgs[0].Content, "Current-turn anchor") {
		t.Fatalf("msgs[0] = %+v, want current-turn anchor system message", msgs[0])
	}
	if msgs[1].Role != llm.RoleSystem || !strings.Contains(msgs[1].Content, historicalContextBackgroundPrefix) {
		t.Fatalf("msgs[1] = %+v, want wrapped historical summary system message", msgs[1])
	}

	rawTokens := estimateTokens(latestUser) + estimateTokens(summary)
	wrappedTokens := estimateTokens(msgs[0].Content) + estimateTokens(msgs[1].Content)
	if wrappedTokens-rawTokens > 100 {
		t.Fatalf("safety wrapper overhead = %d tokens, want <= 100 (raw=%d wrapped=%d)", wrappedTokens-rawTokens, rawTokens, wrappedTokens)
	}
}

func TestBuildSmartContextCueMessagesStayFullHistoryUnderThreshold(t *testing.T) {
	h := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	h.SetProviderPool(newProviderPoolWithContextWindowModels(t, []contextWindowModelSpec{{
		ProviderID:    "p-context",
		ModelID:       "large-history-model",
		ContextWindow: 32000,
	}}))

	preloaded := []memory.Message{
		{Role: "user", Content: "Q1"},
		{Role: "assistant", Content: "A1"},
		{Role: "user", Content: "Q2"},
		{Role: "assistant", Content: "A2"},
		{Role: "user", Content: "Q3"},
		{Role: "assistant", Content: "A3"},
		{Role: "user", Content: "Q4"},
	}

	for _, cue := range []string{
		"换个话题，解释一下 Docker 网络",
		"忽略之前，直接回答这个问题",
		"继续",
		"上一个方案为什么失败",
	} {
		got := h.buildSmartContext(context.Background(), smartContextParams{
			ConvID:            "conv-cue-full-history",
			UserMessage:       cue,
			Model:             "large-history-model",
			MaxTokens:         256,
			PreloadedMessages: preloaded,
		})
		if got.Tier != TierFullHistory {
			t.Fatalf("cue %q tier = %v, want %v", cue, got.Tier, TierFullHistory)
		}
		if len(got.Messages) != len(preloaded) {
			t.Fatalf("cue %q message count = %d, want %d", cue, len(got.Messages), len(preloaded))
		}
	}
}

func TestBuildSmartContextPressureThresholdControlsCompression(t *testing.T) {
	h := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	h.SetProviderPool(newProviderPoolWithContextWindowModels(t, []contextWindowModelSpec{{
		ProviderID:    "p-context",
		ModelID:       "threshold-model",
		ContextWindow: 512,
	}}))

	var belowThreshold []memory.Message
	var aboveThreshold []memory.Message
	var belowBudget chatInputBudgetEstimate
	var aboveBudget chatInputBudgetEstimate

	buildMessages := func(repeat int) []memory.Message {
		return []memory.Message{
			{Role: "user", Content: strings.Repeat("Investigate threshold behavior ", repeat)},
			{Role: "assistant", Content: strings.Repeat("Capturing prior findings and file paths. ", repeat)},
			{Role: "user", Content: strings.Repeat("Keep the previous plan in mind. ", repeat)},
			{Role: "assistant", Content: strings.Repeat("Noted, preserving the durable context. ", repeat)},
			{Role: "user", Content: "继续上一个方案"},
		}
	}

	for repeat := 4; repeat <= 80; repeat++ {
		candidate := buildMessages(repeat)
		budget := h.measurePreparedInputBudget("threshold-model", 64, removeOrphanedToolResults(convertToLLMMessages(candidate)))
		switch {
		case budget.ContextUsageRatio() < smartContextSoftCompressionThreshold:
			belowThreshold = candidate
			belowBudget = budget
		case budget.ContextUsageRatio() >= smartContextSoftCompressionThreshold:
			aboveThreshold = candidate
			aboveBudget = budget
			repeat = 1000
		}
	}

	if len(belowThreshold) == 0 || len(aboveThreshold) == 0 {
		t.Fatalf(
			"failed to find threshold fixtures below/above %.0f%%: below=%v above=%v",
			smartContextSoftCompressionThreshold*100,
			belowBudget.ContextUsageRatio(),
			aboveBudget.ContextUsageRatio(),
		)
	}

	below := h.buildSmartContext(context.Background(), smartContextParams{
		ConvID:            "conv-threshold-below",
		UserMessage:       "继续上一个方案",
		Model:             "threshold-model",
		MaxTokens:         64,
		PreloadedMessages: belowThreshold,
	})
	if below.Tier != TierFullHistory {
		t.Fatalf("below-threshold tier = %v, want %v (ratio=%.3f)", below.Tier, TierFullHistory, belowBudget.ContextUsageRatio())
	}

	h.summaryCache.Put("conv-threshold-above", &ConversationSummary{
		Text:         "Goal\n- Keep prior implementation context\n\nAccomplished\n- Preserve recent turns\n\nRelevant Files\n- server/internal/server/chat.go",
		MessageCount: len(aboveThreshold),
	})

	above := h.buildSmartContext(context.Background(), smartContextParams{
		ConvID:            "conv-threshold-above",
		UserMessage:       "继续上一个方案",
		Model:             "threshold-model",
		MaxTokens:         64,
		PreloadedMessages: aboveThreshold,
	})
	if above.Tier != TierCompressedMemory {
		t.Fatalf("above-threshold tier = %v, want %v (ratio=%.3f)", above.Tier, TierCompressedMemory, aboveBudget.ContextUsageRatio())
	}
	if above.Summary == "" {
		t.Fatalf("expected structured summary above threshold, ratio=%.3f", aboveBudget.ContextUsageRatio())
	}
}

func TestBuildSmartContextIgnoresStaleSummaryCache(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	h := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	h.SetProviderPool(newProviderPoolWithContextWindowModels(t, []contextWindowModelSpec{{
		ProviderID:    "p-context",
		ModelID:       "tiny-history-model",
		ContextWindow: 128,
	}}))
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	summaryEnabled := true
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelSummaryEnabled = &summaryEnabled
	h.SetSettingsHandler(settings)
	sm := &smallModelRuntimeMock{respText: "- Goal: keep recent context\n- Pending: verify fresh summary"}
	h.SetSmallModelRuntime(sm)

	preloaded := []memory.Message{
		{Role: "user", Content: strings.Repeat("Investigate login retry ordering ", 12)},
		{Role: "assistant", Content: strings.Repeat("I am checking middleware ordering and token refresh timing. ", 12)},
		{Role: "user", Content: strings.Repeat("Also remember the regression test path. ", 10)},
		{Role: "assistant", Content: strings.Repeat("Noted, I will keep the file path and pending test in memory. ", 10)},
		{Role: "user", Content: strings.Repeat("Keep the deployment preference and the file path in mind. ", 10)},
		{Role: "assistant", Content: strings.Repeat("I will preserve those details in the context summary. ", 10)},
		{Role: "user", Content: strings.Repeat("继续", 4)},
	}
	h.summaryCache.Put("conv-stale-summary", &ConversationSummary{
		Text:         "stale summary should not survive",
		MessageCount: len(preloaded) - 2,
	})

	got := h.buildSmartContext(context.Background(), smartContextParams{
		ConvID:            "conv-stale-summary",
		UserMessage:       "继续",
		Model:             "tiny-history-model",
		MaxTokens:         16,
		PreloadedMessages: preloaded,
	})

	if got.Tier != TierCompressedMemory {
		t.Fatalf("tier = %v, want %v", got.Tier, TierCompressedMemory)
	}
	if strings.Contains(got.Summary, "stale summary") {
		t.Fatalf("expected stale summary to be ignored, got %q", got.Summary)
	}
	if !strings.Contains(got.Summary, "Accomplished") {
		t.Fatalf("expected regenerated canonical summary, got %q", got.Summary)
	}
	if sm.calls != 1 {
		t.Fatalf("small model calls = %d, want 1", sm.calls)
	}
	if cached, ok := h.summaryCache.Get("conv-stale-summary"); !ok {
		t.Fatal("expected refreshed summary cache entry")
	} else if summary, ok := cached.(*ConversationSummary); !ok || summary.MessageCount != len(preloaded) {
		t.Fatalf("cached summary = %#v, want refreshed message count %d", cached, len(preloaded))
	}
}

func TestContextTierString(t *testing.T) {
	if TierNoHistory.String() != "no_history" {
		t.Errorf("TierNoHistory.String() = %q", TierNoHistory.String())
	}
	if TierFullHistory.String() != "full_history" {
		t.Errorf("TierFullHistory.String() = %q", TierFullHistory.String())
	}
	if TierCompressedMemory.String() != "compressed_memory" {
		t.Errorf("TierCompressedMemory.String() = %q", TierCompressedMemory.String())
	}
}

func TestGenerateSummarySync_UsesSmallModelWhenEnabled(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	h := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	smallModelEnabled := true
	summaryEnabled := true
	settings.settings.SmallModelEnabled = &smallModelEnabled
	settings.settings.SmallModelSummaryEnabled = &summaryEnabled
	h.SetSettingsHandler(settings)

	sm := &smallModelRuntimeMock{respText: "Summary: Login flow fixed, pending one regression test."}
	h.SetSmallModelRuntime(sm)

	allMessages := []memory.Message{
		{Role: "user", Content: "Login keeps failing after password reset"},
		{Role: "assistant", Content: "I will inspect auth middleware and retry policy."},
		{Role: "user", Content: "Capture the file path for the fix."},
		{Role: "assistant", Content: "Noted, I will preserve the middleware path."},
		{Role: "user", Content: "Now it works but add regression tests."},
		{Role: "assistant", Content: "I fixed the middleware ordering and prepared tests."},
		{Role: "user", Content: "Also mention the test file in the summary."},
		{Role: "assistant", Content: "I will keep the regression test path in context."},
	}
	recent := []llm.Message{
		{Role: llm.RoleUser, Content: "Capture the file path for the fix."},
		{Role: llm.RoleAssistant, Content: "Noted, I will preserve the middleware path."},
		{Role: llm.RoleUser, Content: "Now it works but add regression tests."},
		{Role: llm.RoleAssistant, Content: "I fixed the middleware ordering and prepared tests."},
		{Role: llm.RoleUser, Content: "Also mention the test file in the summary."},
		{Role: llm.RoleAssistant, Content: "I will keep the regression test path in context."},
	}

	got := h.generateSummarySync(context.Background(), "conv-small-summary", allMessages, recent)
	if got == "" {
		t.Fatal("expected non-empty summary from small model")
	}
	if sm.calls != 1 {
		t.Fatalf("small model calls = %d, want 1", sm.calls)
	}
	snap := h.smallModelStats.Snapshot()
	if snap.SummaryAttempts != 1 || snap.SummarySuccess != 1 {
		t.Fatalf("unexpected summary stats: attempts=%d success=%d", snap.SummaryAttempts, snap.SummarySuccess)
	}
	if cached, ok := h.summaryCache.Get("conv-small-summary"); !ok || cached.(*ConversationSummary).Text == "" {
		t.Fatal("expected summary cache to be populated")
	}
}

func TestGenerateSummarySync_UsesSmallModelContextCompressionWhenEnabled(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	h := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	smallModelEnabled := true
	contextCompressEnabled := true
	settings.settings.SmallModelEnabled = &smallModelEnabled
	settings.settings.SmallModelContextCompressEnabled = &contextCompressEnabled
	h.SetSettingsHandler(settings)

	sm := &smallModelRuntimeMock{respText: "Goal\n- Fix login retry ordering\n\nAccomplished\n- [carry-over] Add one regression test\n- I will inspect more logs later\n\nRelevant Files\n- auth/middleware.go"}
	h.SetSmallModelRuntime(sm)

	allMessages := []memory.Message{
		{Role: "user", Content: "Fix the login retry ordering"},
		{Role: "assistant", Content: "I will inspect the middleware ordering first."},
		{Role: "user", Content: "Keep auth/middleware.go in the summary."},
		{Role: "assistant", Content: "Noted, I will preserve that path."},
		{Role: "user", Content: "Also mention the pending regression test."},
		{Role: "assistant", Content: "I fixed the ordering; one regression test is still pending."},
		{Role: "user", Content: "Now answer the latest question briefly."},
	}
	recent := []llm.Message{
		{Role: llm.RoleUser, Content: "Keep auth/middleware.go in the summary."},
		{Role: llm.RoleAssistant, Content: "Noted, I will preserve that path."},
		{Role: llm.RoleUser, Content: "Also mention the pending regression test."},
		{Role: llm.RoleAssistant, Content: "I fixed the ordering; one regression test is still pending."},
		{Role: llm.RoleUser, Content: "Now answer the latest question briefly."},
	}

	got := h.generateSummarySync(context.Background(), "conv-small-context-compress", allMessages, recent)
	if got == "" {
		t.Fatal("expected non-empty summary from small-model context compression")
	}
	if strings.Contains(got, "I will inspect more logs later") {
		t.Fatalf("summary = %q, want process-only chatter omitted", got)
	}
	if !strings.Contains(got, "[carry-over]") {
		t.Fatalf("summary = %q, want carry-over marker preserved", got)
	}
	if sm.calls != 1 {
		t.Fatalf("small model calls = %d, want 1", sm.calls)
	}
	snap := h.smallModelStats.Snapshot()
	if snap.ContextCompressAttempts != 1 || snap.ContextCompressSuccess != 1 {
		t.Fatalf("unexpected context-compress stats: attempts=%d success=%d", snap.ContextCompressAttempts, snap.ContextCompressSuccess)
	}
	if snap.SummaryAttempts != 0 {
		t.Fatalf("summary attempts = %d, want 0 when context-compress route handled the request", snap.SummaryAttempts)
	}
}

func TestGenerateSummarySync_SmallModelSummaryDisabled(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	h := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	smallModelEnabled := true
	summaryEnabled := false
	settings.settings.SmallModelEnabled = &smallModelEnabled
	settings.settings.SmallModelSummaryEnabled = &summaryEnabled
	h.SetSettingsHandler(settings)

	sm := &smallModelRuntimeMock{respText: "should not be used"}
	h.SetSmallModelRuntime(sm)

	allMessages := []memory.Message{
		{Role: "user", Content: "older context 1"},
		{Role: "assistant", Content: "older context 2"},
		{Role: "user", Content: "older context 3"},
		{Role: "assistant", Content: "older context 4"},
		{Role: "user", Content: "older context 5"},
		{Role: "assistant", Content: "older context 6"},
		{Role: "user", Content: "recent question"},
	}
	recent := []llm.Message{
		{Role: llm.RoleUser, Content: "older context 3"},
		{Role: llm.RoleAssistant, Content: "older context 4"},
		{Role: llm.RoleUser, Content: "older context 5"},
		{Role: llm.RoleAssistant, Content: "older context 6"},
		{Role: llm.RoleUser, Content: "recent question"},
	}

	got := h.generateSummarySync(context.Background(), "conv-small-summary-disabled", allMessages, recent)
	if got == "" {
		t.Fatal("expected offline compression summary when small-model summary is disabled")
	}
	if sm.calls != 0 {
		t.Fatalf("small model calls = %d, want 0 when summary switch disabled", sm.calls)
	}
	snap := h.smallModelStats.Snapshot()
	if snap.SummaryAttempts != 0 || snap.SummarySuccess != 0 {
		t.Fatalf("unexpected summary stats when disabled: attempts=%d success=%d", snap.SummaryAttempts, snap.SummarySuccess)
	}
	if !strings.Contains(got, "Goal") && !strings.Contains(got, "Instructions") && !strings.Contains(got, "Discoveries") {
		t.Fatalf("expected canonical offline summary, got %q", got)
	}
}

func TestGenerateSummarySync_LegacyOffModeFallsBackToAutomaticCompression(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	h := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	settings.settings.ContextCompressionMode = "off"
	h.SetSettingsHandler(settings)

	got := h.generateSummarySync(context.Background(), "conv-compression-off", []memory.Message{
		{Role: "user", Content: "older context 1"},
		{Role: "assistant", Content: "older context 2"},
		{Role: "user", Content: "older context 3"},
		{Role: "assistant", Content: "older context 4"},
		{Role: "user", Content: "older context 5"},
		{Role: "assistant", Content: "older context 6"},
		{Role: "user", Content: "recent question"},
	}, []llm.Message{{Role: llm.RoleUser, Content: "recent question"}})
	if got == "" {
		t.Fatal("expected legacy off mode to fall back to automatic compression")
	}
}

func TestGenerateSummarySync_ContextCompressionModeOfflineSkipsSmallModel(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	h := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	smallModelEnabled := true
	summaryEnabled := true
	contextCompressEnabled := true
	settings.settings.SmallModelEnabled = &smallModelEnabled
	settings.settings.SmallModelSummaryEnabled = &summaryEnabled
	settings.settings.SmallModelContextCompressEnabled = &contextCompressEnabled
	settings.settings.ContextCompressionMode = "offline"
	h.SetSettingsHandler(settings)

	sm := &smallModelRuntimeMock{respText: "should not be used in offline mode"}
	h.SetSmallModelRuntime(sm)

	got := h.generateSummarySync(context.Background(), "conv-compression-offline", []memory.Message{
		{Role: "user", Content: "older context 1"},
		{Role: "assistant", Content: "older context 2"},
		{Role: "user", Content: "older context 3"},
		{Role: "assistant", Content: "older context 4"},
		{Role: "user", Content: "older context 5"},
		{Role: "assistant", Content: "older context 6"},
		{Role: "user", Content: "recent question"},
	}, []llm.Message{
		{Role: llm.RoleUser, Content: "older context 5"},
		{Role: llm.RoleAssistant, Content: "older context 6"},
		{Role: llm.RoleUser, Content: "recent question"},
	})
	if got == "" {
		t.Fatal("expected offline summary when context_compression_mode=offline")
	}
	if sm.calls != 0 {
		t.Fatalf("small model calls = %d, want 0 when context_compression_mode=offline", sm.calls)
	}
}

func TestGenerateSummarySync_OfflineCompressionDedupesRepeatedGoalFallback(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	h := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	mode := "offline"
	settings.settings.ContextCompressionMode = mode
	h.SetSettingsHandler(settings)

	all := []memory.Message{
		{
			Role: "user",
			Content: strings.Repeat(
				"Debug the login retry regression and keep only the durable root cause details. ",
				6,
			),
		},
		{Role: "assistant", Content: "I am inspecting logs, middleware order, and retry traces before the final answer."},
		{Role: "user", Content: "Keep auth/middleware.go exact if it matters."},
		{Role: "assistant", Content: "Noted, I will preserve the exact file path."},
		{Role: "user", Content: "Also keep the 2026-03-18 date if that was the root cause timing."},
		{Role: "assistant", Content: "Understood, I will keep the date precise."},
	}
	recent := []llm.Message{
		{Role: llm.RoleUser, Content: "What was the root cause and which file changed?"},
	}
	all = append(all, llmMessagesToSummaryMemory(recent)...)

	summary := h.generateSummarySync(context.Background(), "conv-offline-fallback-dedupe", all, recent)
	if summary == "" {
		t.Fatal("expected non-empty offline summary")
	}
	if !strings.Contains(summary, "Historical task: Debug the login retry regression") {
		t.Fatalf("expected offline summary goal to be marked historical, got %q", summary)
	}
	if strings.Count(summary, "Debug the login retry regression and keep only the durable root cause details.") > 1 {
		t.Fatalf("expected repeated goal text to be deduped, got %q", summary)
	}
}

func TestCompressionHeuristics_HandleNonEnglishProcessAndFormattingMarkers(t *testing.T) {
	processSamples := []string{
		"vaig a revisar els registres abans de respondre",
		"podívám se nejdřív na logy",
		"jeg vil først tjekke loggene",
		"voy a revisar los logs antes de responder",
		"je vais vérifier les traces avant de répondre",
		"ich werde zuerst die Protokolle prüfen",
		"prima controllo i log",
		"ik ga eerst de logs controleren",
		"vou verificar os logs primeiro",
		"jag ska först kontrollera loggarna",
		"θα ελέγξω πρώτα τα αρχεία καταγραφής",
		"sprawdzę najpierw logi",
		"сначала проверю логи",
		"まずログを確認します",
		"먼저 로그를 확인할게",
		"我先检查日志",
		"ഞാൻ ലോഗുകൾ പരിശോധിക്കാം",
	}
	for _, sample := range processSamples {
		if !looksLikeProcessOnlyItem(sample) {
			t.Fatalf("looksLikeProcessOnlyItem(%q) = false, want true", sample)
		}
	}

	formattingSamples := []string{
		"fes-ne un resum concís",
		"udělej shrnutí stručně",
		"gør opsummering kort",
		"hazlo conciso y no repitas toda la investigación",
		"gardez le résumé concis",
		"halte die zusammenfassung kurz",
		"maak de samenvatting beknopt",
		"faça um resumo conciso",
		"håll sammanfattningen kort",
		"κράτα τη σύνοψη σύντομη",
		"podsumowanie ma być krótkie",
		"rezumat concis",
		"сводка должна быть краткой",
		"要約は簡潔にして",
		"요약은 간결하게 해줘",
		"总结要简洁",
		"സംഗ്രഹം ചുരുക്കമായി വേണം",
	}
	for _, sample := range formattingSamples {
		if !looksLikeFormattingOnlyInstruction(sample) {
			t.Fatalf("looksLikeFormattingOnlyInstruction(%q) = false, want true", sample)
		}
	}
}
