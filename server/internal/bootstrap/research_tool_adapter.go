package bootstrap

import (
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type deepResearchToolAdapter struct {
	service              *deepresearch.Service
	manager              researchHarnessRunSubmitter
	defaultWorkspaceRoot string
}

func newDeepResearchToolAdapter(service *deepresearch.Service, manager researchHarnessRunSubmitter, defaultWorkspaceRoot string) *deepResearchToolAdapter {
	return &deepResearchToolAdapter{
		service:              service,
		manager:              normalizeResearchHarnessSubmitter(manager),
		defaultWorkspaceRoot: strings.TrimSpace(defaultWorkspaceRoot),
	}
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
