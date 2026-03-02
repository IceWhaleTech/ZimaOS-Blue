package pruner

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
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

// Middleware intercepts proxy requests and prunes code content in tool messages.
type Middleware struct {
	mu      sync.RWMutex
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
	m.mu.RLock()
	enabled := m.config.Enabled
	backend := m.backend
	minLines := m.config.MinLines
	threshold := m.config.Threshold
	statsCollector := m.stats
	m.mu.RUnlock()

	if !enabled || backend == nil {
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

	var messages []map[string]json.RawMessage
	if err := json.Unmarshal(messagesRaw, &messages); err != nil {
		return body, nil
	}

	passthrough := true
	defer func() {
		if passthrough && statsCollector != nil {
			statsCollector.RecordPassthrough()
		}
		if passthrough {
			slog.Debug("[pruner] passthrough request")
		}
	}()

	modified := false
	var totalBefore, totalAfter, msgsPruned int
	for i, msg := range messages {
		role, ok := rawMessageString(msg, "role")
		if !ok {
			continue
		}
		content, ok := rawMessageString(msg, "content")
		if !ok {
			// Skip non-string content (e.g. multimodal array parts).
			continue
		}

		// Determine if this message is prunable
		switch role {
		case "tool":
			// Tool messages: prune if content is long enough
			if DetectContentType(content, minLines) == ContentUnknown {
				continue
			}
		case "user", "system":
			// User/system messages: prune if content is long enough
			if DetectContentType(content, minLines) == ContentUnknown {
				continue
			}
		default:
			continue
		}

		// Derive query: for tool messages, look at prior conversation context;
		// for user/system messages, use the first 500 chars as self-query.
		var query string
		if role == "tool" {
			query = extractQueryContextRaw(messages, i)
		} else {
			query = content
			if len(query) > 500 {
				query = query[:500]
			}
		}

		result, err := backend.Prune(ctx, PruneRequest{
			Content:   content,
			Code:      content, // backward compat
			Query:     query,
			Threshold: threshold,
		})
		if err != nil {
			slog.Warn("[pruner] prune failed for message",
				"message_index", i,
				"role", role,
				"error", err)
			continue
		}

		// Prefer PrunedContent, fall back to PrunedCode for backward compat
		pruned := result.PrunedContent
		if pruned == "" {
			pruned = result.PrunedCode
		}
		if err := setRawMessageString(messages[i], "content", pruned); err != nil {
			slog.Warn("[pruner] failed to write pruned content", "message_index", i, "error", err)
			continue
		}

		if statsCollector != nil {
			statsCollector.Record(result)
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

// extractQueryContextRaw derives a pruning hint from prior string-based user/assistant messages.
func extractQueryContextRaw(messages []map[string]json.RawMessage, toolIdx int) string {
	for i := toolIdx - 1; i >= 0; i-- {
		role, ok := rawMessageString(messages[i], "role")
		if !ok || (role != "user" && role != "assistant") {
			continue
		}
		content, ok := rawMessageString(messages[i], "content")
		if !ok {
			continue
		}
		if len(content) > 500 {
			content = content[:500]
		}
		return content
	}
	return ""
}

func rawMessageString(msg map[string]json.RawMessage, key string) (string, bool) {
	raw, ok := msg[key]
	if !ok {
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	return s, true
}

func setRawMessageString(msg map[string]json.RawMessage, key, value string) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	msg[key] = encoded
	return nil
}

// Enabled returns whether the middleware is active.
func (m *Middleware) Enabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.Enabled && m.backend != nil
}

// SetEnabled toggles the middleware on or off at runtime.
func (m *Middleware) SetEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.Enabled = enabled
}

// SetBackend swaps the pruning backend at runtime.
func (m *Middleware) SetBackend(b Backend) Backend {
	m.mu.Lock()
	defer m.mu.Unlock()
	old := m.backend
	m.backend = b
	return old
}

// GetStats returns the current pruning statistics.
func (m *Middleware) GetStats() *Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}
