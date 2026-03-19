package bootstrap

import (
	"context"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
)

type harnessResearchCreator struct {
	manager              *harness.Controller
	service              *deepresearch.Service
	defaultWorkspaceRoot string
}

func newHarnessResearchCreator(manager *harness.Controller, service *deepresearch.Service, defaultWorkspaceRoot string) *harnessResearchCreator {
	if manager == nil || service == nil {
		return nil
	}
	return &harnessResearchCreator{
		manager:              manager,
		service:              service,
		defaultWorkspaceRoot: strings.TrimSpace(defaultWorkspaceRoot),
	}
}

func (a *harnessResearchCreator) CreateJob(ctx context.Context, req deepresearch.CreateJobRequest) (*deepresearch.Job, error) {
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
			"max_sources":   researchBudgetSources(req.Budget),
			"max_seconds":   researchBudgetSeconds(req.Budget),
		},
	})
	if err != nil {
		return nil, err
	}
	job, err := a.service.GetJobForUser(run.ID, req.UserID, req.TenantID)
	if err == nil {
		return job, nil
	}
	return &deepresearch.Job{
		ID:             run.ID,
		ConversationID: run.ConversationID,
		UserID:         run.UserID,
		Query:          run.Goal,
		Status:         deepresearch.JobStatus(run.Status),
	}, nil
}

func researchBudgetSources(budget *deepresearch.Budget) int {
	if budget == nil {
		return 0
	}
	return budget.MaxSources
}

func researchBudgetSeconds(budget *deepresearch.Budget) int {
	if budget == nil {
		return 0
	}
	return budget.MaxSeconds
}
