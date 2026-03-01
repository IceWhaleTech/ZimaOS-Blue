package proxy

import "context"

type sessionIDKeyType struct{}
type disableResponsesContinuationKeyType struct{}
type localeKeyType struct{}

// WithSessionID stores the proxy session ID in request context.
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDKeyType{}, sessionID)
}

// SessionIDFromContext returns proxy session ID from request context.
func SessionIDFromContext(ctx context.Context) string {
	sessionID, _ := ctx.Value(sessionIDKeyType{}).(string)
	return sessionID
}

// WithDisableResponsesContinuation marks a request context so bridge/proxy
// strips previous_response_id before forwarding to Responses endpoints.
func WithDisableResponsesContinuation(ctx context.Context) context.Context {
	return context.WithValue(ctx, disableResponsesContinuationKeyType{}, true)
}

// DisableResponsesContinuationFromContext reports whether continuation should be disabled.
func DisableResponsesContinuationFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(disableResponsesContinuationKeyType{}).(bool)
	return v
}

// WithLocale stores the request locale in context (e.g. "en-US", "zh-CN").
func WithLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, localeKeyType{}, locale)
}

// LocaleFromContext returns locale from request context.
func LocaleFromContext(ctx context.Context) string {
	locale, _ := ctx.Value(localeKeyType{}).(string)
	return locale
}
