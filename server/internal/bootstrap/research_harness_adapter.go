package bootstrap

import (
	"context"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
)

type harnessResearchCreator struct {
	manager              researchHarnessRunSubmitter
	service              researchHarnessJobStore
	defaultWorkspaceRoot string
}

func newHarnessResearchCreator(manager researchHarnessRunSubmitter, service researchHarnessJobStore, defaultWorkspaceRoot string) *harnessResearchCreator {
	manager = normalizeResearchHarnessSubmitter(manager)
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
	job, _, err := submitResearchHarnessJob(ctx, a.manager, a.service, req, a.defaultWorkspaceRoot)
	return job, err
}
