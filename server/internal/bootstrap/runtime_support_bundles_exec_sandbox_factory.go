package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sandbox"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newRuntimeExecSandboxExecutorForTier(manager *sandbox.Manager, tier sandbox.Tier) tools.SandboxExecutor {
	if manager == nil || !manager.SupportsTier(tier) {
		return nil
	}
	return &sandboxExecAdapter{mgr: manager, tier: tier}
}
