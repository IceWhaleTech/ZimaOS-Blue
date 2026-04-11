package bootstrap

import (
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type deepResearchToolAdapter struct {
	service *deepresearch.Service
	jobStore researchHarnessJobStore
	manager researchHarnessRunSubmitter
	defaultWorkspaceRoot string
}

func newDeepResearchToolAdapter(service *deepresearch.Service, manager researchHarnessRunSubmitter, defaultWorkspaceRoot string) *deepResearchToolAdapter {
	return &deepResearchToolAdapter{service: service, jobStore: service, manager: normalizeResearchHarnessSubmitter(manager), defaultWorkspaceRoot: strings.TrimSpace(defaultWorkspaceRoot)}
}

func (a *deepResearchToolAdapter) GetJobForUser(id, userID string) (*tools.ResearchJob, error) {
	store := a.researchJobStore()
	if store == nil {
		return nil, deepresearch.ErrJobNotFound
	}
	job, err := store.GetJobForUser(id, userID, "")
	if err != nil {
		return nil, err
	}
	return toToolResearchJob(job), nil
}
