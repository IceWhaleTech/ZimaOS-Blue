package cron

import (
	"fmt"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
)

// SkillAdapter adapts the cron Service to builtin.CronServiceInterface.
// It supports lazy initialization — the service is resolved on first use.
type SkillAdapter struct {
	resolve func() *Service
}

// NewSkillAdapter creates a new adapter that resolves the service lazily.
// The resolve function should trigger lazy init if needed (e.g. Handler.GetService()).
func NewSkillAdapter(resolve func() *Service) *SkillAdapter {
	return &SkillAdapter{resolve: resolve}
}

func (a *SkillAdapter) svc() (*Service, error) {
	s := a.resolve()
	if s == nil {
		return nil, fmt.Errorf("cron service not available")
	}
	return s, nil
}

func jobToInfo(j *Job) builtin.CronJobInfo {
	return builtin.CronJobInfo{
		ID:          j.ID,
		Name:        j.Name,
		Description: j.Description,
		Schedule:    j.Schedule,
		Handler:     j.Handler,
		Enabled:     j.Enabled,
		Status:      string(j.Status),
		RunCount:    j.RunCount,
		FailCount:   j.FailCount,
	}
}

func (a *SkillAdapter) Create(name, description, schedule, handler string, payload map[string]interface{}) (builtin.CronJobInfo, error) {
	s, err := a.svc()
	if err != nil {
		return builtin.CronJobInfo{}, err
	}
	job, err := s.Create(name, description, schedule, handler, payload)
	if err != nil {
		return builtin.CronJobInfo{}, err
	}
	return jobToInfo(job), nil
}

func (a *SkillAdapter) List() []builtin.CronJobInfo {
	s, err := a.svc()
	if err != nil {
		return nil
	}
	jobs := s.List()
	results := make([]builtin.CronJobInfo, len(jobs))
	for i, j := range jobs {
		results[i] = jobToInfo(j)
	}
	return results
}

func (a *SkillAdapter) Delete(id string) error {
	s, err := a.svc()
	if err != nil {
		return err
	}
	return s.Delete(id)
}

func (a *SkillAdapter) Trigger(id string) error {
	s, err := a.svc()
	if err != nil {
		return err
	}
	return s.Trigger(id)
}

func (a *SkillAdapter) Enable(id string) error {
	s, err := a.svc()
	if err != nil {
		return err
	}
	return s.Enable(id)
}

func (a *SkillAdapter) Disable(id string) error {
	s, err := a.svc()
	if err != nil {
		return err
	}
	return s.Disable(id)
}
