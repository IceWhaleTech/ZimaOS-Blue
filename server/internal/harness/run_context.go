package harness

import (
	"context"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type runContextKey struct{}

// RunContext captures the run-scoped execution envelope shared by controller,
// middlewares, and drivers during dispatch.
type RunContext struct {
	Run        *Run
	Parent     *Run
	Driver     Driver
	Controller *Controller
	Values     map[string]interface{}
}

// ExecutionMiddleware provides run-scoped lifecycle hooks around driver start.
type ExecutionMiddleware interface {
	BeforeStart(ctx context.Context, runCtx *RunContext) error
	AfterStart(ctx context.Context, runCtx *RunContext)
	OnStartError(ctx context.Context, runCtx *RunContext, runErr error)
}

// ExecutionMiddlewareHooks is a small adapter for wiring hook functions.
type ExecutionMiddlewareHooks struct {
	BeforeStartFunc  func(ctx context.Context, runCtx *RunContext) error
	AfterStartFunc   func(ctx context.Context, runCtx *RunContext)
	OnStartErrorFunc func(ctx context.Context, runCtx *RunContext, runErr error)
}

func (h ExecutionMiddlewareHooks) BeforeStart(ctx context.Context, runCtx *RunContext) error {
	if h.BeforeStartFunc == nil {
		return nil
	}
	return h.BeforeStartFunc(ctx, runCtx)
}

func (h ExecutionMiddlewareHooks) AfterStart(ctx context.Context, runCtx *RunContext) {
	if h.AfterStartFunc != nil {
		h.AfterStartFunc(ctx, runCtx)
	}
}

func (h ExecutionMiddlewareHooks) OnStartError(ctx context.Context, runCtx *RunContext, runErr error) {
	if h.OnStartErrorFunc != nil {
		h.OnStartErrorFunc(ctx, runCtx, runErr)
	}
}

// WithRunContext annotates a context with the current run-scoped execution envelope.
func WithRunContext(ctx context.Context, runCtx *RunContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if runCtx == nil {
		return ctx
	}
	return context.WithValue(ctx, runContextKey{}, runCtx)
}

// GetRunContext returns the run-scoped execution envelope carried on the context.
func GetRunContext(ctx context.Context) *RunContext {
	if ctx == nil {
		return nil
	}
	runCtx, _ := ctx.Value(runContextKey{}).(*RunContext)
	return runCtx
}

func annotateRunExecutionContext(ctx context.Context, run *Run) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if run == nil {
		return ctx
	}
	if runID := strings.TrimSpace(run.ID); runID != "" {
		ctx = tools.WithRunID(ctx, runID)
	}
	if userID := strings.TrimSpace(run.UserID); userID != "" {
		ctx = tools.WithUserID(ctx, userID)
	}
	if sessionID := strings.TrimSpace(run.SessionID); sessionID != "" {
		ctx = tools.WithSessionID(ctx, sessionID)
	}
	if model := strings.TrimSpace(run.Model); model != "" {
		ctx = tools.WithModel(ctx, model)
	}
	if agentID := strings.TrimSpace(run.AgentID); agentID != "" {
		ctx = tools.WithAgentID(ctx, agentID)
	}
	return ctx
}
