package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sandbox"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newRuntimeExecSandboxExecutor(manager *sandbox.Manager) tools.SandboxExecutor {
	if manager == nil || !manager.SupportsTier(sandbox.TierLight) {
		return nil
	}
	return &sandboxExecAdapter{mgr: manager, tier: sandbox.TierLight}
}

func newRuntimeExecStrongSandboxExecutor(manager *sandbox.Manager) tools.SandboxExecutor {
	if manager == nil || !manager.SupportsTier(sandbox.TierStrong) {
		return nil
	}
	return &sandboxExecAdapter{mgr: manager, tier: sandbox.TierStrong}
}
