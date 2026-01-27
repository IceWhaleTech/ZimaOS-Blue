package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/config"
	sessionctx "github.com/IceWhaleTech/ZimaOS-Echo/server/internal/context"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/llm"
)

// SessionCompactor handles session compaction with summarization.
type SessionCompactor struct {
	llmProvider llm.Provider
	config      config.SessionCompactionConfig
}

// CompactionResult holds the result of compaction.
type CompactionResult struct {
	Summary           string        `json:"summary"`
	PreservedMessages int           `json:"preserved_messages"`
	RemovedMessages   int           `json:"removed_messages"`
	TokensBefore      int           `json:"tokens_before"`
	TokensAfter       int           `json:"tokens_after"`
	TokensSaved       int           `json:"tokens_saved"`
	Duration          time.Duration `json:"duration"`
}

// NewSessionCompactor creates a new SessionCompactor.
func NewSessionCompactor(provider llm.Provider, cfg config.SessionCompactionConfig) *SessionCompactor {
	return &SessionCompactor{
		llmProvider: provider,
		config:      cfg,
	}
}

// ShouldCompact returns true if the session should be compacted.
func (c *SessionCompactor) ShouldCompact(session *Session) bool {
	if !c.config.Enabled {
		return false
	}

	// Check token usage ratio
	ratio := session.TokenUsageRatio()
	return ratio >= c.config.Threshold
}

// Compact compacts a session by summarizing old messages.
func (c *SessionCompactor) Compact(session *Session) (*CompactionResult, error) {
	start := time.Now()

	session.SetState(SessionStateCompacting)
	defer session.SetState(SessionStateActive)

	messages := session.GetMessages()
	tokensBefore := session.TokenCount()

	// Find messages to preserve (recent ones)
	preserveCount := c.config.PreserveRecent
	if preserveCount > len(messages) {
		preserveCount = len(messages)
	}

	// Separate system prompt, messages to summarize, and messages to preserve
	var systemPrompt *sessionctx.Message
	var toSummarize []sessionctx.Message
	var toPreserve []sessionctx.Message

	for i, msg := range messages {
		if msg.Role == sessionctx.RoleSystem {
			systemPrompt = &messages[i]
			continue
		}
		if i >= len(messages)-preserveCount {
			toPreserve = append(toPreserve, msg)
		} else {
			toSummarize = append(toSummarize, msg)
		}
	}

	// If nothing to summarize, return early
	if len(toSummarize) == 0 {
		return &CompactionResult{
			PreservedMessages: len(toPreserve),
			RemovedMessages:   0,
			TokensBefore:      tokensBefore,
			TokensAfter:       tokensBefore,
			TokensSaved:       0,
			Duration:          time.Since(start),
		}, nil
	}

	// Generate summary based on strategy
	var summary string
	var err error

	switch c.config.Strategy {
	case "summarize":
		summary, err = c.generateSummary(toSummarize)
		if err != nil {
			return nil, fmt.Errorf("failed to generate summary: %w", err)
		}
	case "truncate":
		// Just truncate, no summary
		summary = ""
	case "hybrid":
		// Summarize if LLM available, otherwise truncate
		if c.llmProvider != nil {
			summary, err = c.generateSummary(toSummarize)
			if err != nil {
				// Fall back to truncate
				summary = ""
			}
		}
	default:
		summary = ""
	}

	// Clear session and rebuild
	session.Clear()

	// Re-add system prompt
	if systemPrompt != nil {
		session.SetSystemPrompt(systemPrompt.Content)
	}

	// Add summary as a system message if available
	if summary != "" {
		session.SetSummary(summary)
		// Add summary context to the conversation
		session.AddMessage(sessionctx.Message{
			Role:    sessionctx.RoleSystem,
			Content: fmt.Sprintf("[Previous conversation summary: %s]", summary),
		})
	}

	// Re-add preserved messages
	for _, msg := range toPreserve {
		session.AddMessage(msg)
	}

	tokensAfter := session.TokenCount()

	return &CompactionResult{
		Summary:           summary,
		PreservedMessages: len(toPreserve),
		RemovedMessages:   len(toSummarize),
		TokensBefore:      tokensBefore,
		TokensAfter:       tokensAfter,
		TokensSaved:       tokensBefore - tokensAfter,
		Duration:          time.Since(start),
	}, nil
}

// generateSummary generates a summary of messages using LLM.
func (c *SessionCompactor) generateSummary(messages []sessionctx.Message) (string, error) {
	if c.llmProvider == nil {
		return "", fmt.Errorf("LLM provider not configured")
	}

	// Build conversation text
	var sb strings.Builder
	for _, msg := range messages {
		sb.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
	}

	// Create summarization request
	req := llm.ChatRequest{
		Messages: []llm.Message{
			{
				Role: "system",
				Content: `You are a conversation summarizer. Summarize the following conversation concisely,
capturing the key points, decisions, and context that would be important for continuing the conversation.
Keep the summary under 500 tokens. Focus on:
1. Main topics discussed
2. Key decisions or conclusions
3. Important context or preferences mentioned
4. Any pending questions or tasks`,
			},
			{
				Role:    "user",
				Content: fmt.Sprintf("Please summarize this conversation:\n\n%s", sb.String()),
			},
		},
		MaxTokens:   c.config.SummaryMaxTokens,
		Temperature: 0.3, // Lower temperature for more consistent summaries
	}

	ctx := context.Background()
	resp, err := c.llmProvider.Chat(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Message.Content, nil
}

// CompactWithCustomPrompt compacts with a custom summarization prompt.
func (c *SessionCompactor) CompactWithCustomPrompt(session *Session, prompt string) (*CompactionResult, error) {
	// Store original config
	originalStrategy := c.config.Strategy
	c.config.Strategy = "summarize"
	defer func() { c.config.Strategy = originalStrategy }()

	return c.Compact(session)
}
