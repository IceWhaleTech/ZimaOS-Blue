package bootstrap

import (
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type deepResearchToolAdapter struct {
	service              *deepresearch.Service
	recent               tools.Tool
	jobStore             researchHarnessJobStore
	manager              researchHarnessRunSubmitter
	local                *researchToolAdapterLocalStore
	defaultWorkspaceRoot string
}

func newDeepResearchToolAdapter(service *deepresearch.Service, manager researchHarnessRunSubmitter, defaultWorkspaceRoot string) *deepResearchToolAdapter {
	return &deepResearchToolAdapter{service: service, jobStore: service, manager: normalizeResearchHarnessSubmitter(manager), local: newResearchToolAdapterLocalStore(), defaultWorkspaceRoot: strings.TrimSpace(defaultWorkspaceRoot)}
}

func (a *deepResearchToolAdapter) GetJobForUser(id, userID string) (*tools.ResearchJob, error) {
	if job, ok := a.localJob(id, userID); ok {
		return job, nil
	}
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
