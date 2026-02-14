package memory

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/session"
)

// SessionMemoryHook saves session summaries to daily memory when sessions end.
type SessionMemoryHook struct {
	layeredMemory *LayeredMemoryService
	minMessages   int // Minimum messages to trigger save (default: 3)
}

// NewSessionMemoryHook creates a new session memory hook.
func NewSessionMemoryHook(layeredMemory *LayeredMemoryService) *SessionMemoryHook {
	return &SessionMemoryHook{
		layeredMemory: layeredMemory,
		minMessages:   3,
	}
}

// SetMinMessages sets the minimum message count to trigger memory save.
func (h *SessionMemoryHook) SetMinMessages(min int) {
	h.minMessages = min
}

// OnSessionEnd implements SessionHook interface.
func (h *SessionMemoryHook) OnSessionEnd(ctx context.Context, sess *session.Session, reason session.EndReason) error {
	if h.layeredMemory == nil {
		return nil
	}

	// Skip if session has too few messages
	if sess.Metadata.MessageCount < h.minMessages {
		log.Printf("[DEBUG] session %s has only %d messages, skipping memory save", sess.ID.String(), sess.Metadata.MessageCount)
		return nil
	}

	// Build memory content from session
	content := h.buildMemoryContent(sess, reason)
	if content == "" {
		return nil
	}

	// Build tags
	tags := h.buildTags(sess, reason)

	// Save to daily log
	if err := h.layeredMemory.AppendToDaily(ctx, content, tags); err != nil {
		return fmt.Errorf("failed to save session to daily memory: %w", err)
	}

	log.Printf("[INFO] saved session %s to daily memory (reason: %s, messages: %d)",
		sess.ID.String(), reason, sess.Metadata.MessageCount)

	return nil
}

// buildMemoryContent builds the memory content from session data.
func (h *SessionMemoryHook) buildMemoryContent(sess *session.Session, reason session.EndReason) string {
	var sb strings.Builder

	// Session identifier
	sb.WriteString(fmt.Sprintf("**Session:** %s\n", sess.ID.String()))
	sb.WriteString(fmt.Sprintf("**End Reason:** %s\n", reason))
	sb.WriteString(fmt.Sprintf("**Messages:** %d\n", sess.Metadata.MessageCount))
	sb.WriteString(fmt.Sprintf("**Duration:** %s\n\n", sess.LastActiveAt.Sub(sess.CreatedAt).Round(1e9).String()))

	// Include title if available
	if sess.Metadata.Title != "" {
		sb.WriteString(fmt.Sprintf("**Topic:** %s\n\n", sess.Metadata.Title))
	}

	// Include summary if available (from compaction)
	if sess.Metadata.Summary != "" {
		sb.WriteString("### Summary\n\n")
		sb.WriteString(sess.Metadata.Summary)
		sb.WriteString("\n")
	} else {
		// Generate a basic summary from messages
		messages := sess.GetMessages()
		if len(messages) > 0 {
			sb.WriteString("### Conversation Highlights\n\n")

			// Get first user message as topic indicator
			for _, msg := range messages {
				if msg.Role == "user" {
					preview := truncateString(msg.Content, 200)
					sb.WriteString(fmt.Sprintf("- **User asked:** %s\n", preview))
					break
				}
			}

			// Get last assistant response
			for i := len(messages) - 1; i >= 0; i-- {
				if messages[i].Role == "assistant" {
					preview := truncateString(messages[i].Content, 200)
					sb.WriteString(fmt.Sprintf("- **Final response:** %s\n", preview))
					break
				}
			}
		}
	}

	return sb.String()
}

// buildTags builds tags for the memory entry.
func (h *SessionMemoryHook) buildTags(sess *session.Session, reason session.EndReason) []string {
	tags := []string{"session", string(reason)}

	// Add session tags
	tags = append(tags, sess.GetTags()...)

	// Add channel info
	if sess.ID.ChannelID != "" {
		tags = append(tags, "channel:"+sess.ID.ChannelID)
	}

	// Add agent info
	if sess.ID.AgentID != "" {
		tags = append(tags, "agent:"+sess.ID.AgentID)
	}

	return tags
}

// truncateString truncates a string to maxLen and adds ellipsis if needed.
func truncateString(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
