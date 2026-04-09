package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sandbox"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newRuntimeExecSandboxExecutor(manager *sandbox.Manager) tools.SandboxExecutor {
	return newRuntimeExecSandboxExecutorForTier(manager, sandbox.TierLight)
}

func newRuntimeExecStrongSandboxExecutor(manager *sandbox.Manager) tools.SandboxExecutor {
	return newRuntimeExecSandboxExecutorForTier(manager, sandbox.TierStrong)
}
