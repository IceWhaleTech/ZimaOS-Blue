package uiexec

import (
	"context"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a11y"
)

// SemanticRuntime is the minimal interface the execution engine needs from the AX/UIA layer.
// It is intentionally small to keep the execution engine app-agnostic and testable.
type SemanticRuntime interface {
	Capture(ctx context.Context, scope a11y.CaptureScope) (a11y.RawTree, error)
	Compile(raw a11y.RawTree, options a11y.CompileOptions) (*a11y.Snapshot, error)
	Execute(ctx context.Context, snapshot *a11y.Snapshot, action a11y.Action) (a11y.ExecutionResult, error)
	Refresh(ctx context.Context, prior *a11y.Snapshot, hint a11y.RefreshHint) (*a11y.Snapshot, error)
}

// SDKRuntime adapts the existing a11y.SemanticSDK into a SemanticRuntime.
type SDKRuntime struct {
	SDK *a11y.SemanticSDK
}

func (r SDKRuntime) Capture(ctx context.Context, scope a11y.CaptureScope) (a11y.RawTree, error) {
	if r.SDK == nil {
		return a11y.RawTree{}, a11y.NewError("backend_unavailable", "semantic runtime is unavailable", nil)
	}
	return r.SDK.Capture(ctx, scope)
}

func (r SDKRuntime) Compile(raw a11y.RawTree, options a11y.CompileOptions) (*a11y.Snapshot, error) {
	if r.SDK == nil {
		return nil, a11y.NewError("backend_unavailable", "semantic runtime is unavailable", nil)
	}
	return r.SDK.Compile(raw, options)
}

func (r SDKRuntime) Execute(ctx context.Context, snapshot *a11y.Snapshot, action a11y.Action) (a11y.ExecutionResult, error) {
	if r.SDK == nil {
		return a11y.ExecutionResult{}, a11y.NewError("backend_unavailable", "semantic runtime is unavailable", nil)
	}
	return r.SDK.Execute(ctx, snapshot, action)
}

func (r SDKRuntime) Refresh(ctx context.Context, prior *a11y.Snapshot, hint a11y.RefreshHint) (*a11y.Snapshot, error) {
	if r.SDK == nil {
		return nil, a11y.NewError("backend_unavailable", "semantic runtime is unavailable", nil)
	}
	return r.SDK.Refresh(ctx, prior, hint)
}
