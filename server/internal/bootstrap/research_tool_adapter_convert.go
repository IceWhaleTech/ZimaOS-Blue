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
	out := &tools.ResearchJob{
		ID:                 job.ID,
		ConversationID:     job.ConversationID,
		Status:             string(job.Status),
		Query:              job.Query,
		Mode:               string(job.Mode),
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
	out.Report = map[string]interface{}{
		"answer":               job.Report.Answer,
		"confidence":           job.Report.Confidence,
		"citations":            append([]deepresearch.Citation(nil), job.Report.Citations...),
		"open_questions":       append([]string(nil), job.Report.OpenQuestions...),
		"support_count":        job.Report.SupportCount,
		"conflict_count":       job.Report.ConflictCount,
		"has_conflict":         job.Report.HasConflict,
		"iterations":           job.Report.Iterations,
		"stop_reason":          job.Report.StopReason,
		"citation_coverage":    job.Report.CitationCoverage,
		"stage_errors":         append([]string(nil), job.Report.StageErrors...),
		"timeline_sections":    append([]deepresearch.TimelineSection(nil), job.Report.TimelineSections...),
		"research_trace":       append([]deepresearch.ResearchTraceEntry(nil), job.Report.ResearchTrace...),
		"verification_summary": job.Report.VerificationSummary,
		"calibration":          cloneCalibrationForTool(job.Report.Calibration),
	}
	if job.Report.Calibration != nil {
		out.Report["takeaway_candidates"] = append([]deepresearch.TakeawayCandidate(nil), job.Report.Calibration.TakeawayCandidates...)
	}
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
