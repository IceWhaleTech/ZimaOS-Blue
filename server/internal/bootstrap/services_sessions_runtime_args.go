package bootstrap

import (
	"context"
	"strings"
)

func runtimeCompatString(args map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if args == nil {
			continue
		}
		value, ok := args[key]
		if !ok {
			continue
		}
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func runtimeCompatInt(args map[string]interface{}, key string, fallback int) int {
	if args == nil {
		return fallback
	}
	raw, ok := args[key]
	if !ok {
		return fallback
	}
	switch typed := raw.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return fallback
	}
}

func runtimeCompatTitle(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) <= 72 {
		return trimmed
	}
	return trimmed[:72]
}

func runtimeSessionUserID(ctx context.Context, args map[string]interface{}) string {
	return sessionScopedUserID(ctx, runtimeCompatString(args, "user_id", "user", "owner_id", "ownerId"))
}

func runtimeSessionID(args map[string]interface{}) string {
	return runtimeCompatString(args, "id", "session_id", "session", "conversation_id")
}
