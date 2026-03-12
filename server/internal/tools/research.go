package tools

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type ResearchService interface {
	CreateJob(ctx context.Context, req ResearchCreateJobRequest) (*ResearchJob, error)
	GetJobForUser(id, userID string) (*ResearchJob, error)
}

type ResearchBudget struct {
	MaxSources int
	MaxSeconds int
}

type ResearchCreateJobRequest struct {
	Query          string
	Mode           string
	RouteMode      string
	Lang           string
	Budget         *ResearchBudget
	StrictEntity   *bool
	TimeWindows    []string
	ReportStyle    string
	UserID         string
	ConversationID string
}

type ResearchJob struct {
	ID                 string
	ConversationID     string
	Status             string
	Query              string
	Mode               string
	RequestedRouteMode string
	EffectiveRouteMode string
	RouteReason        string
	Progress           int
	EvidenceCount      int
	Answer             string
	Confidence         float64
	Error              string
	Report             map[string]interface{}
}

type ResearchRunTool struct {
	service ResearchService
}

type ResearchStatusTool struct {
	service ResearchService
}

func NewResearchRunTool(service ResearchService) *ResearchRunTool {
	return &ResearchRunTool{service: service}
}

func NewResearchStatusTool(service ResearchService) *ResearchStatusTool {
	return &ResearchStatusTool{service: service}
}

func RegisterResearchTools(registry *Registry, service ResearchService) {
	if registry == nil || service == nil {
		return
	}
	registry.Register(NewResearchRunTool(service))
	registry.Register(NewResearchStatusTool(service))
}

func (t *ResearchRunTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "research_run",
		Description: "Run deep research with route_mode auto|web|experiment|hybrid. Optionally wait for the final report or return a job handle for later polling.",
		Icon:        "research",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query":                map[string]interface{}{"type": "string", "description": "Research query or objective"},
				"mode":                 map[string]interface{}{"type": "string", "description": "Research depth: fast, standard, deep"},
				"route_mode":           map[string]interface{}{"type": "string", "description": "Routing mode: auto, web, experiment, hybrid"},
				"lang":                 map[string]interface{}{"type": "string", "description": "Preferred output language"},
				"max_sources":          map[string]interface{}{"type": "integer", "description": "Optional source budget override"},
				"max_seconds":          map[string]interface{}{"type": "integer", "description": "Optional time budget override"},
				"strict_entity":        map[string]interface{}{"type": "boolean", "description": "Enable strict same-entity filtering"},
				"time_windows":         map[string]interface{}{"type": "array", "description": "Optional timeline windows", "items": map[string]interface{}{"type": "string"}},
				"report_style":         map[string]interface{}{"type": "string", "description": "summary|timeline|knowledge_base"},
				"wait":                 map[string]interface{}{"type": "boolean", "description": "Whether to wait for completion (default true)"},
				"wait_timeout_seconds": map[string]interface{}{"type": "integer", "description": "Optional max wait time before returning pending status"},
				"poll_interval_ms":     map[string]interface{}{"type": "integer", "description": "Polling interval when wait=true (default 500ms)"},
			},
			"required": []string{"query"},
		},
	}
}

func (t *ResearchStatusTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "research_status",
		Description: "Get status or final report for a deep research job created by research_run.",
		Icon:        "research-status",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"job_id": map[string]interface{}{"type": "string", "description": "Deep research job ID"},
				"id":     map[string]interface{}{"type": "string", "description": "Alias for job_id"},
			},
			"required": []string{"job_id"},
		},
	}
}

func (t *ResearchRunTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.service == nil {
		return nil, errors.New("research service not available")
	}
	query := strings.TrimSpace(firstCompatString(args, "query", "objective", "prompt", "message"))
	if query == "" {
		return nil, errors.New("query is required")
	}
	userID := GetUserID(ctx)
	job, err := t.service.CreateJob(ctx, ResearchCreateJobRequest{
		Query:          query,
		Mode:           strings.TrimSpace(firstCompatString(args, "mode")),
		RouteMode:      strings.TrimSpace(firstCompatString(args, "route_mode", "routeMode")),
		Lang:           strings.TrimSpace(firstCompatString(args, "lang", "language")),
		Budget:         parseResearchBudget(args),
		StrictEntity:   parseResearchBoolArg(args, "strict_entity", "strictEntity"),
		TimeWindows:    parseResearchStringSliceArgs(args, "time_windows", "timeWindows"),
		ReportStyle:    strings.TrimSpace(firstCompatString(args, "report_style", "reportStyle")),
		UserID:         userID,
		ConversationID: strings.TrimSpace(GetSessionID(ctx)),
	})
	if err != nil {
		return nil, err
	}
	EmitCard(ctx, map[string]interface{}{
		"type":                 "deep-research-job",
		"status":               job.Status,
		"job_id":               job.ID,
		"conversation_id":      job.ConversationID,
		"query":                job.Query,
		"requested_route_mode": job.RequestedRouteMode,
		"effective_route_mode": job.EffectiveRouteMode,
	})
	wait := true
	if parsed := parseResearchBoolArg(args, "wait"); parsed != nil {
		wait = *parsed
	}
	if !wait {
		payload := researchJobToMap(job)
		payload["accepted"] = true
		payload["terminal"] = isResearchTerminalStatus(job.Status)
		return payload, nil
	}
	pollInterval := 500 * time.Millisecond
	if raw, ok := compatArgValue(args, "poll_interval_ms", "pollIntervalMs"); ok {
		if ms := parseResearchIntArg(raw); ms > 0 {
			pollInterval = time.Duration(ms) * time.Millisecond
		}
	}
	var waitCtx context.Context
	var cancel context.CancelFunc
	if raw, ok := compatArgValue(args, "wait_timeout_seconds", "waitTimeoutSeconds"); ok {
		if seconds := parseResearchIntArg(raw); seconds > 0 {
			waitCtx, cancel = context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
			defer cancel()
		} else {
			waitCtx = ctx
		}
	} else {
		waitCtx = ctx
	}
	finalJob, err := waitForResearchJob(waitCtx, t.service, job.ID, userID, pollInterval)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			payload := researchJobToMap(job)
			payload["accepted"] = true
			payload["terminal"] = false
			payload["wait_timeout"] = true
			return payload, nil
		}
		return nil, err
	}
	if strings.EqualFold(finalJob.Status, "failed") {
		if strings.TrimSpace(finalJob.Error) == "" {
			return nil, fmt.Errorf("research job failed")
		}
		return nil, fmt.Errorf("research job failed: %s", finalJob.Error)
	}
	if strings.EqualFold(finalJob.Status, "cancelled") {
		return nil, fmt.Errorf("research job cancelled")
	}
	payload := researchJobToMap(finalJob)
	payload["accepted"] = true
	payload["terminal"] = true
	return payload, nil
}

func (t *ResearchStatusTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.service == nil {
		return nil, errors.New("research service not available")
	}
	jobID := strings.TrimSpace(firstCompatString(args, "job_id", "jobId", "id"))
	if jobID == "" {
		return nil, errors.New("job_id is required")
	}
	job, err := t.service.GetJobForUser(jobID, GetUserID(ctx))
	if err != nil {
		return nil, err
	}
	payload := researchJobToMap(job)
	payload["terminal"] = isResearchTerminalStatus(job.Status)
	return payload, nil
}

func waitForResearchJob(ctx context.Context, service ResearchService, jobID, userID string, pollInterval time.Duration) (*ResearchJob, error) {
	if pollInterval <= 0 {
		pollInterval = 500 * time.Millisecond
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		job, err := service.GetJobForUser(jobID, userID)
		if err != nil {
			return nil, err
		}
		if isResearchTerminalStatus(job.Status) {
			return job, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func researchJobToMap(job *ResearchJob) map[string]interface{} {
	payload := map[string]interface{}{
		"job_id":               job.ID,
		"status":               job.Status,
		"query":                job.Query,
		"mode":                 job.Mode,
		"requested_route_mode": job.RequestedRouteMode,
		"effective_route_mode": job.EffectiveRouteMode,
		"route_reason":         job.RouteReason,
		"progress":             job.Progress,
		"evidence_count":       job.EvidenceCount,
		"answer":               job.Answer,
		"confidence":           job.Confidence,
	}
	if strings.TrimSpace(job.Error) != "" {
		payload["error"] = job.Error
	}
	if len(job.Report) > 0 {
		payload["report"] = job.Report
	}
	return payload
}

func isResearchTerminalStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func parseResearchBudget(args map[string]interface{}) *ResearchBudget {
	maxSourcesRaw, _ := compatArgValue(args, "max_sources", "maxSources")
	maxSecondsRaw, _ := compatArgValue(args, "max_seconds", "maxSeconds")
	maxSources := parseResearchIntArg(maxSourcesRaw)
	maxSeconds := parseResearchIntArg(maxSecondsRaw)
	if maxSources <= 0 && maxSeconds <= 0 {
		return nil
	}
	return &ResearchBudget{MaxSources: maxSources, MaxSeconds: maxSeconds}
}

func parseResearchBoolArg(args map[string]interface{}, keys ...string) *bool {
	if args == nil {
		return nil
	}
	raw, ok := compatArgValue(args, keys...)
	if !ok {
		return nil
	}
	if value, ok := asCompatBool(raw); ok {
		return &value
	}
	return nil
}

func parseResearchStringSliceArgs(args map[string]interface{}, keys ...string) []string {
	raw, ok := compatArgValue(args, keys...)
	if !ok {
		return nil
	}
	return parseResearchStringSlice(raw)
}

func parseResearchIntArg(v interface{}) int {
	switch tv := v.(type) {
	case int:
		return tv
	case int32:
		return int(tv)
	case int64:
		return int(tv)
	case float32:
		return int(tv)
	case float64:
		return int(tv)
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(tv)); err == nil {
			return n
		}
	}
	return 0
}

func parseResearchStringSlice(v interface{}) []string {
	switch tv := v.(type) {
	case []string:
		out := make([]string, 0, len(tv))
		for _, item := range tv {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(tv))
		for _, item := range tv {
			s, _ := item.(string)
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		s := strings.TrimSpace(tv)
		if s != "" {
			return []string{s}
		}
	}
	return nil
}
