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
	if a.manager != nil {
		job, _, err := submitCanonicalResearchHarnessJob(ctx, a.manager, a.service, req, a.defaultWorkspaceRoot)
		if err != nil {
			return nil, err
		}
		return toToolResearchJob(job), nil
	}
	job, err := a.service.CreateJob(ctx, toDeepResearchCreateJobRequest(req))
	if err != nil {
		return nil, err
	}
	return toToolResearchJob(job), nil
}

func toDeepResearchCreateJobRequest(req tools.ResearchCreateJobRequest) deepresearch.CreateJobRequest {
	mode := req.ResearchDepth
	if mode == "" {
		mode = req.Mode
	}
	return deepresearch.CreateJobRequest{
		UserID:         req.UserID,
		ConversationID: req.ConversationID,
		Query:          req.Query,
		Mode:           deepresearch.Mode(mode),
		RouteMode:      deepresearch.RouteMode(req.RouteMode),
		Lang:           req.Lang,
		Budget:         toDeepResearchBudget(req.Budget),
		StrictEntity:   req.StrictEntity,
		TimeWindows:    append([]string(nil), req.TimeWindows...),
		ReportStyle:    req.ReportStyle,
	}
}
