package tools

import "context"

const writePathGuardKey toolContextKey = "tool_write_path_guard"

// WritePathGuard enforces runtime-specific direct-write restrictions for file
// mutation tools after workspace/approval path resolution.
type WritePathGuard interface {
	CheckWritePath(ctx context.Context, absPath string) error
}

// WithWritePathGuard returns a context carrying a direct-write guard.
func WithWritePathGuard(ctx context.Context, guard WritePathGuard) context.Context {
	if guard == nil {
		return ctx
	}
	return context.WithValue(ctx, writePathGuardKey, guard)
}

// GetWritePathGuard extracts the direct-write guard from the context.
func GetWritePathGuard(ctx context.Context) WritePathGuard {
	if ctx == nil {
		return nil
	}
	guard, _ := ctx.Value(writePathGuardKey).(WritePathGuard)
	return guard
}

func enforceWritePathGuard(ctx context.Context, absPath string) error {
	guard := GetWritePathGuard(ctx)
	if guard == nil {
		return nil
	}
	return guard.CheckWritePath(ctx, absPath)
}
