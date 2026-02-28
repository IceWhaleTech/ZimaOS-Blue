package server

import (
	"context"
	"regexp"
	"sync/atomic"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/claudecode"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/logger"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
)

// ContextTier represents the 3-tier context strategy.
type ContextTier int

const (
	// TierNoHistory: standalone question, no history needed.
	TierNoHistory ContextTier = iota
	// TierRecentOnly: references to recent context, last 1-2 rounds.
	TierRecentOnly
	// TierCompressedMemory: needs older context, inject compressed summary.
	TierCompressedMemory
)

func (t ContextTier) String() string {
	switch t {
	case TierNoHistory:
		return "no_history"
	case TierRecentOnly:
		return "recent_only"
	case TierCompressedMemory:
		return "compressed_memory"
	default:
		return "unknown"
	}
}

// ContextStrategyResult holds the output of buildSmartContext.
type ContextStrategyResult struct {
	Tier     ContextTier
	Messages []llm.Message // Context messages (excluding system prompt)
	Summary  string        // Non-empty only for TierCompressedMemory
}

// ConversationSummary is the cached summary for a conversation.
type ConversationSummary struct {
	Text         string
	MessageCount int
}

// --- Classification ---

// Reference patterns for Chinese and English.
// Detect pronouns, demonstratives, and continuity markers.
var (
	reChineseRef = regexp.MustCompile(
		`(?:这个|那个|刚才|之前|上面|上次|前面|它们?|他们?|她们?|` +
			`继续|然后|接着|还有|另外|再说|也是|同样|` +
			`你说的|你提到|刚说|上一个|那个方案|这个方案|` +
			`为什么|怎么回事|什么意思|` +
			`对了|不过|但是|所以|因此)`)

	reEnglishRef = regexp.MustCompile(
		`(?i)\b(?:` +
			`this|that|these|those|it|they|them|its|their|` +
			`earlier|before|previous|previously|above|` +
			`continue|also|furthermore|moreover|` +
			`you said|you mentioned|as I said|the one|` +
			`why did|what do you mean|go on|keep going|` +
			`same|again|another|next)\b`)

	reContinuation = regexp.MustCompile(
		`(?i)^(继续|go on|keep going|continue|接着说|然后呢|next|more)\s*[.?!？。！]*$`)

	// Chinese elliptical follow-ups are often short and omit explicit pronouns,
	// e.g. "ZIMAOS上呢". Treat them as context-dependent questions.
	reChineseEllipsisRef = regexp.MustCompile(
		`(?i)^[\p{Han}A-Za-z0-9_\-\s]{1,32}(?:上|里|中)?呢[？?]?$`)

	// Memory-intent hints: user explicitly asks for remembered preferences/facts.
	reMemoryCue = regexp.MustCompile(
		`(?i)(?:\bremember\b|\bmemory\b|\bpreference\b|\bprofile\b|\bas i said\b|` +
			`记得|记住|你还记得|我喜欢|我的偏好|之前说过|个人资料|习惯)`)
)

// MemoryRecallReason indicates why memory recall was triggered or skipped.
type MemoryRecallReason string

const (
	MemoryRecallReasonRegenerateSkip MemoryRecallReason = "regenerate_skip"
	MemoryRecallReasonAgentMode      MemoryRecallReason = "agent_mode"
	MemoryRecallReasonCompressedTier MemoryRecallReason = "compressed_tier"
	MemoryRecallReasonMemoryCue      MemoryRecallReason = "memory_cue"
	MemoryRecallReasonDefaultSkip    MemoryRecallReason = "default_skip"
)

// MemoryRecallSource indicates where the memory decision came from.
type MemoryRecallSource string

const (
	MemoryRecallSourceUnknown MemoryRecallSource = "unknown"
	MemoryRecallSourceIM      MemoryRecallSource = "im"
	MemoryRecallSourceSend    MemoryRecallSource = "send"
	MemoryRecallSourceStream  MemoryRecallSource = "stream"
)

// MemoryRecallMode controls recall aggressiveness.
type MemoryRecallMode string

const (
	MemoryRecallModeAggressive MemoryRecallMode = "aggressive"
	MemoryRecallModeBalanced   MemoryRecallMode = "balanced"
	MemoryRecallModeQuality    MemoryRecallMode = "quality"
)

// MemoryRecallStats tracks memory recall decisions for observability.
type MemoryRecallStats struct {
	Total            atomic.Int64
	Recalled         atomic.Int64
	Skipped          atomic.Int64
	InjectedContexts atomic.Int64
	InjectedTokens   atomic.Int64
	ReasonRegenerate atomic.Int64
	ReasonAgentMode  atomic.Int64
	ReasonCompressed atomic.Int64
	ReasonMemoryCue  atomic.Int64
	ReasonDefault    atomic.Int64

	TotalIM            atomic.Int64
	RecalledIM         atomic.Int64
	SkippedIM          atomic.Int64
	InjectedContextsIM atomic.Int64
	InjectedTokensIM   atomic.Int64

	TotalSend            atomic.Int64
	RecalledSend         atomic.Int64
	SkippedSend          atomic.Int64
	InjectedContextsSend atomic.Int64
	InjectedTokensSend   atomic.Int64

	TotalStream            atomic.Int64
	RecalledStream         atomic.Int64
	SkippedStream          atomic.Int64
	InjectedContextsStream atomic.Int64
	InjectedTokensStream   atomic.Int64

	ReasonRegenerateIM atomic.Int64
	ReasonAgentModeIM  atomic.Int64
	ReasonCompressedIM atomic.Int64
	ReasonMemoryCueIM  atomic.Int64
	ReasonDefaultIM    atomic.Int64

	ReasonRegenerateSend atomic.Int64
	ReasonAgentModeSend  atomic.Int64
	ReasonCompressedSend atomic.Int64
	ReasonMemoryCueSend  atomic.Int64
	ReasonDefaultSend    atomic.Int64

	ReasonRegenerateStream atomic.Int64
	ReasonAgentModeStream  atomic.Int64
	ReasonCompressedStream atomic.Int64
	ReasonMemoryCueStream  atomic.Int64
	ReasonDefaultStream    atomic.Int64
}

// MemoryRecallStatsSnapshot is a JSON-serializable snapshot.
type MemoryRecallStatsSnapshot struct {
	Total                int64                                 `json:"total"`
	Recalled             int64                                 `json:"recalled"`
	Skipped              int64                                 `json:"skipped"`
	RecallRate           float64                               `json:"recall_rate"`
	InjectedContexts     int64                                 `json:"injected_contexts"`
	InjectedTokens       int64                                 `json:"injected_tokens"`
	AvgInjectedTokens    int64                                 `json:"avg_injected_tokens"`
	EstimatedSavedTokens int64                                 `json:"estimated_saved_tokens"`
	ReasonCounts         map[string]int64                      `json:"reason_counts"`
	BySource             map[string]MemoryRecallSourceSnapshot `json:"by_source"`
}

// MemoryRecallSourceSnapshot is a source-specific recall snapshot.
type MemoryRecallSourceSnapshot struct {
	Total                int64            `json:"total"`
	Recalled             int64            `json:"recalled"`
	Skipped              int64            `json:"skipped"`
	RecallRate           float64          `json:"recall_rate"`
	InjectedContexts     int64            `json:"injected_contexts"`
	InjectedTokens       int64            `json:"injected_tokens"`
	AvgInjectedTokens    int64            `json:"avg_injected_tokens"`
	EstimatedSavedTokens int64            `json:"estimated_saved_tokens"`
	ReasonCounts         map[string]int64 `json:"reason_counts"`
}

func (s *MemoryRecallStats) Record(recalled bool, reason MemoryRecallReason) {
	s.RecordWithSource(recalled, reason, MemoryRecallSourceUnknown)
}

func (s *MemoryRecallStats) RecordWithSource(recalled bool, reason MemoryRecallReason, source MemoryRecallSource) {
	if s == nil {
		return
	}
	s.Total.Add(1)
	if recalled {
		s.Recalled.Add(1)
	} else {
		s.Skipped.Add(1)
	}
	switch reason {
	case MemoryRecallReasonRegenerateSkip:
		s.ReasonRegenerate.Add(1)
	case MemoryRecallReasonAgentMode:
		s.ReasonAgentMode.Add(1)
	case MemoryRecallReasonCompressedTier:
		s.ReasonCompressed.Add(1)
	case MemoryRecallReasonMemoryCue:
		s.ReasonMemoryCue.Add(1)
	default:
		s.ReasonDefault.Add(1)
	}
	switch source {
	case MemoryRecallSourceIM:
		s.TotalIM.Add(1)
		if recalled {
			s.RecalledIM.Add(1)
		} else {
			s.SkippedIM.Add(1)
		}
		recordReasonCounters(reason, &s.ReasonRegenerateIM, &s.ReasonAgentModeIM, &s.ReasonCompressedIM, &s.ReasonMemoryCueIM, &s.ReasonDefaultIM)
	case MemoryRecallSourceSend:
		s.TotalSend.Add(1)
		if recalled {
			s.RecalledSend.Add(1)
		} else {
			s.SkippedSend.Add(1)
		}
		recordReasonCounters(reason, &s.ReasonRegenerateSend, &s.ReasonAgentModeSend, &s.ReasonCompressedSend, &s.ReasonMemoryCueSend, &s.ReasonDefaultSend)
	case MemoryRecallSourceStream:
		s.TotalStream.Add(1)
		if recalled {
			s.RecalledStream.Add(1)
		} else {
			s.SkippedStream.Add(1)
		}
		recordReasonCounters(reason, &s.ReasonRegenerateStream, &s.ReasonAgentModeStream, &s.ReasonCompressedStream, &s.ReasonMemoryCueStream, &s.ReasonDefaultStream)
	}
}

// RecordInjection records actual injected memory tokens for one turn.
func (s *MemoryRecallStats) RecordInjection(tokens int) {
	s.RecordInjectionWithSource(tokens, MemoryRecallSourceUnknown)
}

// RecordInjectionWithSource records actual injected memory tokens with source.
func (s *MemoryRecallStats) RecordInjectionWithSource(tokens int, source MemoryRecallSource) {
	if s == nil || tokens <= 0 {
		return
	}
	s.InjectedContexts.Add(1)
	s.InjectedTokens.Add(int64(tokens))
	switch source {
	case MemoryRecallSourceIM:
		s.InjectedContextsIM.Add(1)
		s.InjectedTokensIM.Add(int64(tokens))
	case MemoryRecallSourceSend:
		s.InjectedContextsSend.Add(1)
		s.InjectedTokensSend.Add(int64(tokens))
	case MemoryRecallSourceStream:
		s.InjectedContextsStream.Add(1)
		s.InjectedTokensStream.Add(int64(tokens))
	}
}

func (s *MemoryRecallStats) Snapshot() MemoryRecallStatsSnapshot {
	if s == nil {
		return MemoryRecallStatsSnapshot{}
	}
	total := s.Total.Load()
	recalled := s.Recalled.Load()
	recallRate := 0.0
	if total > 0 {
		recallRate = float64(recalled) / float64(total)
	}
	injectedContexts := s.InjectedContexts.Load()
	injectedTokens := s.InjectedTokens.Load()
	avgInjected := int64(0)
	if injectedContexts > 0 {
		avgInjected = injectedTokens / injectedContexts
	}
	skipped := s.Skipped.Load()
	return MemoryRecallStatsSnapshot{
		Total:                total,
		Recalled:             recalled,
		Skipped:              skipped,
		RecallRate:           recallRate,
		InjectedContexts:     injectedContexts,
		InjectedTokens:       injectedTokens,
		AvgInjectedTokens:    avgInjected,
		EstimatedSavedTokens: skipped * avgInjected,
		ReasonCounts: map[string]int64{
			string(MemoryRecallReasonRegenerateSkip): s.ReasonRegenerate.Load(),
			string(MemoryRecallReasonAgentMode):      s.ReasonAgentMode.Load(),
			string(MemoryRecallReasonCompressedTier): s.ReasonCompressed.Load(),
			string(MemoryRecallReasonMemoryCue):      s.ReasonMemoryCue.Load(),
			string(MemoryRecallReasonDefaultSkip):    s.ReasonDefault.Load(),
		},
		BySource: map[string]MemoryRecallSourceSnapshot{
			string(MemoryRecallSourceIM): sourceSnapshot(
				s.TotalIM.Load(), s.RecalledIM.Load(), s.SkippedIM.Load(), s.InjectedContextsIM.Load(), s.InjectedTokensIM.Load(),
				s.ReasonRegenerateIM.Load(), s.ReasonAgentModeIM.Load(), s.ReasonCompressedIM.Load(), s.ReasonMemoryCueIM.Load(), s.ReasonDefaultIM.Load(),
			),
			string(MemoryRecallSourceSend): sourceSnapshot(
				s.TotalSend.Load(), s.RecalledSend.Load(), s.SkippedSend.Load(), s.InjectedContextsSend.Load(), s.InjectedTokensSend.Load(),
				s.ReasonRegenerateSend.Load(), s.ReasonAgentModeSend.Load(), s.ReasonCompressedSend.Load(), s.ReasonMemoryCueSend.Load(), s.ReasonDefaultSend.Load(),
			),
			string(MemoryRecallSourceStream): sourceSnapshot(
				s.TotalStream.Load(), s.RecalledStream.Load(), s.SkippedStream.Load(), s.InjectedContextsStream.Load(), s.InjectedTokensStream.Load(),
				s.ReasonRegenerateStream.Load(), s.ReasonAgentModeStream.Load(), s.ReasonCompressedStream.Load(), s.ReasonMemoryCueStream.Load(), s.ReasonDefaultStream.Load(),
			),
		},
	}
}

func sourceSnapshot(
	total, recalled, skipped, injectedContexts, injectedTokens int64,
	reasonRegenerate, reasonAgentMode, reasonCompressed, reasonMemoryCue, reasonDefault int64,
) MemoryRecallSourceSnapshot {
	recallRate := 0.0
	if total > 0 {
		recallRate = float64(recalled) / float64(total)
	}
	avgInjected := int64(0)
	if injectedContexts > 0 {
		avgInjected = injectedTokens / injectedContexts
	}
	return MemoryRecallSourceSnapshot{
		Total:                total,
		Recalled:             recalled,
		Skipped:              skipped,
		RecallRate:           recallRate,
		InjectedContexts:     injectedContexts,
		InjectedTokens:       injectedTokens,
		AvgInjectedTokens:    avgInjected,
		EstimatedSavedTokens: skipped * avgInjected,
		ReasonCounts: map[string]int64{
			string(MemoryRecallReasonRegenerateSkip): reasonRegenerate,
			string(MemoryRecallReasonAgentMode):      reasonAgentMode,
			string(MemoryRecallReasonCompressedTier): reasonCompressed,
			string(MemoryRecallReasonMemoryCue):      reasonMemoryCue,
			string(MemoryRecallReasonDefaultSkip):    reasonDefault,
		},
	}
}

func recordReasonCounters(
	reason MemoryRecallReason,
	reasonRegenerate, reasonAgentMode, reasonCompressed, reasonMemoryCue, reasonDefault *atomic.Int64,
) {
	switch reason {
	case MemoryRecallReasonRegenerateSkip:
		reasonRegenerate.Add(1)
	case MemoryRecallReasonAgentMode:
		reasonAgentMode.Add(1)
	case MemoryRecallReasonCompressedTier:
		reasonCompressed.Add(1)
	case MemoryRecallReasonMemoryCue:
		reasonMemoryCue.Add(1)
	default:
		reasonDefault.Add(1)
	}
}

// Reset clears all counters. Used for short-window measurement.
func (s *MemoryRecallStats) Reset() {
	if s == nil {
		return
	}
	*s = MemoryRecallStats{}
}

// classifyContext determines which context tier to use.
// Pure regex/keyword matching, < 1ms, no LLM calls.
func classifyContext(userMessage string, messageCount int, isAgentMode, isRegenerate bool) ContextTier {
	// First message or empty conversation
	if messageCount <= 1 {
		return TierNoHistory
	}

	// Agent mode always needs context for tool continuity
	if isAgentMode {
		if messageCount > 6 {
			return TierCompressedMemory
		}
		return TierRecentOnly
	}

	// Regenerate needs the original message context
	if isRegenerate {
		return TierRecentOnly
	}

	hasRef := hasReference(userMessage)

	// Short conversation (≤3 rounds): always include recent context
	if messageCount <= 6 {
		return TierRecentOnly
	}

	// Long conversation (> 3 rounds)
	if hasRef {
		return TierCompressedMemory
	}

	// No references in a long conversation → fresh question
	return TierNoHistory
}

// hasReference checks if the message contains reference/continuity markers.
func hasReference(msg string) bool {
	return reChineseRef.MatchString(msg) ||
		reChineseEllipsisRef.MatchString(msg) ||
		reEnglishRef.MatchString(msg) ||
		reContinuation.MatchString(msg)
}

func hasMemoryCue(msg string) bool {
	return reMemoryCue.MatchString(msg)
}

func parseMemoryRecallMode(mode string) MemoryRecallMode {
	switch MemoryRecallMode(mode) {
	case MemoryRecallModeAggressive, MemoryRecallModeQuality:
		return MemoryRecallMode(mode)
	default:
		return MemoryRecallModeBalanced
	}
}

type memoryRecallLimits struct {
	MaxResults int
	ChunkRunes int
	TotalRunes int
}

func recallLimitsForMode(mode MemoryRecallMode) memoryRecallLimits {
	switch parseMemoryRecallMode(string(mode)) {
	case MemoryRecallModeAggressive:
		return memoryRecallLimits{MaxResults: 3, ChunkRunes: 160, TotalRunes: 480}
	case MemoryRecallModeQuality:
		return memoryRecallLimits{MaxResults: 6, ChunkRunes: 280, TotalRunes: 1200}
	default:
		return memoryRecallLimits{MaxResults: 5, ChunkRunes: 200, TotalRunes: 800}
	}
}

func recallMinScoreForMode(mode MemoryRecallMode) float64 {
	switch parseMemoryRecallMode(string(mode)) {
	case MemoryRecallModeAggressive:
		return 0.65
	case MemoryRecallModeQuality:
		return 0.40
	default:
		return 0.50
	}
}

func memoryRecallDecision(userMessage string, tier ContextTier, isAgentMode, isRegenerate bool, mode MemoryRecallMode) (bool, MemoryRecallReason) {
	if isRegenerate {
		return false, MemoryRecallReasonRegenerateSkip
	}
	if isAgentMode {
		return true, MemoryRecallReasonAgentMode
	}
	switch parseMemoryRecallMode(string(mode)) {
	case MemoryRecallModeAggressive:
		if hasMemoryCue(userMessage) {
			return true, MemoryRecallReasonMemoryCue
		}
		return false, MemoryRecallReasonDefaultSkip
	case MemoryRecallModeQuality:
		if tier == TierCompressedMemory || tier == TierRecentOnly {
			return true, MemoryRecallReasonCompressedTier
		}
		if hasMemoryCue(userMessage) {
			return true, MemoryRecallReasonMemoryCue
		}
		return false, MemoryRecallReasonDefaultSkip
	default:
		if tier == TierCompressedMemory {
			return true, MemoryRecallReasonCompressedTier
		}
		if hasMemoryCue(userMessage) {
			return true, MemoryRecallReasonMemoryCue
		}
		return false, MemoryRecallReasonDefaultSkip
	}
}

// shouldRecallMemories decides whether to inject memory context for this turn.
// It keeps recall for context-heavy turns, while skipping most standalone queries
// to reduce token usage.
func shouldRecallMemories(userMessage string, tier ContextTier, isAgentMode, isRegenerate bool, mode MemoryRecallMode) bool {
	should, _ := memoryRecallDecision(userMessage, tier, isAgentMode, isRegenerate, mode)
	return should
}

// --- Message Extraction ---

// convertToLLMMessages converts memory.Message slice to llm.Message slice.
func convertToLLMMessages(messages []memory.Message) []llm.Message {
	result := make([]llm.Message, 0, len(messages))
	for _, msg := range messages {
		m := llm.Message{
			Role:       llm.Role(msg.Role),
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}
		for _, tc := range msg.ToolCalls {
			m.ToolCalls = append(m.ToolCalls, llm.ToolCall{
				ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments,
			})
		}
		result = append(result, m)
	}
	return result
}

// extractRecentRounds extracts the last N user-assistant rounds from messages,
// preserving tool call/result pairs within each round.
// A "round" starts at a "user" message and includes everything until the next "user".
func extractRecentRounds(messages []memory.Message, rounds int) []llm.Message {
	if len(messages) == 0 || rounds <= 0 {
		return nil
	}

	// Walk backwards to find round boundaries
	roundCount := 0
	startIdx := len(messages)

	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			roundCount++
			if roundCount > rounds {
				break
			}
			startIdx = i
		}
	}

	result := convertToLLMMessages(messages[startIdx:])

	// Sanitize: remove orphaned tool_result messages whose corresponding
	// tool_use (in an assistant message) was truncated by the round boundary.
	// Without this, Anthropic API returns "tool_result with tool_use_id has no
	// corresponding tool_use in previous messages".
	return removeOrphanedToolResults(result)
}

// removeOrphanedToolResults drops tool-role messages whose ToolCallID
// doesn't match any tool_use ID in a preceding assistant message.
func removeOrphanedToolResults(msgs []llm.Message) []llm.Message {
	// Collect all tool_use IDs from assistant messages.
	toolUseIDs := make(map[string]struct{})
	for _, m := range msgs {
		if m.Role == llm.RoleAssistant {
			for _, tc := range m.ToolCalls {
				toolUseIDs[tc.ID] = struct{}{}
			}
		}
	}

	// Fast path: if no tool messages at all, return as-is.
	hasTool := false
	for _, m := range msgs {
		if m.Role == llm.RoleTool {
			hasTool = true
			break
		}
	}
	if !hasTool {
		return msgs
	}

	filtered := make([]llm.Message, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == llm.RoleTool && m.ToolCallID != "" {
			if _, ok := toolUseIDs[m.ToolCallID]; !ok {
				continue // orphaned tool_result, skip
			}
		}
		filtered = append(filtered, m)
	}
	return filtered
}

// extractLatestTurn returns only the newest user turn (from the last user message
// to the end). For fresh standalone questions we intentionally avoid replaying
// older history/tool traces.
func extractLatestTurn(messages []memory.Message) []llm.Message {
	if len(messages) == 0 {
		return nil
	}
	start := len(messages) - 1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			start = i
			break
		}
	}
	result := convertToLLMMessages(messages[start:])
	return removeOrphanedToolResults(result)
}

// --- Smart Context Builder ---

// smartContextParams holds the inputs for buildSmartContext.
type smartContextParams struct {
	ConvID       string
	UserMessage  string
	IsRegenerate bool
	// If non-nil, use these instead of fetching from DB/cache.
	PreloadedMessages []memory.Message
}

// buildSmartContext applies the 3-tier context strategy and returns
// the appropriate messages to send to the LLM.
// Does NOT inject system prompt or memory recall — those are handled by the caller.
func (h *ChatHandler) buildSmartContext(ctx context.Context, params smartContextParams) ContextStrategyResult {
	isAgentMode := h.settingsHandler != nil && h.settingsHandler.GetAgentMode()

	// 1. Get messages
	var messages []memory.Message
	if params.PreloadedMessages != nil {
		messages = params.PreloadedMessages
	} else {
		cached, hit := h.conversationCache.Get(params.ConvID)
		if hit {
			messages = cached
		} else {
			var err error
			messages, err = h.store.GetMessages(ctx, params.ConvID, 50, 0)
			if err != nil {
				logger.Warn().Err(err).Str("conv_id", params.ConvID).Msg("[context] failed to fetch messages")
				return ContextStrategyResult{Tier: TierNoHistory}
			}
			h.conversationCache.Set(params.ConvID, messages)
		}
	}

	messageCount := len(messages)

	// 2. Classify
	tier := classifyContext(params.UserMessage, messageCount, isAgentMode, params.IsRegenerate)

	logger.Debug().
		Str("conv_id", params.ConvID).
		Str("tier", tier.String()).
		Int("message_count", messageCount).
		Bool("agent_mode", isAgentMode).
		Msg("[context] classified")

	// 3. Build context based on tier
	var result ContextStrategyResult
	result.Tier = tier

	switch tier {
	case TierNoHistory:
		// No history context needed: include only the latest turn so stale tool
		// traces from earlier rounds are not replayed.
		result.Messages = extractLatestTurn(messages)

	case TierRecentOnly:
		result.Messages = extractRecentRounds(messages, 2)

	case TierCompressedMemory:
		recentMessages := extractRecentRounds(messages, 2)

		// Try cached summary
		var summaryText string
		if h.summaryCache != nil {
			if cached, ok := h.summaryCache.Get(params.ConvID); ok {
				summaryText = cached.(*ConversationSummary).Text
			}
		}

		if summaryText == "" {
			// Generate summary synchronously (first time only)
			summaryText = h.generateSummarySync(ctx, params.ConvID, messages, recentMessages)
		}

		if summaryText != "" {
			result.Summary = summaryText
			summaryMsg := llm.Message{
				Role:    llm.RoleSystem,
				Content: "Previous conversation context: " + summaryText,
			}
			result.Messages = append([]llm.Message{summaryMsg}, recentMessages...)
		} else {
			// Fallback: just use recent messages
			result.Messages = recentMessages
		}
	}

	return result
}

// summaryCustomInstructions is the prompt for generating compressed summaries.
const summaryCustomInstructions = "Summarize in 1-2 sentences (30-50 tokens max). " +
	"Focus on: current topic, key decisions, open questions. " +
	"Do NOT include greetings or meta-commentary."

// generateSummarySync generates a compressed summary for older messages.
// Called synchronously when no cached summary exists.
func (h *ChatHandler) generateSummarySync(ctx context.Context, convID string, allMessages []memory.Message, recentLLM []llm.Message) string {
	recentCount := len(recentLLM)
	olderCount := len(allMessages) - recentCount
	if olderCount <= 0 {
		return ""
	}

	olderMessages := convertToLLMMessages(allMessages[:olderCount])
	if len(olderMessages) == 0 {
		return ""
	}

	if h.proxyBridge == nil {
		return ""
	}

	provider := &bridgeProvider{bridge: h.proxyBridge, model: "auto"}
	compactor := claudecode.NewCompactor(claudecode.CompactionConfig{
		MaxContextTokens:   4096,
		MaxHistoryShare:    1.0,
		ReserveTokens:      100,
		CustomInstructions: summaryCustomInstructions,
	}, provider)

	summary, err := compactor.Summarize(ctx, olderMessages, "")
	if err != nil || summary == claudecode.DefaultSummaryFallback {
		return ""
	}

	// Cache it
	if h.summaryCache != nil {
		h.summaryCache.Put(convID, &ConversationSummary{
			Text:         summary,
			MessageCount: len(allMessages),
		})
	}

	return summary
}

// refreshSummaryAsync asynchronously updates the summary cache for a conversation.
// Called after each response in long conversations.
func (h *ChatHandler) refreshSummaryAsync(convID string, messages []memory.Message) {
	if h.summaryCache == nil || h.proxyBridge == nil || len(messages) <= 6 {
		return
	}

	// Check if cached summary is still fresh
	if cached, ok := h.summaryCache.Get(convID); ok {
		cs := cached.(*ConversationSummary)
		if cs.MessageCount >= len(messages)-2 {
			return // Within 1 round, still fresh
		}
	}

	// Copy messages for goroutine safety
	msgCopy := make([]memory.Message, len(messages))
	copy(msgCopy, messages)

	h.queueEvent(func() {
		llmMessages := convertToLLMMessages(msgCopy)
		if len(llmMessages) <= 4 {
			return
		}

		provider := &bridgeProvider{bridge: h.proxyBridge, model: "auto"}
		compactor := claudecode.NewCompactor(claudecode.CompactionConfig{
			MaxContextTokens:   4096,
			MaxHistoryShare:    1.0,
			ReserveTokens:      100,
			CustomInstructions: summaryCustomInstructions,
		}, provider)

		summary, err := compactor.Summarize(context.Background(), llmMessages, "")
		if err != nil || summary == claudecode.DefaultSummaryFallback {
			return
		}

		h.summaryCache.Put(convID, &ConversationSummary{
			Text:         summary,
			MessageCount: len(msgCopy),
		})

		logger.Debug().Str("conv_id", convID).Int("messages", len(msgCopy)).Msg("[context] summary refreshed async")
	})
}
