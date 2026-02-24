package sockipc

import (
	"context"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
)

// CronIPCAdapter adapts builtin.CronServiceInterface to the sockipc.CronBackend interface.
type CronIPCAdapter struct {
	svc builtin.CronServiceInterface
}

// NewCronIPCAdapter creates a new adapter.
func NewCronIPCAdapter(svc builtin.CronServiceInterface) *CronIPCAdapter {
	return &CronIPCAdapter{svc: svc}
}

func (a *CronIPCAdapter) Create(_ context.Context, name, description, schedule, command string) (CronResult, error) {
	payload := map[string]interface{}{"command": command}
	job, err := a.svc.Create(name, description, schedule, "command", payload)
	if err != nil {
		return CronResult{}, err
	}
	return toCronResult(job), nil
}

func (a *CronIPCAdapter) List(_ context.Context) ([]CronResult, error) {
	jobs := a.svc.List()
	results := make([]CronResult, len(jobs))
	for i, j := range jobs {
		results[i] = toCronResult(j)
	}
	return results, nil
}

func (a *CronIPCAdapter) Delete(_ context.Context, id string) error {
	return a.svc.Delete(id)
}

func (a *CronIPCAdapter) Trigger(_ context.Context, id string) error {
	return a.svc.Trigger(id)
}

func (a *CronIPCAdapter) Enable(_ context.Context, id string) error {
	return a.svc.Enable(id)
}

func (a *CronIPCAdapter) Disable(_ context.Context, id string) error {
	return a.svc.Disable(id)
}

func toCronResult(j builtin.CronJobInfo) CronResult {
	return CronResult{
		ID:          j.ID,
		Name:        j.Name,
		Description: j.Description,
		Schedule:    j.Schedule,
		Handler:     j.Handler,
		Enabled:     j.Enabled,
		Status:      j.Status,
		RunCount:    j.RunCount,
		FailCount:   j.FailCount,
	}
}
