package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func toDeepResearchBudget(budget *tools.ResearchBudget) *deepresearch.Budget {
	if budget == nil {
		return nil
	}
	return &deepresearch.Budget{MaxSources: budget.MaxSources, MaxSeconds: budget.MaxSeconds}
}

func toToolResearchJob(job *deepresearch.Job) *tools.ResearchJob {
	if job == nil {
		return nil
	}
	mode, researchDepth := toolResearchModeAndDepth(job.Mode)
	out := &tools.ResearchJob{
		ID:                 job.ID,
		ConversationID:     job.ConversationID,
		Status:             string(job.Status),
		Query:              job.Query,
		Mode:               mode,
		ResearchDepth:      researchDepth,
		RequestedRouteMode: string(job.RequestedRouteMode),
		EffectiveRouteMode: string(job.EffectiveRouteMode),
		RouteReason:        job.RouteReason,
		Progress:           job.Progress,
		EvidenceCount:      len(job.Evidence),
		Error:              job.Error,
	}
	if job.Report == nil {
		return out
	}
	out.Answer = job.Report.Answer
	out.Confidence = job.Report.Confidence
	out.RetrievalProfile = job.Report.RetrievalProfile
	out.Report = toolResearchReport(job.Report)
	return out
}

func cloneCalibrationForTool(calibration *deepresearch.Calibration) map[string]interface{} {
	if calibration == nil {
		return nil
	}
	out := map[string]interface{}{
		"coverage":           calibration.Coverage,
		"groundedness":       calibration.Groundedness,
		"freshness":          calibration.Freshness,
		"conflict_risk":      calibration.ConflictRisk,
		"confidence":         calibration.Confidence,
		"recommended_action": calibration.RecommendedAction,
	}
	out["takeaway_candidates"] = append([]deepresearch.TakeawayCandidate(nil), calibration.TakeawayCandidates...)
	return out
}
