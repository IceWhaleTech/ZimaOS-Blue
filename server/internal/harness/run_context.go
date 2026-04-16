package harness

import (
	"context"
	"encoding/json"
	"strings"
	"time"

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
	if providerID := strings.TrimSpace(run.ProviderID); providerID != "" {
		ctx = tools.WithProviderID(ctx, providerID)
	}
	if model := strings.TrimSpace(run.Model); model != "" {
		ctx = tools.WithModel(ctx, model)
	}
	if agentID := strings.TrimSpace(run.AgentID); agentID != "" {
		ctx = tools.WithAgentID(ctx, agentID)
	}
	if artifactRoot := strings.TrimSpace(run.ArtifactRoot); artifactRoot != "" {
		ctx = tools.WithRunArtifactRoot(ctx, artifactRoot)
	}
	if runCtx := GetRunContext(ctx); runCtx != nil && runCtx.Controller != nil && strings.TrimSpace(run.ID) != "" {
		ctx = tools.WithArtifactEmitter(ctx, func(emitCtx context.Context, artifact tools.ToolArtifact) error {
			metadataJSON := ""
			if len(artifact.Metadata) > 0 {
				if raw, err := json.Marshal(artifact.Metadata); err == nil {
					metadataJSON = string(raw)
				}
			}
			return runCtx.Controller.AttachArtifact(emitCtx, ArtifactRef{
				RunID:        strings.TrimSpace(run.ID),
				Kind:         strings.TrimSpace(artifact.Kind),
				Label:        strings.TrimSpace(artifact.Label),
				PathOrURL:    strings.TrimSpace(artifact.PathOrURL),
				MIMEType:     strings.TrimSpace(artifact.MimeType),
				SizeBytes:    artifact.SizeBytes,
				MetadataJSON: metadataJSON,
			})
		})
		ctx = tools.WithEventEmitter(ctx, func(emitCtx context.Context, event tools.ToolEvent) error {
			payloadJSON := ""
			if len(event.Payload) > 0 {
				if raw, err := json.Marshal(event.Payload); err == nil {
					payloadJSON = string(raw)
				}
			}
			return runCtx.Controller.AppendEvent(emitCtx, RunEvent{
				RunID:          strings.TrimSpace(run.ID),
				RootRunID:      strings.TrimSpace(run.RootRunID),
				ParentRunID:    strings.TrimSpace(run.ParentRunID),
				Type:           strings.TrimSpace(event.Type),
				StepIndex:      event.StepIndex,
				ToolName:       strings.TrimSpace(event.ToolName),
				CapabilityKind: strings.TrimSpace(event.CapabilityKind),
				Message:        strings.TrimSpace(event.Message),
				PayloadJSON:    payloadJSON,
				CreatedAt:      time.Now().UTC(),
			})
		})
	}
	return ctx
}
