package server

import (
	"context"
	"strings"
	"testing"

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

		// Short conversation
		{"short_no_ref", "What is Go?", 4, false, false, TierNoHistory},
		{"short_with_ref", "这个怎么用？", 4, false, false, TierCompressedMemory},

		// Long conversation, no references → fresh question
		{"long_no_ref", "What is the weather today?", 10, false, false, TierNoHistory},
		{"long_no_ref_en", "How do I install Docker?", 20, false, false, TierNoHistory},
		{"long_topic_switch_cn", "换个话题，聊聊 Docker 网络", 10, false, false, TierNoHistory},
		{"topic_switch_overrides_reference_cues", "对了，换个话题，忽略之前那段", 10, false, false, TierNoHistory},
		{"topic_switch_overrides_agent_mode", "切换话题，解释一下 HTTP/3", 10, true, false, TierNoHistory},

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
		{"regenerate_keeps_context_even_with_switch_phrase", "换个话题", 10, false, true, TierCompressedMemory},
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
	if !shouldUseFreshStandaloneIMContext("换个话题，解释 Kubernetes", true) {
		t.Fatalf("expected explicit topic switch to force fresh IM context")
	}
	if shouldUseFreshStandaloneIMContext("继续上一个任务", true) {
		t.Fatalf("expected continuation cue to keep history in IM context")
	}
	if shouldUseFreshStandaloneIMContext("换个话题", false) {
		t.Fatalf("expected agent_mode=false to keep existing behavior")
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

func TestBuildSmartContextTierNoHistoryKeepsRecentRoundsWhenConfigured(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	h := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
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

	if got.Tier != TierNoHistory {
		t.Fatalf("tier = %v, want %v", got.Tier, TierNoHistory)
	}
	if len(got.Messages) != 3 {
		t.Fatalf("messages len = %d, want 3", len(got.Messages))
	}
	if got.Messages[0].Role != llm.RoleUser || got.Messages[0].Content != "Q2" {
		t.Fatalf("messages[0] = %+v, want user/Q2", got.Messages[0])
	}
	if got.Messages[1].Role != llm.RoleAssistant || got.Messages[1].Content != "A2" {
		t.Fatalf("messages[1] = %+v, want assistant/A2", got.Messages[1])
	}
	if got.Messages[2].Role != llm.RoleUser || got.Messages[2].Content != "What is Rust?" {
		t.Fatalf("messages[2] = %+v, want user/What is Rust?", got.Messages[2])
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
	preloaded := []memory.Message{
		{Role: "user", Content: "Q1"},
		{Role: "assistant", Content: "A1"},
		{Role: "user", Content: "Q2"},
		{Role: "assistant", Content: "A2"},
		{Role: "user", Content: "Q3"},
		{Role: "assistant", Content: "A3"},
		{Role: "user", Content: "继续"},
	}
	h.summaryCache.Put("conv-compaction-counts", &ConversationSummary{
		Text:         "Older context summary",
		MessageCount: len(preloaded),
	})

	got := h.buildSmartContext(context.Background(), smartContextParams{
		ConvID:            "conv-compaction-counts",
		UserMessage:       "继续",
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
	if got.MessageCountAfter != 6 {
		t.Fatalf("MessageCountAfter = %d, want 6", got.MessageCountAfter)
	}
	if got.MessageCountAfter >= got.MessageCountBefore {
		t.Fatalf("expected compacted message count to shrink, before=%d after=%d", got.MessageCountBefore, got.MessageCountAfter)
	}
	if got.Messages[0].Role != llm.RoleSystem || !strings.Contains(got.Messages[0].Content, "Older context summary") {
		t.Fatalf("messages[0] = %+v, want summary system message", got.Messages[0])
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
		{Role: "user", Content: "Now it works but add regression tests."},
		{Role: "assistant", Content: "I fixed the middleware ordering and prepared tests."},
	}
	recent := []llm.Message{
		{Role: llm.RoleUser, Content: "Now it works but add regression tests."},
		{Role: llm.RoleAssistant, Content: "I fixed the middleware ordering and prepared tests."},
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
		{Role: "user", Content: "recent question"},
	}
	recent := []llm.Message{
		{Role: llm.RoleUser, Content: "recent question"},
	}

	got := h.generateSummarySync(context.Background(), "conv-small-summary-disabled", allMessages, recent)
	if got != "" {
		t.Fatalf("summary = %q, want empty when small-model summary is disabled and no proxy bridge", got)
	}
	if sm.calls != 0 {
		t.Fatalf("small model calls = %d, want 0 when summary switch disabled", sm.calls)
	}
	snap := h.smallModelStats.Snapshot()
	if snap.SummaryAttempts != 0 || snap.SummarySuccess != 0 {
		t.Fatalf("unexpected summary stats when disabled: attempts=%d success=%d", snap.SummaryAttempts, snap.SummarySuccess)
	}
}
