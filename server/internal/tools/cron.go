package tools

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
)

// CronConfigInfo exposes scheduler runtime limits and retention.
type CronConfigInfo struct {
	Enabled                 bool `json:"enabled"`
	MaxConcurrentJobs       int  `json:"max_concurrent_jobs"`
	JobTimeoutSeconds       int  `json:"job_timeout_seconds"`
	ExecutionRetentionHours int  `json:"execution_retention_hours"`
	MaxExecutionsPerJob     int  `json:"max_executions_per_job"`
}

// CronJobInfo is the normalized job view exposed by the cron tool.
type CronJobInfo struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Schedule    string                 `json:"schedule"`
	Handler     string                 `json:"handler"`
	Payload     map[string]interface{} `json:"payload,omitempty"`
	Status      string                 `json:"status"`
	Enabled     bool                   `json:"enabled"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	LastRunAt   *time.Time             `json:"last_run_at,omitempty"`
	NextRunAt   *time.Time             `json:"next_run_at,omitempty"`
	RunCount    int64                  `json:"run_count"`
	FailCount   int64                  `json:"fail_count"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// CronExecutionInfo is the normalized job execution view.
type CronExecutionInfo struct {
	ID        string        `json:"id"`
	JobID     string        `json:"job_id"`
	StartedAt time.Time     `json:"started_at"`
	EndedAt   *time.Time    `json:"ended_at,omitempty"`
	Duration  time.Duration `json:"duration,omitempty"`
	Status    string        `json:"status"`
	Error     string        `json:"error,omitempty"`
	Result    interface{}   `json:"result,omitempty"`
}

// CronService provides stable access to scheduler capabilities.
type CronService interface {
	Config(ctx context.Context) CronConfigInfo
	Handlers(ctx context.Context) []string
	ListJobs(ctx context.Context) ([]CronJobInfo, error)
	GetJob(ctx context.Context, id string) (*CronJobInfo, error)
	CreateJob(ctx context.Context, name, description, schedule, handler string, payload map[string]interface{}) (*CronJobInfo, error)
	UpdateJob(ctx context.Context, id, name, description, schedule string, payload map[string]interface{}) error
	DeleteJob(ctx context.Context, id string) error
	EnableJob(ctx context.Context, id string) error
	DisableJob(ctx context.Context, id string) error
	TriggerJob(ctx context.Context, id string) error
	GetExecutions(ctx context.Context, id string, limit int) ([]CronExecutionInfo, error)
}

// CronTool manages scheduled jobs.
type CronTool struct {
	service CronService
}

// NewCronTool creates a native cron tool.
func NewCronTool(service CronService) *CronTool {
	return &CronTool{service: service}
}

// Definition returns the tool schema.
func (t *CronTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "cron",
		Description: "Create, inspect, trigger, and manage scheduled jobs.",
		Icon:        "cron",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"list", "get", "create", "update", "delete", "enable", "disable", "trigger", "executions", "handlers"},
					"description": "Cron action. Defaults to list, or get/create based on arguments.",
				},
				"id":          map[string]interface{}{"type": "string", "description": "Job ID for get/update/delete/enable/disable/trigger/executions."},
				"name":        map[string]interface{}{"type": "string", "description": "Job name."},
				"description": map[string]interface{}{"type": "string", "description": "Job description."},
				"schedule":    map[string]interface{}{"type": "string", "description": "Cron schedule expression."},
				"handler":     map[string]interface{}{"type": "string", "description": "Registered cron handler name for create."},
				"payload":     map[string]interface{}{"type": "object", "description": "Handler payload."},
				"limit":       map[string]interface{}{"type": "integer", "description": "Limit for execution history (default 20, max 100)."},
			},
		},
	}
}

// Execute dispatches the requested cron action.
func (t *CronTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.service == nil {
		return nil, errors.New("cron service not available")
	}
	switch cronAction(args) {
	case "list":
		return t.executeList(ctx)
	case "get":
		return t.executeGet(ctx, args)
	case "create":
		return t.executeCreate(ctx, args)
	case "update":
		return t.executeUpdate(ctx, args)
	case "delete":
		return t.executeDelete(ctx, args)
	case "enable":
		return t.executeEnable(ctx, args)
	case "disable":
		return t.executeDisable(ctx, args)
	case "trigger":
		return t.executeTrigger(ctx, args)
	case "executions":
		return t.executeExecutions(ctx, args)
	case "handlers":
		return t.executeHandlers(ctx)
	default:
		return nil, errors.New("unsupported cron action")
	}
}

func (t *CronTool) executeList(ctx context.Context) (interface{}, error) {
	jobs, err := t.service.ListJobs(ctx)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
	})
	return map[string]interface{}{
		"jobs":   jobs,
		"count":  len(jobs),
		"config": t.service.Config(ctx),
	}, nil
}

func (t *CronTool) executeGet(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id := firstCompatString(args, "id", "job_id", "jobId")
	if id == "" {
		return nil, errors.New("id is required")
	}
	job, err := t.service.GetJob(ctx, id)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, errors.New("job not found")
	}
	return map[string]interface{}{"job": job}, nil
}

func (t *CronTool) executeCreate(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	name := firstCompatString(args, "name", "title")
	schedule := firstCompatString(args, "schedule", "cron")
	handler := firstCompatString(args, "handler", "type")
	if schedule == "" || handler == "" {
		return nil, errors.New("schedule and handler are required")
	}
	payload := enrichCronPayloadWithContext(ctx, asMap(firstCompatRawValue(args, "payload")))
	job, err := t.service.CreateJob(ctx, name, firstCompatString(args, "description"), schedule, handler, payload)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"job":     job,
		"created": true,
		"id":      job.ID,
	}, nil
}

func (t *CronTool) executeUpdate(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id := firstCompatString(args, "id", "job_id", "jobId")
	if id == "" {
		return nil, errors.New("id is required")
	}
	job, err := t.service.GetJob(ctx, id)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, errors.New("job not found")
	}
	name := firstCompatString(args, "name", "title")
	if name == "" {
		name = job.Name
	}
	description := firstCompatString(args, "description")
	if description == "" {
		description = job.Description
	}
	schedule := firstCompatString(args, "schedule", "cron")
	if schedule == "" {
		schedule = job.Schedule
	}
	payload := asMap(firstCompatRawValue(args, "payload"))
	if payload == nil {
		payload = job.Payload
	}
	payload = enrichCronPayloadWithContext(ctx, payload)
	if err := t.service.UpdateJob(ctx, id, name, description, schedule, payload); err != nil {
		return nil, err
	}
	updated, err := t.service.GetJob(ctx, id)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"job":     updated,
		"updated": true,
		"id":      id,
	}, nil
}

func (t *CronTool) executeDelete(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id := firstCompatString(args, "id", "job_id", "jobId")
	if id == "" {
		return nil, errors.New("id is required")
	}
	if err := t.service.DeleteJob(ctx, id); err != nil {
		return nil, err
	}
	return map[string]interface{}{"deleted": true, "id": id}, nil
}

func (t *CronTool) executeEnable(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id := firstCompatString(args, "id", "job_id", "jobId")
	if id == "" {
		return nil, errors.New("id is required")
	}
	if err := t.service.EnableJob(ctx, id); err != nil {
		return nil, err
	}
	job, err := t.service.GetJob(ctx, id)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"enabled": true, "id": id, "job": job}, nil
}

func (t *CronTool) executeDisable(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id := firstCompatString(args, "id", "job_id", "jobId")
	if id == "" {
		return nil, errors.New("id is required")
	}
	if err := t.service.DisableJob(ctx, id); err != nil {
		return nil, err
	}
	job, err := t.service.GetJob(ctx, id)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"disabled": true, "id": id, "job": job}, nil
}

func (t *CronTool) executeTrigger(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id := firstCompatString(args, "id", "job_id", "jobId")
	if id == "" {
		return nil, errors.New("id is required")
	}
	if err := t.service.TriggerJob(ctx, id); err != nil {
		return nil, err
	}
	job, err := t.service.GetJob(ctx, id)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"triggered": true, "id": id, "job": job}, nil
}

func (t *CronTool) executeExecutions(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id := firstCompatString(args, "id", "job_id", "jobId")
	if id == "" {
		return nil, errors.New("id is required")
	}
	limit, err := fsAsInt(args, "limit", 20)
	if err != nil {
		return nil, err
	}
	limit = fsClamp(limit, 1, 100)
	executions, err := t.service.GetExecutions(ctx, id, limit)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"job_id":     id,
		"executions": executions,
		"count":      len(executions),
	}, nil
}

func (t *CronTool) executeHandlers(ctx context.Context) (interface{}, error) {
	handlers := t.service.Handlers(ctx)
	return map[string]interface{}{
		"handlers": handlers,
		"count":    len(handlers),
		"config":   t.service.Config(ctx),
	}, nil
}

func cronAction(args map[string]interface{}) string {
	action := strings.ToLower(strings.TrimSpace(firstCompatString(args, "action", "op", "operation", "command")))
	switch action {
	case "", "list", "ls", "status":
		if firstCompatString(args, "id", "job_id", "jobId") != "" {
			return "get"
		}
		if firstCompatString(args, "name", "title") != "" && firstCompatString(args, "schedule", "cron") != "" && firstCompatString(args, "handler", "type") != "" {
			return "create"
		}
		return "list"
	case "get", "show", "read":
		return "get"
	case "create", "add":
		return "create"
	case "update", "edit", "patch":
		return "update"
	case "delete", "remove", "rm":
		return "delete"
	case "enable":
		return "enable"
	case "disable", "pause":
		return "disable"
	case "trigger", "run", "execute":
		return "trigger"
	case "executions", "history":
		return "executions"
	case "handlers":
		return "handlers"
	default:
		return action
	}
}

func asMap(value interface{}) map[string]interface{} {
	if value == nil {
		return nil
	}
	if typed, ok := value.(map[string]interface{}); ok {
		return typed
	}
	return nil
}

// RegisterCronTool registers the native cron tool.
func RegisterCronTool(registry *Registry, service CronService) {
	if registry == nil || service == nil {
		return
	}
	registry.Register(NewCronTool(service))
}

func enrichCronPayloadWithContext(ctx context.Context, payload map[string]interface{}) map[string]interface{} {
	if payload == nil {
		payload = map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(payload)+2)
	for key, value := range payload {
		out[key] = value
	}
	if strings.TrimSpace(firstCompatString(out, "user_id", "userId", "owner_id", "ownerId", "notify_user_id", "notifyUserId")) == "" {
		if userID := strings.TrimSpace(GetUserID(ctx)); userID != "" {
			out["user_id"] = userID
		}
	}
	if strings.TrimSpace(firstCompatString(out, "conversation_id", "conversationId", "session_id", "sessionId")) == "" {
		if conversationID := strings.TrimSpace(GetSessionID(ctx)); conversationID != "" {
			out["conversation_id"] = conversationID
		}
	}
	return out
}
