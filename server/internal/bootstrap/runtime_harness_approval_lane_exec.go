package bootstrap

import (
	networkapi "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/api"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// execApprovalAdapter bridges tools.ApprovalManager to api.ExecApprovalResolver.
type execApprovalAdapter struct {
	mgr *tools.ApprovalManager
}

func (a execApprovalAdapter) ResolveApproval(id string, decision string, bindingHash string) networkapi.ExecApprovalResolveResult {
	switch a.mgr.ResolveApprovalWithBindingStatus(id, tools.ApprovalDecision(decision), bindingHash) {
	case tools.ApprovalResolveSuccess:
		return networkapi.ExecApprovalResolveResult{Resolved: true}
	case tools.ApprovalResolveBindingMismatch:
		return networkapi.ExecApprovalResolveResult{BindingMismatch: true}
	default:
		return networkapi.ExecApprovalResolveResult{}
	}
}
