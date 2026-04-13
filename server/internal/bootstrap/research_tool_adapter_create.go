package bootstrap

import (
	"context"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func (a *deepResearchToolAdapter) CreateJob(ctx context.Context, req tools.ResearchCreateJobRequest) (*tools.ResearchJob, error) {
	if a == nil || a.service == nil {
		return nil, deepresearch.ErrJobNotFound
	}
	if a.canRunLocalRecent(req) {
		return a.createLocalRecentJob(ctx, req)
	}
	if a.manager != nil {
		store := a.researchJobStore()
		job, run, err := submitCanonicalResearchHarnessJob(ctx, a.manager, store, req, a.defaultWorkspaceRoot) // submitCanonicalResearchHarnessJob(ctx, a.manager, a.service, ...)
		if err != nil {
			return nil, err
		}
		return toolResearchJobWithRunMetadata(toToolResearchJob(job), run), nil
	}
	job, err := a.service.CreateJob(ctx, toDeepResearchCreateJobRequest(req))
	if err != nil {
		return nil, err
	}
	return toToolResearchJob(job), nil
}

func toDeepResearchCreateJobRequest(req tools.ResearchCreateJobRequest) deepresearch.CreateJobRequest {
	return deepresearch.CreateJobRequest{
		UserID:         req.UserID,
		ConversationID: req.ConversationID,
		Query:          req.Query,
		Mode:           deepresearch.Mode(toolResearchRequestMode(req)),
		RouteMode:      deepresearch.RouteMode(req.RouteMode),
		Lang:           req.Lang,
		Budget:         toDeepResearchBudget(req.Budget),
		StrictEntity:   req.StrictEntity,
		TimeWindows:    append([]string(nil), req.TimeWindows...),
		ReportStyle:    req.ReportStyle,
	}
}
