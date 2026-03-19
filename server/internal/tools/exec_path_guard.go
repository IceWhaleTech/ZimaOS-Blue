package tools

import "context"

const execPathGuardKey toolContextKey = "tool_exec_path_guard"

// ExecPathGuard enforces runtime-specific path restrictions for exec-style
// tools before directory approvals or host execution.
type ExecPathGuard interface {
	CheckExecWorkdir(ctx context.Context, absWorkdir string) error
	CheckExecPath(ctx context.Context, absPath string) error
}

// WithExecPathGuard returns a context carrying an exec path guard.
func WithExecPathGuard(ctx context.Context, guard ExecPathGuard) context.Context {
	if guard == nil {
		return ctx
	}
	return context.WithValue(ctx, execPathGuardKey, guard)
}

// GetExecPathGuard extracts the exec path guard from the context.
func GetExecPathGuard(ctx context.Context) ExecPathGuard {
	if ctx == nil {
		return nil
	}
	guard, _ := ctx.Value(execPathGuardKey).(ExecPathGuard)
	return guard
}

func enforceExecWorkdirGuard(ctx context.Context, absWorkdir string) error {
	guard := GetExecPathGuard(ctx)
	if guard == nil {
		return nil
	}
	return guard.CheckExecWorkdir(ctx, absWorkdir)
}

func enforceExecPathGuard(ctx context.Context, absPath string) error {
	guard := GetExecPathGuard(ctx)
	if guard == nil {
		return nil
	}
	return guard.CheckExecPath(ctx, absPath)
}
