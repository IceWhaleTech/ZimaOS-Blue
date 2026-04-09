package sandbox

import (
	"context"
	"fmt"
)

// unsupportedExecutor is a fail-closed executor used when a platform-specific
// sandbox backend is unavailable or not yet implemented.
type unsupportedExecutor struct {
	reason string
}

func newUnsupportedExecutor(reason string) Executor {
	return &unsupportedExecutor{reason: reason}
}

func (e *unsupportedExecutor) Execute(context.Context, *ExecutionRequest) (*ExecutionResult, error) {
	if e.reason == "" {
		return nil, ErrSandboxNotSupported
	}
	return nil, fmt.Errorf("%w: %s", ErrSandboxNotSupported, e.reason)
}

func (e *unsupportedExecutor) GetStatus(string) (*ExecutionResult, error) {
	return nil, ErrExecutionNotFound
}

func (e *unsupportedExecutor) Kill(string) error {
	return ErrExecutionNotFound
}

func (e *unsupportedExecutor) Cleanup() error {
	return nil
}

func (e *unsupportedExecutor) IsSupported() bool {
	return false
}

func (e *unsupportedExecutor) SupportReason() string {
	return e.reason
}

func (e *unsupportedExecutor) SupportsNetworkEnabled() bool {
	return false
}

var _ Executor = (*unsupportedExecutor)(nil)
