package bootstrap

import (
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type researchToolAdapterLocalStore struct {
	mu   sync.RWMutex
	jobs map[string]researchToolAdapterLocalJob
}

type researchToolAdapterLocalJob struct {
	owner string
	job   *tools.ResearchJob
}

func newResearchToolAdapterLocalStore() *researchToolAdapterLocalStore {
	return &researchToolAdapterLocalStore{jobs: map[string]researchToolAdapterLocalJob{}}
}

func (a *deepResearchToolAdapter) localJob(id, userID string) (*tools.ResearchJob, bool) {
	if a == nil || a.local == nil {
		return nil, false
	}
	return a.local.get(id, userID)
}

func (a *deepResearchToolAdapter) storeLocalJob(userID string, job *tools.ResearchJob) {
	if a == nil || a.local == nil || job == nil {
		return
	}
	a.local.put(userID, job)
}

func (s *researchToolAdapterLocalStore) get(id, userID string) (*tools.ResearchJob, bool) {
	if s == nil || strings.TrimSpace(id) == "" {
		return nil, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.jobs[id]
	if !ok || entry.job == nil {
		return nil, false
	}
	if strings.TrimSpace(entry.owner) != "" && strings.TrimSpace(userID) != strings.TrimSpace(entry.owner) {
		return nil, false
	}
	return cloneToolResearchJob(entry.job), true
}

func (s *researchToolAdapterLocalStore) put(userID string, job *tools.ResearchJob) {
	if s == nil || job == nil || strings.TrimSpace(job.ID) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = researchToolAdapterLocalJob{
		owner: strings.TrimSpace(userID),
		job:   cloneToolResearchJob(job),
	}
}

func cloneToolResearchJob(job *tools.ResearchJob) *tools.ResearchJob {
	if job == nil {
		return nil
	}
	cp := *job
	if job.Report != nil {
		cp.Report = cloneResearchToolMap(job.Report)
	}
	return &cp
}
