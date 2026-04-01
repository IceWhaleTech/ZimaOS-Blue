package bootstrap

import (
	"strings"

	networkapi "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/api"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type harnessDetailProvider struct {
	approvalHandler *networkapi.ApprovalHandler
	execApprovals   *tools.ApprovalManager
	questionMgr     *tools.QuestionManager
}

func (p *harnessDetailProvider) SetApprovalHandler(handler *networkapi.ApprovalHandler) {
	if p == nil {
		return
	}
	p.approvalHandler = handler
}

func (p *harnessDetailProvider) PendingApprovals(runID string) []map[string]interface{} {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil
	}
	out := make([]map[string]interface{}, 0, 2)
	if p != nil && p.approvalHandler != nil {
		if req := p.approvalHandler.GetPendingByRun(runID); req != nil {
			out = append(out, map[string]interface{}{
				"kind":          "tool",
				"id":            req.ID,
				"run_id":        req.RunID,
				"step_index":    req.StepIndex,
				"tool_name":     req.ToolName,
				"tool_call_id":  req.ToolCallID,
				"session_id":    req.SessionID,
				"policy_source": req.PolicySource,
				"risk_level":    req.RiskLevel,
				"binding_hash":  req.BindingHash,
				"expires_at":    req.ExpiresAt,
			})
		}
	}
	if p != nil && p.execApprovals != nil {
		if req := p.execApprovals.GetPendingByRun(runID); req != nil {
			out = append(out, map[string]interface{}{
				"kind":          "exec",
				"id":            req.ID,
				"run_id":        req.RunID,
				"step_index":    req.StepIndex,
				"tool_name":     "exec",
				"command":       req.Command,
				"directory":     req.Directory,
				"session_id":    req.SessionID,
				"policy_source": req.PolicySource,
				"risk_level":    req.RiskLevel,
				"binding_hash":  req.BindingHash,
				"expires_at":    req.ExpiresAt,
			})
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (p *harnessDetailProvider) PendingQuestions(runID string) []map[string]interface{} {
	runID = strings.TrimSpace(runID)
	if runID == "" || p == nil || p.questionMgr == nil {
		return nil
	}
	req := p.questionMgr.GetPendingByRun(runID)
	if req == nil {
		return nil
	}
	return []map[string]interface{}{{
		"id":         req.ID,
		"run_id":     req.RunID,
		"step_index": req.StepIndex,
		"session_id": req.SessionID,
		"expires_at": req.ExpiresAt,
		"questions":  append([]tools.QuestionItem(nil), req.Questions...),
		"context":    cloneMap(req.Context),
	}}
}

func cloneMap(in map[string]interface{}) map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
