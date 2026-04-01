package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sandbox"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newRuntimeExecSandboxExecutor(manager *sandbox.Manager) tools.SandboxExecutor {
	if manager == nil || !manager.IsSupported() {
		return nil
	}
	return &sandboxExecAdapter{mgr: manager}
}
