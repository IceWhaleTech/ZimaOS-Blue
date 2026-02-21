package sandbox

import (
	"context"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
)

// SkillAdapter adapts sandbox Manager to builtin.SandboxServiceInterface.
type SkillAdapter struct {
	mgr *Manager
}

// NewSkillAdapter creates a new skill adapter for the sandbox manager.
func NewSkillAdapter(mgr *Manager) *SkillAdapter {
	return &SkillAdapter{mgr: mgr}
}

func (a *SkillAdapter) Execute(ctx context.Context, command string, args []string, stdin string, timeoutSecs int) (builtin.SandboxResultInfo, error) {
	req := NewExecutionRequest(command, args...)
	req.Stdin = stdin
	if timeoutSecs > 0 {
		req.Timeout = time.Duration(timeoutSecs) * time.Second
	}

	result, err := a.mgr.Execute(ctx, req)
	if err != nil {
		return builtin.SandboxResultInfo{}, err
	}
	return toResultInfo(result), nil
}

func (a *SkillAdapter) GetStatus(id string) (builtin.SandboxResultInfo, error) {
	result, err := a.mgr.GetStatus(id)
	if err != nil {
		return builtin.SandboxResultInfo{}, err
	}
	return toResultInfo(result), nil
}

func (a *SkillAdapter) Kill(id string) error {
	return a.mgr.Kill(id)
}

func (a *SkillAdapter) IsSupported() bool {
	return a.mgr.IsSupported()
}

func toResultInfo(r *ExecutionResult) builtin.SandboxResultInfo {
	return builtin.SandboxResultInfo{
		ID:       r.ID,
		Status:   string(r.Status),
		ExitCode: r.ExitCode,
		Stdout:   r.Stdout,
		Stderr:   r.Stderr,
		Duration: r.Duration.String(),
		Error:    r.Error,
	}
}
