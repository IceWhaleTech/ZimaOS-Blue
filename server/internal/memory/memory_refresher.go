package memory

import (
	"context"
	"fmt"
	"log"
	"strings"

	sessionctx "github.com/IceWhaleTech/ZimaOS-Echo/server/internal/context"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/llm"
)

// LayeredMemoryRefresher implements session.MemoryRefresher using LayeredMemoryService.
type LayeredMemoryRefresher struct {
	layeredMemory *LayeredMemoryService
	llmProvider   llm.Provider
	systemPrompt  string
	maxTokens     int
}

// NewLayeredMemoryRefresher creates a new memory refresher.
func NewLayeredMemoryRefresher(
	layeredMemory *LayeredMemoryService,
	llmProvider llm.Provider,
) *LayeredMemoryRefresher {
	return &LayeredMemoryRefresher{
		layeredMemory: layeredMemory,
		llmProvider:   llmProvider,
		maxTokens:     500,
		systemPrompt: `You are a memory extraction assistant. Analyze the conversation and extract important information that should be remembered.

Focus on:
1. User preferences and settings
2. Important decisions made
3. Key facts about the user or their projects
4. Recurring topics or interests
5. Action items or commitments

Format your response as a concise summary with bullet points for discrete facts.
Only include information worth remembering. Skip trivial or temporary information.
If nothing important to remember, respond with "NO_MEMORY_NEEDED".`,
	}
}

// RefreshMemory extracts important information and saves to daily log.
func (r *LayeredMemoryRefresher) RefreshMemory(ctx context.Context, messages []sessionctx.Message, sessionID string) error {
	if r.layeredMemory == nil {
		return nil
	}

	// Extract important information
	extracted, err := r.extractInfo(ctx, messages)
	if err != nil {
		return fmt.Errorf("failed to extract info: %w", err)
	}

	if extracted == "" || extracted == "NO_MEMORY_NEEDED" {
		log.Printf("[DEBUG] no important info to save for session %s", sessionID)
		return nil
	}

	// Build memory content
	content := fmt.Sprintf("**Auto-extracted from session:** %s\n\n%s", sessionID, extracted)
	tags := []string{"auto-refresh", "session:" + sessionID}

	// Save to daily log
	if err := r.layeredMemory.AppendToDaily(ctx, content, tags); err != nil {
		return fmt.Errorf("failed to append to daily: %w", err)
	}

	log.Printf("[INFO] saved auto-extracted memory for session %s", sessionID)
	return nil
}

// extractInfo uses LLM to extract important information.
func (r *LayeredMemoryRefresher) extractInfo(ctx context.Context, messages []sessionctx.Message) (string, error) {
	if r.llmProvider == nil {
		// Fallback: create simple summary without LLM
		return r.simpleSummary(messages), nil
	}

	// Build conversation text
	var sb strings.Builder
	for _, msg := range messages {
		if msg.Role == sessionctx.RoleSystem {
			continue
		}
		sb.WriteString(fmt.Sprintf("%s: %s\n\n", msg.Role, msg.Content))
	}

	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: r.systemPrompt},
			{Role: "user", Content: fmt.Sprintf("Extract important information:\n\n%s", sb.String())},
		},
		MaxTokens:   r.maxTokens,
		Temperature: 0.3,
	}

	resp, err := r.llmProvider.Chat(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Message.Content, nil
}

// simpleSummary creates a basic summary without LLM.
func (r *LayeredMemoryRefresher) simpleSummary(messages []sessionctx.Message) string {
	if len(messages) == 0 {
		return "NO_MEMORY_NEEDED"
	}

	var userMessages []string
	for _, msg := range messages {
		if msg.Role == "user" {
			preview := msg.Content
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			userMessages = append(userMessages, preview)
		}
	}

	if len(userMessages) == 0 {
		return "NO_MEMORY_NEEDED"
	}

	var sb strings.Builder
	sb.WriteString("**Topics discussed:**\n")
	for i, topic := range userMessages {
		if i >= 3 {
			break
		}
		sb.WriteString(fmt.Sprintf("- %s\n", topic))
	}

	return sb.String()
}
