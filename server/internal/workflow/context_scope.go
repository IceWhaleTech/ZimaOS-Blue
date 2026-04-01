package workflow

import (
	"context"
	"strings"
)

type workflowContextKey string

const workflowTenantContextKey workflowContextKey = "workflow.tenant_id"
const workflowUserContextKey workflowContextKey = "workflow.user_id"
const workflowConversationContextKey workflowContextKey = "workflow.conversation_id"

func withWorkflowTenant(ctx context.Context, tenantID string) context.Context {
	trimmed := strings.TrimSpace(tenantID)
	if trimmed == "" {
		return ctx
	}
	return context.WithValue(ctx, workflowTenantContextKey, trimmed)
}

func workflowTenantFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if tenantID, ok := ctx.Value(workflowTenantContextKey).(string); ok {
		return strings.TrimSpace(tenantID)
	}
	return ""
}

func WithTenantContext(ctx context.Context, tenantID string) context.Context {
	return withWorkflowTenant(ctx, tenantID)
}

func TenantFromContext(ctx context.Context) string {
	return workflowTenantFromContext(ctx)
}

func withWorkflowUser(ctx context.Context, userID string) context.Context {
	trimmed := strings.TrimSpace(userID)
	if trimmed == "" {
		return ctx
	}
	return context.WithValue(ctx, workflowUserContextKey, trimmed)
}

func workflowUserFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if userID, ok := ctx.Value(workflowUserContextKey).(string); ok {
		return strings.TrimSpace(userID)
	}
	return ""
}

func withWorkflowConversation(ctx context.Context, conversationID string) context.Context {
	trimmed := strings.TrimSpace(conversationID)
	if trimmed == "" {
		return ctx
	}
	return context.WithValue(ctx, workflowConversationContextKey, trimmed)
}

func workflowConversationFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if conversationID, ok := ctx.Value(workflowConversationContextKey).(string); ok {
		return strings.TrimSpace(conversationID)
	}
	return ""
}
