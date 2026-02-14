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
		ReserveTokens:    4096,
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

// EstimateTokens estimates the number of tokens in a message.
// This is a rough estimate based on character count.
func EstimateTokens(msg llm.Message) int {
	// Rough estimate: ~4 characters per token for English text
	content := msg.Content
	if content == "" {
		return 0
	}
	return len(content) / 4
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
	budgetTokens := int(float64(c.config.MaxContextTokens) * c.config.MaxHistoryShare)

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
