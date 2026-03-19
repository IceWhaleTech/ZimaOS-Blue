package bootstrap

import (
	"context"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type cronToolAdapter struct {
	resolve func() *cron.Service
}

func (a cronToolAdapter) runtime() *cron.Service {
	if a.resolve == nil {
		return nil
	}
	return a.resolve()
}

func (a cronToolAdapter) Config(_ context.Context) tools.CronConfigInfo {
	runtime := a.runtime()
	if runtime == nil {
		return tools.CronConfigInfo{}
	}
	cfg := runtime.Config()
	return tools.CronConfigInfo{
		Enabled:                 cfg.Enabled,
		MaxConcurrentJobs:       cfg.MaxConcurrentJobs,
		JobTimeoutSeconds:       cfg.JobTimeoutSeconds,
		ExecutionRetentionHours: cfg.ExecutionRetentionHours,
		MaxExecutionsPerJob:     cfg.MaxExecutionsPerJob,
	}
}

func (a cronToolAdapter) Handlers(_ context.Context) []string {
	runtime := a.runtime()
	if runtime == nil {
		return nil
	}
	return runtime.Handlers()
}

func (a cronToolAdapter) ListJobs(_ context.Context) ([]tools.CronJobInfo, error) {
	runtime := a.runtime()
	if runtime == nil {
		return nil, nil
	}
	jobs := runtime.List()
	result := make([]tools.CronJobInfo, 0, len(jobs))
	for _, job := range jobs {
		if job == nil {
			continue
		}
		result = append(result, toCronJobInfo(job))
	}
	return result, nil
}

func (a cronToolAdapter) GetJob(_ context.Context, id string) (*tools.CronJobInfo, error) {
	runtime := a.runtime()
	if runtime == nil {
		return nil, nil
	}
	job, ok := runtime.Get(id)
	if !ok || job == nil {
		return nil, nil
	}
	info := toCronJobInfo(job)
	return &info, nil
}

func (a cronToolAdapter) CreateJob(_ context.Context, name, description, schedule, handler string, payload map[string]interface{}) (*tools.CronJobInfo, error) {
	runtime := a.runtime()
	if runtime == nil {
		return nil, nil
	}
	job, err := runtime.Create(name, description, schedule, handler, payload)
	if err != nil {
		return nil, err
	}
	info := toCronJobInfo(job)
	return &info, nil
}

func (a cronToolAdapter) UpdateJob(_ context.Context, id, name, description, schedule string, payload map[string]interface{}) error {
	runtime := a.runtime()
	if runtime == nil {
		return nil
	}
	return runtime.Update(id, name, description, schedule, payload)
}

func (a cronToolAdapter) DeleteJob(_ context.Context, id string) error {
	runtime := a.runtime()
	if runtime == nil {
		return nil
	}
	return runtime.Delete(id)
}

func (a cronToolAdapter) EnableJob(_ context.Context, id string) error {
	runtime := a.runtime()
	if runtime == nil {
		return nil
	}
	return runtime.Enable(id)
}

func (a cronToolAdapter) DisableJob(_ context.Context, id string) error {
	runtime := a.runtime()
	if runtime == nil {
		return nil
	}
	return runtime.Disable(id)
}

func (a cronToolAdapter) TriggerJob(_ context.Context, id string) error {
	runtime := a.runtime()
	if runtime == nil {
		return nil
	}
	return runtime.Trigger(id)
}

func (a cronToolAdapter) GetExecutions(_ context.Context, id string, limit int) ([]tools.CronExecutionInfo, error) {
	runtime := a.runtime()
	if runtime == nil {
		return nil, nil
	}
	executions, err := runtime.GetExecutions(id, limit)
	if err != nil {
		return nil, err
	}
	result := make([]tools.CronExecutionInfo, 0, len(executions))
	for _, execution := range executions {
		if execution == nil {
			continue
		}
		result = append(result, tools.CronExecutionInfo{
			ID:        execution.ID,
			JobID:     execution.JobID,
			StartedAt: execution.StartedAt,
			EndedAt:   execution.EndedAt,
			Duration:  execution.Duration,
			Status:    execution.Status,
			Error:     execution.Error,
			Result:    execution.Result,
		})
	}
	return result, nil
}

func toCronJobInfo(job *cron.Job) tools.CronJobInfo {
	return tools.CronJobInfo{
		ID:          job.ID,
		Name:        job.Name,
		Description: job.Description,
		Schedule:    job.Schedule,
		Handler:     job.Handler,
		Payload:     job.Payload,
		Status:      string(job.Status),
		Enabled:     job.Enabled,
		CreatedAt:   job.CreatedAt,
		UpdatedAt:   job.UpdatedAt,
		LastRunAt:   job.LastRunAt,
		NextRunAt:   job.NextRunAt,
		RunCount:    job.RunCount,
		FailCount:   job.FailCount,
		Metadata:    job.Metadata,
	}
}
