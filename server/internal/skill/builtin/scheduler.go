package builtin

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// CronServiceInterface defines the interface for cron service used by the scheduler skill.
// This avoids circular imports with the cron package.
type CronServiceInterface interface {
	Create(name, description, schedule, handler string, payload map[string]interface{}) (CronJobInfo, error)
	List() []CronJobInfo
	Get(id string) (CronJobInfo, bool)
	Delete(id string) error
	Trigger(id string) error
	Enable(id string) error
	Disable(id string) error
	GetExecutions(jobID string, limit int) ([]CronJobExecution, error)
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

// CronJobExecution represents a single job execution record.
type CronJobExecution struct {
	ID        string  `json:"id"`
	JobID     string  `json:"job_id"`
	StartedAt string  `json:"started_at"`
	EndedAt   *string `json:"ended_at,omitempty"`
	Duration  string  `json:"duration,omitempty"`
	Status    string  `json:"status"`
	Error     string  `json:"error,omitempty"`
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
					Description: "Action to perform: create, list, get, delete, trigger, enable, disable, executions",
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
				{
					Name:        "limit",
					Type:        "integer",
					Description: "Max number of execution records to return (default 20, for executions action)",
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
					Description: "Created/triggered/retrieved job",
				},
				{
					Name:        "executions",
					Type:        "array",
					Description: "List of job execution records",
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
		"create": true, "list": true, "get": true, "delete": true,
		"trigger": true, "enable": true, "disable": true, "executions": true,
	}
	if !validActions[actionStr] {
		return fmt.Errorf("invalid action: %s (must be create, list, get, delete, trigger, enable, disable, or executions)", actionStr)
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
	case "get", "delete", "trigger", "enable", "disable", "executions":
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
		return s.createJob(ctx, svc, input)
	case "list":
		return s.listJobs(svc)
	case "get":
		return s.getJob(svc, input)
	case "delete":
		return s.deleteJob(svc, input)
	case "trigger":
		return s.triggerJob(svc, input)
	case "enable":
		return s.enableJob(svc, input)
	case "disable":
		return s.disableJob(svc, input)
	case "executions":
		return s.getExecutions(svc, input)
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}

func (s *Scheduler) createJob(ctx context.Context, svc CronServiceInterface, input map[string]any) (*skill.Result, error) {
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
	if userID := strings.TrimSpace(tools.GetUserID(ctx)); userID != "" {
		payload["user_id"] = userID
	}
	if conversationID := strings.TrimSpace(tools.GetSessionID(ctx)); conversationID != "" {
		payload["conversation_id"] = conversationID
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
		"message": fmt.Sprintf("%d scheduled jobs:\n%s\n\nView and manage in the web UI: /cron", len(jobs), strings.Join(lines, "\n")),
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

func (s *Scheduler) getJob(svc CronServiceInterface, input map[string]any) (*skill.Result, error) {
	id := input["id"].(string)

	job, found := svc.Get(id)
	if !found {
		return skill.NewErrorResult(fmt.Errorf("job %s not found", id)), nil
	}

	status := "enabled"
	if !job.Enabled {
		status = "disabled"
	}

	return skill.NewResult(map[string]any{
		"job":     job,
		"message": fmt.Sprintf("Job '%s' (%s): %s [%s] runs:%d fails:%d\n\nView details in the web UI: /cron", job.Name, job.ID, job.Schedule, status, job.RunCount, job.FailCount),
	}), nil
}

func (s *Scheduler) getExecutions(svc CronServiceInterface, input map[string]any) (*skill.Result, error) {
	id := input["id"].(string)

	limit := 20
	if l, ok := input["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	execs, err := svc.GetExecutions(id, limit)
	if err != nil {
		return skill.NewErrorResult(fmt.Errorf("failed to get executions: %w", err)), nil
	}

	if len(execs) == 0 {
		return skill.NewResult(map[string]any{
			"executions": []CronJobExecution{},
			"count":      0,
			"message":    fmt.Sprintf("No executions for job %s", id),
		}), nil
	}

	var lines []string
	for _, e := range execs {
		line := fmt.Sprintf("- %s: %s (%s)", e.ID, e.Status, e.Duration)
		if e.Error != "" {
			line += " error: " + e.Error
		}
		lines = append(lines, line)
	}

	return skill.NewResult(map[string]any{
		"executions": execs,
		"count":      len(execs),
		"message":    fmt.Sprintf("%d executions for job %s:\n%s\n\nView full history in the web UI: /cron", len(execs), id, strings.Join(lines, "\n")),
	}), nil
}
