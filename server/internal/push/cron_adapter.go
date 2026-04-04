package push

import (
	"context"
	"fmt"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
)

// CronAdapter adapts cron.Service to the CronService interface used by push.Service.
type CronAdapter struct {
	resolve func() *cron.Service

	mu       sync.Mutex
	bound    *cron.Service
	handlers []registeredHandler
}

type registeredHandler struct {
	name    string
	handler func(ctx context.Context, payload map[string]interface{}) (interface{}, error)
}

// NewCronAdapter creates a new adapter with lazy resolution.
func NewCronAdapter(resolve func() *cron.Service) *CronAdapter {
	return &CronAdapter{resolve: resolve}
}

func (a *CronAdapter) svc() (*cron.Service, error) {
	if a == nil || a.resolve == nil {
		return nil, fmt.Errorf("cron service not available")
	}
	s := a.resolve()
	if s == nil {
		return nil, fmt.Errorf("cron service not available")
	}
	a.bindHandlers(s)
	return s, nil
}

// RegisterHandler registers a push handler that receives payload from the cron Job.
func (a *CronAdapter) RegisterHandler(name string, handler func(ctx context.Context, payload map[string]interface{}) (interface{}, error)) {
	if a == nil || handler == nil {
		return
	}

	var bound *cron.Service
	a.mu.Lock()
	a.handlers = append(a.handlers, registeredHandler{name: name, handler: handler})
	bound = a.bound
	a.mu.Unlock()

	// If the cron service is already alive, register immediately.
	if bound != nil {
		bound.RegisterHandler(name, wrapPayloadHandler(handler))
	}
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

func (a *CronAdapter) bindHandlers(s *cron.Service) {
	if s == nil {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.bound == s {
		return
	}
	for _, registered := range a.handlers {
		s.RegisterHandler(registered.name, wrapPayloadHandler(registered.handler))
	}
	a.bound = s
}

func wrapPayloadHandler(handler func(ctx context.Context, payload map[string]interface{}) (interface{}, error)) func(context.Context, *cron.Job) (interface{}, error) {
	return func(ctx context.Context, job *cron.Job) (interface{}, error) {
		return handler(ctx, job.Payload)
	}
}
