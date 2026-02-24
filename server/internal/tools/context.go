package tools

import (
	"context"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

type toolContextKey string

const (
	langKey     toolContextKey = "tool_lang"
	channelKey  toolContextKey = "tool_channel"
	deviceKey   toolContextKey = "tool_device"
	cardEmitKey toolContextKey = "tool_card_emit"
)

// CardEmitFunc is a callback that tools can use to emit streaming typeless
// cards during execution. The card map is JSON-serialised and pushed to the
// client's SSE stream immediately.
type CardEmitFunc func(card map[string]interface{})

// WithCardEmitter returns a context carrying a card emitter callback.
func WithCardEmitter(ctx context.Context, fn CardEmitFunc) context.Context {
	return context.WithValue(ctx, cardEmitKey, fn)
}

// EmitCard sends a typeless card to the client if an emitter is set.
// Safe to call even when no emitter is present (no-op).
func EmitCard(ctx context.Context, card map[string]interface{}) {
	if fn, ok := ctx.Value(cardEmitKey).(CardEmitFunc); ok && fn != nil {
		fn(card)
	}
}

// WithUserID returns a context carrying the user ID for tool/skill execution.
func WithUserID(ctx context.Context, userID string) context.Context {
	return skill.WithUserID(ctx, userID)
}

// GetUserID extracts the user ID from the context.
func GetUserID(ctx context.Context) string {
	return skill.GetUserID(ctx)
}

// WithLang returns a context carrying the language/locale (e.g. "en-US", "zh-CN").
func WithLang(ctx context.Context, lang string) context.Context {
	return context.WithValue(ctx, langKey, lang)
}

// GetLang extracts the language from the context. Returns "en-US" if not set.
func GetLang(ctx context.Context) string {
	if v, ok := ctx.Value(langKey).(string); ok && v != "" {
		return v
	}
	return "en-US"
}

// WithChannel returns a context carrying the channel name (e.g. "telegram", "web").
func WithChannel(ctx context.Context, ch string) context.Context {
	return context.WithValue(ctx, channelKey, ch)
}

// GetChannel extracts the channel name from the context.
func GetChannel(ctx context.Context) string {
	if v, ok := ctx.Value(channelKey).(string); ok {
		return v
	}
	return ""
}

// WithDevice returns a context carrying the device type ("desktop" or "mobile").
func WithDevice(ctx context.Context, device string) context.Context {
	return context.WithValue(ctx, deviceKey, device)
}

// GetDevice extracts the device type from the context.
func GetDevice(ctx context.Context) string {
	if v, ok := ctx.Value(deviceKey).(string); ok {
		return v
	}
	return ""
}
