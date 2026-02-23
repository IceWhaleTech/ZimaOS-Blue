package reminder

import (
	"context"
	"fmt"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
)

// CronAdapter adapts cron.Service to the CronService interface used by reminder.Service.
type CronAdapter struct {
	resolve func() *cron.Service
}

// NewCronAdapter creates a new adapter with lazy resolution.
func NewCronAdapter(resolve func() *cron.Service) *CronAdapter {
	return &CronAdapter{resolve: resolve}
}

func (a *CronAdapter) svc() (*cron.Service, error) {
	s := a.resolve()
	if s == nil {
		return nil, fmt.Errorf("cron service not available")
	}
	return s, nil
}

// RegisterHandler registers a reminder handler that receives payload from the cron Job.
func (a *CronAdapter) RegisterHandler(name string, handler func(ctx context.Context, payload map[string]interface{}) (interface{}, error)) {
	s, err := a.svc()
	if err != nil {
		return
	}
	// Wrap to extract payload from *cron.Job
	s.RegisterHandler(name, func(ctx context.Context, job *cron.Job) (interface{}, error) {
		return handler(ctx, job.Payload)
	})
}

// CreateJob creates a cron job and returns its ID.
func (a *CronAdapter) CreateJob(name, description, schedule, handler string, payload map[string]interface{}) (string, error) {
	s, err := a.svc()
	if err != nil {
		return "", err
	}
	job, err := s.Create(name, description, schedule, handler, payload)
	if err != nil {
		return "", err
	}
	return job.ID, nil
}

// DeleteJob deletes a cron job by ID.
func (a *CronAdapter) DeleteJob(id string) error {
	s, err := a.svc()
	if err != nil {
		return err
	}
	return s.Delete(id)
}
