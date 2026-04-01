package bootstrap

import (
	networkapi "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/api"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type runtimeExecResolverTarget interface {
	SetExecResolver(resolver networkapi.ExecApprovalResolver)
}

type approvalRuntimeHandlerTarget interface {
	runtimeExecResolverTarget
	runtimeObserverTarget
}

type approvalRuntimeDetailTarget interface {
	SetApprovalHandler(handler *networkapi.ApprovalHandler)
}

type approvalRuntimeBinding struct {
	handler      *networkapi.ApprovalHandler
	approver     tools.ToolApprover
	execResolver networkapi.ExecApprovalResolver
	observer     tools.RuntimeEventObserver
	workflow     workflowRuntimeBinding
}
