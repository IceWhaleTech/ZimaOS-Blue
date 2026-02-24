package builtin

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

// CronServiceInterface defines the interface for cron service used by the scheduler skill.
// This avoids circular imports with the cron package.
type CronServiceInterface interface {
	Create(name, description, schedule, handler string, payload map[string]interface{}) (CronJobInfo, error)
	List() []CronJobInfo
	Delete(id string) error
	Trigger(id string) error
	Enable(id string) error
	Disable(id string) error
}

// CronJobInfo represents a cron job returned by the interface.
type CronJobInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Schedule    string `json:"schedule"`
	Handler     string `json:"handler"`
	Enabled     bool   `json:"enabled"`
	Status      string `json:"status"`
	RunCount    int64  `json:"run_count"`
	FailCount   int64  `json:"fail_count"`
}

// Scheduler is a built-in skill for managing scheduled/cron jobs.
// It wraps the cron service to let the LLM create, list, delete, and trigger
// scheduled command executions.
type Scheduler struct {
	manifest *skill.Manifest
	mu       sync.RWMutex
	svc      CronServiceInterface
}

// NewScheduler creates a new scheduler skill.
func NewScheduler() *Scheduler {
	return &Scheduler{
		manifest: &skill.Manifest{
			ID:          "scheduler",
			Name:        "Scheduler",
			Version:     "1.0.0",
			Description: "Create, list, delete, and trigger scheduled tasks (cron jobs). Supports shell commands executed on a cron schedule.",
			Category:    "system",
			Icon:        "schedule",
			Tags:        []string{"cron", "schedule", "timer", "automation", "task"},
			Inputs: []skill.Parameter{
				{
					Name:        "action",
					Type:        "string",
					Description: "Action to perform: create, list, delete, trigger, enable, disable",
					Required:    true,
				},
				{
					Name:        "name",
					Type:        "string",
					Description: "Job name (required for create)",
					Required:    false,
				},
				{
					Name:        "description",
					Type:        "string",
					Description: "Job description (optional for create)",
					Required:    false,
				},
				{
					Name:        "schedule",
					Type:        "string",
					Description: "Cron expression: '*/5 * * * *' (every 5 min), '0 9 * * *' (daily 9am), '0 0 * * 1' (weekly Monday). Standard 5-field format.",
					Required:    false,
				},
				{
					Name:        "command",
					Type:        "string",
					Description: "Shell command to execute on schedule (required for create). Allowed: echo, date, uptime, df, free, ps, curl, wget.",
					Required:    false,
				},
				{
					Name:        "id",
					Type:        "string",
					Description: "Job ID (required for delete, trigger, enable, disable)",
					Required:    false,
				},
				{
					Name:        "locale",
					Type:        "string",
					Description: "Language/locale code for localized responses (e.g., en-US, zh-CN)",
					Required:    false,
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "jobs",
					Type:        "array",
					Description: "List of scheduled jobs",
				},
				{
					Name:        "job",
					Type:        "object",
					Description: "Created/triggered job",
				},
			},
		},
	}
}

// SetCronService injects the cron service (called after lazy init).
func (s *Scheduler) SetCronService(svc CronServiceInterface) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.svc = svc
}

// Manifest returns the skill manifest.
func (s *Scheduler) Manifest() *skill.Manifest {
	return s.manifest
}

// Validate validates the input parameters.
func (s *Scheduler) Validate(input map[string]any) error {
	action, ok := input["action"]
	if !ok {
		return fmt.Errorf("action is required")
	}

	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("action must be a string")
	}

	validActions := map[string]bool{
		"create": true, "list": true, "delete": true,
		"trigger": true, "enable": true, "disable": true,
	}
	if !validActions[actionStr] {
		return fmt.Errorf("invalid action: %s (must be create, list, delete, trigger, enable, or disable)", actionStr)
	}

	switch actionStr {
	case "create":
		if _, ok := input["name"]; !ok {
			return fmt.Errorf("name is required for create action")
		}
		if _, ok := input["schedule"]; !ok {
			return fmt.Errorf("schedule is required for create action")
		}
		if _, ok := input["command"]; !ok {
			return fmt.Errorf("command is required for create action")
		}
	case "delete", "trigger", "enable", "disable":
		if _, ok := input["id"]; !ok {
			return fmt.Errorf("id is required for %s action", actionStr)
		}
	}

	return nil
}

// Execute executes the scheduler skill.
func (s *Scheduler) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	s.mu.RLock()
	svc := s.svc
	s.mu.RUnlock()

	if svc == nil {
		return skill.NewErrorResult(fmt.Errorf("scheduler service not available")), nil
	}

	action := input["action"].(string)

	switch action {
	case "create":
		return s.createJob(svc, input)
	case "list":
		return s.listJobs(svc)
	case "delete":
		return s.deleteJob(svc, input)
	case "trigger":
		return s.triggerJob(svc, input)
	case "enable":
		return s.enableJob(svc, input)
	case "disable":
		return s.disableJob(svc, input)
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}

func (s *Scheduler) createJob(svc CronServiceInterface, input map[string]any) (*skill.Result, error) {
	name := input["name"].(string)
	schedule := input["schedule"].(string)
	command := input["command"].(string)

	var description string
	if d, ok := input["description"].(string); ok {
		description = d
	}

	payload := map[string]interface{}{
		"command": command,
	}

	job, err := svc.Create(name, description, schedule, "command", payload)
	if err != nil {
		return skill.NewErrorResult(fmt.Errorf("failed to create job: %w", err)), nil
	}

	return skill.NewResult(map[string]any{
		"job":     job,
		"message": fmt.Sprintf("Scheduled job '%s' created: %s → %s", name, schedule, command),
	}), nil
}

func (s *Scheduler) listJobs(svc CronServiceInterface) (*skill.Result, error) {
	jobs := svc.List()

	if len(jobs) == 0 {
		return skill.NewResult(map[string]any{
			"jobs":    []CronJobInfo{},
			"count":   0,
			"message": "No scheduled jobs",
		}), nil
	}

	// Build human-readable summary
	var lines []string
	for _, j := range jobs {
		status := "enabled"
		if !j.Enabled {
			status = "disabled"
		}
		lines = append(lines, fmt.Sprintf("- %s (%s): %s [%s] runs:%d fails:%d",
			j.Name, j.ID, j.Schedule, status, j.RunCount, j.FailCount))
	}

	return skill.NewResult(map[string]any{
		"jobs":    jobs,
		"count":   len(jobs),
		"message": fmt.Sprintf("%d scheduled jobs:\n%s", len(jobs), strings.Join(lines, "\n")),
	}), nil
}

func (s *Scheduler) deleteJob(svc CronServiceInterface, input map[string]any) (*skill.Result, error) {
	id := input["id"].(string)

	if err := svc.Delete(id); err != nil {
		return skill.NewErrorResult(fmt.Errorf("failed to delete job: %w", err)), nil
	}

	return skill.NewResult(map[string]any{
		"id":      id,
		"deleted": true,
		"message": fmt.Sprintf("Job %s deleted", id),
	}), nil
}

func (s *Scheduler) triggerJob(svc CronServiceInterface, input map[string]any) (*skill.Result, error) {
	id := input["id"].(string)

	if err := svc.Trigger(id); err != nil {
		return skill.NewErrorResult(fmt.Errorf("failed to trigger job: %w", err)), nil
	}

	return skill.NewResult(map[string]any{
		"id":        id,
		"triggered": true,
		"message":   fmt.Sprintf("Job %s triggered", id),
	}), nil
}

func (s *Scheduler) enableJob(svc CronServiceInterface, input map[string]any) (*skill.Result, error) {
	id := input["id"].(string)

	if err := svc.Enable(id); err != nil {
		return skill.NewErrorResult(fmt.Errorf("failed to enable job: %w", err)), nil
	}

	return skill.NewResult(map[string]any{
		"id":      id,
		"enabled": true,
		"message": fmt.Sprintf("Job %s enabled", id),
	}), nil
}

func (s *Scheduler) disableJob(svc CronServiceInterface, input map[string]any) (*skill.Result, error) {
	id := input["id"].(string)

	if err := svc.Disable(id); err != nil {
		return skill.NewErrorResult(fmt.Errorf("failed to disable job: %w", err)), nil
	}

	return skill.NewResult(map[string]any{
		"id":       id,
		"disabled": true,
		"message":  fmt.Sprintf("Job %s disabled", id),
	}), nil
}
