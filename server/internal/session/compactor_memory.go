package session

import (
	"context"
	"fmt"
	"log"
	"strings"

	sessionctx "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/context"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

// MemoryRefresher interface for memory integration during compaction.
type MemoryRefresher interface {
	// RefreshMemory extracts important information from messages and saves to memory.
	RefreshMemory(ctx context.Context, messages []sessionctx.Message, sessionID string) error
}

// MemoryRefreshConfig holds configuration for memory refresh before compaction.
type MemoryRefreshConfig struct {
	// Enabled enables memory refresh before compaction.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// SoftThresholdRatio triggers memory refresh (e.g., 0.6 = 60% of max tokens).
	SoftThresholdRatio float64 `json:"soft_threshold_ratio" yaml:"soft_threshold_ratio"`
	// SystemPrompt for extracting important information.
	SystemPrompt string `json:"system_prompt" yaml:"system_prompt"`
	// MaxExtractTokens limits the extraction response.
	MaxExtractTokens int `json:"max_extract_tokens" yaml:"max_extract_tokens"`
}

// DefaultMemoryRefreshConfig returns default configuration.
func DefaultMemoryRefreshConfig() MemoryRefreshConfig {
	return MemoryRefreshConfig{
		Enabled:            true,
		SoftThresholdRatio: 0.6,
		MaxExtractTokens:   500,
		SystemPrompt: `You are a memory extraction assistant. Analyze the conversation and extract important information that should be remembered long-term.

Focus on:
1. User preferences and settings mentioned
2. Important decisions made
3. Key facts about the user or their projects
4. Recurring topics or interests
5. Action items or commitments
6. Explicit capability/tool expectations the user wants remembered (e.g. session query capability)

Format your response as a bullet list of discrete facts/preferences to remember.
Only include information worth remembering long-term. Skip trivial or temporary information.
If nothing important to remember, respond with "NO_MEMORY_NEEDED".`,
	}
}

// CompactorMemoryIntegration adds memory refresh capability to SessionCompactor.
type CompactorMemoryIntegration struct {
	compactor       *SessionCompactor
	memoryRefresher MemoryRefresher
	llmProvider     llm.Provider
	config          MemoryRefreshConfig
}

// NewCompactorMemoryIntegration creates a new compactor with memory integration.
func NewCompactorMemoryIntegration(
	compactor *SessionCompactor,
	memoryRefresher MemoryRefresher,
	llmProvider llm.Provider,
	config MemoryRefreshConfig,
) *CompactorMemoryIntegration {
	return &CompactorMemoryIntegration{
		compactor:       compactor,
		memoryRefresher: memoryRefresher,
		llmProvider:     llmProvider,
		config:          config,
	}
}

// ShouldRefreshMemory checks if memory refresh should be triggered (soft threshold).
func (c *CompactorMemoryIntegration) ShouldRefreshMemory(session *Session) bool {
	if !c.config.Enabled {
		return false
	}
	ratio := session.TokenUsageRatio()
	return ratio >= c.config.SoftThresholdRatio && ratio < c.compactor.config.Threshold
}

// ShouldCompact delegates to the underlying compactor.
func (c *CompactorMemoryIntegration) ShouldCompact(session *Session) bool {
	return c.compactor.ShouldCompact(session)
}

// RefreshMemoryBeforeCompaction extracts and saves important information before compaction.
func (c *CompactorMemoryIntegration) RefreshMemoryBeforeCompaction(ctx context.Context, session *Session) error {
	if !c.config.Enabled || c.memoryRefresher == nil {
		return nil
	}

	messages := session.GetMessages()
	if len(messages) < 3 {
		return nil // Not enough messages to extract from
	}

	// Extract important information using LLM
	extracted, err := c.extractImportantInfo(ctx, messages)
	if err != nil {
		log.Printf("[WARN] failed to extract important info: %v", err)
		return nil // Don't fail compaction if extraction fails
	}

	if extracted == "" || extracted == "NO_MEMORY_NEEDED" {
		log.Printf("[DEBUG] no important information to save from session %s", session.ID.String())
		return nil
	}

	// Save to memory
	if err := c.memoryRefresher.RefreshMemory(ctx, messages, session.ID.String()); err != nil {
		log.Printf("[WARN] failed to refresh memory: %v", err)
		return nil
	}

	log.Printf("[INFO] refreshed memory before compaction for session %s", session.ID.String())
	return nil
}

// extractImportantInfo uses LLM to extract important information from messages.
func (c *CompactorMemoryIntegration) extractImportantInfo(ctx context.Context, messages []sessionctx.Message) (string, error) {
	if c.llmProvider == nil {
		return "", fmt.Errorf("LLM provider not configured")
	}

	// Build conversation text
	var sb strings.Builder
	for _, msg := range messages {
		if msg.Role == sessionctx.RoleSystem {
			continue // Skip system messages
		}
		sb.WriteString(fmt.Sprintf("%s: %s\n\n", msg.Role, msg.Content))
	}

	req := llm.ChatRequest{
		Messages: []llm.Message{
			{
				Role:    "system",
				Content: c.config.SystemPrompt,
			},
			{
				Role:    "user",
				Content: fmt.Sprintf("Extract important information from this conversation:\n\n%s", sb.String()),
			},
		},
		MaxTokens:   c.config.MaxExtractTokens,
		Temperature: 0.3,
	}

	resp, err := c.llmProvider.Chat(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Message.Content, nil
}

// CompactWithMemoryRefresh performs compaction with memory refresh.
func (c *CompactorMemoryIntegration) CompactWithMemoryRefresh(ctx context.Context, session *Session) (*CompactionResult, error) {
	// First, refresh memory if enabled
	if c.config.Enabled {
		if err := c.RefreshMemoryBeforeCompaction(ctx, session); err != nil {
			log.Printf("[WARN] memory refresh failed, continuing with compaction: %v", err)
		}
	}

	// Then perform regular compaction
	return c.compactor.Compact(session)
}
