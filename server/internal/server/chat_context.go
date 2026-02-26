package server

import (
	"context"
	"regexp"

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
)

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
		reEnglishRef.MatchString(msg) ||
		reContinuation.MatchString(msg)
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
		// No history context needed, but still include the current message(s)
		// so the user's message reaches the LLM.
		result.Messages = convertToLLMMessages(messages)

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
		MaxContextTokens: 4096,
		MaxHistoryShare:  1.0,
		ReserveTokens:    100,
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
			MaxContextTokens: 4096,
			MaxHistoryShare:  1.0,
			ReserveTokens:    100,
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
