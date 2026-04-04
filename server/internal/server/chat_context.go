package server

import (
	"context"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/logger"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// ContextTier represents the context strategy.
type ContextTier int

const (
	// TierNoHistory: standalone question, no history needed.
	TierNoHistory ContextTier = iota
	// TierFullHistory: continuation detected, but raw history still fits budget.
	TierFullHistory
	// TierCompressedMemory: needs older context, inject compressed summary.
	TierCompressedMemory
)

func (t ContextTier) String() string {
	switch t {
	case TierNoHistory:
		return "no_history"
	case TierFullHistory:
		return "full_history"
	case TierCompressedMemory:
		return "compressed_memory"
	default:
		return "unknown"
	}
}

// ContextStrategyResult holds the output of buildSmartContext.
type ContextStrategyResult struct {
	Tier               ContextTier
	Messages           []llm.Message // Context messages (excluding system prompt)
	Summary            string        // Non-empty only for TierCompressedMemory
	MessageCountBefore int           // Conversation messages seen before smart-context selection
	MessageCountAfter  int           // Context messages kept after smart-context selection/pruning
}

// ConversationSummary is the cached summary for a conversation.
type ConversationSummary struct {
	Text         string
	MessageCount int
}

const compressedTierRecentRounds = 3
const smartContextSoftCompressionThreshold = 0.85

const (
	trimPolicyCharsPerTokenEstimate = 4
	trimPolicyImageCharEstimate     = 8000
)

type trimPolicyToolMatch struct {
	Allow []string
	Deny  []string
}

type trimPolicySoftTrimSettings struct {
	MaxChars  int
	HeadChars int
	TailChars int
}

type trimPolicyHardClearSettings struct {
	Enabled     bool
	Placeholder string
}

type trimPolicyPruneSettings struct {
	KeepLastAssistants   int
	SoftTrimRatio        float64
	HardClearRatio       float64
	MinPrunableToolChars int
	Tools                trimPolicyToolMatch
	SoftTrim             trimPolicySoftTrimSettings
	HardClear            trimPolicyHardClearSettings
}

type trimPolicyPruneReport struct {
	BeforeChars         int
	AfterChars          int
	ContextWindowChars  int
	BeforeRatio         float64
	AfterRatio          float64
	ExaminedToolResults int
	EligibleToolResults int
	SkippedByToolPolicy int
	SkippedByImage      int
	SoftTrimmed         int
	HardCleared         int
	SoftTrimByTool      map[string]int
	HardClearByTool     map[string]int
}

var trimPolicyDefaultPruneSettings = trimPolicyPruneSettings{
	KeepLastAssistants:   3,
	SoftTrimRatio:        0.3,
	HardClearRatio:       0.5,
	MinPrunableToolChars: 50000,
	Tools:                trimPolicyToolMatch{},
	SoftTrim: trimPolicySoftTrimSettings{
		MaxChars:  4000,
		HeadChars: 1500,
		TailChars: 1500,
	},
	HardClear: trimPolicyHardClearSettings{
		Enabled:     true,
		Placeholder: "[Old tool result content cleared]",
	},
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
		`(?i)(?:\bremember\b|\bmemory\b|\bmemories\b|\bpreference\b|\bprofile\b|` +
			`\bcapability\b|\bcapabilities\b|\bsession\s*query\b|\bquery\s*sessions?\b|\bas i said\b|` +
			`记得|记住|记忆|你还记得|我喜欢|我的偏好|之前说过|个人资料|习惯|会话查询|查询能力|工具能力|能力偏好)`)

	// Topic-switch cues that explicitly ask to stop using prior discussion as
	// continuation context in the current session.
	reTopicSwitchCue = regexp.MustCompile(
		`(?i)(?:` +
			`换个话题|切换话题|换个问题|另起一个问题|重新开始|从头开始|先不说这个|` +
			`忽略之前|忽略前面|抛开之前|我们聊点别的|聊点别的|` +
			`new topic|change (?:the )?topic|switch (?:to )?(?:a )?new topic|` +
			`let'?s talk about something else|ignore (?:the )?previous|` +
			`forget (?:the )?previous|start over|from scratch)`)
)

// MemoryRecallReason indicates why memory recall was triggered or skipped.
type MemoryRecallReason string

const (
	MemoryRecallReasonRegenerateSkip   MemoryRecallReason = "regenerate_skip"
	MemoryRecallReasonContinuationSkip MemoryRecallReason = "continuation_skip"
	MemoryRecallReasonAutomaticTurn    MemoryRecallReason = "automatic_turn"
	MemoryRecallReasonAgentMode        MemoryRecallReason = "agent_mode"
	MemoryRecallReasonCompressedTier   MemoryRecallReason = "compressed_tier"
	MemoryRecallReasonMemoryCue        MemoryRecallReason = "memory_cue"
	MemoryRecallReasonDefaultSkip      MemoryRecallReason = "default_skip"
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
	Total              atomic.Int64
	Recalled           atomic.Int64
	Skipped            atomic.Int64
	InjectedContexts   atomic.Int64
	InjectedTokens     atomic.Int64
	ReasonRegenerate   atomic.Int64
	ReasonContinuation atomic.Int64
	ReasonAutomatic    atomic.Int64
	ReasonAgentMode    atomic.Int64
	ReasonCompressed   atomic.Int64
	ReasonMemoryCue    atomic.Int64
	ReasonDefault      atomic.Int64

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

	ReasonRegenerateIM   atomic.Int64
	ReasonContinuationIM atomic.Int64
	ReasonAutomaticIM    atomic.Int64
	ReasonAgentModeIM    atomic.Int64
	ReasonCompressedIM   atomic.Int64
	ReasonMemoryCueIM    atomic.Int64
	ReasonDefaultIM      atomic.Int64

	ReasonRegenerateSend   atomic.Int64
	ReasonContinuationSend atomic.Int64
	ReasonAutomaticSend    atomic.Int64
	ReasonAgentModeSend    atomic.Int64
	ReasonCompressedSend   atomic.Int64
	ReasonMemoryCueSend    atomic.Int64
	ReasonDefaultSend      atomic.Int64

	ReasonRegenerateStream   atomic.Int64
	ReasonContinuationStream atomic.Int64
	ReasonAutomaticStream    atomic.Int64
	ReasonAgentModeStream    atomic.Int64
	ReasonCompressedStream   atomic.Int64
	ReasonMemoryCueStream    atomic.Int64
	ReasonDefaultStream      atomic.Int64
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
	case MemoryRecallReasonContinuationSkip:
		s.ReasonContinuation.Add(1)
	case MemoryRecallReasonAutomaticTurn:
		s.ReasonAutomatic.Add(1)
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
		recordReasonCounters(reason, &s.ReasonRegenerateIM, &s.ReasonContinuationIM, &s.ReasonAutomaticIM, &s.ReasonAgentModeIM, &s.ReasonCompressedIM, &s.ReasonMemoryCueIM, &s.ReasonDefaultIM)
	case MemoryRecallSourceSend:
		s.TotalSend.Add(1)
		if recalled {
			s.RecalledSend.Add(1)
		} else {
			s.SkippedSend.Add(1)
		}
		recordReasonCounters(reason, &s.ReasonRegenerateSend, &s.ReasonContinuationSend, &s.ReasonAutomaticSend, &s.ReasonAgentModeSend, &s.ReasonCompressedSend, &s.ReasonMemoryCueSend, &s.ReasonDefaultSend)
	case MemoryRecallSourceStream:
		s.TotalStream.Add(1)
		if recalled {
			s.RecalledStream.Add(1)
		} else {
			s.SkippedStream.Add(1)
		}
		recordReasonCounters(reason, &s.ReasonRegenerateStream, &s.ReasonContinuationStream, &s.ReasonAutomaticStream, &s.ReasonAgentModeStream, &s.ReasonCompressedStream, &s.ReasonMemoryCueStream, &s.ReasonDefaultStream)
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
			string(MemoryRecallReasonRegenerateSkip):   s.ReasonRegenerate.Load(),
			string(MemoryRecallReasonContinuationSkip): s.ReasonContinuation.Load(),
			string(MemoryRecallReasonAutomaticTurn):    s.ReasonAutomatic.Load(),
			string(MemoryRecallReasonAgentMode):        s.ReasonAgentMode.Load(),
			string(MemoryRecallReasonCompressedTier):   s.ReasonCompressed.Load(),
			string(MemoryRecallReasonMemoryCue):        s.ReasonMemoryCue.Load(),
			string(MemoryRecallReasonDefaultSkip):      s.ReasonDefault.Load(),
		},
		BySource: map[string]MemoryRecallSourceSnapshot{
			string(MemoryRecallSourceIM): sourceSnapshot(
				s.TotalIM.Load(), s.RecalledIM.Load(), s.SkippedIM.Load(), s.InjectedContextsIM.Load(), s.InjectedTokensIM.Load(),
				s.ReasonRegenerateIM.Load(), s.ReasonContinuationIM.Load(), s.ReasonAutomaticIM.Load(), s.ReasonAgentModeIM.Load(), s.ReasonCompressedIM.Load(), s.ReasonMemoryCueIM.Load(), s.ReasonDefaultIM.Load(),
			),
			string(MemoryRecallSourceSend): sourceSnapshot(
				s.TotalSend.Load(), s.RecalledSend.Load(), s.SkippedSend.Load(), s.InjectedContextsSend.Load(), s.InjectedTokensSend.Load(),
				s.ReasonRegenerateSend.Load(), s.ReasonContinuationSend.Load(), s.ReasonAutomaticSend.Load(), s.ReasonAgentModeSend.Load(), s.ReasonCompressedSend.Load(), s.ReasonMemoryCueSend.Load(), s.ReasonDefaultSend.Load(),
			),
			string(MemoryRecallSourceStream): sourceSnapshot(
				s.TotalStream.Load(), s.RecalledStream.Load(), s.SkippedStream.Load(), s.InjectedContextsStream.Load(), s.InjectedTokensStream.Load(),
				s.ReasonRegenerateStream.Load(), s.ReasonContinuationStream.Load(), s.ReasonAutomaticStream.Load(), s.ReasonAgentModeStream.Load(), s.ReasonCompressedStream.Load(), s.ReasonMemoryCueStream.Load(), s.ReasonDefaultStream.Load(),
			),
		},
	}
}

func sourceSnapshot(
	total, recalled, skipped, injectedContexts, injectedTokens int64,
	reasonRegenerate, reasonContinuation, reasonAutomatic, reasonAgentMode, reasonCompressed, reasonMemoryCue, reasonDefault int64,
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
			string(MemoryRecallReasonRegenerateSkip):   reasonRegenerate,
			string(MemoryRecallReasonContinuationSkip): reasonContinuation,
			string(MemoryRecallReasonAutomaticTurn):    reasonAutomatic,
			string(MemoryRecallReasonAgentMode):        reasonAgentMode,
			string(MemoryRecallReasonCompressedTier):   reasonCompressed,
			string(MemoryRecallReasonMemoryCue):        reasonMemoryCue,
			string(MemoryRecallReasonDefaultSkip):      reasonDefault,
		},
	}
}

func recordReasonCounters(
	reason MemoryRecallReason,
	reasonRegenerate, reasonContinuation, reasonAutomatic, reasonAgentMode, reasonCompressed, reasonMemoryCue, reasonDefault *atomic.Int64,
) {
	switch reason {
	case MemoryRecallReasonRegenerateSkip:
		reasonRegenerate.Add(1)
	case MemoryRecallReasonContinuationSkip:
		reasonContinuation.Add(1)
	case MemoryRecallReasonAutomaticTurn:
		reasonAutomatic.Add(1)
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
	_ = userMessage
	_ = isAgentMode
	_ = isRegenerate

	// First message or empty conversation
	if messageCount <= 1 {
		return TierNoHistory
	}
	return TierFullHistory
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

func hasTopicSwitchCue(msg string) bool {
	trimmed := strings.TrimSpace(msg)
	if trimmed == "" {
		return false
	}
	return reTopicSwitchCue.MatchString(trimmed)
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

func memoryRecallDecision(userMessage string, usesContinuation, isRegenerate bool, mode MemoryRecallMode) (bool, MemoryRecallReason) {
	_ = userMessage
	_ = mode
	if isRegenerate {
		return false, MemoryRecallReasonRegenerateSkip
	}
	if usesContinuation {
		return false, MemoryRecallReasonContinuationSkip
	}
	return true, MemoryRecallReasonAutomaticTurn
}

// shouldRecallMemories decides whether to inject memory context for this turn.
// It keeps recall for context-heavy turns, while skipping most standalone queries
// to reduce token usage.
func shouldRecallMemories(userMessage string, _ ContextTier, _ bool, isRegenerate bool, mode MemoryRecallMode) bool {
	should, _ := memoryRecallDecision(userMessage, false, isRegenerate, mode)
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
			ToolName:   msg.ToolName,
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

func splitConversationByRecentUserTurns(messages []memory.Message, rounds int) (older, recent []llm.Message) {
	if len(messages) == 0 {
		return nil, nil
	}
	if rounds <= 0 {
		return convertToLLMMessages(messages), nil
	}

	roundCount := 0
	startIdx := len(messages)
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "user" {
			continue
		}
		roundCount++
		if roundCount > rounds {
			break
		}
		startIdx = i
	}

	older = convertToLLMMessages(messages[:startIdx])
	recent = removeOrphanedToolResults(convertToLLMMessages(messages[startIdx:]))
	return older, recent
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

func trimPolicyEstimateMessageChars(msg llm.Message) int {
	estimateParts := func(parts []llm.ContentPart) int {
		chars := 0
		for _, p := range parts {
			switch p.Type {
			case "text":
				chars += len([]rune(p.Text))
			case "image":
				chars += trimPolicyImageCharEstimate
			}
		}
		return chars
	}

	switch msg.Role {
	case llm.RoleUser:
		if len(msg.ContentParts) > 0 {
			return estimateParts(msg.ContentParts)
		}
		return len([]rune(msg.Content))
	case llm.RoleAssistant:
		chars := len([]rune(msg.Content))
		if len(msg.ContentParts) > 0 {
			chars += estimateParts(msg.ContentParts)
		}
		for _, tc := range msg.ToolCalls {
			chars += len([]rune(tc.Arguments))
			chars += len([]rune(tc.Name))
		}
		return chars
	case llm.RoleTool:
		if len(msg.ContentParts) > 0 {
			return estimateParts(msg.ContentParts)
		}
		return len([]rune(msg.Content))
	default:
		if msg.Content != "" {
			return len([]rune(msg.Content))
		}
		return 256
	}
}

func trimPolicyEstimateContextChars(messages []llm.Message) int {
	total := 0
	for _, m := range messages {
		total += trimPolicyEstimateMessageChars(m)
	}
	return total
}

func trimPolicyFindAssistantCutoffIndex(messages []llm.Message, keepLastAssistants int) (int, bool) {
	if keepLastAssistants <= 0 {
		return len(messages), true
	}
	remaining := keepLastAssistants
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != llm.RoleAssistant {
			continue
		}
		remaining--
		if remaining == 0 {
			return i, true
		}
	}
	return 0, false
}

func trimPolicyFirstUserIndex(messages []llm.Message) int {
	for i := 0; i < len(messages); i++ {
		if messages[i].Role == llm.RoleUser {
			return i
		}
	}
	return -1
}

func trimPolicyHasImageParts(msg llm.Message) bool {
	for _, p := range msg.ContentParts {
		if p.Type == "image" {
			return true
		}
	}
	return false
}

func trimPolicyClampRatio(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func trimPolicyNormalizePruneSettings(settings trimPolicyPruneSettings) trimPolicyPruneSettings {
	if settings.KeepLastAssistants < 0 {
		settings.KeepLastAssistants = 0
	}
	settings.SoftTrimRatio = trimPolicyClampRatio(settings.SoftTrimRatio)
	settings.HardClearRatio = trimPolicyClampRatio(settings.HardClearRatio)
	if settings.MinPrunableToolChars < 0 {
		settings.MinPrunableToolChars = 0
	}
	if settings.SoftTrim.MaxChars < 0 {
		settings.SoftTrim.MaxChars = 0
	}
	if settings.SoftTrim.HeadChars < 0 {
		settings.SoftTrim.HeadChars = 0
	}
	if settings.SoftTrim.TailChars < 0 {
		settings.SoftTrim.TailChars = 0
	}
	if strings.TrimSpace(settings.HardClear.Placeholder) == "" {
		settings.HardClear.Placeholder = trimPolicyDefaultPruneSettings.HardClear.Placeholder
	}
	return settings
}

func trimPolicyNormalizeGlob(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func trimPolicyNormalizeGlobs(globs []string) []string {
	if len(globs) == 0 {
		return nil
	}
	out := make([]string, 0, len(globs))
	for _, g := range globs {
		if s := trimPolicyNormalizeGlob(g); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func trimPolicyMatchesAnyGlob(value string, globs []string) bool {
	for _, g := range globs {
		if ok, err := filepath.Match(g, value); err == nil && ok {
			return true
		}
	}
	return false
}

func trimPolicyMakeToolPrunablePredicate(match trimPolicyToolMatch) func(string) bool {
	deny := trimPolicyNormalizeGlobs(match.Deny)
	allow := trimPolicyNormalizeGlobs(match.Allow)
	return func(toolName string) bool {
		normalized := trimPolicyNormalizeGlob(toolName)
		if trimPolicyMatchesAnyGlob(normalized, deny) {
			return false
		}
		if len(allow) == 0 {
			return true
		}
		return trimPolicyMatchesAnyGlob(normalized, allow)
	}
}

func trimPolicyToolNameForMessage(msg llm.Message, toolCallNameIndex map[string]string) string {
	if name := strings.TrimSpace(msg.ToolName); name != "" {
		return name
	}
	if id := strings.TrimSpace(msg.ToolCallID); id != "" {
		if name := strings.TrimSpace(toolCallNameIndex[id]); name != "" {
			return name
		}
	}
	return strings.TrimSpace(detectToolNameFromToolResultContent(msg.Content))
}

func trimPolicySemanticToolMessage(msg llm.Message, toolName string, byteBudget int) (llm.Message, bool) {
	switch strings.ToLower(strings.TrimSpace(toolName)) {
	case "web_query", "web":
		compacted := compactWebQueryContentForLLM(nil, msg.Content, msg.Content, strings.TrimSpace(msg.ToolCallID), byteBudget, tools.WebQueryLLMCompactionMinimal, true)
		if compacted == "" || compacted == msg.Content {
			return msg, false
		}
		msg.Content = compacted
		msg.ContentParts = nil
		return msg, true
	default:
		return msg, false
	}
}

func trimPolicySoftTrimToolMessage(msg llm.Message, toolName string, settings trimPolicyPruneSettings) (llm.Message, bool) {
	if msg.Role != llm.RoleTool {
		return msg, false
	}
	if trimPolicyHasImageParts(msg) {
		return msg, false
	}
	text := strings.TrimSpace(msg.Content)
	if text == "" && len(msg.ContentParts) > 0 {
		parts := make([]string, 0, len(msg.ContentParts))
		for _, p := range msg.ContentParts {
			if p.Type == "text" {
				parts = append(parts, p.Text)
			}
		}
		text = strings.Join(parts, "\n")
	}
	runes := []rune(text)
	rawLen := len(runes)
	if rawLen <= settings.SoftTrim.MaxChars {
		return msg, false
	}
	if updated, changed := trimPolicySemanticToolMessage(msg, toolName, maxLLMSearchSummaryBytes); changed {
		return updated, true
	}
	if settings.SoftTrim.HeadChars+settings.SoftTrim.TailChars >= rawLen {
		return msg, false
	}
	head := string(runes[:settings.SoftTrim.HeadChars])
	tail := string(runes[rawLen-settings.SoftTrim.TailChars:])
	trimmed := head + "\n...\n" + tail
	trimmed += "\n\n[Tool result trimmed: kept first "
	trimmed += intToString(settings.SoftTrim.HeadChars)
	trimmed += " chars and last "
	trimmed += intToString(settings.SoftTrim.TailChars)
	trimmed += " chars of "
	trimmed += intToString(rawLen)
	trimmed += " chars.]"

	msg.Content = trimmed
	msg.ContentParts = nil
	return msg, true
}

func intToString(v int) string {
	// Avoid importing strconv in this hot path file.
	if v == 0 {
		return "0"
	}
	if v < 0 {
		return "-" + intToString(-v)
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

func trimPolicyPruneContextMessages(messages []llm.Message, contextWindowTokens int) []llm.Message {
	out, _ := trimPolicyPruneContextMessagesWithReport(messages, contextWindowTokens, trimPolicyDefaultPruneSettings)
	return out
}

func trimPolicyPruneContextMessagesWithSettings(messages []llm.Message, contextWindowTokens int, settings trimPolicyPruneSettings) []llm.Message {
	out, _ := trimPolicyPruneContextMessagesWithReport(messages, contextWindowTokens, settings)
	return out
}

func trimPolicyPruneContextMessagesWithReport(
	messages []llm.Message,
	contextWindowTokens int,
	settings trimPolicyPruneSettings,
) ([]llm.Message, trimPolicyPruneReport) {
	settings = trimPolicyNormalizePruneSettings(settings)
	report := trimPolicyPruneReport{}
	if len(messages) == 0 || contextWindowTokens <= 0 {
		return messages, report
	}
	charWindow := contextWindowTokens * trimPolicyCharsPerTokenEstimate
	report.ContextWindowChars = charWindow
	if charWindow <= 0 {
		return messages, report
	}

	cutoffIndex, ok := trimPolicyFindAssistantCutoffIndex(messages, settings.KeepLastAssistants)
	if !ok {
		return messages, report
	}
	firstUser := trimPolicyFirstUserIndex(messages)
	pruneStart := len(messages)
	if firstUser >= 0 {
		pruneStart = firstUser
	}

	totalChars := trimPolicyEstimateContextChars(messages)
	report.BeforeChars = totalChars
	ratio := float64(totalChars) / float64(charWindow)
	report.BeforeRatio = ratio
	report.AfterChars = totalChars
	report.AfterRatio = ratio
	if ratio < settings.SoftTrimRatio {
		return messages, report
	}

	prunableToolIndexes := make([]int, 0)
	isToolPrunable := trimPolicyMakeToolPrunablePredicate(settings.Tools)
	toolCallNameIndex := buildToolCallNameIndex(messages)
	prunableToolNames := make(map[int]string)
	var next []llm.Message
	for i := pruneStart; i < cutoffIndex; i++ {
		msg := messages[i]
		if msg.Role != llm.RoleTool {
			continue
		}
		report.ExaminedToolResults++
		toolName := trimPolicyToolNameForMessage(msg, toolCallNameIndex)
		if !isToolPrunable(toolName) {
			report.SkippedByToolPolicy++
			continue
		}
		if trimPolicyHasImageParts(msg) {
			report.SkippedByImage++
			continue
		}
		report.EligibleToolResults++
		prunableToolIndexes = append(prunableToolIndexes, i)
		prunableToolNames[i] = toolName
		updated, changed := trimPolicySoftTrimToolMessage(msg, toolName, settings)
		if !changed {
			continue
		}
		report.SoftTrimmed++
		if report.SoftTrimByTool == nil {
			report.SoftTrimByTool = make(map[string]int)
		}
		report.SoftTrimByTool[toolName]++
		beforeChars := trimPolicyEstimateMessageChars(msg)
		afterChars := trimPolicyEstimateMessageChars(updated)
		totalChars += afterChars - beforeChars
		if next == nil {
			next = make([]llm.Message, len(messages))
			copy(next, messages)
		}
		next[i] = updated
	}

	outputAfterSoftTrim := messages
	if next != nil {
		outputAfterSoftTrim = next
	}
	ratio = float64(totalChars) / float64(charWindow)
	report.AfterChars = totalChars
	report.AfterRatio = ratio
	if ratio < settings.HardClearRatio || !settings.HardClear.Enabled {
		return outputAfterSoftTrim, report
	}

	prunableToolChars := 0
	for _, i := range prunableToolIndexes {
		msg := outputAfterSoftTrim[i]
		if msg.Role != llm.RoleTool {
			continue
		}
		prunableToolChars += trimPolicyEstimateMessageChars(msg)
	}
	if prunableToolChars < settings.MinPrunableToolChars {
		return outputAfterSoftTrim, report
	}

	if next == nil {
		next = make([]llm.Message, len(messages))
		copy(next, messages)
	}
	for _, i := range prunableToolIndexes {
		if ratio < settings.HardClearRatio {
			break
		}
		msg := next[i]
		if msg.Role != llm.RoleTool {
			continue
		}
		if updated, changed := trimPolicySemanticToolMessage(msg, prunableToolNames[i], 640); changed {
			beforeChars := trimPolicyEstimateMessageChars(msg)
			next[i] = updated
			if report.SoftTrimByTool == nil {
				report.SoftTrimByTool = make(map[string]int)
			}
			report.SoftTrimmed++
			report.SoftTrimByTool[prunableToolNames[i]]++
			afterChars := trimPolicyEstimateMessageChars(updated)
			totalChars += afterChars - beforeChars
			ratio = float64(totalChars) / float64(charWindow)
			report.AfterChars = totalChars
			report.AfterRatio = ratio
			continue
		}
		beforeChars := trimPolicyEstimateMessageChars(msg)
		msg.Content = settings.HardClear.Placeholder
		msg.ContentParts = nil
		next[i] = msg
		report.HardCleared++
		if report.HardClearByTool == nil {
			report.HardClearByTool = make(map[string]int)
		}
		report.HardClearByTool[prunableToolNames[i]]++
		afterChars := trimPolicyEstimateMessageChars(msg)
		totalChars += afterChars - beforeChars
		ratio = float64(totalChars) / float64(charWindow)
		report.AfterChars = totalChars
		report.AfterRatio = ratio
	}

	return next, report
}

func trimPolicyTopToolCounters(counter map[string]int, limit int) []string {
	if len(counter) == 0 {
		return nil
	}
	if limit <= 0 {
		limit = 3
	}
	type pair struct {
		name  string
		count int
	}
	pairs := make([]pair, 0, len(counter))
	for name, count := range counter {
		if strings.TrimSpace(name) == "" || count <= 0 {
			continue
		}
		pairs = append(pairs, pair{name: name, count: count})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count == pairs[j].count {
			return pairs[i].name < pairs[j].name
		}
		return pairs[i].count > pairs[j].count
	})
	if len(pairs) > limit {
		pairs = pairs[:limit]
	}
	out := make([]string, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, p.name+":"+intToString(p.count))
	}
	return out
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
	Model        string
	MaxTokens    int
	IsRegenerate bool
	// When >0 and tier=no_history, keep last N user-assistant rounds instead of
	// only the latest user turn.
	NoHistoryRecentRounds int
	// If non-nil, use these instead of fetching from DB/cache.
	PreloadedMessages []memory.Message
}

func isConversationSummaryFresh(summary *ConversationSummary, messageCount int) bool {
	if summary == nil {
		return false
	}
	if strings.TrimSpace(summary.Text) == "" {
		return false
	}
	return summary.MessageCount == messageCount
}

// buildSmartContext applies the context strategy and returns
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
			messages, err = h.getRecentMessagesForContext(ctx, params.ConvID, h.contextHistoryFetchLimit(params.Model))
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

	// 3. Build context based on tier
	result := ContextStrategyResult{
		Tier:               tier,
		MessageCountBefore: messageCount,
	}

	switch tier {
	case TierNoHistory:
		if params.NoHistoryRecentRounds > 0 {
			// IM-style conservative context: keep a small recent window even when
			// classified as no_history.
			result.Messages = extractRecentRounds(messages, params.NoHistoryRecentRounds)
		} else {
			// No history context needed: include only the latest turn so stale tool
			// traces from earlier rounds are not replayed.
			result.Messages = extractLatestTurn(messages)
		}

	default:
		fullHistory := removeOrphanedToolResults(convertToLLMMessages(messages))
		fullHistoryBudget := h.measurePreparedInputBudget(params.Model, params.MaxTokens, fullHistory)
		if !fullHistoryBudget.NeedsPressureCompaction() {
			result.Tier = TierFullHistory
			result.Messages = fullHistory
			break
		}
		result.Tier = TierCompressedMemory

		// Keep more recent rounds (TrimPolicy-style protected tail) so short-lived
		// decisions and constraints survive summary compression.
		recentMessages := extractRecentRounds(messages, compressedTierRecentRounds)

		// Try cached summary
		var summaryText string
		if h.summaryCache != nil {
			if cached, ok := h.summaryCache.Get(params.ConvID); ok {
				if summary, ok := cached.(*ConversationSummary); ok && isConversationSummaryFresh(summary, len(messages)) {
					summaryText = strings.TrimSpace(summary.Text)
				}
			}
		}

		if summaryText == "" {
			// Generate summary synchronously (first time only)
			summaryText = h.generateSummarySync(ctx, params.ConvID, messages, recentMessages)
		}

		if summaryText != "" {
			result.Summary = summaryText
			historyMsgs := compressedHistoryContextMessages(params.UserMessage, summaryText)
			result.Messages = append(historyMsgs, recentMessages...)
		} else {
			// Fallback: just use recent messages
			result.Messages = recentMessages
		}

		compressedBudget := h.measurePreparedInputBudget(params.Model, params.MaxTokens, result.Messages)
		if compressedBudget.Exceeds() {
			contextWindowTokens := compressedBudget.ContextWindow
			if contextWindowTokens <= 0 {
				contextWindowTokens = h.compactionConfig.MaxContextTokens
			}
			if contextWindowTokens <= 0 {
				contextWindowTokens = agentcore.DefaultContextTokens
			}
			pruneSettings := trimPolicyDefaultPruneSettings
			if h.settingsHandler != nil {
				pruneSettings.Tools.Allow = h.settingsHandler.GetSmallModelContextPruneToolAllow()
				pruneSettings.Tools.Deny = h.settingsHandler.GetSmallModelContextPruneToolDeny()
			}
			pruned, report := trimPolicyPruneContextMessagesWithReport(result.Messages, contextWindowTokens, pruneSettings)
			result.Messages = pruned

			if report.ExaminedToolResults > 0 {
				ev := logger.Debug().
					Str("conv_id", params.ConvID).
					Int("tool_results_examined", report.ExaminedToolResults).
					Int("tool_results_eligible", report.EligibleToolResults).
					Int("skipped_by_tool_policy", report.SkippedByToolPolicy).
					Int("skipped_by_image", report.SkippedByImage).
					Int("soft_trimmed", report.SoftTrimmed).
					Int("hard_cleared", report.HardCleared).
					Int("before_chars", report.BeforeChars).
					Int("after_chars", report.AfterChars)
				if report.ContextWindowChars > 0 {
					ev = ev.Float64("before_ratio", report.BeforeRatio).Float64("after_ratio", report.AfterRatio)
				}
				if top := trimPolicyTopToolCounters(report.SoftTrimByTool, 3); len(top) > 0 {
					ev = ev.Strs("soft_trim_tools", top)
				}
				if top := trimPolicyTopToolCounters(report.HardClearByTool, 3); len(top) > 0 {
					ev = ev.Strs("hard_clear_tools", top)
				}
				ev.Msg("[context] trim policy report")
			}
		}
	}

	result.MessageCountAfter = len(result.Messages)
	finalBudget := h.measurePreparedInputBudget(params.Model, params.MaxTokens, result.Messages)
	logger.Debug().
		Str("conv_id", params.ConvID).
		Str("tier", result.Tier.String()).
		Int("message_count", messageCount).
		Int("message_count_after", result.MessageCountAfter).
		Float64("usage_ratio", finalBudget.ContextUsageRatio()).
		Bool("agent_mode", isAgentMode).
		Msg("[context] classified")

	return result
}

const summaryCustomFocus = "Keep the total under 320 tokens. Include exact filenames, directories, IDs, and dates when they matter."

// generateSummarySync generates a compressed summary for older messages.
// Called synchronously when no cached summary exists.
func (h *ChatHandler) generateSummarySync(ctx context.Context, convID string, allMessages []memory.Message, recentLLM []llm.Message) string {
	olderMessages, splitRecent := splitConversationByRecentUserTurns(allMessages, compressedTierRecentRounds)
	if len(olderMessages) == 0 {
		return ""
	}
	relevantMessages := cloneLLMMessages(olderMessages)
	if len(recentLLM) > 0 {
		relevantMessages = append(relevantMessages, cloneLLMMessages(recentLLM)...)
	} else {
		relevantMessages = append(relevantMessages, cloneLLMMessages(splitRecent)...)
	}

	mode := h.contextCompressionMode()

	cacheSummary := func(summary string) string {
		if strings.TrimSpace(summary) == "" {
			return ""
		}
		if h.summaryCache != nil {
			h.summaryCache.Put(convID, &ConversationSummary{
				Text:         summary,
				MessageCount: len(allMessages),
			})
		}
		return summary
	}

	if mode != "offline" {
		if summary := h.buildSmallModelConversationCompression(ctx, olderMessages, relevantMessages); summary != "" {
			return cacheSummary(summary)
		}
	}

	if mode == "small_model" || mode == "auto" {
		if summary := h.generateConversationSummaryWithSmallModel(ctx, olderMessages); summary != "" {
			return cacheSummary(summary)
		}
	}

	if summary := h.buildOfflineConversationCompression(olderMessages, relevantMessages); summary != "" {
		return cacheSummary(summary)
	}

	if h.proxyBridge == nil {
		return ""
	}

	provider := &bridgeProvider{bridge: h.proxyBridge, model: "auto"}
	compactor := agentcore.NewCompactor(agentcore.CompactionConfig{
		MaxContextTokens:   4096,
		MaxHistoryShare:    1.0,
		ReserveTokens:      220,
		CustomInstructions: summaryCustomFocus,
	}, provider)

	summary, err := compactor.Summarize(ctx, olderMessages, "")
	if err != nil || summary == agentcore.DefaultSummaryFallback {
		return ""
	}
	summary = agentcore.NormalizeStructuredSummary(summary, "", relevantMessages)
	summary = sanitizeCompressedSummaryOutput(summary)

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
	if h.summaryCache == nil || len(messages) <= 6 {
		return
	}

	// Check if cached summary is still fresh
	if cached, ok := h.summaryCache.Get(convID); ok {
		if cs, ok := cached.(*ConversationSummary); ok && isConversationSummaryFresh(cs, len(messages)) {
			return
		}
	}

	// Copy messages for goroutine safety
	msgCopy := make([]memory.Message, len(messages))
	copy(msgCopy, messages)

	h.queueEvent(func() {
		olderMessages, recentMessages := splitConversationByRecentUserTurns(msgCopy, compressedTierRecentRounds)
		if len(olderMessages) == 0 {
			return
		}
		relevantMessages := append(cloneLLMMessages(olderMessages), cloneLLMMessages(recentMessages)...)

		mode := h.contextCompressionMode()
		cacheSummary := func(summary string) {
			h.summaryCache.Put(convID, &ConversationSummary{
				Text:         summary,
				MessageCount: len(msgCopy),
			})
		}

		if mode != "offline" {
			if summary := h.buildSmallModelConversationCompression(context.Background(), olderMessages, relevantMessages); summary != "" {
				cacheSummary(summary)
				logger.Debug().Str("conv_id", convID).Int("messages", len(msgCopy)).Msg("[context] summary refreshed async via small-model context compression")
				return
			}
		}

		if mode == "small_model" || mode == "auto" {
			if summary := h.generateConversationSummaryWithSmallModel(context.Background(), olderMessages); summary != "" {
				cacheSummary(summary)
				logger.Debug().Str("conv_id", convID).Int("messages", len(msgCopy)).Msg("[context] summary refreshed async via small model")
				return
			}
		}

		if summary := h.buildOfflineConversationCompression(olderMessages, relevantMessages); summary != "" {
			cacheSummary(summary)
			logger.Debug().Str("conv_id", convID).Int("messages", len(msgCopy)).Msg("[context] summary refreshed async via offline compression")
			return
		}

		if h.proxyBridge == nil {
			return
		}

		provider := &bridgeProvider{bridge: h.proxyBridge, model: "auto"}
		compactor := agentcore.NewCompactor(agentcore.CompactionConfig{
			MaxContextTokens:   4096,
			MaxHistoryShare:    1.0,
			ReserveTokens:      220,
			CustomInstructions: summaryCustomFocus,
		}, provider)

		summary, err := compactor.Summarize(context.Background(), olderMessages, "")
		if err != nil || summary == agentcore.DefaultSummaryFallback {
			return
		}
		summary = agentcore.NormalizeStructuredSummary(summary, "", relevantMessages)
		summary = sanitizeCompressedSummaryOutput(summary)

		h.summaryCache.Put(convID, &ConversationSummary{
			Text:         summary,
			MessageCount: len(msgCopy),
		})

		logger.Debug().Str("conv_id", convID).Int("messages", len(msgCopy)).Msg("[context] summary refreshed async")
	})
}
