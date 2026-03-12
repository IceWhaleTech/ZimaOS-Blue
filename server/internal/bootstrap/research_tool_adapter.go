package bootstrap

import (
	"context"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type deepResearchToolAdapter struct {
	service *deepresearch.Service
}

func newDeepResearchToolAdapter(service *deepresearch.Service) *deepResearchToolAdapter {
	return &deepResearchToolAdapter{service: service}
}

func (a *deepResearchToolAdapter) CreateJob(ctx context.Context, req tools.ResearchCreateJobRequest) (*tools.ResearchJob, error) {
	if a == nil || a.service == nil {
		return nil, deepresearch.ErrJobNotFound
	}
	job, err := a.service.CreateJob(ctx, deepresearch.CreateJobRequest{
		UserID:       req.UserID,
		ConversationID: req.ConversationID,
		Query:        req.Query,
		Mode:         deepresearch.Mode(req.Mode),
		RouteMode:    deepresearch.RouteMode(req.RouteMode),
		Lang:         req.Lang,
		Budget:       toDeepResearchBudget(req.Budget),
		StrictEntity: req.StrictEntity,
		TimeWindows:  append([]string(nil), req.TimeWindows...),
		ReportStyle:  req.ReportStyle,
	})
	if err != nil {
		return nil, err
	}
	return toToolResearchJob(job), nil
}

func (a *deepResearchToolAdapter) GetJobForUser(id, userID string) (*tools.ResearchJob, error) {
	if a == nil || a.service == nil {
		return nil, deepresearch.ErrJobNotFound
	}
	job, err := a.service.GetJobForUser(id, userID, "")
	if err != nil {
		return nil, err
	}
	return toToolResearchJob(job), nil
}

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
	if job.Report != nil {
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
			"experiment":           cloneExperimentReportForTool(job.Report.Experiment),
		}
	}
	return out
}

func cloneExperimentReportForTool(report *deepresearch.ExperimentReport) map[string]interface{} {
	if report == nil {
		return nil
	}
	out := map[string]interface{}{
		"summary":        report.Summary,
		"findings":       append([]string(nil), report.Findings...),
		"artifacts":      append([]deepresearch.ExperimentArtifact(nil), report.Artifacts...),
		"open_questions": append([]string(nil), report.OpenQuestions...),
	}
	if len(report.Metadata) > 0 {
		meta := make(map[string]interface{}, len(report.Metadata))
		for k, v := range report.Metadata {
			meta[k] = v
		}
		out["metadata"] = meta
	}
	return out
}
