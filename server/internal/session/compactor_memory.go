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
	// RefreshMemory stores already-extracted important information.
	RefreshMemory(ctx context.Context, extracted string, sessionID string) error
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
	if !c.config.Enabled || c.compactor == nil || !c.compactor.config.Enabled {
		return false
	}
	return c.shouldRefreshForRatio(session.TokenUsageRatio())
}

func (c *CompactorMemoryIntegration) shouldRefreshForRatio(ratio float64) bool {
	return ratio >= c.config.SoftThresholdRatio && ratio < c.compactor.config.Threshold
}

// ShouldRefreshMemoryOnTransition returns true only when token usage crosses
// from below soft threshold into the refresh window.
func (c *CompactorMemoryIntegration) ShouldRefreshMemoryOnTransition(previousRatio float64, session *Session) bool {
	if !c.ShouldRefreshMemory(session) {
		return false
	}
	return !c.shouldRefreshForRatio(previousRatio)
}

// ShouldCompact delegates to the underlying compactor.
func (c *CompactorMemoryIntegration) ShouldCompact(session *Session) bool {
	if c.compactor == nil {
		return false
	}
	return c.compactor.ShouldCompact(session)
}

// ShouldCompactOnTransition returns true only when token usage crosses
// from below hard threshold into compaction window.
func (c *CompactorMemoryIntegration) ShouldCompactOnTransition(previousRatio float64, session *Session) bool {
	if !c.ShouldCompact(session) {
		return false
	}
	return previousRatio < c.compactor.config.Threshold
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

	// Save extracted memory
	if err := c.memoryRefresher.RefreshMemory(ctx, extracted, session.ID.String()); err != nil {
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
	if c.compactor == nil {
		return nil, fmt.Errorf("compactor not configured")
	}

	// First, refresh memory if enabled
	if c.config.Enabled {
		if err := c.RefreshMemoryBeforeCompaction(ctx, session); err != nil {
			log.Printf("[WARN] memory refresh failed, continuing with compaction: %v", err)
		}
	}

	// Then perform regular compaction
	return c.compactor.Compact(session)
}
