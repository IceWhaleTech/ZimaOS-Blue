package proxy

import "context"

type sessionIDKeyType struct{}
type disableResponsesContinuationKeyType struct{}

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
