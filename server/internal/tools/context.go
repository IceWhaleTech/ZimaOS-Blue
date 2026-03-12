package tools

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

type toolContextKey string

const (
	langKey        toolContextKey = "tool_lang"
	channelKey     toolContextKey = "tool_channel"
	deviceKey      toolContextKey = "tool_device"
	cardEmitKey    toolContextKey = "tool_card_emit"
	sessionIDKey   toolContextKey = "tool_session_id"
	checkpointKey  toolContextKey = "tool_browser_checkpoint"
	browserModeKey toolContextKey = "tool_browser_launch_mode"
	fsScopeKey     toolContextKey = "tool_fs_scope"
)

type BrowserLaunchMode string

type fsScopeContext struct {
	roots   []string
	aliases map[string]string
}

const (
	BrowserLaunchModeDefault BrowserLaunchMode = ""
	BrowserLaunchModeVisible BrowserLaunchMode = "visible"
)

// CardEmitFunc is a callback that tools can use to emit streaming typeless
// cards during execution. The card map is JSON-serialised and pushed to the
// client's SSE stream immediately.
type CardEmitFunc func(card map[string]interface{})

// BrowserCheckpointFunc asks host application to resolve a browser checkpoint.
type BrowserCheckpointFunc func(ctx context.Context, req BrowserCheckpointRequest) (BrowserCheckpointResult, error)

// BrowserCheckpointResult is the host resolution for a checkpoint request.
type BrowserCheckpointResult struct {
	Decision     BrowserCheckpointDecision
	CheckpointID string
	Pending      bool
	Message      string
}

// WithCardEmitter returns a context carrying a card emitter callback.
func WithCardEmitter(ctx context.Context, fn CardEmitFunc) context.Context {
	return context.WithValue(ctx, cardEmitKey, fn)
}

// WithBrowserCheckpointRequester returns a context carrying checkpoint callback.
func WithBrowserCheckpointRequester(ctx context.Context, fn BrowserCheckpointFunc) context.Context {
	return context.WithValue(ctx, checkpointKey, fn)
}

// EmitCard sends a typeless card to the client if an emitter is set.
// Safe to call even when no emitter is present (no-op).
// Automatically adds "typeless": true to the card.
func EmitCard(ctx context.Context, card map[string]interface{}) {
	// Add typeless tag to identify this as a typeless card
	card["typeless"] = true
	if fn, ok := ctx.Value(cardEmitKey).(CardEmitFunc); ok && fn != nil {
		fn(card)
	}
}

// RequestBrowserCheckpoint asks the host to resolve a browser checkpoint.
// Returns ok=false when no requester exists in context.
func RequestBrowserCheckpoint(ctx context.Context, req BrowserCheckpointRequest) (BrowserCheckpointResult, bool, error) {
	fn, ok := ctx.Value(checkpointKey).(BrowserCheckpointFunc)
	if !ok || fn == nil {
		return BrowserCheckpointResult{}, false, nil
	}
	result, err := fn(ctx, req)
	return result, true, err
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

// WithSessionID returns a context carrying the conversation/session ID.
func WithSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, sessionIDKey, id)
}

// GetSessionID extracts the conversation/session ID from the context.
func GetSessionID(ctx context.Context) string {
	if v, ok := ctx.Value(sessionIDKey).(string); ok {
		return v
	}
	return ""
}

// WithFSScope returns a context carrying additional filesystem roots and aliases
// for file tools (read/write/edit/grep/find/ls).
func WithFSScope(ctx context.Context, roots []string, aliases map[string]string) context.Context {
	normalizedRoots := make([]string, 0, len(roots))
	seenRoots := make(map[string]struct{}, len(roots))
	for _, raw := range roots {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		abs, err := filepath.Abs(trimmed)
		if err != nil {
			continue
		}
		clean := filepath.Clean(abs)
		if _, ok := seenRoots[clean]; ok {
			continue
		}
		seenRoots[clean] = struct{}{}
		normalizedRoots = append(normalizedRoots, clean)
	}

	normalizedAliases := make(map[string]string, len(aliases))
	for rawAlias, rawPath := range aliases {
		alias := normalizeFSAliasKey(rawAlias)
		if alias == "" {
			continue
		}
		trimmed := strings.TrimSpace(rawPath)
		if trimmed == "" {
			continue
		}
		abs, err := filepath.Abs(trimmed)
		if err != nil {
			continue
		}
		normalizedAliases[alias] = filepath.Clean(abs)
	}

	if len(normalizedRoots) == 0 && len(normalizedAliases) == 0 {
		return ctx
	}

	return context.WithValue(ctx, fsScopeKey, fsScopeContext{
		roots:   normalizedRoots,
		aliases: normalizedAliases,
	})
}

// GetFSScope returns additional filesystem roots and aliases from the context.
func GetFSScope(ctx context.Context) (roots []string, aliases map[string]string) {
	v, ok := ctx.Value(fsScopeKey).(fsScopeContext)
	if !ok {
		return nil, nil
	}

	if len(v.roots) > 0 {
		roots = make([]string, len(v.roots))
		copy(roots, v.roots)
	}
	if len(v.aliases) > 0 {
		aliases = make(map[string]string, len(v.aliases))
		for k, path := range v.aliases {
			aliases[k] = path
		}
	}
	return roots, aliases
}

func normalizeFSAliasKey(alias string) string {
	return strings.ToLower(strings.TrimSpace(alias))
}

// WithBrowserLaunchMode returns a context carrying a browser launch hint.
func WithBrowserLaunchMode(ctx context.Context, mode BrowserLaunchMode) context.Context {
	if mode == BrowserLaunchModeDefault {
		return ctx
	}
	return context.WithValue(ctx, browserModeKey, mode)
}

// GetBrowserLaunchMode extracts the browser launch hint from the context.
func GetBrowserLaunchMode(ctx context.Context) BrowserLaunchMode {
	if v, ok := ctx.Value(browserModeKey).(BrowserLaunchMode); ok {
		return v
	}
	if v, ok := ctx.Value(browserModeKey).(string); ok {
		return BrowserLaunchMode(v)
	}
	return BrowserLaunchModeDefault
}
