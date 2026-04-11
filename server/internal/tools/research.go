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
	Query            string
	Mode             string
	ResearchDepth    string
	RouteMode        string
	Lang             string
	Budget           *ResearchBudget
	StrictEntity     *bool
	TimeWindows      []string
	ReportStyle      string
	Topic            string
	URLs             []string
	Text             string
	SearchQueries    []string
	OutputMode       string
	Question         string
	Category         string
	DecisionMode     string
	Candidates       []string
	Context          map[string]interface{}
	Constraints      map[string]interface{}
	Grounding        string
	Depth            string
	Output           string
	ScorecardPack    string
	ScorecardWeights map[string]float64
	Action           string
	URL              string
	Image            string
	Device           string
	Channel          string
	WaitMS           int
	Threshold        float64
	Format           string
	Profile          string
	UserID           string
	ConversationID   string
}

type ResearchJob struct {
	ID                 string
	ConversationID     string
	Status             string
	Query              string
	Mode               string
	ResearchDepth      string
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

type DeepResearchTool struct {
	service ResearchService
}

const (
	DeepResearchActionRun    = "run"
	DeepResearchActionStatus = "status"
)

func NewResearchRunTool(service ResearchService) *ResearchRunTool {
	return &ResearchRunTool{service: service}
}

func NewResearchStatusTool(service ResearchService) *ResearchStatusTool {
	return &ResearchStatusTool{service: service}
}

func NewDeepResearchTool(service ResearchService) *DeepResearchTool {
	return &DeepResearchTool{service: service}
}

func RegisterResearchTools(registry *Registry, service ResearchService) {
	if registry == nil || service == nil {
		return
	}
	registry.Register(NewDeepResearchTool(service))
}

func (t *DeepResearchTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "research",
		Description: "Run a research workflow or poll the status of an existing research job. Use action=run to start Harness research and action=status to resume or poll a job.",
		Icon:        "research",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{DeepResearchActionRun, DeepResearchActionStatus},
					"description": "run to start a research job, status to get progress or final results for an existing job",
				},
				"query":                map[string]interface{}{"type": "string", "description": "Research query or objective. Required for action=run."},
				"input":                map[string]interface{}{"type": "string", "description": "Alias for query. Accepted for compatibility when older callers send input=..."},
				"mode":                 map[string]interface{}{"type": "string", "description": "Research family mode: auto|deep_research|analyze|advisor|ui_review"},
				"research_depth":       map[string]interface{}{"type": "string", "description": "Deep-research depth: fast|standard|deep. Only applies when mode resolves to deep_research."},
				"route_mode":           map[string]interface{}{"type": "string", "description": "Routing mode: web"},
				"lang":                 map[string]interface{}{"type": "string", "description": "Preferred output language"},
				"max_sources":          map[string]interface{}{"type": "integer", "description": "Optional source budget override"},
				"max_seconds":          map[string]interface{}{"type": "integer", "description": "Optional time budget override"},
				"strict_entity":        map[string]interface{}{"type": "boolean", "description": "Enable strict same-entity filtering"},
				"time_windows":         map[string]interface{}{"type": "array", "description": "Optional timeline windows", "items": map[string]interface{}{"type": "string"}},
				"report_style":         map[string]interface{}{"type": "string", "description": "summary|timeline|knowledge_base"},
				"topic":                map[string]interface{}{"type": "string", "description": "Analyze-mode topic/title. Required for bounded synthesis when no explicit query is supplied."},
				"urls":                 map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Analyze-mode source URLs."},
				"text":                 map[string]interface{}{"type": "string", "description": "Analyze-mode direct text input."},
				"search_queries":       map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Analyze-mode supplemental search queries."},
				"output_mode":          map[string]interface{}{"type": "string", "description": "Analyze-mode output: inline|report"},
				"question":             map[string]interface{}{"type": "string", "description": "Advisor-mode decision question."},
				"category":             map[string]interface{}{"type": "string", "description": "Advisor-mode category: architecture|language|framework|library|process|migration|ops"},
				"decision_mode":        map[string]interface{}{"type": "string", "description": "Advisor-mode decision style: recommend|compare|review|replace|best_practice"},
				"candidates":           map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Advisor-mode candidate list."},
				"context":              map[string]interface{}{"type": "object", "description": "Advisor-mode context such as stack, team_size, data_scale, deployment, current_solution."},
				"constraints":          map[string]interface{}{"type": "object", "description": "Advisor-mode hard constraints."},
				"grounding":            map[string]interface{}{"type": "string", "description": "Advisor grounding policy: auto|none|web"},
				"depth":                map[string]interface{}{"type": "string", "description": "Advisor depth: quick|standard|deep"},
				"output":               map[string]interface{}{"type": "string", "description": "Advisor output: decision_memo|scorecard|decision_pack"},
				"scorecard_pack":       map[string]interface{}{"type": "string", "description": "Advisor scorecard pack: auto|solution_selection_v1|migration_v1|architecture_v1|process_v1"},
				"scorecard_weights":    map[string]interface{}{"type": "object", "description": "Advisor scorecard weight overrides keyed by criterion id."},
				"review_action":        map[string]interface{}{"type": "string", "description": "UI-review action alias for callers that keep top-level action reserved for run/status. Accepted values: review_url, review_image, check_accessibility."},
				"url":                  map[string]interface{}{"type": "string", "description": "UI-review target URL."},
				"image":                map[string]interface{}{"type": "string", "description": "UI-review base64 image."},
				"device":               map[string]interface{}{"type": "string", "description": "UI-review device hint: desktop or mobile."},
				"channel":              map[string]interface{}{"type": "string", "description": "UI-review channel hint used for device inference."},
				"wait_ms":              map[string]interface{}{"type": "integer", "description": "UI-review wait time in milliseconds after page load."},
				"threshold":            map[string]interface{}{"type": "number", "description": "UI-review pass threshold."},
				"format":               map[string]interface{}{"type": "string", "description": "UI-review output format: json or human."},
				"profile":              map[string]interface{}{"type": "string", "description": "UI-review rubric profile."},
				"wait":                 map[string]interface{}{"type": "boolean", "description": "Whether to wait for completion when action=run (default true)"},
				"wait_timeout_seconds": map[string]interface{}{"type": "integer", "description": "Optional max wait time before returning pending status"},
				"poll_interval_ms":     map[string]interface{}{"type": "integer", "description": "Polling interval when wait=true (default 500ms)"},
				"job_id":               map[string]interface{}{"type": "string", "description": "Research job ID. Required for action=status."},
				"id":                   map[string]interface{}{"type": "string", "description": "Alias for job_id"},
			},
			"anyOf": []interface{}{
				map[string]interface{}{
					"required": []string{"query"},
				},
				map[string]interface{}{
					"required": []string{"input"},
				},
				map[string]interface{}{
					"required": []string{"job_id"},
				},
				map[string]interface{}{
					"required": []string{"id"},
				},
				map[string]interface{}{
					"required": []string{"topic"},
				},
				map[string]interface{}{
					"required": []string{"url"},
				},
				map[string]interface{}{
					"required": []string{"image"},
				},
			},
		},
	}
}

func firstDeepResearchQuery(args map[string]interface{}) string {
	return strings.TrimSpace(firstCompatString(
		args,
		"query",
		"q",
		"search",
		"input",
		"objective",
		"prompt",
		"message",
		"content",
		"text",
		"topic",
		"question",
	))
}

func firstResearchGoal(args map[string]interface{}) string {
	return strings.TrimSpace(firstCompatString(
		args,
		"query",
		"q",
		"search",
		"input",
		"objective",
		"prompt",
		"message",
		"question",
	))
}

func normalizeDeepResearchAction(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", DeepResearchActionRun:
		return DeepResearchActionRun
	case DeepResearchActionStatus, "poll", "resume":
		return DeepResearchActionStatus
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func resolveDeepResearchAction(args map[string]interface{}) string {
	action := normalizeDeepResearchAction(firstCompatString(args, "action"))
	if action != "" && action != DeepResearchActionRun {
		if action == DeepResearchActionStatus {
			return action
		}
		if parseResearchUIAction(args) != "" {
			return DeepResearchActionRun
		}
		return action
	}
	if strings.TrimSpace(firstCompatString(args, "job_id", "jobId", "id")) != "" &&
		firstResearchGoal(args) == "" {
		return DeepResearchActionStatus
	}
	return DeepResearchActionRun
}

func (t *ResearchRunTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "research_run",
		Description: "Run a Harness research workflow. Optionally wait for the final report or return a job handle for later polling.",
		Icon:        "research",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query":                map[string]interface{}{"type": "string", "description": "Research query or objective"},
				"mode":                 map[string]interface{}{"type": "string", "description": "Research depth: fast, standard, deep"},
				"route_mode":           map[string]interface{}{"type": "string", "description": "Routing mode: web"},
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
		Description: "Get status or final report for a research job created by research_run.",
		Icon:        "research-status",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"job_id": map[string]interface{}{"type": "string", "description": "Research job ID"},
				"id":     map[string]interface{}{"type": "string", "description": "Alias for job_id"},
			},
			"required": []string{"job_id"},
		},
	}
}

func autoSelectResearchMode(query string, requestedMode string) string {
	mode := strings.ToLower(strings.TrimSpace(requestedMode))
	if mode != "" && mode != "auto" {
		return mode
	}
	q := strings.ToLower(strings.TrimSpace(query))

	// UI/UX review signals
	uiSignals := []string{"ui", "ux", "interface", "layout", "design", "mockup", "wireframe",
		"component", "visual", "accessibility", "a11y", "screenshot", "review ui", "ui review",
		"design audit", "界面", "布局", "设计", "视觉", "无障碍", "可访问性", "评审"}
	for _, s := range uiSignals {
		if strings.Contains(q, s) {
			return "ui_review"
		}
	}

	// Deep research signals
	deepSignals := []string{"citations", "evidence", "timeline", "tradeoff", "benchmark",
		"multi-source", "investigate", "study", "research deeply", "深入", "引用", "证据",
		"时间线", "权衡", "基准", "多来源", "调研"}
	for _, s := range deepSignals {
		if strings.Contains(q, s) {
			return "deep_research"
		}
	}

	advisorSignals := []string{"replace", "replacement", "migration", "migrate", "best practice", "tradeoff", "should we use",
		"选型", "替代", "替换", "迁移", "最佳实践", "权衡", " vs ", " versus "}
	for _, s := range advisorSignals {
		if strings.Contains(q, s) {
			return "advisor"
		}
	}

	// Analyze signals (bounded synthesis)
	analyzeSignals := []string{"analyze", "analysis", "summarize", "summary", "synthesize",
		"compare", "report", "insight", "findings", "extract", "分析", "总结", "提炼",
		"比较", "报告", "洞察", "归纳"}
	for _, s := range analyzeSignals {
		if strings.Contains(q, s) {
			return "analyze"
		}
	}

	// Default to deep_research for generic research queries
	return "deep_research"
}

func normalizeResearchDepth(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "fast", "standard", "deep":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func resolveResearchExecutionMode(query string, args map[string]interface{}) (string, string) {
	requestedMode := strings.ToLower(strings.TrimSpace(firstCompatString(args, "mode")))
	requestedDepth := normalizeResearchDepth(firstCompatString(args, "research_depth", "researchDepth", "depth"))

	// Backward compatibility for older deep-research callers that used mode as depth.
	if depth := normalizeResearchDepth(requestedMode); depth != "" {
		if requestedDepth == "" {
			requestedDepth = depth
		}
		return "deep_research", requestedDepth
	}

	effectiveMode := autoSelectResearchMode(query, requestedMode)
	if inferredMode := inferResearchModeFromArgs(args); inferredMode != "" {
		if strings.TrimSpace(query) == "" || effectiveMode == "deep_research" {
			effectiveMode = inferredMode
		}
	}
	if effectiveMode == "deep_research" && requestedDepth == "" {
		requestedDepth = "deep"
	}
	return effectiveMode, requestedDepth
}

func (t *DeepResearchTool) executeRun(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.service == nil {
		return nil, errors.New("research service not available")
	}
	req, err := buildResearchCreateJobRequest(ctx, args)
	if err != nil {
		return nil, err
	}

	job, err := t.service.CreateJob(ctx, req)
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
	finalJob, err := waitForResearchJob(waitCtx, t.service, job.ID, req.UserID, pollInterval)
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

func (t *DeepResearchTool) executeStatus(ctx context.Context, args map[string]interface{}) (interface{}, error) {
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

func (t *DeepResearchTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	switch resolveDeepResearchAction(args) {
	case DeepResearchActionRun:
		return t.executeRun(ctx, args)
	case DeepResearchActionStatus:
		return t.executeStatus(ctx, args)
	default:
		return nil, fmt.Errorf("unsupported deep_research action %q", firstCompatString(args, "action"))
	}
}

func (t *ResearchRunTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	return (&DeepResearchTool{service: t.service}).executeRun(ctx, args)
}

func (t *ResearchStatusTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	return (&DeepResearchTool{service: t.service}).executeStatus(ctx, args)
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
		"research_depth":       job.ResearchDepth,
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

func buildResearchCreateJobRequest(ctx context.Context, args map[string]interface{}) (ResearchCreateJobRequest, error) {
	goal := firstResearchGoal(args)
	effectiveMode, researchDepth := resolveResearchExecutionMode(goal, args)

	req := ResearchCreateJobRequest{
		Mode:           effectiveMode,
		ResearchDepth:  researchDepth,
		RouteMode:      strings.TrimSpace(firstCompatString(args, "route_mode", "routeMode")),
		Lang:           strings.TrimSpace(firstCompatString(args, "lang", "language")),
		Budget:         parseResearchBudget(args),
		StrictEntity:   parseResearchBoolArg(args, "strict_entity", "strictEntity"),
		TimeWindows:    parseResearchStringSliceArgs(args, "time_windows", "timeWindows"),
		ReportStyle:    strings.TrimSpace(firstCompatString(args, "report_style", "reportStyle")),
		UserID:         GetUserID(ctx),
		ConversationID: strings.TrimSpace(GetSessionID(ctx)),
	}
	// Deep-research jobs use mode as the depth selector (fast|standard|deep) in the
	// underlying harness; keep `ResearchDepth` populated for diagnostics/forward
	// compatibility, but always pass the depth via `Mode`.
	if effectiveMode == "deep_research" {
		if researchDepth != "" {
			req.Mode = researchDepth
		} else {
			req.Mode = "deep"
		}
		if req.RouteMode == "" {
			req.RouteMode = "web"
		}
	}

	switch effectiveMode {
	case "analyze":
		normalized := cloneResearchArgs(args)
		normalizeAnalyzeToolArgs(normalized)
		req.Topic = strings.TrimSpace(firstCompatString(normalized, "topic", "subject"))
		req.URLs = parseResearchStringSliceArgs(normalized, "urls")
		req.Text = strings.TrimSpace(firstCompatString(normalized, "text"))
		req.SearchQueries = parseResearchStringSliceArgs(normalized, "search_queries", "searchQueries", "queries")
		req.OutputMode = resolveAnalyzeOutputMode(normalized)
		if reportStyle := resolveAnalyzeReportStyle(normalized, req.Topic); reportStyle != "" {
			req.ReportStyle = reportStyle
		}
		req.Query = firstNonEmptyResearchValue(
			goal,
			req.Topic,
			firstResearchListValue(req.URLs),
			firstResearchListValue(req.SearchQueries),
		)
	case "advisor":
		normalized := cloneResearchArgs(args)
		normalizeAdvisorArgs(normalized)
		req.Question = strings.TrimSpace(firstCompatString(normalized, "question"))
		req.Category = strings.TrimSpace(firstCompatString(normalized, "category"))
		req.DecisionMode = strings.TrimSpace(firstCompatString(normalized, "decision_mode"))
		req.Candidates = advisorCollectStrings(normalized["candidates"])
		req.Context = advisorContextMap(normalized)
		req.Constraints = advisorConstraintsMap(normalized)
		req.Grounding = strings.TrimSpace(firstCompatString(normalized, "grounding"))
		req.Depth = strings.TrimSpace(firstCompatString(normalized, "depth"))
		req.Output = strings.TrimSpace(firstCompatString(normalized, "output"))
		req.ScorecardPack = strings.TrimSpace(firstCompatString(normalized, "scorecard_pack"))
		req.ScorecardWeights = advisorParseWeightMap(normalized["scorecard_weights"])
		req.Query = firstNonEmptyResearchValue(goal, req.Question)
	case "ui_review":
		req.URL = strings.TrimSpace(firstUIReviewCompatURL(args))
		if req.URL == "" {
			req.URL = firstResearchEmbeddedURL(goal)
		}
		req.Image = strings.TrimSpace(firstUIReviewCompatImage(args))
		action, err := parseResearchUICanonicalAction(args, req.URL, req.Image)
		if err != nil {
			return ResearchCreateJobRequest{}, err
		}
		req.Action = action
		req.Device = strings.TrimSpace(firstCompatString(args, "device"))
		req.Channel = strings.TrimSpace(firstCompatString(args, "channel"))
		req.WaitMS = parseResearchIntArg(firstResearchCompatValue(args, "wait_ms", "waitMs"))
		req.Threshold = compatFloat64(args, "threshold")
		req.Format = strings.TrimSpace(firstCompatString(args, "format", "output_format", "outputFormat"))
		req.Profile = strings.TrimSpace(firstCompatString(args, "profile", "quality_profile", "qualityProfile"))
		req.Query = firstNonEmptyResearchValue(
			goal,
			req.URL,
		)
		if req.Query == "" && req.Image != "" {
			req.Query = "Review provided image"
		}
	default:
		req.Query = firstNonEmptyResearchValue(goal, strings.TrimSpace(firstCompatString(args, "text", "content")))
	}

	if req.Query == "" {
		switch effectiveMode {
		case "deep_research":
			return ResearchCreateJobRequest{}, errors.New("query is required")
		case "ui_review":
			return ResearchCreateJobRequest{}, errors.New("url or image is required for ui_review")
		case "analyze":
			return ResearchCreateJobRequest{}, errors.New("topic, urls, text, or search_queries are required for analyze")
		case "advisor":
			return ResearchCreateJobRequest{}, errors.New("question is required for advisor")
		default:
			return ResearchCreateJobRequest{}, errors.New("query is required")
		}
	}

	return req, nil
}

func inferResearchModeFromArgs(args map[string]interface{}) string {
	if strings.TrimSpace(firstCompatString(args, "question")) != "" ||
		strings.TrimSpace(firstCompatString(args, "category")) != "" ||
		strings.TrimSpace(firstCompatString(args, "decision_mode", "decisionMode")) != "" ||
		len(advisorCollectStrings(args["candidates"])) > 0 {
		return "advisor"
	}
	if contextMap, ok := coerceCompatMap(args["context"]); ok {
		for _, key := range []string{"current_solution", "team_size", "data_scale", "deployment", "must_have", "must_avoid"} {
			if strings.TrimSpace(fmt.Sprint(contextMap[key])) != "" {
				return "advisor"
			}
		}
	}
	if strings.TrimSpace(parseResearchUIAction(args)) != "" ||
		strings.TrimSpace(firstUIReviewCompatImage(args)) != "" {
		return "ui_review"
	}
	if url := strings.TrimSpace(firstUIReviewCompatURL(args)); url != "" {
		if strings.TrimSpace(firstCompatString(args, "device", "channel", "format", "profile", "review_action", "reviewAction", "ui_review_action", "uiReviewAction")) != "" ||
			firstResearchCompatValue(args, "wait_ms", "waitMs") != nil ||
			firstResearchCompatValue(args, "threshold") != nil ||
			(strings.TrimSpace(firstCompatString(args, "topic", "subject")) == "" &&
				len(parseResearchStringSliceArgs(args, "urls")) == 0 &&
				len(parseResearchStringSliceArgs(args, "search_queries", "searchQueries", "queries")) == 0 &&
				strings.TrimSpace(firstCompatString(args, "text")) == "") {
			return "ui_review"
		}
	}
	if strings.TrimSpace(firstCompatString(args, "topic", "subject")) != "" ||
		len(parseResearchStringSliceArgs(args, "urls")) > 0 ||
		len(parseResearchStringSliceArgs(args, "search_queries", "searchQueries", "queries")) > 0 ||
		strings.TrimSpace(firstCompatString(args, "text")) != "" ||
		strings.TrimSpace(firstCompatString(args, "output_mode", "outputMode")) != "" ||
		parseResearchBoolArg(args, "report", "generate_report", "generateReport", "html_report", "htmlReport") != nil {
		return "analyze"
	}
	return ""
}

func parseResearchUICanonicalAction(args map[string]interface{}, url string, image string) (string, error) {
	action := strings.TrimSpace(parseResearchUIAction(args))
	return CanonicalizeUIReviewAction(action, url, image)
}

func parseResearchUIAction(args map[string]interface{}) string {
	if args == nil {
		return ""
	}
	if action := strings.TrimSpace(firstCompatString(args, "review_action", "reviewAction", "ui_review_action", "uiReviewAction")); action != "" {
		return action
	}
	for _, containerKey := range []string{"arguments", "input", "params", "payload"} {
		nested, ok := coerceCompatMap(args[containerKey])
		if !ok {
			continue
		}
		if action := strings.TrimSpace(firstCompatString(nested, "action", "op", "operation", "command")); action != "" {
			return action
		}
	}
	if raw, ok := args["action"]; ok {
		action := strings.TrimSpace(fmt.Sprint(raw))
		if normalized := normalizeDeepResearchAction(action); normalized != DeepResearchActionRun && normalized != DeepResearchActionStatus {
			return action
		}
	}
	return ""
}

func firstResearchCompatValue(args map[string]interface{}, keys ...string) interface{} {
	if args == nil {
		return nil
	}
	value, ok := compatArgValue(args, keys...)
	if !ok {
		return nil
	}
	return value
}

func cloneResearchArgs(args map[string]interface{}) map[string]interface{} {
	if len(args) == 0 {
		return map[string]interface{}{}
	}
	cloned := make(map[string]interface{}, len(args))
	for key, value := range args {
		cloned[key] = value
	}
	return cloned
}

func firstResearchListValue(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

func firstResearchEmbeddedURL(value string) string {
	return strings.TrimSpace(analyzeTopicURLPattern.FindString(strings.TrimSpace(value)))
}

func firstNonEmptyResearchValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
