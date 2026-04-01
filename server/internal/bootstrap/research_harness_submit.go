package bootstrap

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
)

type researchHarnessRunSubmitter interface {
	Submit(ctx context.Context, spec harness.RunSpec) (*harness.Run, error)
}

type researchHarnessJobStore interface {
	GetJobForUser(id, userID, tenantID string) (*deepresearch.Job, error)
}

func submitResearchHarnessJob(
	ctx context.Context,
	submitter researchHarnessRunSubmitter,
	service researchHarnessJobStore,
	req deepresearch.CreateJobRequest,
	workspaceRoot string,
) (*deepresearch.Job, *harness.Run, error) {
	submitter = normalizeResearchHarnessSubmitter(submitter)
	if submitter == nil {
		return nil, nil, fmt.Errorf("research harness submitter is required")
	}
	run, err := submitter.Submit(ctx, newResearchHarnessRunSpec(researchHarnessRunInputFromJobRequest(req, workspaceRoot)))
	if err != nil {
		return nil, nil, err
	}
	return loadSubmittedResearchJob(run, service, req.UserID, req.TenantID), run, nil
}

func researchHarnessRunInputFromJobRequest(req deepresearch.CreateJobRequest, workspaceRoot string) researchHarnessRunInput {
	return researchHarnessRunInput{
		Query:          req.Query,
		UserID:         req.UserID,
		ConversationID: req.ConversationID,
		WorkspaceRoot:  strings.TrimSpace(workspaceRoot),
		Mode:           string(req.Mode),
		RouteMode:      string(req.RouteMode),
		Lang:           req.Lang,
		ReportStyle:    req.ReportStyle,
		TimeWindows:    append([]string(nil), req.TimeWindows...),
		StrictEntity:   req.StrictEntity != nil && *req.StrictEntity,
		MaxSources:     deepResearchBudgetMaxSources(req.Budget),
		MaxSeconds:     deepResearchBudgetMaxSeconds(req.Budget),
	}
}

func loadSubmittedResearchJob(run *harness.Run, service researchHarnessJobStore, userID, tenantID string) *deepresearch.Job {
	if service != nil {
		job, err := service.GetJobForUser(run.ID, strings.TrimSpace(userID), strings.TrimSpace(tenantID))
		if err == nil {
			return job
		}
	}
	return synthesizeResearchJobFromRun(run)
}

func normalizeResearchHarnessSubmitter(submitter researchHarnessRunSubmitter) researchHarnessRunSubmitter {
	if submitter == nil {
		return nil
	}
	value := reflect.ValueOf(submitter)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if value.IsNil() {
			return nil
		}
	}
	return submitter
}

func synthesizeResearchJobFromRun(run *harness.Run) *deepresearch.Job {
	if run == nil {
		return nil
	}
	return &deepresearch.Job{
		ID:             run.ID,
		ConversationID: run.ConversationID,
		UserID:         run.UserID,
		Query:          run.Goal,
		Status:         deepresearch.JobStatus(run.Status),
	}
}

func deepResearchBudgetMaxSources(budget *deepresearch.Budget) int {
	if budget == nil {
		return 0
	}
	return budget.MaxSources
}

func deepResearchBudgetMaxSeconds(budget *deepresearch.Budget) int {
	if budget == nil {
		return 0
	}
	return budget.MaxSeconds
}
