package claudecode

import (
	"context"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

const (
	// BaseChunkRatio is the default ratio of context to use per chunk.
	BaseChunkRatio = 0.4
	// MinChunkRatio is the minimum ratio to prevent chunks from being too small.
	MinChunkRatio = 0.15
	// SafetyMargin accounts for token estimation inaccuracy.
	SafetyMargin = 1.2
	// DefaultContextTokens is the default context window size.
	DefaultContextTokens = 200000
	// DefaultSummaryFallback is returned when summarization fails.
	DefaultSummaryFallback = "No prior history."
	// DefaultParts is the default number of parts to split messages into.
	DefaultParts = 2
)

// CompactionConfig holds configuration for context compaction.
type CompactionConfig struct {
	// MaxContextTokens is the maximum context window size.
	MaxContextTokens int
	// MaxHistoryShare is the maximum share of context for history (0.0-1.0).
	MaxHistoryShare float64
	// ReserveTokens is the number of tokens to reserve for the response.
	ReserveTokens int
	// CustomInstructions are additional instructions for summarization.
	CustomInstructions string
}

// DefaultCompactionConfig returns the default compaction configuration.
func DefaultCompactionConfig() CompactionConfig {
	return CompactionConfig{
		MaxContextTokens: DefaultContextTokens,
		MaxHistoryShare:  0.5,
		ReserveTokens:    20000, // system prompt + memory + tools + response headroom
	}
}

// Compactor handles context compaction and summarization.
type Compactor struct {
	config   CompactionConfig
	provider llm.Provider
}

// NewCompactor creates a new Compactor.
func NewCompactor(config CompactionConfig, provider llm.Provider) *Compactor {
	return &Compactor{
		config:   config,
		provider: provider,
	}
}

// countTokensInString estimates tokens using a hybrid heuristic:
//   - Short ASCII words (≤10 bytes): 1 token each
//   - Long ASCII words (>10 bytes, e.g. URLs, JSON): ceil(len/4)
//   - Non-ASCII runes (CJK, emoji): ~1 token each
//   - ASCII bytes adjacent to non-ASCII: ceil(len/4)
//
// Optimized: starts in fast ASCII-only mode, falls back to mixed-mode
// mid-word only when a high-bit byte is encountered.
func countTokensInString(s string) int {
	n := len(s)
	if n == 0 {
		return 0
	}
	count := 0
	i := 0
	for i < n {
		c := s[i]
		// skip whitespace
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			i++
			continue
		}
		// start of a word — try ASCII fast path
		start := i
		for i < n {
			c = s[i]
			if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
				break
			}
			if c&0x80 != 0 {
				// hit non-ASCII mid-word: switch to mixed counting for this word
				asciiBytes := i - start
				nonASCIIRunes := 0
				if asciiBytes > 0 {
					count += (asciiBytes + 3) / 4
					asciiBytes = 0
				}
				for i < n {
					c = s[i]
					if c <= ' ' && (c == ' ' || c == '\t' || c == '\n' || c == '\r') {
						break
					}
					if c&0x80 == 0 {
						asciiBytes++
						i++
					} else {
						if asciiBytes > 0 && nonASCIIRunes > 0 {
							count += (asciiBytes + 3) / 4
							asciiBytes = 0
						}
						i++
						for i < n && s[i]&0xC0 == 0x80 {
							i++
						}
						nonASCIIRunes++
					}
				}
				count += nonASCIIRunes
				if asciiBytes > 0 {
					count += (asciiBytes + 3) / 4
				}
				goto nextWord
			}
			i++
		}
		// pure ASCII word
		{
			wordLen := i - start
			if wordLen > 10 {
				count += (wordLen + 3) / 4
			} else {
				count++
			}
		}
	nextWord:
	}
	return count
}

// EstimateTokens estimates the number of tokens in a message.
// Uses a hybrid whitespace-split + byte-length heuristic for better accuracy
// across English, CJK, and mixed content.
// It accounts for Content, ContentParts (multimodal), and ToolCalls.
func EstimateTokens(msg llm.Message) int {
	total := 0

	// Primary content field
	if msg.Content != "" {
		total += countTokensInString(msg.Content)
	}

	// ContentParts (used for multimodal messages where Content is cleared)
	for _, part := range msg.ContentParts {
		switch part.Type {
		case "text":
			total += countTokensInString(part.Text)
		case "image":
			// Images consume ~85 tokens for low-res, ~765 for high-res.
			// Use a conservative estimate since we can't know the resolution.
			total += 765
		}
	}

	// ToolCalls (assistant requesting tool use)
	for _, tc := range msg.ToolCalls {
		total += countTokensInString(tc.Name) + countTokensInString(tc.Arguments) + 10 // +10 for structural overhead
	}

	// Minimum 4 tokens per message for role/structural overhead
	if total == 0 && (msg.Role != "" || msg.ToolCallID != "") {
		total = 4
	}

	return total
}

// EstimateMessagesTokens estimates the total tokens in a slice of messages.
func EstimateMessagesTokens(messages []llm.Message) int {
	total := 0
	for _, msg := range messages {
		total += EstimateTokens(msg)
	}
	return total
}

// SplitMessagesByTokenShare splits messages into parts based on token share.
func SplitMessagesByTokenShare(messages []llm.Message, parts int) [][]llm.Message {
	if len(messages) == 0 {
		return nil
	}

	normalizedParts := normalizeParts(parts, len(messages))
	if normalizedParts <= 1 {
		return [][]llm.Message{messages}
	}

	totalTokens := EstimateMessagesTokens(messages)
	targetTokens := totalTokens / normalizedParts
	chunks := make([][]llm.Message, 0, normalizedParts)
	current := make([]llm.Message, 0)
	currentTokens := 0

	for _, msg := range messages {
		msgTokens := EstimateTokens(msg)
		if len(chunks) < normalizedParts-1 &&
			len(current) > 0 &&
			currentTokens+msgTokens > targetTokens {
			chunks = append(chunks, current)
			current = make([]llm.Message, 0)
			currentTokens = 0
		}

		current = append(current, msg)
		currentTokens += msgTokens
	}

	if len(current) > 0 {
		chunks = append(chunks, current)
	}

	return chunks
}

// ChunkMessagesByMaxTokens splits messages into chunks with a maximum token count.
func ChunkMessagesByMaxTokens(messages []llm.Message, maxTokens int) [][]llm.Message {
	if len(messages) == 0 {
		return nil
	}

	chunks := make([][]llm.Message, 0)
	currentChunk := make([]llm.Message, 0)
	currentTokens := 0

	for _, msg := range messages {
		msgTokens := EstimateTokens(msg)
		if len(currentChunk) > 0 && currentTokens+msgTokens > maxTokens {
			chunks = append(chunks, currentChunk)
			currentChunk = make([]llm.Message, 0)
			currentTokens = 0
		}

		currentChunk = append(currentChunk, msg)
		currentTokens += msgTokens

		// Split oversized messages to avoid unbounded chunk growth
		if msgTokens > maxTokens {
			chunks = append(chunks, currentChunk)
			currentChunk = make([]llm.Message, 0)
			currentTokens = 0
		}
	}

	if len(currentChunk) > 0 {
		chunks = append(chunks, currentChunk)
	}

	return chunks
}

// ComputeAdaptiveChunkRatio computes an adaptive chunk ratio based on message size.
func ComputeAdaptiveChunkRatio(messages []llm.Message, contextWindow int) float64 {
	if len(messages) == 0 {
		return BaseChunkRatio
	}

	totalTokens := EstimateMessagesTokens(messages)
	avgTokens := float64(totalTokens) / float64(len(messages))

	// Apply safety margin
	safeAvgTokens := avgTokens * SafetyMargin
	avgRatio := safeAvgTokens / float64(contextWindow)

	// If average message is > 10% of context, reduce chunk ratio
	if avgRatio > 0.1 {
		reduction := min(avgRatio*2, BaseChunkRatio-MinChunkRatio)
		return max(MinChunkRatio, BaseChunkRatio-reduction)
	}

	return BaseChunkRatio
}

// IsOversizedForSummary checks if a message is too large to summarize.
func IsOversizedForSummary(msg llm.Message, contextWindow int) bool {
	tokens := float64(EstimateTokens(msg)) * SafetyMargin
	return tokens > float64(contextWindow)*0.5
}

// PruneResult contains the result of pruning history.
type PruneResult struct {
	Messages        []llm.Message
	DroppedMessages []llm.Message
	DroppedChunks   int
	DroppedCount    int
	DroppedTokens   int
	KeptTokens      int
	BudgetTokens    int
}

// PruneHistoryForContextShare prunes history to fit within context budget.
// It ensures tool call/result pairs are kept together — orphaned tool_result
// messages (whose tool_use_id has no matching assistant tool_call) are removed.
func PruneHistoryForContextShare(messages []llm.Message, maxContextTokens int, maxHistoryShare float64, parts int) PruneResult {
	if maxHistoryShare <= 0 {
		maxHistoryShare = 0.5
	}
	budgetTokens := int(float64(maxContextTokens) * maxHistoryShare)
	if budgetTokens < 1 {
		budgetTokens = 1
	}

	keptMessages := messages
	allDroppedMessages := make([]llm.Message, 0)
	droppedChunks := 0
	droppedCount := 0
	droppedTokens := 0

	normalizedParts := normalizeParts(parts, len(keptMessages))

	for len(keptMessages) > 0 && EstimateMessagesTokens(keptMessages) > budgetTokens {
		chunks := SplitMessagesByTokenShare(keptMessages, normalizedParts)
		if len(chunks) <= 1 {
			// Single chunk still over budget — drop oldest messages one by one
			for len(keptMessages) > 1 && EstimateMessagesTokens(keptMessages) > budgetTokens {
				dropped := keptMessages[0]
				keptMessages = keptMessages[1:]
				droppedChunks++
				droppedCount++
				droppedTokens += EstimateTokens(dropped)
				allDroppedMessages = append(allDroppedMessages, dropped)
			}
			break
		}

		// Drop the first (oldest) chunk
		dropped := chunks[0]
		droppedChunks++
		droppedCount += len(dropped)
		droppedTokens += EstimateMessagesTokens(dropped)
		allDroppedMessages = append(allDroppedMessages, dropped...)

		// Keep the rest
		keptMessages = make([]llm.Message, 0)
		for i := 1; i < len(chunks); i++ {
			keptMessages = append(keptMessages, chunks[i]...)
		}
	}

	// Sanitize: remove orphaned tool results whose tool_use has been pruned.
	keptMessages, orphaned := sanitizeToolPairs(keptMessages)
	if len(orphaned) > 0 {
		allDroppedMessages = append(allDroppedMessages, orphaned...)
		droppedCount += len(orphaned)
		droppedTokens += EstimateMessagesTokens(orphaned)
	}

	return PruneResult{
		Messages:        keptMessages,
		DroppedMessages: allDroppedMessages,
		DroppedChunks:   droppedChunks,
		DroppedCount:    droppedCount,
		DroppedTokens:   droppedTokens,
		KeptTokens:      EstimateMessagesTokens(keptMessages),
		BudgetTokens:    budgetTokens,
	}
}

// sanitizeToolPairs removes orphaned tool result messages whose ToolCallID
// doesn't match any ToolCall.ID in the kept assistant messages. It also
// removes assistant messages with ToolCalls if none of their tool results
// are present (to avoid the API complaining about missing tool results).
func sanitizeToolPairs(messages []llm.Message) (kept []llm.Message, dropped []llm.Message) {
	// Build set of all tool_call IDs from assistant messages.
	toolCallIDs := make(map[string]struct{})
	for _, msg := range messages {
		if msg.Role == llm.RoleAssistant {
			for _, tc := range msg.ToolCalls {
				toolCallIDs[tc.ID] = struct{}{}
			}
		}
	}

	// Build set of all tool_result IDs from tool messages.
	toolResultIDs := make(map[string]struct{})
	for _, msg := range messages {
		if msg.Role == llm.RoleTool && msg.ToolCallID != "" {
			toolResultIDs[msg.ToolCallID] = struct{}{}
		}
	}

	kept = make([]llm.Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == llm.RoleTool && msg.ToolCallID != "" {
			// Drop tool result if its tool_use was pruned.
			if _, ok := toolCallIDs[msg.ToolCallID]; !ok {
				dropped = append(dropped, msg)
				continue
			}
		}
		if msg.Role == llm.RoleAssistant && len(msg.ToolCalls) > 0 {
			// Drop assistant tool_use if ALL of its results were pruned.
			hasAnyResult := false
			for _, tc := range msg.ToolCalls {
				if _, ok := toolResultIDs[tc.ID]; ok {
					hasAnyResult = true
					break
				}
			}
			if !hasAnyResult {
				dropped = append(dropped, msg)
				continue
			}
		}
		kept = append(kept, msg)
	}
	return kept, dropped
}

// Summarize generates a summary of the given messages.
func (c *Compactor) Summarize(ctx context.Context, messages []llm.Message, previousSummary string) (string, error) {
	if len(messages) == 0 {
		if previousSummary != "" {
			return previousSummary, nil
		}
		return DefaultSummaryFallback, nil
	}

	// Build summarization prompt
	var sb strings.Builder
	sb.WriteString("Please summarize the following conversation, preserving key decisions, TODOs, open questions, and constraints:\n\n")

	if previousSummary != "" {
		sb.WriteString("Previous context summary:\n")
		sb.WriteString(previousSummary)
		sb.WriteString("\n\n")
	}

	sb.WriteString("Conversation to summarize:\n")
	for _, msg := range messages {
		sb.WriteString(string(msg.Role))
		sb.WriteString(": ")
		sb.WriteString(msg.Content)
		sb.WriteString("\n\n")
	}

	if c.config.CustomInstructions != "" {
		sb.WriteString("\nAdditional focus:\n")
		sb.WriteString(c.config.CustomInstructions)
	}

	// Call LLM for summarization
	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: sb.String()},
		},
		MaxTokens: c.config.ReserveTokens,
	}

	resp, err := c.provider.Chat(ctx, req)
	if err != nil {
		return DefaultSummaryFallback, err
	}

	return resp.Message.Content, nil
}

// CompactMessages compacts messages to fit within context limits.
func (c *Compactor) CompactMessages(ctx context.Context, messages []llm.Message) ([]llm.Message, string, error) {
	totalTokens := EstimateMessagesTokens(messages)
	// Reserve space for system prompt, memory context, tools, and response.
	// These are injected AFTER compaction, so we must account for them here.
	reserveTokens := c.config.ReserveTokens
	if reserveTokens < 4096 {
		reserveTokens = 4096
	}
	budgetTokens := int(float64(c.config.MaxContextTokens)*c.config.MaxHistoryShare) - reserveTokens

	// If within budget, no compaction needed
	if totalTokens <= budgetTokens {
		return messages, "", nil
	}

	// Prune history
	result := PruneHistoryForContextShare(
		messages,
		c.config.MaxContextTokens,
		c.config.MaxHistoryShare,
		DefaultParts,
	)

	// If we dropped messages, generate a summary
	var summary string
	if len(result.DroppedMessages) > 0 {
		var err error
		summary, err = c.Summarize(ctx, result.DroppedMessages, "")
		if err != nil {
			// Log error but continue with pruned messages
			summary = DefaultSummaryFallback
		}
	}

	return result.Messages, summary, nil
}

// normalizeParts ensures parts is within valid range.
func normalizeParts(parts, messageCount int) int {
	if parts <= 1 {
		return 1
	}
	if parts > messageCount {
		return messageCount
	}
	return parts
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
