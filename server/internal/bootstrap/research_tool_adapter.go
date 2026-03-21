package bootstrap

import (
	"context"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type deepResearchToolAdapter struct {
	service              *deepresearch.Service
	manager              *harness.Controller
	defaultWorkspaceRoot string
}

func newDeepResearchToolAdapter(service *deepresearch.Service, manager *harness.Controller, defaultWorkspaceRoot string) *deepResearchToolAdapter {
	return &deepResearchToolAdapter{
		service:              service,
		manager:              manager,
		defaultWorkspaceRoot: strings.TrimSpace(defaultWorkspaceRoot),
	}
}

func (a *deepResearchToolAdapter) CreateJob(ctx context.Context, req tools.ResearchCreateJobRequest) (*tools.ResearchJob, error) {
	if a == nil || a.service == nil {
		return nil, deepresearch.ErrJobNotFound
	}
	if a.manager != nil {
		run, err := a.manager.Submit(ctx, harness.RunSpec{
			Kind:           harness.RunKindResearch,
			Goal:           req.Query,
			UserID:         req.UserID,
			ConversationID: req.ConversationID,
			SessionID:      req.ConversationID,
			WorkspaceRoot:  a.defaultWorkspaceRoot,
			Metadata: map[string]interface{}{
				"mode":          req.Mode,
				"route_mode":    req.RouteMode,
				"lang":          req.Lang,
				"report_style":  req.ReportStyle,
				"time_windows":  append([]string(nil), req.TimeWindows...),
				"strict_entity": req.StrictEntity != nil && *req.StrictEntity,
				"max_sources":   budgetMaxSources(req.Budget),
				"max_seconds":   budgetMaxSeconds(req.Budget),
			},
		})
		if err != nil {
			return nil, err
		}
		job, err := a.service.GetJobForUser(run.ID, req.UserID, "")
		if err == nil {
			return toToolResearchJob(job), nil
		}
		return &tools.ResearchJob{
			ID:             run.ID,
			ConversationID: run.ConversationID,
			Status:         string(run.Status),
			Query:          run.Goal,
		}, nil
	}
	job, err := a.service.CreateJob(ctx, deepresearch.CreateJobRequest{
		UserID:         req.UserID,
		ConversationID: req.ConversationID,
		Query:          req.Query,
		Mode:           deepresearch.Mode(req.Mode),
		RouteMode:      deepresearch.RouteMode(req.RouteMode),
		Lang:           req.Lang,
		Budget:         toDeepResearchBudget(req.Budget),
		StrictEntity:   req.StrictEntity,
		TimeWindows:    append([]string(nil), req.TimeWindows...),
		ReportStyle:    req.ReportStyle,
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
			"calibration":          cloneCalibrationForTool(job.Report.Calibration),
		}
		if job.Report.Calibration != nil {
			out.Report["takeaway_candidates"] = append([]deepresearch.TakeawayCandidate(nil), job.Report.Calibration.TakeawayCandidates...)
		}
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

func budgetMaxSources(budget *tools.ResearchBudget) int {
	if budget == nil {
		return 0
	}
	return budget.MaxSources
}

func budgetMaxSeconds(budget *tools.ResearchBudget) int {
	if budget == nil {
		return 0
	}
	return budget.MaxSeconds
}
