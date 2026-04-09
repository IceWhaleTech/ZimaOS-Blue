package bootstrap

import (
	"context"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sandbox"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// sandboxExecAdapter wraps sandbox.Manager to implement tools.SandboxExecutor.
type sandboxExecAdapter struct {
	mgr  *sandbox.Manager
	tier sandbox.Tier
}

func (a *sandboxExecAdapter) RunInSandbox(ctx context.Context, command, workdir string, env map[string]string, timeout time.Duration) (string, string, int, error) {
	shell, shellArgs := tools.GetShellConfig()
	req := sandbox.NewExecutionRequest(shell, append(shellArgs, command)...)
	req.Tier = a.tier
	req.WorkDir = workdir
	req.Timeout = timeout
	req.Env = env

	result, err := a.mgr.Execute(ctx, req)
	if err != nil {
		return "", "", -1, err
	}
	return result.Stdout, result.Stderr, result.ExitCode, nil
}
