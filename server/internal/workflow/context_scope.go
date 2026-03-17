package workflow

import (
	"context"
	"strings"
)

type workflowContextKey string

const workflowTenantContextKey workflowContextKey = "workflow.tenant_id"

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
