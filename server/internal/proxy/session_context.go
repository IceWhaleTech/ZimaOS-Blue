package proxy

import "context"

type sessionIDKeyType struct{}

// WithSessionID stores the proxy session ID in request context.
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDKeyType{}, sessionID)
}

// SessionIDFromContext returns proxy session ID from request context.
func SessionIDFromContext(ctx context.Context) string {
	sessionID, _ := ctx.Value(sessionIDKeyType{}).(string)
	return sessionID
}
