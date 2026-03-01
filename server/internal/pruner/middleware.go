package pruner

import (
	"context"
	"encoding/json"
	"log/slog"
)

// contextKey is a private type for context keys in this package.
type contextKey struct{}

// pruneStatsKey is the context key for per-request pruning stats.
var pruneStatsKey = contextKey{}

// prunerDisabledKey marks requests that should bypass pruning middleware.
var prunerDisabledKey = contextKey{}

// RequestPruneStats holds per-request pruning statistics.
type RequestPruneStats struct {
	Pruned         bool `json:"pruned"`
	MessagesPruned int  `json:"messages_pruned"`
	TokensBefore   int  `json:"tokens_before"`
	TokensAfter    int  `json:"tokens_after"`
}

// WithPruneStats returns a new context with pruning stats attached.
func WithPruneStats(ctx context.Context, stats *RequestPruneStats) context.Context {
	return context.WithValue(ctx, pruneStatsKey, stats)
}

// GetPruneStats retrieves per-request pruning stats from context, or nil.
func GetPruneStats(ctx context.Context) *RequestPruneStats {
	if v, ok := ctx.Value(pruneStatsKey).(*RequestPruneStats); ok {
		return v
	}
	return nil
}

// WithPrunerDisabled marks whether pruning should be bypassed for this request.
func WithPrunerDisabled(ctx context.Context, disabled bool) context.Context {
	return context.WithValue(ctx, prunerDisabledKey, disabled)
}

// IsPrunerDisabled checks whether pruning is disabled for this request context.
func IsPrunerDisabled(ctx context.Context) bool {
	v, ok := ctx.Value(prunerDisabledKey).(bool)
	return ok && v
}

// openaiMessage represents a message in the OpenAI chat completions format.
type openaiMessage struct {
	Role       string `json:"role"`
	Content    string `json:"content,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	Name       string `json:"name,omitempty"`
}

// openaiRequest is a minimal representation of the chat completions request.
type openaiRequest struct {
	Messages []openaiMessage        `json:"messages"`
	Extra    map[string]interface{} `json:"-"`
}

// Middleware intercepts proxy requests and prunes code content in tool messages.
type Middleware struct {
	backend Backend
	config  Config
	stats   *Stats
}

// NewMiddleware creates a new pruner middleware.
func NewMiddleware(backend Backend, cfg Config, stats *Stats) *Middleware {
	return &Middleware{
		backend: backend,
		config:  cfg,
		stats:   stats,
	}
}

// ProcessRequest scans the request body for tool messages containing code,
// prunes them, and returns the modified body. Returns the original body
// unchanged if no pruning was applied or on any error.
// If a *RequestPruneStats is attached to ctx via WithPruneStats, it will be populated.
func (m *Middleware) ProcessRequest(ctx context.Context, body []byte) ([]byte, error) {
	if !m.config.Enabled || m.backend == nil {
		return body, nil
	}

	// Parse into a generic map to preserve all fields
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return body, nil
	}

	messagesRaw, ok := raw["messages"]
	if !ok {
		return body, nil
	}

	var messages []openaiMessage
	if err := json.Unmarshal(messagesRaw, &messages); err != nil {
		return body, nil
	}

	passthrough := true
	defer func() {
		if passthrough && m.stats != nil {
			m.stats.RecordPassthrough()
		}
		if passthrough {
			slog.Debug("[pruner] passthrough request")
		}
	}()

	modified := false
	var totalBefore, totalAfter, msgsPruned int
	for i, msg := range messages {
		// Determine if this message is prunable
		switch msg.Role {
		case "tool":
			// Tool messages: prune if content is long enough
			if DetectContentType(msg.Content, m.config.MinLines) == ContentUnknown {
				continue
			}
		case "user", "system":
			// User/system messages: prune if content is long enough
			if DetectContentType(msg.Content, m.config.MinLines) == ContentUnknown {
				continue
			}
		default:
			continue
		}

		// Derive query: for tool messages, look at prior conversation context;
		// for user/system messages, use the first 500 chars as self-query.
		var query string
		if msg.Role == "tool" {
			query = extractQueryContext(messages, i)
		} else {
			query = msg.Content
			if len(query) > 500 {
				query = query[:500]
			}
		}

		result, err := m.backend.Prune(ctx, PruneRequest{
			Content:   msg.Content,
			Code:      msg.Content, // backward compat
			Query:     query,
			Threshold: m.config.Threshold,
		})
		if err != nil {
			slog.Warn("[pruner] prune failed for message",
				"message_index", i,
				"role", msg.Role,
				"error", err)
			continue
		}

		// Prefer PrunedContent, fall back to PrunedCode for backward compat
		pruned := result.PrunedContent
		if pruned == "" {
			pruned = result.PrunedCode
		}
		messages[i].Content = pruned
		if m.stats != nil {
			m.stats.Record(result)
		}
		totalBefore += result.OriginalTokens
		totalAfter += result.PrunedTokens
		msgsPruned++
		modified = true
	}

	if !modified {
		return body, nil
	}
	passthrough = false

	saved := totalBefore - totalAfter
	slog.Info("[pruner] request pruned",
		"messages_pruned", msgsPruned,
		"tokens_before", totalBefore,
		"tokens_after", totalAfter,
		"tokens_saved", saved)

	// Populate per-request stats if context has a slot
	if stats := GetPruneStats(ctx); stats != nil {
		stats.Pruned = true
		stats.MessagesPruned = msgsPruned
		stats.TokensBefore = totalBefore
		stats.TokensAfter = totalAfter
	}

	// Re-serialize messages back into the raw map
	newMessages, err := json.Marshal(messages)
	if err != nil {
		return body, nil
	}
	raw["messages"] = newMessages

	return json.Marshal(raw)
}

// extractQueryContext derives a pruning hint from the conversation context.
// It looks for the last user or assistant message before the tool message.
func extractQueryContext(messages []openaiMessage, toolIdx int) string {
	for i := toolIdx - 1; i >= 0; i-- {
		if messages[i].Role == "user" || messages[i].Role == "assistant" {
			content := messages[i].Content
			if len(content) > 500 {
				content = content[:500]
			}
			return content
		}
	}
	return ""
}

// Enabled returns whether the middleware is active.
func (m *Middleware) Enabled() bool {
	return m.config.Enabled && m.backend != nil
}

// SetEnabled toggles the middleware on or off at runtime.
func (m *Middleware) SetEnabled(enabled bool) {
	m.config.Enabled = enabled
}

// SetBackend swaps the pruning backend at runtime.
func (m *Middleware) SetBackend(b Backend) {
	m.backend = b
}

// GetStats returns the current pruning statistics.
func (m *Middleware) GetStats() *Stats {
	return m.stats
}
